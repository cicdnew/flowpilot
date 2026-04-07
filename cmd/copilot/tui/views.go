package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the entire TUI.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	switch m.viewMode {
	case ViewModeCommandPalette:
		return m.viewCommandPalette()
	case ViewModeModelSelect:
		return m.viewModelSelect()
	case ViewModeHelp:
		return m.viewHelp()
	default:
		return m.viewChat()
	}
}

// viewChat renders the main chat view.
func (m Model) viewChat() string {
	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Chat panel (scrollable)
	chatPanel := m.renderChatPanel()
	b.WriteString(chatPanel)
	b.WriteString("\n")

	// Input area
	inputArea := m.renderInput()
	b.WriteString(inputArea)
	b.WriteString("\n")

	// Status bar
	statusBar := m.renderStatusBar()
	b.WriteString(statusBar)

	return b.String()
}

// renderHeader renders the top header bar.
func (m Model) renderHeader() string {
	title := titleStyle.Render(" FlowPilot Copilot ")

	var status string
	if m.connected {
		status = connectedStyle.Render(" ● " + m.Provider + "/" + m.modelName + " ")
	} else {
		status = disconnectedStyle.Render(" ○ Not Connected ")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, title, " ", status)
}

// renderChatPanel renders the scrollable chat history.
func (m Model) renderChatPanel() string {
	chatHeight := max(5, m.height-8) // reserve space for header, input, status

	var lines []string

	// Calculate visible messages based on scroll
	startIdx := 0
	if len(m.messages) > chatHeight {
		startIdx = len(m.messages) - chatHeight
		if m.scrollPos > 0 {
			startIdx = max(0, startIdx-m.scrollPos)
		}
	}

	for i := startIdx; i < len(m.messages); i++ {
		msg := m.messages[i]
		lines = append(lines, m.renderMessage(msg))
	}

	// Pad to fill height
	for len(lines) < chatHeight {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return chatPanelStyle.Render(content)
}

// renderMessage renders a single chat message.
func (m Model) renderMessage(msg ChatMessage) string {
	var prefix string
	var content string

	switch msg.Role {
	case "user":
		prefix = userPrefixStyle.Render("You:")
		content = userContentStyle.Render(msg.Content)
	case "assistant":
		prefix = assistantPrefixStyle.Render("Copilot:")
		content = assistantContentStyle.Render(msg.Content)
		if msg.Streaming {
			content += cursorStyle.Render("█")
		}
	default:
		prefix = systemPrefixStyle.Render("System:")
		content = systemContentStyle.Render(msg.Content)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, prefix, " ", content)
}

// renderInput renders the input area.
func (m Model) renderInput() string {
	prompt := inputPromptStyle.Render("> ")
	inputText := inputStyle.Render(m.input)
	if m.streaming {
		inputText = inputStyle.Render("[streaming...]")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, prompt, inputText, cursorStyle.Render("█"))
}

// renderStatusBar renders the bottom status bar.
func (m Model) renderStatusBar() string {
	hints := []string{"/help", "/connect", "/exit"}
	worker := "worker:off"
	if m.workerRunning {
		worker = "worker:on"
	}
	browserMode := "browser:visible"
	if m.headlessMode {
		browserMode = "browser:headless"
	}
	hintText := hintStyle.Render(strings.Join(hints, " | ") + "   " + worker + "  " + browserMode)
	return statusBarStyle.Render(hintText)
}

// viewCommandPalette renders the command palette overlay.
func (m Model) viewCommandPalette() string {
	var b strings.Builder

	b.WriteString(paletteTitleStyle.Render(" Command Palette "))
	b.WriteString("\n\n")

	filtered := m.filterCommands()
	for i, cmd := range filtered {
		style := commandItemStyle
		if i == m.cmdIndex {
			style = commandSelectedStyle
		}
		b.WriteString(style.Render(cmd.Name + " - " + cmd.Description))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(paletteHintStyle.Render("Type to filter, Enter to select, Esc to cancel"))

	return paletteStyle.Render(b.String())
}

// viewModelSelect renders the model selection overlay.
func (m Model) viewModelSelect() string {
	var b strings.Builder

	b.WriteString(paletteTitleStyle.Render(" Select Model "))
	b.WriteString("\n\n")

	for i, model := range m.models {
		style := commandItemStyle
		if i == m.modelIndex {
			style = commandSelectedStyle
		}
		info := model.Name
		if model.ContextSize > 0 {
			info += " (" + formatContextSize(model.ContextSize) + ")"
		}
		b.WriteString(style.Render(info))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(paletteHintStyle.Render("↑↓ Navigate, Enter to select, Esc to cancel"))

	return paletteStyle.Render(b.String())
}

// viewHelp renders the help overlay.
func (m Model) viewHelp() string {
	var b strings.Builder

	b.WriteString(helpTitleStyle.Render(" FlowPilot Copilot Help "))
	b.WriteString("\n\n")

	helpContent := `Keyboard Shortcuts:
  Enter      Send message
  Ctrl+P     Open command palette
  Ctrl+L     Clear chat
  Ctrl+C     Exit
  ↑/↓        Navigate history
  Esc        Close overlay

Commands:
  /connect <provider> <api-key> [model]
  /models    List available models
  /set-model <model-id>
  /status    Show connection status
  /clear     Clear chat history
  /help      Show this help
  /exit      Exit copilot

Supported Providers:
  openai, openrouter, gemini, nvidia, huggingface, github, kilo
`
	b.WriteString(helpContentStyle.Render(helpContent))
	b.WriteString("\n")
	b.WriteString(helpHintStyle.Render("Press Esc to close"))

	return helpBoxStyle.Render(b.String())
}

// formatContextSize formats context size for display.
func formatContextSize(size int) string {
	if size >= 1000000 {
		return "1M+"
	}
	if size >= 1000 {
		return strings.TrimSuffix(strings.TrimSuffix(
			lipgloss.NewStyle().Render(""), "000"), "000") + "K"
	}
	return ""
}
