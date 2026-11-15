package route

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestTranslateStore(t *testing.T) *translateConfigStore {
	t.Helper()
	return &translateConfigStore{path: filepath.Join(t.TempDir(), translateConfigName)}
}

func TestTranslateConfigEnvBaseline(t *testing.T) {
	s := newTestTranslateStore(t)
	t.Setenv(translateConfigEnv, `{"engine":"baiduai","baidu-app-id":"env-id","baidu-app-key":"env-key"}`)

	got := s.values()
	if got["engine"] != "baiduai" || got["baidu-app-id"] != "env-id" || got["baidu-app-key"] != "env-key" {
		t.Fatalf("values() = %v, want env baseline", got)
	}
	if s.engine() != "baiduai" {
		t.Fatalf("engine() = %q, want baiduai", s.engine())
	}
	if resp := s.response(); !resp.Ready || resp.Fields["baidu-app-key"] == "env-key" {
		t.Fatalf("response() = %+v, want ready with a masked secret", resp)
	}
}

func TestTranslateConfigFileOverridesEnv(t *testing.T) {
	s := newTestTranslateStore(t)
	t.Setenv(translateConfigEnv, `{"engine":"baiduai","baidu-app-id":"env-id","baidu-app-key":"env-key"}`)
	if err := os.WriteFile(s.path, []byte(`{"engine":"google","baidu-app-key":"file-key"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := s.values()
	if got["engine"] != "google" {
		t.Errorf("engine = %q, want file value google", got["engine"])
	}
	if got["baidu-app-key"] != "file-key" {
		t.Errorf("baidu-app-key = %q, want file value", got["baidu-app-key"])
	}
	if got["baidu-app-id"] != "env-id" {
		t.Errorf("baidu-app-id = %q, want env baseline", got["baidu-app-id"])
	}
}

func TestTranslateConfigEnvChangeReloads(t *testing.T) {
	s := newTestTranslateStore(t)
	t.Setenv(translateConfigEnv, `{"engine":"googlefree"}`)
	if got := s.engine(); got != "googlefree" {
		t.Fatalf("engine() = %q, want googlefree", got)
	}
	// Same file state, environment changed: the cache key must include it.
	t.Setenv(translateConfigEnv, `{"engine":"baiduai"}`)
	if got := s.engine(); got != "baiduai" {
		t.Fatalf("engine() = %q after env change, want baiduai", got)
	}
}

func TestTranslateConfigMalformedEnvIgnored(t *testing.T) {
	s := newTestTranslateStore(t)
	t.Setenv(translateConfigEnv, `not-json`)
	if got := s.values(); len(got) != 0 {
		t.Fatalf("values() = %v, want empty", got)
	}
	if err := os.WriteFile(s.path, []byte(`{"engine":"googlefree"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := s.engine(); got != "googlefree" {
		t.Fatalf("engine() = %q, want file value", got)
	}
}

func TestTranslateConfigSaveStillWorks(t *testing.T) {
	s := newTestTranslateStore(t)
	t.Setenv(translateConfigEnv, `{"engine":"googlefree"}`)
	if err := s.save("baiduai", map[string]string{
		"baidu-app-id": "saved-id", "baidu-app-key": "saved-key",
	}); err != nil {
		t.Fatal(err)
	}
	got := s.values()
	if got["engine"] != "baiduai" || got["baidu-app-id"] != "saved-id" {
		t.Fatalf("values() = %v, want saved values", got)
	}
	// save writes the merged snapshot, so removing the environment still
	// leaves a usable file behind.
	t.Setenv(translateConfigEnv, "")
	if got := s.values(); got["baidu-app-key"] != "saved-key" {
		t.Fatalf("values() after env removed = %v", got)
	}
}
