package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	var availabilityRows []models.PostAvailability
	if err := d.db.WithContext(ctx).Where("pid IN ?", matchedPIDs).Find(&availabilityRows).Error; err != nil {
		return nil, err
	}
	for _, availability := range availabilityRows {
		if index, ok := indexByPID[availability.PID]; ok {
			records[index].Availability = portableAvailability(availability)
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
	var tagRows []struct {
		PID   int32  `gorm:"column:pid"`
		Name  string `gorm:"column:name"`
		Color string `gorm:"column:color"`
	}
	if err := d.db.WithContext(ctx).Table("post_tags").Select("post_tags.pid, local_tags.name, local_tags.color").Joins("JOIN local_tags ON local_tags.id = post_tags.tag_id").Where("post_tags.pid IN ?", matchedPIDs).Order("post_tags.pid ASC, local_tags.name ASC").Scan(&tagRows).Error; err != nil {
		return nil, err
	}
	for _, tag := range tagRows {
		if index, ok := indexByPID[tag.PID]; ok {
			records[index].Studio.Tags = append(records[index].Studio.Tags, archivepkg.PortableLocalTag{Name: tag.Name, Color: tag.Color})
		}
	}
	commentPID := make(map[int32]int32, len(comments))
	commentIDs := make([]int32, 0, len(comments))
	for _, comment := range comments {
		commentPID[comment.Cid] = comment.Pid
		commentIDs = append(commentIDs, comment.Cid)
	}
	var notes []models.Note
	noteQuery := d.db.WithContext(ctx).Where("owner_type = ? AND owner_id IN ?", "post", matchedPIDs)
	if len(commentIDs) > 0 {
		noteQuery = noteQuery.Or("owner_type = ? AND owner_id IN ?", "comment", commentIDs)
	}
	if err := noteQuery.Find(&notes).Error; err != nil {
		return nil, err
	}
	for _, note := range notes {
		if note.OwnerType == "post" {
			if index, ok := indexByPID[int32(note.OwnerID)]; ok {
				records[index].Studio.Note = note.Content
			}
			continue
		}
		pid, ok := commentPID[int32(note.OwnerID)]
		if !ok {
			continue
		}
		index := indexByPID[pid]
		if records[index].Studio.CommentNotes == nil {
			records[index].Studio.CommentNotes = make(map[string]string)
		}
		records[index].Studio.CommentNotes[fmt.Sprint(note.OwnerID)] = note.Content
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

func portableAvailability(row models.PostAvailability) *archivepkg.AvailabilityMetadata {
	value := &archivepkg.AvailabilityMetadata{
		State: row.State, ObservedAt: row.ObservedAt.UTC().Format(time.RFC3339),
		Completeness: row.Completeness, SnapshotID: row.SnapshotID,
		ObserverID: row.ObserverID, RemoteInstanceID: row.RemoteInstanceID,
	}
	if row.FirstUnavailableAt != nil {
		value.FirstUnavailableAt = row.FirstUnavailableAt.UTC().Format(time.RFC3339)
	}
	if row.LastUnavailableAt != nil {
		value.LastUnavailableAt = row.LastUnavailableAt.UTC().Format(time.RFC3339)
	}
	if row.RestoredAt != nil {
		value.RestoredAt = row.RestoredAt.UTC().Format(time.RFC3339)
	}
	return value
}

func (a *archiveTransaction) UpsertAvailability(ctx context.Context, records []archivepkg.AvailabilityRecord) error {
	for _, record := range records {
		observedAt, err := time.Parse(time.RFC3339, record.ObservedAt)
		if err != nil {
			return err
		}
		row := models.PostAvailability{
			PID: record.PID, State: record.State, ObservedAt: observedAt.UTC(),
			Completeness: record.Completeness, SnapshotID: record.SnapshotID,
			ObserverID: record.ObserverID, RemoteInstanceID: record.RemoteInstanceID,
			UpdatedAt: time.Now().UTC(),
		}
		if row.Completeness == "" {
			row.Completeness = "unknown"
		}
		if row.ObserverID == "" {
			row.ObserverID = "archive"
		}
		if row.RemoteInstanceID == "" {
			row.RemoteInstanceID = "archive"
		}
		parseOptional := func(raw string) (*time.Time, error) {
			if raw == "" {
				return nil, nil
			}
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return nil, err
			}
			parsed = parsed.UTC()
			return &parsed, nil
		}
		if row.FirstUnavailableAt, err = parseOptional(record.FirstUnavailableAt); err != nil {
			return err
		}
		if row.LastUnavailableAt, err = parseOptional(record.LastUnavailableAt); err != nil {
			return err
		}
		if row.RestoredAt, err = parseOptional(record.RestoredAt); err != nil {
			return err
		}
		var current models.PostAvailability
		err = a.tx.WithContext(ctx).Where("pid = ?", row.PID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := a.tx.WithContext(ctx).Create(&row).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if row.ObservedAt.Before(current.ObservedAt) {
			continue
		}
		if current.FirstUnavailableAt != nil && (row.FirstUnavailableAt == nil || current.FirstUnavailableAt.Before(*row.FirstUnavailableAt)) {
			row.FirstUnavailableAt = current.FirstUnavailableAt
		}
		if row.State == "restored" {
			if row.LastUnavailableAt == nil {
				row.LastUnavailableAt = current.LastUnavailableAt
			}
			if row.SnapshotID == "" {
				row.SnapshotID = current.SnapshotID
			}
		} else if row.State == "confirmed_unavailable" {
			row.RestoredAt = nil
		}
		if row.Completeness == "" {
			row.Completeness = current.Completeness
		}
		updates := map[string]any{
			"state": row.State, "observed_at": row.ObservedAt, "first_unavailable_at": row.FirstUnavailableAt,
			"last_unavailable_at": row.LastUnavailableAt, "restored_at": row.RestoredAt,
			"observer_id": row.ObserverID, "remote_instance_id": row.RemoteInstanceID,
			"completeness": row.Completeness, "snapshot_id": row.SnapshotID, "updated_at": row.UpdatedAt,
		}
		if err := a.tx.WithContext(ctx).Model(&models.PostAvailability{}).Where("pid = ?", row.PID).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *archiveTransaction) UpsertPosts(ctx context.Context, posts []models.Post) error {
	if len(posts) == 0 {
		return nil
	}
	sanitizePosts(posts)
	pids := make([]int32, 0, len(posts))
	for _, post := range posts {
		pids = append(pids, post.Pid)
	}
	var existing []models.Post
	if err := a.tx.WithContext(ctx).Where("pid IN ?", pids).Find(&existing).Error; err != nil {
		return err
	}
	byPID := make(map[int32]models.Post, len(existing))
	for _, post := range existing {
		byPID[post.Pid] = post
	}
	for index := range posts {
		if current, ok := byPID[posts[index].Pid]; ok {
			mergeArchivePost(&posts[index], current)
		}
	}
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
	cids := make([]int32, 0, len(comments))
	for _, comment := range comments {
		cids = append(cids, comment.Cid)
	}
	var existing []models.Comment
	if err := a.tx.WithContext(ctx).Where("cid IN ?", cids).Find(&existing).Error; err != nil {
		return err
	}
	byCID := make(map[int32]models.Comment, len(existing))
	for _, comment := range existing {
		byCID[comment.Cid] = comment
	}
	for index := range comments {
		if current, ok := byCID[comments[index].Cid]; ok {
			mergeArchiveComment(&comments[index], current)
		}
	}
	return a.tx.WithContext(ctx).Omit("Quote").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cid"}},
		UpdateAll: true,
	}).CreateInBatches(comments, 100).Error
}

// Archive imports may come from older or partial captures. Preserve fields
// that carry real content when an incoming sparse record omits them, and keep
// monotonic engagement counters from moving backwards.
func mergeArchivePost(incoming *models.Post, current models.Post) {
	if incoming.Text == "" {
		incoming.Text = current.Text
	}
	if incoming.Type == "" {
		incoming.Type = current.Type
	}
	if incoming.Timestamp == 0 {
		incoming.Timestamp = current.Timestamp
	}
	if incoming.TagsIds == "" {
		incoming.TagsIds = current.TagsIds
	}
	if incoming.AutoTagsIds == "" {
		incoming.AutoTagsIds = current.AutoTagsIds
	}
	if incoming.MgrTagsIds == "" {
		incoming.MgrTagsIds = current.MgrTagsIds
	}
	if incoming.MediaIds == "" {
		incoming.MediaIds = current.MediaIds
	}
	if incoming.IdentityType == "" {
		incoming.IdentityType = current.IdentityType
	}
	if incoming.IdentityInfo == "" {
		incoming.IdentityInfo = current.IdentityInfo
	}
	if incoming.ExclusiveIdInfo == "" {
		incoming.ExclusiveIdInfo = current.ExclusiveIdInfo
	}
	if incoming.Mention == "" {
		incoming.Mention = current.Mention
	}
	if incoming.ImageSize == "" {
		incoming.ImageSize = current.ImageSize
	}
	incoming.Reply = max(incoming.Reply, current.Reply)
	incoming.Likenum = max(incoming.Likenum, current.Likenum)
	incoming.PraiseNum = max(incoming.PraiseNum, current.PraiseNum)
	incoming.PraiseNumShow = max(incoming.PraiseNumShow, current.PraiseNumShow)
	incoming.TreadNum = max(incoming.TreadNum, current.TreadNum)
	incoming.FoldNum = max(incoming.FoldNum, current.FoldNum)
}

func mergeArchiveComment(incoming *models.Comment, current models.Comment) {
	if incoming.Pid == 0 {
		incoming.Pid = current.Pid
	}
	if incoming.Text == "" {
		incoming.Text = current.Text
	}
	if incoming.Timestamp == 0 {
		incoming.Timestamp = current.Timestamp
	}
	if incoming.IdentityType == "" {
		incoming.IdentityType = current.IdentityType
	}
	if incoming.IdentityInfo == "" {
		incoming.IdentityInfo = current.IdentityInfo
	}
	if incoming.ExclusiveIdInfo == "" {
		incoming.ExclusiveIdInfo = current.ExclusiveIdInfo
	}
	if incoming.Mention == "" {
		incoming.Mention = current.Mention
	}
	if incoming.NameTag == "" {
		incoming.NameTag = current.NameTag
	}
	if incoming.MediaIds == "" {
		incoming.MediaIds = current.MediaIds
	}
	if incoming.QuoteID == nil {
		incoming.QuoteID = current.QuoteID
	}
	incoming.RewardGood = max(incoming.RewardGood, current.RewardGood)
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

func (a *archiveTransaction) MergeLocalMetadata(ctx context.Context, records []archivepkg.Record) error {
	now := time.Now().UTC()
	for _, record := range records {
		for _, portable := range record.Studio.Tags {
			name := strings.TrimSpace(portable.Name)
			if name == "" {
				continue
			}
			tag := models.LocalTag{Name: name, Color: strings.TrimSpace(portable.Color), CreatedAt: now, UpdatedAt: now}
			if err := a.tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&tag).Error; err != nil {
				return err
			}
			if err := a.tx.WithContext(ctx).Where("name = ?", name).First(&tag).Error; err != nil {
				return err
			}
			if err := a.tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&models.PostTag{PID: record.PID, TagID: tag.ID, CreatedAt: now}).Error; err != nil {
				return err
			}
		}
		if strings.TrimSpace(record.Studio.Note) != "" {
			note := models.Note{OwnerType: "post", OwnerID: int64(record.PID), Content: record.Studio.Note, CreatedAt: now, UpdatedAt: now}
			if err := a.tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_type"}, {Name: "owner_id"}}, DoNothing: true}).Create(&note).Error; err != nil {
				return err
			}
		}
		validComments := make(map[int32]bool, len(record.Comments))
		for _, comment := range record.Comments {
			validComments[comment.Cid] = true
		}
		for rawCID, content := range record.Studio.CommentNotes {
			cid64, err := strconv.ParseInt(rawCID, 10, 32)
			if err != nil || !validComments[int32(cid64)] || strings.TrimSpace(content) == "" {
				continue
			}
			note := models.Note{OwnerType: "comment", OwnerID: cid64, Content: content, CreatedAt: now, UpdatedAt: now}
			if err := a.tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_type"}, {Name: "owner_id"}}, DoNothing: true}).Create(&note).Error; err != nil {
				return err
			}
		}
	}
	return nil
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
