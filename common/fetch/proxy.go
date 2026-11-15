package fetch

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// proxyCacheTTL bounds how long a resolution result is reused. It is short so
// that toggling the system proxy takes effect without restarting the server,
// and long enough that a burst of provider requests does not read the registry
// on every call.
const proxyCacheTTL = 10 * time.Second

type proxyState struct {
	url     *url.URL
	raw     string
	source  string
	bypass  []string
	checked time.Time
}

var (
	proxyMu    sync.Mutex
	proxyCache proxyState

	changeMu       sync.Mutex
	changeReporter func(string)
)

// SetChangeReporter registers a callback that is invoked whenever the proxy
// decision changes while the process is running. Without it, toggling or
// repointing the system proxy would only show up as providers suddenly failing.
func SetChangeReporter(fn func(string)) {
	changeMu.Lock()
	changeReporter = fn
	changeMu.Unlock()
}

func reportChange(msg string) {
	changeMu.Lock()
	fn := changeReporter
	changeMu.Unlock()
	if fn != nil {
		fn(msg)
	}
}

// Proxy returns the selector installed on every provider transport.
//
// The server only observes the host proxy configuration, it never changes it:
// switching the system proxy on or off stays entirely under the user's control.
//
//   - MT_PROXY_URL          one explicit proxy for every provider (highest);
//   - MT_NO_PROXY=1, or
//     MT_SYSTEM_PROXY=0     ignore the system proxy for this process only;
//   - otherwise             follow the system proxy: the Windows Internet
//     Settings on Windows, the proxy environment variables elsewhere. The
//     environment is ignored on Windows, where it is typically injected per
//     process instead of being a user setting.
//
// The choice is re-evaluated every proxyCacheTTL, so a proxy enabled later in
// the OS is picked up without a restart.
//
// Per-provider settings (MT_<TYPE>_PROVIDER_<NAME>__PROXY) still win over this
// selector, applied by engine.applyProviderConfig.
func Proxy() func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		state := current()
		if state.url == nil {
			return nil, nil
		}
		if req != nil && req.URL != nil && state.bypassed(req.URL.Hostname()) {
			return nil, nil
		}
		return state.url, nil
	}
}

// Current reports the proxy that applies right now, or nil for a direct
// connection.
func Current() *url.URL { return current().url }

// InstallDefaultTransport applies the selector to http.DefaultTransport.
//
// Several providers are built on colly, whose collector leaves its client
// transport nil and therefore falls back to http.DefaultTransport — that one
// honours the proxy environment variables and would ignore the system proxy,
// so the same provider set could end up leaving through two different exits.
// Wiring the selector in here keeps every provider on one decision.
func InstallDefaultTransport() {
	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		transport.Proxy = Proxy()
	}
}

// Describe renders the current proxy decision for the startup log.
func Describe() string { return describeState(current()) }

func describeState(state proxyState) string {
	if state.url == nil {
		if state.source == "" {
			return "direct"
		}
		return state.source
	}
	if len(state.bypass) > 0 {
		return fmt.Sprintf("%s (via %s; %d bypass rule(s))", state.raw, state.source, len(state.bypass))
	}
	return fmt.Sprintf("%s (via %s)", state.raw, state.source)
}

func current() proxyState {
	proxyMu.Lock()
	if !proxyCache.checked.IsZero() && time.Since(proxyCache.checked) < proxyCacheTTL {
		state := proxyCache
		proxyMu.Unlock()
		return state
	}
	prev := proxyCache
	next := resolve()
	proxyCache = next
	proxyMu.Unlock()

	// The first resolution is already covered by the startup log; only later
	// transitions are worth reporting.
	if !prev.checked.IsZero() && describeState(prev) != describeState(next) {
		reportChange("Outbound proxy changed: " + describeState(prev) + " -> " + describeState(next))
	}
	return next
}

func resolve() proxyState {
	state := proxyState{checked: time.Now()}

	if raw := strings.TrimSpace(os.Getenv("MT_PROXY_URL")); raw != "" {
		u, canonical := parseProxyURL(raw)
		if u == nil {
			state.source = "direct (invalid MT_PROXY_URL: " + raw + ")"
			return state
		}
		state.url, state.raw, state.source = u, canonical, "MT_PROXY_URL"
		return state
	}

	if systemProxyDisabled() {
		state.source = "direct (system proxy ignored: MT_SYSTEM_PROXY=0)"
		return state
	}

	raw, source := platformProxy()
	if raw == "" {
		state.source = source
		if state.source == "" {
			state.source = "direct (no system proxy)"
		}
		return state
	}
	u, canonical := parseProxyURL(raw)
	if u == nil {
		state.source = "direct (unusable system proxy: " + raw + ")"
		return state
	}
	state.url, state.raw, state.source = u, canonical, source
	state.bypass = platformBypass()
	return state
}

// systemProxyDisabled reports whether this process was explicitly told to skip
// the system proxy. Only affects this process; the OS setting is untouched.
func systemProxyDisabled() bool {
	if truthy(os.Getenv("MT_NO_PROXY")) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MT_SYSTEM_PROXY"))) {
	case "0", "false", "no", "off":
		return true
	}
	return false
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func (s proxyState) bypassed(host string) bool {
	if len(s.bypass) == 0 {
		return false
	}
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "" {
		return false
	}
	for _, rule := range s.bypass {
		if matchBypass(rule, h) {
			return true
		}
	}
	return false
}

// matchBypass implements the subset of the Windows ProxyOverride / NO_PROXY
// syntax that matters here: "<local>", exact hosts, domain suffixes and
// trailing wildcards such as "192.168.1.*".
func matchBypass(rule, host string) bool {
	rule = strings.ToLower(strings.TrimSpace(rule))
	if rule == "" {
		return false
	}
	if rule == "<local>" {
		// Same meaning as Windows: names without a dot are intranet hosts.
		return !strings.Contains(host, ".")
	}
	if strings.Contains(rule, "*") && rule == "*" {
		return true
	}
	// A rule may carry a port, which is irrelevant for the host match.
	if h, _, err := net.SplitHostPort(rule); err == nil {
		rule = h
	}
	rule = strings.TrimPrefix(strings.TrimPrefix(rule, "*"), ".")

	if idx := strings.Index(rule, "*"); idx >= 0 {
		return strings.HasPrefix(host, strings.TrimSuffix(rule[:idx], "."))
	}
	if rule == "" {
		return false
	}
	return host == rule || strings.HasSuffix(host, "."+rule)
}

// envProxy reads the conventional proxy environment variables.
func envProxy() (string, string) {
	for _, name := range []string{
		"HTTPS_PROXY", "https_proxy",
		"HTTP_PROXY", "http_proxy",
		"ALL_PROXY", "all_proxy",
	} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v, "environment variable " + name
		}
	}
	return "", ""
}

// envBypass reads NO_PROXY / no_proxy.
func envBypass() []string {
	raw := strings.TrimSpace(os.Getenv("NO_PROXY"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("no_proxy"))
	}
	if raw == "" {
		return nil
	}
	return splitBypass(raw, ",")
}

func splitBypass(raw, sep string) []string {
	parts := strings.Split(raw, sep)
	rules := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			rules = append(rules, p)
		}
	}
	return rules
}

// parseProxyURL normalises "host:port" to "http://host:port" and validates it.
func parseProxyURL(raw string) (*url.URL, string) {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, ""
	}
	return u, u.String()
}
