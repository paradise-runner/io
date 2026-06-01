package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type messageComponent struct {
	transcript []transcriptItem
	input      textarea.Model
	viewport   viewport.Model
	bar        Bar

	sent    []string
	histPos int
	ready   bool
	width   int
	height  int
}

func newMessageComponent(history []HistoryEntry) messageComponent {
	ta := textarea.New()
	ta.Placeholder = "Message io…"
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.SetHeight(inputHeight)
	// A single pink "›" marks the entry line; lines below it stay clear so the
	// caret has room without a column of repeated glyphs.
	ta.SetPromptFunc(2, func(line int) string {
		if line == 0 {
			return "› "
		}
		return "  "
	})
	styleInput(&ta)

	c := messageComponent{input: ta}
	c.loadHistory(history)
	return c
}

// styleInput dresses the textarea in io's palette so the prompt blends into the
// frame instead of the default black cursor-line highlight and gray text.
func styleInput(ta *textarea.Model) {
	for _, s := range []*textarea.Style{&ta.FocusedStyle, &ta.BlurredStyle} {
		s.Base = lipgloss.NewStyle()
		s.CursorLine = lipgloss.NewStyle() // drop the dark highlight bar
		s.EndOfBuffer = lipgloss.NewStyle().Foreground(colorInk)
		s.Placeholder = lipgloss.NewStyle().Foreground(colorDim)
		s.Prompt = lipgloss.NewStyle().Foreground(colorPink)
		s.Text = lipgloss.NewStyle().Foreground(colorCream)
	}
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(colorPink)
}

func (c *messageComponent) loadHistory(history []HistoryEntry) {
	c.transcript = nil
	c.sent = nil
	for _, h := range history {
		c.transcript = append(c.transcript, transcriptItem{role: h.Role, text: h.Text})
		if h.Role == RoleYou {
			c.sent = append(c.sent, h.Text)
		}
	}
	c.histPos = len(c.sent)
}

func (c messageComponent) Init() tea.Cmd { return textarea.Blink }

func (c *messageComponent) Layout(width, height int) {
	if width < 1 {
		width = 1
	}
	if height < trayHeight+1 {
		height = trayHeight + 1
	}
	c.width = width
	c.height = height

	vpHeight := height - trayHeight
	if vpHeight < 1 {
		vpHeight = 1
	}
	if !c.ready {
		c.viewport = viewport.New(width, vpHeight)
		c.ready = true
	} else {
		c.viewport.Width = width
		c.viewport.Height = vpHeight
	}
	composeInnerW := width - 4 // tray border (2) + horizontal padding (2)
	if composeInnerW < 1 {
		composeInnerW = 1
	}
	c.input.SetWidth(composeInnerW)
}

func (c *messageComponent) Clear() {
	c.transcript = nil
	c.sent = nil
	c.histPos = 0
	c.input.Reset()
	c.Refresh()
}

func (c *messageComponent) AppendAssistant(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	c.transcript = append(c.transcript, transcriptItem{role: RoleIO, text: text})
	c.Refresh()
}

func (c *messageComponent) AppendUser(text string) {
	c.transcript = append(c.transcript, transcriptItem{role: RoleYou, text: text})
	c.sent = append(c.sent, text)
	c.histPos = len(c.sent)
	c.Refresh()
}

func (c messageComponent) Value() string { return c.input.Value() }

func (c *messageComponent) ResetInput() { c.input.Reset() }

func (c *messageComponent) GotoBottom() { c.viewport.GotoBottom() }

func (c messageComponent) AtTop() bool { return c.viewport.AtTop() }

func (c messageComponent) AtBottom() bool { return c.viewport.AtBottom() }

func (c messageComponent) ScrollPercent() float64 { return c.viewport.ScrollPercent() }

func (c messageComponent) ViewportHeight() int { return c.viewport.Height }

func (c messageComponent) BarHitTest(x int) (Action, bool) { return c.bar.HitTest(x) }

func (c messageComponent) HotkeyAction(key string) (Action, bool) {
	return c.bar.HotkeyAction(key)
}

func (c *messageComponent) UpdateViewport(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)
	return cmd
}

func (c *messageComponent) UpdateInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

// TryRecall walks the sent-message history into the input. dir<0 is older,
// dir>0 is newer. It returns false when the key should fall through to the text
// input instead (multi-line editing, or paging past the newest entry).
func (c *messageComponent) TryRecall(dir int) bool {
	if strings.Contains(c.input.Value(), "\n") || len(c.sent) == 0 {
		return false
	}
	if dir < 0 {
		if c.histPos == 0 {
			return true // already at the oldest; swallow the key
		}
		c.histPos--
		c.input.SetValue(c.sent[c.histPos])
		c.input.CursorEnd()
		return true
	}
	if c.histPos >= len(c.sent) {
		return false // not browsing; let the input handle it
	}
	c.histPos++
	if c.histPos >= len(c.sent) {
		c.input.SetValue("")
	} else {
		c.input.SetValue(c.sent[c.histPos])
		c.input.CursorEnd()
	}
	return true
}

func (c *messageComponent) Refresh() {
	if !c.ready {
		return
	}
	// Stay pinned to the newest message only when already following the bottom,
	// so reading back through history isn't yanked away by an incoming reply.
	stick := c.viewport.AtBottom()
	face := Face(Resting, 0)
	var b strings.Builder
	for _, it := range c.transcript {
		if it.role == RoleYou {
			b.WriteString(renderBubble(RoleYou, it.text, "", c.viewport.Width))
		} else {
			for _, seg := range splitSegments(it.text) {
				b.WriteString(renderSegment(seg, face, c.viewport.Width))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	c.viewport.SetContent(b.String())
	if stick {
		c.viewport.GotoBottom()
	}
}

func (c messageComponent) View() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		c.viewport.View(),
		c.composeTray(),
	)
}

// composeTray renders the message composer panel inside the active screen: text
// input, divider rule, and the button toolbar.
func (c messageComponent) composeTray() string {
	innerW := c.width - 4 // tray border (2) + horizontal padding (2)
	if innerW < 1 {
		innerW = 1
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		c.input.View(),
		composeDividerStyle.Render(strings.Repeat("─", innerW)),
		c.bar.View(),
	)
	return composeTrayStyle.Width(innerW + 2).Render(body)
}
