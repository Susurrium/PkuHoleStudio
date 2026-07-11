package service

import "github.com/Susurrium/PkuHoleStudio/internal/models"

const (
	// SourceLocal reads from the local archive repository.
	SourceLocal = "local"
	// SourceLive reads from the authenticated Treehole API.
	SourceLive = "live"
)

// PostQuery describes a cursor-based post list or search request. Cursor is an
// opaque value: local repositories use a PID while the live API uses a page
// number.
type PostQuery struct {
	Cursor   int      `json:"cursor,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	Query    string   `json:"query,omitempty"`
	Source   string   `json:"source,omitempty"`
	Sort     string   `json:"sort,omitempty"`
	HasMedia *bool    `json:"has_media,omitempty"`
	Label    int      `json:"label,omitempty"`
	From     int64    `json:"from,omitempty"`
	To       int64    `json:"to,omitempty"`
	Origins  []string `json:"origins,omitempty"`
	TagIDs   []uint   `json:"tag_ids,omitempty"`
}

// ParsedPostQuery is the service-level interpretation of the compact query
// syntax shared by the TUI and Web API.
type ParsedPostQuery struct {
	PID      int32 `json:"pid,omitempty"`
	Keyword  string
	IsFollow *bool `json:"is_follow,omitempty"`
}

// PostSummary wraps the existing post model while leaving room for local
// metadata and search snippets without changing models.Post.
type PostSummary struct {
	models.Post
	Snippet        string                    `json:"snippet,omitempty"`
	Score          float64                   `json:"score,omitempty"`
	CommentMatches []models.CommentSearchHit `json:"comment_matches,omitempty"`
}

type PostPage struct {
	Items      []PostSummary `json:"items"`
	NextCursor int           `json:"next_cursor,omitempty"`
	HasMore    bool          `json:"has_more"`
}

type PostDetail struct {
	Post              models.Post      `json:"post"`
	Comments          []models.Comment `json:"comments"`
	References        []Reference      `json:"references"`
	NextCommentCursor int32            `json:"next_comment_cursor,omitempty"`
	HasMoreComments   bool             `json:"has_more_comments"`
}

// CommentQuery describes a cursor-based comment request. As with PostQuery,
// Cursor is a CID locally and a page number for the live source.
type CommentQuery struct {
	Cursor int32  `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Sort   string `json:"sort,omitempty"`
	Source string `json:"source,omitempty"`
}

type CommentPage struct {
	Items      []models.Comment `json:"items"`
	NextCursor int32            `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

// Reference represents a local relationship between posts or comments. A nil
// CID means that endpoint of the relationship is a post.
type Reference struct {
	Kind      string `json:"kind,omitempty"`
	SourcePID int32  `json:"source_pid"`
	SourceCID *int32 `json:"source_cid,omitempty"`
	TargetPID int32  `json:"target_pid"`
	TargetCID *int32 `json:"target_cid,omitempty"`
}
