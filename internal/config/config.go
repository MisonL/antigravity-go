package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider    string `yaml:"provider" json:"provider"`
	Model       string `yaml:"model" json:"model"`
	BaseURL     string `yaml:"base_url" json:"base_url"`
	APIKey      string `yaml:"api_key" json:"api_key"` // Optional, env var is preferred
	MaxContext  int    `yaml:"max_context_tokens" json:"max_context_tokens"`
	MaxOutput   int    `yaml:"max_output_tokens" json:"max_output_tokens"`
	CoreBinPath string `yaml:"core_bin_path" json:"core_bin_path"`
	DataDir     string `yaml:"data_dir" json:"data_dir"`
	WebHost     string `yaml:"web_host" json:"web_host"`
	WebPort     int    `yaml:"web_port" json:"web_port"`
	AuthToken   string `yaml:"auth_token" json:"auth_token"`
	Approvals   string `yaml:"approvals" json:"approvals"`
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ".agy_go"
	}
	return filepath.Join(home, ".agy_go")
}

func normalizeDataDir(dataDir string) string {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return defaultDataDir()
	}
	return dataDir
}

func ConfigPathForDataDir(dataDir string) string {
	return filepath.Join(normalizeDataDir(dataDir), "config.yaml")
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	dataDir := defaultDataDir()
	return &Config{
		Provider:    "openai",
		Model:       "gpt-5.4",
		BaseURL:     "",
		MaxContext:  10000,
		MaxOutput:   0,
		DataDir:     dataDir,
		CoreBinPath: "./antigravity_core", // Expecting in PATH or cwd
		WebHost:     "127.0.0.1",
		WebPort:     8888,
		AuthToken:   "", // Empty means disabled
		Approvals:   "prompt",
	}
}

// Load reads the config file from default location or specific path
// Default location: ~/.agy_go/config.yaml
func Load() (*Config, error) {
	return LoadFromDataDir("")
}

func LoadFromDataDir(dataDir string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.DataDir = normalizeDataDir(dataDir)
	configPath := ConfigPathForDataDir(dataDir)
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
	configDir := normalizeDataDir(c.DataDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	configPath := ConfigPathForDataDir(configDir)
	copyCfg := *c
	copyCfg.DataDir = configDir

	data, err := yaml.Marshal(&copyCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
