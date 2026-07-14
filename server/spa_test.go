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

	for _, route := range []string{
		"/", "/posts", "/posts/123456", "/search?q=test", "/sync", "/imports",
		"/ai", "/settings", "/notifications", "/logs", "/campus", "/unknown-browser-route",
	} {
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
		if route == "/api/v1/missing" && strings.Contains(response.Header().Get("Content-Type"), "text/html") {
			t.Errorf("GET %s returned the SPA document", route)
		}
	}
}

func TestAttachSPAPreservesLegacyPostsAPIWhileRefreshingReactPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err := AttachSPA(router); err != nil {
		t.Fatal(err)
	}
	router.GET("/posts", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"legacy": true}) })

	browser := httptest.NewRequest(http.MethodGet, "/posts?sort=desc&tag=1", nil)
	browser.Header.Set("Accept", "text/html,application/xhtml+xml")
	browserResponse := httptest.NewRecorder()
	router.ServeHTTP(browserResponse, browser)
	if browserResponse.Code != http.StatusOK || !strings.Contains(browserResponse.Header().Get("Content-Type"), "text/html") || !strings.Contains(browserResponse.Body.String(), "PkuHoleStudio") {
		t.Fatalf("browser refresh = %d %v %q", browserResponse.Code, browserResponse.Header(), browserResponse.Body.String())
	}

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/posts", nil),
		httptest.NewRequest(http.MethodGet, "/posts?begin=0&limit=25", nil),
	} {
		request.Header.Set("Accept", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"legacy":true`) {
			t.Fatalf("legacy request %s = %d %q", request.URL, response.Code, response.Body.String())
		}
	}

	legacyBrowser := httptest.NewRequest(http.MethodGet, "/posts?begin=0&limit=25", nil)
	legacyBrowser.Header.Set("Accept", "text/html")
	legacyResponse := httptest.NewRecorder()
	router.ServeHTTP(legacyResponse, legacyBrowser)
	if !strings.Contains(legacyResponse.Body.String(), `"legacy":true`) {
		t.Fatalf("legacy browser query was intercepted: %q", legacyResponse.Body.String())
	}
}
