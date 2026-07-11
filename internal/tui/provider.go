package tui

import (
	"context"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

type PostsProvider interface {
	ListPosts(cursor, limit, label int, keyword string) ([]models.Post, int, bool, error)
	GetPostDetail(pid int32, sortAsc bool) (*models.Post, []models.Comment, int32, bool, error)
	ListComments(pid int32, sortAsc bool, cursor int32) ([]models.Comment, int32, bool, error)
	SearchPosts(keyword string, cursor, limit, label int) ([]models.Post, int, bool, error)
	ListTags() ([]models.Tag, error)
	GetCourseTable() ([]models.CourseScheduleRow, error)
	GetCourseScores() (*models.ScoreSummary, error)
	RefreshPost(pid int32) (*models.Post, error)
	TogglePraise(pid int32) error
	ToggleAttention(pid int32) error
	CreateComment(pid int32, text string, quoteID *int32, imagePaths []string) error
	CreatePost(text string, imagePaths []string) error
	CanWrite() bool
	Mode() SessionMode
}

// postsServiceAdapter keeps the TUI's context-free provider contract isolated
// from the application service API. Query interpretation, pagination, mention
// enrichment, uploads, and source-specific validation all belong to PostService.
type postsServiceAdapter struct {
	posts  *service.PostService
	source string
}

type OfflinePostsProvider struct {
	postsServiceAdapter
}

func NewOfflinePostsProvider(posts *service.PostService) *OfflinePostsProvider {
	return &OfflinePostsProvider{postsServiceAdapter: postsServiceAdapter{
		posts:  posts,
		source: service.SourceLocal,
	}}
}

func (p *OfflinePostsProvider) Mode() SessionMode { return SessionModeOffline }

type OnlinePostsProvider struct {
	postsServiceAdapter
}

func NewOnlinePostsProvider(posts *service.PostService) *OnlinePostsProvider {
	return &OnlinePostsProvider{postsServiceAdapter: postsServiceAdapter{
		posts:  posts,
		source: service.SourceLive,
	}}
}

func (p *OnlinePostsProvider) Mode() SessionMode { return SessionModeOnline }

func (p *postsServiceAdapter) ListPosts(cursor, limit, label int, keyword string) ([]models.Post, int, bool, error) {
	page, err := p.posts.List(context.Background(), service.PostQuery{
		Cursor: cursor,
		Limit:  limit,
		Query:  keyword,
		Source: p.source,
		Label:  label,
	})
	if err != nil {
		return nil, 0, false, err
	}
	return postSummariesToModels(page.Items), page.NextCursor, page.HasMore, nil
}

func (p *postsServiceAdapter) SearchPosts(keyword string, cursor, limit, label int) ([]models.Post, int, bool, error) {
	page, err := p.posts.Search(context.Background(), service.PostQuery{
		Cursor: cursor,
		Limit:  limit,
		Query:  keyword,
		Source: p.source,
		Label:  label,
	})
	if err != nil {
		return nil, 0, false, err
	}
	return postSummariesToModels(page.Items), page.NextCursor, page.HasMore, nil
}

func (p *postsServiceAdapter) GetPostDetail(pid int32, sortAsc bool) (*models.Post, []models.Comment, int32, bool, error) {
	detail, err := p.posts.Get(context.Background(), pid, service.CommentQuery{
		Limit:  50,
		Sort:   commentSort(sortAsc),
		Source: p.source,
	})
	if err != nil {
		return nil, nil, 0, false, err
	}
	post := detail.Post
	return &post, detail.Comments, detail.NextCommentCursor, detail.HasMoreComments, nil
}

func (p *postsServiceAdapter) ListComments(pid int32, sortAsc bool, cursor int32) ([]models.Comment, int32, bool, error) {
	page, err := p.posts.Comments(context.Background(), pid, service.CommentQuery{
		Cursor: cursor,
		Limit:  50,
		Sort:   commentSort(sortAsc),
		Source: p.source,
	})
	if err != nil {
		return nil, 0, false, err
	}
	return page.Items, page.NextCursor, page.HasMore, nil
}

func (p *postsServiceAdapter) ListTags() ([]models.Tag, error) {
	return p.posts.ListTags(context.Background(), p.source)
}

func (p *postsServiceAdapter) GetCourseTable() ([]models.CourseScheduleRow, error) {
	return p.posts.GetCourseTable(context.Background(), p.source)
}

func (p *postsServiceAdapter) GetCourseScores() (*models.ScoreSummary, error) {
	return p.posts.GetCourseScores(context.Background(), p.source)
}

func (p *postsServiceAdapter) RefreshPost(pid int32) (*models.Post, error) {
	return p.posts.RefreshPost(context.Background(), pid, p.source)
}

func (p *postsServiceAdapter) TogglePraise(pid int32) error {
	return p.posts.TogglePraise(context.Background(), pid, p.source)
}

func (p *postsServiceAdapter) ToggleAttention(pid int32) error {
	return p.posts.ToggleAttention(context.Background(), pid, p.source)
}

func (p *postsServiceAdapter) CreateComment(pid int32, text string, quoteID *int32, imagePaths []string) error {
	return p.posts.CreateComment(context.Background(), pid, text, quoteID, imagePaths, p.source)
}

func (p *postsServiceAdapter) CreatePost(text string, imagePaths []string) error {
	return p.posts.CreatePost(context.Background(), text, imagePaths, p.source)
}

func (p *postsServiceAdapter) CanWrite() bool {
	return p.posts.CanWrite(context.Background(), p.source)
}

func postSummariesToModels(items []service.PostSummary) []models.Post {
	if items == nil {
		return nil
	}
	posts := make([]models.Post, len(items))
	for i := range items {
		posts[i] = items[i].Post
	}
	return posts
}

func commentSort(sortAsc bool) string {
	if sortAsc {
		return "asc"
	}
	return "desc"
}

// Compatibility helpers delegate compact query and upload-path rules to the
// service layer so older TUI callers and focused tests have one source of truth.
type postListSearch struct {
	pid      int32
	keyword  string
	isFollow *bool
}

func parsePostListSearch(raw string) postListSearch {
	parsed := service.ParsePostQuery(raw)
	return postListSearch{
		pid:      parsed.PID,
		keyword:  parsed.Keyword,
		isFollow: parsed.IsFollow,
	}
}

func splitPIDSearch(keyword string) (int32, string) {
	parsed := service.ParsePostQuery(keyword)
	return parsed.PID, parsed.Keyword
}

func normalizeUploadImagePath(path string) (string, error) {
	return service.NormalizeUploadImagePath(path)
}

func mentionedPostID(post models.Post) int32 {
	return service.MentionedPostID(post)
}

var (
	_ PostsProvider = (*OfflinePostsProvider)(nil)
	_ PostsProvider = (*OnlinePostsProvider)(nil)
)
