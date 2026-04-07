package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// CommandActions maps command names to their actions.
func GetCommandActions() map[string]func(m Model, args []string) (Model, tea.Cmd) {
	return map[string]func(m Model, args []string) (Model, tea.Cmd){
		"/connect":       actionConnect,
		"/models":        actionModels,
		"/set-model":     actionSetModel,
		"/status":        actionStatus,
		"/clear":         actionClear,
		"/help":          actionHelp,
		"/exit":          actionExit,
		"/worker-start":  actionWorkerStart,
		"/worker-stop":   actionWorkerStop,
		"/worker-status": actionWorkerStatus,
		"/headless":      actionHeadless,
		"/no-headless":   actionNoHeadless,
		"/run-task":      actionRunTask,
	}
}

// actionConnect handles /connect command.
func actionConnect(m Model, args []string) (Model, tea.Cmd) {
	if len(args) < 2 {
		m = m.AddMessage("system", "Usage: /connect <provider> <api-key> [base-url] [model]")
		return m, nil
	}

	provider := args[0]
	apiKey := args[1]
	baseURL := ""
	modelName := ""

	if len(args) > 2 {
		baseURL = args[2]
	}
	if len(args) > 3 {
		modelName = args[3]
	}

	// Emit connection request - handled by main program
	return m, func() tea.Msg {
		return ConnectRequestMsg{
			Provider: provider,
			APIKey:   apiKey,
			BaseURL:  baseURL,
			Model:    modelName,
		}
	}
}

// actionModels handles /models command.
func actionModels(m Model, args []string) (Model, tea.Cmd) {
	if !m.connected {
		m = m.AddMessage("system", "Not connected. Use /connect first.")
		return m, nil
	}

	return m, func() tea.Msg {
		return ListModelsRequestMsg{}
	}
}

// actionSetModel handles /set-model command.
func actionSetModel(m Model, args []string) (Model, tea.Cmd) {
	if len(args) < 1 {
		m = m.AddMessage("system", "Usage: /set-model <model-id>")
		return m, nil
	}

	if !m.connected {
		m = m.AddMessage("system", "Not connected. Use /connect first.")
		return m, nil
	}

	return m, func() tea.Msg {
		return SetModelRequestMsg{ModelID: args[0]}
	}
}

// actionStatus handles /status command.
func actionStatus(m Model, args []string) (Model, tea.Cmd) {
	if m.connected {
		m = m.AddMessage("system", "Connected to "+m.Provider+"/"+m.modelName)
	} else {
		m = m.AddMessage("system", "Not connected. Use /connect <provider> <api-key>")
	}
	return m, nil
}

// actionClear handles /clear command.
func actionClear(m Model, args []string) (Model, tea.Cmd) {
	m = m.ClearMessages()
	m = m.AddMessage("system", "Chat cleared.")
	return m, nil
}

// actionHelp handles /help command.
func actionHelp(m Model, args []string) (Model, tea.Cmd) {
	m.viewMode = ViewModeHelp
	return m, nil
}

// actionExit handles /exit command.
func actionExit(m Model, args []string) (Model, tea.Cmd) {
	return m, tea.Quit
}

func actionWorkerStart(m Model, args []string) (Model, tea.Cmd) {
	return m, func() tea.Msg { return WorkerStartRequestMsg{} }
}

func actionWorkerStop(m Model, args []string) (Model, tea.Cmd) {
	return m, func() tea.Msg { return WorkerStopRequestMsg{} }
}

func actionWorkerStatus(m Model, args []string) (Model, tea.Cmd) {
	return m, func() tea.Msg { return WorkerStatusRequestMsg{} }
}

// actionHeadless toggles headless mode. Args: "on"|"off" (default toggles to "on").
func actionHeadless(m Model, args []string) (Model, tea.Cmd) {
	headless := true // default: switch to headless
	if len(args) > 0 {
		switch args[0] {
		case "off", "false", "no":
			headless = false
		}
	}
	return m, func() tea.Msg { return SetHeadlessModeMsg{Headless: headless} }
}

// actionNoHeadless switches to visible-browser mode.
func actionNoHeadless(m Model, args []string) (Model, tea.Cmd) {
	return m, func() tea.Msg { return SetHeadlessModeMsg{Headless: false} }
}

// actionRunTask creates and immediately runs a task.
// Usage: /run-task <url> [name]
func actionRunTask(m Model, args []string) (Model, tea.Cmd) {
	if len(args) < 1 {
		m = m.AddMessage("system", "Usage: /run-task <url> [name]")
		return m, nil
	}
	url := args[0]
	name := "quick-task"
	if len(args) > 1 {
		name = strings.Join(args[1:], " ")
	}
	return m, func() tea.Msg {
		return RunTaskRequestMsg{
			Name:     name,
			URL:      url,
			Headless: m.headlessMode,
		}
	}
}

// Request messages for copilot integration
type ConnectRequestMsg struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
}

type ListModelsRequestMsg struct{}

type SetModelRequestMsg struct {
	ModelID string
}

type ChatRequestMsg struct {
	Input string
}

type WorkerStartRequestMsg struct{}
type WorkerStopRequestMsg struct{}
type WorkerStatusRequestMsg struct{}

// SetHeadlessModeMsg switches the global headless/visible-browser mode.
type SetHeadlessModeMsg struct {
	Headless bool
}

// RunTaskRequestMsg asks the copilot to create and run a single task immediately.
type RunTaskRequestMsg struct {
	Name     string
	URL      string
	Headless bool
}

// ProcessSlashCommand handles a slash command from input.
func (m Model) ProcessSlashCommand(input string) (Model, tea.Cmd) {
	cmd, args := ParseCommand(input)
	actions := GetCommandActions()

	action, ok := actions[cmd]
	if !ok {
		m = m.AddMessage("system", "Unknown command: "+cmd+". Type /help for available commands.")
		return m, nil
	}

	return action(m, args)
}

// InitializeCommands returns initial commands for the palette.
func InitializeCommands() []Command {
	return []Command{
		{Name: "/connect", Description: "Connect to LLM provider"},
		{Name: "/models", Description: "List available models"},
		{Name: "/set-model", Description: "Switch model"},
		{Name: "/status", Description: "Show connection status"},
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
}
