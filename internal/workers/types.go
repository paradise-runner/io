package workers

import "time"

// State is a worker lifecycle state.
type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateTimedOut  State = "timed_out"
	StateCanceled  State = "canceled"
)

// Terminal reports whether s is a completed state.
func (s State) Terminal() bool {
	switch s {
	case StateSucceeded, StateFailed, StateTimedOut, StateCanceled:
		return true
	default:
		return false
	}
}

// Request describes work delegated to a background worker.
type Request struct {
	CWD             string
	Task            string
	Label           string
	Harness         string
	Model           string
	ReasoningEffort string
	Timeout         time.Duration
}

// Status is the UI-safe lifecycle snapshot of a worker.
type Status struct {
	ID         string
	CWD        string
	Label      string
	State      State
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string
}

// Result is the concise outcome returned by a worker runner.
type Result struct {
	ID      string
	Summary string
	Error   string
}

// Event is emitted whenever a worker changes lifecycle state.
type Event struct {
	Status  Status
	Request Request
	Result  *Result
}
