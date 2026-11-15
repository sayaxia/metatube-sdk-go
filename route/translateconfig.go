package route

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/schema"

	"github.com/metatube-community/metatube-sdk-go/route/auth"
)

// Translation engines are configured per request: the Metadata agent sends its
// own credentials in the query string. The console has nowhere to put them, so
// it keeps them in a small json file next to the server binary and lets the
// server merge them in as defaults.
//
// The file uses the engines' own json tags, so any engine works without a
// schema here:
//
//	{
//	  "engine": "baiduai",
//	  "baidu-app-id": "20240101000000000",
//	  "baidu-app-key": "xxxxxxxx",
//	  "baidu-model-type": "nmt"
//	}
//
// It is re-read when its modification time changes, so editing it (in the
// console or by hand) takes effect without restarting the server.
//
// In containers the binary usually lives on a throwaway layer, so the whole
// object can instead be supplied as inline JSON in MT_TRANSLATE_JSON. That
// value is the baseline; individual keys from the file (if any) override it,
// so the console's PUT still works and a container recreate simply falls back
// to the environment. No file needs to be mounted for credentials to persist.
const translateConfigName = "translate.json"

// translateConfigEnv holds the same JSON object as translate.json, inline.
const translateConfigEnv = "MT_TRANSLATE_JSON"

// translateConfigEngineKey is the only key the file holds that is not an engine
// field: it picks the engine the console defaults to.
const translateConfigEngineKey = "engine"

type translateConfigStore struct {
	mu     sync.Mutex
	path   string
	mtime  time.Time
	size   int64
	envRaw string
	raw    map[string]json.RawMessage
}

var translateConfig = newTranslateConfigStore()

func newTranslateConfigStore() *translateConfigStore {
	return &translateConfigStore{path: translateConfigPath()}
}

// translateConfigPath prefers an explicit override, then the folder holding the
// executable — the same place the database and the panel settings live, so a
// shortcut or a service can start the server from any working directory.
func translateConfigPath() string {
	if p := os.Getenv("MT_TRANSLATE_CONFIG"); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), translateConfigName)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return translateConfigName
	}
	return filepath.Join(cwd, translateConfigName)
}

// parseTranslateEnv reads MT_TRANSLATE_JSON as a JSON object of engine fields.
// Malformed JSON is ignored: a bad environment variable must not break every
// translation request.
func parseTranslateEnv(raw string) map[string]json.RawMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil
	}
	return fields
}

// reload refreshes the cached copy when the file or the environment changed.
// A malformed file keeps the previous values in place instead of breaking
// every translation.
func (s *translateConfigStore) reload() map[string]json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	env := os.Getenv(translateConfigEnv)
	merged := parseTranslateEnv(env)

	st, err := os.Stat(s.path)
	if err != nil {
		if merged == nil {
			merged = map[string]json.RawMessage{}
		}
		s.raw, s.mtime, s.size, s.envRaw = merged, time.Time{}, 0, env
		return s.raw
	}
	if s.raw != nil && s.envRaw == env && st.ModTime().Equal(s.mtime) && st.Size() == s.size {
		return s.raw
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return s.raw
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return s.raw
	}
	if merged == nil {
		merged = map[string]json.RawMessage{}
	}
	for k, v := range fields { // file values override the environment baseline
		merged[k] = v
	}
	s.raw, s.mtime, s.size, s.envRaw = merged, st.ModTime(), st.Size(), env
	return s.raw
}

// values returns the file content as strings, skipping anything that is not a
// plain string (an engine credential never is a number or an object).
func (s *translateConfigStore) values() map[string]string {
	out := map[string]string{}
	for k, v := range s.reload() {
		var str string
		if json.Unmarshal(v, &str) == nil && str != "" {
			out[k] = str
		}
	}
	return out
}

// engine is the engine the console should use by default.
func (s *translateConfigStore) engine() string {
	if v := s.values()[translateConfigEngineKey]; v != "" {
		return v
	}
	return ""
}

// apply fills the engine struct with the stored fields. Query parameters are
// decoded afterwards, so an explicit value in the request always wins.
func (s *translateConfigStore) apply(v any) {
	fields := s.values()
	if len(fields) == 0 {
		return
	}
	delete(fields, translateConfigEngineKey)
	raw, err := json.Marshal(fields)
	if err != nil {
		return
	}
	_ = json.Unmarshal(raw, v)
}

// save merges the given fields into the file: an empty value removes the key,
// a missing key is left untouched. That way the console can edit one engine
// without wiping the credentials of another.
func (s *translateConfigStore) save(engine string, fields map[string]string) error {
	merged := s.values()
	if engine != "" {
		merged[translateConfigEngineKey] = engine
	}
	for k, v := range fields {
		if v = strings.TrimSpace(v); v == "" {
			delete(merged, k)
			continue
		}
		merged[k] = v
	}

	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)

	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err = os.WriteFile(tmp, append(out, '\n'), 0o600); err != nil {
		return err
	}
	if err = os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// Force the next read to pick the new content up even if the file system
	// keeps a coarse modification time.
	s.mu.Lock()
	s.raw, s.mtime, s.size, s.envRaw = nil, time.Time{}, 0, ""
	s.mu.Unlock()
	s.reload()
	return nil
}

// ownTranslateCredentials lists the query parameters that carry engine
// credentials. A request that sets one of them is self-contained.
var ownTranslateCredentials = []string{
	"baidu-app-id", "baidu-app-key", "appid",
	"deepl-api-key", "google-api-key", "openai-api-key",
}

func hasOwnTranslateCredentials(c *gin.Context) bool {
	query := c.Request.URL.Query()
	for _, k := range ownTranslateCredentials {
		if strings.TrimSpace(query.Get(k)) != "" {
			return true
		}
	}
	return false
}

// bearerValid reports whether the request proved it may use the credentials
// kept on the server. A server without a token has nothing to protect.
func bearerValid(c *gin.Context, v auth.Validator) bool {
	if v == nil {
		return true
	}
	bearer, token, found := strings.Cut(c.GetHeader("Authorization"), " ")
	return bearer == "Bearer" && found && v.Valid(token)
}

// translateDecode builds the engine config decoder shared by the translate
// endpoints: stored defaults first, then whatever the request itself carries.
func translateDecode(c *gin.Context, v auth.Validator) func(any) error {
	decoder := schema.NewDecoder()
	decoder.SetAliasTag("json")
	decoder.IgnoreUnknownKeys(true)

	return func(target any) error {
		// Stored credentials are only for callers that proved they may use
		// them, so a credential-less request cannot spend this server's quota.
		if !hasOwnTranslateCredentials(c) && bearerValid(c, v) {
			translateConfig.apply(target)
		}
		return decoder.Decode(target, c.Request.URL.Query())
	}
}

type translateConfigResponse struct {
	Engine string            `json:"engine"`
	Fields map[string]string `json:"fields"`
	Path   string            `json:"path"`
	Ready  bool              `json:"ready"`
}

// isSecretKey marks the fields that must not leave the server in clear. The
// console only needs to know whether a value is stored, so secrets come back
// masked and an empty input means "keep the stored value".
func isSecretKey(k string) bool {
	k = strings.ToLower(k)
	return strings.Contains(k, "key") || strings.Contains(k, "secret") || strings.Contains(k, "token")
}

func maskValue(v string) string {
	r := []rune(v)
	if len(r) <= 4 {
		return "••••"
	}
	return "••••" + string(r[len(r)-4:])
}

func (s *translateConfigStore) response() *translateConfigResponse {
	fields := s.values()
	delete(fields, translateConfigEngineKey)
	for k, v := range fields {
		if isSecretKey(k) {
			fields[k] = maskValue(v)
		}
	}
	engine := s.engine()
	// Ready means "a lookup can be attempted": an engine that needs no
	// credentials is always ready, the others need at least one field.
	ready := len(fields) > 0 || engine == "googlefree"
	return &translateConfigResponse{
		Engine: engine,
		Fields: fields,
		Path:   s.path,
		Ready:  ready,
	}
}

func getTranslateConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, &responseMessage{Data: translateConfig.response()})
	}
}

type translateConfigBody struct {
	Engine string            `json:"engine"`
	Fields map[string]string `json:"fields"`
}

func putTranslateConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body translateConfigBody
		if err := c.ShouldBindJSON(&body); err != nil {
			abortWithStatusMessage(c, http.StatusBadRequest, err)
			return
		}
		if body.Engine == "" && len(body.Fields) == 0 {
			abortWithStatusMessage(c, http.StatusBadRequest, errors.New("nothing to save"))
			return
		}
		if err := translateConfig.save(body.Engine, body.Fields); err != nil {
			abortWithStatusMessage(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, &responseMessage{Data: translateConfig.response()})
	}
}
