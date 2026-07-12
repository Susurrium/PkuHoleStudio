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
var _ archivepkg.ExportStore = (*Database)(nil)

func (d *Database) ArchiveExportSnapshot(ctx context.Context, pids []int32) ([]archivepkg.ExportRecord, error) {
	if d == nil || d.db == nil {
		return nil, errors.New("database is not initialized")
	}
	postQuery := d.db.WithContext(ctx).Order("pid ASC")
	if len(pids) > 0 {
		postQuery = postQuery.Where("pid IN ?", pids)
	}
	var posts []models.Post
	if err := postQuery.Find(&posts).Error; err != nil {
		return nil, err
	}
	if len(posts) == 0 {
		return []archivepkg.ExportRecord{}, nil
	}
	matchedPIDs := make([]int32, len(posts))
	records := make([]archivepkg.ExportRecord, len(posts))
	indexByPID := make(map[int32]int, len(posts))
	for index, post := range posts {
		matchedPIDs[index] = post.Pid
		indexByPID[post.Pid] = index
		records[index].Post = post
	}
	var comments []models.Comment
	if err := d.db.WithContext(ctx).Where("pid IN ?", matchedPIDs).Order("pid ASC, cid ASC").Find(&comments).Error; err != nil {
		return nil, err
	}
	for _, comment := range comments {
		if index, ok := indexByPID[comment.Pid]; ok {
			records[index].Comments = append(records[index].Comments, comment)
		}
	}
	var sources []models.PostSource
	if err := d.db.WithContext(ctx).Where("pid IN ?", matchedPIDs).Order("pid ASC, source ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	for _, source := range sources {
		if index, ok := indexByPID[source.PID]; ok {
			records[index].Sources = append(records[index].Sources, source)
		}
	}
	var media []models.Media
	if err := d.db.WithContext(ctx).Where("(owner_type = ? AND owner_id IN ?) OR (owner_type = ? AND owner_id IN (SELECT cid FROM comments WHERE pid IN ?))", "post", matchedPIDs, "comment", matchedPIDs).Order("owner_type ASC, owner_id ASC, variant ASC").Find(&media).Error; err != nil {
		return nil, err
	}
	for _, item := range media {
		if item.OwnerType == "post" {
			if index, ok := indexByPID[int32(item.OwnerID)]; ok {
				records[index].Media = append(records[index].Media, item)
			}
			continue
		}
		for index := range records {
			for _, comment := range records[index].Comments {
				if int64(comment.Cid) == item.OwnerID {
					records[index].Media = append(records[index].Media, item)
					break
				}
			}
		}
	}
	return records, nil
}

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

func (a *archiveTransaction) UpsertMedia(ctx context.Context, media []archivepkg.MediaRecord) error {
	rows := make([]models.Media, 0, len(media))
	now := time.Now().UTC()
	for _, item := range media {
		rows = append(rows, models.Media{
			RemoteID: item.RemoteID, RemoteURL: item.RemoteURL, ContentHash: item.SHA256,
			OwnerType: item.OwnerType, OwnerID: item.OwnerID, Variant: item.Variant,
			Path: item.Path, MIMEType: item.MIMEType, Size: item.Size,
			Width: item.Width, Height: item.Height, Status: item.Status,
			CreatedAt: now, UpdatedAt: now,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return a.tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "owner_type"}, {Name: "owner_id"}, {Name: "variant"}, {Name: "remote_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"remote_url":   gorm.Expr("CASE WHEN excluded.remote_url <> '' THEN excluded.remote_url ELSE media.remote_url END"),
			"content_hash": gorm.Expr("CASE WHEN excluded.content_hash <> '' THEN excluded.content_hash ELSE media.content_hash END"),
			"path":         gorm.Expr("CASE WHEN excluded.path <> '' THEN excluded.path ELSE media.path END"),
			"mime_type":    gorm.Expr("CASE WHEN excluded.mime_type <> '' THEN excluded.mime_type ELSE media.mime_type END"),
			"size":         gorm.Expr("CASE WHEN excluded.size > 0 THEN excluded.size ELSE media.size END"),
			"width":        gorm.Expr("CASE WHEN excluded.width > 0 THEN excluded.width ELSE media.width END"),
			"height":       gorm.Expr("CASE WHEN excluded.height > 0 THEN excluded.height ELSE media.height END"),
			"status":       gorm.Expr("CASE WHEN excluded.status = 'available' THEN excluded.status ELSE media.status END"),
			"updated_at":   now,
		}),
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
