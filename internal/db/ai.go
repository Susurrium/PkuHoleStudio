package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"

	"gorm.io/gorm"
)

func (d *Database) CreateAISession(ctx context.Context, session models.AISession) error {
	return d.db.WithContext(ctx).Create(&session).Error
}

func (d *Database) ListAISessions(ctx context.Context, limit int) ([]models.AISession, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows []models.AISession
	err := d.db.WithContext(ctx).Order("updated_at DESC, created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (d *Database) GetAISession(ctx context.Context, id string) (models.AISession, error) {
	var row models.AISession
	err := d.db.WithContext(ctx).First(&row, "id = ?", id).Error
	return row, err
}

func (d *Database) UpdateAISessionScope(ctx context.Context, id, scopeJSON string) error {
	return d.db.WithContext(ctx).Model(&models.AISession{}).Where("id = ?", id).Updates(map[string]any{
		"scope_json": scopeJSON,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (d *Database) ListAIMessages(ctx context.Context, sessionID string) ([]models.AIMessage, error) {
	var rows []models.AIMessage
	err := d.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at ASC, CASE WHEN ordinal > 0 THEN ordinal ELSE 9223372036854775807 END ASC, CASE WHEN role = 'user' THEN 0 ELSE 1 END ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (d *Database) ListAISources(ctx context.Context, messageID string) ([]models.AISource, error) {
	var rows []models.AISource
	err := d.db.WithContext(ctx).Where("message_id = ?", messageID).Order("ordinal ASC").Find(&rows).Error
	return rows, err
}

func (d *Database) SaveAIMessage(ctx context.Context, message models.AIMessage, sources []models.AISource) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		if len(sources) > 0 {
			if err := tx.CreateInBatches(sources, 100).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.AISession{}).Where("id = ?", message.SessionID).Update("updated_at", time.Now().UTC()).Error
	})
}

func (d *Database) CreateAIRun(ctx context.Context, run models.AIRun) error {
	return d.db.WithContext(ctx).Create(&run).Error
}

func (d *Database) UpdateAIRun(ctx context.Context, id, status, message string, finished bool) error {
	updates := map[string]any{"status": status, "error": message, "updated_at": time.Now().UTC()}
	if finished {
		now := time.Now().UTC()
		updates["finished_at"] = &now
	}
	return d.db.WithContext(ctx).Model(&models.AIRun{}).Where("id = ?", id).Updates(updates).Error
}

func (d *Database) AppendAIEvent(ctx context.Context, event models.AIEventRecord) error {
	return d.db.WithContext(ctx).Create(&event).Error
}

func (d *Database) ListAIEvents(ctx context.Context, runID string, afterSequence int64) ([]models.AIEventRecord, error) {
	var rows []models.AIEventRecord
	err := d.db.WithContext(ctx).Where("run_id = ? AND sequence > ?", runID, afterSequence).Order("sequence ASC").Find(&rows).Error
	return rows, err
}

func (d *Database) LatestAIRun(ctx context.Context, sessionID string) (*models.AIRun, error) {
	var row models.AIRun
	err := d.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at DESC, id DESC").First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &row, err
}

func (d *Database) SaveAIQueries(ctx context.Context, runID string, queries []models.AIQuery) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("run_id = ?", runID).Delete(&models.AIQuery{}).Error; err != nil {
			return err
		}
		if len(queries) == 0 {
			return nil
		}
		now := time.Now().UTC()
		for index := range queries {
			queries[index].RunID = runID
			queries[index].Ordinal = index + 1
			queries[index].CreatedAt = now
		}
		return tx.CreateInBatches(queries, 100).Error
	})
}

func (d *Database) ListAIQueries(ctx context.Context, runID string) ([]models.AIQuery, error) {
	var rows []models.AIQuery
	err := d.db.WithContext(ctx).Where("run_id = ?", runID).Order("ordinal ASC").Find(&rows).Error
	return rows, err
}

func (d *Database) RecoverRunningAIRuns(ctx context.Context) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var runs []models.AIRun
		if err := tx.Where("status = ?", "running").Find(&runs).Error; err != nil {
			return err
		}
		for _, run := range runs {
			var sequence int64
			if err := tx.Model(&models.AIEventRecord{}).Where("run_id = ?", run.ID).Select("COALESCE(MAX(sequence), 0)").Scan(&sequence).Error; err != nil {
				return err
			}
			payload, _ := json.Marshal(map[string]any{"message": "AI run was interrupted because PkuHoleStudio restarted"})
			if err := tx.Create(&models.AIEventRecord{RunID: run.ID, Sequence: sequence + 1, Type: "error", DataJSON: string(payload), CreatedAt: time.Now().UTC()}).Error; err != nil {
				return err
			}
			now := time.Now().UTC()
			if err := tx.Model(&models.AIRun{}).Where("id = ?", run.ID).Updates(map[string]any{
				"status": "interrupted", "error": "AI run was interrupted because PkuHoleStudio restarted", "finished_at": &now, "updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
