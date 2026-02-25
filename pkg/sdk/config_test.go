package sdk_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dewitt/agents/pkg/sdk"
	"github.com/stretchr/testify/assert"
)

func TestConfigSaveAndLoad(t *testing.T) {
	// Override the HOME environment variable so we don't mess up the actual user's config
	tempHome := t.TempDir()
	os.Setenv("HOME", tempHome)
	defer os.Unsetenv("HOME")

	// Test default load
	cfg, err := sdk.LoadConfig()
	assert.NoError(t, err)
	assert.Equal(t, "gemini-2.5-flash", cfg.Model)

	// Test save
	cfg.Model = "gemini-2.5-pro"
	err = sdk.SaveConfig(cfg)
	assert.NoError(t, err)

	// Verify the file was physically created
	path := filepath.Join(tempHome, ".config", "agents", "config.yaml")
	_, err = os.Stat(path)
	assert.NoError(t, err)

	// Test load modified
	loadedCfg, err := sdk.LoadConfig()
	assert.NoError(t, err)
	assert.Equal(t, "gemini-2.5-pro", loadedCfg.Model)
}
