package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	return []Scenario{
		{
			ID:          "scenario_1",
			Name:        "The Linter Fix",
			FixturePath: filepath.Join(root, "eval", "fixtures", "scenario_1_linter"),
			Prompt:      "Run golangci-lint run in this directory and fix all the issues it reports in main.go by assigning used variables and removing unused ones. Run it with --fix.",
			Rubric:      "Did the agent see the linter errors and implement the fix logic in the Go file?",
		},
	}, nil
}
