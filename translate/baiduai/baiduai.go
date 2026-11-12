// Package baiduai provides a translator that is powered by
// Baidu Translate AIT (large-model) text translation API.
//
// API doc: https://api.fanyi.baidu.com/doc/21
// Endpoint: https://fanyi-api.baidu.com/ait/api/aiTextTranslate
//
// The AIT endpoint is a separate product from the classic MT
// endpoint (api.fanyi.baidu.com/api/trans/vip/translate), with its
// own free character quota, while sharing the same APPID/secret-key
// sign-based authentication (MD5(appid+q+salt+key)) and the same
// JSON response schema (trans_result[].dst) as the classic API.
//
// model_type is set to "nmt" (machine translation) by default, which
// grants personal-certified accounts 1M free characters per month;
// the "llm" mode only carries a one-time 1M-character test quota.
package baiduai

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/metatube-community/metatube-sdk-go/common/fetch"
	"github.com/metatube-community/metatube-sdk-go/translate"
)

var _ translate.Translator = (*BaiduAI)(nil)

// baiduAITranslateAPI is the AIT (large model) text translation endpoint.
const baiduAITranslateAPI = "https://fanyi-api.baidu.com/ait/api/aiTextTranslate"

// defaultModelType defaults to the nmt model which benefits from
// a monthly free character quota for personal-certified accounts.
const defaultModelType = "nmt"

type BaiduAI struct {
	AppID  string `json:"baidu-app-id"`
	AppKey string `json:"baidu-app-key"`
	// ModelType is optional: "nmt" (default) or "llm".
	ModelType string `json:"baidu-model-type"`
}

func (bd *BaiduAI) Translate(text, from, to string) (result string, err error) {
	var (
		resp *http.Response
		// salt & sign
		salt = strconv.Itoa(rand.Intn(0x7FFFFFFF))
		sign = md5sum(bd.AppID + text + salt + bd.AppKey)
	)
	if resp, err = fetch.Post(
		baiduAITranslateAPI,
		fetch.WithURLEncodedBody(map[string]string{
			"q":          text,
			"from":       parseToSupportedLanguage(from),
			"to":         parseToSupportedLanguage(to),
			"appid":      bd.AppID,
			"salt":       salt,
			"sign":       sign,
			"model_type": modelType(bd.ModelType),
		}),
		fetch.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
	); err != nil {
		return
	}
	defer resp.Body.Close()

	data := struct {
		From        string `json:"from"`
		To          string `json:"to"`
		TransResult []struct {
			Src string `json:"src"`
			Dst string `json:"dst"`
		} `json:"trans_result"`
		ErrorCode string `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&data); err == nil {
		if len(data.TransResult) > 0 {
			s := strings.Builder{}
			for _, t := range data.TransResult {
				s.WriteString(t.Dst)
				s.WriteByte('\n')
			}
			result = strings.TrimSpace(s.String())
		} else {
			err = fmt.Errorf("%s: %s", data.ErrorCode, data.ErrorMsg)
		}
	}
	return
}

func modelType(v string) string {
	if v = strings.TrimSpace(strings.ToLower(v)); v != "" {
		return v
	}
	return defaultModelType
}

func md5sum(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// parseToSupportedLanguage maps common language aliases to the
// language codes shared by both Baidu MT and AIT endpoints.
func parseToSupportedLanguage(lang string) string {
	if lang = strings.ToLower(lang); lang == "" || lang == "auto" /* auto detect */ {
		return "auto"
	}
	switch lang {
	case "zh", "chs", "zh-cn", "zh_cn", "zh-hans":
		return "zh"
	case "cht", "zh-tw", "zh_tw", "zh-hk", "zh_hk", "zh-hant":
		return "cht"
	case "jp", "ja":
		return "jp"
	case "kor", "ko":
		return "kor"
	case "vie", "vi":
		return "vie"
	case "spa", "es":
		return "spa"
	case "fra", "fr":
		return "fra"
	case "ara", "ar":
		return "ara"
	case "bul", "bg":
		return "bul"
	case "est", "et":
		return "est"
	case "dan", "da":
		return "dan"
	case "fin", "fi":
		return "fin"
	case "rom", "ro":
		return "rom"
	case "slo", "sl":
		return "slo"
	case "swe", "sv":
		return "swe"
	}
	return lang
}

func init() {
	translate.Register(&BaiduAI{})
}
