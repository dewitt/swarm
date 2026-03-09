package sdk_test

import (
	"os"
	"testing"

	"github.com/dewitt/swarm/pkg/sdk"
	"github.com/stretchr/testify/assert"
)

func TestConfigSaveAndLoad(t *testing.T) {
	// Use t.Setenv so we don't permanently mess up the actual user's config
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Test default load
	cfg, err := sdk.LoadConfig()
	assert.NoError(t, err)
	assert.Equal(t, sdk.DefaultModel, cfg.Model)

	// Test save
	cfg.Model = "gemini-2.5-pro"
	err = sdk.SaveConfig(cfg)
	assert.NoError(t, err)

	// Verify the file was physically created
	path, err := sdk.DefaultConfigPath()
	assert.NoError(t, err)
	_, err = os.Stat(path)
	assert.NoError(t, err)

	// Test load modified
	loadedCfg, err := sdk.LoadConfig()
	assert.NoError(t, err)
	assert.Equal(t, "gemini-2.5-pro", loadedCfg.Model)
}
