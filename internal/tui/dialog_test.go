package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edward-champion/io/internal/personastate"
)

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestConfirmScreen_YesEmitsOnYes(t *testing.T) {
	d := newConfirmScreen("Start fresh?", "io keeps memories.", newChatMsg{})
	next, cmd := d.Update(key("y"))
	if next != nil {
		t.Fatal("confirm yes should close (nil screen)")
	}
	if cmd == nil {
		t.Fatal("confirm yes should emit a command")
	}
	if _, ok := cmd().(newChatMsg); !ok {
		t.Fatalf("confirm yes should emit newChatMsg, got %T", cmd())
	}
}

func TestConfirmScreen_NoCloses(t *testing.T) {
	d := newConfirmScreen("t", "b", newChatMsg{})
	next, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if next != nil || cmd != nil {
		t.Fatalf("esc should close with no command, got %v %v", next, cmd)
	}
}

func TestSettingsScreen_TogglesModelAndSaves(t *testing.T) {
	app := &stubApp{settings: personastate.State{Model: "sonnet", CompactionThreshold: 0.75}}
	d := newSettingsScreen(app)

	// On the model field, right cycles sonnet -> opus.
	d.Update(tea.KeyMsg{Type: tea.KeyRight})
	if d.model != "opus" {
		t.Fatalf("model = %q, want opus after right", d.model)
	}

	// Enter saves.
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next != nil {
		t.Fatal("enter should close settings")
	}
	if app.setModel != "opus" {
		t.Fatalf("SetModel called with %q, want opus", app.setModel)
	}
	if app.saved == nil || app.saved.Model != "opus" {
		t.Fatalf("SaveSettings not called with model=opus: %+v", app.saved)
	}
}

func TestSettingsScreen_ThresholdClamps(t *testing.T) {
	app := &stubApp{settings: personastate.State{Model: "sonnet", CompactionThreshold: 0.92}}
	d := newSettingsScreen(app)
	// Move to threshold field.
	d.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Bump up past the 0.95 ceiling.
	d.Update(tea.KeyMsg{Type: tea.KeyRight})
	d.Update(tea.KeyMsg{Type: tea.KeyRight})
	if d.threshold > 0.95 {
		t.Fatalf("threshold = %v, want clamped at 0.95", d.threshold)
	}
}

func TestSettingsScreen_HarnessSetsHarnessDefaultModel(t *testing.T) {
	app := &stubApp{settings: personastate.State{Model: "sonnet", CompactionThreshold: 0.75}}
	d := newSettingsScreen(app)
	d.field = 2

	d.Update(tea.KeyMsg{Type: tea.KeyRight})
	if d.harness != "codex" {
		t.Fatalf("harness = %q, want codex", d.harness)
	}
	if d.model != "gpt-5.4" {
		t.Fatalf("model = %q, want gpt-5.4", d.model)
	}

	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next != nil {
		t.Fatal("enter should close settings")
	}
	if app.saved == nil || app.saved.Harness != "codex" || app.saved.Model != "gpt-5.4" {
		t.Fatalf("SaveSettings did not persist codex defaults: %+v", app.saved)
	}
}
