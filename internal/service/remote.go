package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

var errRemoteUnavailable = errors.New("treehole remote is not configured")

// RemoteListQuery is the service-owned representation of an online post list
// request. It deliberately keeps client.V3ListPostsParams out of the service
// boundary.
type RemoteListQuery struct {
	Page          int
	Limit         int
	CommentLimit  int
	CommentStream int
	PID           int32
	Keyword       string
	Label         int
	Kind          int
	Followed      *bool
}

// RemoteCommentQuery describes one page of comments from the online API.
type RemoteCommentQuery struct {
	Page   int
	Limit  int
	Sort   int
	Stream int
}

// CreatePostRequest is the provider-neutral input used to create an online
// post. IDs are slices here; the adapter owns the API's comma-separated wire
// representation.
type CreatePostRequest struct {
	Type         string
	Kind         int
	RewardCost   int
	Text         string
	IdentityShow int
	IdentityType string
	ExclusiveID  string
	Fold         int
	Mailbox      int
	TagIDs       []string
	MediaIDs     []string
}

// CreateCommentRequest is the provider-neutral input used to reply to a post.
type CreateCommentRequest struct {
	PID          int32
	QuoteID      *int32
	Text         string
	MediaIDs     []string
	IdentityShow int
	IdentityType string
}

// Remote is the online boundary consumed by application services. No request
// type from internal/client crosses this interface.
type Remote interface {
	ListPosts(ctx context.Context, query RemoteListQuery) ([]models.Post, int, error)
	GetPost(ctx context.Context, pid int32) (*models.Post, error)
	ListComments(ctx context.Context, pid int32, query RemoteCommentQuery) ([]models.Comment, int, error)
	ListTags(ctx context.Context) ([]models.Tag, error)
	GetCourseTable(ctx context.Context) ([]models.CourseScheduleRow, error)
	GetCourseScores(ctx context.Context) (*models.ScoreSummary, error)
	RefreshPost(ctx context.Context, pid int32) (*models.Post, error)
	TogglePraise(ctx context.Context, pid int32) error
	ToggleAttention(ctx context.Context, pid int32) error
	UploadImage(ctx context.Context, path string) (string, error)
	CreatePost(ctx context.Context, request CreatePostRequest) (*models.Post, error)
	CreateComment(ctx context.Context, request CreateCommentRequest) (*models.Comment, error)
	CanWrite(ctx context.Context) (bool, error)
}

// TreeholeRemote adapts client.Client to the service-owned Remote boundary.
type TreeholeRemote struct {
	client *client.Client
}

var _ Remote = (*TreeholeRemote)(nil)

func NewTreeholeRemote(c *client.Client) Remote {
	if c == nil {
		return nil
	}
	return &TreeholeRemote{client: c}
}

func (r *TreeholeRemote) ListPosts(ctx context.Context, query RemoteListQuery) ([]models.Post, int, error) {
	if err := contextError(ctx); err != nil {
		return nil, 0, err
	}
	if r == nil || r.client == nil {
		return nil, 0, errRemoteUnavailable
	}
	return r.client.ListPostsV3(client.V3ListPostsParams{
		Page:          query.Page,
		Limit:         query.Limit,
		CommentLimit:  query.CommentLimit,
		CommentStream: query.CommentStream,
		Pid:           query.PID,
		Keyword:       query.Keyword,
		Label:         query.Label,
		Kind:          query.Kind,
		IsFollow:      query.Followed,
	})
}

func (r *TreeholeRemote) GetPost(ctx context.Context, pid int32) (*models.Post, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if r == nil || r.client == nil {
		return nil, errRemoteUnavailable
	}
	return r.client.GetPostGet(pid)
}

func (r *TreeholeRemote) ListComments(ctx context.Context, pid int32, query RemoteCommentQuery) ([]models.Comment, int, error) {
	if err := contextError(ctx); err != nil {
		return nil, 0, err
	}
	if r == nil || r.client == nil {
		return nil, 0, errRemoteUnavailable
	}
	return r.client.ListCommentsV3(pid, query.Page, query.Limit, query.Sort, query.Stream)
}

func (r *TreeholeRemote) ListTags(ctx context.Context) ([]models.Tag, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if r == nil || r.client == nil {
		return nil, errRemoteUnavailable
	}
	return r.client.GetTagsTreeV3()
}

func (r *TreeholeRemote) GetCourseTable(ctx context.Context) ([]models.CourseScheduleRow, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if r == nil || r.client == nil {
		return nil, errRemoteUnavailable
	}
	return r.client.GetCourseTableV2()
}

func (r *TreeholeRemote) GetCourseScores(ctx context.Context) (*models.ScoreSummary, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if r == nil || r.client == nil {
		return nil, errRemoteUnavailable
	}
	return r.client.GetCourseScoresV2()
}

func (r *TreeholeRemote) RefreshPost(ctx context.Context, pid int32) (*models.Post, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if r == nil || r.client == nil {
		return nil, errRemoteUnavailable
	}
	return r.client.GetPostGet(pid)
}

func (r *TreeholeRemote) TogglePraise(ctx context.Context, pid int32) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if r == nil || r.client == nil {
		return errRemoteUnavailable
	}
	return r.client.TogglePraiseV3(pid)
}

func (r *TreeholeRemote) ToggleAttention(ctx context.Context, pid int32) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if r == nil || r.client == nil {
		return errRemoteUnavailable
	}
	return r.client.ToggleAttentionV3(pid)
}

func (r *TreeholeRemote) UploadImage(ctx context.Context, path string) (string, error) {
	if err := contextError(ctx); err != nil {
		return "", err
	}
	if r == nil || r.client == nil {
		return "", errRemoteUnavailable
	}
	return r.client.UploadImageV3(path)
}

func (r *TreeholeRemote) CreatePost(ctx context.Context, request CreatePostRequest) (*models.Post, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if r == nil || r.client == nil {
		return nil, errRemoteUnavailable
	}
	return r.client.CreatePostV3(client.CreatePostPayload{
		Type:         request.Type,
		Kind:         request.Kind,
		RewardCost:   request.RewardCost,
		Text:         request.Text,
		IdentityShow: request.IdentityShow,
		IdentityType: request.IdentityType,
		ExclusiveID:  request.ExclusiveID,
		Fold:         request.Fold,
		Mailbox:      request.Mailbox,
		TagsIDs:      strings.Join(request.TagIDs, ","),
		MediaIDs:     strings.Join(request.MediaIDs, ","),
	})
}

func (r *TreeholeRemote) CreateComment(ctx context.Context, request CreateCommentRequest) (*models.Comment, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if r == nil || r.client == nil {
		return nil, errRemoteUnavailable
	}
	return r.client.CreateCommentV3(client.CreateCommentPayload{
		PID:          request.PID,
		CommentID:    commentIDString(request.QuoteID),
		Text:         request.Text,
		MediaIDs:     strings.Join(request.MediaIDs, ","),
		IdentityShow: request.IdentityShow,
		IdentityType: request.IdentityType,
	})
}

func (r *TreeholeRemote) CanWrite(ctx context.Context) (bool, error) {
	if err := contextError(ctx); err != nil {
		return false, err
	}
	if r == nil || r.client == nil {
		return false, errRemoteUnavailable
	}
	return r.client.ProbeSession().CanWriteOnline, nil
}

func commentIDString(id *int32) *string {
	if id == nil {
		return nil
	}
	value := strconv.Itoa(int(*id))
	return &value
}
