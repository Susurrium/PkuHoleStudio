package archive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type Importer struct {
	store       Store
	exportStore ExportStore
	dataDir     string
}

func NewImporter(store Store) *Importer {
	return NewImporterWithDataDir(store, "data")
}

func NewImporterWithDataDir(store Store, dataDir string) *Importer {
	exporter, _ := store.(ExportStore)
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "data"
	}
	return &Importer{store: store, exportStore: exporter, dataDir: dataDir}
}

func (i *Importer) Preflight(ctx context.Context, reader io.ReaderAt, size int64) (PreflightReport, error) {
	return Parse(ctx, reader, size)
}

func (i *Importer) Import(ctx context.Context, reader io.ReaderAt, size int64) (ImportReport, error) {
	if i == nil || i.store == nil {
		return ImportReport{}, errors.New("archive store is not configured")
	}
	preflight, err := Parse(ctx, reader, size)
	if err != nil {
		return ImportReport{}, err
	}
	if preflight.Counts.ValidItems == 0 {
		return importReportFromPreflight(preflight), errors.New("archive contains no valid items")
	}
	if previous, found, err := i.store.FindImport(ctx, preflight.ArchiveHash, preflight.RunID); err != nil {
		return ImportReport{}, err
	} else if found {
		return ImportReport{
			Format: previous.Format, Status: StatusDuplicate, ArchiveHash: previous.ArchiveHash,
			RunID: previous.RunID, Counts: previous.Counts, Issues: previous.Issues, Duplicate: true,
		}, nil
	}

	posts := make([]models.Post, 0, preflight.Counts.ValidItems)
	comments := make([]models.Comment, 0, preflight.Counts.Comments)
	sources := make([]PostSource, 0, len(preflight.records))
	references := make([]Reference, 0, preflight.Counts.References)
	for _, record := range preflight.records {
		sources = append(sources, PostSource{
			PID: record.PID, Source: record.Source, ArchiveHash: preflight.ArchiveHash,
			RunID: preflight.RunID, ContextOnly: record.ContextOnly,
		})
		references = append(references, record.References...)
		posts = append(posts, record.Post)
		comments = append(comments, record.Comments...)
	}
	media, createdFiles, err := i.persistArchiveMedia(ctx, preflight.media)
	if err != nil {
		return ImportReport{}, err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, path := range createdFiles {
			_ = os.Remove(path)
		}
	}()

	run := ImportRun{
		Format: preflight.Format, Status: preflight.Status, ArchiveHash: preflight.ArchiveHash,
		RunID: preflight.RunID, Counts: preflight.Counts, Issues: preflight.Issues,
	}
	if err := i.store.Transaction(ctx, func(tx Transaction) error {
		if err := tx.UpsertPosts(ctx, posts); err != nil {
			return err
		}
		if err := tx.UpsertComments(ctx, comments); err != nil {
			return err
		}
		if err := tx.UpsertSources(ctx, sources); err != nil {
			return err
		}
		if err := tx.UpsertReferences(ctx, references); err != nil {
			return err
		}
		if err := tx.UpsertMedia(ctx, media); err != nil {
			return err
		}
		return tx.SaveImportRun(ctx, run)
	}); err != nil {
		return ImportReport{}, err
	}
	committed = true
	return ImportReport{
		Format: preflight.Format, Status: preflight.Status, ArchiveHash: preflight.ArchiveHash,
		RunID: preflight.RunID, Counts: preflight.Counts, Issues: preflight.Issues,
	}, nil
}

func (i *Importer) persistArchiveMedia(ctx context.Context, media []MediaRecord) ([]MediaRecord, []string, error) {
	result := append([]MediaRecord(nil), media...)
	created := make([]string, 0)
	for index := range result {
		if err := ctx.Err(); err != nil {
			return nil, created, err
		}
		item := &result[index]
		if item.Status != "available" || len(item.Content) == 0 {
			item.Path = ""
			item.Content = nil
			continue
		}
		ext := mediaExtension(item.MIMEType)
		relativePath := filepath.Join("media", "objects", item.SHA256+ext)
		absolutePath := filepath.Join(i.dataDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
			return nil, created, err
		}
		if existing, err := os.ReadFile(absolutePath); err == nil {
			digest := sha256.Sum256(existing)
			if hex.EncodeToString(digest[:]) != item.SHA256 {
				return nil, created, errors.New("existing media object failed its SHA-256 check")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, created, err
		} else {
			temporary, err := os.CreateTemp(filepath.Dir(absolutePath), ".import-media-*")
			if err != nil {
				return nil, created, err
			}
			temporaryPath := temporary.Name()
			if _, err := temporary.Write(item.Content); err != nil {
				temporary.Close()
				_ = os.Remove(temporaryPath)
				return nil, created, err
			}
			if err := temporary.Close(); err != nil {
				_ = os.Remove(temporaryPath)
				return nil, created, err
			}
			if err := os.Rename(temporaryPath, absolutePath); err != nil {
				_ = os.Remove(temporaryPath)
				return nil, created, err
			}
			created = append(created, absolutePath)
		}
		item.Path = filepath.ToSlash(relativePath)
		item.Content = nil
	}
	return result, created, nil
}

func mediaExtension(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func importReportFromPreflight(preflight PreflightReport) ImportReport {
	return ImportReport{
		Format: preflight.Format, Status: preflight.Status, ArchiveHash: preflight.ArchiveHash,
		RunID: preflight.RunID, Counts: preflight.Counts, Issues: preflight.Issues,
	}
}
