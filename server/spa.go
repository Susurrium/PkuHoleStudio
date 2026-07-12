package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	webui "github.com/Susurrium/PkuHoleStudio/web"

	"github.com/gin-gonic/gin"
)

// AttachSPA adds the embedded React client after API routes have been
// registered. API and asset misses stay 404; browser routes fall back to the
// same index document so refreshes work with React Router.
func AttachSPA(engine *gin.Engine) error {
	dist, err := webui.Dist()
	if err != nil {
		return err
	}
	assets, err := fs.Sub(dist, "assets")
	if err != nil {
		return err
	}
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return err
	}
	engine.StaticFS("/assets", http.FS(assets))
	engine.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if c.Request.Method != http.MethodGet || strings.HasPrefix(requestPath, "/api/") || strings.Contains(path.Base(requestPath), ".") {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
	return nil
}
