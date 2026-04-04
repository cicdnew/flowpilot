package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"flowpilot/internal/copilot"
)

var version = "dev"

func main() {
	dataDir := flag.String("data-dir", "", "data directory (default ~/.flowpilot)")
	concurrency := flag.Int("concurrency", 10, "max concurrent browser tasks")
	provider := flag.String("provider", "", "LLM provider (openai, openrouter, gemini, nvidia, huggingface, github, kilo)")
	apiKey := flag.String("api-key", "", "API key for the LLM provider")
	baseURL := flag.String("base-url", "", "Custom base URL for LLM provider")
	model := flag.String("model", "", "Model name to use")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("flowpilot-copilot", version)
		os.Exit(0)
	}

	cfg := copilot.Config{
		DataDir:        *dataDir,
		MaxConcurrency: *concurrency,
		ModelProvider:  *provider,
		APIKey:         *apiKey,
		BaseURL:        *baseURL,
		ModelName:      *model,
	}

	c, err := copilot.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create copilot: %v", err)
	}
	defer c.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received %s, shutting down...", sig)
		cancel()
		c.Stop()
		os.Exit(0)
	}()

	fmt.Println("FlowPilot Copilot", version)
	fmt.Println("Type /help for commands, /exit to quit")
	fmt.Println("----------------------------------------")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if err := handleCommand(ctx, c, input); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			continue
		}

		// Process natural language request
		fmt.Println("Thinking...")
		start := time.Now()
		response, err := c.Process(ctx, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\n%s\n", response)
		fmt.Printf("\n(Response time: %v, Model: %s)\n", time.Since(start), c.CurrentModel())
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}

func handleCommand(ctx context.Context, c *copilot.CopilotFlow, cmd string) error {
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "/help":
		fmt.Println("Available commands:")
		fmt.Println("  /connect <provider> <api-key> [base-url] [model]")
		fmt.Println("  /models - List available models from provider")
		fmt.Println("  /set-model <model-id> - Switch to different model")
		fmt.Println("  /status  - Show current connection status")
		fmt.Println("  /exit    - Exit the copilot")
		fmt.Println("\nSupported providers: openai, openrouter, gemini, nvidia, huggingface, github, kilo")
		return nil

	case "/connect":
		if len(parts) < 3 {
			return fmt.Errorf("usage: /connect <provider> <api-key> [base-url] [model]")
		}

		provider := parts[1]
		apiKey := parts[2]
		baseURL := ""
		model := ""

		if len(parts) > 3 {
			baseURL = parts[3]
		}
		if len(parts) > 4 {
			model = parts[4]
		}

		if err := c.Connect(provider, apiKey, baseURL, model); err != nil {
			return err
		}

		fmt.Printf("Connected to %s successfully!\n", provider)
		fmt.Printf("Using model: %s\n", c.CurrentModel())
		return nil

	case "/models":
		if !c.IsConnected() {
			return fmt.Errorf("not connected to any provider")
		}

		fmt.Println("Fetching available models...")
		models, err := c.ListModels(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch models: %w", err)
		}

		if len(models) == 0 {
			fmt.Println("No models available")
			return nil
		}

		fmt.Printf("\n%-60s %s\n", "MODEL ID", "CONTEXT WINDOW")
		fmt.Println(strings.Repeat("-", 100))
		for _, m := range models {
			fmt.Printf("%-60s %dK\n", m.ID, m.MaxContext/1000)
		}
		fmt.Println()
		return nil

	case "/set-model":
		if len(parts) < 2 {
			return fmt.Errorf("usage: /set-model <model-id>")
		}

		modelID := parts[1]
		if err := c.SetModel(modelID); err != nil {
			return err
		}

		fmt.Printf("Switched to model: %s\n", modelID)
		return nil

	case "/status":
		if c.IsConnected() {
			fmt.Println("✅ Connected to LLM provider")
			fmt.Printf("   Provider: %s\n", c.CurrentProvider())
			fmt.Printf("   Model:    %s\n", c.CurrentModel())
			fmt.Println("\nReady to process natural language requests")
		} else {
			fmt.Println("❌ Not connected")
			fmt.Println("Use /connect <provider> <api-key> to connect")
		}
		return nil

	case "/exit":
		fmt.Println("Goodbye!")
		os.Exit(0)
		return nil

	default:
		return fmt.Errorf("unknown command: %s. Type /help for available commands", parts[0])
	}
}
