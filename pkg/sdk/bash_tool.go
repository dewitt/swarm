package sdk

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

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

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	combinedOutput := stdout.String()
	if stderr.Len() > 0 {
		if combinedOutput != "" {
			combinedOutput += "\n"
		}
		combinedOutput += "STDERR:\n" + stderr.String()
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
