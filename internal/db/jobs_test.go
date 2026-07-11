package db

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
)

func TestDatabaseJobStoreRoundTripAndRecovery(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	job := jobs.Job{
		ID:        "job-1",
		Type:      jobs.TypeSyncPIDs,
		Status:    jobs.StatusRunning,
		Payload:   []byte(`{"pids":[12345]}`),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := database.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	loaded, err := database.GetJob(ctx, job.ID)
	if err != nil || loaded.Type != job.Type || loaded.Status != jobs.StatusRunning {
		t.Fatalf("GetJob() = %+v, error = %v", loaded, err)
	}
	mutated, err := database.MutateJob(ctx, job.ID, func(job *jobs.Job) error {
		job.Checkpoint = []byte(`{"page":2}`)
		job.TotalItems = 2
		return nil
	})
	if err != nil || string(mutated.Checkpoint) != `{"page":2}` || mutated.TotalItems != 2 {
		t.Fatalf("MutateJob() = %+v, error = %v", mutated, err)
	}

	recovered, err := database.RecoverRunningToPaused(ctx)
	if err != nil || len(recovered) != 1 || recovered[0].Status != jobs.StatusPaused {
		t.Fatalf("RecoverRunningToPaused() = %+v, error = %v", recovered, err)
	}
	loaded, _ = database.GetJob(ctx, job.ID)
	if loaded.Status != jobs.StatusPaused {
		t.Fatalf("persisted recovered status = %s", loaded.Status)
	}
}

func TestDatabaseJobStoreItemsAndFailedReset(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now().UTC()
	if err := database.CreateJob(ctx, jobs.Job{ID: "items", Type: jobs.TypeSyncPIDs, Status: jobs.StatusQueued, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	for _, item := range []jobs.Item{
		{JobID: "items", Key: "ok", Status: jobs.ItemCompleted, Attempts: 1, UpdatedAt: now},
		{JobID: "items", Key: "bad", Status: jobs.ItemFailed, Attempts: 2, Error: "boom", UpdatedAt: now},
	} {
		if err := database.UpsertItem(ctx, item); err != nil {
			t.Fatalf("UpsertItem(%s) error = %v", item.Key, err)
		}
	}
	if err := database.ResetFailedItems(ctx, "items"); err != nil {
		t.Fatalf("ResetFailedItems() error = %v", err)
	}
	items, err := database.ListItems(ctx, "items")
	if err != nil || len(items) != 2 {
		t.Fatalf("ListItems() = %+v, error = %v", items, err)
	}
	statuses := map[string]jobs.ItemStatus{}
	for _, item := range items {
		statuses[item.Key] = item.Status
	}
	if statuses["ok"] != jobs.ItemCompleted || statuses["bad"] != jobs.ItemQueued {
		t.Fatalf("item statuses after reset = %+v", statuses)
	}
}

func TestDatabaseJobEventsUseContiguousSequence(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now().UTC()
	if err := database.CreateJob(ctx, jobs.Job{ID: "events", Type: jobs.TypeSyncLatest, Status: jobs.StatusQueued, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	const count = 20
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := database.AppendEvent(ctx, jobs.Event{JobID: "events", Type: "progress", Data: []byte(fmt.Sprintf(`{"i":%d}`, index))})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("AppendEvent() error = %v", err)
		}
	}
	events, err := database.ListEvents(ctx, "events", 0, 0)
	if err != nil || len(events) != count {
		t.Fatalf("ListEvents() count = %d, error = %v", len(events), err)
	}
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			t.Fatalf("event[%d].Sequence = %d", i, event.Sequence)
		}
	}
	after, err := database.ListEvents(ctx, "events", 10, 3)
	if err != nil || len(after) != 3 || after[0].Sequence != 11 {
		t.Fatalf("ListEvents(after) = %+v, error = %v", after, err)
	}
}
