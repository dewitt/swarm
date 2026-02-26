package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/dewitt/agents/pkg/sdk"
)

var promptFlag string

var rootCmd = &cobra.Command{
	Use:   "agents",
	Short: "A sophisticated CLI for managing, building, and deploying AI agents.",
	Long: `agents is an advanced, framework-agnostic command-line interface.
It helps developers quickly scaffold, test, and deploy AI agents into native ecosystems.
When run without arguments, it launches a persistent, interactive terminal session.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check for piped input
		var pipedData string
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			b, err := io.ReadAll(os.Stdin)
			if err == nil {
				pipedData = string(b)
			}
		}

		if promptFlag != "" || pipedData != "" {
			fullPrompt := promptFlag
			if pipedData != "" {
				if fullPrompt != "" {
					fullPrompt = fullPrompt + "\n\nContext:\n" + pipedData
				} else {
					fullPrompt = pipedData
				}
			}

			manager := sdk.NewManager()
			ch, err := manager.Chat(context.Background(), fullPrompt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(<-ch)
			return
		}

		// Launch the interactive Bubble Tea shell
		if err := launchInteractiveShell(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&promptFlag, "prompt", "p", "", "Run a single-shot prompt and exit")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
