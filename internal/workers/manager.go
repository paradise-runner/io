package workers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultMaxCompleted = 5

// Manager owns in-memory worker status and lifecycle events.
type Manager struct {
	runner       Runner
	events       chan Event
	maxCompleted int

	mu        sync.Mutex
	nextID    int
	order     []string
	completed []string
	records   map[string]*record
}

type record struct {
	req    Request
	status Status
	cancel context.CancelFunc
}

// Option configures a Manager.
type Option func(*Manager)

// WithMaxCompleted caps completed statuses retained in Snapshot.
func WithMaxCompleted(n int) Option {
	return func(m *Manager) {
		if n >= 0 {
			m.maxCompleted = n
		}
	}
}

// NewManager constructs a worker manager.
func NewManager(runner Runner, opts ...Option) *Manager {
	m := &Manager{
		runner:       runner,
		events:       make(chan Event, 256),
		maxCompleted: defaultMaxCompleted,
		records:      make(map[string]*record),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Events returns lifecycle events for UI and app orchestration.
func (m *Manager) Events() <-chan Event { return m.events }

// Spawn starts a worker asynchronously and returns its id immediately.
func (m *Manager) Spawn(ctx context.Context, req Request) (string, error) {
	if m.runner == nil {
		return "", errors.New("worker runner is required")
	}
	if strings.TrimSpace(req.Task) == "" {
		return "", errors.New("worker task is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	id := m.nextWorkerID()
	if strings.TrimSpace(req.Label) == "" {
		req.Label = defaultLabel(req)
	}
	status := Status{
		ID:    id,
		CWD:   req.CWD,
		Label: req.Label,
		State: StateQueued,
	}

	baseCtx, baseCancel := context.WithCancel(ctx)
	runCtx := baseCtx
	cancel := baseCancel
	if req.Timeout > 0 {
		timeoutCtx, timeoutCancel := context.WithTimeout(baseCtx, req.Timeout)
		runCtx = timeoutCtx
		cancel = func() {
			timeoutCancel()
			baseCancel()
		}
	}

	m.mu.Lock()
	m.order = append(m.order, id)
	m.records[id] = &record{req: req, status: status, cancel: cancel}
	m.mu.Unlock()
	m.emit(Event{Status: status, Request: req})

	go m.run(runCtx, id)
	return id, nil
}

func (m *Manager) nextWorkerID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	return fmt.Sprintf("w%d", m.nextID)
}

func defaultLabel(req Request) string {
	base := strings.TrimSpace(req.Label)
	if base == "" {
		base = strings.TrimSpace(req.CWD)
	}
	if base == "" {
		base = "worker"
	}
	return base
}

func (m *Manager) run(ctx context.Context, id string) {
	req, ok := m.transition(id, StateRunning, "", nil, true)
	if !ok {
		return
	}
	result, err := m.runner.Run(ctx, req)
	if result.ID == "" {
		result.ID = id
	}

	state, errText := finalState(ctx, result, err)
	_, _ = m.transition(id, state, errText, &result, false)
}

func finalState(ctx context.Context, result Result, err error) (State, string) {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return StateTimedOut, "worker timed out"
	case errors.Is(ctx.Err(), context.Canceled):
		return StateCanceled, "worker canceled"
	case err != nil:
		return StateFailed, err.Error()
	case result.Error != "":
		return StateFailed, result.Error
	default:
		return StateSucceeded, ""
	}
}

func (m *Manager) transition(id string, state State, errText string, result *Result, markStarted bool) (Request, bool) {
	m.mu.Lock()
	rec, ok := m.records[id]
	if !ok {
		m.mu.Unlock()
		return Request{}, false
	}
	rec.status.State = state
	if markStarted {
		rec.status.StartedAt = time.Now()
	}
	if state.Terminal() {
		rec.status.FinishedAt = time.Now()
		rec.status.Error = errText
		rec.cancel()
		m.rememberCompleted(id)
	}
	status := rec.status
	req := rec.req
	m.mu.Unlock()

	m.emit(Event{Status: status, Request: req, Result: result})
	return req, true
}

func (m *Manager) rememberCompleted(id string) {
	m.completed = append(m.completed, id)
	if m.maxCompleted < 1 {
		delete(m.records, id)
		return
	}
	for len(m.completed) > m.maxCompleted {
		drop := m.completed[0]
		m.completed = m.completed[1:]
		delete(m.records, drop)
		m.removeOrder(drop)
	}
}

func (m *Manager) removeOrder(id string) {
	for i, existing := range m.order {
		if existing == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			return
		}
	}
}

// Cancel requests cancellation for a worker. It returns false when id is
// unknown or already terminal.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	rec, ok := m.records[id]
	if !ok || rec.status.State.Terminal() {
		m.mu.Unlock()
		return false
	}
	cancel := rec.cancel
	m.mu.Unlock()
	cancel()
	return true
}

// Snapshot returns active workers in spawn order followed by recently completed
// workers, newest completed first.
func (m *Manager) Snapshot() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []Status
	completedSet := make(map[string]bool, len(m.completed))
	for _, id := range m.completed {
		completedSet[id] = true
	}
	for _, id := range m.order {
		if completedSet[id] {
			continue
		}
		if rec, ok := m.records[id]; ok {
			out = append(out, rec.status)
		}
	}
	for i := len(m.completed) - 1; i >= 0; i-- {
		if rec, ok := m.records[m.completed[i]]; ok {
			out = append(out, rec.status)
		}
	}
	return out
}

func (m *Manager) emit(ev Event) {
	m.events <- ev
}
