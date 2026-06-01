package tui

import "github.com/charmbracelet/lipgloss"

// Role distinguishes the two speakers in the chat.
type Role int

const (
	RoleYou Role = iota
	RoleIO
)

// bubbleContentWidth returns the wrapped content width for a bubble: about 60%
// of the available width, clamped to sane bounds.
func bubbleContentWidth(maxWidth int) int {
	if maxWidth < 24 {
		maxWidth = 24
	}
	w := maxWidth * 6 / 10
	if w < 12 {
		w = 12
	}
	if w > maxWidth-6 {
		w = maxWidth - 6
	}
	return w
}

// renderBubble renders one chat message. io messages are left-aligned with the
// given avatar face + label in the gutter; your messages are right-aligned. The
// face is supplied by the caller so the avatar can animate.
func renderBubble(role Role, text, face string, maxWidth int) string {
	cw := bubbleContentWidth(maxWidth)
	if role == RoleIO {
		bubble := ioBubbleStyle.Width(cw).Render(text)
		gutter := avatarStyle.Render(face) + "\n" + labelStyle.Render(" io")
		return lipgloss.JoinHorizontal(lipgloss.Top, gutter, bubble)
	}
	bubble := youBubbleStyle.Width(cw).Render(text)
	placed := lipgloss.PlaceHorizontal(maxWidth, lipgloss.Right, bubble)
	label := lipgloss.PlaceHorizontal(maxWidth, lipgloss.Right, labelStyle.Render("you "))
	return placed + "\n" + label
}
