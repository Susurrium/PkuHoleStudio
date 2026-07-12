package db

import (
	"context"
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

func (d *Database) ListAIMessages(ctx context.Context, sessionID string) ([]models.AIMessage, error) {
	var rows []models.AIMessage
	err := d.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at ASC, id ASC").Find(&rows).Error
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
