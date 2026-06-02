package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// 90s pocket-communicator chrome geometry.
const (
	// antennaHeight is the number of rows drawn above the frame's top border
	// (signal-wave line + mast line).
	antennaHeight = 2
	// grilleHeight is the speaker/control hardware row that belongs to the
	// device shell, outside the active screen.
	grilleHeight = 1
	// screenBaseChromeHeight is the readout inside the screen when there are no
	// workers: status line + rule.
	screenBaseChromeHeight = 2
	// screenBorder is the active screen's top + bottom rounded-border rows.
	screenBorder = 2
	// mastCol is the column (relative to the frame's left edge) where the
	// antenna mast meets the top border.
	mastCol = 6

	sideButtonWidth  = 6
	sideButtonHeight = 5
	sideButtonTopRel = antennaHeight + 1 + grilleHeight + 2
)

// deviceTop renders the antenna and the frame's top border. The top border is
// drawn here (rather than by lipgloss) so the antenna mast can join it with a
// tee. frameW is the full outer width of the frame, including borders.
func deviceTop(frameW int) string {
	pad := strings.Repeat(" ", mastCol)
	labelPad := strings.Repeat(" ", max(mastCol-4, 0))
	waves := labelPad + antennaLabelStyle.Render("ANT ") + antennaWaveStyle.Render(".)))")
	mast := pad + antennaMastStyle.Render("│")

	var b strings.Builder
	b.WriteString("╭")
	for i := 1; i < frameW-1; i++ {
		if i == mastCol {
			b.WriteString("┴")
		} else {
			b.WriteString("─")
		}
	}
	b.WriteString("╮")
	border := antennaMastStyle.Render(b.String())

	return waves + "\n" + mast + "\n" + border
}

func attachSideButton(device string, frameW, sideW int, pressed bool) string {
	if sideW < 1 {
		return device
	}
	button := sideButtonLines(pressed, sideW)
	lines := strings.Split(device, "\n")
	for i := range lines {
		lines[i] = padVisible(lines[i], frameW)
		side := strings.Repeat(" ", sideW)
		if j := i - sideButtonTopRel; j >= 0 && j < len(button) {
			side = button[j]
		}
		lines[i] += side
	}
	return strings.Join(lines, "\n")
}

func sideButtonLines(pressed bool, w int) []string {
	if w < 1 {
		return nil
	}
	if w < 5 {
		lines := []string{
			sideButtonFrameStyle.Render("▐"),
			sideButtonFaceStyle.Render("◆"),
			sideButtonFrameStyle.Render("▟"),
			"",
			"",
		}
		if pressed {
			lines = []string{
				"",
				sideButtonFrameStyle.Render("▐"),
				sideButtonFaceStyle.Render("◆"),
				sideButtonFrameStyle.Render("▟"),
				"",
			}
		}
		for i := range lines {
			lines[i] = padVisible(lines[i], w)
		}
		return lines
	}
	released := []string{
		sideButtonFrameStyle.Render("╭──╮ "),
		sideButtonFrameStyle.Render("│") + sideButtonFaceHighlightStyle.Render("▔▔") + sideButtonFrameStyle.Render("│") + sideButtonShadowStyle.Render("▏"),
		sideButtonFrameStyle.Render("│") + sideButtonFaceStyle.Render("◆ ") + sideButtonFrameStyle.Render("│") + sideButtonShadowStyle.Render("▏"),
		sideButtonFrameStyle.Render("│") + sideButtonFaceShadowStyle.Render("▄▄") + sideButtonFrameStyle.Render("│") + sideButtonShadowStyle.Render("▏"),
		sideButtonFrameStyle.Render("╰──╯") + sideButtonShadowStyle.Render("▔"),
	}
	if pressed {
		released = []string{
			"",
			" " + sideButtonFrameStyle.Render("╭──╮"),
			" " + sideButtonFrameStyle.Render("│") + sideButtonFaceStyle.Render("◆ ") + sideButtonFrameStyle.Render("│"),
			" " + sideButtonFrameStyle.Render("│") + sideButtonFaceShadowStyle.Render("▄▄") + sideButtonFrameStyle.Render("│"),
			" " + sideButtonFrameStyle.Render("╰──╯"),
		}
	}
	for i := range released {
		released[i] = padVisible(released[i], w)
		if lipgloss.Width(released[i]) > w {
			released[i] = lipgloss.NewStyle().Inline(true).MaxWidth(w).Render(released[i])
		}
	}
	return released
}

func padVisible(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if width := lipgloss.Width(s); width < w {
		return s + strings.Repeat(" ", w-width)
	}
	return s
}

// screenHeader renders the LCD readout that runs along the top of the active
// screen: io state, agent/context readouts, the fun battery + clock, and a rule
// that doubles as a scroll-position indicator. Width is the screen content.
func (m Model) screenHeader() string {
	w := m.screenW
	lines := []string{m.statusLine(w)}
	if m.hasWorkerStatuses() {
		lines = append(lines, m.workerStatusStrip(w))
	}
	lines = append(lines, m.scrollRule(w))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// speakerGrille renders the physical speaker/control band exactly w columns
// wide. It belongs to the communicator shell, outside the active LCD surface.
func speakerGrille(w int) string {
	if w < 1 {
		return ""
	}
	if w < 26 {
		raw := []rune(strings.Repeat("·•", (w+1)/2))
		return grilleStyle.Render(string(raw[:w]))
	}

	const (
		leftScrew = "◎ "
		badge     = " IO-LINK "
		rx        = " RX "
		right     = " ◎"
	)
	slotsW := w - lipgloss.Width(leftScrew) - lipgloss.Width(badge) - lipgloss.Width(rx) - lipgloss.Width("●") - lipgloss.Width(right)
	if slotsW < 1 {
		raw := []rune(strings.Repeat("·•", (w+1)/2))
		return grilleStyle.Render(string(raw[:w]))
	}

	return hardwareScrewStyle.Render("◎") +
		" " +
		grilleStyle.Render(speakerSlots(slotsW)) +
		hardwareBadgeStyle.Render(badge) +
		hardwareLabelStyle.Render(rx) +
		hardwareLedStyle.Render("●") +
		" " +
		hardwareScrewStyle.Render("◎")
}

func speakerSlots(w int) string {
	raw := []rune(strings.Repeat("▥▥ ", (w+2)/3))
	return string(raw[:w])
}

// statusLine renders the communicator readout: current io state on the left,
// useful session readouts when they fit, and the battery + clock on the right.
func (m Model) statusLine(w int) string {
	if w < 1 {
		return ""
	}

	led := ledOnlineStyle.Render("●")
	word := "ready"
	if m.takeoverActive() {
		led = ledWorkingStyle.Render(pulseGlyph(m.frame))
		word = "override"
	} else if m.working {
		led = ledWorkingStyle.Render(pulseGlyph(m.frame))
		word = "thinking"
	} else if m.screen != screenMessages {
		word = m.screenStatus()
	} else if strings.TrimSpace(m.messages.Value()) != "" {
		word = "drafting"
	}
	left := led + statusWordStyle.Render(" ") + m.statusFace() + statusNameStyle.Render(" io") + statusWordStyle.Render(" · ") + statusModeStyle.Render(word)

	sep := statusWordStyle.Render("  ")
	right := strings.Join([]string{
		m.activityMeter(),
		batteryStyle.Render("▰ 95%"),
		clockStyle.Render(time.Now().Format("15:04")),
	}, sep)

	middle := strings.Join(m.statusReadouts(), sep)
	return statusLineFit(w, left, middle, right)
}

func (m Model) statusFace() string {
	style := statusFaceStyle
	switch m.expr {
	case Happy:
		style = statusHappyStyle
	case Sleepy:
		style = statusSleepyStyle
	}
	return style.Render(Face(m.expr, m.frame))
}

func (m Model) screenStatus() string {
	switch m.screen {
	case screenSetupPersonality:
		return "persona"
	case screenSettings:
		return "settings"
	case screenNewChat:
		return "new chat"
	case screenContext:
		return "context"
	case screenMemory:
		return "memory"
	default:
		return "agent link"
	}
}

func (m Model) statusReadouts() []string {
	var out []string
	if m.app != nil {
		st := m.app.Settings()
		st.Normalize()
		if model := st.ActiveModel(); model != "" {
			out = append(out, statusWordStyle.Render("agent ")+statusMetricStyle.Render(st.Harness+"/"+model))
		}
	}
	if m.ctx.ContextWindow > 0 && m.ctx.InputTokens > 0 {
		percent := int(float64(m.ctx.InputTokens)/float64(m.ctx.ContextWindow)*100 + 0.5)
		style := statusMetricStyle
		if percent >= 75 {
			style = statusWarnStyle
		}
		out = append(out, statusWordStyle.Render("ctx ")+style.Render(fmt.Sprintf("%d%%", percent)))
	}
	return out
}

func statusLineFit(w int, left, middle, right string) string {
	const minGap = 1
	if middle != "" && lipgloss.Width(left)+lipgloss.Width(middle)+lipgloss.Width(right)+minGap*2 <= w {
		spare := w - lipgloss.Width(left) - lipgloss.Width(middle) - lipgloss.Width(right)
		leftGap := spare / 2
		rightGap := spare - leftGap
		if leftGap < minGap {
			leftGap = minGap
		}
		if rightGap < minGap {
			rightGap = minGap
		}
		return left + strings.Repeat(" ", leftGap) + middle + strings.Repeat(" ", rightGap) + right
	}

	if lipgloss.Width(left)+lipgloss.Width(right)+minGap <= w {
		gap := w - lipgloss.Width(left) - lipgloss.Width(right)
		return left + strings.Repeat(" ", gap) + right
	}
	if lipgloss.Width(left) <= w {
		return left + strings.Repeat(" ", w-lipgloss.Width(left))
	}
	return lipgloss.NewStyle().Inline(true).MaxWidth(w).Render(left)
}

// activityMeter renders four tiny bars. While io is working, one lit bar moves
// across the meter; otherwise it rests at a steady near-full idle signal.
func (m Model) activityMeter() string {
	const n = 4
	on := []bool{true, true, true, false}
	if m.working {
		on = []bool{false, false, false, false}
		on[m.frame%n] = true
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		if on[i] {
			b.WriteString(signalOnStyle.Render("▮"))
		} else {
			b.WriteString(signalOffStyle.Render("▯"))
		}
	}
	return b.String()
}

// scrollRule renders the divider under the status line. When the history is
// scrollable it splices in a percentage readout so scroll position is visible.
func (m Model) scrollRule(w int) string {
	runes := []rune(strings.Repeat("─", w))
	if m.working && w > 5 {
		trail := []rune("╼━╾")
		pos := 1 + (m.frame % (w - len(trail) - 1))
		copy(runes[pos:], trail)
	}
	if m.ready && m.screen == screenMessages && !(m.messages.AtTop() && m.messages.AtBottom()) {
		label := []rune(fmt.Sprintf(" scroll %d%% ", int(m.messages.ScrollPercent()*100)))
		pos := w - len(label) - 2
		if pos > 1 {
			copy(runes[pos:], label)
		}
	}
	return ruleStyle.Render(string(runes))
}

// pulseGlyph animates the status LED while io is thinking.
func pulseGlyph(frame int) string {
	glyphs := []string{"◍", "◉", "●", "◉"}
	return glyphs[frame%len(glyphs)]
}
