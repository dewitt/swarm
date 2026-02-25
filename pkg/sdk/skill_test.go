package sdk_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dewitt/agents/pkg/sdk"
	"github.com/stretchr/testify/assert"
)

func TestLoadSkill_ValidSkill(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "test-skill")
	err := os.Mkdir(skillDir, 0755)
	assert.NoError(t, err)

	// Write instructions.md
	instructions := "You are a test skill. Do test things."
	err = os.WriteFile(filepath.Join(skillDir, "instructions.md"), []byte(instructions), 0644)
	assert.NoError(t, err)

	// Write tools.yaml
	toolsYaml := `
name: test-skill
description: A mock skill for testing
tools:
  - write_local_file
  - list_local_files
`
	err = os.WriteFile(filepath.Join(skillDir, "tools.yaml"), []byte(toolsYaml), 0644)
	assert.NoError(t, err)

	skill, err := sdk.LoadSkill(skillDir)
	assert.NoError(t, err)
	assert.NotNil(t, skill)

	assert.Equal(t, "test-skill", skill.Manifest.Name)
	assert.Equal(t, "A mock skill for testing", skill.Manifest.Description)
	assert.ElementsMatch(t, []string{"write_local_file", "list_local_files"}, skill.Manifest.Tools)
	assert.Equal(t, instructions, skill.Instructions)
}

func TestLoadSkill_MissingInstructions(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "empty-skill")
	err := os.Mkdir(skillDir, 0755)
	assert.NoError(t, err)

	skill, err := sdk.LoadSkill(skillDir)
	assert.Error(t, err)
	assert.Nil(t, skill)
	assert.Contains(t, err.Error(), "missing required instructions.md")
}

func TestLoadSkill_NoToolsYaml(t *testing.T) {
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "simple-skill")
	err := os.Mkdir(skillDir, 0755)
	assert.NoError(t, err)

	instructions := "I only have instructions."
	err = os.WriteFile(filepath.Join(skillDir, "instructions.md"), []byte(instructions), 0644)
	assert.NoError(t, err)

	skill, err := sdk.LoadSkill(skillDir)
	assert.NoError(t, err)
	assert.NotNil(t, skill)
	
	// Should fallback to the directory name
	assert.Equal(t, "simple-skill", skill.Manifest.Name)
	assert.Equal(t, instructions, skill.Instructions)
	assert.Empty(t, skill.Manifest.Tools)
}
