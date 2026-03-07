package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	app := NewApp()

	// Graceful shutdown on OS signals (SIGINT, SIGTERM).
	// Wails calls OnShutdown when the window closes, but OS signals
	// (e.g. kill, Ctrl+C in dev mode) bypass that path.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal, cleaning up...")
		app.shutdownFromSignal()
		os.Exit(0)
	}()

	err := wails.Run(&options.App{
		Title:     "Web Automation Dashboard",
		Width:     1400,
		Height:    900,
		MinWidth:  1024,
		MinHeight: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 17, G: 24, B: 39, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		log.Fatal("Error:", err.Error())
	}
}
