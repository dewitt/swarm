package sdk

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// === Builtin Tools ===

type ListFilesArgs struct {
	Dir       string `json:"dir"`
	Recursive bool   `json:"recursive,omitempty"`
}

type ListFilesResult struct {
	Files []string `json:"files"`
	Error string   `json:"error,omitempty"`
}

func listLocalFiles(ctx tool.Context, args ListFilesArgs) (ListFilesResult, error) {
	if args.Dir == "" {
		args.Dir = "."
	}
	var files []string

	if args.Recursive {
		err := filepath.Walk(args.Dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors (like permission denied)
			}
			// Skip hidden directories like .git
			if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if path != args.Dir {
				name := strings.TrimPrefix(path, args.Dir+"/")
				if info.IsDir() {
					name += "/"
				}
				files = append(files, name)
			}
			return nil
		})
		if err != nil {
			return ListFilesResult{Error: err.Error()}, nil
		}
	} else {
		entries, err := os.ReadDir(args.Dir)
		if err != nil {
			return ListFilesResult{Error: err.Error()}, nil
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			files = append(files, name)
		}
	}

	// Limit to prevent context window explosion
	if len(files) > 1000 {
		files = append(files[:1000], fmt.Sprintf("... and %d more. Use grep_search for specific queries.", len(files)-1000))
	}

	return ListFilesResult{Files: files}, nil
}

type ReadFileArgs struct {
	Path string `json:"path"`
}
type ReadFileResult struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func readLocalFile(ctx tool.Context, args ReadFileArgs) (ReadFileResult, error) {
	b, err := os.ReadFile(args.Path)
	if err != nil {
		return ReadFileResult{Error: err.Error()}, nil
	}
	return ReadFileResult{Content: string(b)}, nil
}

type GrepArgs struct {
	Pattern string `json:"pattern"`
	Dir     string `json:"dir"`
}
type GrepResult struct {
	Matches []string `json:"matches"`
	Error   string   `json:"error,omitempty"`
}

func grepSearch(ctx tool.Context, args GrepArgs) (GrepResult, error) {
	if args.Dir == "" {
		args.Dir = "."
	}
	cmd := exec.CommandContext(ctx, "grep", "-r", "-l", args.Pattern, args.Dir)
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return GrepResult{Matches: []string{}}, nil
		}
		return GrepResult{Error: err.Error()}, nil
	}
	matches := strings.Split(strings.TrimSpace(string(out)), "\n")
	return GrepResult{Matches: matches}, nil
}

type WriteFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type WriteFileResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func writeLocalFile(ctx tool.Context, args WriteFileArgs) (WriteFileResult, error) {
	dir := filepath.Dir(args.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return WriteFileResult{Success: false, Error: err.Error()}, nil
	}
	if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
		return WriteFileResult{Success: false, Error: err.Error()}, nil
	}
	return WriteFileResult{Success: true}, nil
}

type WebFetchArgs struct {
	URL string `json:"url"`
}
type WebFetchResult struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func webFetch(ctx tool.Context, args WebFetchArgs) (WebFetchResult, error) {
	resp, err := http.Get(args.URL)
	if err != nil {
		return WebFetchResult{Error: err.Error()}, nil
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return WebFetchResult{Error: err.Error()}, nil
	}
	return WebFetchResult{Content: string(b)}, nil
}

type GoogleSearchArgs struct {
	Query string `json:"query"`
}
type GoogleSearchResult struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func googleSearchFunc(ctx tool.Context, args GoogleSearchArgs) (GoogleSearchResult, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return GoogleSearchResult{Error: "GOOGLE_API_KEY is not set"}, nil
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return GoogleSearchResult{Error: err.Error()}, nil
	}
	resp, err := client.Models.GenerateContent(context.Background(), "gemini-2.5-flash", genai.Text(args.Query), &genai.GenerateContentConfig{Tools: []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}}})
	if err != nil {
		return GoogleSearchResult{Error: err.Error()}, nil
	}
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
		return GoogleSearchResult{Response: resp.Candidates[0].Content.Parts[0].Text}, nil
	}
	return GoogleSearchResult{Response: "No results found"}, nil
}

// === Bash Tool ===

type BashExecuteArgs struct {
	Command      string `json:"command" description:"The exact bash command to run."`
	Dir          string `json:"dir,omitempty" description:"The working directory to run the command in."`
	IsBackground bool   `json:"is_background,omitempty" description:"Set to true for long-running servers or watchers. The tool will return the PGID and detach."`
}

type BashExecuteResult struct {
	Success  bool   `json:"success"`
	Output   string `json:"output,omitempty"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
	PGID     int    `json:"pgid,omitempty"`
}

func bashExecuteTool(ctx tool.Context, args BashExecuteArgs) (BashExecuteResult, error) {
	if os.Getenv("AGENTS_DRY_RUN") == "true" {
		return BashExecuteResult{
			Success:  true,
			Output:   fmt.Sprintf("[DRY RUN] Would have executed: `%s` in dir: %s", args.Command, args.Dir),
			ExitCode: 0,
		}, nil
	}

	dir := args.Dir
	if dir == "" {
		dir = "."
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	cmd.Dir = dir

	if args.IsBackground {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// Setup telemetry if available in context
	var telemetry chan<- string
	if val := ctx.Value(telemetryContextKey{}); val != nil {
		if ch, ok := val.(chan<- string); ok {
			telemetry = ch
		}
	}

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	var stdoutBuf, stderrBuf bytes.Buffer

	if err := cmd.Start(); err != nil {
		return BashExecuteResult{Success: false, Error: err.Error(), ExitCode: -1}, nil
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout and stderr
	stream := func(r io.Reader, buf *bytes.Buffer) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			buf.WriteString(line + "\n")
			if telemetry != nil {
				telemetry <- line
			}
		}
	}

	go stream(stdoutPipe, &stdoutBuf)
	go stream(stderrPipe, &stderrBuf)

	var err error
	if args.IsBackground {
		// Small sleep to catch immediate errors (e.g. command not found, or immediate crash)
		time.Sleep(500 * time.Millisecond)

		// Check if it's still running
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			err = cmd.Wait() // Populate exit code
			wg.Wait()
		} else {
			// It's still running! Return PGID.
			pgid, _ := syscall.Getpgid(cmd.Process.Pid)
			return BashExecuteResult{
				Success: true,
				Output:  fmt.Sprintf("Process started in background. PGID: %d\nOutput so far:\n%s", pgid, stdoutBuf.String()),
				PGID:    pgid,
			}, nil
		}
	} else {
		err = cmd.Wait()
		wg.Wait()
	}

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	combinedOutput := stdoutBuf.String()
	if stderrBuf.Len() > 0 {
		if combinedOutput != "" {
			combinedOutput += "\n"
		}
		combinedOutput += "STDERR:\n" + stderrBuf.String()
	}

	result := BashExecuteResult{
		Success:  exitCode == 0,
		Output:   combinedOutput,
		ExitCode: exitCode,
	}

	if err != nil && exitCode == -1 {
		result.Error = err.Error()
	}

	return result, nil
}

// === Git Tools ===

// runGitCommand is a helper to run standard Git shell commands.
func runGitCommand(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	branch, err := runGitCommand(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return GitInfo{}, err
	}

	status, err := runGitCommand(ctx, dir, "status", "--porcelain")
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := runGitCommand(ctx, dir, "log", "-n", fmt.Sprint(n), "--pretty=format:%s")
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
	_, err := runGitCommand(ctx, args.Dir, "add", ".")
	if err != nil {
		return GitCommitResult{Success: false, Error: err.Error()}, nil
	}

	out, err := runGitCommand(ctx, args.Dir, "commit", "-m", args.Message)
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

	out, err := runGitCommand(ctx, args.Dir, "push")
	if err != nil {
		return GitPushResult{Success: false, Error: err.Error()}, nil
	}

	return GitPushResult{Success: true, Output: out}, nil
}

// === Replan Tool ===

type ReplanArgs struct {
	Reason      string `json:"reason"`      // Why is a replan necessary?
	Discoveries string `json:"discoveries"` // What new information has been found?
}

type ReplanResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func requestReplan(ctx tool.Context, args ReplanArgs) (ReplanResult, error) {
	// The tool doesn't do much on its own; the Engine loop listens for the result.
	return ReplanResult{
		Success: true,
		Message: "Replan requested. The engine will pivot after this span concludes.",
	}, nil
}
