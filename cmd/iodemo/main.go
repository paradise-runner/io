// Command iodemo runs io's TUI against a canned, offline AppController so the
// chat UI can be exercised and screenshotted without a live claude persona.
// It is a development harness, not a shipped entrypoint.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/edward-champion/io/internal/agentharness"
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/soul"
	"github.com/edward-champion/io/internal/tui"
	"github.com/muesli/termenv"
)

type demoApp struct{}

func (demoApp) NeedsSetup() bool                        { return false }
func (demoApp) CompleteSetup(string, soul.Choice) error { return nil }
func (demoApp) Send(text string) error                  { return nil }
func (demoApp) NewSession() error                       { return nil }
func (demoApp) SetModel(string) error                   { return nil }
func (demoApp) Settings() personastate.State {
	return personastate.State{
		Harness:    string(agentharness.Codex),
		Model:      agentharness.DefaultCodexModel,
		CodexModel: agentharness.DefaultCodexModel,
	}
}
func (demoApp) SaveSettings(personastate.State) error { return nil }
func (demoApp) ContextInfo() tui.ContextInfo {
	return tui.ContextInfo{InputTokens: 18234, ContextWindow: 200000, CostUSD: 0.42}
}
func (demoApp) CompactNow() error { return nil }
func (demoApp) MemorySummary() (string, error) {
	return "# Memory Index\n\n- demo workspace - project notes", nil
}
func (demoApp) History() []tui.HistoryEntry {
	return []tui.HistoryEntry{
		{Role: tui.RoleYou, Text: "hey io, what should I focus on today?"},
		{Role: tui.RoleIO, Text: "Morning! Here's your **Focus / Review** board:\n\n| Key | Area | Summary |\n|-----|------|---------|\n| DEMO-101 | Product | Polish the onboarding copy |\n| DEMO-102 | Docs | Refresh the quickstart screenshot |"},
		{Role: tui.RoleIO, Text: "**Backlog / Refine**"},
		{Role: tui.RoleIO, Text: "| Key | Area | Summary |\n|-----|------|---------|\n| DEMO-203 | App | Add saved workspace shortcuts |\n| DEMO-214 | Ops | Review nightly cleanup logs |\n| DEMO-225 | Docs | Draft release notes |\n| DEMO-231 | Support | Triage feedback from beta users |\n| DEMO-240 | Research | Compare reminder workflows |"},
		{Role: tui.RoleIO, Text: "You've got 2 active items and 5 queued follow-ups. `DEMO-203` is the best next backlog item because it unblocks the settings pass."},
	}
}

func main() {
	var screenshot bool
	var screenshotW int
	var screenshotH int
	var screenshotHold time.Duration
	var screenshotTitle string
	flag.BoolVar(&screenshot, "screenshot", false, "render a deterministic static screen for screenshot capture")
	flag.IntVar(&screenshotW, "width", 100, "screenshot terminal width in columns")
	flag.IntVar(&screenshotH, "height", 40, "screenshot terminal height in rows")
	flag.DurationVar(&screenshotHold, "screenshot-hold", 10*time.Minute, "how long to keep the static screenshot screen open")
	flag.StringVar(&screenshotTitle, "window-title", "io", "terminal window title to show in screenshot mode")
	flag.Parse()

	model := tui.New(demoApp{})
	if screenshot {
		renderScreenshot(model, screenshotW, screenshotH, screenshotHold, screenshotTitle)
		return
	}

	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	prog := tea.NewProgram(model, opts...)
	// Auto-quit after a while so a forgotten harness doesn't linger in tmux.
	go func() { time.Sleep(10 * time.Minute); prog.Quit() }()
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "iodemo:", err)
		os.Exit(1)
	}
}

func renderScreenshot(model tui.Model, width, height int, hold time.Duration, title string) {
	if width < 20 {
		width = 20
	}
	if height < 10 {
		height = 10
	}
	if os.Getenv("IO_TUI_CLOCK") == "" {
		_ = os.Setenv("IO_TUI_CLOCK", "14:11")
	}
	lipgloss.SetColorProfile(termenv.TrueColor)

	if title = cleanTitle(title); title != "" {
		fmt.Printf("\x1b]0;%s\x07", title)
	}
	fmt.Print("\x1b[?25l\x1b[2J\x1b[H")
	fmt.Print(model.ScreenshotView(width, height))
	fmt.Print("\x1b[?25l")
	if hold > 0 {
		time.Sleep(hold)
	}
	fmt.Print("\x1b[?25h")
}

func cleanTitle(title string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, title)
}
