package sdk_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dewitt/swarm/pkg/sdk"
	"github.com/stretchr/testify/assert"
)

func TestDiscover_ManifestExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Write a valid agent.yaml
	yamlContent := `
name: test-agent
framework: adk
language: go
entrypoint: main.go
`
	manifestPath := filepath.Join(tempDir, "agent.yaml")
	err := os.WriteFile(manifestPath, []byte(yamlContent), 0644)
	assert.NoError(t, err)

	// We only need the defaultManager to call Discover, we don't need a real LLM for this
	// But Discover is a method on AgentManager.
	manager := sdk.NewManager(sdk.ManagerConfig{Model: &MockModel{}})

	// Call Discover
	manifest, err := manager.Discover(context.Background(), tempDir)

	assert.NoError(t, err)
	assert.NotNil(t, manifest)
	assert.Equal(t, "test-agent", manifest.Name)
	assert.Equal(t, "adk", manifest.Framework)
	assert.Equal(t, "go", manifest.Language)
	assert.Equal(t, "main.go", manifest.Entrypoint)
}

func TestDiscover_ManifestNotFound(t *testing.T) {
	tempDir := t.TempDir()
	manager := sdk.NewManager(sdk.ManagerConfig{Model: &MockModel{}})

	manifest, err := manager.Discover(context.Background(), tempDir)

	assert.Error(t, err)
	assert.Nil(t, manifest)
	assert.Contains(t, err.Error(), "no agent.yaml found")
}

func TestDiscover_InvalidManifest(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
name: test-agent
# Missing framework and language
`
	manifestPath := filepath.Join(tempDir, "agent.yaml")
	err := os.WriteFile(manifestPath, []byte(yamlContent), 0644)
	assert.NoError(t, err)

	manager := sdk.NewManager(sdk.ManagerConfig{Model: &MockModel{}})

	manifest, err := manager.Discover(context.Background(), tempDir)

	assert.Error(t, err)
	assert.Nil(t, manifest)
	assert.Contains(t, err.Error(), "missing required field: framework")
}
