package ai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

const (
	ModeSelected = "selected"
	ModeLocal    = "local"
	ModeCourse   = "course"
)

type Store interface {
	CreateAISession(ctx context.Context, session models.AISession) error
	ListAISessions(ctx context.Context, limit int) ([]models.AISession, error)
	GetAISession(ctx context.Context, id string) (models.AISession, error)
	ListAIMessages(ctx context.Context, sessionID string) ([]models.AIMessage, error)
	ListAISources(ctx context.Context, messageID string) ([]models.AISource, error)
	SaveAIMessage(ctx context.Context, message models.AIMessage, sources []models.AISource) error
}

type SessionDetail struct {
	Session  models.AISession `json:"session"`
	Messages []MessageDetail  `json:"messages"`
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
	cancel      context.CancelFunc
	history     []service.AIEvent
	subscribers map[uint64]chan service.AIEvent
	done        bool
}

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
	return &Service{rootCtx: ctx, store: store, posts: posts, search: search, runtimes: map[string]providerRuntime{id: {provider: provider, config: cfg, info: info}}, activeID: id, config: cfg, runs: make(map[string]*runState)}
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
		info.Configured = cfg.Enabled
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
	for _, runtime := range s.runtimes {
		if strings.EqualFold(runtime.info.Name, session.Provider) {
			runtime.info.Model = session.Model
			return runtime, true
		}
	}
	return providerRuntime{}, false
}

func (s *Service) CreateSession(ctx context.Context, mode, title string) (models.AISession, error) {
	if s == nil || s.store == nil {
		return models.AISession{}, errors.New("AI store is unavailable")
	}
	if !validMode(mode) {
		return models.AISession{}, fmt.Errorf("unsupported AI mode %q", mode)
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
	session := models.AISession{ID: newAIID(), Title: title, Mode: mode, Provider: runtime.info.Name, Model: runtime.info.Model, CreatedAt: now, UpdatedAt: now}
	if err := s.store.CreateAISession(ctx, session); err != nil {
		return models.AISession{}, err
	}
	return session, nil
}

func (s *Service) ListSessions(ctx context.Context, limit int) ([]models.AISession, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("AI store is unavailable")
	}
	return s.store.ListAISessions(ctx, limit)
}

func (s *Service) GetSession(ctx context.Context, id string) (SessionDetail, error) {
	if s == nil || s.store == nil {
		return SessionDetail{}, errors.New("AI store is unavailable")
	}
	session, err := s.store.GetAISession(ctx, id)
	if err != nil {
		return SessionDetail{}, err
	}
	messages, err := s.store.ListAIMessages(ctx, id)
	if err != nil {
		return SessionDetail{}, err
	}
	detail := SessionDetail{Session: session, Messages: make([]MessageDetail, len(messages))}
	for i, message := range messages {
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
	if request.Mode == "" {
		request.Mode = session.Mode
	}
	if request.Mode != session.Mode || !validMode(request.Mode) {
		return nil, errors.New("message mode does not match the session")
	}
	runtime, ok := s.sessionRuntime(session)
	if !ok || runtime.provider == nil || !runtime.info.Configured {
		return nil, errors.New("the provider used by this session is no longer available; create a new session")
	}
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
	state := &runState{cancel: cancel, subscribers: make(map[uint64]chan service.AIEvent)}
	s.runs[session.ID] = state
	channel := s.subscribeLocked(session.ID, state, nil, nil)
	s.wg.Add(1)
	s.mu.Unlock()
	now := time.Now().UTC()
	if err := s.store.SaveAIMessage(ctx, models.AIMessage{ID: newAIID(), SessionID: session.ID, Role: "user", Content: request.Prompt, Provider: runtime.info.Name, Model: runtime.info.Model, Mode: request.Mode, CreatedAt: now}, nil); err != nil {
		s.finish(session.ID)
		s.wg.Done()
		return nil, err
	}

	go func() {
		defer s.wg.Done()
		s.execute(runCtx, session, request, runtime)
	}()
	return channel, nil
}

func (s *Service) Events(ctx context.Context, sessionID string) (<-chan service.AIEvent, error) {
	s.mu.Lock()
	state := s.runs[sessionID]
	if state == nil {
		s.mu.Unlock()
		return nil, errors.New("AI run was not found")
	}
	history := append([]service.AIEvent(nil), state.history...)
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

func (s *Service) execute(ctx context.Context, session models.AISession, request service.AIRequest, runtime providerRuntime) {
	s.emit(session.ID, service.AIEvent{Type: "started", Data: map[string]any{"mode": request.Mode, "model": runtime.info.Model, "provider": runtime.info.Name}})
	answer, trace, sources, err := s.runWorkflow(ctx, request, runtime)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.emit(session.ID, service.AIEvent{Type: "cancelled", Data: map[string]any{}})
		} else {
			s.emit(session.ID, service.AIEvent{Type: "error", Data: map[string]any{"message": err.Error()}})
		}
		s.finish(session.ID)
		return
	}
	traceJSON, _ := json.Marshal(trace)
	messageID := newAIID()
	rows := make([]models.AISource, len(sources))
	for i, source := range sources {
		rows[i] = models.AISource{MessageID: messageID, Ordinal: i, PID: source.PID, CID: source.CID, Snippet: source.Snippet}
	}
	message := models.AIMessage{ID: messageID, SessionID: session.ID, Role: "assistant", Content: answer, Provider: runtime.info.Name, Model: runtime.info.Model, Mode: request.Mode, TraceJSON: string(traceJSON), CreatedAt: time.Now().UTC()}
	if err := s.store.SaveAIMessage(context.Background(), message, rows); err != nil {
		s.emit(session.ID, service.AIEvent{Type: "error", Data: map[string]any{"message": err.Error()}})
		s.finish(session.ID)
		return
	}
	for _, source := range sources {
		s.emit(session.ID, service.AIEvent{Type: "source", Data: source})
	}
	s.emit(session.ID, service.AIEvent{Type: "completed", Data: map[string]any{"message_id": messageID, "sources": len(sources)}})
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
	defer s.mu.Unlock()
	state := s.runs[sessionID]
	if state == nil || state.done {
		return
	}
	state.history = append(state.history, event)
	for _, subscriber := range state.subscribers {
		select {
		case subscriber <- event:
		default:
		}
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
