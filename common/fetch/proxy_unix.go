//go:build !windows

package fetch

import "os"

// platformProxy follows the conventional proxy environment variables on hosts
// without a system-wide proxy store.
func platformProxy() string {
	for _, name := range []string{
		"HTTPS_PROXY", "https_proxy",
		"HTTP_PROXY", "http_proxy",
		"ALL_PROXY", "all_proxy",
	} {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}
