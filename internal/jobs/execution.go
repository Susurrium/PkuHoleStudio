package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type Execution struct {
	manager *Manager
	jobID   string
}

func (e *Execution) JobID() string { return e.jobID }

func (e *Execution) Checkpoint(ctx context.Context, checkpoint any) error {
	payload, err := marshalOptional(checkpoint)
	if err != nil {
		return err
	}
	_, err = e.manager.store.MutateJob(ctx, e.jobID, func(job *Job) error {
		job.Checkpoint = payload
		job.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err == nil {
		_, err = e.manager.emit(ctx, e.jobID, "checkpoint", checkpoint)
	}
	return err
}

func (e *Execution) SetTotal(ctx context.Context, total int) error {
	if total < 0 {
		return errors.New("total items cannot be negative")
	}
	_, err := e.manager.store.MutateJob(ctx, e.jobID, func(job *Job) error {
		job.TotalItems = total
		job.UpdatedAt = time.Now().UTC()
		return nil
	})
	return err
}

func (e *Execution) Emit(ctx context.Context, eventType string, data any) (Event, error) {
	return e.manager.emit(ctx, e.jobID, eventType, data)
}

func (e *Execution) Items(ctx context.Context) ([]Item, error) {
	return e.manager.store.ListItems(ctx, e.jobID)
}

func (e *Execution) ItemSucceeded(ctx context.Context, key string, checkpoint any) error {
	return e.saveItem(ctx, key, ItemCompleted, checkpoint, "")
}

func (e *Execution) ItemFailed(ctx context.Context, key string, failure error) error {
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	return e.saveItem(ctx, key, ItemFailed, nil, message)
}

func (e *Execution) saveItem(ctx context.Context, key string, status ItemStatus, checkpoint any, message string) error {
	if key == "" {
		return errors.New("item key is required")
	}
	payload, err := marshalOptional(checkpoint)
	if err != nil {
		return err
	}
	items, err := e.manager.store.ListItems(ctx, e.jobID)
	if err != nil {
		return err
	}
	attempts := 1
	for _, item := range items {
		if item.Key == key {
			attempts = item.Attempts + 1
			break
		}
	}
	if err := e.manager.store.UpsertItem(ctx, Item{
		JobID:      e.jobID,
		Key:        key,
		Status:     status,
		Attempts:   attempts,
		Checkpoint: payload,
		Error:      message,
		UpdatedAt:  time.Now().UTC(),
	}); err != nil {
		return err
	}
	items, err = e.manager.store.ListItems(ctx, e.jobID)
	if err != nil {
		return err
	}
	completed, failed := 0, 0
	for _, item := range items {
		switch item.Status {
		case ItemCompleted:
			completed++
		case ItemFailed:
			failed++
		}
	}
	_, err = e.manager.store.MutateJob(ctx, e.jobID, func(job *Job) error {
		job.CompletedItems = completed
		job.FailedItems = failed
		if job.TotalItems < len(items) {
			job.TotalItems = len(items)
		}
		job.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return err
	}
	_, err = e.manager.emit(ctx, e.jobID, "item_"+string(status), map[string]any{"key": key, "error": message})
	return err
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), raw...)
}
