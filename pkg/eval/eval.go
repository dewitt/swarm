package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dewitt/swarm/pkg/sdk"
	"google.golang.org/genai"
)

// Scenario represents a single end-to-end evaluation
type Scenario struct {
	ID                    string
	Name                  string
	FixturePath           string
	Prompt                string
	Rubric                string
	RequiresSystemSandbox bool
}

// Result represents the outcome of an evaluation
type Result struct {
	ScenarioName string
	Passed       bool
	Reasoning    string
	Trajectory   string
}

// Evaluator is the engine that runs test scenarios
type Evaluator struct {
	judgeModel *genai.Client
}

// NewEvaluator creates a new testing evaluator
func NewEvaluator(apiKey string) (*Evaluator, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, err
	}
	return &Evaluator{
		judgeModel: client,
	}, nil
}

// copyDir recursively copies a directory tree
func copyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// RunOption configures a scenario run
type RunOption func(*runOptions)

type runOptions struct {
	progressCallback func(sdk.ObservableEvent)
}

// WithProgress defines a callback that will receive telemetry events as the scenario runs
func WithProgress(cb func(sdk.ObservableEvent)) RunOption {
	return func(opts *runOptions) {
		opts.progressCallback = cb
	}
}

func (e *Evaluator) Run(ctx context.Context, s Scenario, opts ...RunOption) (*Result, error) {
	options := runOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	// 1. Create temporary sandbox
	sandbox, err := os.MkdirTemp("", "swarm-eval-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(sandbox)

	// Copy standard fixture in sandbox
	if err := copyDir(s.FixturePath, sandbox); err != nil {
		return nil, fmt.Errorf("failed to copy fixture: %w", err)
	}

	// Make sure we have the skills directory copied over
	repoRoot, err := getRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to locate repo root: %w", err)
	}
	skillsSrc := filepath.Join(repoRoot, "skills")
	if err := copyDir(skillsSrc, filepath.Join(sandbox, "skills")); err != nil {
		return nil, fmt.Errorf("failed to copy skills: %w", err)
	}

	// Track the current working directory to restore it gracefully
	originalWd, _ := os.Getwd()

	// Capture go caches to prevent massive CPU thrashing during evaluation
	// when HOME is temporarily redirected to the sandbox. Without this,
	// every scenario rebuilds the standard library and go-lint caches from scratch.
	origHome, hasHome := os.LookupEnv("HOME")
	origGocache, hasGocache := os.LookupEnv("GOCACHE")
	origGomodcache, hasGomodcache := os.LookupEnv("GOMODCACHE")
	origGolangci, hasGolangci := os.LookupEnv("GOLANGCI_LINT_CACHE")

	if out, err := exec.Command("go", "env", "GOCACHE", "GOMODCACHE").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) >= 2 {
			os.Setenv("GOCACHE", lines[0])
			os.Setenv("GOMODCACHE", lines[1])
		}
	}
	if !hasGolangci {
		if cacheDir, err := os.UserCacheDir(); err == nil {
			os.Setenv("GOLANGCI_LINT_CACHE", filepath.Join(cacheDir, "golangci-lint"))
		}
	}

	defer func() {
		if hasGocache {
			os.Setenv("GOCACHE", origGocache)
		} else {
			os.Unsetenv("GOCACHE")
		}
		if hasGomodcache {
			os.Setenv("GOMODCACHE", origGomodcache)
		} else {
			os.Unsetenv("GOMODCACHE")
		}
		if hasGolangci {
			os.Setenv("GOLANGCI_LINT_CACHE", origGolangci)
		} else {
			os.Unsetenv("GOLANGCI_LINT_CACHE")
		}
		if hasHome {
			os.Setenv("HOME", origHome)
		} else {
			os.Unsetenv("HOME")
		}
	}()

	// Ensure execution uses the sandbox HOME and CWD
	os.Setenv("HOME", sandbox)
	os.Chdir(sandbox)
	defer os.Chdir(originalWd)

	// Run fixture-specific setup script if it exists
	if _, err := os.Stat("setup.sh"); err == nil {
		setupCtx, cancelSetup := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancelSetup()
		cmd := exec.CommandContext(setupCtx, "bash", "setup.sh")
		cmd.Dir = sandbox
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("fixture setup failed:\n%s\n%w", string(out), err)
		}
	}

	// 3. Instantiate Swarm Engine in the sandbox
	cfg := sdk.SwarmConfig{
		Debug:       true,
		DatabaseURI: "file::memory:?cache=shared",
	}
	instance, err := sdk.NewSwarm(cfg)
	if err != nil {
		return nil, err
	}

	respChan, err := instance.Chat(ctx, s.Prompt)
	if err != nil {
		return nil, err
	}

	// Drain telemetry
	var trajectory string
	for event := range respChan {
		if options.progressCallback != nil {
			options.progressCallback(event)
		}
		if event.Error != nil {
			trajectory += fmt.Sprintf("[%s] %s: ERROR: %v\n", event.AgentName, event.State, event.Error)
		} else if event.FinalContent != "" {
			trajectory += fmt.Sprintf("[%s] %s: %s\n", event.AgentName, event.State, event.FinalContent)
		} else if event.ToolName != "" {
			trajectory += fmt.Sprintf("[%s] %s: Tool %s(%v)\n", event.AgentName, event.State, event.ToolName, event.ToolArgs)
		} else if event.Thought != "" {
			trajectory += fmt.Sprintf("[%s] %s: %s\n", event.AgentName, event.State, event.Thought)
		}
	}

	// 4. Capture Sandbox Diff (For simplicity, we check if there are changes)
	// Or we just feed the Trajectory to the LLM Judge

	// 5. Judge with LLM
	evalPrompt := fmt.Sprintf(`You are an expert autonomous agent evaluator. 
Review the following execution trace of an agent completing a software task.

SCENARIO: %s
TASK: %s
RUBRIC: %s

TRAJECTORY:
%s

Analyze the trace against the rubric. Output a JSON object exactly matching this schema:
{"passed": true|false, "reasoning": "your detailed explanation why the agent passed/failed"}
`, s.Name, s.Prompt, s.Rubric, trajectory)

	judgeResp, err := e.judgeModel.Models.GenerateContent(ctx, "gemini-2.5-pro", genai.Text(evalPrompt), &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	})
	if err != nil {
		return nil, fmt.Errorf("judge failed: %w", err)
	}

	if len(judgeResp.Candidates) == 0 || len(judgeResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("judge returned empty response")
	}

	var evalResult struct {
		Passed    bool   `json:"passed"`
		Reasoning string `json:"reasoning"`
	}

	responseText := judgeResp.Candidates[0].Content.Parts[0].Text
	if err := json.Unmarshal([]byte(responseText), &evalResult); err != nil {
		return nil, fmt.Errorf("judge returned malformed json: %s", responseText)
	}

	return &Result{
		ScenarioName: s.Name,
		Passed:       evalResult.Passed,
		Reasoning:    evalResult.Reasoning,
		Trajectory:   trajectory,
	}, nil
}
