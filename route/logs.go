package route

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// requestLogCapacity bounds the in-memory history. The console shows all of
	// it, so this also bounds what a browser has to hold.
	requestLogCapacity = 500
	// requestLogSearchCap bounds how much of a search reply is buffered. It is
	// used to count the results and to harvest the per-source summary the
	// console groups by number, so an aggregate search over every provider has
	// to fit: 4 MiB covers ~30 providers with full metadata and stays cheap.
	requestLogSearchCap = 4 << 20
	// requestLogBodyCap bounds a small JSON reply (detail/info) buffered only to
	// pull out the number and title.
	requestLogBodyCap = 8 << 10
	// requestLogErrorCap bounds how much of an error reply is buffered, which is
	// only used to pull out the message.
	requestLogErrorCap = 2 << 10
	// requestLogResultCap bounds how many hits are kept per request for the
	// console's friendly summary. Ten is what a task detail can show without
	// scrolling forever.
	requestLogResultCap = 10
	// requestLogSessionCount bounds how many lookups the overview can show.
	requestLogSessionCount = 12
)

// logResult is one search hit, kept small on purpose: the console only needs
// enough of it to draw a card (which source, which number, which title).
type logResult struct {
	Provider string  `json:"provider"`
	ID       string  `json:"id"`
	Number   string  `json:"number,omitempty"`
	Title    string  `json:"title,omitempty"`
	Actors   string  `json:"actors,omitempty"`  // comma-joined, first few only
	Release  string  `json:"release,omitempty"` // yyyy-mm-dd when the provider gave one
	Score    float64 `json:"score,omitempty"`
}

// requestLog is one served request, shaped for display rather than for export:
// the console turns these fields into a single readable line.
type requestLog struct {
	Seq      uint64  `json:"seq"`
	TS       int64   `json:"ts_ms"` // unix milliseconds, formatted by the browser
	Method   string  `json:"method"`
	Path     string  `json:"path"`
	Query    string  `json:"query,omitempty"`
	Route    string  `json:"route,omitempty"` // matched pattern, e.g. /v1/movies/:provider/:id
	Kind     string  `json:"kind"`            // search / movie / image / translate / ...
	Provider string  `json:"provider,omitempty"`
	ID       string  `json:"id,omitempty"`
	Q        string  `json:"q,omitempty"`
	Status   int     `json:"status"`
	Ms       float64 `json:"ms"`
	Bytes    int64   `json:"bytes"`
	IP       string  `json:"ip,omitempty"`
	Auth     bool    `json:"auth"` // request carried a bearer token
	Items    int     `json:"items,omitempty"`
	Err      string  `json:"error,omitempty"`
	// Results is what a successful search actually returned, trimmed to the
	// handful of fields the console draws. It is used both by the log detail
	// view and to rebuild a readable summary of one lookup.
	Results []logResult `json:"results,omitempty"`
}

// requestLogStore keeps the tail of the request stream in memory. Nothing is
// written to disk: the console is a live diagnostic, not an audit trail.
type requestLogStore struct {
	mu    sync.RWMutex
	items []requestLog
	seq   uint64
	total int64
	errs  int64
	slow  int64
	sumMs float64
}

var requestLogs = &requestLogStore{}

func (s *requestLogStore) append(rec requestLog) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	rec.Seq = s.seq
	s.items = append(s.items, rec)
	if len(s.items) > requestLogCapacity {
		// Shift in place so the backing array does not grow over time.
		copy(s.items, s.items[len(s.items)-requestLogCapacity:])
		s.items = s.items[:requestLogCapacity]
	}

	s.total++
	s.sumMs += rec.Ms
	if rec.Status >= http.StatusBadRequest {
		s.errs++
	}
	if rec.Ms >= 1000 {
		s.slow++
	}
}

// snapshot returns entries newer than since, oldest first, capped to the newest
// limit entries, plus the sequence number of the newest entry overall.
func (s *requestLogStore) snapshot(since uint64, limit int) ([]requestLog, uint64, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]requestLog, 0, len(s.items))
	for _, it := range s.items {
		if it.Seq > since {
			out = append(out, it)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, s.seq, len(s.items)
}

func (s *requestLogStore) stats(buffered int) gin.H {
	s.mu.RLock()
	defer s.mu.RUnlock()

	avg := 0.0
	if s.total > 0 {
		avg = s.sumMs / float64(s.total)
	}
	return gin.H{
		"total":     s.total,
		"errors":    s.errs,
		"slow":      s.slow,
		"avg_ms":    float64(int64(avg*10)) / 10,
		"buffered":  buffered,
		"capacity":  requestLogCapacity,
		"last_seq":  s.seq,
		"truncated": s.total > int64(buffered),
	}
}

// reset drops the history but keeps the sequence number monotonic, so a browser
// that is tailing with since=<seq> does not resurrect entries it already had.
func (s *requestLogStore) reset() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := len(s.items)
	s.items = s.items[:0]
	s.total = 0
	s.errs = 0
	s.slow = 0
	s.sumMs = 0
	return n
}

// logWriter captures what the handler answered: status, size, and — only when
// useful — the first part of the body.
type logWriter struct {
	gin.ResponseWriter
	status  int
	bytes   int64
	search  bool
	capture *bytes.Buffer
	budget  int
}

func (w *logWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *logWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	// Buffer the body only when it can tell us something: the result count of a
	// successful search, or the message of an error reply.
	if w.capture == nil {
		if w.search && w.status < http.StatusMultipleChoices {
			w.capture, w.budget = &bytes.Buffer{}, requestLogSearchCap
		} else if w.status >= http.StatusBadRequest {
			w.capture, w.budget = &bytes.Buffer{}, requestLogErrorCap
		}
	}
	if w.capture != nil && w.capture.Len() < w.budget {
		if room := w.budget - w.capture.Len(); room >= len(b) {
			w.capture.Write(b)
		} else {
			w.capture.Write(b[:room])
		}
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

func (w *logWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *logWriter) statusCode() int {
	if w.status != 0 {
		return w.status
	}
	if s := w.ResponseWriter.Status(); s != 0 {
		return s
	}
	return http.StatusOK
}

// requestLogger records every request except the console's own polling, which
// would otherwise drown out the traffic worth looking at.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if skipRequestLog(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()
		w := &logWriter{
			ResponseWriter: c.Writer,
			search:         strings.HasSuffix(c.Request.URL.Path, "/search"),
		}
		c.Writer = w

		// Deferred so a panic that recovery() turns into a 500 is recorded with
		// its real status.
		defer func() {
			rec := requestLog{
				TS:       start.UnixNano() / int64(time.Millisecond),
				Method:   c.Request.Method,
				Path:     c.Request.URL.Path,
				Query:    redactQuery(c.Request.URL.RawQuery),
				Route:    c.FullPath(),
				Kind:     classifyRequest(c.FullPath()),
				Provider: c.Param("provider"),
				ID:       c.Param("id"),
				Q:        c.Query("q"),
				Status:   w.statusCode(),
				Ms:       float64(time.Since(start).Microseconds()) / 1000,
				Bytes:    w.bytes,
				IP:       c.ClientIP(),
				Auth:     c.GetHeader("Authorization") != "",
			}
			if rec.Provider == "" {
				rec.Provider = c.Query("provider")
			}
			if body := w.capture; body != nil {
				if rec.Status >= http.StatusBadRequest {
					rec.Err = errorMessage(body.Bytes())
				} else if w.search {
					rec.Items, rec.Results = parseSearchBody(body.Bytes())
				}
			}
			requestLogs.append(rec)
		}()

		c.Next()
	}
}

// skipRequestLog hides the console's own chatter: the page itself, its status
// poll and the log query that feeds this very list.
func skipRequestLog(path string) bool {
	switch path {
	// /panel/info is the console's own probe when it is served by the panel exe;
	// on a plain server that path simply 404s and is not worth a log line.
	// /v1/translate/config carries engine credentials, so it is never recorded.
	case "/ui", "/v1/status", "/v1/logs", "/favicon.ico", "/panel/info",
		"/v1/translate/config":
		return true
	}
	return strings.HasPrefix(path, "/ui/")
}

func classifyRequest(route string) string {
	switch {
	case route == "":
		return "unknown" // no route matched
	case route == "/":
		return "root"
	case route == "/v1/movies/search":
		return "search"
	case route == "/v1/actors/search":
		return "actor_search"
	case route == "/v1/movies/:provider/:id":
		return "movie"
	case route == "/v1/actors/:provider/:id":
		return "actor"
	case route == "/v1/reviews/:provider/:id":
		return "review"
	case strings.HasPrefix(route, "/v1/images/"):
		return "image"
	case route == "/v1/translate":
		return "translate"
	case route == "/v1/db/version":
		return "db"
	case route == "/v1/providers":
		return "providers"
	case route == "/v1/modules":
		return "modules"
	}
	return "other"
}

// errorMessage pulls "message" out of the response envelope, falling back to the
// raw (already truncated) body when it is not the shape we expect.
// redactQuery keeps the query string readable but hides engine credentials: the
// Metadata agent sends its translation keys as query parameters, and the log is
// shown in the console and kept in memory.
func redactQuery(raw string) string {
	if raw == "" || !strings.Contains(raw, "=") {
		return raw
	}
	parts := strings.Split(raw, "&")
	changed := false
	for i, part := range parts {
		key, _, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		if _, ok := sensitiveQueryKeys[strings.ToLower(key)]; !ok {
			continue
		}
		parts[i] = key + "=***"
		changed = true
	}
	if !changed {
		return raw
	}
	return strings.Join(parts, "&")
}

var sensitiveQueryKeys = map[string]struct{}{
	"token": {}, "access_token": {}, "authorization": {},
	"baidu-app-id": {}, "baidu-app-key": {}, "appid": {},
	"deepl-api-key": {}, "google-api-key": {}, "openai-api-key": {},
}

func errorMessage(body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Message != "" {
		return envelope.Error.Message
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return msg
}

// searchHit mirrors the part of a search result the console shows. Only a few
// fields are decoded: an aggregate reply over every provider is large, and the
// rest of it is of no use to a request log.
type searchHit struct {
	Provider string   `json:"provider"`
	ID       string   `json:"id"`
	Number   string   `json:"number"`
	Title    string   `json:"title"`
	Actors   []string `json:"actors"`
	Release  string   `json:"release_date"`
	Score    float64  `json:"score"`
}

// parseSearchBody numbers the hits and keeps a small digest of each, in the
// order the server returned them (best match first).
func parseSearchBody(body []byte) (int, []logResult) {
	var envelope struct {
		Data []searchHit `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return 0, nil
	}
	out := make([]logResult, 0, requestLogResultCap)
	for _, h := range envelope.Data {
		if len(out) >= requestLogResultCap {
			break
		}
		if h.Provider == "" || h.ID == "" {
			continue
		}
		r := logResult{
			Provider: h.Provider,
			ID:       h.ID,
			Number:   h.Number,
			Title:    truncateRunes(h.Title, 140),
			Release:  h.Release,
			Score:    h.Score,
		}
		if actors := h.Actors; len(actors) > 0 {
			if len(actors) > 4 {
				actors = actors[:4]
			}
			r.Actors = strings.Join(actors, ",")
		}
		out = append(out, r)
	}
	return len(envelope.Data), out
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// logSource is one provider's contribution to a lookup.
type logSource struct {
	Provider string  `json:"provider"`
	Status   int     `json:"status"`
	Ms       float64 `json:"ms"`
	Items    int     `json:"items"`
	Error    string  `json:"error,omitempty"`
}

// logSession merges every request that belongs to one lookup of one number, so
// the overview can say "searched ABC-123, 6 hits from 4 sources" instead of
// filling the page with a dozen rows of transport detail.
type logSession struct {
	Q         string         `json:"q"`
	TS        int64          `json:"ts_ms"`
	StartTS   int64          `json:"start_ms"`
	Requests  int            `json:"requests"`
	Ok        bool           `json:"ok"`
	Status    int            `json:"status"`
	Items     int            `json:"items"`
	Ms        float64        `json:"ms"`
	Providers []string       `json:"providers"`
	Sources   []logSource    `json:"sources,omitempty"`
	Results   []logResult    `json:"results,omitempty"`
	Error     string         `json:"error,omitempty"`
	Kinds     map[string]int `json:"kinds,omitempty"`
}

// sessionGap splits one number's history when the user comes back to the same
// number later: two lookups ten minutes apart are two entries, not one.
const sessionGap = 10 * time.Minute

func sessionKind(kind string) bool {
	switch kind {
	case "search", "actor_search", "movie", "actor", "review":
		return true
	}
	return false
}

// sessionAcc accumulates a lookup while the log is walked from newest to oldest.
type sessionAcc struct {
	sess    logSession
	sources map[string]*logSource
	results map[string]bool
	okSeen  bool
}

func newSessionAcc(q string, ts int64) *sessionAcc {
	return &sessionAcc{
		sess:    logSession{Q: q, TS: ts, StartTS: ts, Kinds: map[string]int{}},
		sources: map[string]*logSource{},
		results: map[string]bool{},
	}
}

func (a *sessionAcc) source(p string) *logSource {
	if s, ok := a.sources[p]; ok {
		return s
	}
	s := &logSource{Provider: p}
	a.sources[p] = s
	return s
}

// add folds one request into the lookup and reports the result ids that were
// new, so a later detail request for the same movie can be attached to it.
func (a *sessionAcc) add(it requestLog) []string {
	a.sess.Requests++
	if it.TS > a.sess.TS {
		a.sess.TS = it.TS
	}
	if it.TS < a.sess.StartTS {
		a.sess.StartTS = it.TS
	}
	a.sess.Ms += it.Ms
	a.sess.Kinds[it.Kind]++
	if it.Status > a.sess.Status {
		a.sess.Status = it.Status
	}
	if it.Status >= http.StatusBadRequest && it.Err != "" && a.sess.Error == "" {
		a.sess.Error = it.Err
	}

	var fresh []string
	switch it.Kind {
	case "search", "actor_search":
		if it.Status < http.StatusBadRequest {
			a.okSeen = true
		}
		if it.Provider != "" {
			s := a.source(it.Provider)
			s.Status = it.Status
			if it.Ms > s.Ms {
				s.Ms = it.Ms
			}
			if it.Items > s.Items {
				s.Items = it.Items
			}
			if it.Err != "" {
				s.Error = it.Err
			}
		} else {
			// Aggregate search: the reply itself says which providers answered,
			// but carries no per-provider timing.
			counts := map[string]int{}
			for _, r := range it.Results {
				counts[r.Provider]++
			}
			for p, n := range counts {
				s := a.source(p)
				if n > s.Items {
					s.Items = n
				}
				if s.Status == 0 {
					s.Status = it.Status
				}
			}
		}
		for _, r := range it.Results {
			key := r.Provider + "/" + r.ID
			if a.results[key] {
				continue
			}
			a.results[key] = true
			a.sess.Results = append(a.sess.Results, r)
			fresh = append(fresh, key)
		}
	}
	return fresh
}

func (a *sessionAcc) finish() logSession {
	out := a.sess
	out.Ok = a.okSeen
	out.Sources = make([]logSource, 0, len(a.sources))
	for _, s := range a.sources {
		out.Sources = append(out.Sources, *s)
		out.Providers = append(out.Providers, s.Provider)
		out.Items += s.Items
	}
	sort.Slice(out.Sources, func(i, j int) bool {
		if out.Sources[i].Items != out.Sources[j].Items {
			return out.Sources[i].Items > out.Sources[j].Items
		}
		return out.Sources[i].Provider < out.Sources[j].Provider
	})
	sort.Strings(out.Providers)
	if out.Items == 0 {
		out.Items = len(out.Results)
	}
	if len(out.Results) > 12 {
		out.Results = out.Results[:12]
	}
	return out
}

// sessions rebuilds readable lookups out of the request stream. It walks from
// newest to oldest so a repeated search of the same number starts a fresh
// entry rather than merging into an older one.
func (s *requestLogStore) sessions(limit int) []logSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		order []*sessionAcc
		byQ   = map[string]*sessionAcc{}
		byID  = map[string]*sessionAcc{}
	)
	for i := len(s.items) - 1; i >= 0; i-- {
		it := s.items[i]
		if !sessionKind(it.Kind) {
			continue
		}

		acc := (*sessionAcc)(nil)
		if q := normalizeNumber(it.Q); q != "" {
			acc = byQ[q]
			if acc == nil || acc.sess.TS-it.TS > sessionGap.Milliseconds() {
				acc = newSessionAcc(it.Q, it.TS)
				byQ[q] = acc
				order = append(order, acc)
			}
		} else if it.Provider != "" && it.ID != "" {
			// A detail/image request that belongs to a lookup we already have.
			acc = byID[it.Provider+"/"+it.ID]
			if acc == nil {
				continue
			}
		} else {
			continue
		}

		for _, key := range acc.add(it) {
			byID[key] = acc
		}
	}

	if limit > 0 && len(order) > limit {
		order = order[:limit]
	}
	out := make([]logSession, 0, len(order))
	for _, a := range order {
		out = append(out, a.finish())
	}
	return out
}

func normalizeNumber(q string) string {
	return strings.ToUpper(strings.TrimSpace(q))
}

// getRequestLogs serves the tail of the log. Consumers pass since=<last seq> to
// get only what is new, which keeps the console's polling cheap.
func getRequestLogs() gin.HandlerFunc {
	return func(c *gin.Context) {
		since, _ := strconv.ParseUint(c.Query("since"), 10, 64)
		limit, _ := strconv.Atoi(c.Query("limit"))
		if limit <= 0 || limit > requestLogCapacity {
			limit = requestLogCapacity
		}

		items, last, buffered := requestLogs.snapshot(since, limit)
		c.JSON(http.StatusOK, &responseMessage{Data: gin.H{
			"items":    items,
			"last_seq": last,
			"stats":    requestLogs.stats(buffered),
			// Lookups are always rebuilt in full: they are the readable view the
			// overview shows, and they must survive the since=<seq> polling.
			"sessions": requestLogs.sessions(requestLogSessionCount),
		}})
	}
}

func clearRequestLogs() gin.HandlerFunc {
	return func(c *gin.Context) {
		cleared := requestLogs.reset()
		c.JSON(http.StatusOK, &responseMessage{Data: gin.H{
			"cleared":  cleared,
			"last_seq": requestLogs.stats(0)["last_seq"],
		}})
	}
}
