package tui

import (
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/soul"
)

// stubApp is a test double for AppController.
type stubApp struct {
	sent        []string
	newSessions int
	setModel    string
	saved       *personastate.State
	settings    personastate.State
	ctx         ContextInfo
	compactErr  error
	compactions int
	memory      string
	history     []HistoryEntry
	needsSetup  bool

	completedSetup bool
	setupHarness   string
	setupChoice    soul.Choice
}

func (s *stubApp) NeedsSetup() bool { return s.needsSetup }
func (s *stubApp) CompleteSetup(harness string, choice soul.Choice) error {
	s.completedSetup = true
	s.needsSetup = false
	s.setupHarness = harness
	s.setupChoice = choice
	return nil
}
func (s *stubApp) Send(text string) error                   { s.sent = append(s.sent, text); return nil }
func (s *stubApp) NewSession() error                        { s.newSessions++; return nil }
func (s *stubApp) SetModel(m string) error                  { s.setModel = m; return nil }
func (s *stubApp) Settings() personastate.State             { return s.settings }
func (s *stubApp) SaveSettings(st personastate.State) error { s.saved = &st; return nil }
func (s *stubApp) ContextInfo() ContextInfo                 { return s.ctx }
func (s *stubApp) CompactNow() error                        { s.compactions++; return s.compactErr }
func (s *stubApp) MemorySummary() (string, error)           { return s.memory, nil }
func (s *stubApp) History() []HistoryEntry                  { return s.history }
