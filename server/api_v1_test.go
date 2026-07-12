package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"
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
	upload.RemoteAddr = "127.0.0.1:54321"
	upload.Header.Set("Content-Type", writer.FormDataContentType())
	uploaded := httptest.NewRecorder()
	router.ServeHTTP(uploaded, upload)
	if uploaded.Code != http.StatusAccepted || !strings.Contains(uploaded.Body.String(), `"status":"awaiting_confirmation"`) || !strings.Contains(uploaded.Body.String(), `"valid_items":1`) {
		t.Fatalf("upload bridge archive = %d %s", uploaded.Code, uploaded.Body.String())
	}

	confirm := httptest.NewRequest(http.MethodPost, "/api/v1/bridge/pairings/"+pairing.Data.Token+"/confirm", nil)
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

	request = httptest.NewRequest(http.MethodPost, "/api/v1/session/sms", strings.NewReader(`{"username":"student"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:54321"
	sms := httptest.NewRecorder()
	router.ServeHTTP(sms, request)
	if sms.Code != http.StatusOK || !strings.Contains(sms.Body.String(), `"challenge_stage":"iaaa"`) {
		t.Fatalf("send sms response = %d %s", sms.Code, sms.Body.String())
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
