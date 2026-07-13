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
	// /posts is also the preserved PKUHoleTUI REST endpoint. A browser document
	// refresh should receive React, while API clients and explicit legacy query
	// parameters must keep receiving the old JSON contract. AttachSPA is called
	// before API routes are registered so this middleware is part of that route.
	engine.Use(func(c *gin.Context) {
		if isSPAPostsDocument(c.Request) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", index)
			c.Abort()
			return
		}
		c.Next()
	})
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

func isSPAPostsDocument(request *http.Request) bool {
	if request == nil || request.Method != http.MethodGet || request.URL.Path != "/posts" || !strings.Contains(strings.ToLower(request.Header.Get("Accept")), "text/html") {
		return false
	}
	query := request.URL.Query()
	for _, legacy := range []string{"begin", "limit", "keyword", "order_by"} {
		if query.Has(legacy) {
			return false
		}
	}
	return true
}
