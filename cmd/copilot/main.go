package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"flowpilot/cmd/copilot/tui"
	"flowpilot/internal/copilot"
)

var version = "dev"

// Global copilot instance and root context shared across TUI handlers.
var c *copilot.CopilotFlow
var ctx context.Context

func main() {
	dataDir := flag.String("data-dir", "", "data directory (default ~/.flowpilot)")
	concurrency := flag.Int("concurrency", 10, "max concurrent browser tasks")
	provider := flag.String("provider", "", "LLM provider (openai, openrouter, gemini, nvidia, huggingface, github, kilo)")
	apiKey := flag.String("api-key", "", "API key for the LLM provider")
	baseURL := flag.String("base-url", "", "Custom base URL for LLM provider")
	model := flag.String("model", "", "Model name to use")
	poll := flag.Duration("poll", 30*time.Second, "interval between pending-task polls")
	healthInterval := flag.Int("health-interval", 300, "proxy health check interval in seconds")
	maxFailures := flag.Int("max-failures", 3, "max proxy failures before marking unhealthy")
	autoPoll := flag.Bool("auto-poll", false, "automatically start background worker on launch")
	headless := flag.Bool("headless", false, "force all tasks into headless mode (default: false = visible browser)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("flowpilot-copilot", version)
		os.Exit(0)
	}

	cfg := copilot.Config{
		DataDir:             *dataDir,
		MaxConcurrency:      *concurrency,
		ModelProvider:       *provider,
		APIKey:              *apiKey,
		BaseURL:             *baseURL,
		ModelName:           *model,
		PollInterval:        *poll,
		HealthCheckInterval: *healthInterval,
		MaxProxyFailures:    *maxFailures,
		Headless:            *headless,
	}

	var err error
	c, err = copilot.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create copilot: %v", err)
	}
	defer c.Stop()

	// Handle OS signals.
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Auto-start background worker if requested.
	if *autoPoll {
		c.StartPolling(ctx)
		log.Printf("[copilot] background worker auto-started (poll: %v)", *poll)
	}

	// Build initial TUI model.
	initialModel := tui.InitialModel()

	if *provider != "" && *apiKey != "" {
		initialModel = initialModel.SetConnected(true, *provider, *model)
	}
	if *autoPoll {
		initialModel = initialModel.SetWorkerRunning(true)
	}

	// Wrap in CopilotModel to intercept copilot-specific messages.
	wrapped := CopilotModel{Model: initialModel}

	p := tea.NewProgram(
		wrapped,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, err := p.Run()
	if err != nil {
		log.Fatalf("TUI error: %v", err)
	}
	_ = finalModel
}

// CopilotModel wraps tui.Model and intercepts copilot-specific messages before
// delegating everything else to the base Bubble Tea model.
type CopilotModel struct {
	tui.Model
}

// Init satisfies tea.Model (delegated to the embedded model).
func (m CopilotModel) Init() tea.Cmd {
	return m.Model.Init()
}

// Update handles copilot-specific messages and delegates the rest to tui.Model.
func (m CopilotModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── LLM provider ────────────────────────────────────────────────────────
	case tui.ConnectRequestMsg:
		err := c.Connect(msg.Provider, msg.APIKey, msg.BaseURL, msg.Model)
		if err != nil {
			return m, func() tea.Msg { return tui.ErrorMsg{Err: err} }
		}
		m.Model = m.Model.SetConnected(true, msg.Provider, msg.Model)
		m.Model = m.Model.AddMessage("system", "Connected to "+msg.Provider+" successfully")
		return m, nil

	case tui.ListModelsRequestMsg:
		if !c.IsConnected() {
			m.Model = m.Model.AddMessage("system", "Not connected. Use /connect first.")
			return m, nil
		}
		models, err := c.ListModels(ctx)
		if err != nil {
			return m, func() tea.Msg { return tui.ErrorMsg{Err: err} }
		}
		var infos []tui.ModelInfo
		for _, mo := range models {
			infos = append(infos, tui.ModelInfo{
				ID:          mo.ID,
				Name:        mo.Name,
				ContextSize: mo.MaxContext,
			})
		}
		m.Model = m.Model.AddMessage("system", formatModels(infos))
		return m, nil

	case tui.SetModelRequestMsg:
		err := c.SetModel(msg.ModelID)
		if err != nil {
			return m, func() tea.Msg { return tui.ErrorMsg{Err: err} }
		}
		m.Model = m.Model.SetConnected(true, m.Provider, msg.ModelID)
		m.Model = m.Model.AddMessage("system", "Switched to model: "+msg.ModelID)
		return m, nil

	case tui.ChatRequestMsg:
		if !c.IsConnected() {
			m.Model = m.Model.AddMessage("system", "Not connected. Use /connect first.")
			return m, nil
		}
		m.Model = m.Model.AddMessage("assistant", "")
		go func() {
			err := c.ProcessStream(ctx, msg.Input, func(token string) {
				// Token streaming — full integration can push via a channel here.
			})
			if err != nil {
				log.Printf("[copilot] stream error: %v", err)
			}
		}()
		return m, nil

	// ── Background worker ────────────────────────────────────────────────────
	case tui.WorkerStartRequestMsg:
		if c.IsPolling() {
			m.Model = m.Model.AddMessage("system", "Background worker is already running.")
			return m, nil
		}
		c.StartPolling(ctx)
		m.Model = m.Model.SetWorkerRunning(true)
		m.Model = m.Model.AddMessage("system", "Background worker started — polling for pending tasks.")
		return m, nil

	case tui.WorkerStopRequestMsg:
		if !c.IsPolling() {
			m.Model = m.Model.AddMessage("system", "Background worker is not running.")
			return m, nil
		}
		c.StopPolling()
		m.Model = m.Model.SetWorkerRunning(false)
		m.Model = m.Model.AddMessage("system", "Background worker stopped.")
		return m, nil

	case tui.WorkerStatusRequestMsg:
		if c.IsPolling() {
			m.Model = m.Model.AddMessage("system", "Background worker: RUNNING  (use /worker-stop to stop)")
		} else {
			m.Model = m.Model.AddMessage("system", "Background worker: STOPPED  (use /worker-start to start)")
		}
		return m, nil

	case tui.SetHeadlessModeMsg:
		c.SetHeadless(msg.Headless)
		mode := "non-headless (visible browser)"
		if msg.Headless {
			mode = "headless"
		}
		m.Model = m.Model.SetHeadlessMode(msg.Headless)
		m.Model = m.Model.AddMessage("system", "Browser mode set to: "+mode)
		return m, nil

	case tui.RunTaskRequestMsg:
		result, err := c.RunTask(ctx, msg.Name, msg.URL, msg.Headless)
		if err != nil {
			m.Model = m.Model.AddMessage("system", fmt.Sprintf("Error: %v", err))
			return m, nil
		}
		mode := "visible browser"
		if msg.Headless {
			mode = "headless"
		}
		m.Model = m.Model.AddMessage("system", fmt.Sprintf("Task queued: %s [%s] id=%s", msg.Name, mode, result))
		return m, nil
	}

	// Delegate everything else (keyboard, window resize, streaming, …) to the
	// base tui.Model.
	newModel, cmd := m.Model.Update(msg)
	m.Model = newModel.(tui.Model)
	return m, cmd
}

// View delegates rendering entirely to the embedded model.
func (m CopilotModel) View() string {
	return m.Model.View()
}

// formatModels formats the model list for display in the chat panel.
func formatModels(models []tui.ModelInfo) string {
	var b strings.Builder
	b.WriteString("Available Models:\n")
	for _, m := range models {
		b.WriteString(fmt.Sprintf("  %-50s", m.Name))
		if m.ContextSize > 0 {
			b.WriteString(fmt.Sprintf(" (%dK context)", m.ContextSize/1000))
		}
		b.WriteString("\n")
	}
	return b.String()
}
