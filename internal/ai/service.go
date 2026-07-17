package ai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

const (
	ModeSelected  = "selected"
	ModeLocal     = "local"
	ModeCourse    = "course"
	promptVersion = "research-v2"
)

type Store interface {
	CreateAISession(ctx context.Context, session models.AISession) error
	ListAISessions(ctx context.Context, limit int) ([]models.AISession, error)
	GetAISession(ctx context.Context, id string) (models.AISession, error)
	UpdateAISessionScope(ctx context.Context, id, scopeJSON string) error
	ListAIMessages(ctx context.Context, sessionID string) ([]models.AIMessage, error)
	ListAISources(ctx context.Context, messageID string) ([]models.AISource, error)
	SaveAIMessage(ctx context.Context, message models.AIMessage, sources []models.AISource) error
	CreateAIRun(ctx context.Context, run models.AIRun) error
	UpdateAIRun(ctx context.Context, id, status, message string, finished bool) error
	AppendAIEvent(ctx context.Context, event models.AIEventRecord) error
	ListAIEvents(ctx context.Context, runID string, afterSequence int64) ([]models.AIEventRecord, error)
	LatestAIRun(ctx context.Context, sessionID string) (*models.AIRun, error)
	SaveAIQueries(ctx context.Context, runID string, queries []models.AIQuery) error
	ListAIQueries(ctx context.Context, runID string) ([]models.AIQuery, error)
	RecoverRunningAIRuns(ctx context.Context) error
}

type SessionDetail struct {
	Session   models.AISession `json:"session"`
	Messages  []MessageDetail  `json:"messages"`
	LatestRun *models.AIRun    `json:"latest_run,omitempty"`
}

type MessageDetail struct {
	models.AIMessage
	Sources []models.AISource `json:"sources"`
}

type Service struct {
	rootCtx   context.Context
	store     Store
	posts     *service.PostService
	search    *service.SearchService
	runtimeMu sync.RWMutex
	runtimes  map[string]providerRuntime
	activeID  string
	config    config.AIConfig

	mu     sync.Mutex
	wg     sync.WaitGroup
	runs   map[string]*runState
	nextID uint64
	closed bool
}

type providerRuntime struct {
	provider AIProvider
	config   config.AIConfig
	info     ProviderInfo
}

type runState struct {
	runID        string
	cancel       context.CancelFunc
	history      []service.AIEvent
	subscribers  map[uint64]chan service.AIEvent
	nextSequence int64
	done         bool
}

var answerCitationPattern = regexp.MustCompile(`\[#([0-9]+)(?:/C([0-9]+))?\]`)

func NewService(ctx context.Context, store Store, posts *service.PostService, search *service.SearchService, provider AIProvider, cfg config.AIConfig, info ProviderInfo) *Service {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.MaxSearchRounds <= 0 || cfg.MaxSearchRounds > 5 {
		cfg.MaxSearchRounds = 5
	}
	if cfg.Provider.MaxOutputTokens <= 0 {
		cfg.Provider.MaxOutputTokens = 4096
	}
	id := strings.TrimSpace(info.ID)
	if id == "" {
		id = strings.TrimSpace(cfg.ActiveProvider)
	}
	if id == "" {
		id = "default"
	}
	info.ID, info.Active = id, true
	result := &Service{rootCtx: ctx, store: store, posts: posts, search: search, runtimes: map[string]providerRuntime{id: {provider: provider, config: cfg, info: info}}, activeID: id, config: cfg, runs: make(map[string]*runState)}
	if store != nil {
		_ = store.RecoverRunningAIRuns(ctx)
	}
	return result
}

func (s *Service) Providers() []ProviderInfo {
	if s == nil {
		return []ProviderInfo{}
	}
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	result := make([]ProviderInfo, 0, len(s.runtimes))
	for id, runtime := range s.runtimes {
		info := runtime.info
		info.Active = id == s.activeID
		result = append(result, info)
	}
	slices.SortFunc(result, func(a, b ProviderInfo) int { return strings.Compare(a.Name, b.Name) })
	return result
}

func (s *Service) LiveSearchEnabled() bool {
	if s == nil {
		return false
	}
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return s.config.AllowLiveSearch
}

func (s *Service) Reconfigure(cfg config.AIConfig) error {
	if s == nil {
		return errors.New("AI service is unavailable")
	}
	config.NormalizeAIProviders(&cfg)
	next := make(map[string]providerRuntime, len(cfg.Providers))
	for _, providerConfig := range cfg.Providers {
		if providerConfig.ID == cfg.ActiveProvider {
			if key := strings.TrimSpace(os.Getenv("PKUHOLE_AI_API_KEY")); key != "" {
				providerConfig.APIKey = key
			}
		}
		provider, err := NewOpenAIProvider(providerConfig)
		if err != nil {
			return fmt.Errorf("configure provider %q: %w", providerConfig.Name, err)
		}
		providerCfg := cfg
		providerCfg.Provider = providerConfig
		info := provider.Info()
		info.ID = providerConfig.ID
		info.Configured = cfg.Enabled && (info.Configured || isLoopbackProvider(providerConfig.BaseURL))
		info.Active = providerConfig.ID == cfg.ActiveProvider
		next[providerConfig.ID] = providerRuntime{provider: provider, config: providerCfg, info: info}
	}
	if _, ok := next[cfg.ActiveProvider]; !ok {
		return errors.New("active AI provider is unavailable")
	}
	s.runtimeMu.Lock()
	s.runtimes, s.activeID, s.config = next, cfg.ActiveProvider, cfg
	s.runtimeMu.Unlock()
	return nil
}

func isLoopbackProvider(baseURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Service) TestProvider(ctx context.Context, id string) (ProviderProbe, error) {
	if s == nil {
		return ProviderProbe{}, errors.New("AI service is unavailable")
	}
	s.runtimeMu.RLock()
	runtime, ok := s.runtimes[strings.TrimSpace(id)]
	s.runtimeMu.RUnlock()
	if !ok || runtime.provider == nil {
		return ProviderProbe{}, errors.New("AI provider was not found")
	}
	started := time.Now()
	_, err := runtime.provider.Chat(ctx, ChatRequest{Model: runtime.info.Model, Messages: []ChatMessage{{Role: "user", Content: "Reply with OK."}}, Temperature: 0, MaxOutputTokens: 8})
	if err != nil {
		message := err.Error()
		if key := strings.TrimSpace(runtime.config.Provider.APIKey); key != "" {
			message = strings.ReplaceAll(message, key, "<redacted>")
		}
		return ProviderProbe{}, errors.New(message)
	}
	return ProviderProbe{ProviderID: runtime.info.ID, Provider: runtime.info.Name, Model: runtime.info.Model, Reachable: true, LatencyMS: time.Since(started).Milliseconds()}, nil
}

func (s *Service) activeRuntime() (providerRuntime, bool) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	runtime, ok := s.runtimes[s.activeID]
	return runtime, ok
}

func (s *Service) sessionRuntime(session models.AISession) (providerRuntime, bool) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	if session.ProviderID != "" {
		if runtime, ok := s.runtimes[session.ProviderID]; ok {
			runtime.info.Model = session.Model
			return runtime, true
		}
	}
	for _, runtime := range s.runtimes {
		if strings.EqualFold(runtime.info.Name, session.Provider) {
			runtime.info.Model = session.Model
			return runtime, true
		}
	}
	return providerRuntime{}, false
}

func (s *Service) CreateSession(ctx context.Context, mode, title string, scope models.AIScope) (models.AISession, error) {
	if s == nil || s.store == nil {
		return models.AISession{}, errors.New("AI store is unavailable")
	}
	if !validMode(mode) {
		return models.AISession{}, fmt.Errorf("unsupported AI mode %q", mode)
	}
	scope, err := normalizeScope(mode, scope)
	if err != nil {
		return models.AISession{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = modeTitle(mode)
	}
	runtime, ok := s.activeRuntime()
	if !ok || runtime.provider == nil || !runtime.info.Configured {
		return models.AISession{}, errors.New("AI provider is not configured")
	}
	now := time.Now().UTC()
	scopeJSON, _ := json.Marshal(scope)
	session := models.AISession{ID: newAIID(), Title: title, Mode: mode, ProviderID: runtime.info.ID, Provider: runtime.info.Name, Model: runtime.info.Model, ScopeJSON: string(scopeJSON), Scope: scope, CreatedAt: now, UpdatedAt: now}
	if err := s.store.CreateAISession(ctx, session); err != nil {
		return models.AISession{}, err
	}
	return session, nil
}

func (s *Service) ListSessions(ctx context.Context, limit int) ([]models.AISession, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("AI store is unavailable")
	}
	sessions, err := s.store.ListAISessions(ctx, limit)
	if err != nil {
		return nil, err
	}
	for index := range sessions {
		decodeSessionScope(&sessions[index])
	}
	return sessions, nil
}

func (s *Service) GetSession(ctx context.Context, id string) (SessionDetail, error) {
	if s == nil || s.store == nil {
		return SessionDetail{}, errors.New("AI store is unavailable")
	}
	session, err := s.store.GetAISession(ctx, id)
	if err != nil {
		return SessionDetail{}, err
	}
	decodeSessionScope(&session)
	messages, err := s.store.ListAIMessages(ctx, id)
	if err != nil {
		return SessionDetail{}, err
	}
	latestRun, err := s.store.LatestAIRun(ctx, id)
	if err != nil {
		return SessionDetail{}, err
	}
	if latestRun != nil {
		latestRun.Queries, err = s.store.ListAIQueries(ctx, latestRun.ID)
		if err != nil {
			return SessionDetail{}, err
		}
	}
	detail := SessionDetail{Session: session, Messages: make([]MessageDetail, len(messages)), LatestRun: latestRun}
	for i, message := range messages {
		if strings.TrimSpace(message.EvidenceJSON) != "" {
			var report models.AIEvidenceReport
			if json.Unmarshal([]byte(message.EvidenceJSON), &report) == nil {
				message.Evidence = &report
			}
		}
		sources, sourceErr := s.store.ListAISources(ctx, message.ID)
		if sourceErr != nil {
			return SessionDetail{}, sourceErr
		}
		detail.Messages[i] = MessageDetail{AIMessage: message, Sources: sources}
	}
	return detail, nil
}

func (s *Service) Run(ctx context.Context, request service.AIRequest) (<-chan service.AIEvent, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("AI provider is not configured")
	}
	request.Prompt = strings.TrimSpace(request.Prompt)
	if request.SessionID == "" || request.Prompt == "" {
		return nil, errors.New("session ID and prompt are required")
	}
	session, err := s.store.GetAISession(ctx, request.SessionID)
	if err != nil {
		return nil, err
	}
	decodeSessionScope(&session)
	if request.Mode == "" {
		request.Mode = session.Mode
	}
	if request.Mode != session.Mode || !validMode(request.Mode) {
		return nil, errors.New("message mode does not match the session")
	}
	resolvedScope, err := resolveScope(request.Mode, session.Scope, request)
	if err != nil {
		return nil, err
	}
	request.PIDs, request.Course, request.Teachers = resolvedScope.PIDs, resolvedScope.Course, resolvedScope.Teachers
	request.From, request.To, request.TagIDs, request.Origins, request.HasMedia = resolvedScope.From, resolvedScope.To, resolvedScope.TagIDs, resolvedScope.Origins, resolvedScope.HasMedia
	if !scopesEqual(session.Scope, resolvedScope) {
		scopeJSON, _ := json.Marshal(resolvedScope)
		if err := s.store.UpdateAISessionScope(ctx, session.ID, string(scopeJSON)); err != nil {
			return nil, err
		}
		session.Scope, session.ScopeJSON = resolvedScope, string(scopeJSON)
	}
	historyRows, err := s.store.ListAIMessages(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	history := boundedConversationHistory(historyRows)
	messageOrdinal := nextMessageOrdinal(historyRows)
	runtime, ok := s.sessionRuntime(session)
	if !ok || runtime.provider == nil || !runtime.info.Configured {
		return nil, errors.New("the provider used by this session is no longer available; create a new session")
	}
	now := time.Now().UTC()
	scopeJSON, _ := json.Marshal(resolvedScope)
	configSnapshot, _ := json.Marshal(map[string]any{
		"allow_live_search": runtime.config.AllowLiveSearch,
		"max_search_rounds": runtime.config.MaxSearchRounds,
		"temperature":       runtime.config.Provider.Temperature,
		"max_output_tokens": runtime.config.Provider.MaxOutputTokens,
		"request_timeout":   runtime.config.Provider.RequestTimeout,
	})
	run := models.AIRun{ID: newAIID(), SessionID: session.ID, Status: "running", Prompt: request.Prompt, ScopeJSON: string(scopeJSON), ProviderID: runtime.info.ID, Provider: runtime.info.Name, Model: runtime.info.Model, PromptVersion: promptVersion, ConfigJSON: string(configSnapshot), StartedAt: &now, CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("AI service is closed")
	}
	if current := s.runs[session.ID]; current != nil && !current.done {
		s.mu.Unlock()
		return nil, errors.New("an AI run is already active for this session")
	}
	runCtx, cancel := context.WithCancel(s.rootCtx)
	state := &runState{runID: run.ID, cancel: cancel, subscribers: make(map[uint64]chan service.AIEvent)}
	s.runs[session.ID] = state
	s.mu.Unlock()
	if err := s.store.CreateAIRun(ctx, run); err != nil {
		cancel()
		s.mu.Lock()
		if s.runs[session.ID] == state {
			delete(s.runs, session.ID)
		}
		s.mu.Unlock()
		return nil, err
	}
	s.mu.Lock()
	channel := s.subscribeLocked(session.ID, state, nil, nil)
	s.wg.Add(1)
	s.mu.Unlock()
	if err := s.store.SaveAIMessage(ctx, models.AIMessage{ID: newAIID(), SessionID: session.ID, RunID: run.ID, Ordinal: messageOrdinal, Role: "user", Content: request.Prompt, Provider: runtime.info.Name, Model: runtime.info.Model, Mode: request.Mode, CreatedAt: now}, nil); err != nil {
		_ = s.store.UpdateAIRun(context.Background(), run.ID, "failed", err.Error(), true)
		s.finish(session.ID)
		s.wg.Done()
		return nil, err
	}

	go func() {
		defer s.wg.Done()
		s.execute(runCtx, run.ID, session, request, history, messageOrdinal+1, runtime)
	}()
	return channel, nil
}

func (s *Service) Events(ctx context.Context, sessionID string) (<-chan service.AIEvent, error) {
	return s.EventsAfter(ctx, sessionID, 0)
}

func (s *Service) EventsAfter(ctx context.Context, sessionID string, afterSequence int64) (<-chan service.AIEvent, error) {
	s.mu.Lock()
	state := s.runs[sessionID]
	if state == nil {
		s.mu.Unlock()
		run, err := s.store.LatestAIRun(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		if run == nil {
			return nil, errors.New("AI run was not found")
		}
		rows, err := s.store.ListAIEvents(ctx, run.ID, afterSequence)
		if err != nil {
			return nil, err
		}
		channel := make(chan service.AIEvent, len(rows))
		for _, row := range rows {
			var data any
			_ = json.Unmarshal([]byte(row.DataJSON), &data)
			channel <- service.AIEvent{Sequence: row.Sequence, Type: row.Type, Data: data}
		}
		close(channel)
		return channel, nil
	}
	history := make([]service.AIEvent, 0, len(state.history))
	for _, event := range state.history {
		if event.Sequence > afterSequence {
			history = append(history, event)
		}
	}
	done := state.done
	if done {
		channel := make(chan service.AIEvent, len(history))
		for _, event := range history {
			channel <- event
		}
		close(channel)
		s.mu.Unlock()
		return channel, nil
	}
	channel := s.subscribeLocked(sessionID, state, ctx, history)
	s.mu.Unlock()
	return channel, nil
}

func (s *Service) Cancel(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.runs[sessionID]
	if state == nil || state.done {
		return errors.New("no active AI run for this session")
	}
	state.cancel()
	return nil
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	for _, state := range s.runs {
		if state != nil && !state.done {
			state.cancel()
		}
	}
	s.mu.Unlock()
	s.wg.Wait()
	return nil
}

func (s *Service) execute(ctx context.Context, runID string, session models.AISession, request service.AIRequest, history []ChatMessage, messageOrdinal int64, runtime providerRuntime) {
	s.emit(session.ID, service.AIEvent{Type: "started", Data: map[string]any{"mode": request.Mode, "model": runtime.info.Model, "provider": runtime.info.Name}})
	answer, trace, sources, err := s.runWorkflow(ctx, request, history, runtime)
	queryRows := make([]models.AIQuery, len(trace))
	for index, item := range trace {
		queryRows[index] = models.AIQuery{Round: item.Round, Tool: item.Tool, Query: item.Query, Reason: item.Reason, Matches: item.Matches}
	}
	if queryErr := s.store.SaveAIQueries(context.Background(), runID, queryRows); queryErr != nil && err == nil {
		err = queryErr
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.updateRun(session.ID, "cancelled", "", true)
			s.emit(session.ID, service.AIEvent{Type: "cancelled", Data: map[string]any{}})
		} else {
			message := redactRuntimeError(err, runtime)
			s.updateRun(session.ID, "failed", message, true)
			s.emit(session.ID, service.AIEvent{Type: "error", Data: map[string]any{"message": message}})
		}
		s.finish(session.ID)
		return
	}
	sources, err = validateAnswerSources(answer, sources)
	if err != nil {
		s.updateRun(session.ID, "failed", err.Error(), true)
		s.emit(session.ID, service.AIEvent{Type: "error", Data: map[string]any{"message": err.Error()}})
		s.finish(session.ID)
		return
	}
	report := buildEvidenceReport(answer, sources)
	s.emit(session.ID, service.AIEvent{Type: "evidence_check_started", Data: map[string]any{"claims": report.Summary.Total, "cited": report.Summary.Cited}})
	verifiedReport, verificationErr := verifyEvidenceReport(ctx, report, sources, runtime)
	if verificationErr != nil {
		for index := range report.Claims {
			if report.Claims[index].Status == claimStatusUnverified {
				report.Claims[index].Reason = "引用已绑定，但本轮语义核对未完成"
			}
		}
		refreshEvidenceSummary(&report)
		s.emit(session.ID, service.AIEvent{Type: "evidence_check_failed", Data: map[string]any{"message": "语义核对未完成，已保留引用绑定结果"}})
	} else {
		report = verifiedReport
	}
	traceJSON, _ := json.Marshal(trace)
	evidenceJSON, _ := json.Marshal(report)
	messageID := newAIID()
	rows := make([]models.AISource, len(sources))
	for i, source := range sources {
		rows[i] = models.AISource{MessageID: messageID, Ordinal: i, Origin: source.Origin, PID: source.PID, CID: source.CID, Snippet: source.Snippet}
	}
	message := models.AIMessage{ID: messageID, SessionID: session.ID, RunID: runID, Ordinal: messageOrdinal, Role: "assistant", Content: answer, Provider: runtime.info.Name, Model: runtime.info.Model, Mode: request.Mode, TraceJSON: string(traceJSON), EvidenceJSON: string(evidenceJSON), Evidence: &report, CreatedAt: time.Now().UTC()}
	if err := s.store.SaveAIMessage(context.Background(), message, rows); err != nil {
		s.updateRun(session.ID, "failed", err.Error(), true)
		s.emit(session.ID, service.AIEvent{Type: "error", Data: map[string]any{"message": err.Error()}})
		s.finish(session.ID)
		return
	}
	for _, source := range sources {
		s.emit(session.ID, service.AIEvent{Type: "source", Data: source})
	}
	s.emit(session.ID, service.AIEvent{Type: "evidence_report", Data: report})
	s.updateRun(session.ID, "completed", "", true)
	s.emit(session.ID, service.AIEvent{Type: "completed", Data: map[string]any{"message_id": messageID, "sources": len(sources), "claims": report.Summary.Total, "citation_coverage": report.Summary.CitationCoverage}})
	s.finish(session.ID)
}

func (s *Service) subscribeLocked(sessionID string, state *runState, ctx context.Context, replay []service.AIEvent) chan service.AIEvent {
	s.nextID++
	id := s.nextID
	channel := make(chan service.AIEvent, len(replay)+4096)
	state.subscribers[id] = channel
	for _, event := range replay {
		channel <- event
	}
	if ctx != nil {
		go func() {
			<-ctx.Done()
			s.mu.Lock()
			if current := s.runs[sessionID]; current != nil {
				delete(current.subscribers, id)
			}
			s.mu.Unlock()
		}()
	}
	return channel
}

func (s *Service) emit(sessionID string, event service.AIEvent) {
	s.mu.Lock()
	state := s.runs[sessionID]
	if state == nil || state.done {
		s.mu.Unlock()
		return
	}
	state.nextSequence++
	event.Sequence = state.nextSequence
	state.history = append(state.history, event)
	for _, subscriber := range state.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
	runID := state.runID
	s.mu.Unlock()
	if event.Type != "delta" && runID != "" {
		encoded, _ := json.Marshal(event.Data)
		_ = s.store.AppendAIEvent(context.Background(), models.AIEventRecord{RunID: runID, Sequence: event.Sequence, Type: event.Type, DataJSON: string(encoded), CreatedAt: time.Now().UTC()})
	}
}

func (s *Service) updateRun(sessionID, status, message string, finished bool) {
	s.mu.Lock()
	state := s.runs[sessionID]
	runID := ""
	if state != nil {
		runID = state.runID
	}
	s.mu.Unlock()
	if runID != "" {
		_ = s.store.UpdateAIRun(context.Background(), runID, status, message, finished)
	}
}

func (s *Service) finish(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.runs[sessionID]
	if state == nil || state.done {
		return
	}
	state.done = true
	state.cancel()
	for id, subscriber := range state.subscribers {
		close(subscriber)
		delete(state.subscribers, id)
	}
}

func validMode(mode string) bool {
	return mode == ModeSelected || mode == ModeLocal || mode == ModeCourse
}

func decodeSessionScope(session *models.AISession) {
	if session == nil || strings.TrimSpace(session.ScopeJSON) == "" {
		return
	}
	_ = json.Unmarshal([]byte(session.ScopeJSON), &session.Scope)
}

func normalizeScope(mode string, scope models.AIScope) (models.AIScope, error) {
	result := models.AIScope{}
	switch mode {
	case ModeSelected:
		seen := make(map[int32]bool)
		for _, pid := range scope.PIDs {
			if pid <= 0 {
				return models.AIScope{}, errors.New("pids must contain positive integers")
			}
			if !seen[pid] {
				seen[pid] = true
				result.PIDs = append(result.PIDs, pid)
			}
			if len(result.PIDs) > 100 {
				return models.AIScope{}, errors.New("selected mode accepts at most 100 PIDs")
			}
		}
	case ModeCourse:
		result.Course = strings.TrimSpace(scope.Course)
		seen := make(map[string]bool)
		for _, teacher := range scope.Teachers {
			teacher = strings.TrimSpace(teacher)
			if teacher == "" || seen[teacher] {
				continue
			}
			seen[teacher] = true
			result.Teachers = append(result.Teachers, teacher)
			if len(result.Teachers) == 10 {
				break
			}
		}
	case ModeLocal:
		// Local research starts with the full searchable archive.
	default:
		return models.AIScope{}, fmt.Errorf("unsupported AI mode %q", mode)
	}
	if mode == ModeLocal || mode == ModeCourse {
		result.From, result.To, result.HasMedia = scope.From, scope.To, scope.HasMedia
		if result.From < 0 || result.To < 0 || (result.From > 0 && result.To > 0 && result.From > result.To) {
			return models.AIScope{}, errors.New("research time range is invalid")
		}
		seenTags := make(map[uint]bool)
		for _, tagID := range scope.TagIDs {
			if tagID == 0 || seenTags[tagID] {
				continue
			}
			seenTags[tagID] = true
			result.TagIDs = append(result.TagIDs, tagID)
			if len(result.TagIDs) > 50 {
				return models.AIScope{}, errors.New("research scope accepts at most 50 local tags")
			}
		}
		seenOrigins := make(map[string]bool)
		for _, origin := range scope.Origins {
			origin = strings.TrimSpace(origin)
			if origin == "" || seenOrigins[origin] {
				continue
			}
			seenOrigins[origin] = true
			result.Origins = append(result.Origins, origin)
			if len(result.Origins) > 20 {
				return models.AIScope{}, errors.New("research scope accepts at most 20 archive origins")
			}
		}
	}
	return result, nil
}

func resolveScope(mode string, stored models.AIScope, request service.AIRequest) (models.AIScope, error) {
	next := stored
	if request.ReplaceScope {
		next = models.AIScope{}
	}
	switch mode {
	case ModeSelected:
		if len(request.PIDs) > 0 {
			next.PIDs = request.PIDs
		}
	case ModeCourse:
		if strings.TrimSpace(request.Course) != "" {
			next.Course = request.Course
		}
		if len(request.Teachers) > 0 {
			next.Teachers = request.Teachers
		}
	}
	if mode == ModeLocal || mode == ModeCourse {
		if request.From > 0 {
			next.From = request.From
		}
		if request.To > 0 {
			next.To = request.To
		}
		if len(request.TagIDs) > 0 {
			next.TagIDs = request.TagIDs
		}
		if len(request.Origins) > 0 {
			next.Origins = request.Origins
		}
		if request.HasMedia != nil {
			next.HasMedia = request.HasMedia
		}
	}
	return normalizeScope(mode, next)
}

func scopesEqual(left, right models.AIScope) bool {
	return slices.Equal(left.PIDs, right.PIDs) && left.Course == right.Course && slices.Equal(left.Teachers, right.Teachers) && left.From == right.From && left.To == right.To && slices.Equal(left.TagIDs, right.TagIDs) && slices.Equal(left.Origins, right.Origins) && equalOptionalBool(left.HasMedia, right.HasMedia)
}

func equalOptionalBool(left, right *bool) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func boundedConversationHistory(rows []models.AIMessage) []ChatMessage {
	const maxMessages = 12
	const maxCharacters = 24_000
	start := len(rows) - maxMessages
	if start < 0 {
		start = 0
	}
	result := make([]ChatMessage, 0, len(rows)-start)
	used := 0
	for index := len(rows) - 1; index >= start; index-- {
		row := rows[index]
		if row.Role != "user" && row.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(row.Content)
		if content == "" {
			continue
		}
		remaining := maxCharacters - used
		if remaining <= 0 {
			break
		}
		content = truncateRunes(content, remaining)
		used += len([]rune(content))
		result = append(result, ChatMessage{Role: row.Role, Content: content})
	}
	slices.Reverse(result)
	return result
}

func nextMessageOrdinal(rows []models.AIMessage) int64 {
	var maximum int64
	for _, row := range rows {
		if row.Ordinal > maximum {
			maximum = row.Ordinal
		}
	}
	if maximum == 0 {
		maximum = int64(len(rows))
	}
	return maximum + 1
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func validateAnswerSources(answer string, available []sourceRef) ([]sourceRef, error) {
	matches := answerCitationPattern.FindAllStringSubmatch(answer, -1)
	if len(matches) == 0 {
		return nil, errors.New("AI answer did not cite any retrieved PID/CID evidence")
	}
	result := make([]sourceRef, 0, len(matches))
	seen := make(map[string]bool)
	for _, match := range matches {
		pidValue, _ := strconv.ParseInt(match[1], 10, 32)
		pid := int32(pidValue)
		var cid *int32
		if match[2] != "" {
			cidValue, _ := strconv.ParseInt(match[2], 10, 32)
			parsed := int32(cidValue)
			cid = &parsed
		}
		found := false
		for _, source := range available {
			if source.PID != pid || (cid == nil) != (source.CID == nil) || (cid != nil && source.CID != nil && *cid != *source.CID) {
				continue
			}
			if source.Origin == "" {
				source.Origin = service.SourceLocal
			}
			key := source.Origin + ":" + strconv.FormatInt(int64(source.PID), 10)
			if source.CID != nil {
				key += ":" + strconv.FormatInt(int64(*source.CID), 10)
			}
			if !seen[key] {
				seen[key] = true
				result = append(result, source)
			}
			found = true
		}
		if !found {
			return nil, fmt.Errorf("AI answer cited unavailable evidence %s", match[0])
		}
	}
	return result, nil
}

func redactRuntimeError(err error, runtime providerRuntime) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if key := strings.TrimSpace(runtime.config.Provider.APIKey); key != "" {
		message = strings.ReplaceAll(message, key, "<redacted>")
	}
	return message
}

func modeTitle(mode string) string {
	switch mode {
	case ModeSelected:
		return "选中内容问答"
	case ModeCourse:
		return "课程分析"
	default:
		return "本地资料研究"
	}
}

func newAIID() string {
	buffer := make([]byte, 16)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}
