package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"flowpilot/internal/agent"
)

var version = "dev"

func main() {
	dataDir := flag.String("data-dir", "", "data directory (default ~/.flowpilot)")
	concurrency := flag.Int("concurrency", 10, "max concurrent browser tasks")
	poll := flag.Duration("poll", 30*time.Second, "interval between pending-task polls")
	healthInterval := flag.Int("health-interval", 300, "proxy health check interval in seconds")
	maxFailures := flag.Int("max-failures", 3, "max proxy failures before marking unhealthy")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("flowpilot-agent", version)
		os.Exit(0)
	}

	cfg := agent.Config{
		DataDir:             *dataDir,
		MaxConcurrency:      *concurrency,
		PollInterval:        *poll,
		HealthCheckInterval: *healthInterval,
		MaxProxyFailures:    *maxFailures,
	}

	a, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		cancel()
	}()

	if err := a.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("agent exited with error: %v", err)
	}
}
