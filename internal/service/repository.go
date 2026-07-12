package service

import "github.com/Susurrium/PkuHoleStudio/internal/models"

// Repository is the read-side subset of the local database consumed by the
// application services. *db.Database satisfies this interface structurally.
// Context cancellation is checked by the service before invoking these legacy
// methods; the database boundary can become context-aware in a later migration.
type Repository interface {
	GetPostsCursor(cursor int, limit int, sortAsc bool) ([]models.Post, error)
	SearchPostsCursor(keyword string, cursor int, limit int, sortAsc bool) ([]models.Post, error)
	GetPostByPid(pid int32) (*models.Post, error)
	GetCommentsByPidCursor(pid int32, cursor int32, limit int, sortAsc bool) ([]models.Comment, error)

	GetCommentByCid(cid int32) (*models.Comment, error)
	GetPostsOrderBy(field string, cursor int, limit int) ([]models.Post, error)
	SearchPostsOrderBy(keyword string, field string, cursor int, limit int) ([]models.Post, error)
	GetPostCount() (int, error)
	GetCommentCount() (int, error)
}

type ReferenceRepository interface {
	GetReferencesByPID(pid int32) ([]models.ReferenceEdge, error)
}
