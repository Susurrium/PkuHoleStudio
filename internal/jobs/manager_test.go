package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second

func TestManagerRunsJobsSequentially(t *testing.T) {
	store := newFakeStore()
	manager := newTestManager(t, store)

	var active atomic.Int32
	var maximum atomic.Int32
	started := make(chan string, 2)
	firstRelease := make(chan struct{})
	secondRelease := make(chan struct{})

	err := manager.Register(TypeSyncPIDs, func(ctx context.Context, _ *Execution, job Job) error {
		var payload struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return err
		}
		current := active.Add(1)
		defer active.Add(-1)
		for {
			old := maximum.Load()
			if current <= old || maximum.CompareAndSwap(old, current) {
				break
			}
		}
		started <- payload.Name
		var release <-chan struct{}
		switch payload.Name {
		case "first":
			release = firstRelease
		case "second":
			release = secondRelease
		default:
			return fmt.Errorf("unexpected job %q", payload.Name)
		}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	first, err := manager.Create(context.Background(), CreateRequest{
		Type:    TypeSyncPIDs,
		Payload: map[string]string{"name": "first"},
	})
	if err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	if got := receive(t, started); got != "first" {
		t.Fatalf("first started job = %q, want first", got)
	}

	second, err := manager.Create(context.Background(), CreateRequest{
		Type:    TypeSyncPIDs,
		Payload: map[string]string{"name": "second"},
	})
	if err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}
	queued, err := manager.Get(context.Background(), second.ID)
	if err != nil {
		t.Fatalf("Get(second) error = %v", err)
	}
	if queued.Status != StatusQueued {
		t.Fatalf("second status while first is blocked = %s, want %s", queued.Status, StatusQueued)
	}

	close(firstRelease)
	if got := receive(t, started); got != "second" {
		t.Fatalf("second started job = %q, want second", got)
	}
	secondEvents, err := manager.Events(context.Background(), second.ID, 0)
	if err != nil {
		t.Fatalf("Events(second) error = %v", err)
	}
	close(secondRelease)
	waitForEvent(t, secondEvents, "completed")

	if got := maximum.Load(); got != 1 {
		t.Fatalf("maximum concurrent handlers = %d, want 1", got)
	}
	for _, id := range []string{first.ID, second.ID} {
		job, err := manager.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("Get(%s) error = %v", id, err)
		}
		if job.Status != StatusCompleted {
			t.Errorf("job %s status = %s, want %s", id, job.Status, StatusCompleted)
		}
	}
}

func TestManagerWaitsForPersistedJobHandlerRegistration(t *testing.T) {
	store := newFakeStore()
	now := time.Now().UTC()
	store.seedJob(Job{ID: "persisted", Type: TypeSyncLatest, Status: StatusQueued, CreatedAt: now, UpdatedAt: now})
	manager := newTestManager(t, store)
	run := make(chan struct{}, 1)
	if err := manager.Register(TypeSyncLatest, func(context.Context, *Execution, Job) error {
		run <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	receiveSignal(t, run)
	events, err := manager.Events(context.Background(), "persisted", 0)
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	waitForEvent(t, events, "completed")
}

func TestManagerReconcilesQueuedJobsFromDurableStore(t *testing.T) {
	store := newFakeStore()
	manager := newTestManager(t, store)
	run := make(chan struct{}, 1)
	if err := manager.Register(TypeImportArchive, func(context.Context, *Execution, Job) error {
		run <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Simulate a queued row committed by the store while its in-memory wake-up
	// notification was lost. The scheduler must discover and run it.
	now := time.Now().UTC()
	store.seedJob(Job{ID: "missed-wakeup", Type: TypeImportArchive, Status: StatusQueued, CreatedAt: now, UpdatedAt: now})
	receiveSignal(t, run)

	events, err := manager.Events(context.Background(), "missed-wakeup", 0)
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	waitForEvent(t, events, "completed")
}

func TestManagerPauseAndResume(t *testing.T) {
	store := newFakeStore()
	manager := newTestManager(t, store)

	firstStarted := make(chan struct{})
	firstStopped := make(chan struct{})
	secondStarted := make(chan struct{})
	err := manager.Register(TypeSyncFollowed, func(ctx context.Context, _ *Execution, job Job) error {
		switch job.Attempts {
		case 1:
			close(firstStarted)
			<-ctx.Done()
			close(firstStopped)
			return ctx.Err()
		case 2:
			close(secondStarted)
			return nil
		default:
			return fmt.Errorf("unexpected attempt %d", job.Attempts)
		}
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	job, err := manager.Create(context.Background(), CreateRequest{Type: TypeSyncFollowed})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	receiveSignal(t, firstStarted)
	events, err := manager.Events(context.Background(), job.ID, 0)
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}

	paused, err := manager.Pause(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	if paused.Status != StatusPaused {
		t.Fatalf("Pause() status = %s, want %s", paused.Status, StatusPaused)
	}
	receiveSignal(t, firstStopped)

	queued, err := manager.Resume(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if queued.Status != StatusQueued {
		t.Fatalf("Resume() status = %s, want %s", queued.Status, StatusQueued)
	}
	receiveSignal(t, secondStarted)
	waitForEvent(t, events, "completed")

	completed, err := manager.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if completed.Status != StatusCompleted {
		t.Errorf("final status = %s, want %s", completed.Status, StatusCompleted)
	}
	if completed.Attempts != 2 {
		t.Errorf("attempts = %d, want 2", completed.Attempts)
	}
}

func TestManagerCancelRunningJob(t *testing.T) {
	store := newFakeStore()
	manager := newTestManager(t, store)

	started := make(chan struct{})
	stopped := make(chan struct{})
	err := manager.Register(TypeRepairMedia, func(ctx context.Context, _ *Execution, _ Job) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	job, err := manager.Create(context.Background(), CreateRequest{Type: TypeRepairMedia})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	receiveSignal(t, started)
	cancelled, err := manager.Cancel(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("Cancel() status = %s, want %s", cancelled.Status, StatusCancelled)
	}
	receiveSignal(t, stopped)

	stored, err := manager.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Status != StatusCancelled {
		t.Errorf("status after handler exits = %s, want %s", stored.Status, StatusCancelled)
	}
}

func TestManagerRetryPreservesSuccessfulItems(t *testing.T) {
	store := newFakeStore()
	now := time.Now().UTC()
	job := Job{
		ID:             "retry-job",
		Type:           TypeRepairComments,
		Status:         StatusFailed,
		CompletedItems: 1,
		FailedItems:    1,
		TotalItems:     2,
		Attempts:       1,
		Error:          "temporary failure",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	store.seedJob(job)
	store.seedItem(Item{JobID: job.ID, Key: "done", Status: ItemCompleted, Attempts: 1, UpdatedAt: now})
	store.seedItem(Item{JobID: job.ID, Key: "retry", Status: ItemFailed, Attempts: 1, Error: "temporary failure", UpdatedAt: now})

	manager := newTestManager(t, store)
	itemsSeen := make(chan []Item, 1)
	err := manager.Register(TypeRepairComments, func(ctx context.Context, execution *Execution, _ Job) error {
		items, err := execution.Items(ctx)
		if err != nil {
			return err
		}
		itemsSeen <- items
		for _, item := range items {
			if item.Key == "retry" {
				return execution.ItemSucceeded(ctx, item.Key, map[string]string{"result": "ok"})
			}
		}
		return errors.New("retry item is missing")
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	events, err := manager.Events(context.Background(), job.ID, 0)
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}

	retried, err := manager.Retry(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if retried.Status != StatusQueued {
		t.Fatalf("Retry() status = %s, want %s", retried.Status, StatusQueued)
	}
	seen := receive(t, itemsSeen)
	assertItemStatus(t, seen, "done", ItemCompleted)
	assertItemStatus(t, seen, "retry", ItemQueued)
	waitForEvent(t, events, "completed")

	items, err := store.ListItems(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	done := findItem(t, items, "done")
	retry := findItem(t, items, "retry")
	if done.Status != ItemCompleted || done.Attempts != 1 {
		t.Errorf("successful item changed on retry: status=%s attempts=%d", done.Status, done.Attempts)
	}
	if retry.Status != ItemCompleted || retry.Attempts != 2 {
		t.Errorf("retried item = status %s attempts %d, want completed/2", retry.Status, retry.Attempts)
	}
	stored, err := manager.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Status != StatusCompleted || stored.CompletedItems != 2 || stored.FailedItems != 0 {
		t.Errorf("retried job = status %s completed %d failed %d", stored.Status, stored.CompletedItems, stored.FailedItems)
	}
}

func TestManagerRecoversRunningJobsAsPaused(t *testing.T) {
	store := newFakeStore()
	now := time.Now().UTC()
	store.seedJob(Job{
		ID:        "interrupted-job",
		Type:      TypeImportArchive,
		Status:    StatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	})

	manager := newTestManager(t, store)
	job, err := manager.Get(context.Background(), "interrupted-job")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if job.Status != StatusPaused {
		t.Fatalf("recovered status = %s, want %s", job.Status, StatusPaused)
	}
	if calls := store.recoverCount(); calls != 1 {
		t.Fatalf("RecoverRunningToPaused() calls = %d, want 1", calls)
	}
	events, err := store.ListEvents(context.Background(), job.ID, 0, 0)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 1 || events[0].Sequence != 1 || events[0].Type != "paused" {
		t.Fatalf("recovery events = %+v, want one sequenced paused event", events)
	}
}

func TestManagerEventsReplayAndLiveDoNotDuplicate(t *testing.T) {
	store := newFakeStore()
	manager := newTestManager(t, store)

	started := make(chan struct{})
	emitNow := make(chan struct{})
	emitted := make(chan Event, 1)
	finish := make(chan struct{})
	err := manager.Register(TypeRebuildSearchIndex, func(ctx context.Context, execution *Execution, _ Job) error {
		close(started)
		select {
		case <-emitNow:
		case <-ctx.Done():
			return ctx.Err()
		}
		event, err := execution.Emit(ctx, "progress", map[string]int{"current": 1})
		if err != nil {
			return err
		}
		emitted <- event
		select {
		case <-finish:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	job, err := manager.Create(context.Background(), CreateRequest{Type: TypeRebuildSearchIndex})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	receiveSignal(t, started)

	listEntered := make(chan struct{})
	listRelease := make(chan struct{})
	store.blockNextListEvents(listEntered, listRelease)
	type subscriptionResult struct {
		events <-chan Event
		err    error
	}
	subscription := make(chan subscriptionResult, 1)
	streamContext, cancelStream := context.WithCancel(context.Background())
	defer cancelStream()
	go func() {
		events, err := manager.Events(streamContext, job.ID, 1)
		subscription <- subscriptionResult{events: events, err: err}
	}()
	receiveSignal(t, listEntered)

	close(emitNow)
	progress := receive(t, emitted)
	if progress.Sequence != 3 {
		t.Fatalf("progress sequence = %d, want 3", progress.Sequence)
	}
	close(listRelease)
	result := receive(t, subscription)
	if result.err != nil {
		t.Fatalf("Events() error = %v", result.err)
	}

	first := receive(t, result.events)
	second := receive(t, result.events)
	if first.Sequence != 2 || first.Type != "started" {
		t.Fatalf("first replayed event = (%d, %s), want (2, started)", first.Sequence, first.Type)
	}
	if second.Sequence != 3 || second.Type != "progress" {
		t.Fatalf("second replayed event = (%d, %s), want (3, progress)", second.Sequence, second.Type)
	}

	close(finish)
	third := receive(t, result.events)
	if third.Sequence != 4 || third.Type != "completed" {
		t.Fatalf("live event = (%d, %s), want (4, completed)", third.Sequence, third.Type)
	}
	seen := map[int64]bool{}
	for _, event := range []Event{first, second, third} {
		if seen[event.Sequence] {
			t.Fatalf("duplicate event sequence %d", event.Sequence)
		}
		seen[event.Sequence] = true
	}

	all, err := store.ListEvents(context.Background(), job.ID, 0, 0)
	if err != nil {
		t.Fatalf("ListEvents(all) error = %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("stored event count = %d, want 4", len(all))
	}
	for index, event := range all {
		want := int64(index + 1)
		if event.Sequence != want {
			t.Errorf("stored event[%d] sequence = %d, want %d", index, event.Sequence, want)
		}
	}
}

func TestManagerCloseIsIdempotentAndWaitsForWorker(t *testing.T) {
	store := newFakeStore()
	manager, err := NewManager(context.Background(), store)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	started := make(chan struct{})
	cancelled := make(chan struct{})
	allowReturn := make(chan struct{})
	err = manager.Register(TypeSyncLatest, func(ctx context.Context, _ *Execution, _ Job) error {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-allowReturn
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	job, err := manager.Create(context.Background(), CreateRequest{Type: TypeSyncLatest})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	receiveSignal(t, started)

	closed := make(chan error, 1)
	go func() { closed <- manager.Close() }()
	receiveSignal(t, cancelled)
	select {
	case err := <-closed:
		t.Fatalf("Close() returned before worker exited: %v", err)
	default:
	}
	close(allowReturn)
	if err := receive(t, closed); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}

	stored, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if stored.Status != StatusPaused {
		t.Errorf("status after Close() = %s, want %s", stored.Status, StatusPaused)
	}
}

func newTestManager(t *testing.T, store *fakeStore) *Manager {
	t.Helper()
	manager, err := NewManager(context.Background(), store)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return manager
}

func receive[T any](t *testing.T, channel <-chan T) T {
	t.Helper()
	select {
	case value, ok := <-channel:
		if !ok {
			t.Fatal("channel closed before a value was received")
		}
		return value
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for channel value")
		var zero T
		return zero
	}
}

func receiveSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
		return
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for signal")
	}
}

func waitForEvent(t *testing.T, events <-chan Event, eventType string) Event {
	t.Helper()
	timer := time.NewTimer(testTimeout)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("event stream closed before %q", eventType)
			}
			if event.Type == eventType {
				return event
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for event %q", eventType)
		}
	}
}

func assertItemStatus(t *testing.T, items []Item, key string, status ItemStatus) {
	t.Helper()
	item := findItem(t, items, key)
	if item.Status != status {
		t.Fatalf("item %q status = %s, want %s", key, item.Status, status)
	}
}

func findItem(t *testing.T, items []Item, key string) Item {
	t.Helper()
	for _, item := range items {
		if item.Key == key {
			return item
		}
	}
	t.Fatalf("item %q not found in %+v", key, items)
	return Item{}
}

type fakeStore struct {
	mu sync.Mutex

	jobs   map[string]Job
	items  map[string]map[string]Item
	events map[string][]Event

	recoverCalls int
	listEntered  chan struct{}
	listRelease  chan struct{}
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		jobs:   make(map[string]Job),
		items:  make(map[string]map[string]Item),
		events: make(map[string][]Event),
	}
}

func (store *fakeStore) CreateJob(ctx context.Context, job Job) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.jobs[job.ID]; exists {
		return fmt.Errorf("job %s already exists", job.ID)
	}
	store.jobs[job.ID] = copyJob(job)
	return nil
}

func (store *fakeStore) GetJob(ctx context.Context, id string) (Job, error) {
	if err := contextError(ctx); err != nil {
		return Job{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	job, exists := store.jobs[id]
	if !exists {
		return Job{}, ErrNotFound
	}
	return copyJob(job), nil
}

func (store *fakeStore) ListJobs(ctx context.Context, limit int) ([]Job, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]Job, 0, len(store.jobs))
	for _, job := range store.jobs {
		result = append(result, copyJob(job))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (store *fakeStore) MutateJob(ctx context.Context, id string, mutate func(*Job) error) (Job, error) {
	if err := contextError(ctx); err != nil {
		return Job{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	job, exists := store.jobs[id]
	if !exists {
		return Job{}, ErrNotFound
	}
	job = copyJob(job)
	if err := mutate(&job); err != nil {
		return Job{}, err
	}
	store.jobs[id] = copyJob(job)
	return copyJob(job), nil
}

func (store *fakeStore) UpsertItem(ctx context.Context, item Item) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.jobs[item.JobID]; !exists {
		return ErrNotFound
	}
	if store.items[item.JobID] == nil {
		store.items[item.JobID] = make(map[string]Item)
	}
	store.items[item.JobID][item.Key] = copyItem(item)
	return nil
}

func (store *fakeStore) ListItems(ctx context.Context, jobID string) ([]Item, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.jobs[jobID]; !exists {
		return nil, ErrNotFound
	}
	result := make([]Item, 0, len(store.items[jobID]))
	for _, item := range store.items[jobID] {
		result = append(result, copyItem(item))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}

func (store *fakeStore) ResetFailedItems(ctx context.Context, jobID string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.jobs[jobID]; !exists {
		return ErrNotFound
	}
	for key, item := range store.items[jobID] {
		if item.Status == ItemCompleted {
			continue
		}
		item.Status = ItemQueued
		item.Error = ""
		item.UpdatedAt = time.Now().UTC()
		store.items[jobID][key] = item
	}
	return nil
}

func (store *fakeStore) AppendEvent(ctx context.Context, event Event) (Event, error) {
	if err := contextError(ctx); err != nil {
		return Event{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.jobs[event.JobID]; !exists {
		return Event{}, ErrNotFound
	}
	sequence := int64(1)
	stored := store.events[event.JobID]
	if len(stored) > 0 {
		sequence = stored[len(stored)-1].Sequence + 1
	}
	event.Sequence = sequence
	event = copyEvent(event)
	store.events[event.JobID] = append(stored, event)
	return copyEvent(event), nil
}

func (store *fakeStore) ListEvents(ctx context.Context, jobID string, afterSequence int64, limit int) ([]Event, error) {
	store.mu.Lock()
	entered, release := store.listEntered, store.listRelease
	store.listEntered, store.listRelease = nil, nil
	store.mu.Unlock()
	if entered != nil {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.jobs[jobID]; !exists {
		return nil, ErrNotFound
	}
	result := make([]Event, 0)
	for _, event := range store.events[jobID] {
		if event.Sequence <= afterSequence {
			continue
		}
		result = append(result, copyEvent(event))
		if limit > 0 && len(result) == limit {
			break
		}
	}
	return result, nil
}

func (store *fakeStore) RecoverRunningToPaused(ctx context.Context) ([]Job, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.recoverCalls++
	result := make([]Job, 0)
	for id, job := range store.jobs {
		if job.Status != StatusRunning {
			continue
		}
		job.Status = StatusPaused
		job.UpdatedAt = time.Now().UTC()
		store.jobs[id] = copyJob(job)
		result = append(result, copyJob(job))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (store *fakeStore) seedJob(job Job) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.jobs[job.ID] = copyJob(job)
}

func (store *fakeStore) seedItem(item Item) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.items[item.JobID] == nil {
		store.items[item.JobID] = make(map[string]Item)
	}
	store.items[item.JobID][item.Key] = copyItem(item)
}

func (store *fakeStore) recoverCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.recoverCalls
}

func (store *fakeStore) blockNextListEvents(entered, release chan struct{}) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.listEntered = entered
	store.listRelease = release
}

func copyJob(job Job) Job {
	job.Payload = cloneRaw(job.Payload)
	job.Checkpoint = cloneRaw(job.Checkpoint)
	if job.StartedAt != nil {
		started := *job.StartedAt
		job.StartedAt = &started
	}
	if job.FinishedAt != nil {
		finished := *job.FinishedAt
		job.FinishedAt = &finished
	}
	return job
}

func copyItem(item Item) Item {
	item.Checkpoint = cloneRaw(item.Checkpoint)
	return item
}

func copyEvent(event Event) Event {
	event.Data = cloneRaw(event.Data)
	return event
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
