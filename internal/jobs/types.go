package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type Type string

const (
	TypeSyncFollowed       Type = "sync_followed"
	TypeSyncPIDs           Type = "sync_pids"
	TypeSyncLatest         Type = "sync_latest"
	TypeRepairComments     Type = "repair_comments"
	TypeRepairMedia        Type = "repair_media"
	TypeImportArchive      Type = "import_archive"
	TypeRebuildSearchIndex Type = "rebuild_search_index"
	TypeRebuildReferences  Type = "rebuild_references"
	TypeSyncPages          Type = "sync_pages"
	TypeMonitorLatest      Type = "monitor_latest"
	TypeRepairThumbnails   Type = "repair_thumbnails"
	TypeCleanupStaging     Type = "cleanup_staging"
	TypeExportArchive      Type = "export_archive"
)

func (t Type) Valid() bool {
	switch t {
	case TypeSyncFollowed, TypeSyncPIDs, TypeSyncLatest, TypeRepairComments,
		TypeRepairMedia, TypeImportArchive, TypeRebuildSearchIndex, TypeRebuildReferences,
		TypeSyncPages, TypeMonitorLatest, TypeRepairThumbnails, TypeCleanupStaging, TypeExportArchive:
		return true
	default:
		return false
	}
}

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusPartial   Status = "partial"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

func (s Status) Terminal() bool {
	return s == StatusCompleted || s == StatusPartial || s == StatusFailed || s == StatusCancelled
}

func CanTransition(from, to Status) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusQueued:
		return to == StatusRunning || to == StatusPaused || to == StatusCancelled || to == StatusFailed
	case StatusRunning:
		return to == StatusPaused || to == StatusCompleted || to == StatusPartial || to == StatusFailed || to == StatusCancelled
	case StatusPaused:
		return to == StatusQueued || to == StatusCancelled
	case StatusPartial, StatusFailed, StatusCancelled:
		return to == StatusQueued
	default:
		return false
	}
}

type Job struct {
	ID             string          `json:"id"`
	Type           Type            `json:"type"`
	Status         Status          `json:"status"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	Checkpoint     json.RawMessage `json:"checkpoint,omitempty"`
	CompletedItems int             `json:"completed_items"`
	FailedItems    int             `json:"failed_items"`
	TotalItems     int             `json:"total_items"`
	Attempts       int             `json:"attempts"`
	Error          string          `json:"error,omitempty"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	FinishedAt     *time.Time      `json:"finished_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type ItemStatus string

const (
	ItemQueued    ItemStatus = "queued"
	ItemRunning   ItemStatus = "running"
	ItemCompleted ItemStatus = "completed"
	ItemFailed    ItemStatus = "failed"
)

type Item struct {
	JobID      string          `json:"job_id"`
	Key        string          `json:"key"`
	Status     ItemStatus      `json:"status"`
	Attempts   int             `json:"attempts"`
	Checkpoint json.RawMessage `json:"checkpoint,omitempty"`
	Error      string          `json:"error,omitempty"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type Event struct {
	JobID     string          `json:"job_id"`
	Sequence  int64           `json:"sequence"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type CreateRequest struct {
	Type       Type
	Payload    any
	TotalItems int
}

type Store interface {
	CreateJob(ctx context.Context, job Job) error
	GetJob(ctx context.Context, id string) (Job, error)
	ListJobs(ctx context.Context, limit int) ([]Job, error)
	MutateJob(ctx context.Context, id string, mutate func(*Job) error) (Job, error)

	UpsertItem(ctx context.Context, item Item) error
	ListItems(ctx context.Context, jobID string) ([]Item, error)
	ResetFailedItems(ctx context.Context, jobID string) error

	AppendEvent(ctx context.Context, event Event) (Event, error)
	ListEvents(ctx context.Context, jobID string, afterSequence int64, limit int) ([]Event, error)
	RecoverRunningToPaused(ctx context.Context) ([]Job, error)
}

var (
	ErrNotFound          = errors.New("job not found")
	ErrInvalidTransition = errors.New("invalid job status transition")
	ErrClosed            = errors.New("job manager is closed")
)

type Handler func(ctx context.Context, execution *Execution, job Job) error
