package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Manager struct {
	store Store

	ctx    context.Context
	cancel context.CancelFunc
	queue  chan string
	wg     sync.WaitGroup

	mu          sync.Mutex
	controlMu   sync.Mutex
	handlers    map[Type]Handler
	active      map[string]*activeRun
	pending     map[string]struct{}
	subscribers map[string]map[uint64]chan Event
	nextSubID   uint64
	closed      bool
}

type activeRun struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func NewManager(ctx context.Context, store Store) (*Manager, error) {
	if store == nil {
		return nil, errors.New("job store is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	managerCtx, cancel := context.WithCancel(ctx)
	m := &Manager{
		store:       store,
		ctx:         managerCtx,
		cancel:      cancel,
		queue:       make(chan string, 256),
		handlers:    make(map[Type]Handler),
		active:      make(map[string]*activeRun),
		pending:     make(map[string]struct{}),
		subscribers: make(map[string]map[uint64]chan Event),
	}

	recovered, err := store.RecoverRunningToPaused(managerCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("recover running jobs: %w", err)
	}
	for _, job := range recovered {
		if _, err := m.emit(managerCtx, job.ID, "paused", map[string]any{"reason": "process_restarted"}); err != nil {
			cancel()
			return nil, fmt.Errorf("record recovered job event: %w", err)
		}
	}

	queued, err := store.ListJobs(managerCtx, 0)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("list queued jobs: %w", err)
	}
	m.wg.Add(2)
	go m.worker()
	go m.scheduler()
	for _, job := range queued {
		if job.Status == StatusQueued {
			m.enqueue(job.ID)
		}
	}
	return m, nil
}

func (m *Manager) Register(jobType Type, handler Handler) error {
	if !jobType.Valid() {
		return fmt.Errorf("invalid job type %q", jobType)
	}
	if handler == nil {
		return errors.New("job handler is required")
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	m.handlers[jobType] = handler
	m.mu.Unlock()

	queued, err := m.store.ListJobs(m.ctx, 0)
	if err != nil {
		m.mu.Lock()
		delete(m.handlers, jobType)
		m.mu.Unlock()
		return err
	}
	for _, job := range queued {
		if job.Type == jobType && job.Status == StatusQueued {
			m.enqueue(job.ID)
		}
	}
	return nil
}

func (m *Manager) Create(ctx context.Context, request CreateRequest) (Job, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !request.Type.Valid() {
		return Job{}, fmt.Errorf("invalid job type %q", request.Type)
	}
	if err := m.ensureOpen(); err != nil {
		return Job{}, err
	}
	payload, err := marshalOptional(request.Payload)
	if err != nil {
		return Job{}, fmt.Errorf("encode job payload: %w", err)
	}
	now := time.Now().UTC()
	job := Job{
		ID:         newID(),
		Type:       request.Type,
		Status:     StatusQueued,
		Payload:    payload,
		TotalItems: request.TotalItems,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := m.store.CreateJob(ctx, job); err != nil {
		return Job{}, err
	}
	if _, err := m.emit(ctx, job.ID, "queued", map[string]any{"type": job.Type}); err != nil {
		return Job{}, err
	}
	m.enqueue(job.ID)
	return job, nil
}

func (m *Manager) Get(ctx context.Context, id string) (Job, error) {
	return m.store.GetJob(ctx, id)
}

func (m *Manager) List(ctx context.Context, limit int) ([]Job, error) {
	return m.store.ListJobs(ctx, limit)
}

func (m *Manager) Pause(ctx context.Context, id string) (Job, error) {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	job, err := m.transition(ctx, id, StatusPaused, "", nil)
	if err != nil {
		return Job{}, err
	}
	if err := m.cancelActiveAndWait(ctx, id); err != nil {
		return Job{}, err
	}
	_, emitErr := m.emit(ctx, id, "paused", nil)
	return job, emitErr
}

func (m *Manager) Resume(ctx context.Context, id string) (Job, error) {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	job, err := m.transition(ctx, id, StatusQueued, "", func(job *Job) {
		job.FinishedAt = nil
	})
	if err != nil {
		return Job{}, err
	}
	if _, err := m.emit(ctx, id, "queued", map[string]any{"reason": "resumed"}); err != nil {
		return Job{}, err
	}
	m.enqueue(id)
	return job, nil
}

func (m *Manager) Cancel(ctx context.Context, id string) (Job, error) {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	now := time.Now().UTC()
	job, err := m.transition(ctx, id, StatusCancelled, "", func(job *Job) {
		job.FinishedAt = &now
	})
	if err != nil {
		return Job{}, err
	}
	if err := m.cancelActiveAndWait(ctx, id); err != nil {
		return Job{}, err
	}
	_, emitErr := m.emit(ctx, id, "cancelled", nil)
	return job, emitErr
}

func (m *Manager) Retry(ctx context.Context, id string) (Job, error) {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	current, err := m.store.GetJob(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if current.Status != StatusFailed && current.Status != StatusPartial && current.Status != StatusCancelled {
		return Job{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, StatusQueued)
	}
	if err := m.store.ResetFailedItems(ctx, id); err != nil {
		return Job{}, err
	}
	job, err := m.transition(ctx, id, StatusQueued, "", func(job *Job) {
		job.Error = ""
		job.FailedItems = 0
		job.FinishedAt = nil
	})
	if err != nil {
		return Job{}, err
	}
	if _, err := m.emit(ctx, id, "queued", map[string]any{"reason": "retry"}); err != nil {
		return Job{}, err
	}
	m.enqueue(id)
	return job, nil
}

func (m *Manager) Events(ctx context.Context, jobID string, afterSequence int64) (<-chan Event, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := m.store.GetJob(ctx, jobID); err != nil {
		return nil, err
	}
	raw := make(chan Event, 256)
	out := make(chan Event, 64)

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrClosed
	}
	m.nextSubID++
	subID := m.nextSubID
	if m.subscribers[jobID] == nil {
		m.subscribers[jobID] = make(map[uint64]chan Event)
	}
	m.subscribers[jobID][subID] = raw
	m.mu.Unlock()

	history, err := m.store.ListEvents(ctx, jobID, afterSequence, 0)
	if err != nil {
		m.unsubscribe(jobID, subID)
		return nil, err
	}
	go func() {
		defer close(out)
		defer m.unsubscribe(jobID, subID)
		last := afterSequence
		for _, event := range history {
			if event.Sequence <= last {
				continue
			}
			select {
			case out <- event:
				last = event.Sequence
			case <-ctx.Done():
				return
			case <-m.ctx.Done():
				return
			}
		}
		for {
			select {
			case event, ok := <-raw:
				if !ok {
					return
				}
				if event.Sequence <= last {
					continue
				}
				select {
				case out <- event:
					last = event.Sequence
				case <-ctx.Done():
					return
				case <-m.ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			case <-m.ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	active := make(map[string]*activeRun, len(m.active))
	for id, run := range m.active {
		active[id] = run
	}
	m.mu.Unlock()

	for id, run := range active {
		_, _ = m.store.MutateJob(context.Background(), id, func(job *Job) error {
			if job.Status == StatusRunning {
				job.Status = StatusPaused
				job.Error = "process stopped"
			}
			return nil
		})
		run.cancel()
	}
	m.cancel()
	m.wg.Wait()

	m.mu.Lock()
	for jobID, subscribers := range m.subscribers {
		for id, channel := range subscribers {
			close(channel)
			delete(subscribers, id)
		}
		delete(m.subscribers, jobID)
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case id := <-m.queue:
			m.mu.Lock()
			delete(m.pending, id)
			m.mu.Unlock()
			m.execute(id)
		}
	}
}

// scheduler periodically reconciles the in-memory wake-up queue with the
// durable job store. The queue is only a notification mechanism: queued jobs
// must remain runnable even if a process restart, a full channel, or a failed
// event append causes an individual notification to be missed.
func (m *Manager) scheduler() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			_ = m.reconcileQueued()
		}
	}
}

func (m *Manager) reconcileQueued() error {
	queued, err := m.store.ListJobs(m.ctx, 0)
	if err != nil {
		return err
	}
	for _, job := range queued {
		if job.Status == StatusQueued {
			m.enqueue(job.ID)
		}
	}
	return nil
}

func (m *Manager) execute(id string) {
	job, err := m.store.GetJob(m.ctx, id)
	if err != nil || job.Status != StatusQueued {
		return
	}
	m.mu.Lock()
	handler := m.handlers[job.Type]
	m.mu.Unlock()
	if handler == nil {
		// Persisted queued jobs wait until their handler is registered. Register
		// scans and re-enqueues matching work, avoiding a startup race.
		return
	}

	jobCtx, cancel := context.WithCancel(m.ctx)
	run := &activeRun{cancel: cancel, done: make(chan struct{})}
	m.mu.Lock()
	m.active[id] = run
	m.mu.Unlock()
	defer m.finishRun(id, run)

	now := time.Now().UTC()
	job, err = m.transition(m.ctx, id, StatusRunning, "", func(job *Job) {
		job.StartedAt = &now
		job.FinishedAt = nil
		job.Attempts++
		job.Error = ""
	})
	if err != nil {
		return
	}
	_, _ = m.emit(m.ctx, id, "started", map[string]any{"attempt": job.Attempts})

	execution := &Execution{manager: m, jobID: id}
	handlerErr := handler(jobCtx, execution, job)
	cancel()

	latest, getErr := m.store.GetJob(context.Background(), id)
	if getErr != nil || latest.Status != StatusRunning {
		return
	}
	finished := time.Now().UTC()
	finalStatus := StatusCompleted
	errorMessage := ""
	if handlerErr != nil {
		finalStatus = StatusFailed
		errorMessage = handlerErr.Error()
		if latest.CompletedItems > 0 {
			finalStatus = StatusPartial
		}
	} else if latest.FailedItems > 0 {
		finalStatus = StatusPartial
	}
	_, err = m.transition(context.Background(), id, finalStatus, errorMessage, func(job *Job) {
		job.FinishedAt = &finished
	})
	if err != nil {
		return
	}
	eventType := string(finalStatus)
	_, _ = m.emit(context.Background(), id, eventType, map[string]any{"error": errorMessage})
}

func (m *Manager) fail(id string, failure error) {
	finished := time.Now().UTC()
	_, _ = m.transition(context.Background(), id, StatusFailed, failure.Error(), func(job *Job) {
		job.FinishedAt = &finished
	})
	_, _ = m.emit(context.Background(), id, "failed", map[string]any{"error": failure.Error()})
}

func (m *Manager) transition(ctx context.Context, id string, to Status, message string, extra func(*Job)) (Job, error) {
	return m.store.MutateJob(ctx, id, func(job *Job) error {
		if !CanTransition(job.Status, to) {
			return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, job.Status, to)
		}
		job.Status = to
		job.Error = message
		job.UpdatedAt = time.Now().UTC()
		if extra != nil {
			extra(job)
		}
		return nil
	})
}

func (m *Manager) emit(ctx context.Context, jobID, eventType string, data any) (Event, error) {
	payload, err := marshalOptional(data)
	if err != nil {
		return Event{}, err
	}
	event, err := m.store.AppendEvent(ctx, Event{JobID: jobID, Type: eventType, Data: payload, CreatedAt: time.Now().UTC()})
	if err != nil {
		return Event{}, err
	}
	m.mu.Lock()
	for _, subscriber := range m.subscribers[jobID] {
		select {
		case subscriber <- event:
		default:
			// Durable replay remains available when a slow live subscriber falls behind.
		}
	}
	m.mu.Unlock()
	return event, nil
}

func (m *Manager) enqueue(id string) {
	if id == "" {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	if _, exists := m.pending[id]; exists {
		m.mu.Unlock()
		return
	}
	if _, active := m.active[id]; active {
		m.mu.Unlock()
		return
	}
	m.pending[id] = struct{}{}
	m.mu.Unlock()

	select {
	case m.queue <- id:
	case <-m.ctx.Done():
		m.mu.Lock()
		delete(m.pending, id)
		m.mu.Unlock()
	default:
		// A periodic durable-store reconciliation retries this wake-up. Do not
		// block an HTTP request merely because the in-memory queue is full.
		m.mu.Lock()
		delete(m.pending, id)
		m.mu.Unlock()
	}
}

func (m *Manager) cancelActiveAndWait(ctx context.Context, id string) error {
	m.mu.Lock()
	run := m.active[id]
	m.mu.Unlock()
	if run == nil {
		return nil
	}
	run.cancel()
	select {
	case <-run.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) finishRun(id string, run *activeRun) {
	m.mu.Lock()
	if m.active[id] == run {
		delete(m.active, id)
	}
	close(run.done)
	m.mu.Unlock()
}

func (m *Manager) unsubscribe(jobID string, subID uint64) {
	m.mu.Lock()
	if subscribers := m.subscribers[jobID]; subscribers != nil {
		delete(subscribers, subID)
		if len(subscribers) == 0 {
			delete(m.subscribers, jobID)
		}
	}
	m.mu.Unlock()
}

func (m *Manager) ensureOpen() error {
	if m == nil {
		return ErrClosed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	return nil
}

func marshalOptional(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	if raw, ok := value.(json.RawMessage); ok {
		return append(json.RawMessage(nil), raw...), nil
	}
	return json.Marshal(value)
}

func newID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
