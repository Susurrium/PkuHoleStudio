package db

import (
	"context"

	archivepkg "github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"gorm.io/gorm"
)

func (d *Database) GetReferencesByPID(pid int32) ([]models.ReferenceEdge, error) {
	if pid <= 0 {
		return []models.ReferenceEdge{}, nil
	}
	var rows []models.ReferenceEdge
	err := d.db.Raw(`SELECT r.kind,
		CASE WHEN r.source_type = 'post' THEN r.source_id ELSE source_comment.pid END AS source_pid,
		CASE WHEN r.source_type = 'comment' THEN r.source_id ELSE NULL END AS source_cid,
		CASE WHEN r.target_type = 'post' THEN r.target_id ELSE target_comment.pid END AS target_pid,
		CASE WHEN r.target_type = 'comment' THEN r.target_id ELSE NULL END AS target_cid
		FROM "references" r
		LEFT JOIN comments source_comment ON r.source_type = 'comment' AND source_comment.cid = r.source_id
		LEFT JOIN comments target_comment ON r.target_type = 'comment' AND target_comment.cid = r.target_id
		WHERE (r.source_type = 'post' AND r.source_id = ?)
		   OR (r.source_type = 'comment' AND source_comment.pid = ?)
		   OR (r.target_type = 'post' AND r.target_id = ?)
		   OR (r.target_type = 'comment' AND target_comment.pid = ?)
		ORDER BY r.id ASC`, pid, pid, pid, pid).Scan(&rows).Error
	return rows, err
}

func (d *Database) RebuildReferences(ctx context.Context) (int, error) {
	var posts []models.Post
	if err := d.db.WithContext(ctx).Select("pid", "text").Find(&posts).Error; err != nil {
		return 0, err
	}
	var comments []models.Comment
	if err := d.db.WithContext(ctx).Select("cid", "pid", "text", "quote_id").Find(&comments).Error; err != nil {
		return 0, err
	}
	knownTargets := make(map[int32]bool, len(posts))
	for _, post := range posts {
		knownTargets[post.Pid] = true
	}
	references := make([]archivepkg.Reference, 0)
	for _, post := range posts {
		references = append(references, archivepkg.ExtractTextReferences(post.Pid, nil, post.Text, knownTargets)...)
	}
	for _, comment := range comments {
		references = append(references, archivepkg.ExtractTextReferences(comment.Pid, &comment.Cid, comment.Text, knownTargets)...)
		if comment.QuoteID != nil {
			target := *comment.QuoteID
			references = append(references, archivepkg.Reference{Kind: "quoted_comment", SourcePID: comment.Pid, SourceCID: &comment.Cid, TargetPID: comment.Pid, TargetCID: &target})
		}
	}
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("kind IN ?", []string{"explicit", "inferred", "quoted_comment", "mentions", "quotes"}).Delete(&models.Reference{}).Error; err != nil {
			return err
		}
		return (&archiveTransaction{tx: tx}).UpsertReferences(ctx, references)
	})
	return len(references), err
}
