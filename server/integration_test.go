package server

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	aipkg "github.com/Susurrium/PkuHoleStudio/internal/ai"
	"github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"

	"github.com/gin-gonic/gin"
)

func setupTestEnv(t *testing.T) (*db.Database, *gin.Engine, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	tmpFile.Close()

	cfg := &config.Config{
		Username:  "test",
		Password:  "test",
		SecretKey: "test",
		Database: config.DatabaseConfig{
			Type:   "sqlite3",
			DBFile: tmpFile.Name(),
		},
		Cors: config.CorsConfig{
			AllowOrigins: []string{"http://localhost:3000"},
			AllowMethods: []string{"GET", "POST", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
		},
	}

	database, err := db.NewDatabase(cfg)
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("NewDatabase: %v", err)
	}

	config.Conf = cfg

	gin.SetMode(gin.TestMode)
	r := gin.New()
	posts := service.NewPostService(database, nil)
	manager, err := jobs.NewManager(context.Background(), database)
	if err != nil {
		database.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("NewManager: %v", err)
	}
	dataDir := t.TempDir()
	search := service.NewSearchService(posts, database)
	aiService := aipkg.NewService(context.Background(), database, posts, search, nil, cfg.AI, aipkg.ProviderInfo{Name: "DeepSeek", BaseURL: "https://api.deepseek.com", Model: "deepseek-chat", Configured: false})
	Init(r, Dependencies{
		Posts:      posts,
		Search:     search,
		Library:    service.NewLocalLibraryService(database),
		Media:      service.NewMediaServiceWithRepository(dataDir, nil, database),
		Archive:    archive.NewImporterWithDataDir(database, dataDir),
		AI:         aiService,
		Auth:       authStub{},
		Jobs:       manager,
		Repository: database,
		DataDir:    dataDir,
	})

	cleanup := func() {
		manager.Close()
		database.Close()
		os.Remove(tmpFile.Name())
	}

	return database, r, cleanup
}

type authStub struct{}

func (authStub) CachedStatus(context.Context) service.AuthStatus {
	return service.AuthStatus{HasSession: true, Message: "尚未检测在线登录状态"}
}
func (authStub) Probe(context.Context) service.AuthStatus {
	return service.AuthStatus{Checked: true, HasSession: true, CanReadOnline: true}
}
func (authStub) Login(context.Context, string, string) service.AuthStatus {
	return service.AuthStatus{Checked: true, HasSession: true, CanReadOnline: true}
}
func (authStub) SendSMS(context.Context, string) service.AuthStatus {
	return service.AuthStatus{Checked: true, Challenge: "sms", ChallengeStage: "iaaa"}
}
func (authStub) Continue(context.Context, string, string, string, string, string) service.AuthStatus {
	return service.AuthStatus{Checked: true, HasSession: true, CanReadOnline: true}
}
func (authStub) Logout(context.Context) error { return nil }

func TestRouterRegistration(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	routes := r.Routes()
	expectedPaths := map[string]bool{
		"/health":                   false,
		"/help":                     false,
		"/posts":                    false,
		"/post/:pid":                false,
		"/comment":                  false,
		"/comments/:pid":            false,
		"/api/v1/session":           false,
		"/api/v1/session/probe":     false,
		"/api/v1/session/login":     false,
		"/api/v1/session/sms":       false,
		"/api/v1/session/challenge": false,
		"/media/image":              false,
		"/api/v1/health":            false,
		"/api/v1/posts":             false,
		"/api/v1/search":            false,
		"/api/v1/jobs":              false,
		"/api/v1/imports":           false,
		"/api/v1/exports":           false,
		"/api/v1/ai/sessions":       false,
	}

	for _, route := range routes {
		if _, ok := expectedPaths[route.Path]; ok {
			expectedPaths[route.Path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("Route %s not registered", path)
		}
	}
}

func TestHealthEndpoint(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Status = %d, want 200", w.Code)
	}
}

func TestPostsEndpoint(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Status = %d, want 200", w.Code)
	}
}

func TestPostEndpointInvalidPid(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/post/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestCommentsEndpointInvalidPid(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/comments/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestCommentEndpointInvalidCid(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/comment?cid=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestHelpEndpoint(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/help", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Status = %d, want 200", w.Code)
	}
}

func TestMediaImageEndpointNoParam(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/media/image", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestCORSHeaders(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("OPTIONS Status = %d, want 204", w.Code)
	}

	ao := w.Header().Get("Access-Control-Allow-Origin")
	if ao != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %s, want http://localhost:3000", ao)
	}
}

func TestCORSAllowMethods(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("OPTIONS", "/posts", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	methods := w.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("Access-Control-Allow-Methods header is missing")
	}
}

func TestEndToEndFlow(t *testing.T) {
	database, r, cleanup := setupTestEnv(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 1, Text: "End to end test post", Type: "text", Timestamp: 1000},
		{Pid: 2, Text: "Another post for testing", Type: "text", Timestamp: 2000},
	}
	if err := database.UpsertPosts(posts); err != nil {
		t.Fatalf("UpsertPosts: %v", err)
	}

	req := httptest.NewRequest("GET", "/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /posts Status = %d, want 200", w.Code)
	}

	req = httptest.NewRequest("GET", "/post/1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /post/1 Status = %d, want 200", w.Code)
	}

	req = httptest.NewRequest("GET", "/posts?keyword=end", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /posts?keyword=end Status = %d, want 200", w.Code)
	}
}
