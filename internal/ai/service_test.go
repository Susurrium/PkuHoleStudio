package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

func TestLocalAgentSearchesStreamsAndPersistsSources(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{{Pid: 12345, Text: "alpha course experience", Timestamp: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertComments([]models.Comment{{Cid: 101, Pid: 12345, Text: "alpha homework was fair"}}); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	search := service.NewSearchService(posts, database)
	provider := &fakeProvider{
		chatResponses: []ChatResponse{
			{ToolCalls: []ToolCall{{
				ID: "call-1", Type: "function",
				Function: ToolCallFunction{Name: "search_archive", Arguments: `{"query":"alpha","reason":"find evidence","limit":5}`},
			}}},
			{ToolCalls: []ToolCall{{
				ID: "check-1", Type: "function",
				Function: ToolCallFunction{Name: "record_evidence_assessment", Arguments: `{"assessments":[{"ordinal":1,"status":"supported","reason":"引文直接支持该结论"}]}`},
			}}},
		},
		deltas: []string{"grounded ", "answer [#12345]"},
	}
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	cfg.MaxSearchRounds = 1
	aiService := NewService(context.Background(), database, posts, search, provider, cfg, ProviderInfo{Name: "fake", Model: "fake-model", Configured: true})
	session, err := aiService.CreateSession(context.Background(), ModeLocal, "test", models.AIScope{})
	if err != nil {
		t.Fatal(err)
	}
	events, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeLocal, Prompt: "how was alpha?"})
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]int{}
	for event := range events {
		types[event.Type]++
	}
	for _, eventType := range []string{"started", "search_started", "search_result", "delta", "evidence_check_started", "source", "evidence_report", "completed"} {
		if types[eventType] == 0 {
			t.Errorf("missing event %q in %#v", eventType, types)
		}
	}
	if provider.chatCalls != 2 || provider.streamCalls != 1 {
		t.Fatalf("provider calls = chat %d stream %d", provider.chatCalls, provider.streamCalls)
	}
	detail, err := aiService.GetSession(context.Background(), session.ID)
	if err != nil || len(detail.Messages) != 2 || detail.Messages[1].Content != "grounded answer [#12345]" || len(detail.Messages[1].Sources) == 0 || detail.Messages[1].Evidence == nil || detail.Messages[1].Evidence.Summary.Supported != 1 {
		t.Fatalf("session detail = %+v, %v", detail, err)
	}
	replay, err := aiService.Events(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	var replayed []string
	for event := range replay {
		replayed = append(replayed, event.Type)
	}
	if len(replayed) == 0 || replayed[0] != "started" || replayed[len(replayed)-1] != "completed" {
		t.Fatalf("replayed events = %v", replayed)
	}
}

func TestReconfigureSwitchesProvidersWithoutRestartAndAllowsKeylessServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" {
			t.Errorf("unexpected authorization header")
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"model":"local-model","choices":[{"message":{"role":"assistant","content":"OK"}}]}`))
	}))
	defer server.Close()
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	cfg.ActiveProvider = "one"
	cfg.Providers = []config.AIProviderConfig{
		{ID: "one", Name: "First", BaseURL: server.URL, Model: "first-model", RequestTimeout: 5, MaxOutputTokens: 32},
		{ID: "two", Name: "Second", BaseURL: server.URL, Model: "local-model", RequestTimeout: 5, MaxOutputTokens: 32},
	}
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), &fakeProvider{}, cfg, ProviderInfo{Name: "bootstrap", Model: "bootstrap", Configured: true})
	if err := aiService.Reconfigure(cfg); err != nil {
		t.Fatal(err)
	}
	first, err := aiService.CreateSession(context.Background(), ModeLocal, "first", models.AIScope{})
	if err != nil || first.Provider != "First" {
		t.Fatalf("first session = %+v, %v", first, err)
	}
	cfg.ActiveProvider = "two"
	if err := aiService.Reconfigure(cfg); err != nil {
		t.Fatal(err)
	}
	second, err := aiService.CreateSession(context.Background(), ModeLocal, "second", models.AIScope{})
	if err != nil || second.Provider != "Second" || second.Model != "local-model" {
		t.Fatalf("second session = %+v, %v", second, err)
	}
	probe, err := aiService.TestProvider(context.Background(), "two")
	if err != nil || !probe.Reachable || probe.Provider != "Second" {
		t.Fatalf("probe = %+v, %v", probe, err)
	}
	providers := aiService.Providers()
	if len(providers) != 2 || !providers[1].Active {
		t.Fatalf("providers = %+v", providers)
	}
}

func TestProviderProbeRedactsConfiguredAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Error(response, "rejected "+request.Header.Get("Authorization"), http.StatusUnauthorized)
	}))
	defer server.Close()
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	cfg.ActiveProvider = "secret"
	cfg.Providers = []config.AIProviderConfig{{ID: "secret", Name: "Secret", BaseURL: server.URL, APIKey: "do-not-leak", Model: "model", RequestTimeout: 5, MaxOutputTokens: 32}}
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), nil, cfg, ProviderInfo{})
	if err := aiService.Reconfigure(cfg); err != nil {
		t.Fatal(err)
	}
	_, err := aiService.TestProvider(context.Background(), "secret")
	if err == nil || strings.Contains(err.Error(), "do-not-leak") {
		t.Fatalf("probe error leaked API key: %v", err)
	}
}

func TestSelectedModeRequiresPIDs(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), &fakeProvider{deltas: []string{"unused"}}, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, _ := aiService.CreateSession(context.Background(), ModeSelected, "", models.AIScope{})
	events, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeSelected, Prompt: "question"})
	if err != nil {
		t.Fatal(err)
	}
	var sawError bool
	for event := range events {
		sawError = sawError || event.Type == "error"
	}
	if !sawError {
		t.Fatal("selected run did not report missing PIDs")
	}
}

func TestCourseModeBuildsAnalysisFromLocalEvidence(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{
		{Pid: 34567, Text: "alpha course with Professor Chen", Timestamp: 1},
		{Pid: 34568, Text: "alpha course with Professor Wang", Timestamp: 2},
		{Pid: 34569, Text: "alpha course with Professor Li", Timestamp: 3},
	}); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	provider := &fakeProvider{deltas: []string{"## 结论摘要\n课程评价存在个体差异。[#34567]\n## 课程难度\n有一定挑战。[#34567]\n## 教学\n需结合教师比较。[#34567]\n## 作业\n资料不足。\n## 考试\n资料不足。\n## 给分\n资料不足。\n## 教师比较\n| 教师 | 评价 |\n| --- | --- |\n| Professor Chen | 有本地资料 [#34567] |\n| Professor Wang | 资料不足 |\n| Professor Li | 资料不足 |\n## 证据不足与冲突\n当前样本有限。\n## 选课建议\n建议继续查看原帖。[#34567]"}}
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), provider, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, _ := aiService.CreateSession(context.Background(), ModeCourse, "", models.AIScope{})
	events, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeCourse, Prompt: "compare teaching", Course: "alpha", Teachers: []string{"Professor Chen", "Professor Wang", "Professor Li", "Professor Chen"}})
	if err != nil {
		t.Fatal(err)
	}
	var completed, searched bool
	for event := range events {
		completed = completed || event.Type == "completed"
		searched = searched || event.Type == "search_result"
	}
	if !completed || !searched || provider.streamCalls != 1 {
		t.Fatalf("completed=%v searched=%v streamCalls=%d", completed, searched, provider.streamCalls)
	}
	if len(provider.streamRequest.Messages) < 2 || !strings.Contains(provider.streamRequest.Messages[1].Content, "Professor Chen、Professor Wang、Professor Li") {
		t.Fatalf("course prompt omitted a teacher: %+v", provider.streamRequest.Messages)
	}
	detail, err := aiService.GetSession(context.Background(), session.ID)
	if err != nil || len(detail.Messages) != 2 || !strings.Contains(detail.Messages[1].TraceJSON, "Professor Li") || len(detail.Messages[1].Sources) != 1 || detail.Messages[1].Sources[0].PID != 34567 || detail.Messages[1].Sources[0].Origin != service.SourceLocal || detail.LatestRun == nil || detail.LatestRun.Status != "completed" {
		t.Fatalf("course detail = %+v, %v", detail, err)
	}
}

func TestCourseModeRetriesAndRejectsIncompleteReport(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{{Pid: 34570, Text: "beta course Professor Zhou", Timestamp: 1}}); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	provider := &fakeProvider{deltas: []string{"brief answer [#34570]"}}
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), provider, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, _ := aiService.CreateSession(context.Background(), ModeCourse, "", models.AIScope{Course: "beta", Teachers: []string{"Professor Zhou"}})
	events, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeCourse, Prompt: "compare"})
	if err != nil {
		t.Fatal(err)
	}
	var retried, failed bool
	for event := range events {
		retried = retried || event.Type == "validation_retry"
		failed = failed || event.Type == "error"
	}
	if !retried || !failed || provider.streamCalls != 2 {
		t.Fatalf("retried=%v failed=%v streamCalls=%d", retried, failed, provider.streamCalls)
	}
	detail, err := aiService.GetSession(context.Background(), session.ID)
	if err != nil || len(detail.Messages) != 1 || detail.LatestRun == nil || detail.LatestRun.Status != "failed" || len(detail.LatestRun.Queries) == 0 {
		t.Fatalf("course detail = %+v, %v", detail, err)
	}
}

func TestResolveScopeReplaceCanClearDurableFilters(t *testing.T) {
	withMedia := true
	stored := models.AIScope{From: 10, To: 20, TagIDs: []uint{1, 2}, Origins: []string{"legacy"}, HasMedia: &withMedia}
	merged, err := resolveScope(ModeLocal, stored, service.AIRequest{})
	if err != nil || !scopesEqual(merged, stored) {
		t.Fatalf("merged scope = %+v, %v", merged, err)
	}
	replaced, err := resolveScope(ModeLocal, stored, service.AIRequest{ReplaceScope: true})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.From != 0 || replaced.To != 0 || len(replaced.TagIDs) != 0 || len(replaced.Origins) != 0 || replaced.HasMedia != nil {
		t.Fatalf("replaced scope retained cleared filters: %+v", replaced)
	}
}

func TestSelectedScopePersistsAndFollowUpIncludesConversationHistory(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{{Pid: 45678, Text: "durable research scope", Timestamp: 1}}); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	provider := &fakeProvider{deltas: []string{"first answer [#45678]"}}
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), provider, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, err := aiService.CreateSession(context.Background(), ModeSelected, "durable", models.AIScope{PIDs: []int32{45678, 45678}})
	if err != nil || len(session.Scope.PIDs) != 1 {
		t.Fatalf("CreateSession() = %+v, %v", session, err)
	}
	first, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeSelected, Prompt: "first question"})
	if err != nil {
		t.Fatal(err)
	}
	for range first {
	}
	provider.deltas = []string{"follow-up answer [#45678]"}
	second, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeSelected, Prompt: "follow-up question"})
	if err != nil {
		t.Fatal(err)
	}
	for range second {
	}

	detail, err := aiService.GetSession(context.Background(), session.ID)
	if err != nil || len(detail.Session.Scope.PIDs) != 1 || detail.Session.Scope.PIDs[0] != 45678 || len(detail.Messages) != 4 {
		t.Fatalf("GetSession() = %+v, %v", detail, err)
	}
	joined := make([]string, len(provider.streamRequest.Messages))
	for index, message := range provider.streamRequest.Messages {
		joined[index] = message.Role + ":" + message.Content
	}
	history := strings.Join(joined, "\n")
	for _, expected := range []string{"user:first question", "assistant:first answer [#45678]", "follow-up question"} {
		if !strings.Contains(history, expected) {
			t.Fatalf("follow-up request omitted %q:\n%s", expected, history)
		}
	}
}

func TestSelectedResearchLoadsCommentsBeyondFirstPage(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	const pid int32 = 47890
	if err := database.UpsertPosts([]models.Post{{Pid: pid, Text: "long discussion", Timestamp: 1, Reply: 150}}); err != nil {
		t.Fatal(err)
	}
	comments := make([]models.Comment, 150)
	for index := range comments {
		comments[index] = models.Comment{Cid: int32(index + 1), Pid: pid, Text: fmt.Sprintf("comment evidence %d", index+1)}
	}
	if err := database.UpsertComments(comments); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	provider := &fakeProvider{deltas: []string{"late evidence [#47890/C150]"}}
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), provider, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, _ := aiService.CreateSession(context.Background(), ModeSelected, "long", models.AIScope{PIDs: []int32{pid}})
	events, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeSelected, Prompt: "find the late comment"})
	if err != nil {
		t.Fatal(err)
	}
	for range events {
	}
	if !strings.Contains(provider.streamRequest.Messages[len(provider.streamRequest.Messages)-1].Content, "[#47890/C150") {
		t.Fatal("selected research context omitted comments beyond the first page")
	}
	detail, err := aiService.GetSession(context.Background(), session.ID)
	if err != nil || len(detail.Messages) != 2 || len(detail.Messages[1].Sources) != 1 || detail.Messages[1].Sources[0].CID == nil || *detail.Messages[1].Sources[0].CID != 150 {
		t.Fatalf("late evidence detail = %+v, %v", detail, err)
	}
}

func TestLocalResearchEnforcesDurableTimeScope(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{
		{Pid: 48100, Text: "alpha outside scope", Timestamp: 100},
		{Pid: 48200, Text: "alpha inside scope", Timestamp: 200},
	}); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	cfg.MaxSearchRounds = 1
	provider := &fakeProvider{
		chat:   ChatResponse{ToolCalls: []ToolCall{{ID: "scope-search", Type: "function", Function: ToolCallFunction{Name: "search_archive", Arguments: `{"query":"alpha","reason":"scope test","limit":10}`}}}},
		deltas: []string{"scoped answer [#48200]"},
	}
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), provider, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, err := aiService.CreateSession(context.Background(), ModeLocal, "scoped", models.AIScope{From: 150, To: 250})
	if err != nil {
		t.Fatal(err)
	}
	events, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeLocal, Prompt: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	for range events {
	}
	detail, err := aiService.GetSession(context.Background(), session.ID)
	if err != nil || detail.Session.Scope.From != 150 || detail.Session.Scope.To != 250 || len(detail.Messages) != 2 || len(detail.Messages[1].Sources) != 1 || detail.Messages[1].Sources[0].PID != 48200 || detail.LatestRun == nil || len(detail.LatestRun.Queries) != 1 || detail.LatestRun.Queries[0].Matches != 1 {
		t.Fatalf("scoped detail = %+v, %v", detail, err)
	}
}

func TestUnavailableCitationFailsRunWithoutSavingAssistantMessage(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{{Pid: 56789, Text: "known evidence", Timestamp: 1}}); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), &fakeProvider{deltas: []string{"invented [#99999]"}}, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, _ := aiService.CreateSession(context.Background(), ModeSelected, "invalid citation", models.AIScope{PIDs: []int32{56789}})
	events, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeSelected, Prompt: "question"})
	if err != nil {
		t.Fatal(err)
	}
	var sawError bool
	for event := range events {
		sawError = sawError || event.Type == "error"
	}
	detail, err := aiService.GetSession(context.Background(), session.ID)
	if err != nil || !sawError || len(detail.Messages) != 1 || detail.LatestRun == nil || detail.LatestRun.Status != "failed" || !strings.Contains(detail.LatestRun.Error, "unavailable evidence") {
		t.Fatalf("invalid citation result = %+v, sawError=%v, err=%v", detail, sawError, err)
	}
}

func TestNewServiceMarksRunningResearchAsInterruptedAndReplaysEvent(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	now := time.Now().UTC()
	session := models.AISession{ID: "recovered-session", Title: "recovery", Mode: ModeLocal, Provider: "fake", Model: "fake", ScopeJSON: "{}", CreatedAt: now, UpdatedAt: now}
	if err := database.CreateAISession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if err := database.CreateAIRun(context.Background(), models.AIRun{ID: "recovered-run", SessionID: session.ID, Status: "running", Prompt: "unfinished", CreatedAt: now, UpdatedAt: now, StartedAt: &now}); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), &fakeProvider{}, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	detail, err := aiService.GetSession(context.Background(), session.ID)
	if err != nil || detail.LatestRun == nil || detail.LatestRun.Status != "interrupted" {
		t.Fatalf("recovered detail = %+v, %v", detail, err)
	}
	events, err := aiService.Events(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	var replayed []service.AIEvent
	for event := range events {
		replayed = append(replayed, event)
	}
	if len(replayed) != 1 || replayed[0].Type != "error" || replayed[0].Sequence != 1 {
		t.Fatalf("replayed events = %+v", replayed)
	}
}

type fakeProvider struct {
	chat          ChatResponse
	chatResponses []ChatResponse
	deltas        []string
	chatCalls     int
	streamCalls   int
	chatRequests  []ChatRequest
	streamRequest ChatRequest
}

func (p *fakeProvider) Chat(_ context.Context, request ChatRequest) (ChatResponse, error) {
	index := p.chatCalls
	p.chatCalls++
	p.chatRequests = append(p.chatRequests, request)
	if index < len(p.chatResponses) {
		return p.chatResponses[index], nil
	}
	return p.chat, nil
}

func (p *fakeProvider) ChatStream(ctx context.Context, request ChatRequest) (<-chan StreamEvent, error) {
	p.streamCalls++
	p.streamRequest = request
	result := make(chan StreamEvent, len(p.deltas)+1)
	for _, delta := range p.deltas {
		result <- StreamEvent{Delta: delta}
	}
	result <- StreamEvent{Done: true}
	close(result)
	return result, ctx.Err()
}

func aiTestDatabase(t *testing.T) (*db.Database, func()) {
	t.Helper()
	file, err := os.CreateTemp("", "ai-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	cfg := config.DefaultConfig()
	cfg.Database.DBFile = file.Name()
	database, err := db.NewDatabase(&cfg)
	if err != nil {
		_ = os.Remove(file.Name())
		t.Fatal(err)
	}
	cleanup := func() {
		_ = database.Close()
		_ = os.Remove(file.Name())
		_ = os.Remove(file.Name() + "-wal")
		_ = os.Remove(file.Name() + "-shm")
	}
	return database, cleanup
}
