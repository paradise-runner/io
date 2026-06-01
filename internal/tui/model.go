package tui

import (
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/edward-champion/io/internal/claudeproc"
)

const (
	barHeight     = 1
	inputHeight   = 3
	dividerHeight = 1 // rule between the input and the toolbar inside the tray
	trayBorder    = 2 // the compose tray's top + bottom rounded-border rows
	// trayHeight is the full vertical footprint of the compose tray inside the
	// active screen: border + input + divider + toolbar.
	trayHeight = trayBorder + inputHeight + dividerHeight + barHeight
	happyFor   = 2 * time.Second

	// The device frame is capped so it reads as a handheld "pocket communicator"
	// rather than filling the whole terminal.
	maxFrameWidth  = 100
	maxFrameHeight = 40
)

type transcriptItem struct {
	role Role
	text string
}

type screenKind int

const (
	screenSetupHarness screenKind = iota
	screenSetupPersonality
	screenMessages
	screenSettings
	screenNewChat
	screenContext
	screenMemory
)

// sgrMouseBody matches the body of an SGR mouse report (e.g. "[<65;51;20M").
// When a rapid scroll flood splits a mouse sequence across terminal reads, the
// leading ESC is consumed on its own and the remaining body arrives as plain
// runes — which must never be typed into the input.
var sgrMouseBody = regexp.MustCompile(`\x1b?\[?<?\d+;\d+;\d+[Mm]`)

// mouseFragment matches the orphaned head of a split mouse sequence — a lone
// "[", "[<", "<65;51;2", etc. These chars are also legitimately typable, so
// they're only treated as noise when they arrive amid a scroll burst.
var mouseFragment = regexp.MustCompile(`^[\[<][\[<;0-9Mm]*$`)

// scrollNoiseWindow is how long after scroll activity a bracket fragment is
// still assumed to be mouse-report debris rather than real typing.
const scrollNoiseWindow = 150 * time.Millisecond

// isMouseNoise reports whether a key event is leaked mouse-report debris that
// must be dropped instead of corrupting the textarea. Full report bodies are
// always noise; bare bracket fragments only count as noise during a scroll
// burst, so a deliberately typed "[" or "<" survives.
func (m Model) isMouseNoise(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes {
		return false
	}
	s := string(msg.Runes)
	if s == "" {
		return false
	}
	if strings.TrimSpace(sgrMouseBody.ReplaceAllString(s, "")) == "" {
		return true
	}
	return mouseFragment.MatchString(s) && time.Since(m.lastScroll) < scrollNoiseWindow
}

// personaEventMsg wraps a persona event for the update loop.
type personaEventMsg struct{ ev claudeproc.Event }

// PersonaEventMsg wraps a persona event as a Bubble Tea message deliverable via
// Program.Send.
func PersonaEventMsg(ev claudeproc.Event) tea.Msg { return personaEventMsg{ev: ev} }

type avatarTickMsg struct{}

func avatarTick() tea.Cmd {
	return tea.Tick(450*time.Millisecond, func(time.Time) tea.Msg { return avatarTickMsg{} })
}

// Model is the Bubble Tea model for io's kawaii chat TUI.
type Model struct {
	app      AppController
	messages messageComponent
	setup    setupFlow
	active   controlScreen // nil unless a control screen is open

	expr       Expression
	frame      int
	working    bool
	happyTill  time.Time
	lastScroll time.Time // last mouse/scroll activity, for mouse-noise gating

	ctx ContextInfo

	// Layout geometry (recomputed on resize).
	termW, termH         int
	interiorW, interiorH int
	screenW, screenH     int
	contentLeft          int // absolute x of the frame interior (for mouse hit-test)
	barX                 int // absolute x of the toolbar's left edge (inside the tray)
	barY                 int // absolute y of the button bar row
	ready                bool
	screen               screenKind
}

// New constructs a Model wired to an AppController.
func New(app AppController) Model {
	m := Model{
		app:      app,
		messages: newMessageComponent(app.History()),
		setup:    newSetupFlow(app.Settings()),
		expr:     Resting,
		screen:   screenMessages,
	}
	if app.NeedsSetup() {
		m.screen = screenSetupHarness
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return tea.Batch(m.messages.Init(), avatarTick()) }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.layout(msg.Width, msg.Height)
		m.refreshMessages()
		return m, nil

	case avatarTickMsg:
		m.frame++
		m.tickExpression()
		return m, avatarTick()

	case personaEventMsg:
		return m.handleEvent(msg.ev), nil

	case newChatMsg:
		_ = m.app.NewSession()
		m.messages.Clear()
		m.working = false
		m.expr = Resting
		m.screen = screenMessages
		m.active = nil
		return m, nil
	}

	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	if m.screen != screenMessages {
		if m.screen == screenSetupHarness || m.screen == screenSetupPersonality {
			return m.updateSetup(msg)
		}
		return m.updateControlScreen(msg)
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		// Note the activity so leaked report fragments can be filtered (#4).
		m.lastScroll = time.Now()
		// Left-click on the button bar opens a communicator screen.
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && msg.Y == m.barY {
			if action, ok := m.messages.BarHitTest(msg.X - m.barX); ok {
				m.openScreen(action)
			}
			return m, nil
		}
		// Wheel scrolls the history.
		return m, m.messages.UpdateViewport(msg)

	case tea.KeyMsg:
		// Drop mouse-report fragments that leaked in as text (see isMouseNoise).
		if m.isMouseNoise(msg) {
			return m, nil
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			return m.send(), nil
		case tea.KeyUp:
			if m.messages.TryRecall(-1) {
				return m, nil
			}
		case tea.KeyDown:
			if m.messages.TryRecall(1) {
				return m, nil
			}
		}
		if action, ok := m.messages.HotkeyAction(msg.String()); ok {
			m.openScreen(action)
			return m, nil
		}
		// Scroll the history with page keys (leaving arrows for the input).
		switch msg.String() {
		case "pgup", "pgdown", "ctrl+u", "ctrl+d":
			return m, m.messages.UpdateViewport(msg)
		}
	}

	return m, m.messages.UpdateInput(msg)
}

func (m Model) updateControlScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.active == nil {
		m.screen = screenMessages
		return m, nil
	}
	next, cmd := m.active.Update(msg)
	m.active = next
	if m.active == nil {
		m.screen = screenMessages
	}
	return m, cmd
}

func (m Model) updateSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.screen == screenSetupPersonality {
			m.screen = screenSetupHarness
		}
	case "up", "left":
		if m.screen == screenSetupPersonality {
			m.setup.cyclePersona(-1)
		} else {
			m.setup.cycleHarness(-1)
		}
	case "down", "right", "tab":
		if m.screen == screenSetupPersonality {
			m.setup.cyclePersona(1)
		} else {
			m.setup.cycleHarness(1)
		}
	case "enter":
		if m.screen == screenSetupHarness {
			m.screen = screenSetupPersonality
			m.setup.lastError = ""
			return m, nil
		}
		if err := m.app.CompleteSetup(m.setup.harness, m.setup.choice()); err != nil {
			m.setup.lastError = err.Error()
			return m, nil
		}
		m.screen = screenMessages
		m.setup.lastError = ""
	}
	return m, nil
}

func (m *Model) layout(w, h int) {
	m.termW, m.termH = w, h

	frameW := w
	if frameW > maxFrameWidth {
		frameW = maxFrameWidth
	}
	// The antenna sits above the frame, so the frame itself must leave room for
	// it within the terminal height.
	frameH := h - antennaHeight
	if frameH > maxFrameHeight {
		frameH = maxFrameHeight
	}

	// Interior excludes the rounded border (2 cols / 2 rows) and 1-col padding.
	interiorW := frameW - 4
	if interiorW < 10 {
		interiorW = 10
	}
	interiorH := frameH - 2
	minInteriorH := grilleHeight + screenBorder + screenChromeHeight + trayHeight + 1
	if interiorH < minInteriorH {
		interiorH = minInteriorH
	}
	m.interiorW, m.interiorH = interiorW, interiorH

	screenW := interiorW - 4 // screen border (2) + horizontal padding (2)
	if screenW < 1 {
		screenW = 1
	}
	screenH := interiorH - grilleHeight
	if screenH < screenBorder+screenChromeHeight+trayHeight+1 {
		screenH = screenBorder + screenChromeHeight + trayHeight + 1
	}
	m.screenW, m.screenH = screenW, screenH

	bodyH := screenH - screenBorder - screenChromeHeight
	if bodyH < trayHeight+1 {
		bodyH = trayHeight + 1
	}
	m.messages.Layout(screenW, bodyH)
	if m.active != nil {
		m.active.Layout(screenW, bodyH)
	}
	m.ready = true

	leftMargin := (w - frameW) / 2
	if leftMargin < 0 {
		leftMargin = 0
	}
	// The whole device is the antenna stacked on top of the frame.
	deviceH := antennaHeight + frameH
	topMargin := (h - deviceH) / 2
	if topMargin < 0 {
		topMargin = 0
	}
	// Frame interior starts after the centering margin + border + left padding.
	m.contentLeft = leftMargin + 2
	// The toolbar is inset by the screen shell and the compose tray border +
	// padding (2 cols each).
	m.barX = m.contentLeft + 4
	// The toolbar sits below the antenna, top border, hardware band, screen
	// border, status readout, message viewport, and the tray's rows.
	const screenTopBorder = 1
	const trayTopBorder = 1
	m.barY = topMargin + antennaHeight + 1 + grilleHeight + screenTopBorder + screenChromeHeight + m.messages.ViewportHeight() + trayTopBorder + inputHeight + dividerHeight
}

func (m *Model) tickExpression() {
	switch {
	case m.working:
		m.expr = Working
	case time.Now().Before(m.happyTill):
		m.expr = Happy
	case m.frame%8 == 0:
		m.expr = Blink
	default:
		m.expr = Resting
	}
}

func (m Model) handleEvent(ev claudeproc.Event) Model {
	switch ev.Kind {
	case claudeproc.KindAssistantText:
		m.messages.AppendAssistant(ev.Text)
	case claudeproc.KindResult:
		m.working = false
		m.expr = Happy
		m.happyTill = time.Now().Add(happyFor)
		m.ctx = m.app.ContextInfo()
	}
	return m
}

func (m Model) send() Model {
	if m.working {
		return m
	}
	text := strings.TrimSpace(m.messages.Value())
	if text == "" {
		return m
	}
	m.messages.AppendUser(text)
	_ = m.app.Send(text)
	m.messages.ResetInput()
	m.working = true
	m.expr = Working
	m.messages.GotoBottom() // sending always returns you to the live conversation
	return m
}

func (m *Model) openScreen(action Action) {
	switch action {
	case ActionSettings:
		m.screen = screenSettings
		m.active = newSettingsScreen(m.app)
	case ActionContext:
		m.screen = screenContext
		m.active = newContextScreen(m.app.ContextInfo())
	case ActionMemory:
		text, _ := m.app.MemorySummary()
		m.screen = screenMemory
		m.active = newMemoryScreen(text)
	case ActionNewChat:
		m.screen = screenNewChat
		m.active = newConfirmScreen("NEW CHAT", "Start a fresh chat?\nio keeps its memories. ♡", newChatMsg{})
	}
	if m.active != nil && m.ready {
		m.active.Layout(m.screenW, m.screenBodyHeight())
	}
}

func (m *Model) refreshMessages() { m.messages.Refresh() }

// activeScreen renders the replaceable body inside the screen shell. Additional
// screens should branch here while leaving the device and screen chrome intact.
func (m Model) activeScreen() string {
	switch m.screen {
	case screenSetupHarness, screenSetupPersonality:
		return m.setup.View(m.screen, m.screenW, m.screenBodyHeight(), m.frame)
	case screenSettings, screenNewChat, screenContext, screenMemory:
		if m.active != nil {
			return m.active.View(m.screenW, m.screenBodyHeight(), m.frame)
		}
		return ""
	case screenMessages:
		fallthrough
	default:
		return m.messages.View()
	}
}

// screenShell renders the inner LCD surface: status readout plus the active
// screen body, wrapped in its own rounded boundary separate from the device.
func (m Model) screenShell() string {
	innerW := m.screenW
	if innerW < 1 {
		innerW = 1
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		m.screenHeader(),
		m.activeScreen(),
	)
	return screenShellStyle.Width(innerW + 2).Render(body)
}

func (m Model) screenBodyHeight() int {
	h := m.screenH - screenBorder - screenChromeHeight
	if h < 1 {
		return 1
	}
	return h
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "starting io…"
	}
	inner := lipgloss.JoinVertical(lipgloss.Left,
		speakerGrille(m.interiorW),
		m.screenShell(),
	)
	// The frame draws its own top border via deviceTop (so the antenna can join
	// it), so the lipgloss border here omits the top row. Width is interiorW+2
	// to leave room for the style's 1-col horizontal padding, so content sized
	// to interiorW fits without wrapping (which would overflow the terminal).
	framed := deviceFrameStyle.
		BorderTop(false).
		Width(m.interiorW + 2).
		Height(m.interiorH).
		Render(inner)
	frameW := m.interiorW + 4
	device := deviceTop(frameW) + "\n" + framed
	base := lipgloss.Place(m.termW, m.termH, lipgloss.Center, lipgloss.Center, device)

	return base
}
