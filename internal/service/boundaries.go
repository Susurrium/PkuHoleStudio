package service

import (
	"context"
	"io"
)

type ArchiveFormat string

const (
	ArchiveFormatLegacyJSON ArchiveFormat = "legacy-json"
	ArchiveFormatTreeholeV2 ArchiveFormat = "treehole-v2"
)

type ArchivePreflight struct {
	Format        ArchiveFormat
	SchemaVersion int
	PostCount     int
	CommentCount  int
	Warnings      []string
}

type ArchiveImportReport struct {
	RunID            string
	ImportedPosts    int
	ImportedComments int
	SkippedRecords   int
	Errors           []string
}

// ArchiveService is implemented in Phase 2. The boundary lives here so App,
// CLI, and HTTP handlers never depend on a ZIP or JSON implementation.
type ArchiveService interface {
	Preflight(ctx context.Context, reader io.ReaderAt, size int64) (ArchivePreflight, error)
	Import(ctx context.Context, reader io.ReaderAt, size int64) (ArchiveImportReport, error)
}

type AIRequest struct {
	SessionID string
	Mode      string
	Prompt    string
	PIDs      []int32
}

type AIEvent struct {
	Type string
	Data any
}

// AIService is intentionally provider-neutral. Phase 4 supplies the concrete
// OpenAI-compatible implementation and persistence.
type AIService interface {
	Run(ctx context.Context, request AIRequest) (<-chan AIEvent, error)
	Cancel(sessionID string) error
}
