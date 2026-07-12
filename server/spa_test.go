package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAttachSPAFallsBackForBrowserRoutesOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	if err := AttachSPA(router); err != nil {
		t.Fatalf("AttachSPA() error = %v", err)
	}

	for _, route := range []string{"/", "/posts", "/posts/123456", "/search?q=test"} {
		request := httptest.NewRequest(http.MethodGet, route, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "PkuHoleStudio") {
			t.Errorf("GET %s = %d %q", route, response.Code, response.Body.String())
		}
	}
	for _, route := range []string{"/api/v1/missing", "/missing.js"} {
		request := httptest.NewRequest(http.MethodGet, route, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", route, response.Code)
		}
	}
}
