package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/soham/rdbatch/internal/config"
	"github.com/soham/rdbatch/internal/search"
	"github.com/soham/rdbatch/internal/ui"
	"github.com/spf13/cobra"
)

const noCometURLHelp = `error: search requires a Comet manifest URL.

The 'fetch' and 'list' commands work without it. To enable search,
set COMET_URL or add "comet_url" to ~/.config/rdbatch/config.json:

  export COMET_URL="https://your-comet-instance/manifest.json"

The Comet instance must be configured for the same debrid provider
as RDBATCH_PROVIDER. See README.md.
`

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for torrents via Comet",
	Long: `Search opens an interactive TUI for finding torrents via a Comet scraper.

It searches Cinemeta for movies and series, then scrapes torrents through your
configured Comet instance. Cached torrents can be streamed immediately (W key),
while uncached torrents can be added to your debrid provider (Enter key).

Requires COMET_URL to be set via environment variable or config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get Comet URL
		cometURL := config.CometURL()
		if cometURL == "" {
			fmt.Fprintln(os.Stderr, noCometURLHelp)
			os.Exit(1)
		}

		// Check for provider mismatch
		if strings.Contains(cometURL, "realdebrid") && provider != nil {
			// Try to detect if Comet is configured for a different provider
			// by checking if provider type matches what's in the URL/config
			if p, ok := provider.(interface{ Name() string }); ok {
				providerName := strings.ToLower(p.Name())
				cometLower := strings.ToLower(cometURL)

				// Simple heuristic checks
				if (strings.Contains(cometLower, "realdebrid") || strings.Contains(cometLower, "real-debrid")) &&
					!strings.Contains(providerName, "real") {
					fmt.Fprintln(os.Stderr, "Warning: Comet appears configured for Real-Debrid but RDBATCH_PROVIDER is set to a different provider.")
					fmt.Fprintln(os.Stderr, "This may cause issues. Ensure both are configured for the same debrid service.")
					fmt.Fprintln(os.Stderr)
				}
			}
		}

		// Initialize clients
		cm := search.NewCinemeta()
		co, err := search.NewComet(cometURL)
		if err != nil {
			return fmt.Errorf("invalid Comet URL: %w", err)
		}

		// Run the search TUI
		return ui.RunSearch(cm, co, provider)
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
