package fetch

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

var (
	proxyOnce sync.Once
	proxyFunc func(*http.Request) (*url.URL, error)
	proxyRaw  string
)

// Proxy returns the outbound proxy selector shared by every provider request.
//
// Resolution order:
//  1. MT_PROXY_URL — one explicit URL applied to every provider;
//  2. the operating system proxy, but only when MT_SYSTEM_PROXY is enabled
//     (Windows Internet Settings on Windows, the proxy environment variables
//     elsewhere);
//  3. nil, meaning a direct connection.
//
// Per-provider settings (MT_<TYPE>_PROVIDER_<NAME>__PROXY) are applied later
// by engine.applyProviderConfig and still take priority over this selector.
func Proxy() func(*http.Request) (*url.URL, error) {
	proxyOnce.Do(func() {
		if raw := strings.TrimSpace(os.Getenv("MT_PROXY_URL")); raw != "" {
			proxyFunc, proxyRaw = fixedProxy(raw)
			return
		}
		if systemProxyEnabled() {
			if raw := platformProxy(); raw != "" {
				proxyFunc, proxyRaw = fixedProxy(raw)
			}
		}
	})
	return proxyFunc
}

// ProxyURL reports the proxy currently in use, or an empty string for direct.
func ProxyURL() string { return proxyRaw }

func systemProxyEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MT_SYSTEM_PROXY"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// fixedProxy builds a selector that routes every request through raw.
func fixedProxy(raw string) (func(*http.Request) (*url.URL, error), string) {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, ""
	}
	return func(*http.Request) (*url.URL, error) { return u, nil }, u.String()
}
