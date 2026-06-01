// Package tui renders io's kawaii chat interface using Bubble Tea.
package tui

import "github.com/charmbracelet/lipgloss"

// Pastel Dream palette. Single source of color truth for the whole UI.
const (
	colorMint     = lipgloss.Color("#A8E6CF") // io bubbles
	colorPink     = lipgloss.Color("#FFB3D9") // your bubbles
	colorLavender = lipgloss.Color("#C8A2E0") // accents, panels, buttons, screens
	colorCream    = lipgloss.Color("#FFF5E1") // primary text on the LCD surface
	colorDim      = lipgloss.Color("#9A8FB0") // labels, hints
	colorInk      = lipgloss.Color("#5B4B7A") // bevel shadow edge
)

var (
	// Chat bubbles.
	ioBubbleStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMint).
			Foreground(colorMint).
			Padding(0, 1)

	youBubbleStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPink).
			Foreground(colorPink).
			Padding(0, 1)

	// Avatar + speaker labels.
	avatarStyle = lipgloss.NewStyle().Foreground(colorMint).Bold(true)
	labelStyle  = lipgloss.NewStyle().Foreground(colorDim)

	// Breakout panel for code/tables.
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorLavender).
			Padding(0, 1)
	panelTitleStyle = lipgloss.NewStyle().Foreground(colorLavender).Bold(true)

	// Pocket-communicator device frame wrapping the whole UI.
	deviceFrameStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorLavender).
				Padding(0, 1)

	// Screen shell: the nested LCD surface whose body can swap between messages
	// and future screens while the outer communicator chrome stays stable.
	screenShellStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorInk).
				Padding(0, 1)

	// Compose tray: the bordered panel inside the screen that holds the text
	// input and button toolbar beneath the message history.
	composeTrayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorLavender).
				Padding(0, 1)
	composeDividerStyle = lipgloss.NewStyle().Foreground(colorInk)

	setupPageStyle        = lipgloss.NewStyle().Foreground(colorCream)
	setupRailStyle        = lipgloss.NewStyle().Foreground(colorInk)
	setupTitleStyle       = lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	setupStageStyle       = lipgloss.NewStyle().Foreground(colorLavender).Bold(true)
	setupOptionStyle      = lipgloss.NewStyle().Foreground(colorCream).Bold(true)
	setupSelectedStyle    = lipgloss.NewStyle().Foreground(colorInk).Background(colorMint).Bold(true)
	setupCursorStyle      = lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	setupHintStyle        = lipgloss.NewStyle().Foreground(colorDim)
	setupDescriptionStyle = lipgloss.NewStyle().Foreground(colorDim)
	setupErrorStyle       = lipgloss.NewStyle().Foreground(colorPink).Bold(true)

	// Beveled buttons (90s look): bright top/left, dark bottom/right. The chip
	// background is supplied per-button by renderButton; text stays dark ink for
	// contrast on every pastel fill.
	buttonStyle = lipgloss.NewStyle().
			Foreground(colorInk).
			Padding(0, 1).
			Bold(true)
	buttonHotkeyStyle = lipgloss.NewStyle().
				Foreground(colorInk).
				Bold(true)

	// 90s pocket-communicator chrome: antenna, speaker/control band, and the
	// LCD-style status readout that runs along the top of the screen.
	antennaLabelStyle  = lipgloss.NewStyle().Foreground(colorDim).Bold(true)
	antennaWaveStyle   = lipgloss.NewStyle().Foreground(colorMint).Bold(true)
	antennaMastStyle   = lipgloss.NewStyle().Foreground(colorLavender)
	grilleStyle        = lipgloss.NewStyle().Foreground(colorInk)
	hardwareScrewStyle = lipgloss.NewStyle().Foreground(colorLavender).Bold(true)
	hardwareBadgeStyle = lipgloss.NewStyle().Foreground(colorDim).Bold(true)
	hardwareLabelStyle = lipgloss.NewStyle().Foreground(colorDim)
	hardwareLedStyle   = lipgloss.NewStyle().Foreground(colorMint).Bold(true)
	ruleStyle          = lipgloss.NewStyle().Foreground(colorInk)

	statusNameStyle   = lipgloss.NewStyle().Foreground(colorLavender).Bold(true)
	statusModeStyle   = lipgloss.NewStyle().Foreground(colorCream).Bold(true)
	statusWordStyle   = lipgloss.NewStyle().Foreground(colorDim)
	statusMetricStyle = lipgloss.NewStyle().Foreground(colorMint).Bold(true)
	statusWarnStyle   = lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	ledOnlineStyle    = lipgloss.NewStyle().Foreground(colorMint).Bold(true)
	ledWorkingStyle   = lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	signalOnStyle     = lipgloss.NewStyle().Foreground(colorMint)
	signalOffStyle    = lipgloss.NewStyle().Foreground(colorInk)
	batteryStyle      = lipgloss.NewStyle().Foreground(colorDim)
	clockStyle        = lipgloss.NewStyle().Foreground(colorDim)

	sideButtonFrameStyle  = lipgloss.NewStyle().Foreground(colorLavender).Bold(true)
	sideButtonFaceStyle   = lipgloss.NewStyle().Foreground(colorInk).Background(colorPink).Bold(true)
	sideButtonShadowStyle = lipgloss.NewStyle().
				Foreground(colorInk)

	takeoverFrameStyle  = lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	takeoverTitleStyle  = lipgloss.NewStyle().Foreground(colorMint).Bold(true)
	takeoverBodyStyle   = lipgloss.NewStyle().Foreground(colorCream)
	takeoverSignalStyle = lipgloss.NewStyle().Foreground(colorLavender).Bold(true)
)
