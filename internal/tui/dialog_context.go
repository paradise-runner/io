package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// contextScreen shows context usage and can request manual compaction.
type contextScreen struct {
	app    AppController
	info   ContextInfo
	status string
}

func newContextScreen(app AppController) *contextScreen {
	info := app.ContextInfo()
	return &contextScreen{app: app, info: info, status: info.Compaction}
}

func (d *contextScreen) Layout(width, height int) {}

func (d *contextScreen) Update(msg tea.Msg) (controlScreen, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return d, nil
	}
	switch k.String() {
	case "esc", "enter", "q":
		return nil, nil
	case "c":
		if err := d.app.CompactNow(); err != nil {
			d.status = err.Error()
		} else {
			d.status = "compaction requested"
		}
		d.info = d.app.ContextInfo()
		if d.info.Compaction != "" {
			d.status = d.info.Compaction
		}
		return d, nil
	default:
		return nil, nil
	}
}

func (d *contextScreen) View(width, height, frame int) string {
	tokens := "—"
	window := "—"
	pct := "—"
	if d.info.InputTokens > 0 {
		tokens = fmt.Sprintf("%d", d.info.InputTokens)
	}
	if d.info.ContextWindow > 0 {
		window = fmt.Sprintf("%d", d.info.ContextWindow)
		if d.info.InputTokens > 0 {
			pct = fmt.Sprintf("%.0f%%", 100*float64(d.info.InputTokens)/float64(d.info.ContextWindow))
		}
	}
	cost := fmt.Sprintf("$%.4f", d.info.CostUSD)
	status := strings.TrimSpace(d.status)
	if status == "" {
		status = "idle"
	}
	dreaming := strings.TrimSpace(d.info.Dreaming)
	if dreaming == "" {
		dreaming = "idle"
	}

	body := fmt.Sprintf(
		"last turn:   %s tokens\nwindow:      %s\nused:        %s\nsession:     %s\ncompact:     %s\ndreaming:    %s",
		tokens, window, pct, cost, status, dreaming)

	return renderControlPage("CONTEXT", "USAGE", body, "c compact now   esc back", width, height, frame)
}
