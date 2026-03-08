package sdk

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillManifest defines the structure of a skill's metadata (from YAML frontmatter).
type SkillManifest struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tools       []string `yaml:"tools"`
	Model       string   `yaml:"model"` // Optional: "flash" or "pro"
}

// Skill represents a dynamically loaded set of capabilities following the open agentskills.io standard.
type Skill struct {
	Manifest     SkillManifest
	Instructions string
	Path         string
}

// LoadSkill reads a skill directory adhering to the agentskills.io standard from the local disk.
func LoadSkill(skillDir string) (*Skill, error) {
	return LoadSkillFromFS(os.DirFS("/"), strings.TrimPrefix(skillDir, "/"))
}

// LoadSkillFromFS reads a skill directory adhering to the agentskills.io standard from an abstract filesystem.
func LoadSkillFromFS(fsys fs.FS, skillDir string) (*Skill, error) {
	skillDir = filepath.Clean(skillDir)
	info, err := fs.Stat(fsys, skillDir)
	if err != nil {
		return nil, fmt.Errorf("skill directory not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skill path %s is not a directory", skillDir)
	}

	skill := &Skill{
		Path: skillDir,
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := fs.Stat(fsys, skillPath); err != nil {
		return nil, fmt.Errorf("could not find SKILL.md in %s: %w", skillDir, err)
	}

	data, err := fs.ReadFile(fsys, skillPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md in %s: %w", skillDir, err)
	}

	// Parse YAML frontmatter
	const separator = "---\n"
	if bytes.HasPrefix(data, []byte(separator)) {
		parts := bytes.SplitN(data[4:], []byte(separator), 2)
		if len(parts) == 2 {
			frontmatter := parts[0]
			instructions := parts[1]

			if err := yaml.Unmarshal(frontmatter, &skill.Manifest); err != nil {
				return nil, fmt.Errorf("invalid YAML frontmatter in SKILL.md: %w", err)
			}
			skill.Instructions = string(instructions)
		} else {
			// Malformed frontmatter, treat whole thing as instructions
			skill.Instructions = string(data)
		}
	} else {
		// No frontmatter found
		skill.Instructions = string(data)
	}

	// Fallback name if frontmatter is missing or didn't provide one
	if skill.Manifest.Name == "" {
		skill.Manifest.Name = filepath.Base(skillDir)
	}

	return skill, nil
}
