package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	archivepkg "github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ archivepkg.Store = (*Database)(nil)

func (d *Database) FindImport(ctx context.Context, archiveHash, runID string) (archivepkg.ImportRun, bool, error) {
	var row models.ImportRun
	query := d.db.WithContext(ctx).Where("archive_hash = ?", archiveHash)
	if runID != "" {
		query = query.Or("archive_run_id = ?", runID)
	}
	if err := query.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return archivepkg.ImportRun{}, false, nil
		}
		return archivepkg.ImportRun{}, false, err
	}
	var report struct {
		Counts archivepkg.Counts  `json:"counts"`
		Issues []archivepkg.Issue `json:"issues"`
	}
	_ = json.Unmarshal([]byte(row.ReportJSON), &report)
	return archivepkg.ImportRun{
		Format: archivepkg.Format(row.Format), Status: archivepkg.Status(row.Status),
		ArchiveHash: row.ArchiveHash, RunID: row.ArchiveRunID,
		Counts: report.Counts, Issues: report.Issues,
	}, true, nil
}

func (d *Database) Transaction(ctx context.Context, fn func(archivepkg.Transaction) error) error {
	if fn == nil {
		return errors.New("archive transaction callback is required")
	}
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&archiveTransaction{tx: tx})
	})
}

type archiveTransaction struct{ tx *gorm.DB }

func (a *archiveTransaction) UpsertPosts(ctx context.Context, posts []models.Post) error {
	if len(posts) == 0 {
		return nil
	}
	sanitizePosts(posts)
	return a.tx.WithContext(ctx).Omit("Comments").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pid"}},
		UpdateAll: true,
	}).CreateInBatches(posts, 100).Error
}

func (a *archiveTransaction) UpsertComments(ctx context.Context, comments []models.Comment) error {
	if len(comments) == 0 {
		return nil
	}
	sanitizeComments(comments)
	return a.tx.WithContext(ctx).Omit("Quote").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cid"}},
		UpdateAll: true,
	}).CreateInBatches(comments, 100).Error
}

func (a *archiveTransaction) UpsertSources(ctx context.Context, sources []archivepkg.PostSource) error {
	now := time.Now().UTC()
	rows := make([]models.PostSource, 0, len(sources))
	for _, source := range sources {
		sourceRef := source.ArchiveHash
		if sourceRef == "" {
			sourceRef = source.RunID
		}
		rows = append(rows, models.PostSource{
			PID: source.PID, Source: source.Source, SourceRef: sourceRef,
			ContextOnly: source.ContextOnly, FirstSeenAt: now, LastSeenAt: now,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return a.tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "pid"}, {Name: "source"}, {Name: "source_ref"}},
		DoUpdates: clause.Assignments(map[string]any{
			"context_only": gorm.Expr("excluded.context_only"),
			"last_seen_at": now,
		}),
	}).CreateInBatches(rows, 100).Error
}

func (a *archiveTransaction) UpsertReferences(ctx context.Context, references []archivepkg.Reference) error {
	rows := make([]models.Reference, 0, len(references))
	for _, reference := range references {
		sourceType := "post"
		sourceID := int64(reference.SourcePID)
		if reference.SourceCID != nil {
			sourceType = "comment"
			sourceID = int64(*reference.SourceCID)
		}
		targetType := "post"
		targetID := int64(reference.TargetPID)
		if reference.TargetCID != nil {
			targetType = "comment"
			targetID = int64(*reference.TargetCID)
		}
		rows = append(rows, models.Reference{
			SourceType: sourceType, SourceID: sourceID, TargetType: targetType,
			TargetID: targetID, Kind: reference.Kind, CreatedAt: time.Now().UTC(),
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return a.tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "source_type"}, {Name: "source_id"}, {Name: "target_type"},
			{Name: "target_id"}, {Name: "kind"},
		},
		DoNothing: true,
	}).CreateInBatches(rows, 100).Error
}

func (a *archiveTransaction) SaveImportRun(ctx context.Context, run archivepkg.ImportRun) error {
	report, err := json.Marshal(map[string]any{"counts": run.Counts, "issues": run.Issues})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	id := run.ArchiveHash
	if len(id) > 32 {
		id = id[:32]
	}
	return a.tx.WithContext(ctx).Create(&models.ImportRun{
		ID: id, ArchiveRunID: run.RunID, ArchiveHash: run.ArchiveHash,
		Format: string(run.Format), Status: string(run.Status),
		ImportedPosts:    run.Counts.ValidItems,
		ImportedComments: run.Counts.Comments,
		SkippedRecords:   run.Counts.SkippedItems + run.Counts.SkippedComments,
		ReportJSON:       string(report), StartedAt: &now, FinishedAt: &now,
		CreatedAt: now, UpdatedAt: now,
	}).Error
}
