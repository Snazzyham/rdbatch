package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/soham/rdbatch/internal/api"
	"github.com/soham/rdbatch/internal/config"
	"github.com/soham/rdbatch/internal/download"
	"github.com/soham/rdbatch/internal/log"
	"github.com/soham/rdbatch/internal/models"
	"github.com/soham/rdbatch/internal/ui"
	"github.com/spf13/cobra"
)

var concurrent int

func isVideoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpeg", ".mpg", ".ts", ".m2ts":
		return true
	}
	return false
}

func selectVideoFiles(client *api.Client, id string) error {
	// Poll for torrent info up to 60 seconds
	var info *models.TorrentInfo
	for i := 0; i < 30; i++ {
		var err error
		info, err = client.GetTorrentInfo(id)
		if err == nil && len(info.Files) > 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if info == nil || len(info.Files) == 0 {
		return fmt.Errorf("torrent files not available yet")
	}

	var videoIDs []string
	for _, f := range info.Files {
		if isVideoFile(f.Path) {
			videoIDs = append(videoIDs, fmt.Sprintf("%d", f.ID))
		}
	}

	if len(videoIDs) == 0 {
		// Fallback to all files if no video files detected
		return client.SelectFiles(id, "all")
	}

	return client.SelectFiles(id, strings.Join(videoIDs, ","))
}

var rootCmd = &cobra.Command{
	Use:   "rdbatch",
	Short: "A terminal-native Real-Debrid batch downloader",
	Long:  `rdbatch integrates with the Real-Debrid API to add magnets, list torrents, and download files via aria2.`,
}

var fetchCmd = &cobra.Command{
	Use:   "fetch <magnet>",
	Short: "Add a magnet link to Real-Debrid",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			log.Printf("config load error: %v", err)
			return err
		}
		log.Printf("fetch: loaded config, magnet=%s", args[0])

		client := api.New(cfg.APIKey)
		magnet := args[0]

		if err := client.ValidateMagnet(magnet); err != nil {
			log.Printf("fetch: invalid magnet: %v", err)
			return err
		}

		resp, err := client.AddMagnet(magnet)
		if err != nil {
			log.Printf("fetch: add magnet failed: %v", err)
			return err
		}
		log.Printf("fetch: magnet added, id=%s", resp.ID)

		// Try to select video files automatically
		if err := selectVideoFiles(client, resp.ID); err != nil {
			log.Printf("fetch: select video files warning: %v", err)
			fmt.Fprintf(os.Stderr, "Warning: could not auto-select video files: %v\n", err)
		}

		fmt.Printf("Added torrent successfully:\n\n")
		fmt.Printf("Name: %s\n", magnet)
		fmt.Printf("ID: %s\n", resp.ID)
		fmt.Printf("Status: magnet_conversion\n")
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent torrents and download selected",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			log.Printf("list: config load error: %v", err)
			return err
		}
		log.Printf("list: config loaded")

		if err := download.CheckAria2(); err != nil {
			log.Printf("list: aria2 check failed: %v", err)
			return err
		}
		log.Printf("list: aria2 found")

		client := api.New(cfg.APIKey)
		torrents, err := client.GetTorrents()
		if err != nil {
			log.Printf("list: get torrents failed: %v", err)
			return err
		}
		log.Printf("list: received %d torrents", len(torrents))

		if len(torrents) == 0 {
			fmt.Println("No torrents found.")
			return nil
		}

		selected, err := ui.Run(torrents)
		if err != nil {
			log.Printf("list: tui error: %v", err)
			return err
		}
		log.Printf("list: %d torrents selected", len(selected))

		if len(selected) == 0 {
			fmt.Println("No torrents selected.")
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not get current directory: %w", err)
		}

		fmt.Printf("Downloading %d torrent(s) to %s...\n", len(selected), cwd)

		dl := download.New(concurrent)
		var failed int
		for _, torrent := range selected {
			log.Printf("list: processing torrent id=%s name=%s", torrent.ID, torrent.Filename)
			info, err := client.GetTorrentInfo(torrent.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting info for %s: %v\n", torrent.Filename, err)
				failed++
				continue
			}

			if len(info.Links) == 0 {
				fmt.Fprintf(os.Stderr, "No links available for %s\n", torrent.Filename)
				failed++
				continue
			}

			var urls []string
			for _, link := range info.Links {
				unrestricted, err := client.UnrestrictLink(link)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error unrestricing link for %s: %v\n", torrent.Filename, err)
					continue
				}
				urls = append(urls, unrestricted.Download)
			}

			if len(urls) == 0 {
				fmt.Fprintf(os.Stderr, "No unrestricted links for %s\n", torrent.Filename)
				failed++
				continue
			}

			if err := dl.Download(urls, cwd); err != nil {
				fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n", torrent.Filename, err)
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
