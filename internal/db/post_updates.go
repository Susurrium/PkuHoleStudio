package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *Database) ReadPostUpdateSchedule(ctx context.Context, account string) (string, error) {
	var row models.PostUpdateSchedule
	err := d.db.WithContext(ctx).Where("account = ?", account).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return row.State, err
}
func (d *Database) WritePostUpdateSchedule(ctx context.Context, account, state string) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&models.PostUpdateSchedule{Account: account, State: state}).Error
}

func (d *Database) ComparePostUpdateSchedule(ctx context.Context, account, previous, next string) (bool, error) {
	result := d.db.WithContext(ctx).Model(&models.PostUpdateSchedule{}).Where("account = ? AND state = ?", account, previous).Update("state", next)
	return result.RowsAffected == 1, result.Error
}

func (d *Database) SavedMediaPreference(ctx context.Context, pid int32) (bool, error) {
	var rows []models.Job
	if err := d.db.WithContext(ctx).Where("type = ?", "sync_pids").Order("created_at DESC").Find(&rows).Error; err != nil {
		return false, err
	}
	for _, row := range rows {
		var payload struct {
			PreserveMediaOptions bool    `json:"preserve_media_options"`
			PIDs                 []int32 `json:"pids"`
			IncludeMedia         bool    `json:"include_media"`
		}
		if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err != nil {
			return false, err
		}
		if payload.PreserveMediaOptions {
			continue
		}
		for _, value := range payload.PIDs {
			if value == pid {
				return payload.IncludeMedia, nil
			}
		}
	}
	mediaRows, err := d.GetMediaByPID(pid)
	if err != nil {
		return false, err
	}
	for _, row := range mediaRows {
		if row.Status == "available" {
			return true, nil
		}
	}
	return false, nil
}
