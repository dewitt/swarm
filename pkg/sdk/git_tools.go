package sdk

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

type GitInfo struct {
	Branch   string
	Modified bool
}

func GetGitInfo(dir string) (GitInfo, error) {
	if dir == "" {
		dir = "."
	}
	branch, err := runGitCommand(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return GitInfo{}, err
	}

	status, err := runGitCommand(dir, "status", "--porcelain")
	modified := false
	if err == nil && len(strings.TrimSpace(status)) > 0 {
		modified = true
	}

	return GitInfo{Branch: strings.TrimSpace(branch), Modified: modified}, nil
}

func GetRecentCommits(dir string, n int) ([]string, error) {
	if dir == "" {
		dir = "."
	}
	out, err := runGitCommand(dir, "log", "-n", fmt.Sprint(n), "--pretty=format:%s")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	var commits []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			commits = append(commits, l)
		}
	}
	return commits, nil
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
	if os.Getenv("AGENTS_DRY_RUN") == "true" {
		return GitCommitResult{Success: true, Output: fmt.Sprintf("[DRY RUN] Would have committed in %s with message: %s", args.Dir, args.Message)}, nil
	}

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
	if os.Getenv("AGENTS_DRY_RUN") == "true" {
		return GitPushResult{Success: true, Output: fmt.Sprintf("[DRY RUN] Would have pushed in %s", args.Dir)}, nil
	}

	if args.Dir == "" {
		args.Dir = "."
	}

	out, err := runGitCommand(args.Dir, "push")
	if err != nil {
		return GitPushResult{Success: false, Error: err.Error()}, nil
	}

	return GitPushResult{Success: true, Output: out}, nil
}
