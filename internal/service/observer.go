package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

const (
	defaultObserverID      = "default"
	maxObserverJSONBytes   = int64(8 << 20)
	maxObserverArchiveSize = int64(200 << 20)
)

type ObserverRepository interface {
	GetObserverSyncState(context.Context, string) (models.ObserverSyncState, bool, error)
	SaveObserverSyncState(context.Context, models.ObserverSyncState) error
	RecordObserverSyncFailure(context.Context, string, string, string) error
	ApplyObserverEvent(context.Context, models.ObserverSyncState, models.ObserverEventReceipt, models.PostAvailability) (bool, error)
	ListPostAvailabilities(context.Context, string, string, int, int) ([]models.PostAvailability, bool, error)
	GetPostAvailability(context.Context, int32) (models.PostAvailability, bool, error)
	ObserverPostContents(context.Context, int32) (*models.Post, []models.Comment, []models.Media, error)
}

type ObserverClient struct {
	mu     sync.RWMutex
	config config.ObserverConfig
	base   *url.URL
	client *http.Client
}

func NewObserverClient(value config.ObserverConfig) (*ObserverClient, error) {
	client := &ObserverClient{}
	if err := client.Configure(value); err != nil {
		return nil, err
	}
	return client, nil
}

func ValidateObserverConfig(value config.ObserverConfig) error {
	config.NormalizeObserver(&value)
	if value.RequestTimeout < 1 || value.RequestTimeout > 300 {
		return errors.New("Observer request timeout must be between 1 and 300 seconds")
	}
	if value.SyncIntervalMins < 1 || value.SyncIntervalMins > 1440 {
		return errors.New("Observer sync interval must be between 1 and 1440 minutes")
	}
	if strings.TrimSpace(value.BaseURL) == "" {
		if value.Enabled {
			return errors.New("Observer base URL is required when Observer is enabled")
		}
		return nil
	}
	parsed, err := url.Parse(value.BaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("Observer base URL must be an absolute HTTPS URL without user info, query, or fragment")
	}
	if value.Enabled && strings.TrimSpace(value.APIToken) == "" {
		return errors.New("Observer API token is required when Observer is enabled")
	}
	return nil
}

func (c *ObserverClient) Configure(value config.ObserverConfig) error {
	config.NormalizeObserver(&value)
	if err := ValidateObserverConfig(value); err != nil {
		return err
	}
	var parsed *url.URL
	if value.BaseURL != "" {
		var err error
		parsed, err = url.Parse(value.BaseURL)
		if err != nil {
			return err
		}
	}
	httpClient := &http.Client{Timeout: time.Duration(value.RequestTimeout) * time.Second}
	httpClient.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("Observer redirect limit exceeded")
		}
		if parsed == nil || !sameOrigin(parsed, request.URL) {
			return errors.New("Observer refused a cross-origin redirect")
		}
		return nil
	}
	c.mu.Lock()
	c.config, c.base, c.client = value, parsed, httpClient
	c.mu.Unlock()
	return nil
}

func (c *ObserverClient) Config() config.ObserverConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

func (c *ObserverClient) snapshot() (config.ObserverConfig, *url.URL, *http.Client, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.base == nil || c.client == nil || strings.TrimSpace(c.config.APIToken) == "" {
		return config.ObserverConfig{}, nil, nil, errors.New("Observer is not configured")
	}
	baseCopy := *c.base
	return c.config, &baseCopy, c.client, nil
}

func (c *ObserverClient) apiURL(endpoint string) (*url.URL, config.ObserverConfig, *http.Client, error) {
	cfg, base, httpClient, err := c.snapshot()
	if err != nil {
		return nil, cfg, nil, err
	}
	relative, err := url.Parse(endpoint)
	if err != nil || relative.IsAbs() || relative.Host != "" {
		return nil, cfg, nil, errors.New("invalid Observer API path")
	}
	base.Path = path.Join(strings.TrimSuffix(base.Path, "/"), strings.TrimPrefix(relative.Path, "/"))
	if strings.HasSuffix(relative.Path, "/") && !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	base.RawQuery = relative.RawQuery
	return base, cfg, httpClient, nil
}

func (c *ObserverClient) DoJSON(ctx context.Context, method, endpoint string, body any, output any) error {
	target, cfg, httpClient, err := c.apiURL(endpoint)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("Observer request failed: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxObserverJSONBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxObserverJSONBytes {
		return errors.New("Observer response exceeded the JSON size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return observerHTTPError(response.StatusCode, data)
	}
	if output == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	return decodeObserverPayload(data, output)
}

func (c *ObserverClient) DownloadSnapshot(ctx context.Context, rawURL, expectedHash string, expectedSize int64, dataDir string) (*os.File, int64, error) {
	cfg, base, httpClient, err := c.snapshot()
	if err != nil {
		return nil, 0, err
	}
	if expectedSize <= 0 || expectedSize > maxObserverArchiveSize {
		return nil, 0, errors.New("Observer snapshot size is invalid")
	}
	if len(expectedHash) != 64 {
		return nil, 0, errors.New("Observer snapshot SHA-256 is invalid")
	}
	reference, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || strings.TrimSpace(rawURL) == "" {
		return nil, 0, errors.New("Observer snapshot URL is invalid")
	}
	target := base.ResolveReference(reference)
	if !sameOrigin(base, target) {
		return nil, 0, errors.New("Observer snapshot URL must be same-origin")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Accept", "application/zip")
	request.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("download Observer snapshot: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		return nil, 0, observerHTTPError(response.StatusCode, body)
	}
	staging := filepath.Join(dataDir, "observer-sync")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return nil, 0, err
	}
	file, err := os.CreateTemp(staging, ".snapshot-*.treehole.zip")
	if err != nil {
		return nil, 0, err
	}
	cleanup := func(err error) (*os.File, int64, error) {
		name := file.Name()
		_ = file.Close()
		_ = os.Remove(name)
		return nil, 0, err
	}
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(response.Body, expectedSize+1))
	if err != nil {
		return cleanup(err)
	}
	if written != expectedSize {
		return cleanup(fmt.Errorf("Observer snapshot size mismatch: received %d, expected %d", written, expectedSize))
	}
	if actual := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(actual, expectedHash) {
		return cleanup(errors.New("Observer snapshot failed its SHA-256 check"))
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return cleanup(err)
	}
	return file, written, nil
}

func sameOrigin(left, right *url.URL) bool {
	return left != nil && right != nil && strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}

func observerHTTPError(status int, data []byte) error {
	message := strings.TrimSpace(string(data))
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &envelope) == nil {
		if envelope.Error.Message != "" {
			message = envelope.Error.Message
		} else if envelope.Message != "" {
			message = envelope.Message
		}
	}
	if len(message) > 300 {
		message = message[:300]
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return fmt.Errorf("Observer returned HTTP %d: %s", status, message)
}

func decodeObserverPayload(data []byte, output any) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("decode Observer response: %w", err)
	}
	if raw, ok := fields["data"]; ok && len(raw) > 0 && string(raw) != "null" {
		data = raw
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode Observer response: %w", err)
	}
	return nil
}

type ObserverStatusResult struct {
	Configured           bool                   `json:"configured"`
	Enabled              bool                   `json:"enabled"`
	Connected            bool                   `json:"connected"`
	Stale                bool                   `json:"stale"`
	InstanceID           string                 `json:"instance_id,omitempty"`
	APIVersion           string                 `json:"api_version,omitempty"`
	AuthState            string                 `json:"auth_state,omitempty"`
	ChallengeRequired    bool                   `json:"challenge_required"`
	Challenge            string                 `json:"challenge,omitempty"`
	ChallengeStage       string                 `json:"challenge_stage,omitempty"`
	MaskedTarget         string                 `json:"masked_target,omitempty"`
	AuthReason           string                 `json:"auth_reason,omitempty"`
	AuthWarning          string                 `json:"auth_warning,omitempty"`
	AuthFailureKind      string                 `json:"auth_failure_kind,omitempty"`
	NextRetryAt          string                 `json:"next_retry_at,omitempty"`
	SMSCanResendAt       string                 `json:"sms_can_resend_at,omitempty"`
	LastSuccessfulScanAt string                 `json:"last_successful_scan_at,omitempty"`
	LatestPostAt         string                 `json:"latest_post_at,omitempty"`
	CoverageDegraded     bool                   `json:"coverage_degraded"`
	BaselineCompleted    bool                   `json:"baseline_completed"`
	QueueDepth           int                    `json:"queue_depth"`
	Traffic              *ObserverTrafficStatus `json:"traffic,omitempty"`
	LastError            string                 `json:"last_error,omitempty"`
	Remote               map[string]any         `json:"remote,omitempty"`
}

// ObserverTrafficStatus mirrors the optional global upstream traffic guard
// exposed by newer Observer versions. It remains a pointer on the Studio
// status response so older Observer instances retain their original wire
// shape instead of gaining an artificial zero-value traffic object.
type ObserverTrafficStatus struct {
	State                      string `json:"state"`
	BlockedUntil               string `json:"blocked_until,omitempty"`
	Reason                     string `json:"reason,omitempty"`
	ConsecutiveRateLimits      int    `json:"consecutive_rate_limits"`
	ConsecutiveServiceFailures int    `json:"consecutive_service_failures"`
}

type ObserverConnectionTest struct {
	OK             bool   `json:"ok"`
	InstanceID     string `json:"instance_id,omitempty"`
	APIVersion     string `json:"api_version,omitempty"`
	ServiceVersion string `json:"service_version,omitempty"`
	Commit         string `json:"commit,omitempty"`
	BuildDate      string `json:"build_date,omitempty"`
	AuthState      string `json:"auth_state,omitempty"`
	Message        string `json:"message"`
}

type ObserverSyncResult struct {
	InstanceID        string    `json:"instance_id"`
	EventsReceived    int       `json:"events_received"`
	EventsApplied     int       `json:"events_applied"`
	SnapshotsImported int       `json:"snapshots_imported"`
	LastEventID       int64     `json:"last_event_id"`
	CompletedAt       time.Time `json:"completed_at"`
}

type ObserverRemovedItem struct {
	PID                int32        `json:"pid"`
	State              string       `json:"state"`
	ObservedAt         time.Time    `json:"observed_at"`
	FirstUnavailableAt *time.Time   `json:"first_unavailable_at,omitempty"`
	LastUnavailableAt  *time.Time   `json:"last_unavailable_at,omitempty"`
	RestoredAt         *time.Time   `json:"restored_at,omitempty"`
	ObserverID         string       `json:"observer_id"`
	Completeness       string       `json:"completeness"`
	Post               *models.Post `json:"post,omitempty"`
}

type ObserverRemovedPage struct {
	Items      []ObserverRemovedItem `json:"items"`
	NextCursor int                   `json:"next_cursor,omitempty"`
	HasMore    bool                  `json:"has_more"`
}

type ObserverRemovedDetail struct {
	Availability models.PostAvailability `json:"availability"`
	Post         *models.Post            `json:"post,omitempty"`
	Comments     []models.Comment        `json:"comments"`
	Media        []models.Media          `json:"media"`
}

type ObserverService struct {
	client      *ObserverClient
	repository  ObserverRepository
	archive     ArchiveService
	dataDir     string
	syncMu      sync.Mutex
	wake        chan struct{}
	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
}

func NewObserverService(repository ObserverRepository, archive ArchiveService, dataDir string, value config.ObserverConfig) (*ObserverService, error) {
	client, err := NewObserverClient(value)
	if err != nil {
		return nil, err
	}
	return &ObserverService{client: client, repository: repository, archive: archive, dataDir: dataDir, wake: make(chan struct{}, 1)}, nil
}

func (s *ObserverService) Configure(value config.ObserverConfig) error {
	if s == nil || s.client == nil {
		return errors.New("Observer service is unavailable")
	}
	// Keep one sync run bound to one immutable endpoint/token pair. Otherwise
	// a settings update between the event page and snapshot download could mix
	// two Observer instances in a single transaction stream.
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	if err := s.client.Configure(value); err != nil {
		return err
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}

func (s *ObserverService) Start(parent context.Context) {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel, s.done = cancel, make(chan struct{})
	go func() {
		defer close(s.done)
		s.syncLoop(ctx)
	}()
}

func (s *ObserverService) Close() {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	cancel, done := s.cancel, s.done
	s.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *ObserverService) syncLoop(ctx context.Context) {
	first := true
	for {
		cfg := s.client.Config()
		if cfg.Enabled && (!first || cfg.AutoSyncOnStart) {
			syncCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			_, _ = s.Sync(syncCtx)
			cancel()
		}
		first = false
		interval := time.Duration(max(1, cfg.SyncIntervalMins)) * time.Minute
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.wake:
			timer.Stop()
			first = true
		case <-timer.C:
		}
	}
}

func (s *ObserverService) Status(ctx context.Context) (ObserverStatusResult, error) {
	if s == nil || s.client == nil {
		return ObserverStatusResult{}, errors.New("Observer service is unavailable")
	}
	cfg := s.client.Config()
	result := ObserverStatusResult{Configured: cfg.BaseURL != "" && cfg.APIToken != "", Enabled: cfg.Enabled}
	if !result.Configured || !cfg.Enabled {
		return result, nil
	}
	remote := map[string]any{}
	if err := s.client.DoJSON(ctx, http.MethodGet, "/api/v1/status", nil, &remote); err != nil {
		result.LastError = err.Error()
		return result, err
	}
	result.Connected, result.Remote = true, remote
	result.InstanceID = stringValue(remote, "instance_id")
	result.APIVersion = stringValue(remote, "api_version")
	result.AuthState = stringValue(remote, "auth_state")
	if result.AuthState == "" {
		result.AuthState = nestedStringValue(remote, "auth", "state")
	}
	result.ChallengeRequired = boolValue(remote, "challenge_required") || result.AuthState == "challenge_required"
	result.Challenge = nestedStringValue(remote, "auth", "challenge")
	result.ChallengeStage = nestedStringValue(remote, "auth", "challenge_stage")
	result.MaskedTarget = nestedStringValue(remote, "auth", "masked_target")
	result.AuthReason = nestedStringValue(remote, "auth", "reason")
	result.AuthWarning = nestedStringValue(remote, "auth", "warning")
	result.AuthFailureKind = nestedStringValue(remote, "auth", "failure_kind")
	result.NextRetryAt = nestedStringValue(remote, "auth", "next_retry_at")
	result.SMSCanResendAt = nestedStringValue(remote, "auth", "sms_can_resend_at")
	result.LastSuccessfulScanAt = stringValue(remote, "last_successful_scan_at")
	result.LatestPostAt = stringValue(remote, "latest_post_at")
	result.CoverageDegraded = boolValue(remote, "coverage_degraded")
	result.BaselineCompleted = boolValue(remote, "baseline_completed")
	result.QueueDepth = intValue(remote, "queue_depth")
	if traffic, ok := remote["traffic"].(map[string]any); ok {
		result.Traffic = &ObserverTrafficStatus{
			State:                      stringValue(traffic, "state"),
			BlockedUntil:               stringValue(traffic, "blocked_until"),
			Reason:                     stringValue(traffic, "reason"),
			ConsecutiveRateLimits:      intValue(traffic, "consecutive_rate_limits"),
			ConsecutiveServiceFailures: intValue(traffic, "consecutive_service_failures"),
		}
	}
	result.Stale = boolValue(remote, "stale")
	if !result.Stale && result.LastSuccessfulScanAt != "" {
		if scannedAt, err := time.Parse(time.RFC3339, result.LastSuccessfulScanAt); err == nil {
			result.Stale = time.Since(scannedAt) > 15*time.Minute
		}
	}
	return result, nil
}

func (s *ObserverService) Test(ctx context.Context) (ObserverConnectionTest, error) {
	var capabilities struct {
		APIVersion     string `json:"api_version"`
		InstanceID     string `json:"instance_id"`
		ServiceVersion string `json:"service_version"`
		Commit         string `json:"commit"`
		BuildDate      string `json:"build_date"`
	}
	if err := s.client.DoJSON(ctx, http.MethodGet, "/api/v1/capabilities", nil, &capabilities); err != nil {
		return ObserverConnectionTest{Message: err.Error()}, err
	}
	result := ObserverConnectionTest{
		InstanceID:     capabilities.InstanceID,
		APIVersion:     capabilities.APIVersion,
		ServiceVersion: capabilities.ServiceVersion,
		Commit:         capabilities.Commit,
		BuildDate:      capabilities.BuildDate,
	}
	status, err := s.Status(ctx)
	if err != nil {
		result.Message = err.Error()
		return result, err
	}
	result.OK = true
	result.AuthState = status.AuthState
	result.Message = "Observer connection succeeded"
	return result, nil
}

func (s *ObserverService) HotPosts(ctx context.Context, limit int, window time.Duration) (HotPostsResult, error) {
	if s == nil || s.client == nil || !s.client.Config().Enabled {
		return HotPostsResult{}, errors.New("Observer is disabled")
	}
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	if window <= 0 {
		window = 12 * time.Hour
	}
	var response struct {
		Posts []struct {
			Post              models.Post `json:"post"`
			Score             float64     `json:"score"`
			AvailabilityState string      `json:"availability_state"`
		} `json:"posts"`
		GeneratedAt          string `json:"generated_at"`
		LastSuccessfulScanAt string `json:"last_successful_scan_at"`
		LatestPostAt         string `json:"latest_post_at"`
		ScoreVersion         string `json:"score_version"`
		Stale                bool   `json:"stale"`
	}
	endpoint := "/api/v1/hot?limit=" + strconv.Itoa(limit) + "&window_hours=" + strconv.Itoa(max(1, int(window/time.Hour)))
	if err := s.client.DoJSON(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return HotPostsResult{}, err
	}
	items := make([]HotPost, 0, min(limit, len(response.Posts)))
	for _, item := range response.Posts {
		if len(items) >= limit {
			break
		}
		items = append(items, HotPost{ID: item.Post.Pid, Text: item.Post.Text, FollowNum: max(int(item.Post.PraiseNum), int(item.Post.Likenum)), ReplyNum: int(item.Post.Reply), Timestamp: int64(item.Post.Timestamp), Score: item.Score, AvailabilityState: item.AvailabilityState})
	}
	updatedAt := time.Now().Unix()
	if parsed, err := time.Parse(time.RFC3339, response.GeneratedAt); err == nil {
		updatedAt = parsed.Unix()
	}
	latestTimestamp := int64(0)
	if parsed, err := time.Parse(time.RFC3339, response.LatestPostAt); err == nil {
		latestTimestamp = parsed.Unix()
	}
	return HotPostsResult{Items: items, Source: "observer", WindowHours: max(1, int(window/time.Hour)), UpdatedAt: updatedAt, LatestTimestamp: latestTimestamp, Stale: response.Stale, GeneratedAt: response.GeneratedAt, LastSuccessfulScanAt: response.LastSuccessfulScanAt, LatestPostAt: response.LatestPostAt, ScoreVersion: response.ScoreVersion}, nil
}

type observerSyncResponse struct {
	SchemaVersion int                 `json:"schema_version"`
	InstanceID    string              `json:"instance_id"`
	Events        []observerSyncEvent `json:"events"`
	NextAfter     int64               `json:"next_after"`
	HasMore       bool                `json:"has_more"`
}

type observerSyncEvent struct {
	ID           int64  `json:"id"`
	Type         string `json:"type"`
	PID          int32  `json:"pid"`
	OccurredAt   string `json:"occurred_at"`
	Availability struct {
		State        string `json:"state"`
		ObservedAt   string `json:"observed_at"`
		Completeness string `json:"completeness,omitempty"`
	} `json:"availability"`
	Snapshot *struct {
		ID     string `json:"id"`
		URL    string `json:"url"`
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
		RunID  string `json:"run_id"`
	} `json:"snapshot,omitempty"`
}

func (s *ObserverService) Sync(ctx context.Context) (result ObserverSyncResult, err error) {
	if s == nil || s.repository == nil || s.archive == nil {
		return result, errors.New("Observer sync is unavailable")
	}
	cfg := s.client.Config()
	if !cfg.Enabled {
		return result, errors.New("Observer is disabled")
	}
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	state, found, err := s.repository.GetObserverSyncState(ctx, defaultObserverID)
	if err != nil {
		return result, err
	}
	after := int64(0)
	if found {
		after = state.LastEventID
	}
	instanceID := state.RemoteInstanceID
	defer func() {
		if err != nil {
			_ = s.repository.RecordObserverSyncFailure(context.Background(), defaultObserverID, instanceID, err.Error())
		}
	}()
	for pages := 0; pages < 1000; pages++ {
		var page observerSyncResponse
		endpoint := fmt.Sprintf("/api/v1/sync/events?after=%d&limit=100", after)
		if err = s.client.DoJSON(ctx, http.MethodGet, endpoint, nil, &page); err != nil {
			return result, err
		}
		if page.SchemaVersion != 1 || strings.TrimSpace(page.InstanceID) == "" {
			return result, errors.New("Observer returned an incompatible sync response")
		}
		if instanceID != "" && instanceID != page.InstanceID && after != 0 {
			instanceID, after = page.InstanceID, 0
			state = models.ObserverSyncState{}
			continue
		}
		instanceID = page.InstanceID
		result.InstanceID = instanceID
		for _, event := range page.Events {
			result.EventsReceived++
			if event.ID <= after || event.PID <= 0 || (event.Type != "availability.confirmed_unavailable" && event.Type != "availability.restored") {
				return result, errors.New("Observer returned an invalid or out-of-order event")
			}
			expectedState := "confirmed_unavailable"
			if event.Type == "availability.restored" {
				expectedState = "restored"
			}
			if event.Availability.State != expectedState {
				return result, fmt.Errorf("Observer event %d type and availability state disagree", event.ID)
			}
			event.Availability.Completeness = strings.TrimSpace(event.Availability.Completeness)
			if event.Availability.Completeness == "" {
				event.Availability.Completeness = "unknown"
			}
			switch event.Availability.Completeness {
			case "complete", "partial", "unknown":
			default:
				return result, fmt.Errorf("Observer event %d has invalid completeness", event.ID)
			}
			observedAt, parseErr := parseObserverEventTime(event.Availability.ObservedAt, event.OccurredAt)
			if parseErr != nil {
				return result, parseErr
			}
			if event.Type == "availability.confirmed_unavailable" && event.Snapshot == nil {
				return result, errors.New("Observer deletion event is missing its snapshot")
			}
			if event.Snapshot != nil {
				if strings.TrimSpace(event.Snapshot.ID) == "" || len(event.Snapshot.ID) > 128 || strings.TrimSpace(event.Snapshot.RunID) == "" || len(event.Snapshot.RunID) > 256 {
					return result, fmt.Errorf("Observer event %d has invalid snapshot identity", event.ID)
				}
				file, size, downloadErr := s.client.DownloadSnapshot(ctx, event.Snapshot.URL, event.Snapshot.SHA256, event.Snapshot.Size, s.dataDir)
				if downloadErr != nil {
					return result, downloadErr
				}
				name := file.Name()
				_, importErr := s.archive.Import(ctx, file, size)
				_ = file.Close()
				_ = os.Remove(name)
				if importErr != nil {
					return result, fmt.Errorf("import Observer snapshot for PID %d: %w", event.PID, importErr)
				}
				result.SnapshotsImported++
			}
			payload, _ := json.Marshal(event)
			digest := sha256.Sum256(payload)
			now := time.Now().UTC()
			availability := models.PostAvailability{PID: event.PID, State: event.Availability.State, ObservedAt: observedAt, ObserverID: defaultObserverID, RemoteInstanceID: instanceID, Completeness: event.Availability.Completeness, UpdatedAt: now}
			if event.Snapshot != nil {
				availability.SnapshotID = event.Snapshot.ID
			}
			if event.Type == "availability.confirmed_unavailable" {
				availability.FirstUnavailableAt, availability.LastUnavailableAt = &observedAt, &observedAt
			}
			if event.Type == "availability.restored" {
				availability.RestoredAt = &observedAt
			}
			state = models.ObserverSyncState{ObserverID: defaultObserverID, RemoteInstanceID: instanceID, LastEventID: event.ID, LastSuccessAt: &now, UpdatedAt: now}
			receipt := models.ObserverEventReceipt{ObserverID: defaultObserverID, RemoteInstanceID: instanceID, EventID: event.ID, PID: event.PID, EventType: event.Type, PayloadHash: hex.EncodeToString(digest[:]), AppliedAt: now}
			applied, applyErr := s.repository.ApplyObserverEvent(ctx, state, receipt, availability)
			if applyErr != nil {
				return result, applyErr
			}
			if applied {
				result.EventsApplied++
			}
			after = event.ID
			result.LastEventID = after
		}
		if page.NextAfter < after {
			return result, errors.New("Observer sync cursor moved backwards")
		}
		if page.NextAfter > after && len(page.Events) == 0 {
			after = page.NextAfter
		}
		if !page.HasMore {
			now := time.Now().UTC()
			state = models.ObserverSyncState{ObserverID: defaultObserverID, RemoteInstanceID: instanceID, LastEventID: after, LastSuccessAt: &now, UpdatedAt: now}
			if err = s.repository.SaveObserverSyncState(ctx, state); err != nil {
				return result, err
			}
			result.LastEventID, result.CompletedAt = after, now
			return result, nil
		}
	}
	return result, errors.New("Observer sync exceeded the page safety limit")
}

func parseObserverEventTime(primary, fallback string) (time.Time, error) {
	for _, raw := range []string{primary, fallback} {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, errors.New("Observer event timestamp is invalid")
}

func (s *ObserverService) Removed(ctx context.Context, state, query string, cursor, limit int) (ObserverRemovedPage, error) {
	rows, hasMore, err := s.repository.ListPostAvailabilities(ctx, state, query, cursor, limit)
	if err != nil {
		return ObserverRemovedPage{}, err
	}
	items := make([]ObserverRemovedItem, 0, len(rows))
	for _, row := range rows {
		post, _, _, contentErr := s.repository.ObserverPostContents(ctx, row.PID)
		if contentErr != nil {
			return ObserverRemovedPage{}, contentErr
		}
		items = append(items, ObserverRemovedItem{PID: row.PID, State: row.State, ObservedAt: row.ObservedAt, FirstUnavailableAt: row.FirstUnavailableAt, LastUnavailableAt: row.LastUnavailableAt, RestoredAt: row.RestoredAt, ObserverID: row.ObserverID, Completeness: row.Completeness, Post: post})
	}
	next := 0
	if hasMore {
		next = cursor + len(items)
	}
	return ObserverRemovedPage{Items: items, NextCursor: next, HasMore: hasMore}, nil
}

func (s *ObserverService) RemovedDetail(ctx context.Context, pid int32) (ObserverRemovedDetail, bool, error) {
	availability, found, err := s.repository.GetPostAvailability(ctx, pid)
	if err != nil || !found {
		return ObserverRemovedDetail{}, found, err
	}
	post, comments, media, err := s.repository.ObserverPostContents(ctx, pid)
	if err != nil {
		return ObserverRemovedDetail{}, false, err
	}
	if comments == nil {
		comments = []models.Comment{}
	}
	if media == nil {
		media = []models.Media{}
	}
	return ObserverRemovedDetail{Availability: availability, Post: post, Comments: comments, Media: media}, true, nil
}

func (s *ObserverService) SubmitChallenge(ctx context.Context, code string) (ObserverStatusResult, error) {
	if strings.TrimSpace(code) == "" {
		return ObserverStatusResult{}, errors.New("verification code is required")
	}
	var ignored map[string]any
	if err := s.client.DoJSON(ctx, http.MethodPost, "/api/v1/auth/challenge/submit", map[string]string{"code": strings.TrimSpace(code)}, &ignored); err != nil {
		return ObserverStatusResult{}, err
	}
	return s.Status(ctx)
}

func (s *ObserverService) ResendChallenge(ctx context.Context) (ObserverStatusResult, error) {
	var ignored map[string]any
	if err := s.client.DoJSON(ctx, http.MethodPost, "/api/v1/auth/challenge/resend", nil, &ignored); err != nil {
		return ObserverStatusResult{}, err
	}
	return s.Status(ctx)
}

func (s *ObserverService) RetryAuth(ctx context.Context) (ObserverStatusResult, error) {
	var ignored map[string]any
	if err := s.client.DoJSON(ctx, http.MethodPost, "/api/v1/auth/retry", nil, &ignored); err != nil {
		return ObserverStatusResult{}, err
	}
	return s.Status(ctx)
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}
func nestedStringValue(values map[string]any, object, key string) string {
	nested, _ := values[object].(map[string]any)
	return stringValue(nested, key)
}
func boolValue(values map[string]any, key string) bool { value, _ := values[key].(bool); return value }
func intValue(values map[string]any, key string) int {
	switch value := values[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	}
	return 0
}
