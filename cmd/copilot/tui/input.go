package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// KeyMsg handles keyboard input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEscape:
			if m.viewMode != ViewModeChat {
				m.viewMode = ViewModeChat
				m.cmdFilter = ""
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyCtrlP:
			if m.viewMode == ViewModeChat {
				m.viewMode = ViewModeCommandPalette
				m.cmdFilter = ""
				m.cmdIndex = 0
			}
			return m, nil

		case tea.KeyCtrlL:
			if m.viewMode == ViewModeChat {
				m = m.ClearMessages()
			}
			return m, nil

		case tea.KeyEnter:
			return m.handleEnter()

		case tea.KeyBackspace:
			return m.handleBackspace()

		case tea.KeyUp:
			return m.handleUp()

		case tea.KeyDown:
			return m.handleDown()

		case tea.KeyLeft, tea.KeyRight:
			// TODO: cursor movement within input
			return m, nil

		case tea.KeyRunes:
			return m.handleRunes(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	// Custom messages
	case StreamChunkMsg:
		return m.handleStreamChunk(msg)

	case StreamDoneMsg:
		return m.handleStreamDone()

	case ErrorMsg:
		m.err = msg.Err
		m.streaming = false
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// handleEnter processes Enter key.
func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeCommandPalette:
		filtered := m.filterCommands()
		if len(filtered) > 0 && m.cmdIndex < len(filtered) {
			cmd := filtered[m.cmdIndex]
			m.viewMode = ViewModeChat
			m.cmdFilter = ""
			newM, c := cmd.Action(m)
			return newM, c
		}

	case ViewModeModelSelect:
		if len(m.models) > 0 && m.modelIndex < len(m.models) {
			// TODO: emit model selection
			m.viewMode = ViewModeChat
		}

	case ViewModeHelp:
		m.viewMode = ViewModeChat

	case ViewModeChat:
		if m.input != "" && !m.streaming {
			// Add user message
			m = m.AddMessage("user", m.input)

			// Store in history
			m.history = append(m.history, m.input)
			m.histIndex = -1

			// Clear input
			input := m.input
			m.input = ""

			// Start streaming
			m.streaming = true
			m.messages[len(m.messages)-1].Streaming = true
			m = m.AddMessage("assistant", "")

			return m, SendStreamCmd(input)
		}
	}

	return m, nil
}

// handleBackspace processes Backspace key.
func (m Model) handleBackspace() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeCommandPalette:
		if len(m.cmdFilter) > 0 {
			m.cmdFilter = m.cmdFilter[:len(m.cmdFilter)-1]
		}
	case ViewModeChat:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	}
	return m, nil
}

// handleUp processes Up arrow.
func (m Model) handleUp() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeCommandPalette:
		if m.cmdIndex > 0 {
			m.cmdIndex--
		}
	case ViewModeModelSelect:
		if m.modelIndex > 0 {
			m.modelIndex--
		}
	case ViewModeChat:
		// Navigate history
		if len(m.history) > 0 {
			if m.histIndex < 0 {
				m.histIndex = len(m.history) - 1
			} else if m.histIndex > 0 {
				m.histIndex--
			}
			if m.histIndex >= 0 && m.histIndex < len(m.history) {
				m.input = m.history[m.histIndex]
			}
		}
	}
	return m, nil
}

// handleDown processes Down arrow.
func (m Model) handleDown() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeCommandPalette:
		filtered := m.filterCommands()
		if m.cmdIndex < len(filtered)-1 {
			m.cmdIndex++
		}
	case ViewModeModelSelect:
		if m.modelIndex < len(m.models)-1 {
			m.modelIndex++
		}
	case ViewModeChat:
		// Navigate history
		if m.histIndex >= 0 && m.histIndex < len(m.history)-1 {
			m.histIndex++
			m.input = m.history[m.histIndex]
		} else {
			m.histIndex = -1
			m.input = ""
		}
	}
	return m, nil
}

// handleRunes processes character input.
func (m Model) handleRunes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	char := string(msg.Runes)

	switch m.viewMode {
	case ViewModeCommandPalette:
		m.cmdFilter += char
		m.cmdIndex = 0 // reset selection on filter change
	case ViewModeChat:
		if !m.streaming {
			m.input += char
		}
	}
	return m, nil
}

// handleStreamChunk appends streaming content.
func (m Model) handleStreamChunk(msg StreamChunkMsg) (tea.Model, tea.Cmd) {
	if msg.Content != "" {
		m = m.AppendToLastMessage(msg.Content)
	}
	return m, nil
}

// handleStreamDone marks streaming complete.
func (m Model) handleStreamDone() (tea.Model, tea.Cmd) {
	m.streaming = false
	if len(m.messages) > 0 {
		m.messages[len(m.messages)-1].Streaming = false
	}
	return m, nil
}

// Custom messages for streaming
type StreamChunkMsg struct {
	Content string
}

type StreamDoneMsg struct{}

type ErrorMsg struct {
	Err error
}

// SendStreamCmd creates a command to start streaming.
func SendStreamCmd(input string) tea.Cmd {
	return func() tea.Msg {
		// This is a placeholder - actual streaming handled by copilot integration
		return StreamChunkMsg{Content: "Streaming not connected. Use /connect first."}
	}
}

// Helper to check if input starts with /
func isCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}

// ParseCommand extracts command name and args.
func ParseCommand(input string) (cmd string, args []string) {
	parts := strings.Fields(strings.TrimSpace(input))
	if len(parts) == 0 {
		return "", nil
	}
	cmd = parts[0]
	if len(parts) > 1 {
		args = parts[1:]
	}
	return
}
