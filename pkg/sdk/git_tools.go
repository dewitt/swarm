package sdk

import (
	"bytes"
	"fmt"
	"os/exec"

	"google.golang.org/adk/tool"
)

// runGitCommand is a helper to run standard Git shell commands.
func runGitCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git error: %s (%w)", stderr.String(), err)
	}

	return stdout.String(), nil
}

type GitCommitArgs struct {
	Message string `json:"message"`
	Dir     string `json:"dir"` // Optional
}

type GitCommitResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

func gitCommitTool(ctx tool.Context, args GitCommitArgs) (GitCommitResult, error) {
	if args.Dir == "" {
		args.Dir = "."
	}

	// For an agent, it's usually safest to just add everything they modified
	_, err := runGitCommand(args.Dir, "add", ".")
	if err != nil {
		return GitCommitResult{Success: false, Error: err.Error()}, nil
	}

	out, err := runGitCommand(args.Dir, "commit", "-m", args.Message)
	if err != nil {
		return GitCommitResult{Success: false, Error: err.Error()}, nil
	}

	return GitCommitResult{Success: true, Output: out}, nil
}

type GitPushArgs struct {
	Dir string `json:"dir"` // Optional
}

type GitPushResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

func gitPushTool(ctx tool.Context, args GitPushArgs) (GitPushResult, error) {
	if args.Dir == "" {
		args.Dir = "."
	}

	out, err := runGitCommand(args.Dir, "push")
	if err != nil {
		return GitPushResult{Success: false, Error: err.Error()}, nil
	}

	return GitPushResult{Success: true, Output: out}, nil
}
