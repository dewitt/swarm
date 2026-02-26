package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "agents",
	Short: "A sophisticated CLI for managing, building, and deploying AI agents.",
	Long: `agents is an advanced, framework-agnostic command-line interface.
It helps developers quickly scaffold, test, and deploy AI agents into native ecosystems.
When run without arguments, it launches a persistent, interactive terminal session.`,
	Run: func(cmd *cobra.Command, args []string) {		// Launch the interactive Bubble Tea shell
		if err := launchInteractiveShell(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
