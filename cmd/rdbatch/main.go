package main

import (
	"fmt"
	"os"

	"github.com/soham/rdbatch/internal/commands"
	"github.com/soham/rdbatch/internal/log"
)

func main() {
	if err := log.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize logger: %v\n", err)
	}

	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
