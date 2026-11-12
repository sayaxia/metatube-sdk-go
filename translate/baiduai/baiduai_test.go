package baiduai

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/metatube-community/metatube-sdk-go/translate"
)

func TestBaiduAITranslate(t *testing.T) {
	appID, appKey := os.Getenv("BAIDU_APP_ID"), os.Getenv("BAIDU_APP_KEY")
	if appID == "" || appKey == "" {
		t.Skip("BAIDU_APP_ID / BAIDU_APP_KEY not set")
	}
	for _, unit := range []struct {
		text, from, to string
	}{
		{`Oh yeah! I'm a translator!`, "auto", "zh-CN"},
		{`Oh yeah! I'm a translator!`, "auto", "zh-TW"},
		{`Oh yeah! I'm a translator!`, "auto", "ja"},
		{`Oh yeah! I'm a translator!`, "auto", "de"},
		{`Oh yeah! I'm a translator!`, "auto", "fr"},
	} {
		result, err := (&BaiduAI{
			AppID:  appID,
			AppKey: appKey,
		}).Translate(unit.text, unit.from, unit.to)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(result)
	}
}

func TestModelType(t *testing.T) {
	for _, unit := range []struct {
		in, want string
	}{
		{"", defaultModelType},
		{"  ", defaultModelType},
		{"LLM", "llm"},
		{" nmt ", "nmt"},
		{"nmt", "nmt"},
	} {
		if got := modelType(unit.in); got != unit.want {
			t.Errorf("modelType(%q) = %q, want %q", unit.in, got, unit.want)
		}
	}
}

func TestParseToSupportedLanguage(t *testing.T) {
	for _, unit := range []struct {
		in, want string
	}{
		{"", "auto"},
		{"auto", "auto"},
		{"zh", "zh"},
		{"zh-cn", "zh"},
		{"zh_CN", "zh"},
		{"zh-hans", "zh"},
		{"zh-tw", "cht"},
		{"zh-HK", "cht"},
		{"ja", "jp"},
		{"jp", "jp"},
		{"ko", "kor"},
		{"vi", "vie"},
		{"es", "spa"},
		{"fr", "fra"},
		{"de", "de"},
		{"en", "en"},
	} {
		if got := parseToSupportedLanguage(unit.in); got != unit.want {
			t.Errorf("parseToSupportedLanguage(%q) = %q, want %q", unit.in, got, unit.want)
		}
	}
}

func TestRegister(t *testing.T) {
	// The translator must be registered under a name matching
	// "baiduai" (case-insensitive) so the engine can be selected
	// via the /v1/translate?engine=baiduai query parameter.
	decode := func(v any) error {
		b, _ := json.Marshal(map[string]string{
			"baidu-app-id":  "appid",
			"baidu-app-key": "secret",
		})
		return json.Unmarshal(b, v)
	}
	tt, ok := translate.New("baiduai", decode).(*BaiduAI)
	if !ok {
		t.Fatalf("baiduai translator not registered, got %T", tt)
	}
	if tt.AppID != "appid" || tt.AppKey != "secret" {
		t.Errorf("decode mismatch: %+v", tt)
	}
}
