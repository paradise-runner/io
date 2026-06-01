// Command iodemo runs io's TUI against a canned, offline AppController so the
// chat UI can be exercised and screenshotted without a live claude persona.
// It is a development harness, not a shipped entrypoint.
package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/soul"
	"github.com/edward-champion/io/internal/tui"
)

type demoApp struct{}

func (demoApp) NeedsSetup() bool                        { return false }
func (demoApp) CompleteSetup(string, soul.Choice) error { return nil }
func (demoApp) Send(text string) error                  { return nil }
func (demoApp) NewSession() error                       { return nil }
func (demoApp) SetModel(string) error                   { return nil }
func (demoApp) Settings() personastate.State            { return personastate.State{Model: "sonnet"} }
func (demoApp) SaveSettings(personastate.State) error   { return nil }
func (demoApp) ContextInfo() tui.ContextInfo {
	return tui.ContextInfo{InputTokens: 18234, ContextWindow: 200000, CostUSD: 0.42}
}
func (demoApp) MemorySummary() (string, error) { return "# Memory Index\n\n- io project — TUI", nil }
func (demoApp) History() []tui.HistoryEntry {
	return []tui.HistoryEntry{
		{Role: tui.RoleYou, Text: "hey io, what's on my plate today?"},
		{Role: tui.RoleIO, Text: "Morning! Here's your **In Progress / Review** board:\n\n| Key | Project | Summary |\n|-----|---------|---------|\n| ACE-268 | ACE | Implement label gating + preview environments for enablement-data-portal |\n| ACE-201 | ACE | Wire up the nightly backfill |"},
		{Role: tui.RoleIO, Text: "**Backlog / Refine**"},
		{Role: tui.RoleIO, Text: "| Key | Project | Summary |\n|-----|---------|---------|\n| ACE-274 | ACE | okta-datalake-pipeline: gold_transform.py — current-state engineering domain gold table |\n| ACE-118 | ACE | Python Packages Paved Road *(Epic)* |\n| RARE-403 | RARE | Kubernetes Evaluation *(High priority)* |\n| RARE-393 | RARE | Implement DD Only Monitor Zone |\n| CLDE-89 | CLDE | Transform service to be a github app |"},
		{Role: tui.RoleIO, Text: "You've got 4 tickets actively in-flight (Development/Review). `RARE-403` is the only High priority item sitting in Backlog — may be worth a look."},
	}
}

func main() {
	model := tui.New(demoApp{})
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	prog := tea.NewProgram(model, opts...)
	// Auto-quit after a while so a forgotten harness doesn't linger in tmux.
	go func() { time.Sleep(10 * time.Minute); prog.Quit() }()
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "iodemo:", err)
		os.Exit(1)
	}
}
