package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider    string `yaml:"provider"`
	Model       string `yaml:"model"`
	BaseURL     string `yaml:"base_url"`
	APIKey      string `yaml:"api_key"` // Optional, env var is preferred
	MaxContext  int    `yaml:"max_context_tokens"`
	MaxOutput   int    `yaml:"max_output_tokens"`
	CoreBinPath string `yaml:"core_bin_path"`
	DataDir     string `yaml:"data_dir"`
	WebHost     string `yaml:"web_host"`
	WebPort     int    `yaml:"web_port"`
	AuthToken   string `yaml:"auth_token"`
	Approvals   string `yaml:"approvals"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Provider:    "openai",
		Model:       "gpt-4o",
		BaseURL:     "",
		MaxContext:  10000,
		MaxOutput:   0,
		DataDir:     filepath.Join(home, ".antigravity"),
		CoreBinPath: "./antigravity_core", // Expecting in PATH or cwd
		WebHost:     "127.0.0.1",
		WebPort:     8888,
		AuthToken:   "", // Empty means disabled
		Approvals:   "prompt",
	}
}

// Load reads the config file from default location or specific path
// Default location: ~/.antigravity/config.yaml
func Load() (*Config, error) {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil // Fallback to defaults if home dir not found
	}

	configPath := filepath.Join(home, ".antigravity", "config.yaml")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil // Use defaults
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Save writes the config to the default location
func (c *Config) Save() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	configDir := filepath.Join(home, ".antigravity")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
