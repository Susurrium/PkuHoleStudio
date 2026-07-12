package archive

import (
	"context"
	"errors"
	"io"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type Importer struct {
	store Store
}

func NewImporter(store Store) *Importer { return &Importer{store: store} }

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
		return tx.SaveImportRun(ctx, run)
	}); err != nil {
		return ImportReport{}, err
	}
	return ImportReport{
		Format: preflight.Format, Status: preflight.Status, ArchiveHash: preflight.ArchiveHash,
		RunID: preflight.RunID, Counts: preflight.Counts, Issues: preflight.Issues,
	}, nil
}

func importReportFromPreflight(preflight PreflightReport) ImportReport {
	return ImportReport{
		Format: preflight.Format, Status: preflight.Status, ArchiveHash: preflight.ArchiveHash,
		RunID: preflight.RunID, Counts: preflight.Counts, Issues: preflight.Issues,
	}
}
