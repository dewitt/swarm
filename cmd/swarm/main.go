package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dewitt/swarm/pkg/sdk"
	"github.com/spf13/cobra"
)

var promptFlag string
var planFlag bool
var resumeFlag bool
var trajectoryFlag bool
var explainFlag bool

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
	Use:   "swarm",
	Short: "A sophisticated CLI for orchestrating AI swarms.",
	Long: `swarm is an advanced, framework-agnostic command-line interface.
It helps developers quickly scaffold, test, and deploy AI swarms into native ecosystems.
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

			var swarm sdk.Swarm
			swarm, err := sdk.NewSwarm()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if trajectoryFlag || explainFlag {
				swarm.SetDebug(true)
			}

			ch, err := swarm.Chat(cmd.Context(), fullPrompt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			var lastTrajectory string
			for event := range ch {
				if event.State == sdk.AgentStateComplete && event.AgentName == "Swarm" {
					lastTrajectory = event.FinalContent
					if trajectoryFlag {
						fmt.Println(event.FinalContent)
					}
					continue
				}

				if event.State == sdk.AgentStateComplete && event.FinalContent != "" && !trajectoryFlag {
					fmt.Printf("[%s] %s\n", event.AgentName, event.FinalContent)
				} else if event.State == sdk.AgentStateError {
					if event.Error != nil {
						fmt.Fprintf(os.Stderr, "Error: %s\n", event.Error.Error())
					} else if event.FinalContent != "" {
						fmt.Fprintf(os.Stderr, "Error: %s\n", event.FinalContent)
					}
				} else if !trajectoryFlag {
					if event.State == sdk.AgentStateThinking && event.Thought != "" {
						fmt.Fprintf(os.Stderr, "[%s] 🤔 %s\n", event.AgentName, event.Thought)
					} else if event.State == sdk.AgentStateExecuting && event.ObserverSummary != "" {
						fmt.Fprintf(os.Stderr, "[%s] 💡 %s\n", event.AgentName, event.ObserverSummary)
					} else if event.State == sdk.AgentStateExecuting && event.ToolName != "" {
						fmt.Fprintf(os.Stderr, "[%s] 🛠️  Executing: %s\n", event.AgentName, event.ToolName)
					} else if event.State == sdk.AgentStateWaiting && event.ObserverSummary != "" {
						fmt.Fprintf(os.Stderr, "[%s] 👀 %s\n", event.AgentName, event.ObserverSummary)
					}
				}
			}

			if explainFlag && lastTrajectory != "" {
				var traj sdk.Trajectory
				if err := json.Unmarshal([]byte(lastTrajectory), &traj); err == nil {
					explanation, err := swarm.Explain(context.Background(), traj)
					if err != nil {
						fmt.Fprintf(os.Stderr, "\nExplanation failed: %v\n", err)
					} else {
						fmt.Printf("\n--- Swarm Explanation ---\n%s\n", explanation)
					}
				}
			}
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
	// Suppress standard log output so underlying libraries (like genai) don't corrupt the TUI
	log.SetOutput(io.Discard)

	rootCmd.Flags().StringVarP(&promptFlag, "prompt", "p", "", "Run a single-shot prompt and exit")
	rootCmd.Flags().BoolVar(&planFlag, "plan", false, "Start the agent in read-only plan mode")
	rootCmd.Flags().BoolVar(&resumeFlag, "resume", false, "Resume the last interactive session")
	rootCmd.PersistentFlags().BoolVar(&trajectoryFlag, "trajectory", false, "Output the full swarm trajectory JSON to stdout instead of the response")
	rootCmd.Flags().BoolVar(&explainFlag, "explain", false, "Provide a human-readable explanation of the swarm trajectory")
	rootCmd.AddCommand(configCmd)
}
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupt received, shutting down gracefully to save trajectory...")
		cancel()
	}()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
