// Package tui provides the terminal user interface for copilot.
package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ViewMode represents different UI states.
type ViewMode int

const (
	ViewModeChat ViewMode = iota
	ViewModeCommandPalette
	ViewModeModelSelect
	ViewModeHelp
)

// ChatMessage represents a single message in chat history.
type ChatMessage struct {
	Role      string    // "user", "assistant", "system"
	Content   string    // message content
	Timestamp time.Time // when message was sent
	Streaming bool      // currently streaming content
}

// Model is the main TUI state.
type Model struct {
	// Connection state
	connected bool
	Provider  string
	modelName string

	// Chat state
	messages  []ChatMessage
	input     string
	history   []string // command history
	histIndex int      // current position in history

	// UI state
	viewMode      ViewMode
	width         int
	height        int
	scrollPos     int // scroll position in chat
	ready         bool
	err           error
	streaming     bool // currently streaming LLM response
	workerRunning bool
	headlessMode  bool

	// Command palette
	cmdFilter string
	cmdIndex  int
	commands  []Command

	// Model selection
	models     []ModelInfo
	modelIndex int
}

// ModelInfo represents an available LLM model.
type ModelInfo struct {
	ID          string
	Name        string
	ContextSize int
}

// Command represents a slash command.
type Command struct {
	Name        string
	Description string
	Action      func(m Model) (Model, tea.Cmd)
}

// InitialModel creates a new TUI model.
func InitialModel() Model {
	commands := []Command{
		{Name: "/connect", Description: "Connect to LLM provider"},
		{Name: "/models", Description: "List available models"},
		{Name: "/set-model", Description: "Switch model"},
		{Name: "/status", Description: "Show status"},
		{Name: "/clear", Description: "Clear chat history"},
		{Name: "/help", Description: "Show help"},
		{Name: "/exit", Description: "Exit copilot"},
		{Name: "/worker-start", Description: "Start background task worker"},
		{Name: "/worker-stop", Description: "Stop background task worker"},
		{Name: "/worker-status", Description: "Show worker status"},
		{Name: "/headless", Description: "Switch to headless mode (no browser window)"},
		{Name: "/no-headless", Description: "Switch to visible browser mode"},
		{Name: "/run-task", Description: "Run a task immediately: /run-task <url> [name]"},
	}

	return Model{
		messages:  []ChatMessage{},
		input:     "",
		history:   []string{},
		histIndex: -1,
		viewMode:  ViewModeChat,
		commands:  commands,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// SetWorkerRunning updates the background worker running state.
func (m Model) SetWorkerRunning(running bool) Model {
	m.workerRunning = running
	return m
}

// SetHeadlessMode updates the current headless/visible-browser state in the model.
func (m Model) SetHeadlessMode(headless bool) Model {
	m.headlessMode = headless
	return m
}

// SetConnected updates connection state.
func (m Model) SetConnected(connected bool, provider, modelName string) Model {
	m.connected = connected
	m.Provider = provider
	m.modelName = modelName
	return m
}

// AddMessage appends a message to chat history.
func (m Model) AddMessage(role, content string) Model {
	m.messages = append(m.messages, ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	return m
}

// AppendToLastMessage adds content to the last message (for streaming).
func (m Model) AppendToLastMessage(content string) Model {
	if len(m.messages) == 0 {
		return m
	}
	m.messages[len(m.messages)-1].Content += content
	return m
}

// ClearMessages clears chat history.
func (m Model) ClearMessages() Model {
	m.messages = []ChatMessage{}
	m.scrollPos = 0
	return m
}

// filterCommands filters commands by the current filter.
func (m Model) filterCommands() []Command {
	if m.cmdFilter == "" {
		return m.commands
	}
	var filtered []Command
	for _, cmd := range m.commands {
		if strings.Contains(cmd.Name, m.cmdFilter) || strings.Contains(strings.ToLower(cmd.Description), strings.ToLower(m.cmdFilter)) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}
