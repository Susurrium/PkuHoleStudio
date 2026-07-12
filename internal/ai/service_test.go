package ai

import (
	"context"
	"os"
	"strings"
	"testing"

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
		chat: ChatResponse{ToolCalls: []ToolCall{{
			ID: "call-1", Type: "function",
			Function: ToolCallFunction{Name: "search_archive", Arguments: `{"query":"alpha","reason":"find evidence","limit":5}`},
		}}},
		deltas: []string{"grounded ", "answer [#12345]"},
	}
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	cfg.MaxSearchRounds = 1
	aiService := NewService(context.Background(), database, posts, search, provider, cfg, ProviderInfo{Name: "fake", Model: "fake-model", Configured: true})
	session, err := aiService.CreateSession(context.Background(), ModeLocal, "test")
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
	for _, eventType := range []string{"started", "search_started", "search_result", "delta", "source", "completed"} {
		if types[eventType] == 0 {
			t.Errorf("missing event %q in %#v", eventType, types)
		}
	}
	if provider.chatCalls != 1 || provider.streamCalls != 1 {
		t.Fatalf("provider calls = chat %d stream %d", provider.chatCalls, provider.streamCalls)
	}
	detail, err := aiService.GetSession(context.Background(), session.ID)
	if err != nil || len(detail.Messages) != 2 || detail.Messages[1].Content != "grounded answer [#12345]" || len(detail.Messages[1].Sources) == 0 {
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

func TestSelectedModeRequiresPIDs(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	posts := service.NewPostService(database, nil)
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), &fakeProvider{deltas: []string{"unused"}}, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, _ := aiService.CreateSession(context.Background(), ModeSelected, "")
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
	provider := &fakeProvider{deltas: []string{"course comparison [#34567]"}}
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), provider, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, _ := aiService.CreateSession(context.Background(), ModeCourse, "")
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
	if err != nil || len(detail.Messages) != 2 || !strings.Contains(detail.Messages[1].TraceJSON, "Professor Li") || len(detail.Messages[1].Sources) < 3 {
		t.Fatalf("course detail = %+v, %v", detail, err)
	}
}

type fakeProvider struct {
	chat          ChatResponse
	deltas        []string
	chatCalls     int
	streamCalls   int
	streamRequest ChatRequest
}

func (p *fakeProvider) Chat(context.Context, ChatRequest) (ChatResponse, error) {
	p.chatCalls++
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
