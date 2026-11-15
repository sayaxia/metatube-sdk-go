//go:build windows

package fetch

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// platformProxy reads — never writes — the proxy stored by the Windows proxy
// settings, so the server follows whatever the user selected there. The
// registry is opened read-only; toggling the system proxy stays the user's
// business.
//
// The proxy environment variables are deliberately NOT consulted here: on
// Windows they are usually injected per process rather than configured by the
// user, and following them can silently route providers through a proxy whose
// egress region they block. Use MT_PROXY_URL to pin a specific proxy instead.
func platformProxy() (string, string) {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return "", "direct (Windows Internet Settings unreadable)"
	}
	defer key.Close()

	enabled, _, enableErr := key.GetIntegerValue("ProxyEnable")
	raw, _, serverErr := key.GetStringValue("ProxyServer")
	if enableErr == nil && enabled != 0 && serverErr == nil && raw != "" {
		if v := pickProxyServer(raw); v != "" {
			return v, "Windows Internet Settings"
		}
	}

	// A PAC script cannot be evaluated without a JavaScript engine, so say so
	// instead of silently looking like "direct".
	if pac, _, err := key.GetStringValue("AutoConfigURL"); err == nil && strings.TrimSpace(pac) != "" {
		return "", "direct (PAC script configured but unsupported: " + pac + ")"
	}

	return "", "direct (system proxy off)"
}

// platformBypass reads the Windows bypass list (ProxyOverride), which contains
// the intranet ranges that must never be proxied.
func platformBypass() []string {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()

	raw, _, err := key.GetStringValue("ProxyOverride")
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	return splitBypass(raw, ";")
}

// pickProxyServer accepts either "host:port" or a per-scheme list such as
// "http=host:port;https=host:port;socks=host:port".
func pickProxyServer(raw string) string {
	if !strings.Contains(raw, "=") {
		return raw
	}
	values := make(map[string]string)
	for _, part := range strings.Split(raw, ";") {
		name, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		values[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
	}
	for _, scheme := range []string{"https", "http", "socks"} {
		if v := values[scheme]; v != "" {
			if scheme == "socks" {
				return "socks5://" + v
			}
			return v
		}
	}
	return ""
}
