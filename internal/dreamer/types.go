// Package dreamer consolidates displayed io chats into durable memory notes.
package dreamer

import "github.com/edward-champion/io/internal/chatlog"

// Candidate is one possible memory discovered by the dreamer model.
type Candidate struct {
	Insight    string   `json:"insight"`
	Evidence   []string `json:"evidence"`
	Confidence float64  `json:"confidence"`
	Tags       []string `json:"tags"`
}

// Request is the model-facing input for one dream pass.
type Request struct {
	History         []chatlog.Entry
	ExistingMemory  string
	Harness         string
	Model           string
	ReasoningEffort string
}
