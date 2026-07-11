package db

import (
	"errors"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func TestSearchFullTextPostCommentPIDAndFilters(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	seedSearchData(t, database)

	postPage, err := database.SearchFullText(models.FullTextQuery{Query: "alpha beta", Limit: 10})
	if err != nil {
		t.Fatalf("SearchFullText(post) error = %v", err)
	}
	if len(postPage.Hits) != 1 || postPage.Hits[0].Post.Pid != 12345 {
		t.Fatalf("post hits = %+v", postPage.Hits)
	}

	commentPage, err := database.SearchFullText(models.FullTextQuery{Query: "teacher grading", Limit: 10})
	if err != nil {
		t.Fatalf("SearchFullText(comment) error = %v", err)
	}
	if len(commentPage.Hits) != 1 || commentPage.Hits[0].Post.Pid != 23456 || len(commentPage.Hits[0].CommentMatches) != 1 {
		t.Fatalf("comment hits = %+v", commentPage.Hits)
	}
	if commentPage.Hits[0].CommentMatches[0].CID != 200 {
		t.Fatalf("comment CID = %d", commentPage.Hits[0].CommentMatches[0].CID)
	}

	pidPage, err := database.SearchFullText(models.FullTextQuery{Query: "#12345", Limit: 10})
	if err != nil || len(pidPage.Hits) != 1 || pidPage.Hits[0].Post.Pid != 12345 {
		t.Fatalf("PID hits = %+v, error = %v", pidPage.Hits, err)
	}
	if len(pidPage.Hits[0].CommentMatches) != 0 {
		t.Fatalf("PID-only search returned unrelated comment matches: %+v", pidPage.Hits[0].CommentMatches)
	}

	hasMedia := true
	filtered, err := database.SearchFullText(models.FullTextQuery{
		Query: "teacher grading", Limit: 10, Sources: []string{"explicit"}, TagIDs: []uint{1}, HasMedia: &hasMedia,
		From: 150, To: 250,
	})
	if err != nil || len(filtered.Hits) != 1 || filtered.Hits[0].Post.Pid != 23456 {
		t.Fatalf("filtered hits = %+v, error = %v", filtered.Hits, err)
	}
	filtered, err = database.SearchFullText(models.FullTextQuery{Query: "alpha beta", Limit: 10, Sources: []string{"explicit"}})
	if err != nil || len(filtered.Hits) != 0 {
		t.Fatalf("source-excluded hits = %+v, error = %v", filtered.Hits, err)
	}
}

func TestSearchFullTextShortChineseAndLiteralWildcardFallback(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{
		{Pid: 34567, Text: "课程体验 100% safe", Timestamp: 1},
		{Pid: 34568, Text: "普通内容 1000 safe", Timestamp: 2},
	}); err != nil {
		t.Fatalf("UpsertPosts() error = %v", err)
	}
	page, err := database.SearchFullText(models.FullTextQuery{Query: "课", Limit: 10})
	if err != nil || len(page.Hits) != 1 || page.Hits[0].Post.Pid != 34567 {
		t.Fatalf("short Chinese hits = %+v, error = %v", page.Hits, err)
	}
	page, err = database.SearchFullText(models.FullTextQuery{Query: "%", Limit: 10})
	if err != nil || len(page.Hits) != 1 || page.Hits[0].Post.Pid != 34567 {
		t.Fatalf("literal wildcard hits = %+v, error = %v", page.Hits, err)
	}
}

func TestFTS5TriggersAndRebuild(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	available, err := database.FTS5Available()
	if err != nil {
		t.Fatalf("FTS5Available() error = %v", err)
	}
	if !available {
		if err := database.RebuildSearchIndex(); !errors.Is(err, ErrFTS5Unavailable) {
			t.Fatalf("RebuildSearchIndex() error = %v, want ErrFTS5Unavailable", err)
		}
		t.Skip("binary was not built with sqlite_fts5")
	}

	if err := database.UpsertPosts([]models.Post{{Pid: 45678, Text: "trigger original phrase"}}); err != nil {
		t.Fatalf("insert post: %v", err)
	}
	page, err := database.SearchFullText(models.FullTextQuery{Query: "original phrase", Limit: 10})
	if err != nil || len(page.Hits) != 1 {
		t.Fatalf("insert trigger hits = %+v, error = %v", page.Hits, err)
	}
	if err := database.UpsertPosts([]models.Post{{Pid: 45678, Text: "replacement wording"}}); err != nil {
		t.Fatalf("update post: %v", err)
	}
	page, err = database.SearchFullText(models.FullTextQuery{Query: "original phrase", Limit: 10})
	if err != nil || len(page.Hits) != 0 {
		t.Fatalf("stale update trigger hits = %+v, error = %v", page.Hits, err)
	}
	page, err = database.SearchFullText(models.FullTextQuery{Query: "replacement wording", Limit: 10})
	if err != nil || len(page.Hits) != 1 {
		t.Fatalf("updated trigger hits = %+v, error = %v", page.Hits, err)
	}

	if err := database.db.Exec("DELETE FROM posts_fts").Error; err != nil {
		t.Fatalf("clear FTS index: %v", err)
	}
	if err := database.RebuildSearchIndex(); err != nil {
		t.Fatalf("RebuildSearchIndex() error = %v", err)
	}
	page, err = database.SearchFullText(models.FullTextQuery{Query: "replacement wording", Limit: 10})
	if err != nil || len(page.Hits) != 1 {
		t.Fatalf("rebuilt hits = %+v, error = %v", page.Hits, err)
	}
}

func seedSearchData(t *testing.T, database *Database) {
	t.Helper()
	if err := database.UpsertPosts([]models.Post{
		{Pid: 12345, Text: "alpha discussion with beta details", Timestamp: 100, Type: "text"},
		{Pid: 23456, Text: "course review", Timestamp: 200, Type: "image", MediaIds: "9"},
		{Pid: 34567, Text: "unrelated", Timestamp: 300, Type: "text"},
	}); err != nil {
		t.Fatalf("UpsertPosts() error = %v", err)
	}
	if err := database.UpsertComments([]models.Comment{
		{Cid: 100, Pid: 12345, Text: "ordinary reply", NameTag: "Alice"},
		{Cid: 200, Pid: 23456, Text: "teacher grading was transparent", NameTag: "Bob"},
	}); err != nil {
		t.Fatalf("UpsertComments() error = %v", err)
	}
	now := time.Now().UTC()
	if err := database.db.Create(&models.PostSource{PID: 12345, Source: "followed", SourceRef: "test", FirstSeenAt: now, LastSeenAt: now}).Error; err != nil {
		t.Fatalf("create followed source: %v", err)
	}
	if err := database.db.Create(&models.PostSource{PID: 23456, Source: "explicit", SourceRef: "test", FirstSeenAt: now, LastSeenAt: now}).Error; err != nil {
		t.Fatalf("create explicit source: %v", err)
	}
	if err := database.db.Create(&models.LocalTag{ID: 1, Name: "course", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create local tag: %v", err)
	}
	if err := database.db.Create(&models.PostTag{PID: 23456, TagID: 1, CreatedAt: now}).Error; err != nil {
		t.Fatalf("create post tag: %v", err)
	}
}
