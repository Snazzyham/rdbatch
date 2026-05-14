package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soham/rdbatch/internal/models"
)

const configDir = ".config/rdbatch"
const configFile = "config.json"

// Load resolves the active provider and its API key.
// Resolution order:
//   Provider: RDBATCH_PROVIDER env > provider in config file
//   Real-Debrid key: REALDEBRID_API_KEY env > realdebrid_api_key in config > api_key in config (backward compat)
//   Torbox key: TORBOX_API_KEY env > torbox_api_key in config
func Load() (*models.Config, error) {
	// Read config file first (we need it for fallbacks)
	fileCfg, filePath, err := loadConfigFile()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Resolve provider
	provider := os.Getenv("RDBATCH_PROVIDER")
	if provider == "" && fileCfg != nil {
		provider = fileCfg.Provider
	}
	if provider == "" {
		return nil, fmt.Errorf("RDBATCH_PROVIDER is not set.\nSet it to either \"torbox\" or \"real-debrid\".\n\nExample:\n  export RDBATCH_PROVIDER=torbox")
	}
	if provider != "torbox" && provider != "real-debrid" {
		return nil, fmt.Errorf("RDBATCH_PROVIDER is not set.\nSet it to either \"torbox\" or \"real-debrid\".\n\nExample:\n  export RDBATCH_PROVIDER=torbox")
	}

	cfg := &models.Config{Provider: provider}

	switch provider {
	case "real-debrid":
		apiKey := os.Getenv("REALDEBRID_API_KEY")
		if apiKey == "" && fileCfg != nil {
			apiKey = fileCfg.RealDebridAPIKey
		}
		// Backward compatibility: fall back to old api_key field
		if apiKey == "" && fileCfg != nil {
			apiKey = fileCfg.APIKey
		}
		if apiKey == "" {
			return nil, fmt.Errorf("no API key found for provider \"real-debrid\".\nSet REALDEBRID_API_KEY or add \"realdebrid_api_key\" to %s", filePath)
		}
		cfg.RealDebridAPIKey = apiKey

	case "torbox":
		apiKey := os.Getenv("TORBOX_API_KEY")
		if apiKey == "" && fileCfg != nil {
			apiKey = fileCfg.TorboxAPIKey
		}
		if apiKey == "" {
			return nil, fmt.Errorf("no API key found for provider \"torbox\".\nSet TORBOX_API_KEY or add \"torbox_api_key\" to %s", filePath)
		}
		cfg.TorboxAPIKey = apiKey
	}

	return cfg, nil
}

func loadConfigFile() (*models.Config, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("could not get home directory: %w", err)
	}

	path := filepath.Join(home, configDir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, err
	}

	var cfg models.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, path, fmt.Errorf("could not parse config file: %w", err)
	}

	return &cfg, path, nil
}
