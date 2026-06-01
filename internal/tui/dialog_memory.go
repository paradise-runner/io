package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// memoryScreen shows io's memory index (read-only).
type memoryScreen struct {
	text     string
	viewport viewport.Model
	ready    bool
	width    int
	height   int
}

func newMemoryScreen(text string) *memoryScreen {
	if text == "" {
		text = "io hasn't saved any memories yet ♡"
	}
	return &memoryScreen{text: text}
}

func (d *memoryScreen) Layout(width, height int) {
	if width < 1 {
		width = 1
	}
	bodyH := controlBodyHeight(height)
	if !d.ready {
		d.viewport = viewport.New(width, bodyH)
		d.ready = true
	} else {
		d.viewport.Width = width
		d.viewport.Height = bodyH
	}
	d.width = width
	d.height = height
	d.viewport.SetContent(lipgloss.NewStyle().Width(width).Render(d.text))
}

func (d *memoryScreen) Update(msg tea.Msg) (controlScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "enter", "q":
			return nil, nil
		case "up", "down", "pgup", "pgdown", "ctrl+u", "ctrl+d":
			var cmd tea.Cmd
			d.viewport, cmd = d.viewport.Update(msg)
			return d, cmd
		}
	case tea.MouseMsg:
		var cmd tea.Cmd
		d.viewport, cmd = d.viewport.Update(msg)
		return d, cmd
	}
	return d, nil
}

func (d *memoryScreen) View(width, height, frame int) string {
	if !d.ready || d.width != width || d.height != height {
		d.Layout(width, height)
	}
	return renderControlPageBody("MEMORY", "STORE", d.viewport.View(), "up/down scroll   esc back", width, height, frame)
}
