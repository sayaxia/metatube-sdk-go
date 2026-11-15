//go:build windows

package fetch

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// platformProxy reads the proxy stored by the Windows proxy settings UI, so the
// server follows the system configuration instead of needing per-provider
// environment variables.
func platformProxy() string {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	if enabled, _, err := key.GetIntegerValue("ProxyEnable"); err != nil || enabled == 0 {
		return ""
	}

	raw, _, err := key.GetStringValue("ProxyServer")
	if err != nil || raw == "" {
		return ""
	}
	// ProxyServer is either "host:port" or a per-scheme list such as
	// "http=host:port;https=host:port;socks=host:port".
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
