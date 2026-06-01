package tui

import (
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/soul"
)

// ContextInfo is the live context/usage snapshot shown in the Context screen.
type ContextInfo struct {
	InputTokens   int
	ContextWindow int
	CostUSD       float64
}

// HistoryEntry is one prior message loaded at startup.
type HistoryEntry struct {
	Role Role
	Text string
}

// AppController is the TUI's view of the application. It is implemented in
// cmd/io and stubbed in tests, keeping the tui package decoupled from the
// persona/state concretes.
type AppController interface {
	// NeedsSetup reports whether first-run setup must complete before chatting.
	NeedsSetup() bool
	// CompleteSetup persists the selected harness/personality and starts io.
	CompleteSetup(harness string, choice soul.Choice) error
	// Send delivers a user turn to the persona.
	Send(text string) error
	// NewSession starts a fresh persona session (memory is kept).
	NewSession() error
	// SetModel persists the model; it applies on the next session.
	SetModel(model string) error
	// Settings returns the current settings snapshot.
	Settings() personastate.State
	// SaveSettings persists settings.
	SaveSettings(personastate.State) error
	// ContextInfo returns the last-observed usage + cost.
	ContextInfo() ContextInfo
	// MemorySummary returns io's memory index contents (or a friendly empty state).
	MemorySummary() (string, error)
	// History returns the prior conversation to repopulate on startup.
	History() []HistoryEntry
}
