package tui

import "strings"

// SegKind classifies a chunk of an io reply.
type SegKind int

const (
	Prose SegKind = iota
	Code
	Table
)

// Segment is one renderable chunk of a reply: prose goes in a bubble, code and
// tables break out into full-width panels.
type Segment struct {
	Kind SegKind
	Text string
	Lang string // Code only
}

func isTableRow(line string) bool {
	return strings.Contains(line, "|")
}

func isTableSeparator(line string) bool {
	s := strings.TrimSpace(line)
	if !strings.Contains(s, "|") || !strings.Contains(s, "-") {
		return false
	}
	for _, r := range s {
		switch r {
		case '|', '-', ':', ' ':
		default:
			return false
		}
	}
	return true
}

// splitSegments breaks an io reply into prose, fenced code blocks, and markdown
// tables, in order. Blank-only prose runs are dropped.
func splitSegments(text string) []Segment {
	lines := strings.Split(text, "\n")
	var segs []Segment
	var prose []string

	flush := func() {
		joined := strings.TrimRight(strings.Join(prose, "\n"), "\n")
		if strings.TrimSpace(joined) != "" {
			segs = append(segs, Segment{Kind: Prose, Text: joined})
		}
		prose = nil
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flush()
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			var body []string
			i++
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				body = append(body, lines[i])
				i++
			}
			segs = append(segs, Segment{Kind: Code, Text: strings.Join(body, "\n"), Lang: lang})
			continue
		}

		if isTableRow(line) && i+1 < len(lines) && isTableSeparator(lines[i+1]) {
			flush()
			var body []string
			for i < len(lines) && isTableRow(lines[i]) {
				body = append(body, lines[i])
				i++
			}
			i--
			segs = append(segs, Segment{Kind: Table, Text: strings.Join(body, "\n")})
			continue
		}

		prose = append(prose, line)
	}
	flush()
	return segs
}

// renderSegment renders one segment: prose as an io bubble, code/tables as a
// full-width lavender-framed panel labeled with the io avatar.
func renderSegment(seg Segment, face string, maxWidth int) string {
	switch seg.Kind {
	case Code, Table:
		title := "table"
		if seg.Kind == Code {
			title = "code"
			if seg.Lang != "" {
				title = seg.Lang
			}
		}
		head := panelTitleStyle.Render(" " + face + " " + title)
		w := maxWidth - 4
		if w < 20 {
			w = 20
		}
		body := panelStyle.Width(w).Render(seg.Text)
		return head + "\n" + body
	default:
		return renderBubble(RoleIO, seg.Text, face, maxWidth)
	}
}
