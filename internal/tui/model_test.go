package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edward-champion/io/internal/claudeproc"
	"github.com/edward-champion/io/internal/personastate"
)

func newTestModel(app AppController) Model {
	m := New(app)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return mm.(Model)
}

func TestUpdate_AppendsAssistantText(t *testing.T) {
	m := newTestModel(&stubApp{})
	updated := m.handleEvent(claudeproc.Event{Kind: claudeproc.KindAssistantText, Text: "hihi"})
	last := updated.messages.transcript[len(updated.messages.transcript)-1]
	if last.role != RoleIO || last.text != "hihi" {
		t.Fatalf("last item = %+v, want io 'hihi'", last)
	}
}

func TestUpdate_EnterSendsAndEchoesUser(t *testing.T) {
	app := &stubApp{}
	m := newTestModel(app)
	m.messages.input.SetValue("do the thing")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)

	if len(app.sent) != 1 || app.sent[0] != "do the thing" {
		t.Fatalf("app.sent = %v, want [do the thing]", app.sent)
	}
	last := mm.messages.transcript[len(mm.messages.transcript)-1]
	if last.role != RoleYou || last.text != "do the thing" {
		t.Fatalf("last item = %+v, want you 'do the thing'", last)
	}
	if mm.messages.input.Value() != "" {
		t.Fatalf("input not cleared: %q", mm.messages.input.Value())
	}
	if !mm.working {
		t.Fatal("model should be 'working' after sending")
	}
}

func TestUpdate_EnterEmptyDoesNothing(t *testing.T) {
	app := &stubApp{}
	m := newTestModel(app)
	m.messages.input.SetValue("   ")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)
	if len(app.sent) != 0 || len(mm.messages.transcript) != 0 {
		t.Fatalf("blank input must not send: sent=%v transcript=%v", app.sent, mm.messages.transcript)
	}
}

func TestUpdate_HotkeyOpensSettings(t *testing.T) {
	app := &stubApp{settings: personastate.State{Model: "sonnet", CompactionThreshold: 0.75}}
	m := newTestModel(app)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	mm := updated.(Model)
	if mm.screen != screenSettings {
		t.Fatalf("screen = %v, want settings", mm.screen)
	}
	if _, ok := mm.active.(*settingsScreen); !ok {
		t.Fatalf("ctrl+s should open settings screen, got %T", mm.active)
	}
	if strings.Contains(mm.View(), "Message io") {
		t.Fatalf("settings screen should replace the composer:\n%s", mm.View())
	}
	if !strings.Contains(mm.View(), "SETTINGS") {
		t.Fatalf("settings screen missing title:\n%s", mm.View())
	}
}

func TestUpdate_ResultRefreshesContextAndHappy(t *testing.T) {
	app := &stubApp{ctx: ContextInfo{InputTokens: 1234, ContextWindow: 200000, CostUSD: 0.02}}
	m := newTestModel(app)
	m.working = true
	updated := m.handleEvent(claudeproc.Event{Kind: claudeproc.KindResult})
	if updated.working {
		t.Fatal("result should clear working")
	}
	if updated.expr != Happy {
		t.Fatalf("expr = %v, want Happy after result", updated.expr)
	}
	if updated.ctx.InputTokens != 1234 {
		t.Fatalf("ctx not refreshed: %+v", updated.ctx)
	}
}

func TestNew_PrefillsFromHistory(t *testing.T) {
	app := &stubApp{history: []HistoryEntry{
		{Role: RoleYou, Text: "first"},
		{Role: RoleIO, Text: "hihi ♡"},
		{Role: RoleYou, Text: "second"},
	}}
	m := newTestModel(app)
	if len(m.messages.transcript) != 3 {
		t.Fatalf("transcript len = %d, want 3", len(m.messages.transcript))
	}
	// Only the "you" messages seed the input-recall history.
	if len(m.messages.sent) != 2 || m.messages.sent[0] != "first" || m.messages.sent[1] != "second" {
		t.Fatalf("sent = %v, want [first second]", m.messages.sent)
	}
	if m.messages.histPos != 2 {
		t.Fatalf("histPos = %d, want 2 (new line)", m.messages.histPos)
	}
}

func TestUpDownRecallsSentMessages(t *testing.T) {
	app := &stubApp{history: []HistoryEntry{
		{Role: RoleYou, Text: "first"},
		{Role: RoleYou, Text: "second"},
	}}
	m := newTestModel(app)

	// Up → most recent sent.
	up, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = up.(Model)
	if m.messages.input.Value() != "second" {
		t.Fatalf("after up, input = %q, want 'second'", m.messages.input.Value())
	}
	// Up again → older.
	up, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = up.(Model)
	if m.messages.input.Value() != "first" {
		t.Fatalf("after up x2, input = %q, want 'first'", m.messages.input.Value())
	}
	// Down → back to newer.
	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = down.(Model)
	if m.messages.input.Value() != "second" {
		t.Fatalf("after down, input = %q, want 'second'", m.messages.input.Value())
	}
	// Down past newest → cleared input.
	down, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = down.(Model)
	if m.messages.input.Value() != "" {
		t.Fatalf("after down past newest, input = %q, want empty", m.messages.input.Value())
	}
}

func TestUpdate_NewChatMsgClearsTranscript(t *testing.T) {
	app := &stubApp{}
	m := newTestModel(app)
	m.messages.transcript = []transcriptItem{{role: RoleYou, text: "hi"}}
	updated, _ := m.Update(newChatMsg{})
	mm := updated.(Model)
	if app.newSessions != 1 {
		t.Fatalf("NewSession calls = %d, want 1", app.newSessions)
	}
	if len(mm.messages.transcript) != 0 {
		t.Fatalf("transcript not cleared: %v", mm.messages.transcript)
	}
}

func TestNew_FirstLaunchStartsOnSetupHarness(t *testing.T) {
	m := newTestModel(&stubApp{needsSetup: true})
	if m.screen != screenSetupHarness {
		t.Fatalf("screen = %v, want setup harness", m.screen)
	}
	if !strings.Contains(m.View(), "AGENT LINK") {
		t.Fatalf("setup harness page missing:\n%s", m.View())
	}
}

func TestSetupPagesAnimate(t *testing.T) {
	m := newTestModel(&stubApp{needsSetup: true})
	first := m.View()
	m.frame++
	next := m.View()
	if first == next {
		t.Fatal("setup view did not change across animation frames")
	}
}

func TestSetupPagesCompleteBeforeMessages(t *testing.T) {
	app := &stubApp{needsSetup: true}
	m := newTestModel(app)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.screen != screenSetupPersonality {
		t.Fatalf("screen = %v, want setup personality", m.screen)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !app.completedSetup {
		t.Fatal("CompleteSetup was not called")
	}
	if app.setupHarness != "claude" {
		t.Fatalf("setup harness = %q, want claude", app.setupHarness)
	}
	if app.setupChoice.PresetID == "" {
		t.Fatal("setup choice missing persona")
	}
	if m.screen != screenMessages {
		t.Fatalf("screen = %v, want messages", m.screen)
	}
}
