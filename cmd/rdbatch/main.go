package main

import (
	"fmt"
	"os"

	"github.com/soham/rdbatch/internal/api"
	"github.com/soham/rdbatch/internal/commands"
	"github.com/soham/rdbatch/internal/config"
	"github.com/soham/rdbatch/internal/log"
)

func main() {
	if err := log.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize logger: %v\n", err)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	log.Printf("main: provider=%s", cfg.Provider)

	var client api.Provider
	switch cfg.Provider {
	case "torbox":
		client = api.NewTorboxClient(cfg.TorboxAPIKey)
	case "real-debrid":
		client = api.NewRealDebridClient(cfg.RealDebridAPIKey)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown provider %q\n", cfg.Provider)
		os.Exit(1)
	}

	commands.SetProvider(client)

	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
