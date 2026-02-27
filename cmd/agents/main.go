package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dewitt/agents/pkg/sdk"
	"github.com/spf13/cobra"
)

var promptFlag string
var planFlag bool
var resumeFlag bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Prints the current global configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := sdk.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("Global Configuration:")
		fmt.Printf("  Model: %s\n", cfg.Model)
		
		path, err := sdk.DefaultConfigPath()
		if err == nil {
			fmt.Printf("\nStored at: %s\n", path)
		}
	},
}

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

			if planFlag {
				fullPrompt = "[SYSTEM: You are in PLAN MODE. You must strictly act as a read-only architectural advisor. Under NO circumstances should you use tools to write files, execute bash commands, or alter git state. Only use tools to read and list files.]\n\nUser: " + fullPrompt
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
		if err := launchInteractiveShell(planFlag, resumeFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&promptFlag, "prompt", "p", "", "Run a single-shot prompt and exit")
	rootCmd.Flags().BoolVar(&planFlag, "plan", false, "Start the agent in read-only plan mode")
	rootCmd.Flags().BoolVar(&resumeFlag, "resume", false, "Resume the last interactive session")
	rootCmd.AddCommand(configCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
