package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

func TestExpandResearchQueriesCoversTreeholeCourseVocabulary(t *testing.T) {
	variants := expandResearchQueries("高数 给分怎么样")
	if len(variants) < 3 || variants[0].Query != "高数 给分怎么样" || variants[0].Weight != 1 || len(variants) > maxResearchQueryVariants {
		t.Fatalf("variants = %+v", variants)
	}
	queries := make(map[string]bool)
	for _, variant := range variants {
		queries[variant.Query] = true
	}
	for _, expected := range []string{"高数 绩点怎么样", "高数 背刺怎么样", "高数 给分"} {
		if !queries[expected] {
			t.Errorf("expanded queries omitted %q: %+v", expected, variants)
		}
	}
	ddlVariants := expandResearchQueries("高数 DDL多吗")
	if !containsResearchVariant(ddlVariants, "高数 作业多吗") {
		t.Fatalf("DDL variants = %+v", ddlVariants)
	}
	workloadVariants := expandResearchQueries("高数 作业量")
	if !containsResearchVariant(workloadVariants, "高数 作业") || containsResearchVariant(workloadVariants, "高数 大作业量") {
		t.Fatalf("作业量 variants = %+v", workloadVariants)
	}
}

func containsResearchVariant(variants []researchQueryVariant, expected string) bool {
	for _, variant := range variants {
		if variant.Query == expected {
			return true
		}
	}
	return false
}

func TestResearchSearchRecallsSynonymsWithoutDroppingCourseAnchor(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{
		{Pid: 61001, Text: "高数 给分 很稳定", Timestamp: 1},
		{Pid: 61002, Text: "高数 课程体验", Timestamp: 2},
		{Pid: 61003, Text: "高数 背刺 的经历", Timestamp: 3},
		{Pid: 61004, Text: "其他课程 绩点 很高", Timestamp: 4},
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertComments([]models.Comment{{Cid: 61102, Pid: 61002, Text: "高数 绩点 还不错"}}); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	aiService := &Service{search: service.NewSearchService(posts, database)}
	items, variants, err := aiService.searchResearchArchive(context.Background(), service.AIRequest{}, "高数 给分", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(variants) < 3 {
		t.Fatalf("variants = %+v", variants)
	}
	found := make(map[int32]bool)
	for _, item := range items {
		found[item.Pid] = true
	}
	for _, pid := range []int32{61001, 61002, 61003} {
		if !found[pid] {
			t.Errorf("research search missed PID %d: %+v", pid, items)
		}
	}
	if found[61004] {
		t.Fatalf("synonym expansion lost the course anchor: %+v", items)
	}
}

func TestFuseResearchRankingsLimitsCommentsPerPID(t *testing.T) {
	comments := make([]models.CommentSearchHit, 0, 6)
	for cid := int32(1); cid <= 6; cid++ {
		comments = append(comments, models.CommentSearchHit{PID: 62001, CID: cid, Snippet: "match"})
	}
	items := fuseResearchRankings([]researchRanking{
		{Weight: 1, Items: []service.PostSummary{{Post: models.Post{Pid: 62001}, CommentMatches: comments}, {Post: models.Post{Pid: 62002}}}},
		{Weight: 0.8, Items: []service.PostSummary{{Post: models.Post{Pid: 62002}}, {Post: models.Post{Pid: 62003}}}},
	}, 10)
	if len(items) != 3 {
		t.Fatalf("fused items = %+v", items)
	}
	for _, item := range items {
		if item.Pid == 62001 && len(item.CommentMatches) != maxCommentsPerSearchHit {
			t.Fatalf("comment quota = %d, want %d", len(item.CommentMatches), maxCommentsPerSearchHit)
		}
	}
}

func TestFormatRankedPostSummaryIncludesOnlySelectedComments(t *testing.T) {
	hit := service.PostSummary{
		Post: models.Post{Pid: 63001, Text: "课程正文"},
		CommentMatches: []models.CommentSearchHit{
			{PID: 63001, CID: 3, Snippet: "评论内容 <mark>C</mark>"},
			{PID: 63001, CID: 8, Snippet: "评论内容 H"},
		},
	}
	formatted, sources := formatRankedPostSummaryLimited(hit, 3_000)
	if !strings.Contains(formatted, "评论内容 C") || !strings.Contains(formatted, "评论内容 H") || strings.Contains(formatted, "评论内容 B") {
		t.Fatalf("formatted evidence = %q", formatted)
	}
	if len(sources) != 3 || sources[1].CID == nil || *sources[1].CID != 3 || sources[2].CID == nil || *sources[2].CID != 8 {
		t.Fatalf("sources = %+v", sources)
	}
}

func TestSelectedResearchReservesContextForEveryPID(t *testing.T) {
	database, cleanup := aiTestDatabase(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{
		{Pid: 64001, Text: strings.Repeat("第一个超长洞内容", 12_000), Timestamp: 1},
		{Pid: 64002, Text: "第二个洞的关键证据", Timestamp: 2},
	}); err != nil {
		t.Fatal(err)
	}
	posts := service.NewPostService(database, nil)
	provider := &fakeProvider{deltas: []string{"结论来自后一个洞 [#64002]"}}
	cfg := config.DefaultConfig().AI
	cfg.Enabled = true
	aiService := NewService(context.Background(), database, posts, service.NewSearchService(posts, database), provider, cfg, ProviderInfo{Name: "fake", Model: "fake", Configured: true})
	session, err := aiService.CreateSession(context.Background(), ModeSelected, "context diversity", models.AIScope{PIDs: []int32{64001, 64002}})
	if err != nil {
		t.Fatal(err)
	}
	events, err := aiService.Run(context.Background(), service.AIRequest{SessionID: session.ID, Mode: ModeSelected, Prompt: "比较两个洞"})
	if err != nil {
		t.Fatal(err)
	}
	var completed bool
	for event := range events {
		completed = completed || event.Type == "completed"
	}
	if !completed {
		t.Fatal("selected research did not complete with evidence from the later PID")
	}
	if !strings.Contains(provider.streamRequest.Messages[len(provider.streamRequest.Messages)-1].Content, "第二个洞的关键证据") {
		t.Fatal("later PID was still excluded from the model context")
	}
}
