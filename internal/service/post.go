package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

const (
	defaultPostLimit    = 25
	defaultCommentLimit = 50
	maxPageLimit        = 100
)

var (
	errRepositoryUnavailable = errors.New("local repository is not configured")
	errLiveOnly              = errors.New("operation requires the live Treehole source")
)

func (q ParsedPostQuery) KeywordWithPID() string {
	if q.PID == 0 {
		return q.Keyword
	}
	if q.Keyword == "" {
		return fmt.Sprintf("#%d", q.PID)
	}
	return fmt.Sprintf("#%d %s", q.PID, q.Keyword)
}

// ParsePostQuery recognizes an exact leading #PID and the :follow modifier.
// Remaining words are preserved as an AND-style keyword query by repositories.
func ParsePostQuery(raw string) ParsedPostQuery {
	parts := strings.Fields(strings.TrimSpace(raw))
	result := ParsedPostQuery{}
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == ":follow" {
			followed := true
			result.IsFollow = &followed
			continue
		}
		if result.PID == 0 && len(keywords) == 0 && strings.HasPrefix(part, "#") && len(part) > 1 {
			pid, err := strconv.ParseInt(strings.TrimPrefix(part, "#"), 10, 32)
			if err == nil && pid > 0 {
				result.PID = int32(pid)
				continue
			}
		}
		keywords = append(keywords, part)
	}
	result.Keyword = strings.Join(keywords, " ")
	return result
}

// MentionedPostID returns the positive PID referenced by a post's Mention
// field, or zero when the field is empty or invalid.
func MentionedPostID(post models.Post) int32 {
	pid, err := strconv.ParseInt(strings.TrimSpace(post.Mention), 10, 32)
	if err != nil || pid <= 0 {
		return 0
	}
	return int32(pid)
}

type PostService struct {
	repository Repository
	remote     Remote
}

func NewPostService(repository Repository, remote Remote) *PostService {
	return &PostService{repository: repository, remote: remote}
}

func (s *PostService) List(ctx context.Context, query PostQuery) (PostPage, error) {
	if err := contextError(ctx); err != nil {
		return PostPage{}, err
	}
	query = normalizePostQuery(query)
	parsed := ParsePostQuery(query.Query)
	switch query.Source {
	case SourceLocal:
		return s.listLocal(ctx, query, parsed)
	case SourceLive:
		return s.listLive(ctx, query, parsed)
	default:
		return PostPage{}, fmt.Errorf("unsupported post source %q", query.Source)
	}
}

func (s *PostService) Search(ctx context.Context, query PostQuery) (PostPage, error) {
	return s.List(ctx, query)
}

func (s *PostService) Get(ctx context.Context, pid int32, query CommentQuery) (PostDetail, error) {
	if err := contextError(ctx); err != nil {
		return PostDetail{}, err
	}
	if pid <= 0 {
		return PostDetail{}, errors.New("pid must be positive")
	}
	query = normalizeCommentQuery(query)
	comments, err := s.Comments(ctx, pid, query)
	if err != nil {
		return PostDetail{}, err
	}
	var post *models.Post
	switch query.Source {
	case SourceLocal:
		if s == nil || s.repository == nil {
			return PostDetail{}, errRepositoryUnavailable
		}
		post, err = s.repository.GetPostByPid(pid)
	case SourceLive:
		if s == nil || s.remote == nil {
			return PostDetail{}, errRemoteUnavailable
		}
		post, err = s.remote.GetPost(ctx, pid)
	default:
		err = fmt.Errorf("unsupported post source %q", query.Source)
	}
	if err != nil {
		return PostDetail{}, err
	}
	if post == nil {
		return PostDetail{}, errors.New("post was not found")
	}
	references := []Reference{}
	media := []models.Media{}
	if query.Source == SourceLocal {
		if repository, ok := s.repository.(ReferenceRepository); ok {
			edges, referenceErr := repository.GetReferencesByPID(pid)
			if referenceErr != nil {
				return PostDetail{}, referenceErr
			}
			for _, edge := range edges {
				references = append(references, Reference{
					Kind: edge.Kind, SourcePID: edge.SourcePID, SourceCID: edge.SourceCID,
					TargetPID: edge.TargetPID, TargetCID: edge.TargetCID,
				})
			}
		}
		if repository, ok := s.repository.(MediaRepository); ok {
			media, err = repository.GetMediaByPID(pid)
			if err != nil {
				return PostDetail{}, err
			}
		}
	} else {
		media = liveMedia(post, comments.Items)
	}
	return PostDetail{
		Post:              *post,
		Comments:          comments.Items,
		References:        references,
		Media:             media,
		NextCommentCursor: comments.NextCursor,
		HasMoreComments:   comments.HasMore,
	}, nil
}

func liveMedia(post *models.Post, comments []models.Comment) []models.Media {
	result := make([]models.Media, 0)
	appendOwner := func(ownerType string, ownerID int64, rawIDs string, force bool) {
		ids := strings.FieldsFunc(rawIDs, func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
		if len(ids) == 0 && force {
			ids = []string{""}
		}
		for _, id := range ids {
			result = append(result, models.Media{OwnerType: ownerType, OwnerID: ownerID, RemoteID: strings.TrimSpace(id), Variant: "original", Status: "remote"})
		}
	}
	appendOwner("post", int64(post.Pid), post.MediaIds, post.Type == "image")
	for _, comment := range comments {
		appendOwner("comment", int64(comment.Cid), comment.MediaIds, false)
	}
	return result
}

func (s *PostService) Detail(ctx context.Context, pid int32, query CommentQuery) (PostDetail, error) {
	return s.Get(ctx, pid, query)
}

func (s *PostService) Comments(ctx context.Context, pid int32, query CommentQuery) (CommentPage, error) {
	if err := contextError(ctx); err != nil {
		return CommentPage{}, err
	}
	if pid <= 0 {
		return CommentPage{}, errors.New("pid must be positive")
	}
	query = normalizeCommentQuery(query)
	sortAscending := query.Sort != "desc"
	switch query.Source {
	case SourceLocal:
		if s == nil || s.repository == nil {
			return CommentPage{}, errRepositoryUnavailable
		}
		items, err := s.repository.GetCommentsByPidCursor(pid, query.Cursor, query.Limit, sortAscending)
		if err != nil {
			return CommentPage{}, err
		}
		return CommentPage{Items: items, NextCursor: lastCommentCursor(items), HasMore: len(items) == query.Limit}, contextError(ctx)
	case SourceLive:
		if s == nil || s.remote == nil {
			return CommentPage{}, errRemoteUnavailable
		}
		page := int(query.Cursor) + 1
		if query.Cursor <= 0 {
			page = 1
		}
		sortValue := 0
		if !sortAscending {
			sortValue = 1
		}
		items, total, err := s.remote.ListComments(ctx, pid, RemoteCommentQuery{Page: page, Limit: query.Limit, Sort: sortValue, Stream: 1})
		if err != nil {
			return CommentPage{}, err
		}
		return CommentPage{Items: items, NextCursor: int32(page), HasMore: page*query.Limit < total}, contextError(ctx)
	default:
		return CommentPage{}, fmt.Errorf("unsupported post source %q", query.Source)
	}
}

// GetComment returns a single comment from the requested source. The live API
// does not expose a lookup by CID, so this operation is currently local-only.
func (s *PostService) GetComment(ctx context.Context, cid int32, source string) (*models.Comment, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if cid <= 0 {
		return nil, errors.New("cid must be positive")
	}
	if normalizeSource(source) != SourceLocal {
		return nil, errLiveOnly
	}
	if s == nil || s.repository == nil {
		return nil, errRepositoryUnavailable
	}
	comment, err := s.repository.GetCommentByCid(cid)
	if err != nil {
		return nil, err
	}
	if comment == nil {
		return nil, errors.New("comment was not found")
	}
	return comment, contextError(ctx)
}

// Counts reports the number of locally archived posts and comments.
func (s *PostService) Counts(ctx context.Context, source string) (postCount int, commentCount int, err error) {
	if err := contextError(ctx); err != nil {
		return 0, 0, err
	}
	if normalizeSource(source) != SourceLocal {
		return 0, 0, errLiveOnly
	}
	if s == nil || s.repository == nil {
		return 0, 0, errRepositoryUnavailable
	}
	postCount, err = s.repository.GetPostCount()
	if err != nil {
		return 0, 0, err
	}
	if err := contextError(ctx); err != nil {
		return 0, 0, err
	}
	commentCount, err = s.repository.GetCommentCount()
	if err != nil {
		return 0, 0, err
	}
	return postCount, commentCount, contextError(ctx)
}

func (s *PostService) ListTags(ctx context.Context, source string) ([]models.Tag, error) {
	if normalizeSource(source) != SourceLive {
		return nil, errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return nil, err
	}
	return s.remote.ListTags(ctx)
}

func (s *PostService) GetCourseTable(ctx context.Context, source string) ([]models.CourseScheduleRow, error) {
	if normalizeSource(source) != SourceLive {
		return nil, errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return nil, err
	}
	return s.remote.GetCourseTable(ctx)
}

func (s *PostService) GetCourseScores(ctx context.Context, source string) (*models.ScoreSummary, error) {
	if normalizeSource(source) != SourceLive {
		return nil, errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return nil, err
	}
	return s.remote.GetCourseScores(ctx)
}

func (s *PostService) RefreshPost(ctx context.Context, pid int32, source string) (*models.Post, error) {
	switch normalizeSource(source) {
	case SourceLocal:
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		if s == nil || s.repository == nil {
			return nil, errRepositoryUnavailable
		}
		return s.repository.GetPostByPid(pid)
	case SourceLive:
		if err := s.requireRemote(ctx); err != nil {
			return nil, err
		}
		return s.remote.RefreshPost(ctx, pid)
	default:
		return nil, fmt.Errorf("unsupported post source %q", source)
	}
}

func (s *PostService) CanWrite(ctx context.Context, source string) bool {
	if normalizeSource(source) != SourceLive || s == nil || s.remote == nil {
		return false
	}
	canWrite, err := s.remote.CanWrite(ctx)
	return err == nil && canWrite
}

func (s *PostService) TogglePraise(ctx context.Context, pid int32, source string) error {
	if normalizeSource(source) != SourceLive {
		return errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return err
	}
	return s.remote.TogglePraise(ctx, pid)
}

func (s *PostService) ToggleAttention(ctx context.Context, pid int32, source string) error {
	if normalizeSource(source) != SourceLive {
		return errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return err
	}
	return s.remote.ToggleAttention(ctx, pid)
}

func (s *PostService) CreateComment(ctx context.Context, pid int32, text string, quoteID *int32, imagePaths []string, source string) error {
	if normalizeSource(source) != SourceLive {
		return errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return err
	}
	mediaIDs, err := s.uploadImages(ctx, imagePaths)
	if err != nil {
		return err
	}
	_, err = s.remote.CreateComment(ctx, CreateCommentRequest{PID: pid, QuoteID: quoteID, Text: text, MediaIDs: mediaIDs})
	return err
}

func (s *PostService) CreatePost(ctx context.Context, text string, imagePaths []string, source string) error {
	if normalizeSource(source) != SourceLive {
		return errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return err
	}
	mediaIDs, err := s.uploadImages(ctx, imagePaths)
	if err != nil {
		return err
	}
	postType := "text"
	if len(mediaIDs) > 0 {
		postType = "image"
	}
	_, err = s.remote.CreatePost(ctx, CreatePostRequest{Type: postType, Text: text, MediaIDs: mediaIDs})
	return err
}

func (s *PostService) UploadMedia(ctx context.Context, imagePath, source string) (string, error) {
	if normalizeSource(source) != SourceLive {
		return "", errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return "", err
	}
	if strings.TrimSpace(imagePath) == "" {
		return "", errors.New("image path is required")
	}
	return s.remote.UploadImage(ctx, imagePath)
}

func (s *PostService) PublishPost(ctx context.Context, text string, mediaIDs []string, source string) (*models.Post, error) {
	if normalizeSource(source) != SourceLive {
		return nil, errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" && len(mediaIDs) == 0 {
		return nil, errors.New("post text or media is required")
	}
	postType := "text"
	if len(mediaIDs) > 0 {
		postType = "image"
	}
	return s.remote.CreatePost(ctx, CreatePostRequest{Type: postType, Text: text, MediaIDs: mediaIDs})
}

func (s *PostService) Reply(ctx context.Context, pid int32, text string, quoteID *int32, mediaIDs []string, source string) (*models.Comment, error) {
	if normalizeSource(source) != SourceLive {
		return nil, errLiveOnly
	}
	if err := s.requireRemote(ctx); err != nil {
		return nil, err
	}
	if pid <= 0 {
		return nil, errors.New("pid must be positive")
	}
	text = strings.TrimSpace(text)
	if text == "" && len(mediaIDs) == 0 {
		return nil, errors.New("comment text or media is required")
	}
	return s.remote.CreateComment(ctx, CreateCommentRequest{PID: pid, Text: text, QuoteID: quoteID, MediaIDs: mediaIDs})
}

func (s *PostService) listLocal(ctx context.Context, query PostQuery, parsed ParsedPostQuery) (PostPage, error) {
	if s == nil || s.repository == nil {
		return PostPage{}, errRepositoryUnavailable
	}
	if parsed.IsFollow != nil {
		return PostPage{}, errors.New("offline source does not support followed filtering")
	}
	if query.Label != 0 {
		return PostPage{}, errors.New("offline source does not support remote label filtering")
	}
	if query.HasMedia != nil {
		return PostPage{}, errors.New("media filtering is not available before the local media index migration")
	}
	var (
		posts []models.Post
		err   error
	)
	orderField := normalizeOrderField(query.Sort)
	keyword := parsed.KeywordWithPID()
	if orderField != "" {
		if keyword == "" {
			posts, err = s.repository.GetPostsOrderBy(orderField, query.Cursor, query.Limit)
		} else {
			posts, err = s.repository.SearchPostsOrderBy(keyword, orderField, query.Cursor, query.Limit)
		}
	} else {
		sortAscending := query.Sort == "asc"
		if keyword == "" {
			posts, err = s.repository.GetPostsCursor(query.Cursor, query.Limit, sortAscending)
		} else {
			posts, err = s.repository.SearchPostsCursor(keyword, query.Cursor, query.Limit, sortAscending)
		}
	}
	if err != nil {
		return PostPage{}, err
	}
	s.enrichPosts(ctx, posts, SourceLocal)
	return PostPage{Items: summarizePosts(posts), NextCursor: postCursor(posts, orderField), HasMore: len(posts) == query.Limit}, contextError(ctx)
}

func (s *PostService) listLive(ctx context.Context, query PostQuery, parsed ParsedPostQuery) (PostPage, error) {
	if s == nil || s.remote == nil {
		return PostPage{}, errRemoteUnavailable
	}
	page := query.Cursor + 1
	if query.Cursor <= 0 {
		page = 1
	}
	posts, total, err := s.remote.ListPosts(ctx, RemoteListQuery{
		Page:          page,
		Limit:         query.Limit,
		CommentLimit:  10,
		CommentStream: 1,
		PID:           parsed.PID,
		Keyword:       parsed.Keyword,
		Label:         query.Label,
		Followed:      parsed.IsFollow,
	})
	if err != nil {
		return PostPage{}, err
	}
	if query.HasMedia != nil {
		posts = filterPostsWithMedia(posts, *query.HasMedia)
	}
	s.enrichPosts(ctx, posts, SourceLive)
	return PostPage{Items: summarizePosts(posts), NextCursor: page, HasMore: page*query.Limit < total}, contextError(ctx)
}

func (s *PostService) enrichPosts(ctx context.Context, posts []models.Post, source string) {
	cache := make(map[int32]*models.Post)
	for i := range posts {
		mentionedPID := MentionedPostID(posts[i])
		if mentionedPID <= 0 {
			continue
		}
		mentioned, found := cache[mentionedPID]
		if !found {
			var err error
			switch source {
			case SourceLocal:
				mentioned, err = s.repository.GetPostByPid(mentionedPID)
			case SourceLive:
				mentioned, err = s.remote.GetPost(ctx, mentionedPID)
			}
			if err != nil || mentioned == nil || mentioned.Pid != mentionedPID {
				mentioned = nil
			}
			cache[mentionedPID] = mentioned
		}
		posts[i].MentionedPost = mentioned
	}
}

func (s *PostService) uploadImages(ctx context.Context, paths []string) ([]string, error) {
	ids := make([]string, 0, len(paths))
	for _, path := range paths {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		resolved, err := NormalizeUploadImagePath(path)
		if err != nil {
			return nil, err
		}
		id, err := s.remote.UploadImage(ctx, resolved)
		if err != nil {
			return nil, fmt.Errorf("upload image %s: %w", path, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func NormalizeUploadImagePath(path string) (string, error) {
	path = strings.Trim(strings.TrimSpace(path), `"'`)
	if path == "" {
		return "", errors.New("image path cannot be empty")
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("image path is a directory: %s", path)
	}
	return path, nil
}

func (s *PostService) requireRemote(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil || s.remote == nil {
		return errRemoteUnavailable
	}
	return nil
}

func normalizePostQuery(query PostQuery) PostQuery {
	query.Source = normalizeSource(query.Source)
	query.Sort = strings.ToLower(strings.TrimSpace(query.Sort))
	query.Limit = normalizeLimit(query.Limit, defaultPostLimit)
	return query
}

func normalizeCommentQuery(query CommentQuery) CommentQuery {
	query.Source = normalizeSource(query.Source)
	query.Sort = strings.ToLower(strings.TrimSpace(query.Sort))
	if query.Sort == "" || query.Sort == "asc" || query.Sort == "0" {
		query.Sort = "asc"
	} else {
		query.Sort = "desc"
	}
	query.Limit = normalizeLimit(query.Limit, defaultCommentLimit)
	return query
}

func normalizeSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" || source == "offline" {
		return SourceLocal
	}
	if source == "online" {
		return SourceLive
	}
	return source
}

func normalizeLimit(limit, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > maxPageLimit {
		return maxPageLimit
	}
	return limit
}

func normalizeOrderField(sort string) string {
	switch sort {
	case "reply", "likenum", "praise_num":
		return sort
	default:
		return ""
	}
}

func summarizePosts(posts []models.Post) []PostSummary {
	items := make([]PostSummary, len(posts))
	for i := range posts {
		items[i] = PostSummary{Post: posts[i]}
	}
	return items
}

func lastCommentCursor(comments []models.Comment) int32 {
	if len(comments) == 0 {
		return 0
	}
	return comments[len(comments)-1].Cid
}

func postCursor(posts []models.Post, orderField string) int {
	if len(posts) == 0 {
		return 0
	}
	last := posts[len(posts)-1]
	switch orderField {
	case "reply":
		return int(last.Reply)
	case "likenum":
		return int(last.Likenum)
	case "praise_num":
		return int(last.PraiseNum)
	default:
		return int(last.Pid)
	}
}

func filterPostsWithMedia(posts []models.Post, want bool) []models.Post {
	filtered := make([]models.Post, 0, len(posts))
	for _, post := range posts {
		hasMedia := post.Type == "image" || strings.TrimSpace(post.MediaIds) != ""
		if hasMedia == want {
			filtered = append(filtered, post)
		}
	}
	return filtered
}
