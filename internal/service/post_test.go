package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type fakeRepository struct {
	posts          []models.Post
	comments       []models.Comment
	searchKeyword  string
	postFetchCount map[int32]int
}

type fakePostMediaRepository struct {
	*fakeRepository
	media []models.Media
}

func (f *fakePostMediaRepository) GetMediaByPID(int32) ([]models.Media, error) {
	return append([]models.Media(nil), f.media...), nil
}
func (f *fakePostMediaRepository) GetMediaByID(uint) (*models.Media, error) { return nil, nil }

func (f *fakeRepository) GetPostsCursor(int, int, bool) ([]models.Post, error) {
	return append([]models.Post(nil), f.posts...), nil
}
func (f *fakeRepository) SearchPostsCursor(keyword string, _ int, _ int, _ bool) ([]models.Post, error) {
	f.searchKeyword = keyword
	return append([]models.Post(nil), f.posts...), nil
}
func (f *fakeRepository) GetPostByPid(pid int32) (*models.Post, error) {
	if f.postFetchCount == nil {
		f.postFetchCount = map[int32]int{}
	}
	f.postFetchCount[pid]++
	for _, post := range f.posts {
		if post.Pid == pid {
			copy := post
			return &copy, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeRepository) GetCommentsByPidCursor(int32, int32, int, bool) ([]models.Comment, error) {
	return append([]models.Comment(nil), f.comments...), nil
}
func (f *fakeRepository) GetCommentByCid(int32) (*models.Comment, error) { return nil, nil }
func (f *fakeRepository) GetPostsOrderBy(string, int, int) ([]models.Post, error) {
	return append([]models.Post(nil), f.posts...), nil
}
func (f *fakeRepository) SearchPostsOrderBy(keyword, _ string, _ int, _ int) ([]models.Post, error) {
	f.searchKeyword = keyword
	return append([]models.Post(nil), f.posts...), nil
}
func (f *fakeRepository) GetPostCount() (int, error)    { return len(f.posts), nil }
func (f *fakeRepository) GetCommentCount() (int, error) { return len(f.comments), nil }

type fakeRemote struct {
	posts         []models.Post
	comments      []models.Comment
	listQuery     RemoteListQuery
	commentQuery  RemoteCommentQuery
	postFetches   map[int32]int
	canWrite      bool
	uploadedPaths []string
}

func (f *fakeRemote) ListPosts(_ context.Context, query RemoteListQuery) ([]models.Post, int, error) {
	f.listQuery = query
	return append([]models.Post(nil), f.posts...), 30, nil
}
func (f *fakeRemote) GetPost(_ context.Context, pid int32) (*models.Post, error) {
	if f.postFetches == nil {
		f.postFetches = map[int32]int{}
	}
	f.postFetches[pid]++
	for _, post := range f.posts {
		if post.Pid == pid {
			copy := post
			return &copy, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeRemote) ListComments(_ context.Context, _ int32, query RemoteCommentQuery) ([]models.Comment, int, error) {
	f.commentQuery = query
	return append([]models.Comment(nil), f.comments...), 80, nil
}
func (f *fakeRemote) ListTags(context.Context) ([]models.Tag, error) { return nil, nil }
func (f *fakeRemote) GetCourseTable(context.Context) ([]models.CourseScheduleRow, error) {
	return nil, nil
}
func (f *fakeRemote) GetCourseScores(context.Context) (*models.ScoreSummary, error) { return nil, nil }
func (f *fakeRemote) RefreshPost(ctx context.Context, pid int32) (*models.Post, error) {
	return f.GetPost(ctx, pid)
}
func (f *fakeRemote) TogglePraise(context.Context, int32) error    { return nil }
func (f *fakeRemote) ToggleAttention(context.Context, int32) error { return nil }
func (f *fakeRemote) UploadImage(_ context.Context, path string) (string, error) {
	f.uploadedPaths = append(f.uploadedPaths, path)
	return "1", nil
}
func (f *fakeRemote) CreatePost(context.Context, CreatePostRequest) (*models.Post, error) {
	return &models.Post{}, nil
}
func (f *fakeRemote) CreateComment(context.Context, CreateCommentRequest) (*models.Comment, error) {
	return &models.Comment{}, nil
}
func (f *fakeRemote) CanWrite(context.Context) (bool, error) { return f.canWrite, nil }

func TestPostServiceLocalSearchAndMentionCache(t *testing.T) {
	repository := &fakeRepository{posts: []models.Post{{Pid: 9, Mention: "42"}, {Pid: 8, Mention: "42"}, {Pid: 42, Text: "mentioned"}}}
	service := NewPostService(repository, nil)
	page, err := service.Search(context.Background(), PostQuery{Query: "#9 course review", Limit: 3})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if repository.searchKeyword != "#9 course review" {
		t.Fatalf("repository keyword = %q", repository.searchKeyword)
	}
	if page.NextCursor != 42 || !page.HasMore {
		t.Fatalf("page cursor/more = %d/%v", page.NextCursor, page.HasMore)
	}
	if repository.postFetchCount[42] != 1 {
		t.Fatalf("mentioned PID fetched %d times", repository.postFetchCount[42])
	}
	if page.Items[0].MentionedPost == nil || page.Items[1].MentionedPost == nil {
		t.Fatal("mentioned post was not enriched")
	}
}

func TestPostServiceLiveCursorAndFilters(t *testing.T) {
	remote := &fakeRemote{posts: []models.Post{{Pid: 10}}}
	service := NewPostService(nil, remote)
	page, err := service.List(context.Background(), PostQuery{Source: SourceLive, Cursor: 1, Limit: 10, Query: ":follow #10 course", Label: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if remote.listQuery.Page != 2 || remote.listQuery.PID != 10 || remote.listQuery.Keyword != "course" || remote.listQuery.Followed == nil || !*remote.listQuery.Followed {
		t.Fatalf("remote query = %+v", remote.listQuery)
	}
	if page.NextCursor != 2 || !page.HasMore {
		t.Fatalf("page = %+v", page)
	}
}

func TestPostServiceCommentCursorAndSort(t *testing.T) {
	remote := &fakeRemote{comments: []models.Comment{{Cid: 5}}}
	service := NewPostService(nil, remote)
	page, err := service.Comments(context.Background(), 10, CommentQuery{Source: SourceLive, Cursor: 1, Limit: 20, Sort: "desc"})
	if err != nil {
		t.Fatalf("Comments() error = %v", err)
	}
	if remote.commentQuery.Page != 2 || remote.commentQuery.Sort != 1 || page.NextCursor != 2 || !page.HasMore {
		t.Fatalf("query=%+v page=%+v", remote.commentQuery, page)
	}
}

func TestPostServiceLocalDetailIncludesMediaCatalog(t *testing.T) {
	repository := &fakePostMediaRepository{
		fakeRepository: &fakeRepository{posts: []models.Post{{Pid: 123456}}, comments: []models.Comment{{Cid: 7, Pid: 123456}}},
		media:          []models.Media{{ID: 1, OwnerType: "post", OwnerID: 123456, Status: "available"}, {ID: 2, OwnerType: "comment", OwnerID: 7, Status: "missing"}},
	}
	detail, err := NewPostService(repository, nil).Get(context.Background(), 123456, CommentQuery{Source: SourceLocal})
	if err != nil || len(detail.Media) != 2 {
		t.Fatalf("Get() = %+v, %v", detail, err)
	}
}

func TestPostServiceLiveDetailDescribesRemotePostAndCommentMedia(t *testing.T) {
	remote := &fakeRemote{
		posts:    []models.Post{{Pid: 123456, Type: "image", MediaIds: "10,11"}},
		comments: []models.Comment{{Cid: 7, Pid: 123456, MediaIds: "12"}},
	}
	detail, err := NewPostService(nil, remote).Get(context.Background(), 123456, CommentQuery{Source: SourceLive})
	if err != nil || len(detail.Media) != 3 {
		t.Fatalf("Get() = %+v, %v", detail, err)
	}
	for _, item := range detail.Media {
		if item.Status != "remote" {
			t.Fatalf("media = %+v", item)
		}
	}
}

func TestPostServicePublishesAndRepliesWithUploadedMediaIDs(t *testing.T) {
	remote := &fakeRemote{canWrite: true}
	posts := NewPostService(nil, remote)
	id, err := posts.UploadMedia(context.Background(), "image.png", SourceLive)
	if err != nil || id != "1" {
		t.Fatalf("UploadMedia() = %q, %v", id, err)
	}
	if _, err := posts.PublishPost(context.Background(), "post", []string{id}, SourceLive); err != nil {
		t.Fatal(err)
	}
	quote := int32(7)
	if _, err := posts.Reply(context.Background(), 123456, "reply", &quote, []string{id}, SourceLive); err != nil {
		t.Fatal(err)
	}
	if len(remote.uploadedPaths) != 1 || remote.uploadedPaths[0] != "image.png" {
		t.Fatalf("uploaded paths = %+v", remote.uploadedPaths)
	}
}

func TestPostServiceRejectsLocalFollowAndCancellation(t *testing.T) {
	service := NewPostService(&fakeRepository{}, nil)
	if _, err := service.List(context.Background(), PostQuery{Query: ":follow"}); err == nil {
		t.Fatal("local followed filter should fail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.List(ctx, PostQuery{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled List() error = %v", err)
	}
}

func TestParsePostQuery(t *testing.T) {
	parsed := ParsePostQuery(":follow #8123 course review")
	if parsed.PID != 8123 || parsed.Keyword != "course review" || parsed.IsFollow == nil || !*parsed.IsFollow {
		t.Fatalf("ParsePostQuery() = %+v", parsed)
	}
	invalid := ParsePostQuery("course #8123")
	if invalid.PID != 0 || invalid.Keyword != "course #8123" {
		t.Fatalf("ParsePostQuery(non-prefix) = %+v", invalid)
	}
}
