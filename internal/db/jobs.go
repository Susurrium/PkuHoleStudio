package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ jobs.Store = (*Database)(nil)

func (d *Database) CreateJob(ctx context.Context, job jobs.Job) error {
	return d.db.WithContext(ctx).Create(jobToModel(job)).Error
}

func (d *Database) GetJob(ctx context.Context, id string) (jobs.Job, error) {
	var row models.Job
	if err := d.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return jobs.Job{}, jobs.ErrNotFound
		}
		return jobs.Job{}, err
	}
	return jobFromModel(row), nil
}

func (d *Database) ListJobs(ctx context.Context, limit int) ([]jobs.Job, error) {
	var rows []models.Job
	query := d.db.WithContext(ctx).Order("created_at DESC, id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]jobs.Job, len(rows))
	for i := range rows {
		result[i] = jobFromModel(rows[i])
	}
	return result, nil
}

func (d *Database) MutateJob(ctx context.Context, id string, mutate func(*jobs.Job) error) (jobs.Job, error) {
	if mutate == nil {
		return jobs.Job{}, errors.New("job mutation is required")
	}
	var result jobs.Job
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row models.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return jobs.ErrNotFound
			}
			return err
		}
		job := jobFromModel(row)
		if err := mutate(&job); err != nil {
			return err
		}
		updated := jobToModel(job)
		if err := tx.Model(&models.Job{}).Where("id = ?", id).Select("*").Updates(updated).Error; err != nil {
			return err
		}
		result = job
		return nil
	})
	return result, err
}

func (d *Database) UpsertItem(ctx context.Context, item jobs.Item) error {
	row := models.JobItem{
		JobID:      item.JobID,
		ItemKey:    item.Key,
		Status:     string(item.Status),
		Attempts:   item.Attempts,
		Checkpoint: string(item.Checkpoint),
		Error:      item.Error,
		UpdatedAt:  item.UpdatedAt,
	}
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "job_id"}, {Name: "item_key"}},
		UpdateAll: true,
	}).Create(&row).Error
}

func (d *Database) ListItems(ctx context.Context, jobID string) ([]jobs.Item, error) {
	var rows []models.JobItem
	if err := d.db.WithContext(ctx).Where("job_id = ?", jobID).Order("item_key ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]jobs.Item, len(rows))
	for i, row := range rows {
		result[i] = jobs.Item{
			JobID:      row.JobID,
			Key:        row.ItemKey,
			Status:     jobs.ItemStatus(row.Status),
			Attempts:   row.Attempts,
			Checkpoint: cloneJSON(row.Checkpoint),
			Error:      row.Error,
			UpdatedAt:  row.UpdatedAt,
		}
	}
	return result, nil
}

func (d *Database) ResetFailedItems(ctx context.Context, jobID string) error {
	return d.db.WithContext(ctx).Model(&models.JobItem{}).
		Where("job_id = ? AND status = ?", jobID, string(jobs.ItemFailed)).
		Updates(map[string]any{
			"status":     string(jobs.ItemQueued),
			"error":      "",
			"updated_at": time.Now().UTC(),
		}).Error
}

func (d *Database) AppendEvent(ctx context.Context, event jobs.Event) (jobs.Event, error) {
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var jobRow models.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&jobRow, "id = ?", event.JobID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return jobs.ErrNotFound
			}
			return err
		}
		var sequence int64
		if err := tx.Model(&models.JobEvent{}).Where("job_id = ?", event.JobID).
			Select("COALESCE(MAX(sequence), 0)").Scan(&sequence).Error; err != nil {
			return err
		}
		event.Sequence = sequence + 1
		if event.CreatedAt.IsZero() {
			event.CreatedAt = time.Now().UTC()
		}
		return tx.Create(&models.JobEvent{
			JobID:     event.JobID,
			Sequence:  event.Sequence,
			Type:      event.Type,
			DataJSON:  string(event.Data),
			CreatedAt: event.CreatedAt,
		}).Error
	})
	return event, err
}

func (d *Database) ListEvents(ctx context.Context, jobID string, afterSequence int64, limit int) ([]jobs.Event, error) {
	var rows []models.JobEvent
	query := d.db.WithContext(ctx).Where("job_id = ? AND sequence > ?", jobID, afterSequence).Order("sequence ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]jobs.Event, len(rows))
	for i, row := range rows {
		result[i] = jobs.Event{
			JobID:     row.JobID,
			Sequence:  row.Sequence,
			Type:      row.Type,
			Data:      cloneJSON(row.DataJSON),
			CreatedAt: row.CreatedAt,
		}
	}
	return result, nil
}

func (d *Database) RecoverRunningToPaused(ctx context.Context) ([]jobs.Job, error) {
	var recovered []jobs.Job
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []models.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status = ?", string(jobs.StatusRunning)).Find(&rows).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for i := range rows {
			rows[i].Status = string(jobs.StatusPaused)
			rows[i].Error = "process restarted"
			rows[i].UpdatedAt = now
			if err := tx.Model(&models.Job{}).Where("id = ?", rows[i].ID).Updates(map[string]any{
				"status":     rows[i].Status,
				"error":      rows[i].Error,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
			recovered = append(recovered, jobFromModel(rows[i]))
		}
		return nil
	})
	return recovered, err
}

func jobToModel(job jobs.Job) *models.Job {
	return &models.Job{
		ID:             job.ID,
		Type:           string(job.Type),
		Status:         string(job.Status),
		PayloadJSON:    string(job.Payload),
		CheckpointJSON: string(job.Checkpoint),
		CompletedItems: job.CompletedItems,
		FailedItems:    job.FailedItems,
		TotalItems:     job.TotalItems,
		Attempts:       job.Attempts,
		Error:          job.Error,
		StartedAt:      job.StartedAt,
		FinishedAt:     job.FinishedAt,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
	}
}

func jobFromModel(row models.Job) jobs.Job {
	return jobs.Job{
		ID:             row.ID,
		Type:           jobs.Type(row.Type),
		Status:         jobs.Status(row.Status),
		Payload:        cloneJSON(row.PayloadJSON),
		Checkpoint:     cloneJSON(row.CheckpointJSON),
		CompletedItems: row.CompletedItems,
		FailedItems:    row.FailedItems,
		TotalItems:     row.TotalItems,
		Attempts:       row.Attempts,
		Error:          row.Error,
		StartedAt:      row.StartedAt,
		FinishedAt:     row.FinishedAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func cloneJSON(value string) json.RawMessage {
	if value == "" {
		return nil
	}
	if !json.Valid([]byte(value)) {
		return json.RawMessage(fmt.Sprintf("%q", value))
	}
	return append(json.RawMessage(nil), value...)
}
