package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const controlHeaderHeight = 4

func controlBodyHeight(height int) int {
	h := height - controlHeaderHeight - 2 // header + blank separator + hint
	if h < 1 {
		return 1
	}
	return h
}

func renderControlPage(title, stage, body, hint string, width, height, frame int) string {
	body = fitControlBody(body, width, controlBodyHeight(height))
	return renderControlPageBody(title, stage, body, hint, width, height, frame)
}

func renderControlPageBody(title, stage, body, hint string, width, height, frame int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	hint = setupHintStyle.Render(truncateDisplay(hint, width))
	page := strings.Join([]string{
		setupHeader(title, stage, width, frame),
		body,
		hint,
	}, "\n")
	return setupPageStyle.Width(width).Height(height).Render(page)
}

func fitControlBody(body string, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[:height]
		lines[height-1] = truncateDisplay(lines[height-1]+"...", width)
	}
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			lines[i] = lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
