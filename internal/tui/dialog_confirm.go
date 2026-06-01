package tui

import tea "github.com/charmbracelet/bubbletea"

// confirmScreen is a generic yes/no page. On confirm it emits onYes.
type confirmScreen struct {
	title string
	body  string
	onYes tea.Msg
}

func newConfirmScreen(title, body string, onYes tea.Msg) *confirmScreen {
	return &confirmScreen{title: title, body: body, onYes: onYes}
}

func (d *confirmScreen) Layout(width, height int) {}

func (d *confirmScreen) Update(msg tea.Msg) (controlScreen, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "y", "Y", "enter":
			m := d.onYes
			return nil, func() tea.Msg { return m }
		case "n", "N", "esc":
			return nil, nil
		}
	}
	return d, nil
}

func (d *confirmScreen) View(width, height, frame int) string {
	return renderControlPage(d.title, "CONFIRM", d.body, "y/enter yes   n/esc no", width, height, frame)
}
