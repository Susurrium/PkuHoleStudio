package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

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

	request = httptest.NewRequest(http.MethodPost, "/api/v1/session/login", strings.NewReader(`{"username":"student","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "192.0.2.10:54321"
	remote := httptest.NewRecorder()
	router.ServeHTTP(remote, request)
	if remote.Code != http.StatusForbidden || !strings.Contains(remote.Body.String(), `"code":"local_access_required"`) {
		t.Fatalf("remote login response = %d %s", remote.Code, remote.Body.String())
	}
}

func TestAPIV1AISessionLifecycleWithoutConfiguredProvider(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	response := performRequest(router, http.MethodPost, "/api/v1/ai/sessions", strings.NewReader(`{"mode":"local","title":"Research"}`), "application/json")
	if response.Code != http.StatusCreated {
		t.Fatalf("create AI session = %d %s", response.Code, response.Body.String())
	}
	var created struct {
		Data models.AISession `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.Data.ID == "" {
		t.Fatalf("created AI session = %+v, %v", created, err)
	}
	response = performRequest(router, http.MethodGet, "/api/v1/ai/sessions/"+created.Data.ID, nil, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Research") {
		t.Fatalf("get AI session = %d %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodPost, "/api/v1/ai/sessions/"+created.Data.ID+"/messages", strings.NewReader(`{"prompt":"question"}`), "application/json")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "not configured") {
		t.Fatalf("unconfigured AI message = %d %s", response.Code, response.Body.String())
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
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
