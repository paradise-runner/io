package tui

import tea "github.com/charmbracelet/bubbletea"

// controlScreen is a full-screen page inside the communicator LCD. Update
// returns the next screen state, or nil to return to the chat screen.
type controlScreen interface {
	Layout(width, height int)
	Update(tea.Msg) (controlScreen, tea.Cmd)
	View(width, height, frame int) string
}

// newChatMsg is emitted by the New-chat confirmation screen; the model handles
// it by starting a fresh session and clearing the transcript.
type newChatMsg struct{}
