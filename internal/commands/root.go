package commands

import (
	"fmt"
	"os"

	"github.com/soham/rdbatch/internal/api"
	"github.com/soham/rdbatch/internal/download"
	"github.com/soham/rdbatch/internal/log"
	"github.com/soham/rdbatch/internal/ui"
	"github.com/spf13/cobra"
)

var concurrent int

var provider api.Provider

func SetProvider(p api.Provider) {
	provider = p
}

var rootCmd = &cobra.Command{
	Use:   "rdbatch",
	Short: "A terminal-native batch downloader for Real-Debrid and Torbox",
	Long:  `rdbatch integrates with Real-Debrid and Torbox APIs to add magnets, list torrents, and download files via aria2.`,
}

var fetchCmd = &cobra.Command{
	Use:   "fetch <magnet>",
	Short: "Add a magnet link",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if provider == nil {
			return fmt.Errorf("provider not initialized")
		}

		magnet := args[0]
		log.Printf("fetch: magnet=%s", magnet)

		id, name, status, err := provider.AddMagnet(magnet)
		if err != nil {
			log.Printf("fetch: add magnet failed: %v", err)
			return err
		}
		log.Printf("fetch: magnet added, id=%s", id)

		fmt.Printf("Added torrent successfully:\n\n")
		fmt.Printf("Name: %s\n", name)
		fmt.Printf("ID: %s\n", id)
		fmt.Printf("Status: %s\n", status)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent torrents and download selected",
	RunE: func(cmd *cobra.Command, args []string) error {
		if provider == nil {
			return fmt.Errorf("provider not initialized")
		}

		if err := download.CheckAria2(); err != nil {
			log.Printf("list: aria2 check failed: %v", err)
			return err
		}
		log.Printf("list: aria2 found")

		torrents, err := provider.ListTorrents()
		if err != nil {
			log.Printf("list: get torrents failed: %v", err)
			return err
		}
		log.Printf("list: received %d torrents", len(torrents))

		if len(torrents) == 0 {
			fmt.Println("No torrents found.")
			return nil
		}

		selected, err := ui.Run(provider, torrents)
		if err != nil {
			log.Printf("list: tui error: %v", err)
			return err
		}
		log.Printf("list: %d items selected", len(selected))

		if len(selected) == 0 {
			fmt.Println("No items selected.")
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not get current directory: %w", err)
		}

		fmt.Printf("Downloading %d selection(s) to %s...\n", len(selected), cwd)

		dl := download.New(concurrent, provider.Aria2Flags())
		var failed int
		for _, sel := range selected {
			log.Printf("list: processing torrent id=%s name=%s files=%v", sel.TorrentID, sel.Name, sel.FileIDs)

			urls, err := provider.GetDownloadLinks(sel.TorrentID, sel.FileIDs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting links for %s: %v\n", sel.Name, err)
				failed++
				continue
			}

			if len(urls) == 0 {
				fmt.Fprintf(os.Stderr, "No links available for %s\n", sel.Name)
				failed++
				continue
			}

			if err := dl.Download(urls, cwd); err != nil {
				fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n", sel.Name, err)
				failed++
				continue
			}
		}

		if failed > 0 {
			fmt.Printf("\n%d torrent(s) failed to download.\n", failed)
		} else {
			fmt.Println("\nAll downloads complete.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntVarP(&concurrent, "concurrent", "c", 0, "Maximum concurrent aria2 downloads (0 = unlimited)")
}

func Execute() error {
	return rootCmd.Execute()
}
