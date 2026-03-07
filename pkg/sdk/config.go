package sdk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	if err := os.MkdirAll(configDir, 0o755); err != nil {
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

	if err := os.WriteFile(path, data, 0o600); err != nil {
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

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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

// LoadContextFiles searches for and concatenates AGENTS.md files for context.
func LoadContextFiles() (string, []string) {
	var contextParts []string
	var loadedFiles []string
	seen := make(map[string]bool)

	addContext := func(path string, description string) {
		path = filepath.Clean(path)
		if seen[path] {
			return
		}
		if b, err := os.ReadFile(path); err == nil {
			contextParts = append(contextParts, "--- Context from: "+description+" ---\n"+string(b)+"\n--- End of Context from: "+description+" ---")
			loadedFiles = append(loadedFiles, path)
			seen[path] = true
		}
	}

	// 1. Global Context
	if cfgDir, err := GetConfigDir(); err == nil {
		addContext(filepath.Join(cfgDir, "AGENTS.md"), "~/.config/swarm/AGENTS.md")
	}

	// 2. Workspace/Project Context
	cwd, err := os.Getwd()
	if err == nil {
		dir := cwd
		var projectDirs []string
		for {
			projectDirs = append([]string{dir}, projectDirs...)
			if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
		for _, d := range projectDirs {
			addContext(filepath.Join(d, "AGENTS.md"), filepath.Join(d, "AGENTS.md"))
		}

		// 3. Local/Sub-directory Context
		count := 0
		_ = filepath.WalkDir(cwd, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // Ignore permission errors and continue walking
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "vendor" {
					return filepath.SkipDir
				}
				count++
				if count > 200 {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Name() == "AGENTS.md" {
				addContext(path, path)
			}
			return nil
		})
	}

	if len(contextParts) == 0 {
		return "", loadedFiles
	}
	return "\n<loaded_context>\n" + strings.Join(contextParts, "\n") + "\n</loaded_context>\n", loadedFiles
}
