package archive

import (
	"context"
	"encoding/json"
	"io"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type ExportFormat string

const (
	ExportFormatTreeholeV2 ExportFormat = "treehole-v2"
	ExportFormatMarkdown   ExportFormat = "markdown"
)

type ExportRequest struct {
	Format          ExportFormat
	PIDs            []int32
	IncludeComments bool
}

type ExportReport struct {
	Format       ExportFormat `json:"format"`
	Posts        int          `json:"posts"`
	Comments     int          `json:"comments"`
	Media        int          `json:"media"`
	MissingMedia int          `json:"missing_media"`
	RunID        string       `json:"run_id"`
}

type ExportRecord struct {
	Post     models.Post
	Comments []models.Comment
	Sources  []models.PostSource
	Media    []models.Media
	Studio   StudioMetadata
}

type PortableLocalTag struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// StudioMetadata is an optional, backwards-compatible data.json extension.
// Toolkit and older Studio builds ignore this unknown field.
type StudioMetadata struct {
	Tags         []PortableLocalTag `json:"tags,omitempty"`
	Note         string             `json:"note,omitempty"`
	CommentNotes map[string]string  `json:"commentNotes,omitempty"`
}

type ExportStore interface {
	ArchiveExportSnapshot(ctx context.Context, pids []int32) ([]ExportRecord, error)
}

type Exporter interface {
	Export(ctx context.Context, writer io.Writer, request ExportRequest) (ExportReport, error)
}

const (
	MaxArchiveBytes      int64 = 200 << 20
	MaxUncompressedBytes int64 = 500 << 20
	MaxMediaBytes        int64 = 50 << 20
)

type Format string

const (
	FormatLegacyV1 Format = "legacy-v1"
	FormatV2       Format = "v2"
)

type Status string

const (
	StatusCompleted Status = "completed"
	StatusPartial   Status = "partial"
	StatusFailed    Status = "failed"
	StatusDuplicate Status = "duplicate"
)

type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Issue identifies a structural or record-level archive problem. Path uses a
// JSON-style location so the Web client can point users at the bad record.
type Issue struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Path     string   `json:"path,omitempty"`
	PID      int32    `json:"pid,omitempty"`
	CID      int32    `json:"cid,omitempty"`
}

type Counts struct {
	Items            int `json:"items"`
	ValidItems       int `json:"valid_items"`
	SkippedItems     int `json:"skipped_items"`
	ContextOnly      int `json:"context_only"`
	ArchivedComments int `json:"archived_comments"`
	Comments         int `json:"comments"`
	SkippedComments  int `json:"skipped_comments"`
	Sources          int `json:"sources"`
	References       int `json:"references"`
	Media            int `json:"media"`
	AvailableMedia   int `json:"available_media"`
	MissingMedia     int `json:"missing_media"`
}

type ManifestCounts struct {
	ExpectedHoles *int `json:"expectedHoles"`
	ExportedHoles *int `json:"exportedHoles"`
	Comments      *int `json:"comments"`
	Failed        *int `json:"failed"`
}

type Manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	ToolVersion   string            `json:"toolVersion"`
	RunID         string            `json:"runId"`
	ExportedAt    string            `json:"exportedAt"`
	Scope         json.RawMessage   `json:"scope"`
	Complete      bool              `json:"complete"`
	Counts        ManifestCounts    `json:"counts"`
	Errors        []json.RawMessage `json:"errors"`
}

type PreflightReport struct {
	Format      Format    `json:"format"`
	Status      Status    `json:"status"`
	ArchiveHash string    `json:"hash"`
	RunID       string    `json:"run_id"`
	Counts      Counts    `json:"counts"`
	Issues      []Issue   `json:"issues"`
	Manifest    *Manifest `json:"manifest,omitempty"`
	records     []Record
	media       []MediaRecord
}

type ImportReport struct {
	Format      Format  `json:"format"`
	Status      Status  `json:"status"`
	ArchiveHash string  `json:"hash"`
	RunID       string  `json:"run_id"`
	Counts      Counts  `json:"counts"`
	Issues      []Issue `json:"issues"`
	Duplicate   bool    `json:"duplicate,omitempty"`
}

// Record is a validated archive item. ContextOnly records are intentionally
// excluded from the post/comment upsert batches.
type Record struct {
	PID         int32
	Source      string
	FetchStatus string
	Post        models.Post
	Comments    []models.Comment
	ContextOnly bool
	References  []Reference
	Studio      StudioMetadata
}

// MediaRecord is the portable representation used by Studio's backward-
// compatible archive v2 media extension. Content is populated only while an
// archive is being parsed and is never exposed through API reports.
type MediaRecord struct {
	OwnerType string `json:"ownerType"`
	OwnerID   int64  `json:"ownerId"`
	RemoteID  string `json:"remoteId,omitempty"`
	RemoteURL string `json:"remoteUrl,omitempty"`
	Variant   string `json:"variant"`
	Path      string `json:"path,omitempty"`
	MIMEType  string `json:"mimeType,omitempty"`
	Size      int64  `json:"size,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Status    string `json:"status"`
	Content   []byte `json:"-"`
}

type PostSource struct {
	PID         int32
	Source      string
	ArchiveHash string
	RunID       string
	ContextOnly bool
}

type Reference struct {
	Kind      string
	SourcePID int32
	SourceCID *int32
	TargetPID int32
	TargetCID *int32
}

// ImportRun is the idempotence record. Store implementations must consider a
// run found when either ArchiveHash or a non-empty RunID matches a prior run.
type ImportRun struct {
	Format      Format
	Status      Status
	ArchiveHash string
	RunID       string
	Counts      Counts
	Issues      []Issue
}

// Store and Transaction are consumer-side boundaries. UpsertPosts and
// UpsertComments update only remote/archive-owned model fields; implementations
// must not overwrite local tags, notes, AI sessions, or other local metadata.
type Store interface {
	FindImport(ctx context.Context, archiveHash, runID string) (ImportRun, bool, error)
	Transaction(ctx context.Context, fn func(Transaction) error) error
}

type Transaction interface {
	UpsertPosts(ctx context.Context, posts []models.Post) error
	UpsertComments(ctx context.Context, comments []models.Comment) error
	UpsertSources(ctx context.Context, sources []PostSource) error
	UpsertReferences(ctx context.Context, references []Reference) error
	UpsertMedia(ctx context.Context, media []MediaRecord) error
	SaveImportRun(ctx context.Context, run ImportRun) error
}

type LocalMetadataTransaction interface {
	MergeLocalMetadata(ctx context.Context, records []Record) error
}
