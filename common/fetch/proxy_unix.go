//go:build !windows

package fetch

// platformProxy follows the conventional proxy environment variables on hosts
// without a system-wide proxy store.
func platformProxy() (string, string) {
	if raw, src := envProxy(); raw != "" {
		return raw, src
	}
	return "", "direct (no proxy environment variable)"
}

// platformBypass follows NO_PROXY.
func platformBypass() []string { return envBypass() }
