package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// getRepoRoot attempts to find the root of the sworn repository
// so that fixture paths resolve correctly regardless of where the command is run.
func getRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err == nil && strings.Contains(string(content), "module github.com/dewitt/swarm") {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find swarm repository root")
}

// GetScenarios returns the list of all defined E2E evaluation scenarios.
func GetScenarios() ([]Scenario, error) {
	root, err := getRepoRoot()
	if err != nil {
		return nil, err
	}

	fixturesDir := filepath.Join(root, "eval", "fixtures")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read fixtures directory: %w", err)
	}

	var scenarios []Scenario
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		scenarioDir := filepath.Join(fixturesDir, entry.Name())
		yamlPath := filepath.Join(scenarioDir, "scenario.yaml")

		data, err := os.ReadFile(yamlPath)
		if err != nil {
			// Skip directories that don't have a scenario.yaml
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read %s: %w", yamlPath, err)
		}

		var s struct {
			ID                    string `yaml:"id"`
			Name                  string `yaml:"name"`
			Prompt                string `yaml:"prompt"`
			Rubric                string `yaml:"rubric"`
			RequiresSystemSandbox bool   `yaml:"requires_system_sandbox"`
		}

		if err := yaml.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", yamlPath, err)
		}

		scenarios = append(scenarios, Scenario{
			ID:                    s.ID,
			Name:                  s.Name,
			FixturePath:           scenarioDir,
			Prompt:                strings.TrimSpace(s.Prompt),
			Rubric:                strings.TrimSpace(s.Rubric),
			RequiresSystemSandbox: s.RequiresSystemSandbox,
		})
	}

	if len(scenarios) == 0 {
		return nil, fmt.Errorf("no scenarios found in %s", fixturesDir)
	}

	return scenarios, nil
}
