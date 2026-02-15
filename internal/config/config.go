package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	GitHubUsername string
	GitHubToken    string

	SpaceID  string
	CMAToken string
	EntryID  string

	GeminiAPIKey string

	MaxFeatured int
	MaxProjects int
	ForceUpdate bool
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		GitHubUsername: os.Getenv("GITHUB_USERNAME"),
		GitHubToken:   os.Getenv("GITHUB_TOKEN"),
		SpaceID:       os.Getenv("CONTENTFUL_SPACE_ID"),
		CMAToken:      os.Getenv("CONTENTFUL_CMA_TOKEN"),
		EntryID:       os.Getenv("CONTENTFUL_ENTRY_ID"),
		GeminiAPIKey:  os.Getenv("GEMINI_API_KEY"),
	}

	if cfg.GitHubUsername == "" {
		cfg.GitHubUsername = "alberto-moreno-sa"
	}

	if cfg.SpaceID == "" {
		return nil, fmt.Errorf("CONTENTFUL_SPACE_ID is required")
	}
	if cfg.CMAToken == "" {
		return nil, fmt.Errorf("CONTENTFUL_CMA_TOKEN is required")
	}
	if cfg.EntryID == "" {
		return nil, fmt.Errorf("CONTENTFUL_ENTRY_ID is required")
	}
	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}

	cfg.MaxFeatured = envInt("MAX_FEATURED", 5)
	cfg.MaxProjects = envInt("MAX_PROJECTS", 15)
	cfg.ForceUpdate, _ = strconv.ParseBool(os.Getenv("FORCE_UPDATE"))

	return cfg, nil
}

func envInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
