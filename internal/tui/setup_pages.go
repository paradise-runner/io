package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/edward-champion/io/internal/agentharness"
	"github.com/edward-champion/io/internal/personastate"
	"github.com/edward-champion/io/internal/soul"
)

type setupFlow struct {
	harness   string
	persona   int
	lastError string
}

func newSetupFlow(st personastate.State) setupFlow {
	st.Normalize()
	return setupFlow{harness: st.Harness}
}

func (s *setupFlow) cycleHarness(dir int) {
	options := agentharness.HarnessOptions()
	idx := 0
	for i, option := range options {
		if option == s.harness {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(options)) % len(options)
	s.harness = options[idx]
	s.lastError = ""
}

func (s *setupFlow) cyclePersona(dir int) {
	options := soul.Presets()
	s.persona = (s.persona + dir + len(options)) % len(options)
	s.lastError = ""
}

func (s setupFlow) choice() soul.Choice {
	presets := soul.Presets()
	idx := s.persona
	if idx < 0 || idx >= len(presets) {
		idx = 0
	}
	return soul.Choice{
		PresetID:    presets[idx].ID,
		Verbosity:   soul.Balanced,
		Proactivity: soul.Reactive,
	}
}

func (s setupFlow) View(page screenKind, width, height, frame int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	var body string
	switch page {
	case screenSetupPersonality:
		body = s.personalityView(width, frame)
	default:
		body = s.harnessView(width, frame)
	}
	return setupPageStyle.Width(width).Height(height).Render(body)
}

func (s setupFlow) harnessView(width, frame int) string {
	var b strings.Builder
	b.WriteString(setupHeader("AGENT LINK", "STAGE 1/2", width, frame))
	b.WriteString("\n")
	for _, option := range agentharness.HarnessOptions() {
		b.WriteString(s.optionLine(option, harnessDescription(option), option == s.harness, width, frame))
		b.WriteString("\n")
	}
	b.WriteString(setupHintStyle.Render("↑↓ SELECT   ⏎ OK"))
	if s.lastError != "" {
		b.WriteString("\n\n")
		b.WriteString(setupErrorStyle.Render(s.lastError))
	}
	return b.String()
}

func (s setupFlow) personalityView(width, frame int) string {
	var b strings.Builder
	b.WriteString(setupHeader("SOUL CHIP", "STAGE 2/2", width, frame))
	b.WriteString("\n")
	presets := soul.Presets()
	for i, preset := range presets {
		b.WriteString(s.optionLine(preset.Name, preset.Description, i == s.persona, width, frame))
		b.WriteString("\n")
	}
	b.WriteString(setupHintStyle.Render("↑↓ SELECT   ⏎ START   ESC BACK"))
	if s.lastError != "" {
		b.WriteString("\n\n")
		b.WriteString(setupErrorStyle.Render(s.lastError))
	}
	return b.String()
}

func setupHeader(title, stage string, width, frame int) string {
	sparkles := []string{"✦", "♡", "◆", "✧"}
	left := sparkles[frame%len(sparkles)]
	right := sparkles[(frame+2)%len(sparkles)]
	titleLine := setupTitleStyle.Render(left + " " + title + " " + right)
	stageLine := setupStageStyle.Render(stage + " " + setupMeter(frame))
	return strings.Join([]string{
		setupRail(width, frame),
		lipgloss.PlaceHorizontal(width, lipgloss.Center, titleLine),
		lipgloss.PlaceHorizontal(width, lipgloss.Center, stageLine),
		setupRail(width, frame+2),
	}, "\n")
}

func setupRail(width, frame int) string {
	if width < 1 {
		return ""
	}
	cells := make([]string, width)
	for i := range cells {
		if (i+frame)%4 == 0 {
			cells[i] = "·"
		} else {
			cells[i] = " "
		}
	}
	marks := []string{"✦", "♡", "◆"}
	for i, mark := range marks {
		pos := (frame*2 + i*(width/3+1)) % width
		cells[pos] = mark
	}
	return setupRailStyle.Render(strings.Join(cells, ""))
}

func setupMeter(frame int) string {
	pieces := []string{"▰▱▱", "▰▰▱", "▰▰▰", "▱▰▰", "▱▱▰", "▱▰▱"}
	return pieces[frame%len(pieces)]
}

func (s setupFlow) optionLine(name, description string, selected bool, width, frame int) string {
	marker := "◇"
	nameStyle := setupOptionStyle
	if selected {
		cursors := []string{"▶", "▸", "◆", "▸"}
		marker = cursors[frame%len(cursors)]
		nameStyle = setupSelectedStyle
	}
	nameWidth := width - 5
	descWidth := width - 5
	if nameWidth < 1 {
		nameWidth = 1
	}
	if descWidth < 1 {
		descWidth = 1
	}
	return fmt.Sprintf("%s %s\n  %s",
		setupCursorStyle.Render(marker),
		nameStyle.Render(" "+truncateDisplay(name, nameWidth)+" "),
		setupDescriptionStyle.Render(truncateDisplay(description, descWidth)),
	)
}

func truncateDisplay(s string, maxWidth int) string {
	if maxWidth < 1 || lipgloss.Width(s) <= maxWidth {
		return s
	}
	const ellipsis = "…"
	limit := maxWidth - lipgloss.Width(ellipsis)
	if limit < 1 {
		return ellipsis
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if width+w > limit {
			break
		}
		b.WriteRune(r)
		width += w
	}
	return b.String() + ellipsis
}

func harnessDescription(harness string) string {
	switch harness {
	case string(agentharness.Codex):
		return "Codex CLI with GPT-5.4 by default."
	default:
		return "Claude Code with Sonnet by default."
	}
}
