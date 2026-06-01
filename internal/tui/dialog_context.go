package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// contextScreen is a read-only readout of context usage and cost.
type contextScreen struct {
	info ContextInfo
}

func newContextScreen(info ContextInfo) *contextScreen {
	return &contextScreen{info: info}
}

func (d *contextScreen) Layout(width, height int) {}

func (d *contextScreen) Update(msg tea.Msg) (controlScreen, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return nil, nil // any key returns to chat
	}
	return d, nil
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

	body := fmt.Sprintf(
		"last turn:   %s tokens\nwindow:      %s\nused:        %s\nsession:     %s",
		tokens, window, pct, cost)

	return renderControlPage("CONTEXT", "USAGE", body, "any key back", width, height, frame)
}
