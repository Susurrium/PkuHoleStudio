package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

type providerRepository struct {
	posts []models.Post

	listCursor    int
	listLimit     int
	listSortAsc   bool
	searchQuery   string
	commentPID    int32
	commentCursor int32
	commentLimit  int
	commentAsc    bool
}

func (r *providerRepository) GetPostsCursor(cursor, limit int, sortAsc bool) ([]models.Post, error) {
	r.listCursor = cursor
	r.listLimit = limit
	r.listSortAsc = sortAsc
	return append([]models.Post(nil), r.posts...), nil
}

func (r *providerRepository) SearchPostsCursor(keyword string, cursor, limit int, sortAsc bool) ([]models.Post, error) {
	r.searchQuery = keyword
	r.listCursor = cursor
	r.listLimit = limit
	r.listSortAsc = sortAsc
	return append([]models.Post(nil), r.posts...), nil
}

func (r *providerRepository) GetPostByPid(pid int32) (*models.Post, error) {
	for i := range r.posts {
		if r.posts[i].Pid == pid {
			post := r.posts[i]
			return &post, nil
		}
	}
	return nil, errors.New("post not found")
}

func (r *providerRepository) GetCommentsByPidCursor(pid, cursor int32, limit int, sortAsc bool) ([]models.Comment, error) {
	r.commentPID = pid
	r.commentCursor = cursor
	r.commentLimit = limit
	r.commentAsc = sortAsc
	return []models.Comment{{Cid: 7, Pid: pid}}, nil
}

func (r *providerRepository) GetCommentByCid(int32) (*models.Comment, error) { return nil, nil }
func (r *providerRepository) GetPostsOrderBy(string, int, int) ([]models.Post, error) {
	return append([]models.Post(nil), r.posts...), nil
}
func (r *providerRepository) SearchPostsOrderBy(keyword, _ string, _, _ int) ([]models.Post, error) {
	r.searchQuery = keyword
	return append([]models.Post(nil), r.posts...), nil
}
func (r *providerRepository) GetPostCount() (int, error)    { return len(r.posts), nil }
func (r *providerRepository) GetCommentCount() (int, error) { return 1, nil }

type providerRemote struct {
	posts        []models.Post
	listQuery    service.RemoteListQuery
	commentPID   int32
	commentQuery service.RemoteCommentQuery
	canWrite     bool

	refreshPID       int32
	praisePID        int32
	attentionPID     int32
	uploadedPaths    []string
	createdPost      service.CreatePostRequest
	createdComment   service.CreateCommentRequest
	createPostCalls  int
	createReplyCalls int
}

func (r *providerRemote) ListPosts(_ context.Context, query service.RemoteListQuery) ([]models.Post, int, error) {
	r.listQuery = query
	return append([]models.Post(nil), r.posts...), 25, nil
}

func (r *providerRemote) GetPost(_ context.Context, pid int32) (*models.Post, error) {
	for i := range r.posts {
		if r.posts[i].Pid == pid {
			post := r.posts[i]
			return &post, nil
		}
	}
	return nil, errors.New("post not found")
}

func (r *providerRemote) ListComments(_ context.Context, pid int32, query service.RemoteCommentQuery) ([]models.Comment, int, error) {
	r.commentPID = pid
	r.commentQuery = query
	return []models.Comment{{Cid: 9, Pid: pid}}, 120, nil
}

func (r *providerRemote) ListTags(context.Context) ([]models.Tag, error) {
	return []models.Tag{{}}, nil
}

func (r *providerRemote) GetCourseTable(context.Context) ([]models.CourseScheduleRow, error) {
	return []models.CourseScheduleRow{{}}, nil
}

func (r *providerRemote) GetCourseScores(context.Context) (*models.ScoreSummary, error) {
	return &models.ScoreSummary{}, nil
}

func (r *providerRemote) RefreshPost(ctx context.Context, pid int32) (*models.Post, error) {
	r.refreshPID = pid
	return r.GetPost(ctx, pid)
}

func (r *providerRemote) TogglePraise(_ context.Context, pid int32) error {
	r.praisePID = pid
	return nil
}

func (r *providerRemote) ToggleAttention(_ context.Context, pid int32) error {
	r.attentionPID = pid
	return nil
}

func (r *providerRemote) UploadImage(_ context.Context, path string) (string, error) {
	r.uploadedPaths = append(r.uploadedPaths, path)
	return "media-id", nil
}

func (r *providerRemote) CreatePost(_ context.Context, request service.CreatePostRequest) (*models.Post, error) {
	r.createPostCalls++
	r.createdPost = request
	return &models.Post{}, nil
}

func (r *providerRemote) CreateComment(_ context.Context, request service.CreateCommentRequest) (*models.Comment, error) {
	r.createReplyCalls++
	r.createdComment = request
	return &models.Comment{}, nil
}

func (r *providerRemote) CanWrite(context.Context) (bool, error) { return r.canWrite, nil }

func TestOfflinePostsProviderAdaptsLocalPostService(t *testing.T) {
	repository := &providerRepository{posts: []models.Post{
		{Pid: 50, Text: "first", Mention: "99"},
		{Pid: 40, Text: "second"},
		{Pid: 99, Text: "mentioned"},
	}}
	remote := &providerRemote{canWrite: true}
	provider := NewOfflinePostsProvider(service.NewPostService(repository, remote))

	posts, next, hasMore, err := provider.ListPosts(60, 3, 0, "")
	if err != nil {
		t.Fatalf("ListPosts() error = %v", err)
	}
	if repository.listCursor != 60 || repository.listLimit != 3 || repository.listSortAsc {
		t.Fatalf("repository list args = cursor %d, limit %d, asc %v", repository.listCursor, repository.listLimit, repository.listSortAsc)
	}
	if len(posts) != 3 || posts[0].Pid != 50 || posts[0].MentionedPost == nil || posts[0].MentionedPost.Pid != 99 {
		t.Fatalf("ListPosts() posts = %+v", posts)
	}
	if next != 99 || !hasMore {
		t.Fatalf("ListPosts() cursor/more = %d/%v", next, hasMore)
	}

	_, _, _, err = provider.SearchPosts("#50 course", 55, 3, 0)
	if err != nil {
		t.Fatalf("SearchPosts() error = %v", err)
	}
	if repository.searchQuery != "#50 course" || repository.listCursor != 55 {
		t.Fatalf("repository search = %q at %d", repository.searchQuery, repository.listCursor)
	}

	post, comments, commentCursor, commentsMore, err := provider.GetPostDetail(50, true)
	if err != nil {
		t.Fatalf("GetPostDetail() error = %v", err)
	}
	if post.Pid != 50 || len(comments) != 1 || commentCursor != 7 || commentsMore {
		t.Fatalf("GetPostDetail() = post %+v, comments %+v, cursor %d, more %v", post, comments, commentCursor, commentsMore)
	}
	if repository.commentPID != 50 || repository.commentCursor != 0 || repository.commentLimit != 50 || !repository.commentAsc {
		t.Fatalf("detail comment args = pid %d, cursor %d, limit %d, asc %v", repository.commentPID, repository.commentCursor, repository.commentLimit, repository.commentAsc)
	}

	comments, commentCursor, commentsMore, err = provider.ListComments(50, false, 12)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if len(comments) != 1 || commentCursor != 7 || commentsMore || repository.commentCursor != 12 || repository.commentAsc {
		t.Fatalf("ListComments() = %+v, cursor %d, more %v; repository cursor/asc %d/%v", comments, commentCursor, commentsMore, repository.commentCursor, repository.commentAsc)
	}

	refreshed, err := provider.RefreshPost(50)
	if err != nil || refreshed.Pid != 50 {
		t.Fatalf("RefreshPost() = %+v, %v", refreshed, err)
	}
	if provider.CanWrite() || provider.Mode() != SessionModeOffline {
		t.Fatalf("offline capability/mode = %v/%v", provider.CanWrite(), provider.Mode())
	}
	if _, err := provider.ListTags(); err == nil {
		t.Fatal("offline ListTags() should be rejected by PostService")
	}
	if err := provider.TogglePraise(50); err == nil || remote.praisePID != 0 {
		t.Fatalf("offline TogglePraise() error/remote PID = %v/%d", err, remote.praisePID)
	}
}

func TestOnlinePostsProviderAdaptsLivePostService(t *testing.T) {
	remote := &providerRemote{
		posts:    []models.Post{{Pid: 21, Text: "live"}},
		canWrite: true,
	}
	provider := NewOnlinePostsProvider(service.NewPostService(&providerRepository{}, remote))

	posts, next, hasMore, err := provider.ListPosts(1, 10, 3, ":follow #21 course")
	if err != nil {
		t.Fatalf("ListPosts() error = %v", err)
	}
	if len(posts) != 1 || posts[0].Pid != 21 || next != 2 || !hasMore {
		t.Fatalf("ListPosts() = %+v, cursor %d, more %v", posts, next, hasMore)
	}
	query := remote.listQuery
	if query.Page != 2 || query.Limit != 10 || query.Label != 3 || query.PID != 21 || query.Keyword != "course" || query.Followed == nil || !*query.Followed {
		t.Fatalf("remote list query = %+v", query)
	}

	post, comments, commentCursor, commentsMore, err := provider.GetPostDetail(21, false)
	if err != nil {
		t.Fatalf("GetPostDetail() error = %v", err)
	}
	if post.Pid != 21 || len(comments) != 1 || commentCursor != 1 || !commentsMore {
		t.Fatalf("GetPostDetail() = post %+v, comments %+v, cursor %d, more %v", post, comments, commentCursor, commentsMore)
	}
	if remote.commentPID != 21 || remote.commentQuery.Page != 1 || remote.commentQuery.Limit != 50 || remote.commentQuery.Sort != 1 {
		t.Fatalf("detail remote comment query = pid %d, %+v", remote.commentPID, remote.commentQuery)
	}

	_, nextComment, moreComments, err := provider.ListComments(21, true, 1)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if nextComment != 2 || !moreComments || remote.commentQuery.Page != 2 || remote.commentQuery.Sort != 0 {
		t.Fatalf("ListComments() cursor/more/query = %d/%v/%+v", nextComment, moreComments, remote.commentQuery)
	}

	if tags, err := provider.ListTags(); err != nil || len(tags) != 1 {
		t.Fatalf("ListTags() = %+v, %v", tags, err)
	}
	if table, err := provider.GetCourseTable(); err != nil || len(table) != 1 {
		t.Fatalf("GetCourseTable() = %+v, %v", table, err)
	}
	if scores, err := provider.GetCourseScores(); err != nil || scores == nil {
		t.Fatalf("GetCourseScores() = %+v, %v", scores, err)
	}
	if refreshed, err := provider.RefreshPost(21); err != nil || refreshed.Pid != 21 || remote.refreshPID != 21 {
		t.Fatalf("RefreshPost() = %+v, %v; remote PID %d", refreshed, err, remote.refreshPID)
	}
	if !provider.CanWrite() || provider.Mode() != SessionModeOnline {
		t.Fatalf("online capability/mode = %v/%v", provider.CanWrite(), provider.Mode())
	}

	if err := provider.TogglePraise(21); err != nil || remote.praisePID != 21 {
		t.Fatalf("TogglePraise() error/remote PID = %v/%d", err, remote.praisePID)
	}
	if err := provider.ToggleAttention(21); err != nil || remote.attentionPID != 21 {
		t.Fatalf("ToggleAttention() error/remote PID = %v/%d", err, remote.attentionPID)
	}

	imagePath := filepath.Join(t.TempDir(), "image with space.jpg")
	if err := os.WriteFile(imagePath, []byte("image"), 0o600); err != nil {
		t.Fatalf("write image fixture: %v", err)
	}
	quoteID := int32(9)
	if err := provider.CreateComment(21, "reply", &quoteID, []string{`"` + imagePath + `"`}); err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}
	if remote.createReplyCalls != 1 || remote.createdComment.PID != 21 || remote.createdComment.Text != "reply" || remote.createdComment.QuoteID == nil || *remote.createdComment.QuoteID != quoteID || len(remote.createdComment.MediaIDs) != 1 {
		t.Fatalf("created comment = %+v (calls %d)", remote.createdComment, remote.createReplyCalls)
	}
	if len(remote.uploadedPaths) != 1 || remote.uploadedPaths[0] != imagePath {
		t.Fatalf("uploaded paths = %q", remote.uploadedPaths)
	}

	if err := provider.CreatePost("new post", []string{imagePath}); err != nil {
		t.Fatalf("CreatePost() error = %v", err)
	}
	if remote.createPostCalls != 1 || remote.createdPost.Text != "new post" || remote.createdPost.Type != "image" || len(remote.createdPost.MediaIDs) != 1 {
		t.Fatalf("created post = %+v (calls %d)", remote.createdPost, remote.createPostCalls)
	}
}

func TestProviderCompatibilityHelpersDelegateToServiceRules(t *testing.T) {
	parsed := parsePostListSearch(":follow #8123 course review")
	if parsed.pid != 8123 || parsed.keyword != "course review" || parsed.isFollow == nil || !*parsed.isFollow {
		t.Fatalf("parsePostListSearch() = %+v", parsed)
	}
	pid, keyword := splitPIDSearch("course #8123")
	if pid != 0 || keyword != "course #8123" {
		t.Fatalf("splitPIDSearch() = %d, %q", pid, keyword)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "image.jpg")
	if err := os.WriteFile(path, []byte("image"), 0o600); err != nil {
		t.Fatalf("write image fixture: %v", err)
	}
	got, err := normalizeUploadImagePath(`"` + path + `"`)
	if err != nil || got != path {
		t.Fatalf("normalizeUploadImagePath() = %q, %v", got, err)
	}
}
