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

func Load() (*models.Config, error) {
	// Environment variable takes precedence
	if apiKey := os.Getenv("REALDEBRID_API_KEY"); apiKey != "" {
		return &models.Config{APIKey: apiKey}, nil
	}

	// Fallback to config file
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get home directory: %w", err)
	}

	path := filepath.Join(home, configDir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no API key found: set REALDEBRID_API_KEY or create %s", path)
		}
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var cfg models.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is empty in config file")
	}

	return &cfg, nil
}
