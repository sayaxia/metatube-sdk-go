package route

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/metatube-community/metatube-sdk-go/route/auth"
	"github.com/metatube-community/metatube-sdk-go/translate"
	_ "github.com/metatube-community/metatube-sdk-go/translate/baidu"
	_ "github.com/metatube-community/metatube-sdk-go/translate/baiduai"
	_ "github.com/metatube-community/metatube-sdk-go/translate/deepl"
	_ "github.com/metatube-community/metatube-sdk-go/translate/google"
	_ "github.com/metatube-community/metatube-sdk-go/translate/googlefree"
	_ "github.com/metatube-community/metatube-sdk-go/translate/openai"
)

type translateQuery struct {
	Q      string `form:"q" binding:"required"`
	From   string `form:"from"`
	To     string `form:"to" binding:"required"`
	Engine string `form:"engine" binding:"required"`
}

type translateResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
	Text string `json:"translated_text"`
}

func getTranslate(v auth.Validator) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := &translateQuery{
			From: "auto",
		}
		if err := c.ShouldBindQuery(query); err != nil {
			abortWithStatusMessage(c, http.StatusBadRequest, err)
			return
		}

		result, err := translate.
			New(query.Engine, translateDecode(c, v)).
			Translate(query.Q, query.From, query.To)
		if err != nil {
			abortWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, &responseMessage{
			Data: &translateResponse{
				From: query.From,
				To:   query.To,
				Text: result,
			},
		})
	}
}

// translateBatchMax bounds one batch request. A console search rarely needs
// more than a title plus a handful of actor names per number.
const (
	translateBatchMax     = 40
	translateBatchTextMax = 2000
)

type translateBatchQuery struct {
	To     string `form:"to" binding:"required"`
	From   string `form:"from"`
	Engine string `form:"engine" binding:"required"`
}

type translateBatchBody struct {
	Texts []string `json:"texts"`
}

type translateBatchItem struct {
	Source string `json:"source"`
	Text   string `json:"translated_text,omitempty"`
	Error  string `json:"error,omitempty"`
}

// postTranslateBatch translates many short strings in one request. The console
// uses it to localise titles and actor names after a search, so a single
// lookup costs one extra request instead of a dozen.
func postTranslateBatch(v auth.Validator) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := &translateBatchQuery{From: "auto"}
		if err := c.ShouldBindQuery(q); err != nil {
			abortWithStatusMessage(c, http.StatusBadRequest, err)
			return
		}
		var body translateBatchBody
		if err := c.ShouldBindJSON(&body); err != nil {
			abortWithStatusMessage(c, http.StatusBadRequest, err)
			return
		}

		// Duplicates are common (the same actor appears in several results) and
		// each one would be billed separately.
		seen := make(map[string]bool, len(body.Texts))
		texts := make([]string, 0, len(body.Texts))
		for _, t := range body.Texts {
			t = strings.TrimSpace(t)
			if t == "" || seen[t] {
				continue
			}
			if r := []rune(t); len(r) > translateBatchTextMax {
				t = string(r[:translateBatchTextMax])
			}
			seen[t] = true
			texts = append(texts, t)
			if len(texts) >= translateBatchMax {
				break
			}
		}

		tr := translate.New(q.Engine, translateDecode(c, v))

		items := make([]translateBatchItem, 0, len(texts))
		for _, t := range texts {
			item := translateBatchItem{Source: t}
			out, err := tr.Translate(t, q.From, q.To)
			if err != nil {
				item.Error = err.Error()
			} else {
				item.Text = out
			}
			items = append(items, item)
		}

		c.JSON(http.StatusOK, &responseMessage{Data: gin.H{"items": items}})
	}
}
