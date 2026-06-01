package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TestView_FitsTerminal guards the device-frame width math: content sized to
// interiorW must not wrap inside the frame's padding, which would inflate
// View() past the terminal height and scroll the top of the chat off-screen.
func TestView_FitsTerminal(t *testing.T) {
	app := &stubApp{history: []HistoryEntry{
		{Role: RoleYou, Text: "a fairly long user line that lands near the right edge of the frame to test wrapping"},
		{Role: RoleIO, Text: "an io reply that is also long enough to exercise full-width rendering inside the frame"},
		{Role: RoleYou, Text: "and one more user line to push content well past the bottom of the viewport for scrolling"},
	}}
	for _, dim := range [][2]int{{100, 40}, {120, 50}, {80, 30}, {100, 24}} {
		var m tea.Model = New(app)
		m, _ = m.Update(tea.WindowSizeMsg{Width: dim[0], Height: dim[1]})
		for k := 0; k < 20; k++ { // scroll to the top — the worst case for overflow
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		}
		v := m.(Model).View()
		if h := lipgloss.Height(v); h > dim[1] {
			t.Errorf("term %dx%d: View height %d exceeds terminal height %d", dim[0], dim[1], h, dim[1])
		}
		for _, line := range strings.Split(v, "\n") {
			if w := lipgloss.Width(line); w > dim[0] {
				t.Errorf("term %dx%d: line width %d exceeds terminal width %d: %q", dim[0], dim[1], w, dim[0], line)
				break
			}
		}
	}
}

func TestSetupView_FitsTerminal(t *testing.T) {
	for _, dim := range [][2]int{{100, 40}, {80, 30}, {100, 24}} {
		var m tea.Model = New(&stubApp{needsSetup: true})
		m, _ = m.Update(tea.WindowSizeMsg{Width: dim[0], Height: dim[1]})
		v := m.(Model).View()
		if h := lipgloss.Height(v); h > dim[1] {
			t.Errorf("term %dx%d: setup View height %d exceeds terminal height %d", dim[0], dim[1], h, dim[1])
		}
		for _, line := range strings.Split(v, "\n") {
			if w := lipgloss.Width(line); w > dim[0] {
				t.Errorf("term %dx%d: setup line width %d exceeds terminal width %d: %q", dim[0], dim[1], w, dim[0], line)
				break
			}
		}
	}
}

func TestControlScreens_FitTerminal(t *testing.T) {
	app := &stubApp{
		ctx:    ContextInfo{InputTokens: 1234, ContextWindow: 200000, CostUSD: 0.02},
		memory: strings.Repeat("a saved memory line with enough words to wrap inside the LCD surface\n", 12),
	}
	for _, action := range []Action{ActionSettings, ActionNewChat, ActionContext, ActionMemory} {
		for _, dim := range [][2]int{{100, 40}, {80, 30}, {100, 24}} {
			var model tea.Model = New(app)
			model, _ = model.Update(tea.WindowSizeMsg{Width: dim[0], Height: dim[1]})
			m := model.(Model)
			m.openScreen(action)
			v := m.View()
			if h := lipgloss.Height(v); h > dim[1] {
				t.Errorf("action %v term %dx%d: View height %d exceeds terminal height %d", action, dim[0], dim[1], h, dim[1])
			}
			for _, line := range strings.Split(v, "\n") {
				if w := lipgloss.Width(line); w > dim[0] {
					t.Errorf("action %v term %dx%d: line width %d exceeds terminal width %d: %q", action, dim[0], dim[1], w, dim[0], line)
					break
				}
			}
			if strings.Contains(v, "Message io") {
				t.Errorf("action %v should replace the composer", action)
			}
		}
	}
}

func TestLayout_NestsMessagesInsideScreenShell(t *testing.T) {
	m := newTestModel(&stubApp{})
	if m.screenW != m.interiorW-4 {
		t.Fatalf("screenW = %d, want interiorW-4 (%d)", m.screenW, m.interiorW-4)
	}
	if m.screenH != m.interiorH-grilleHeight {
		t.Fatalf("screenH = %d, want interiorH-grilleHeight (%d)", m.screenH, m.interiorH-grilleHeight)
	}
	if m.messages.viewport.Width != m.screenW {
		t.Fatalf("viewport.Width = %d, want screenW %d", m.messages.viewport.Width, m.screenW)
	}
	wantViewportH := m.screenH - screenBorder - screenChromeHeight - trayHeight
	if m.messages.viewport.Height != wantViewportH {
		t.Fatalf("viewport.Height = %d, want screen height minus chrome and composer (%d)", m.messages.viewport.Height, wantViewportH)
	}
	if got := lipgloss.Width(m.messages.composeTray()); got != m.screenW {
		t.Fatalf("composeTray width = %d, want screenW %d", got, m.screenW)
	}
	if got := lipgloss.Height(m.messages.composeTray()); got != trayHeight {
		t.Fatalf("composeTray height = %d, want trayHeight %d", got, trayHeight)
	}
	if got := lipgloss.Height(m.screenShell()); got != m.screenH {
		t.Fatalf("screenShell height = %d, want screenH %d", got, m.screenH)
	}
	v := m.View()
	if !strings.Contains(v, "ready") || !strings.Contains(v, "agent claude/sonnet") {
		t.Fatalf("screen shell should include status readout:\n%s", v)
	}
	if !strings.Contains(v, "Message io") {
		t.Fatalf("screen shell should include message composer:\n%s", v)
	}
}

func TestStatusLine_KeepsBatteryAndClock(t *testing.T) {
	m := newTestModel(&stubApp{})
	line := m.statusLine(72)
	if !strings.Contains(line, "▰ 95%") {
		t.Fatalf("status line should keep the fun battery: %q", line)
	}
	if !strings.Contains(line, ":") {
		t.Fatalf("status line should include a clock: %q", line)
	}
}

func TestHardwareChrome_AddsReadableDeviceDetails(t *testing.T) {
	top := deviceTop(72)
	if !strings.Contains(top, "ANT") || !strings.Contains(top, ".)))") {
		t.Fatalf("device top should label the antenna signal:\n%s", top)
	}

	for _, w := range []int{10, 26, 72} {
		grille := speakerGrille(w)
		if got := lipgloss.Width(grille); got != w {
			t.Fatalf("speakerGrille(%d) width = %d, want %d: %q", w, got, w, grille)
		}
	}

	grille := speakerGrille(72)
	for _, want := range []string{"◎", "IO-LINK", "RX", "●"} {
		if !strings.Contains(grille, want) {
			t.Fatalf("hardware band should include %q detail: %q", want, grille)
		}
	}
}

func TestSideButton_AttachesAndAnimatesPress(t *testing.T) {
	device := deviceTop(40) + "\n" + strings.Repeat("│"+strings.Repeat(" ", 38)+"│\n", 10)
	released := attachSideButton(device, 40, sideButtonWidth, false)
	pressed := attachSideButton(device, 40, sideButtonWidth, true)

	if released == pressed {
		t.Fatal("pressed side button should render differently")
	}
	if !strings.Contains(released, "◆") || !strings.Contains(pressed, "◆") {
		t.Fatalf("side button should include its button face:\nreleased:\n%s\npressed:\n%s", released, pressed)
	}
	for _, line := range strings.Split(released, "\n") {
		if w := lipgloss.Width(line); w != 40+sideButtonWidth {
			t.Fatalf("released line width = %d, want %d: %q", w, 40+sideButtonWidth, line)
		}
	}
}

func TestTakeoverScreen_FillsBounds(t *testing.T) {
	for _, dim := range [][2]int{{20, 6}, {72, 18}} {
		out := takeoverScreen(dim[0], dim[1], 2)
		if h := lipgloss.Height(out); h != dim[1] {
			t.Fatalf("takeover height = %d, want %d:\n%s", h, dim[1], out)
		}
		for _, line := range strings.Split(out, "\n") {
			if w := lipgloss.Width(line); w != dim[0] {
				t.Fatalf("takeover line width = %d, want %d: %q", w, dim[0], line)
			}
		}
	}
}

func TestStatusLine_WorkingAnimates(t *testing.T) {
	m := newTestModel(&stubApp{})
	m.working = true

	m.frame = 0
	first := m.statusLine(72) + "\n" + m.scrollRule(72)
	m.frame = 1
	next := m.statusLine(72) + "\n" + m.scrollRule(72)

	if first == next {
		t.Fatalf("working status should animate:\n%s", first)
	}
	if !strings.Contains(first, "thinking") {
		t.Fatalf("working status should say thinking:\n%s", first)
	}
}

func TestIsMouseNoise(t *testing.T) {
	cases := []struct {
		name       string
		runes      string
		recentScrl bool
		want       bool
	}{
		{"full SGR body", "[<65;51;20M", false, true},
		{"repeated bodies", "[<65;51;20M[<64;10;10M", false, true},
		{"bare bracket during scroll", "[", true, true},
		{"angle head during scroll", "<65;51;2", true, true},
		{"bare bracket while typing", "[", false, false},
		{"normal text", "hello", true, false},
		{"code with brackets while typing", "arr[0]", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{}
			if c.recentScrl {
				m.lastScroll = time.Now()
			} else {
				m.lastScroll = time.Now().Add(-time.Second)
			}
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.runes)}
			if got := m.isMouseNoise(msg); got != c.want {
				t.Errorf("isMouseNoise(%q, recentScroll=%v) = %v, want %v", c.runes, c.recentScrl, got, c.want)
			}
		})
	}
}
