package service

import (
	"context"
	"io"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"
)

type ArchiveFormat = archive.Format
type ArchivePreflight = archive.PreflightReport
type ArchiveImportReport = archive.ImportReport
type ArchiveExportRequest = archive.ExportRequest
type ArchiveExportReport = archive.ExportReport
type ArchiveExportFormat = archive.ExportFormat

const (
	ArchiveFormatLegacyJSON = archive.FormatLegacyV1
	ArchiveFormatTreeholeV2 = archive.FormatV2
	ArchiveExportTreeholeV2 = archive.ExportFormatTreeholeV2
	ArchiveExportMarkdown   = archive.ExportFormatMarkdown
)

// ArchiveService is implemented in Phase 2. The boundary lives here so App,
// CLI, and HTTP handlers never depend on a ZIP or JSON implementation.
type ArchiveService interface {
	Preflight(ctx context.Context, reader io.ReaderAt, size int64) (ArchivePreflight, error)
	Import(ctx context.Context, reader io.ReaderAt, size int64) (ArchiveImportReport, error)
	Export(ctx context.Context, writer io.Writer, request ArchiveExportRequest) (ArchiveExportReport, error)
}

type AIRequest struct {
	SessionID string
	Mode      string
	Prompt    string
	PIDs      []int32
	Course    string
	Teachers  []string
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
