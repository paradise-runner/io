package ioipc

import "time"

const (
	ProtocolVersion = 1

	OpSpawnWorker  = "spawn_worker"
	OpWorkerStatus = "worker_status"
	OpListProjects = "list_projects"
)

// Request is a versioned IPC operation request.
type Request struct {
	Version     int                 `json:"version"`
	Operation   string              `json:"operation"`
	SpawnWorker *SpawnWorkerRequest `json:"spawn_worker,omitempty"`
}

// Response is a versioned IPC operation response.
type Response struct {
	Version int    `json:"version"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`

	WorkerID string         `json:"worker_id,omitempty"`
	Statuses []WorkerStatus `json:"statuses,omitempty"`
	Projects []string       `json:"projects,omitempty"`
}

// SpawnWorkerRequest asks the parent process to start a worker.
type SpawnWorkerRequest struct {
	CWD             string        `json:"cwd,omitempty"`
	Task            string        `json:"task"`
	Label           string        `json:"label,omitempty"`
	Harness         string        `json:"harness,omitempty"`
	Model           string        `json:"model,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	Timeout         time.Duration `json:"timeout,omitempty"`
}

// WorkerStatus is the IPC-safe shape of a worker lifecycle snapshot.
type WorkerStatus struct {
	ID         string    `json:"id"`
	CWD        string    `json:"cwd"`
	Label      string    `json:"label"`
	State      string    `json:"state"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Error      string    `json:"error,omitempty"`
}
