package server

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/app"
	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type updatingRemote struct{ writeRemoteStub }

func (r *updatingRemote) RefreshPost(_ context.Context, pid int32) (*models.Post, error) {
	return &models.Post{Pid: pid, Text: "online content", Reply: 1, Likenum: 2, IsFollow: true}, nil
}
func (r *updatingRemote) ListComments(_ context.Context, pid int32, _ service.RemoteCommentQuery) ([]models.Comment, int, error) {
	return []models.Comment{{Pid: pid, Cid: 1, Text: "online comment"}}, 1, nil
}

func TestPostUpdatesAPIThroughApplicationAndDatabase(t *testing.T) {
	dataDir := t.TempDir()
	config.SetRuntimeDataDir(dataDir)
	t.Cleanup(func() { config.SetRuntimeDataDir("") })
	c, err := client.NewClient("test-device")
	if err != nil {
		t.Fatal(err)
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "test-account"}).SignedString([]byte("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	c.SetPkuToken(token)
	if err := c.SaveCookies(); err != nil {
		t.Fatal(err)
	}
	if err := c.ReloadSessionCookies(); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	a, err := app.Open(context.Background(), app.Options{Config: &cfg, Client: c, Remote: &updatingRemote{}, DataDir: dataDir, DatabasePath: filepath.Join(dataDir, "updates.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.Repository.SaveCrawlResultWithSource([]models.Post{{Pid: 12345, Text: "old content", Likenum: 50}}, nil, "explicit", "test"); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	Init(router, Dependencies{Posts: a.Posts, PostUpdates: a.PostUpdates, Repository: a.Repository, Jobs: a.Jobs, Media: a.Media, DataDir: dataDir})
	response := performRequest(router, http.MethodPost, "/api/v1/posts/12345/refresh", strings.NewReader(`{"full":false}`), "application/json")
	if response.Code != 200 {
		t.Fatalf("refresh = %d %s", response.Code, response.Body.String())
	}
	post, err := a.Repository.GetPostByPid(12345)
	if err != nil || post.Text != "online content" || post.Likenum != 2 {
		t.Fatalf("persisted post=%+v %v", post, err)
	}
	response = performRequest(router, http.MethodPost, "/api/v1/posts/12345/refresh", strings.NewReader(`{"full":true}`), "application/json")
	if response.Code != 202 {
		t.Fatalf("full refresh = %d %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data publicJob `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		job, err := a.Jobs.Get(context.Background(), envelope.Data.ID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status.Terminal() {
			if job.Status != "completed" {
				t.Fatalf("update job=%+v", job)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("update job did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	response = performRequest(router, http.MethodPut, "/api/v1/post-updates", strings.NewReader(`{"enabled":true,"interval_minutes":30,"pids":[12345]}`), "application/json")
	if response.Code != 200 {
		t.Fatalf("configure=%d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodPut, "/api/v1/post-updates", strings.NewReader(`{"enabled":true,"interval_minutes":5,"pids":[12345]}`), "application/json")
	if response.Code != 400 {
		t.Fatalf("short interval accepted: %d", response.Code)
	}
	response = performRequest(router, http.MethodPost, "/api/v1/posts/54321/refresh", strings.NewReader(`{"full":false}`), "application/json")
	if response.Code != 502 {
		t.Fatalf("unsaved refresh accepted: %d", response.Code)
	}
	visible, err := a.Repository.GetVisiblePostIDs([]int32{54321})
	if err != nil || visible[54321] {
		t.Fatal("unsaved post entered library")
	}
}

func TestPIDUpdateQueueCoalescesOnlyMatchingActiveScopes(t *testing.T) {
	repo, _, cleanup := setupTestEnv(t)
	defer cleanup()
	manager, err := jobs.NewManager(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	deps := Dependencies{Jobs: manager}
	request := map[string]any{"account": "a", "pids": []int32{12345}, "include_comments": true, "include_media": false}
	first, err := createOrReusePIDJob(context.Background(), deps, request, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := createOrReusePIDJob(context.Background(), deps, request, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("matching active update was duplicated")
	}
	request["account"] = "b"
	other, err := createOrReusePIDJob(context.Background(), deps, request, 1)
	if err != nil {
		t.Fatal(err)
	}
	if other.ID == first.ID {
		t.Fatal("different accounts shared an update")
	}
	request["account"] = "a"
	request["include_media"] = true
	complete, err := createOrReusePIDJob(context.Background(), deps, request, 1)
	if err != nil {
		t.Fatal(err)
	}
	if complete.ID == first.ID {
		t.Fatal("update without media satisfied a media request")
	}
}
