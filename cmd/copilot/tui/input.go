package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles keyboard input and other messages for the TUI model.
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

		case tea.KeyLeft:
			return m.handleCursorLeft()

		case tea.KeyRight:
			return m.handleCursorRight()

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

	case StreamTokenMsg:
		// Append the token to the last assistant message in history.
		return m.appendToLastAssistantMessage(msg.Token), nil

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
			model := m.models[m.modelIndex]
			m.viewMode = ViewModeChat
			return m, func() tea.Msg {
				return SetModelRequestMsg{ModelID: model.ID}
			}
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

			// Clear input and reset cursor to start.
			input := m.input
			m.input = ""
			m.cursor = 0

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
// In chat mode it deletes the rune immediately to the left of the cursor.
func (m Model) handleBackspace() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeCommandPalette:
		if len(m.cmdFilter) > 0 {
			m.cmdFilter = m.cmdFilter[:len(m.cmdFilter)-1]
		}
	case ViewModeChat:
		if m.cursor > 0 && len(m.input) > 0 {
			// Find the start of the rune that ends at m.cursor.
			runeStart := m.cursor - 1
			for runeStart > 0 && isRuneContinuation(m.input[runeStart]) {
				runeStart--
			}
			m.input = m.input[:runeStart] + m.input[m.cursor:]
			m.cursor = runeStart
		}
	}
	return m, nil
}

// isRuneContinuation reports whether b is a UTF-8 continuation byte (10xxxxxx).
func isRuneContinuation(b byte) bool {
	return b&0xC0 == 0x80
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

// handleRunes processes character input, inserting at the cursor position.
func (m Model) handleRunes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	char := string(msg.Runes)

	switch m.viewMode {
	case ViewModeCommandPalette:
		m.cmdFilter += char
		m.cmdIndex = 0 // reset selection on filter change
	case ViewModeChat:
		if !m.streaming {
			// Insert the new rune(s) at the cursor position.
			m.input = m.input[:m.cursor] + char + m.input[m.cursor:]
			m.cursor += len(char)
		}
	}
	return m, nil
}

// handleCursorLeft moves the cursor one rune to the left (clamped to 0).
func (m Model) handleCursorLeft() (tea.Model, tea.Cmd) {
	if m.viewMode == ViewModeChat && m.cursor > 0 {
		// Step back past any UTF-8 continuation bytes to land on the rune start.
		m.cursor--
		for m.cursor > 0 && isRuneContinuation(m.input[m.cursor]) {
			m.cursor--
		}
	}
	return m, nil
}

// handleCursorRight moves the cursor one rune to the right (clamped to len(input)).
func (m Model) handleCursorRight() (tea.Model, tea.Cmd) {
	if m.viewMode == ViewModeChat && m.cursor < len(m.input) {
		// Advance past the current rune (which may be multi-byte).
		m.cursor++
		for m.cursor < len(m.input) && isRuneContinuation(m.input[m.cursor]) {
			m.cursor++
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

// StreamChunkMsg carries a single streaming content token from the LLM provider.
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
