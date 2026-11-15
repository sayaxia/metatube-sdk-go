package fetch

import (
	"strings"
	"testing"
	"time"
)

// resetProxyCache drops any cached decision and restores the reporter, so each
// test starts from "nothing resolved yet".
func resetProxyCache(t *testing.T) {
	t.Helper()
	proxyMu.Lock()
	proxyCache = proxyState{}
	proxyMu.Unlock()
	t.Cleanup(func() {
		proxyMu.Lock()
		proxyCache = proxyState{}
		proxyMu.Unlock()
		SetChangeReporter(nil)
	})
}

// expireProxyCache makes the cached decision stale without waiting for the TTL,
// while keeping its timestamp non-zero so it still counts as a previous state.
func expireProxyCache() {
	proxyMu.Lock()
	proxyCache.checked = time.Now().Add(-time.Hour)
	proxyMu.Unlock()
}

func TestProxyReportsChange(t *testing.T) {
	resetProxyCache(t)
	t.Setenv("MT_PROXY_URL", "http://127.0.0.1:1111")

	var events []string
	SetChangeReporter(func(msg string) { events = append(events, msg) })

	// The first resolution is reported by the startup log, not the reporter.
	if got := Current(); got == nil || got.Port() != "1111" {
		t.Fatalf("first resolution: got %v, want port 1111", got)
	}
	if len(events) != 0 {
		t.Fatalf("first resolution should not report, got %v", events)
	}

	// Repointing the proxy while running must be reported once.
	t.Setenv("MT_PROXY_URL", "http://127.0.0.1:2222")
	expireProxyCache()
	if got := Current(); got == nil || got.Port() != "2222" {
		t.Fatalf("after change: got %v, want port 2222", got)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 change event, got %d: %v", len(events), events)
	}
	if !strings.Contains(events[0], "->") {
		t.Fatalf("change event should show the transition, got %q", events[0])
	}
	t.Logf("reported: %s", events[0])

	// A re-resolution with an identical decision must stay silent.
	expireProxyCache()
	Current()
	if len(events) != 1 {
		t.Fatalf("unchanged decision should not report, got %v", events)
	}
}

func TestProxyFallsBackToDirectOnInvalidURL(t *testing.T) {
	resetProxyCache(t)
	t.Setenv("MT_PROXY_URL", "://not a proxy")

	if got := Current(); got != nil {
		t.Fatalf("invalid MT_PROXY_URL should resolve to direct, got %v", got)
	}
	if msg := Describe(); !strings.Contains(msg, "invalid MT_PROXY_URL") {
		t.Fatalf("Describe should name the problem, got %q", msg)
	}
}

func TestBypassMatching(t *testing.T) {
	for _, tc := range []struct {
		rule, host string
		want       bool
	}{
		{"<local>", "nas", true},
		{"<local>", "nas.lan", false},
		{".example.com", "a.example.com", true},
		{"example.com", "example.com", true},
		{"192.168.*", "192.168.1.10", true},
		{"192.168.*", "10.0.0.1", false},
		{"*", "anything", true},
	} {
		if got := matchBypass(tc.rule, tc.host); got != tc.want {
			t.Errorf("matchBypass(%q, %q) = %v, want %v", tc.rule, tc.host, got, tc.want)
		}
	}
}
