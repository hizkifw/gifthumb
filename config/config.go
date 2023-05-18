package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Config struct {
	// Number of snapshots to take
	NSnapshots int
	// Resize snapshot to this height
	ThumbHeight int
	// Framerate of the gif
	GifFramerate int
	// Path to the cache directory
	CacheDir string
	// List of allowed hosts for the URL
	AllowedHosts []string
}

func GetConfig() (*Config, error) {
	// Check if CONFIG_JSON environment variable is set
	configJSON := os.Getenv("CONFIG_JSON")
	if configJSON != "" {
		log.Println("Using config from CONFIG_JSON environment variable")
		var config *Config
		err := json.Unmarshal([]byte(configJSON), &config)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal config JSON: %v", err)
		}
		return config, nil
	}

	// Check if CONFIG_PATH file exists
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.json" // Default config file path
	}

	log.Println("Using config from", configPath)
	_, err := os.Stat(configPath)
	if err == nil {
		file, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %v", err)
		}

		var config *Config
		err = json.Unmarshal(file, &config)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal config JSON: %v", err)
		}
		return config, nil
	}

	return nil, fmt.Errorf("config not found")
}

func (c *Config) IsHostAllowed(host string) bool {
	for _, h := range c.AllowedHosts {
		if h == host {
			return true
		}
	}
	return false
}
