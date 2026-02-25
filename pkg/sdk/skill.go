package sdk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SkillManifest defines the structure of a skill's tools.yaml file.
// For now, it simply declares a list of SDK-provided tools the skill requires.
type SkillManifest struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tools       []string `yaml:"tools"`
}

// Skill represents a dynamically loaded set of capabilities.
type Skill struct {
	Manifest     SkillManifest
	Instructions string
	Path         string
}

// LoadSkill reads a skill directory and returns the parsed Skill.
// A valid skill directory must contain an instructions.md file.
// A tools.yaml file is optional.
func LoadSkill(skillDir string) (*Skill, error) {
	info, err := os.Stat(skillDir)
	if err != nil {
		return nil, fmt.Errorf("skill directory not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skill path %s is not a directory", skillDir)
	}

	skill := &Skill{
		Path: skillDir,
	}

	// 1. Read instructions.md (Required)
	instructionsPath := filepath.Join(skillDir, "instructions.md")
	instructionsData, err := os.ReadFile(instructionsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("skill missing required instructions.md: %s", skillDir)
		}
		return nil, fmt.Errorf("error reading instructions.md: %w", err)
	}
	skill.Instructions = string(instructionsData)

	// 2. Read tools.yaml (Optional)
	toolsPath := filepath.Join(skillDir, "tools.yaml")
	toolsData, err := os.ReadFile(toolsPath)
	if err == nil {
		if err := yaml.Unmarshal(toolsData, &skill.Manifest); err != nil {
			return nil, fmt.Errorf("invalid tools.yaml format: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("error reading tools.yaml: %w", err)
	}

	// Fallback name if tools.yaml is missing or didn't provide one
	if skill.Manifest.Name == "" {
		skill.Manifest.Name = filepath.Base(skillDir)
	}

	return skill, nil
}
