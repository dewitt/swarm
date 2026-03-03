package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dewitt/swarm/pkg/eval"
	"github.com/spf13/cobra"
)

var evalCmd = &cobra.Command{
	Use:   "eval [scenario_id]",
	Short: "Run end-to-end agentic evaluations",
	Long: `Run LLM-as-a-judge end-to-end evaluations on the Swarm CLI.
If no scenario_id is provided, all scenarios will be run.`,
	Run: func(cmd *cobra.Command, args []string) {
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			fmt.Println("Error: GOOGLE_API_KEY environment variable is required to run evaluations.")
			os.Exit(1)
		}

		evaluator, err := eval.NewEvaluator(apiKey)
		if err != nil {
			fmt.Printf("Failed to initialize evaluator: %v\n", err)
			os.Exit(1)
		}

		scenarios, err := eval.GetScenarios()
		if err != nil {
			fmt.Printf("Failed to load scenarios: %v\n", err)
			os.Exit(1)
		}

		var toRun []eval.Scenario
		if len(args) > 0 {
			target := args[0]
			for _, s := range scenarios {
				if s.ID == target {
					toRun = append(toRun, s)
					break
				}
			}
			if len(toRun) == 0 {
				fmt.Printf("Error: Scenario '%s' not found.\n", target)
				os.Exit(1)
			}
		} else {
			toRun = scenarios
		}

		fmt.Printf("Running %d evaluation(s)...\n\n", len(toRun))
		passed := 0
		for _, s := range toRun {
			fmt.Printf("==> Scenario: %s (%s)\n", s.Name, s.ID)

			res, err := evaluator.Run(context.Background(), s)
			if err != nil {
				fmt.Printf("    ERROR: %v\n\n", err)
				continue
			}

			fmt.Printf("    Trajectory:\n")
			lines := strings.Split(res.Trajectory, "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				if len(line) > 120 {
					line = line[:117] + "..."
				}
				fmt.Printf("      %s\n", line)
			}

			if res.Passed {
				fmt.Println("    PASS")
				passed++
			} else {
				fmt.Println("    FAIL")
			}
			fmt.Printf("    Reasoning: %s\n\n", res.Reasoning)
		}

		fmt.Printf("Results: %d/%d passed.\n", passed, len(toRun))
		if passed < len(toRun) {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(evalCmd)
}
