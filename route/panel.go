package route

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/metatube-community/metatube-sdk-go/common/fetch"
	"github.com/metatube-community/metatube-sdk-go/engine"
	V "github.com/metatube-community/metatube-sdk-go/internal/version"
	"github.com/metatube-community/metatube-sdk-go/route/ui"
)

// startedAt marks process start, used for the uptime shown on the console.
var startedAt = time.Now()

// getPanel serves the embedded console page. It is deliberately not cached:
// after an upgrade the browser should pick the new page up on reload.
func getPanel() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", ui.Index)
	}
}

// getFavicon answers the browser's automatic icon request. The console page
// carries its icon inline, but a bare /favicon.ico hit would otherwise land in
// the request log as a 404 on every page load.
func getFavicon() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.Status(http.StatusNoContent)
	}
}

// getSystemStatus reports the runtime facts the console shows on its status
// page. The outbound proxy decision is included because every provider depends
// on it, and outside the console it is only visible in the startup log.
func getSystemStatus(app *engine.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		proxy := fetch.Current()
		c.JSON(http.StatusOK, &responseMessage{Data: gin.H{
			"app":             app.String(),
			"version":         V.BuildString(),
			"uptime_seconds":  int64(time.Since(startedAt).Seconds()),
			"db_driver":       app.DBDriver(),
			"go_version":      runtime.Version(),
			"goos":            runtime.GOOS,
			"goarch":          runtime.GOARCH,
			"num_cpu":         runtime.NumCPU(),
			"proxy":           fetch.Describe(),
			"proxy_active":    proxy != nil,
			"movie_providers": len(app.GetMovieProviders()),
			"actor_providers": len(app.GetActorProviders()),
			"modules":         len(V.Modules()),
		}})
	}
}
