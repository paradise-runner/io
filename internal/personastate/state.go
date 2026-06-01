// Package personastate persists io's cross-restart state under the io root
// directory (default ~/.io): the captured persona session id and user settings.
package personastate

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/edward-champion/io/internal/agentharness"
)

const (
	DefaultCompactionThreshold = 0.75
	DefaultDreamChatThreshold  = 7
	DefaultHarness             = string(agentharness.DefaultKind)
	DefaultModel               = agentharness.DefaultClaudeModel
	DefaultReasoningEffort     = agentharness.DefaultReasoningEffort
)

// State is the persisted state of an io installation.
type State struct {
	PersonaSessionID    string  `json:"persona_session_id"`
	ClaudeSessionID     string  `json:"claude_session_id,omitempty"`
	CodexSessionID      string  `json:"codex_session_id,omitempty"`
	Harness             string  `json:"harness"`
	Model               string  `json:"model"`
	ClaudeModel         string  `json:"claude_model,omitempty"`
	CodexModel          string  `json:"codex_model,omitempty"`
	ReasoningEffort     string  `json:"reasoning_effort"`
	CompactionThreshold float64 `json:"compaction_threshold"`
	DreamChatThreshold  int     `json:"dream_chat_threshold"`
}

// DefaultRoot returns the default io root directory (~/.io).
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".io"), nil
}

func statePath(root string) string { return filepath.Join(root, "state.json") }

// SoulPath returns the path to the personality file under root.
func SoulPath(root string) string { return filepath.Join(root, "SOUL.md") }

// MemoryDir returns the path io uses for harness-backed assistant memory.
func MemoryDir(root string) string { return filepath.Join(root, "memory") }

// HistoryPath returns the path to io's display-transcript log.
func HistoryPath(root string) string { return filepath.Join(root, "history.jsonl") }

// Load reads state.json from root. If the file does not exist, it returns a
// State populated with defaults (and a nil error).
func Load(root string) (*State, error) {
	b, err := os.ReadFile(statePath(root))
	if errors.Is(err, fs.ErrNotExist) {
		s := defaultState()
		return &s, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	s.Normalize()
	return &s, nil
}

func defaultState() State {
	s := State{
		Harness:             DefaultHarness,
		Model:               DefaultModel,
		ClaudeModel:         agentharness.DefaultClaudeModel,
		CodexModel:          agentharness.DefaultCodexModel,
		ReasoningEffort:     DefaultReasoningEffort,
		CompactionThreshold: DefaultCompactionThreshold,
		DreamChatThreshold:  DefaultDreamChatThreshold,
	}
	s.Normalize()
	return s
}

// Normalize fills defaults and keeps legacy fields in sync with the selected
// harness-specific settings.
func (s *State) Normalize() {
	h, ok := agentharness.NormalizeKind(s.Harness)
	if !ok {
		h = agentharness.DefaultKind
	}
	s.Harness = string(h)
	if s.ClaudeModel == "" {
		s.ClaudeModel = agentharness.NormalizeModel(string(agentharness.Claude), s.Model)
	}
	if s.CodexModel == "" {
		s.CodexModel = agentharness.NormalizeModel(string(agentharness.Codex), s.Model)
	}
	s.ClaudeModel = agentharness.NormalizeModel(string(agentharness.Claude), s.ClaudeModel)
	s.CodexModel = agentharness.NormalizeModel(string(agentharness.Codex), s.CodexModel)
	s.ReasoningEffort = agentharness.NormalizeReasoningEffort(s.ReasoningEffort)
	if s.PersonaSessionID != "" {
		if h == agentharness.Codex {
			if s.CodexSessionID == "" {
				s.CodexSessionID = s.PersonaSessionID
			}
		} else if s.ClaudeSessionID == "" {
			s.ClaudeSessionID = s.PersonaSessionID
		}
	}
	s.Model = s.ActiveModel()
	s.PersonaSessionID = s.ActiveSessionID()
	if s.CompactionThreshold == 0 {
		s.CompactionThreshold = DefaultCompactionThreshold
	}
	if s.DreamChatThreshold == 0 {
		s.DreamChatThreshold = DefaultDreamChatThreshold
	}
}

// Save writes state.json to root, creating root (0700) if needed. It writes to
// a temp file and renames for atomicity.
func Save(root string, s *State) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	s.Normalize()
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := statePath(root) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, statePath(root))
}

func (s State) ActiveModel() string {
	if s.Harness == string(agentharness.Codex) {
		return agentharness.NormalizeModel(s.Harness, s.CodexModel)
	}
	return agentharness.NormalizeModel(s.Harness, s.ClaudeModel)
}

func (s *State) SetActiveModel(model string) {
	if s.Harness == string(agentharness.Codex) {
		s.CodexModel = model
	} else {
		s.ClaudeModel = model
	}
	s.Model = model
	s.Normalize()
}

func (s *State) SetHarness(harness string) {
	h, ok := agentharness.NormalizeKind(harness)
	if ok {
		if s.Harness != "" && s.Harness != string(h) {
			s.PersonaSessionID = ""
		}
		s.Harness = string(h)
	}
	s.Normalize()
}

func (s State) ActiveSessionID() string {
	if s.Harness == string(agentharness.Codex) {
		return s.CodexSessionID
	}
	return s.ClaudeSessionID
}

func (s *State) SetActiveSessionID(sessionID string) {
	s.SetSessionIDForHarness(s.Harness, sessionID)
}

func (s *State) SetSessionIDForHarness(harness, sessionID string) {
	h, ok := agentharness.NormalizeKind(harness)
	if !ok {
		h = agentharness.DefaultKind
	}
	if h == agentharness.Codex {
		s.CodexSessionID = sessionID
	} else {
		s.ClaudeSessionID = sessionID
	}
	s.Normalize()
}

func (s *State) ClearActiveSessionID() {
	s.SetActiveSessionID("")
}
