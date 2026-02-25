package main

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSnapshotUI(t *testing.T) {
	// 1. Instantiate the model
	m := initialModel()

	// 2. Simulate a terminal window size (e.g., 80x24 standard terminal)
	// This is critical because the View() function relies on m.width and m.height
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(model)

	// 3. Render the view
	rawView := m.View()

	// Print the output so the agent can inspect it in the test logs
	fmt.Println("=== START TUI SNAPSHOT (80x24) ===")
	fmt.Println(rawView)
	fmt.Println("=== END TUI SNAPSHOT ===")
}
