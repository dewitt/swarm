package sdk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ParseManifest reads and parses an agent.yaml file.
func ParseManifest(path string) (*AgentManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("manifest not found at %s", path)
		}
		return nil, fmt.Errorf("error reading manifest: %w", err)
	}

	var manifest AgentManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid agent.yaml format: %w", err)
	}

	// Basic validation
	if manifest.Name == "" {
		return nil, errors.New("agent manifest missing required field: name")
	}
	if manifest.Framework == "" {
		return nil, errors.New("agent manifest missing required field: framework")
	}
	if manifest.Language == "" {
		return nil, errors.New("agent manifest missing required field: language")
	}

	return &manifest, nil
}

// Discover checks the current directory (and parent directories up to a limit)
// for an agent.yaml manifest.
func (m *defaultManager) Discover(ctx context.Context, dir string) (*AgentManifest, error) {
	currentDir := dir
	if currentDir == "" {
		currentDir = "."
	}

	absPath, err := filepath.Abs(currentDir)
	if err != nil {
		return nil, err
	}

	// Search up the directory tree for agent.yaml (max 5 levels deep)
	for i := 0; i < 5; i++ {
		manifestPath := filepath.Join(absPath, "agent.yaml")
		if _, err := os.Stat(manifestPath); err == nil {
			return ParseManifest(manifestPath)
		}

		parentDir := filepath.Dir(absPath)
		if parentDir == absPath {
			break // Reached root
		}
		absPath = parentDir
	}

	return nil, errors.New("no agent.yaml found in the current directory or its parents")
}
