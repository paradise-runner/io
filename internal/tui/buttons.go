package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Action identifies what a footer button does.
type Action int

const (
	ActionNone Action = iota
	ActionSettings
	ActionNewChat
	ActionContext
	ActionMemory
)

type button struct {
	action Action
	icon   string
	label  string
	hotkey string         // matches tea.KeyMsg.String(), e.g. "ctrl+s"
	bg     lipgloss.Color // chip background fill
}

// barButtons is the footer button set. Memory uses ^R because ^M is Enter in
// terminals. Colors pair by meaning: lavender for the system controls
// (Settings, Context), mint for a fresh start (New), pink for keepsakes
// (Memory).
var barButtons = []button{
	{ActionSettings, "⚙", "Settings", "ctrl+s", colorLavender},
	{ActionNewChat, "✨", "New", "ctrl+n", colorMint},
	{ActionContext, "◔", "Context", "ctrl+o", colorLavender},
	{ActionMemory, "♡", "Memory", "ctrl+r", colorPink},
}

const barSeparator = 2 // visible columns between buttons

var (
	bevelLeftStyle  = lipgloss.NewStyle().Foreground(colorCream)
	bevelRightStyle = lipgloss.NewStyle().Foreground(colorInk)
)

func hotkeyDisplay(hk string) string {
	return "^" + strings.ToUpper(strings.TrimPrefix(hk, "ctrl+"))
}

// Bar is the (stateless) footer button bar.
type Bar struct{}

func (Bar) renderButton(b button) string {
	content := buttonHotkeyStyle.Background(b.bg).Render(hotkeyDisplay(b.hotkey)+" ") +
		buttonStyle.Background(b.bg).Render(b.icon+" "+b.label)
	return bevelLeftStyle.Render("▕") + content + bevelRightStyle.Render("▏")
}

// View renders the whole bar as a single line.
func (bar Bar) View() string {
	parts := make([]string, len(barButtons))
	for i, b := range barButtons {
		parts[i] = bar.renderButton(b)
	}
	return strings.Join(parts, strings.Repeat(" ", barSeparator))
}

// HitTest maps a mouse x-coordinate (relative to the bar's left edge) to a
// button action.
func (bar Bar) HitTest(x int) (Action, bool) {
	if x < 0 {
		return ActionNone, false
	}
	pos := 0
	for _, b := range barButtons {
		w := lipgloss.Width(bar.renderButton(b))
		if x >= pos && x < pos+w {
			return b.action, true
		}
		pos += w + barSeparator
	}
	return ActionNone, false
}

// HotkeyAction maps a key string (tea.KeyMsg.String()) to a button action.
func (Bar) HotkeyAction(key string) (Action, bool) {
	for _, b := range barButtons {
		if b.hotkey == key {
			return b.action, true
		}
	}
	return ActionNone, false
}
