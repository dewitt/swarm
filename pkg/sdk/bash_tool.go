package sdk

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"google.golang.org/adk/tool"
)

type BashExecuteArgs struct {
	Command string `json:"command"`
	Dir     string `json:"dir,omitempty"`
}

type BashExecuteResult struct {
	Success  bool   `json:"success"`
	Output   string `json:"output,omitempty"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
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

	cmd := exec.Command("bash", "-c", args.Command)
	cmd.Dir = dir

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

	err := cmd.Wait()
	wg.Wait()

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
