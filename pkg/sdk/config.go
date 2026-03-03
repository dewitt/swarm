package sdk

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultModel is the fallback model string if no config is present.
const DefaultModel = "gemini-2.5-flash"

// Config represents the global user configuration for the Swarm CLI.
type Config struct {
	Model string `yaml:"model"`
	// Additional global preferences (e.g. editor, default skills dir) can go here.
}

// GetConfigDir returns the directory for Swarm configuration and state, creating it if necessary.
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(home, ".config", "swarm")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return configDir, nil
}

// DefaultConfigPath returns the path to the global configuration file.
func DefaultConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// LoadConfig reads the global configuration file.
func LoadConfig() (*Config, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return a default config if it doesn't exist
			return &Config{Model: DefaultModel}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Fallback to default if somehow blank
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}

	return &cfg, nil
}

// SaveConfig writes the configuration to the global file.
func SaveConfig(cfg *Config) error {
	path, err := DefaultConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// MemoryPath returns the path to the global memory file.
func MemoryPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "memory.md"), nil
}

// SaveMemory appends a fact or preference to the global memory file.
func SaveMemory(fact string) error {
	path, err := MemoryPath()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("- " + fact + "\n")
	return err
}

// LoadMemory reads the global memory file and returns its contents.
func LoadMemory() (string, error) {
	path, err := MemoryPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	return string(data), nil
}
