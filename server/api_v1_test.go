package server

import (
	stdzip "archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
	"github.com/gin-gonic/gin"
)

type writeRemoteStub struct {
	postRequest    service.CreatePostRequest
	commentRequest service.CreateCommentRequest
	uploaded       bool
	uploadedPath   string
	postErr        error
}

func (s *writeRemoteStub) ListPosts(context.Context, service.RemoteListQuery) ([]models.Post, int, error) {
	return nil, 0, nil
}
func (s *writeRemoteStub) GetPost(context.Context, int32) (*models.Post, error) {
	return &models.Post{Pid: 123456}, nil
}
func (s *writeRemoteStub) ListComments(context.Context, int32, service.RemoteCommentQuery) ([]models.Comment, int, error) {
	return nil, 0, nil
}
func (s *writeRemoteStub) ListTags(context.Context) ([]models.Tag, error) { return nil, nil }
func (s *writeRemoteStub) GetCourseTable(context.Context) ([]models.CourseScheduleRow, error) {
	return nil, nil
}
func (s *writeRemoteStub) GetCourseScores(context.Context) (*models.ScoreSummary, error) {
	return nil, nil
}
func (s *writeRemoteStub) RefreshPost(context.Context, int32) (*models.Post, error) {
	return &models.Post{Pid: 123456}, nil
}
func (s *writeRemoteStub) TogglePraise(context.Context, int32) error    { return nil }
func (s *writeRemoteStub) ToggleAttention(context.Context, int32) error { return nil }
func (s *writeRemoteStub) UploadImage(_ context.Context, path string) (string, error) {
	_, err := os.Stat(path)
	s.uploaded = err == nil
	s.uploadedPath = path
	return "77", err
}
func (s *writeRemoteStub) CreatePost(_ context.Context, request service.CreatePostRequest) (*models.Post, error) {
	s.postRequest = request
	if s.postErr != nil {
		return nil, s.postErr
	}
	return &models.Post{Pid: 123456, Text: request.Text}, nil
}
func (s *writeRemoteStub) CreateComment(_ context.Context, request service.CreateCommentRequest) (*models.Comment, error) {
	s.commentRequest = request
	return &models.Comment{Cid: 7, Pid: request.PID, Text: request.Text}, nil
}
func (s *writeRemoteStub) CanWrite(context.Context) (bool, error) { return true, nil }

func TestAPIV1PostsSearchAndErrorShape(t *testing.T) {
	database, router, cleanup := setupTestEnv(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{
		{Pid: 12345, Text: "alpha beta", Timestamp: 100},
		{Pid: 23456, Text: "alpha only", Timestamp: 200},
	}); err != nil {
		t.Fatal(err)
	}

	response := performRequest(router, http.MethodGet, "/api/v1/posts?limit=1", nil, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"has_more":true`) {
		t.Fatalf("posts response = %d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodGet, "/api/v1/search?q=alpha+beta", nil, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"pid":12345`) || strings.Contains(response.Body.String(), `"pid":23456`) {
		t.Fatalf("search response = %d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodGet, "/api/v1/search/history", nil, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "alpha beta") {
		t.Fatalf("history response = %d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodGet, "/api/v1/posts?cursor=-1", nil, "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid response = %d %s", response.Code, response.Body.String())
	}
	var failure apiErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil || failure.Error.Code != "invalid_input" || failure.Error.Details == nil {
		t.Fatalf("error body = %+v, %v", failure, err)
	}
}

func TestAPIV1ResearchProjectLifecycle(t *testing.T) {
	database, router, cleanup := setupTestEnv(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{{Pid: 8133824, Text: "project evidence"}}); err != nil {
		t.Fatal(err)
	}
	created := performRequest(router, http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"选课研究","description":"整理课程评价","color":"#0f766e"}`), "application/json")
	if created.Code != http.StatusCreated {
		t.Fatalf("create project = %d %s", created.Code, created.Body.String())
	}
	var projectEnvelope struct {
		Data models.ResearchProject `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &projectEnvelope); err != nil || projectEnvelope.Data.ID == 0 {
		t.Fatalf("created project = %+v, %v", projectEnvelope, err)
	}
	assigned := performRequest(router, http.MethodPut, "/api/v1/posts/8133824/projects", strings.NewReader(fmt.Sprintf(`{"project_ids":[%d]}`, projectEnvelope.Data.ID)), "application/json")
	if assigned.Code != http.StatusOK || !strings.Contains(assigned.Body.String(), `"name":"选课研究"`) {
		t.Fatalf("assign project = %d %s", assigned.Code, assigned.Body.String())
	}
	list := performRequest(router, http.MethodGet, "/api/v1/projects", nil, "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"post_count":1`) {
		t.Fatalf("list projects = %d %s", list.Code, list.Body.String())
	}
	posts := performRequest(router, http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/posts", projectEnvelope.Data.ID), nil, "")
	if posts.Code != http.StatusOK || !strings.Contains(posts.Body.String(), `"pid":8133824`) {
		t.Fatalf("project posts = %d %s", posts.Code, posts.Body.String())
	}
	deleted := performRequest(router, http.MethodDelete, fmt.Sprintf("/api/v1/projects/%d", projectEnvelope.Data.ID), nil, "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete project = %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestAPIV1LocalTagsAndNotes(t *testing.T) {
	database, router, cleanup := setupTestEnv(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{{Pid: 8133824, Text: "local post"}}); err != nil {
		t.Fatal(err)
	}

	created := performRequest(router, http.MethodPost, "/api/v1/local-tags", strings.NewReader(`{"name":"重点","color":"#ef6548"}`), "application/json")
	if created.Code != http.StatusCreated {
		t.Fatalf("create tag = %d %s", created.Code, created.Body.String())
	}
	var tagResponse struct {
		Data models.LocalTag `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &tagResponse); err != nil {
		t.Fatal(err)
	}

	assigned := performRequest(router, http.MethodPut, "/api/v1/posts/8133824/tags", strings.NewReader(`{"tag_ids":[`+strconv.FormatUint(uint64(tagResponse.Data.ID), 10)+`]}`), "application/json")
	if assigned.Code != http.StatusOK || !strings.Contains(assigned.Body.String(), `"name":"重点"`) {
		t.Fatalf("assign tags = %d %s", assigned.Code, assigned.Body.String())
	}
	note := performRequest(router, http.MethodPut, "/api/v1/posts/8133824/note", strings.NewReader(`{"content":"验收笔记"}`), "application/json")
	if note.Code != http.StatusOK || !strings.Contains(note.Body.String(), `"content":"验收笔记"`) {
		t.Fatalf("save note = %d %s", note.Code, note.Body.String())
	}
	read := performRequest(router, http.MethodGet, "/api/v1/posts/8133824/note", nil, "")
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"content":"验收笔记"`) {
		t.Fatalf("read note = %d %s", read.Code, read.Body.String())
	}
	commentNote := performRequest(router, http.MethodPut, "/api/v1/comments/9001/note", strings.NewReader(`{"content":"评论线索"}`), "application/json")
	if commentNote.Code != http.StatusOK || !strings.Contains(commentNote.Body.String(), `"owner_type":"comment"`) {
		t.Fatalf("save comment note = %d %s", commentNote.Code, commentNote.Body.String())
	}
}

func TestAPIV1BatchMetadataAssignments(t *testing.T) {
	database, router, cleanup := setupTestEnv(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{{Pid: 8133824, Text: "first"}, {Pid: 8133825, Text: "second"}}); err != nil {
		t.Fatal(err)
	}
	tag, err := database.CreateLocalTag("批量标签", "#0f766e")
	if err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateResearchProject("批量项目", "", "#0f766e")
	if err != nil {
		t.Fatal(err)
	}
	tags := performRequest(router, http.MethodPost, "/api/v1/posts/batch/tags", strings.NewReader(fmt.Sprintf(`{"pids":[8133824,8133825,8133824],"tag_ids":[%d]}`, tag.ID)), "application/json")
	if tags.Code != http.StatusOK || !strings.Contains(tags.Body.String(), `"updated":2`) {
		t.Fatalf("batch tags = %d %s", tags.Code, tags.Body.String())
	}
	projects := performRequest(router, http.MethodPost, "/api/v1/posts/batch/projects", strings.NewReader(fmt.Sprintf(`{"pids":[8133824,8133825],"project_ids":[%d]}`, project.ID)), "application/json")
	if projects.Code != http.StatusOK || !strings.Contains(projects.Body.String(), `"updated":2`) {
		t.Fatalf("batch projects = %d %s", projects.Code, projects.Body.String())
	}
	rows, err := database.GetResearchProjectPosts(project.ID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("project posts = %+v, %v", rows, err)
	}
	removed := performRequest(router, http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/posts/remove", project.ID), strings.NewReader(`{"pids":[8133824,8133825]}`), "application/json")
	if removed.Code != http.StatusOK || !strings.Contains(removed.Body.String(), `"updated":2`) {
		t.Fatalf("remove project posts = %d %s", removed.Code, removed.Body.String())
	}
	invalid := performRequest(router, http.MethodPost, "/api/v1/posts/batch/tags", strings.NewReader(fmt.Sprintf(`{"pids":[8133824,9999999],"tag_ids":[%d]}`, tag.ID)), "application/json")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid batch tags = %d %s", invalid.Code, invalid.Body.String())
	}
	nonPositive := performRequest(router, http.MethodPost, "/api/v1/posts/batch/projects", strings.NewReader(fmt.Sprintf(`{"pids":[8133824,-1],"project_ids":[%d]}`, project.ID)), "application/json")
	if nonPositive.Code != http.StatusBadRequest {
		t.Fatalf("non-positive batch project assignment = %d %s", nonPositive.Code, nonPositive.Body.String())
	}
}

func TestAPIV1ReferenceGraphExpandsLocalNeighbors(t *testing.T) {
	database, router, cleanup := setupTestEnv(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{
		{Pid: 8133824, Text: "参见 #7853541"},
		{Pid: 7853541, Text: "目标洞"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.RebuildReferences(context.Background()); err != nil {
		t.Fatal(err)
	}
	response := performRequest(router, http.MethodGet, "/api/v1/posts/8133824/references?depth=2", nil, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"pid":7853541`) || !strings.Contains(response.Body.String(), `"kind":"explicit"`) {
		t.Fatalf("reference graph = %d %s", response.Code, response.Body.String())
	}
	invalid := performRequest(router, http.MethodGet, "/api/v1/posts/8133824/references?depth=9", nil, "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid depth = %d %s", invalid.Code, invalid.Body.String())
	}
}

func TestAPIV1OnlineWriteEndpointsUsePostService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	remote := &writeRemoteStub{}
	router := gin.New()
	Init(router, Dependencies{Posts: service.NewPostService(nil, remote), Media: service.NewMediaService(t.TempDir(), nil), DataDir: t.TempDir()})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/posts", strings.NewReader(`{"text":"hello","media_ids":["77"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:50000"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || remote.postRequest.Text != "hello" || len(remote.postRequest.MediaIDs) != 1 {
		t.Fatalf("create post = %d %s request=%+v", response.Code, response.Body.String(), remote.postRequest)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/posts/123456/comments", strings.NewReader(`{"text":"reply","quote_cid":6,"media_ids":["77"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:50000"
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || remote.commentRequest.QuoteID == nil || *remote.commentRequest.QuoteID != 6 {
		t.Fatalf("create comment = %d %s request=%+v", response.Code, response.Body.String(), remote.commentRequest)
	}
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	part, _ := writer.CreateFormFile("file", "image.png")
	_, _ = part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00})
	_ = writer.Close()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/media/uploads", &upload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.RemoteAddr = "127.0.0.1:50000"
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !remote.uploaded || filepath.Ext(remote.uploadedPath) != ".png" || !strings.Contains(response.Body.String(), `"id":"77"`) {
		t.Fatalf("upload = %d %s", response.Code, response.Body.String())
	}
}

func TestAPIV1OnlineWriteMapsRemoteErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	remote := &writeRemoteStub{postErr: &client.RemoteAPIError{HTTPStatus: http.StatusUnauthorized, Code: "40003", Message: "登录状态已失效", Endpoint: "/chapi/api/v3/hole/post"}}
	router := gin.New()
	Init(router, Dependencies{Posts: service.NewPostService(nil, remote), DataDir: t.TempDir()})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/posts", strings.NewReader(`{"text":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:50000"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("response = %d %s headers=%v", response.Code, response.Body.String(), response.Header())
	}
	var failure apiErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
		t.Fatal(err)
	}
	if failure.Error.Code != "online_session_expired" || failure.Error.Message != "登录状态已失效" || failure.Error.Details["remote_code"] != "40003" || failure.Error.Details["request_id"] == "" {
		t.Fatalf("failure = %+v", failure)
	}
}

func TestClassifyOnlineWriteErrorMapsRemoteStatuses(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "forbidden", err: &client.RemoteAPIError{HTTPStatus: http.StatusForbidden, Message: "forbidden"}, wantStatus: http.StatusUnauthorized, wantCode: "online_session_expired"},
		{name: "validation", err: &client.RemoteAPIError{HTTPStatus: http.StatusUnprocessableEntity, Message: "invalid"}, wantStatus: http.StatusUnprocessableEntity, wantCode: "remote_rejected"},
		{name: "business", err: &client.RemoteAPIError{HTTPStatus: http.StatusOK, Code: "41001", Message: "rejected"}, wantStatus: http.StatusUnprocessableEntity, wantCode: "remote_rejected"},
		{name: "rate limit", err: &client.RemoteAPIError{HTTPStatus: http.StatusTooManyRequests, Message: "slow down"}, wantStatus: http.StatusTooManyRequests, wantCode: "remote_rate_limited"},
		{name: "upstream", err: &client.RemoteAPIError{HTTPStatus: http.StatusInternalServerError, Message: "upstream"}, wantStatus: http.StatusBadGateway, wantCode: "publish_failed"},
		{name: "timeout", err: context.DeadlineExceeded, wantStatus: http.StatusGatewayTimeout, wantCode: "remote_timeout"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, code, _, _ := classifyOnlineWriteError(test.err, "publish_failed")
			if status != test.wantStatus || code != test.wantCode {
				t.Fatalf("classifyOnlineWriteError() = %d/%s, want %d/%s", status, code, test.wantStatus, test.wantCode)
			}
		})
	}
}

func TestAPIV1JobLifecycleAndSSEReplay(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	body := strings.NewReader(`{"type":"rebuild_search_index"}`)
	response := performRequest(router, http.MethodPost, "/api/v1/jobs", body, "application/json")
	if response.Code != http.StatusAccepted {
		t.Fatalf("create job = %d %s", response.Code, response.Body.String())
	}
	var created struct {
		Data publicJob `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.Data.ID == "" {
		t.Fatalf("created job = %+v, %v", created, err)
	}
	response = performRequest(router, http.MethodPost, "/api/v1/jobs/"+created.Data.ID+"/cancel", nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("cancel job = %d %s", response.Code, response.Body.String())
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+created.Data.ID+"/events", nil)
	request.Header.Set("Last-Event-ID", "0")
	replay := httptest.NewRecorder()
	router.ServeHTTP(replay, request)
	if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), "event: queued") || !strings.Contains(replay.Body.String(), "event: cancelled") || !strings.Contains(replay.Body.String(), "id: 1") {
		t.Fatalf("SSE replay = %d %q", replay.Code, replay.Body.String())
	}
	scoped := performRequest(router, http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"type":"sync_pids","payload":{"pids":[123456,123456,234567],"include_comments":true}}`), "application/json")
	if scoped.Code != http.StatusAccepted || !strings.Contains(scoped.Body.String(), `"scope":{"pids":[123456,234567]}`) || strings.Contains(scoped.Body.String(), "include_comments") {
		t.Fatalf("safe public job scope = %d %s", scoped.Code, scoped.Body.String())
	}
}

func TestAPIV1ArchiveUploadPreflightAndQueue(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	legacy := `{"holes":[{"pid":123456,"text":"archive"}],"comments":[[]]}`
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "test.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(legacy)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	response := performRequest(router, http.MethodPost, "/api/v1/imports", &body, writer.FormDataContentType())
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"format":"legacy-v1"`) || !strings.Contains(response.Body.String(), `"type":"import_archive"`) {
		t.Fatalf("import response = %d %s", response.Code, response.Body.String())
	}
	history := performRequest(router, http.MethodGet, "/api/v1/imports?limit=50", nil, "")
	if history.Code != http.StatusOK || !strings.Contains(history.Body.String(), `"type":"import_archive"`) || strings.Contains(history.Body.String(), "staging") {
		t.Fatalf("import history = %d %s", history.Code, history.Body.String())
	}
}

func TestAPIV1ToolkitBridgeRequiresPairingAndLocalConfirmation(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()

	create := httptest.NewRequest(http.MethodPost, "/api/v1/bridge/pairings", nil)
	create.Host = "127.0.0.1:8080"
	create.RemoteAddr = "127.0.0.1:54321"
	create.Header.Set("Origin", "http://127.0.0.1:8080")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, create)
	var pairing struct {
		Data BridgePairing `json:"data"`
	}
	if created.Code != http.StatusCreated || json.Unmarshal(created.Body.Bytes(), &pairing) != nil || pairing.Data.Token == "" || !strings.HasPrefix(pairing.Data.Code, "8080:") {
		t.Fatalf("create pairing = %d %s", created.Code, created.Body.String())
	}

	legacy := `{"holes":[{"pid":123456,"text":"bridge archive"}],"comments":[[]]}`
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "bridge.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(legacy))
	_ = writer.Close()
	upload := httptest.NewRequest(http.MethodPost, "/api/v1/bridge/pairings/"+pairing.Data.Token+"/archive", &body)
	upload.Host = "127.0.0.1:8080"
	upload.RemoteAddr = "127.0.0.1:54321"
	upload.Header.Set("Content-Type", writer.FormDataContentType())
	uploaded := httptest.NewRecorder()
	router.ServeHTTP(uploaded, upload)
	if uploaded.Code != http.StatusAccepted || !strings.Contains(uploaded.Body.String(), `"status":"awaiting_confirmation"`) || !strings.Contains(uploaded.Body.String(), `"valid_items":1`) {
		t.Fatalf("upload bridge archive = %d %s", uploaded.Code, uploaded.Body.String())
	}

	confirm := httptest.NewRequest(http.MethodPost, "/api/v1/bridge/pairings/"+pairing.Data.Token+"/confirm", nil)
	confirm.Host = "127.0.0.1:8080"
	confirm.RemoteAddr = "127.0.0.1:54321"
	confirmed := httptest.NewRecorder()
	router.ServeHTTP(confirmed, confirm)
	if confirmed.Code != http.StatusAccepted || !strings.Contains(confirmed.Body.String(), `"status":"queued"`) || !strings.Contains(confirmed.Body.String(), `"type":"import_archive"`) {
		t.Fatalf("confirm bridge import = %d %s", confirmed.Code, confirmed.Body.String())
	}
}

func TestAPIV1ToolkitBridgeRejectsForeignBrowserOrigin(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/bridge/pairings", nil)
	request.Host = "127.0.0.1:8080"
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("Origin", "https://example.com")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign origin = %d %s", response.Code, response.Body.String())
	}
}

func TestAPIV1ArchiveWithNoValidItemsReturnsPreflightWithoutQueue(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	legacy := `{"holes":[{"pid":123456,"text":"archive","timestamp":{"invalid":true}}],"comments":[[]]}`
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "invalid.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(legacy)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	response := performRequest(router, http.MethodPost, "/api/v1/imports", &body, writer.FormDataContentType())
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"code":"archive_no_valid_items"`) || !strings.Contains(response.Body.String(), `"valid_items":0`) {
		t.Fatalf("invalid import response = %d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodGet, "/api/v1/jobs?limit=50", nil, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"data":[]`) {
		t.Fatalf("jobs after rejected import = %d %s", response.Code, response.Body.String())
	}
}

func TestAPIV1FileImportRequiresExplicitConfirmationAfterPreflight(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	legacy := `{"holes":[{"pid":123456,"text":"preview first"}],"comments":[[]]}`
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "preview.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(legacy))
	_ = writer.Close()
	preview := performRequest(router, http.MethodPost, "/api/v1/imports/preflight", &body, writer.FormDataContentType())
	var envelope struct {
		Data BridgePairing `json:"data"`
	}
	if preview.Code != http.StatusOK || json.Unmarshal(preview.Body.Bytes(), &envelope) != nil || envelope.Data.Token == "" || envelope.Data.Preflight == nil || envelope.Data.Status != "awaiting_confirmation" {
		t.Fatalf("import preview = %d %s", preview.Code, preview.Body.String())
	}
	jobsBefore := performRequest(router, http.MethodGet, "/api/v1/jobs?limit=50", nil, "")
	if jobsBefore.Code != http.StatusOK || !strings.Contains(jobsBefore.Body.String(), `"data":[]`) {
		t.Fatalf("jobs before confirmation = %d %s", jobsBefore.Code, jobsBefore.Body.String())
	}
	confirmed := performRequest(router, http.MethodPost, "/api/v1/imports/preflight/"+envelope.Data.Token+"/confirm", nil, "")
	if confirmed.Code != http.StatusAccepted || !strings.Contains(confirmed.Body.String(), `"status":"queued"`) || !strings.Contains(confirmed.Body.String(), `"type":"import_archive"`) {
		t.Fatalf("confirm preview = %d %s", confirmed.Code, confirmed.Body.String())
	}
}

func TestAPIV1FileImportKeepsInvalidPreflightVisibleWithoutCreatingJob(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "empty.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(`{"holes":[],"comments":[]}`))
	_ = writer.Close()
	preview := performRequest(router, http.MethodPost, "/api/v1/imports/preflight", &body, writer.FormDataContentType())
	var envelope struct {
		Data BridgePairing `json:"data"`
	}
	if preview.Code != http.StatusOK || json.Unmarshal(preview.Body.Bytes(), &envelope) != nil || envelope.Data.Preflight == nil || envelope.Data.Preflight.Counts.ValidItems != 0 || envelope.Data.Status != "awaiting_confirmation" {
		t.Fatalf("invalid import preview = %d %s", preview.Code, preview.Body.String())
	}
	confirmed := performRequest(router, http.MethodPost, "/api/v1/imports/preflight/"+envelope.Data.Token+"/confirm", nil, "")
	if confirmed.Code != http.StatusConflict || !strings.Contains(confirmed.Body.String(), `archive contains no valid items`) {
		t.Fatalf("confirm invalid preview = %d %s", confirmed.Code, confirmed.Body.String())
	}
	jobsAfter := performRequest(router, http.MethodGet, "/api/v1/jobs?limit=50", nil, "")
	if jobsAfter.Code != http.StatusOK || !strings.Contains(jobsAfter.Body.String(), `"data":[]`) {
		t.Fatalf("jobs after invalid confirmation = %d %s", jobsAfter.Code, jobsAfter.Body.String())
	}
}

func TestAPIV1ExportsTreeholeV2Archive(t *testing.T) {
	database, router, cleanup := setupTestEnv(t)
	defer cleanup()
	if err := database.SaveCrawlResult([]models.Post{{Pid: 123456, Text: "exported post"}}, []models.Comment{{Cid: 1001, Pid: 123456, Text: "exported comment"}}); err != nil {
		t.Fatal(err)
	}
	response := performRequest(router, http.MethodPost, "/api/v1/exports", strings.NewReader(`{"format":"treehole-v2","pids":[123456],"include_comments":true}`), "application/json")
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Disposition"), ".treehole.zip") || !bytes.HasPrefix(response.Body.Bytes(), []byte{'P', 'K'}) {
		t.Fatalf("export response = %d %v %q", response.Code, response.Header(), response.Body.Bytes())
	}
	parsed, err := archive.Parse(context.Background(), bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil || parsed.Counts.ValidItems != 1 || parsed.Counts.Comments != 1 {
		t.Fatalf("parse exported archive = %+v, %v", parsed, err)
	}
	preview := performRequest(router, http.MethodPost, "/api/v1/exports/preview", strings.NewReader(`{"format":"treehole-v2","pids":[123456],"include_comments":true}`), "application/json")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), `"posts":1`) || !strings.Contains(preview.Body.String(), `"comments":1`) {
		t.Fatalf("export preview = %d %s", preview.Code, preview.Body.String())
	}
}

func TestAPIV1CreatesAndListsPersistentExportJobs(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	created := performRequest(router, http.MethodPost, "/api/v1/exports/jobs", strings.NewReader(`{"format":"treehole-v2","pids":[8133824],"include_comments":true}`), "application/json")
	if created.Code != http.StatusAccepted || !strings.Contains(created.Body.String(), `"type":"export_archive"`) {
		t.Fatalf("create persistent export = %d %s", created.Code, created.Body.String())
	}
	history := performRequest(router, http.MethodGet, "/api/v1/exports/jobs", nil, "")
	if history.Code != http.StatusOK || !strings.Contains(history.Body.String(), `"type":"export_archive"`) {
		t.Fatalf("export history = %d %s", history.Code, history.Body.String())
	}
	invalid := performRequest(router, http.MethodPost, "/api/v1/exports/jobs", strings.NewReader(`{"format":"bad"}`), "application/json")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid export = %d %s", invalid.Code, invalid.Body.String())
	}
	missingPID := performRequest(router, http.MethodPost, "/api/v1/exports/jobs", strings.NewReader(`{"format":"treehole-v2","capture_live":true,"include_media":true}`), "application/json")
	if missingPID.Code != http.StatusBadRequest {
		t.Fatalf("capture export without PID = %d %s", missingPID.Code, missingPID.Body.String())
	}
	capture := performRequest(router, http.MethodPost, "/api/v1/exports/jobs", strings.NewReader(`{"format":"treehole-v2","pids":[8328353],"include_comments":true,"capture_live":true,"include_media":true}`), "application/json")
	if capture.Code != http.StatusAccepted || !strings.Contains(capture.Body.String(), `"total_items":2`) {
		t.Fatalf("capture export = %d %s", capture.Code, capture.Body.String())
	}
}

func TestAPIV1RegeneratesFinishedExportAsNewJob(t *testing.T) {
	database, router, cleanup := setupTestEnv(t)
	defer cleanup()
	now := time.Now().UTC()
	payload := json.RawMessage(`{"format":"treehole-v2","pids":[8133824,8133824],"include_comments":true}`)
	if err := database.CreateJob(context.Background(), jobs.Job{
		ID: "finished-export", Type: jobs.TypeExportArchive, Status: jobs.StatusCompleted,
		Payload: payload, TotalItems: 1, CompletedItems: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	response := performRequest(router, http.MethodPost, "/api/v1/exports/finished-export/regenerate", nil, "")
	if response.Code != http.StatusAccepted {
		t.Fatalf("regenerate export = %d %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data publicJob `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ID == "" || envelope.Data.ID == "finished-export" || envelope.Data.Status != jobs.StatusQueued {
		t.Fatalf("regenerated job = %+v", envelope.Data)
	}
	regenerated, err := database.GetJob(context.Background(), envelope.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	var request exportJobRequest
	if err := json.Unmarshal(regenerated.Payload, &request); err != nil {
		t.Fatal(err)
	}
	if request.Format != archive.ExportFormatTreeholeV2 || !request.IncludeComments || len(request.PIDs) != 1 || request.PIDs[0] != 8133824 {
		t.Fatalf("regenerated payload = %+v", request)
	}
	original, err := database.GetJob(context.Background(), "finished-export")
	if err != nil || original.Status != jobs.StatusCompleted {
		t.Fatalf("original export was mutated: %+v, %v", original, err)
	}
}

func TestAPIV1RejectsRegeneratingActiveExport(t *testing.T) {
	database, router, cleanup := setupTestEnv(t)
	defer cleanup()
	now := time.Now().UTC()
	if err := database.CreateJob(context.Background(), jobs.Job{
		ID: "active-export", Type: jobs.TypeExportArchive, Status: jobs.StatusRunning,
		Payload: json.RawMessage(`{"format":"markdown","include_comments":true}`), TotalItems: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	response := performRequest(router, http.MethodPost, "/api/v1/exports/active-export/regenerate", nil, "")
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"export_not_terminal"`) {
		t.Fatalf("regenerate active export = %d %s", response.Code, response.Body.String())
	}
}

func TestAPIV1ListsAndDownloadsManagedRawJSON(t *testing.T) {
	database, _, cleanup := setupTestEnv(t)
	defer cleanup()
	manager, err := jobs.NewManager(context.Background(), database)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	dataDir := t.TempDir()
	directory := filepath.Join(dataDir, "raw")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	filename := "raw-finished.json"
	content := []byte(`[{"code":20000}]`)
	if err := os.WriteFile(filepath.Join(directory, filename), content, 0o600); err != nil {
		t.Fatal(err)
	}
	checkpoint, _ := json.Marshal(rawJSONJobCheckpoint{Filename: filename, Responses: 1, Bytes: int64(len(content)), ExpiresAt: time.Now().UTC().Add(time.Hour)})
	now := time.Now().UTC()
	if err := database.CreateJob(context.Background(), jobs.Job{
		ID: "raw-finished", Type: jobs.TypeSaveRawJSON, Status: jobs.StatusCompleted,
		Checkpoint: checkpoint, TotalItems: 1, CompletedItems: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	registerAPIV1(router.Group("/api/v1"), Dependencies{Jobs: manager, DataDir: dataDir})
	history := performRequest(router, http.MethodGet, "/api/v1/raw-json/jobs", nil, "")
	if history.Code != http.StatusOK || !strings.Contains(history.Body.String(), `"id":"raw-finished"`) {
		t.Fatalf("raw JSON history = %d %s", history.Code, history.Body.String())
	}
	download := performRequest(router, http.MethodGet, "/api/v1/raw-json/raw-finished/download", nil, "")
	if download.Code != http.StatusOK || !bytes.Equal(download.Body.Bytes(), content) || !strings.Contains(download.Header().Get("Content-Disposition"), filename) {
		t.Fatalf("raw JSON download = %d %v %s", download.Code, download.Header(), download.Body.String())
	}
}

func TestAPIV1RejectsUnknownJSONFieldsAndMissingSearchQuery(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	response := performRequest(router, http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"type":"sync_latest","unknown":true}`), "application/json")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_input"`) {
		t.Fatalf("unknown field response = %d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodGet, "/api/v1/search", nil, "")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_input"`) {
		t.Fatalf("missing q response = %d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodGet, "/api/v1/posts?source=live&label=bad", nil, "")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"field":"label"`) {
		t.Fatalf("invalid label response = %d %s", response.Code, response.Body.String())
	}
}

func TestRemoveStagedImportFileCannotEscapeStagingDirectory(t *testing.T) {
	dataDir := t.TempDir()
	staging := filepath.Join(dataDir, "imports", "staging")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(staging, "cancelled.treehole.zip")
	outside := filepath.Join(dataDir, "keep.txt")
	if err := os.WriteFile(inside, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	insidePayload, _ := json.Marshal(map[string]any{"path": inside})
	removeStagedImportFile(dataDir, insidePayload)
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("staged file still exists: %v", err)
	}
	outsidePayload, _ := json.Marshal(map[string]any{"path": outside})
	removeStagedImportFile(dataDir, outsidePayload)
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was removed: %v", err)
	}
}

func TestAPIV1SessionStatusAndLocalLogin(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	response := performRequest(router, http.MethodGet, "/api/v1/session", nil, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"has_session":true`) {
		t.Fatalf("session response = %d %s", response.Code, response.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/session/login", strings.NewReader(`{"username":"student","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:54321"
	login := httptest.NewRecorder()
	router.ServeHTTP(login, request)
	if login.Code != http.StatusOK || !strings.Contains(login.Body.String(), `"can_read_online":true`) {
		t.Fatalf("local login response = %d %s", login.Code, login.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/session/sms", strings.NewReader(`{"stage":"iaaa","username":"student"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:54321"
	sms := httptest.NewRecorder()
	router.ServeHTTP(sms, request)
	if sms.Code != http.StatusOK || !strings.Contains(sms.Body.String(), `"challenge_stage":"iaaa"`) {
		t.Fatalf("send sms response = %d %s", sms.Code, sms.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/session/sms", strings.NewReader(`{"stage":"treehole"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:54321"
	treeholeSMS := httptest.NewRecorder()
	router.ServeHTTP(treeholeSMS, request)
	if treeholeSMS.Code != http.StatusOK || !strings.Contains(treeholeSMS.Body.String(), `"challenge_stage":"treehole"`) {
		t.Fatalf("send treehole sms response = %d %s", treeholeSMS.Code, treeholeSMS.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/session/challenge", strings.NewReader(`{"stage":"iaaa","challenge":"sms","username":"student","password":"secret","code":"654321"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:54321"
	challenge := httptest.NewRecorder()
	router.ServeHTTP(challenge, request)
	if challenge.Code != http.StatusOK || !strings.Contains(challenge.Body.String(), `"can_read_online":true`) {
		t.Fatalf("challenge response = %d %s", challenge.Code, challenge.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/session/login", strings.NewReader(`{"username":"student","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "192.0.2.10:54321"
	remote := httptest.NewRecorder()
	router.ServeHTTP(remote, request)
	if remote.Code != http.StatusForbidden || !strings.Contains(remote.Body.String(), `"code":"local_access_required"`) {
		t.Fatalf("remote login response = %d %s", remote.Code, remote.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/session/logout", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	logout := httptest.NewRecorder()
	router.ServeHTTP(logout, request)
	if logout.Code != http.StatusOK || !strings.Contains(logout.Body.String(), `"has_session":false`) {
		t.Fatalf("logout response = %d %s", logout.Code, logout.Body.String())
	}
}

func TestAPIV1RejectsSessionWithoutConfiguredProvider(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	response := performRequest(router, http.MethodPost, "/api/v1/ai/sessions", strings.NewReader(`{"mode":"local","title":"Research"}`), "application/json")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "not configured") {
		t.Fatalf("create AI session = %d %s", response.Code, response.Body.String())
	}
}

func TestAPIV1DiagnosticBundleExcludesSecretsAndDatabasePath(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	response := performRequest(router, http.MethodGet, "/api/v1/diagnostics/bundle", nil, "")
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("diagnostics response = %d %s", response.Code, response.Body.String())
	}
	reader, err := stdzip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 3 {
		t.Fatalf("diagnostic entries = %d", len(reader.File))
	}
	var combined strings.Builder
	for _, file := range reader.File {
		stream, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		_, _ = io.Copy(&combined, stream)
		_ = stream.Close()
	}
	text := combined.String()
	if strings.Contains(text, ".db") || strings.Contains(text, "password") || strings.Contains(text, "secret_key") {
		t.Fatalf("diagnostic bundle leaked sensitive configuration: %s", text)
	}
}

func performRequest(router http.Handler, method, target string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	var request *http.Request
	if body == nil {
		request = httptest.NewRequest(method, target, nil)
	} else {
		request = httptest.NewRequest(method, target, body)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request.RemoteAddr = "127.0.0.1:54321"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
