package personastate

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := &State{
		PersonaSessionID:    "sess-1",
		CompactionThreshold: 0.75,
		DreamChatThreshold:  7,
		LastDreamAt:         "2026-06-02T10:00:00Z",
		ChatsSinceDream:     3,
		DreamerModel:        "opus",
	}
	if err := Save(dir, s); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.PersonaSessionID != "sess-1" {
		t.Fatalf("PersonaSessionID = %q, want sess-1", got.PersonaSessionID)
	}
	if got.ClaudeSessionID != "sess-1" {
		t.Fatalf("ClaudeSessionID = %q, want sess-1", got.ClaudeSessionID)
	}
	if got.Harness != DefaultHarness {
		t.Fatalf("Harness = %q, want %q", got.Harness, DefaultHarness)
	}
	if got.ReasoningEffort != DefaultReasoningEffort {
		t.Fatalf("ReasoningEffort = %q, want %q", got.ReasoningEffort, DefaultReasoningEffort)
	}
	if got.CompactionThreshold != 0.75 {
		t.Fatalf("CompactionThreshold = %v, want 0.75", got.CompactionThreshold)
	}
	if got.DreamChatThreshold != 7 {
		t.Fatalf("DreamChatThreshold = %v, want 7", got.DreamChatThreshold)
	}
	if got.LastDreamAt != "2026-06-02T10:00:00Z" {
		t.Fatalf("LastDreamAt = %q, want saved timestamp", got.LastDreamAt)
	}
	if got.ChatsSinceDream != 3 {
		t.Fatalf("ChatsSinceDream = %d, want 3", got.ChatsSinceDream)
	}
	if got.DreamerModel != "opus" {
		t.Fatalf("DreamerModel = %q, want opus", got.DreamerModel)
	}
}

func TestLoad_MissingReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.PersonaSessionID != "" {
		t.Fatalf("PersonaSessionID = %q, want empty for fresh state", got.PersonaSessionID)
	}
	if got.Model != DefaultModel {
		t.Fatalf("Model = %q, want default %q", got.Model, DefaultModel)
	}
	if got.CodexModel != "gpt-5.4" {
		t.Fatalf("CodexModel = %q, want gpt-5.4", got.CodexModel)
	}
	if got.ReasoningEffort != DefaultReasoningEffort {
		t.Fatalf("ReasoningEffort = %q, want default %q", got.ReasoningEffort, DefaultReasoningEffort)
	}
	if got.CompactionThreshold != DefaultCompactionThreshold {
		t.Fatalf("CompactionThreshold = %v, want default %v", got.CompactionThreshold, DefaultCompactionThreshold)
	}
	if got.DreamChatThreshold != DefaultDreamChatThreshold {
		t.Fatalf("DreamChatThreshold = %v, want default %v", got.DreamChatThreshold, DefaultDreamChatThreshold)
	}
	if got.DreamerModel != DefaultModel {
		t.Fatalf("DreamerModel = %q, want default %q", got.DreamerModel, DefaultModel)
	}
}

func TestPathsUnderRoot(t *testing.T) {
	dir := "/tmp/io-root"
	if SoulPath(dir) != filepath.Join(dir, "SOUL.md") {
		t.Fatalf("SoulPath = %q", SoulPath(dir))
	}
	if MemoryDir(dir) != filepath.Join(dir, "memory") {
		t.Fatalf("MemoryDir = %q", MemoryDir(dir))
	}
	if statePath(dir) != filepath.Join(dir, "state.json") {
		t.Fatalf("statePath = %q", statePath(dir))
	}
}

func TestStateTracksHarnessSpecificSessionsAndModels(t *testing.T) {
	var st State
	st.Normalize()
	st.SetActiveSessionID("claude-session")
	st.SetActiveModel("opus")
	st.SetHarness("codex")
	if st.ActiveSessionID() != "" {
		t.Fatalf("codex ActiveSessionID = %q, want empty", st.ActiveSessionID())
	}
	if st.ActiveModel() != "gpt-5.4" {
		t.Fatalf("codex ActiveModel = %q, want gpt-5.4", st.ActiveModel())
	}
	st.SetActiveSessionID("codex-session")
	st.SetActiveModel("gpt-5.5")
	st.SetHarness("claude")
	if st.ActiveSessionID() != "claude-session" {
		t.Fatalf("claude ActiveSessionID = %q, want claude-session", st.ActiveSessionID())
	}
	if st.ActiveModel() != "opus" {
		t.Fatalf("claude ActiveModel = %q, want opus", st.ActiveModel())
	}
}
