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
		{
			ID:          "scenario_2",
			Name:        "Unfamiliar API Migration",
			FixturePath: filepath.Join(root, "eval", "fixtures", "scenario_2_migration"),
			Prompt:      "Migrate main.py to use `httpx` with `async/await`. Also update requirements.txt to reflect this change. You must read the web documentation for httpx if you don't know the exact syntax.",
			Rubric:      "Did the agent successfully replace all `requests` logic with asynchronous `httpx` logic (e.g. `async with httpx.AsyncClient()`)? Does the code still correctly fetch the URLs? Is `requests` completely removed from main.py and requirements.txt?",
		},
		{
			ID:          "scenario_3",
			Name:        "The Security Audit",
			FixturePath: filepath.Join(root, "eval", "fixtures", "scenario_3_security"),
			Prompt:      "I am the CTO. Have a Security Expert agent audit server.js and fix any critical vulnerabilities they find.",
			Rubric:      "Did the agent identify the SQL injection and fix it by parameterizing the query? Did the agent identify the hardcoded JWT secret and move it to an environment variable fallback (e.g., process.env.JWT_SECRET)? If both vulnerabilities are fixed, the test passes.",
		},
		{
			ID:          "scenario_4",
			Name:        "Git-Native PR Review",
			FixturePath: filepath.Join(root, "eval", "fixtures", "scenario_4_pr"),
			Prompt:      "Act as a strict code reviewer. Review the diff on this branch against main. Leave a markdown file REVIEW.md containing your critique.",
			Rubric:      "Did the agent successfully identify the semantic bug in the refactor diff (subtracting the percentage instead of the calculated amount)? Was the critique written to REVIEW.md?",
		},
		{
			ID:          "scenario_5",
			Name:        "The Logic Bug",
			FixturePath: filepath.Join(root, "eval", "fixtures", "scenario_5_logic"),
			Prompt:      "Users are reporting the Fibonacci generator returns 0 for everything, but the tests are passing. Figure out why and fix both the code and the tests.",
			Rubric:      "Did the agent see that the tests were a false positive? Did the agent successfully fix the logic in main.go (changing `seq := []int{0, 0}` to `seq := []int{0, 1}`) AND update the tests in main_test.go to match the true Fibonacci sequence?",
		},
	}, nil
}
