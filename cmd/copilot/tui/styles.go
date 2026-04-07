package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	colorPrimary   = lipgloss.Color("#7C3AED") // purple
	colorSecondary = lipgloss.Color("#06B6D4") // cyan
	colorSuccess   = lipgloss.Color("#10B981") // green
	colorWarning   = lipgloss.Color("#F59E0B") // amber
	colorError     = lipgloss.Color("#EF4444") // red
	colorMuted     = lipgloss.Color("#6B7280") // gray
	colorBackground = lipgloss.Color("#1F2937") // dark gray
)

// Header styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 1).
			MarginRight(1)

	connectedStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Padding(0, 1)

	disconnectedStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 1)
)

// Chat styles
var (
	chatPanelStyle = lipgloss.NewStyle().
			Padding(0, 1).
			MarginTop(1)

	userPrefixStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	userContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB"))

	assistantPrefixStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSecondary)

	assistantContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F3F4F6"))

	systemPrefixStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorMuted)

	systemContentStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)
)

// Input styles
var (
	inputPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Blink(true)
)

// Status bar styles
var (
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	hintStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

// Command palette styles
var (
	paletteStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Background(lipgloss.Color("#111827"))

	paletteTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary).
				MarginBottom(1)

	paletteHintStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				MarginTop(1)

	commandItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D1D5DB")).
				Padding(0, 1)

	commandSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorPrimary).
				Bold(true).
				Padding(0, 1)
)

// Model select styles
var (
	modelItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB")).
			Padding(0, 1)

	modelSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorPrimary).
				Bold(true).
				Padding(0, 1)
)

// Help styles
var (
	helpBoxStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Background(lipgloss.Color("#111827"))

	helpTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary).
				MarginBottom(1)

	helpContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D1D5DB"))

	helpHintStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				MarginTop(1)
)

// Error styles
var (
	errorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)
)
