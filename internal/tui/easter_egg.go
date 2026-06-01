package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func takeoverScreen(w, h, frame int) string {
	if w < 1 || h < 1 {
		return ""
	}

	lines := make([]string, h)
	scanY := frame % h
	for y := 0; y < h; y++ {
		line := takeoverNoiseLine(w, y, frame)
		if y == scanY {
			line = takeoverSignalStyle.Render(strings.Repeat("━", w))
		} else {
			line = takeoverBodyStyle.Render(line)
		}
		lines[y] = padVisible(line, w)
	}

	center := h / 2
	placeTakeoverLine(lines, center-2, w, "╔═ IO-LINK OVERRIDE ═╗", takeoverFrameStyle)
	placeTakeoverLine(lines, center-1, w, Face(Working, frame), takeoverTitleStyle)
	placeTakeoverLine(lines, center, w, takeoverSignal(frame, w), takeoverSignalStyle)
	placeTakeoverLine(lines, center+1, w, "secret channel unlocked", takeoverBodyStyle)

	return strings.Join(lines, "\n")
}

func takeoverNoiseLine(w, y, frame int) string {
	const glyphs = ".·░▒ "
	gr := []rune(glyphs)
	var b strings.Builder
	for x := 0; x < w; x++ {
		idx := (x*3 + y*5 + frame) % len(gr)
		if (x+frame)%17 == y%7 {
			b.WriteRune('✦')
			continue
		}
		b.WriteRune(gr[idx])
	}
	return b.String()
}

func takeoverSignal(frame, w int) string {
	glyphs := []string{"<<<", "<<>", "<>>", ">>>", ">><", "><<"}
	text := " " + glyphs[frame%len(glyphs)] + " SIGNAL BLOOM " + glyphs[(frame+3)%len(glyphs)] + " "
	if lipgloss.Width(text) <= w {
		return text
	}
	return "SIGNAL"
}

func placeTakeoverLine(lines []string, y, w int, text string, style lipgloss.Style) {
	if y < 0 || y >= len(lines) || w < 1 {
		return
	}
	if lipgloss.Width(text) > w {
		text = lipgloss.NewStyle().Inline(true).MaxWidth(w).Render(text)
	}
	styled := style.Render(text)
	left := (w - lipgloss.Width(styled)) / 2
	if left < 0 {
		left = 0
	}
	lines[y] = padVisible(strings.Repeat(" ", left)+styled, w)
}
