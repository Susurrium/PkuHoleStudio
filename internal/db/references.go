package db

import "github.com/Susurrium/PkuHoleStudio/internal/models"

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
		ORDER BY r.id ASC`, pid, pid).Scan(&rows).Error
	return rows, err
}
