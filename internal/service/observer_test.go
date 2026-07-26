package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func TestValidateObserverConfigRequiresSafeHTTPSURL(t *testing.T) {
	valid := config.DefaultConfig().Observer
	valid.Enabled, valid.BaseURL, valid.APIToken = true, "https://observer.example", "secret"
	if err := ValidateObserverConfig(valid); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"http://observer.example", "https://user@observer.example", "https://observer.example?token=x", "javascript:alert(1)"} {
		candidate := valid
		candidate.BaseURL = raw
		if err := ValidateObserverConfig(candidate); err == nil {
			t.Fatalf("unsafe URL %q was accepted", raw)
		}
	}
}

func TestObserverClientUsesBearerAndAcceptsBarePayload(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer top-secret" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.URL.Path != "/api/v1/capabilities" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		_, _ = io.WriteString(writer, `{"api_version":"v1","instance_id":"instance-1"}`)
	}))
	defer server.Close()
	client := testObserverClient(t, server, "top-secret")
	var capabilities struct {
		APIVersion string `json:"api_version"`
		InstanceID string `json:"instance_id"`
	}
	if err := client.DoJSON(context.Background(), http.MethodGet, "/api/v1/capabilities", nil, &capabilities); err != nil {
		t.Fatal(err)
	}
	if capabilities.APIVersion != "v1" || capabilities.InstanceID != "instance-1" {
		t.Fatalf("capabilities = %+v", capabilities)
	}
}

func TestObserverStatusPreservesChallengeAndBaselineDetails(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/status" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(writer, `{"api_version":"v1","instance_id":"instance-1","auth_state":"challenge_required","challenge_required":true,"baseline_completed":false,"queue_depth":3,"auth":{"state":"challenge_required","challenge":"otp","challenge_stage":"treehole","reason":"token verification required","warning":"session persistence warning","failure_kind":"challenge","masked_target":"138****0000","next_retry_at":"2026-07-16T10:00:00Z","sms_can_resend_at":"2026-07-16T09:00:00Z"}}`)
	}))
	defer server.Close()
	service, err := NewObserverService(&observerRepositoryStub{receipts: map[int64]string{}}, &observerArchiveStub{}, t.TempDir(), config.ObserverConfig{Enabled: true, BaseURL: server.URL, APIToken: "token", RequestTimeout: 15, SyncIntervalMins: 5})
	if err != nil {
		t.Fatal(err)
	}
	service.client.mu.Lock()
	service.client.client.Transport = server.Client().Transport
	service.client.mu.Unlock()
	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || !status.ChallengeRequired || status.Challenge != "otp" || status.ChallengeStage != "treehole" || status.MaskedTarget != "138****0000" || status.AuthWarning != "session persistence warning" || status.AuthFailureKind != "challenge" || status.QueueDepth != 3 || status.BaselineCompleted {
		t.Fatalf("status = %+v", status)
	}
	if status.Traffic != nil {
		t.Fatalf("legacy status unexpectedly gained traffic = %+v", status.Traffic)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"traffic"`) {
		t.Fatalf("legacy status wire shape contains traffic: %s", encoded)
	}
}

func TestObserverConnectionReportsOptionalBuildIdentity(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/capabilities":
			_, _ = io.WriteString(writer, `{"api_version":"v1","instance_id":"instance-build","service_version":"v0.1.0-alpha.1","commit":"0123456789abcdef","build_date":"2026-07-17T04:00:00Z"}`)
		case "/api/v1/status":
			_, _ = io.WriteString(writer, `{"api_version":"v1","instance_id":"instance-build","auth_state":"authenticated","challenge_required":false,"baseline_completed":true,"queue_depth":0}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	service, err := NewObserverService(&observerRepositoryStub{receipts: map[int64]string{}}, &observerArchiveStub{}, t.TempDir(), config.ObserverConfig{Enabled: true, BaseURL: server.URL, APIToken: "token", RequestTimeout: 15, SyncIntervalMins: 5})
	if err != nil {
		t.Fatal(err)
	}
	service.client.mu.Lock()
	service.client.client.Transport = server.Client().Transport
	service.client.mu.Unlock()
	result, err := service.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.ServiceVersion != "v0.1.0-alpha.1" || result.Commit != "0123456789abcdef" || result.BuildDate != "2026-07-17T04:00:00Z" || result.AuthState != "authenticated" {
		t.Fatalf("connection result = %+v", result)
	}
}

func TestObserverStatusMapsOptionalGlobalTrafficGuard(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/status" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(writer, `{"api_version":"v1","instance_id":"instance-traffic","auth_state":"authenticated","baseline_completed":true,"queue_depth":1,"traffic":{"state":"circuit_open","blocked_until":"2026-07-16T10:05:00Z","reason":"consecutive upstream 503 responses","consecutive_rate_limits":2,"consecutive_service_failures":4}}`)
	}))
	defer server.Close()
	service, err := NewObserverService(&observerRepositoryStub{receipts: map[int64]string{}}, &observerArchiveStub{}, t.TempDir(), config.ObserverConfig{Enabled: true, BaseURL: server.URL, APIToken: "token", RequestTimeout: 15, SyncIntervalMins: 5})
	if err != nil {
		t.Fatal(err)
	}
	service.client.mu.Lock()
	service.client.client.Transport = server.Client().Transport
	service.client.mu.Unlock()
	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Traffic == nil || status.Traffic.State != "circuit_open" || status.Traffic.BlockedUntil != "2026-07-16T10:05:00Z" || status.Traffic.Reason != "consecutive upstream 503 responses" || status.Traffic.ConsecutiveRateLimits != 2 || status.Traffic.ConsecutiveServiceFailures != 4 {
		t.Fatalf("traffic = %+v", status.Traffic)
	}
}

func TestObserverSyncImportsVerifiedSnapshotAndAdvancesCursorIdempotently(t *testing.T) {
	archiveBytes := []byte("test observer archive")
	digest := sha256.Sum256(archiveBytes)
	hash := hex.EncodeToString(digest[:])
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/v1/sync/events":
			if request.URL.Query().Get("after") == "0" {
				_, _ = fmt.Fprintf(writer, `{"schema_version":1,"instance_id":"instance-a","events":[{"id":1,"type":"availability.confirmed_unavailable","pid":123456,"occurred_at":"2026-07-16T10:00:00Z","availability":{"state":"confirmed_unavailable","observed_at":"2026-07-16T10:00:00Z"},"snapshot":{"id":"snap-1","url":"/api/v1/snapshots/snap-1.treehole.zip","sha256":"%s","size":%d,"run_id":"run-1"}}],"next_after":1,"has_more":false}`, hash, len(archiveBytes))
			} else {
				_, _ = io.WriteString(writer, `{"schema_version":1,"instance_id":"instance-a","events":[],"next_after":1,"has_more":false}`)
			}
		case "/api/v1/snapshots/snap-1.treehole.zip":
			_, _ = writer.Write(archiveBytes)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	repository := &observerRepositoryStub{receipts: map[int64]string{}}
	archiveService := &observerArchiveStub{}
	service, err := NewObserverService(repository, archiveService, t.TempDir(), config.ObserverConfig{Enabled: true, BaseURL: server.URL, APIToken: "token", RequestTimeout: 15, SyncIntervalMins: 5})
	if err != nil {
		t.Fatal(err)
	}
	service.client.mu.Lock()
	service.client.client.Transport = server.Client().Transport
	service.client.mu.Unlock()
	first, err := service.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.EventsApplied != 1 || first.LastEventID != 1 || second.EventsApplied != 0 || archiveService.imports != 1 {
		t.Fatalf("first=%+v second=%+v imports=%d", first, second, archiveService.imports)
	}
	if repository.availability.State != "confirmed_unavailable" || repository.availability.PID != 123456 || repository.availability.Completeness != "unknown" {
		t.Fatalf("availability = %+v", repository.availability)
	}
}

func TestObserverSettingsKeepTokenWriteOnly(t *testing.T) {
	current := config.DefaultConfig()
	current.Observer = config.ObserverConfig{Enabled: true, BaseURL: "https://old.example", APIToken: "preserved", RequestTimeout: 15, SyncIntervalMins: 5}
	settings := NewSettingsService(&current)
	settings.save = func(*config.Config) error { return nil }
	view, err := settings.UpdateObserver(context.Background(), ObserverSettingsUpdate{Enabled: true, BaseURL: "https://new.example/", RequestTimeout: 20, SyncIntervalMins: 10, AutoSyncOnStart: true, SyncBeforeExport: true})
	if err != nil {
		t.Fatal(err)
	}
	if current.Observer.APIToken != "preserved" || !view.APITokenConfigured || strings.Contains(fmt.Sprintf("%+v", view), "preserved") {
		t.Fatalf("token handling failed: config=%+v view=%+v", current.Observer, view)
	}
}

func TestObserverServiceCloseWaitsForSyncLoop(t *testing.T) {
	service, err := NewObserverService(&observerRepositoryStub{receipts: map[int64]string{}}, &observerArchiveStub{}, t.TempDir(), config.DefaultConfig().Observer)
	if err != nil {
		t.Fatal(err)
	}
	service.Start(context.Background())
	service.Close()
	service.Close()
	select {
	case <-service.done:
	default:
		t.Fatal("Close returned before the Observer sync loop stopped")
	}
}

func TestDisabledObserverDoesNotFetchHotPosts(t *testing.T) {
	service, err := NewObserverService(&observerRepositoryStub{receipts: map[int64]string{}}, &observerArchiveStub{}, t.TempDir(), config.DefaultConfig().Observer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.HotPosts(context.Background(), 5, 12*time.Hour); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("HotPosts() error = %v, want disabled", err)
	}
}

func testObserverClient(t *testing.T, server *httptest.Server, token string) *ObserverClient {
	t.Helper()
	client, err := NewObserverClient(config.ObserverConfig{Enabled: true, BaseURL: server.URL, APIToken: token, RequestTimeout: 15, SyncIntervalMins: 5})
	if err != nil {
		t.Fatal(err)
	}
	client.mu.Lock()
	client.client.Transport = server.Client().Transport
	client.mu.Unlock()
	return client
}

type observerRepositoryStub struct {
	mu           sync.Mutex
	state        models.ObserverSyncState
	receipts     map[int64]string
	availability models.PostAvailability
}

func (r *observerRepositoryStub) GetObserverSyncState(context.Context, string) (models.ObserverSyncState, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state, r.state.ObserverID != "", nil
}
func (r *observerRepositoryStub) SaveObserverSyncState(_ context.Context, state models.ObserverSyncState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = state
	return nil
}
func (r *observerRepositoryStub) RecordObserverSyncFailure(context.Context, string, string, string) error {
	return nil
}
func (r *observerRepositoryStub) ApplyObserverEvent(_ context.Context, state models.ObserverSyncState, receipt models.ObserverEventReceipt, availability models.PostAvailability) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if hash, exists := r.receipts[receipt.EventID]; exists {
		if hash != receipt.PayloadHash {
			return false, fmt.Errorf("changed payload")
		}
		r.state = state
		return false, nil
	}
	r.receipts[receipt.EventID], r.state, r.availability = receipt.PayloadHash, state, availability
	return true, nil
}
func (*observerRepositoryStub) ListPostAvailabilities(context.Context, string, string, int, int) ([]models.PostAvailability, bool, error) {
	return nil, false, nil
}
func (*observerRepositoryStub) GetPostAvailability(context.Context, int32) (models.PostAvailability, bool, error) {
	return models.PostAvailability{}, false, nil
}
func (*observerRepositoryStub) ObserverPostContents(context.Context, int32) (*models.Post, []models.Comment, []models.Media, error) {
	return nil, nil, nil, nil
}

type observerArchiveStub struct{ imports int }

func (*observerArchiveStub) Preflight(context.Context, io.ReaderAt, int64) (ArchivePreflight, error) {
	return ArchivePreflight{}, nil
}
func (s *observerArchiveStub) Import(context.Context, io.ReaderAt, int64) (ArchiveImportReport, error) {
	s.imports++
	return ArchiveImportReport{Status: archive.StatusCompleted}, nil
}
func (*observerArchiveStub) Preview(context.Context, ArchiveExportRequest) (ArchiveExportReport, error) {
	return ArchiveExportReport{}, nil
}
func (*observerArchiveStub) Export(context.Context, io.Writer, ArchiveExportRequest) (ArchiveExportReport, error) {
	return ArchiveExportReport{}, nil
}
