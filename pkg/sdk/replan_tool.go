package sdk

import (
	"google.golang.org/adk/tool"
)

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
