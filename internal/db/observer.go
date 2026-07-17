package db

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *Database) GetObserverSyncState(ctx context.Context, observerID string) (models.ObserverSyncState, bool, error) {
	if d == nil || d.db == nil {
		return models.ObserverSyncState{}, false, errors.New("database is not initialized")
	}
	var state models.ObserverSyncState
	err := d.db.WithContext(ctx).Where("observer_id = ?", observerID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ObserverSyncState{}, false, nil
	}
	return state, err == nil, err
}

func (d *Database) RecordObserverSyncFailure(ctx context.Context, observerID, instanceID, message string) error {
	if d == nil || d.db == nil {
		return errors.New("database is not initialized")
	}
	now := time.Now().UTC()
	row := models.ObserverSyncState{
		ObserverID: observerID, RemoteInstanceID: instanceID,
		LastError: strings.TrimSpace(message), UpdatedAt: now,
	}
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "observer_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"remote_instance_id": gorm.Expr("CASE WHEN excluded.remote_instance_id <> '' THEN excluded.remote_instance_id ELSE observer_sync_states.remote_instance_id END"),
			"last_error":         row.LastError, "updated_at": now,
		}),
	}).Create(&row).Error
}

func (d *Database) SaveObserverSyncState(ctx context.Context, state models.ObserverSyncState) error {
	if d == nil || d.db == nil {
		return errors.New("database is not initialized")
	}
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertObserverSyncState(tx, state)
	})
}

// ApplyObserverEvent commits the receipt, newer-wins availability state,
// Observer provenance, and cursor as one transaction. A replayed receipt with
// the same payload is success; a changed payload for the same event ID is an
// integrity error.
func (d *Database) ApplyObserverEvent(ctx context.Context, state models.ObserverSyncState, receipt models.ObserverEventReceipt, availability models.PostAvailability) (bool, error) {
	if d == nil || d.db == nil {
		return false, errors.New("database is not initialized")
	}
	applied := false
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existingReceipt models.ObserverEventReceipt
		err := tx.Where("observer_id = ? AND remote_instance_id = ? AND event_id = ?", receipt.ObserverID, receipt.RemoteInstanceID, receipt.EventID).First(&existingReceipt).Error
		if err == nil {
			if existingReceipt.PayloadHash != receipt.PayloadHash {
				return errors.New("observer event ID was replayed with a different payload")
			}
			return upsertObserverSyncState(tx, state)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if availability.PID > 0 && !availability.ObservedAt.IsZero() {
			var current models.PostAvailability
			err = tx.Where("pid = ?", availability.PID).First(&current).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				if err := tx.Create(&availability).Error; err != nil {
					return err
				}
			case err != nil:
				return err
			case !availability.ObservedAt.Before(current.ObservedAt):
				if current.FirstUnavailableAt != nil && (availability.FirstUnavailableAt == nil || current.FirstUnavailableAt.Before(*availability.FirstUnavailableAt)) {
					availability.FirstUnavailableAt = current.FirstUnavailableAt
				}
				if availability.State == "restored" {
					if availability.LastUnavailableAt == nil {
						availability.LastUnavailableAt = current.LastUnavailableAt
					}
					if availability.SnapshotID == "" {
						availability.SnapshotID = current.SnapshotID
					}
				} else if availability.State == "confirmed_unavailable" {
					availability.RestoredAt = nil
				}
				if availability.Completeness == "" {
					availability.Completeness = current.Completeness
				}
				updates := map[string]any{
					"state": availability.State, "observed_at": availability.ObservedAt,
					"first_unavailable_at": availability.FirstUnavailableAt, "last_unavailable_at": availability.LastUnavailableAt,
					"restored_at": availability.RestoredAt, "observer_id": availability.ObserverID,
					"remote_instance_id": availability.RemoteInstanceID, "completeness": availability.Completeness,
					"snapshot_id": availability.SnapshotID, "updated_at": availability.UpdatedAt,
				}
				if err := tx.Model(&models.PostAvailability{}).Where("pid = ?", availability.PID).Updates(updates).Error; err != nil {
					return err
				}
			}
			now := time.Now().UTC()
			source := models.PostSource{
				PID: availability.PID, Source: "observer", SourceRef: receipt.RemoteInstanceID,
				FirstSeenAt: now, LastSeenAt: now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "pid"}, {Name: "source"}, {Name: "source_ref"}},
				DoUpdates: clause.Assignments(map[string]any{"context_only": false, "last_seen_at": now}),
			}).Create(&source).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		if err := upsertObserverSyncState(tx, state); err != nil {
			return err
		}
		applied = true
		return nil
	})
	return applied, err
}

func upsertObserverSyncState(tx *gorm.DB, state models.ObserverSyncState) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "observer_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"remote_instance_id": state.RemoteInstanceID,
			"last_event_id":      state.LastEventID,
			"last_success_at":    state.LastSuccessAt,
			"last_error":         state.LastError,
			"updated_at":         state.UpdatedAt,
		}),
	}).Create(&state).Error
}

func (d *Database) ListPostAvailabilities(ctx context.Context, state, query string, offset, limit int) ([]models.PostAvailability, bool, error) {
	if d == nil || d.db == nil {
		return nil, false, errors.New("database is not initialized")
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	dbQuery := d.db.WithContext(ctx).Model(&models.PostAvailability{}).
		Joins("LEFT JOIN posts ON posts.pid = post_availability.pid")
	switch strings.TrimSpace(state) {
	case "", "confirmed_unavailable":
		dbQuery = dbQuery.Where("post_availability.state = ?", "confirmed_unavailable")
	case "restored":
		dbQuery = dbQuery.Where("post_availability.state = ?", "restored")
	case "all":
	default:
		return nil, false, errors.New("invalid availability state filter")
	}
	if query = strings.TrimSpace(query); query != "" {
		dbQuery = dbQuery.Where("CAST(post_availability.pid AS TEXT) LIKE ? OR posts.text LIKE ?", escapeLikePattern(query), escapeLikePattern(query))
	}
	var rows []models.PostAvailability
	if err := dbQuery.Order("post_availability.observed_at DESC, post_availability.pid DESC").Offset(offset).Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return rows, hasMore, nil
}

func (d *Database) GetPostAvailability(ctx context.Context, pid int32) (models.PostAvailability, bool, error) {
	if d == nil || d.db == nil {
		return models.PostAvailability{}, false, errors.New("database is not initialized")
	}
	var row models.PostAvailability
	err := d.db.WithContext(ctx).Where("pid = ?", pid).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PostAvailability{}, false, nil
	}
	return row, err == nil, err
}

func (d *Database) ObserverPostContents(ctx context.Context, pid int32) (*models.Post, []models.Comment, []models.Media, error) {
	if d == nil || d.db == nil {
		return nil, nil, nil, errors.New("database is not initialized")
	}
	var post models.Post
	if err := d.db.WithContext(ctx).Where("pid = ?", pid).First(&post).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, []models.Comment{}, []models.Media{}, nil
		}
		return nil, nil, nil, err
	}
	var comments []models.Comment
	if err := d.db.WithContext(ctx).Where("pid = ?", pid).Order("cid ASC").Find(&comments).Error; err != nil {
		return nil, nil, nil, err
	}
	commentIDs := make([]int32, 0, len(comments))
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.Cid)
	}
	mediaQuery := d.db.WithContext(ctx).Where("owner_type = ? AND owner_id = ?", "post", pid)
	if len(commentIDs) > 0 {
		mediaQuery = mediaQuery.Or("owner_type = ? AND owner_id IN ?", "comment", commentIDs)
	}
	var media []models.Media
	if err := mediaQuery.Order("owner_type ASC, owner_id ASC, id ASC").Find(&media).Error; err != nil {
		return nil, nil, nil, err
	}
	return &post, comments, media, nil
}
