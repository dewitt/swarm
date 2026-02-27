package main

import (
	"fmt"
	"testing"

	"github.com/dewitt/agents/pkg/sdk"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSnapshotUI(t *testing.T) {
	// 1. Instantiate the model
	m := initialModel(false, false)

	// 2. Simulate a terminal window size (e.g., 80x24 standard terminal)
	// This is critical because the View() function relies on m.width and m.height
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(model)

	// Simulate typing /skills
	for _, r := range "/skills" {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(model)
	}

	// Simulate pressing Enter
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)

	// 3. Render the view
	rawView := m.View()
	fmt.Println(rawView)
}

func TestSnapshotModelList(t *testing.T) {
	m := initialModel(false, false)

	// 1. Simulate size
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(model)

	// 2. Simulate entering stateModelList and loading fake models
	m.state = stateModelList
	modelsMsg := modelsLoadedMsg{
		models: []sdk.ModelInfo{
			{Name: "gemini-2.5-flash", DisplayName: "Gemini 2.5 Flash", Description: "Fast and versatile"},
			{Name: "gemini-2.5-pro", DisplayName: "Gemini 2.5 Pro", Description: "Best performance"},
			{Name: "gemini-3.1-pro-preview", DisplayName: "Gemini 3.1 Pro Preview", Description: "Experimental"},
		},
		err: nil,
	}

	newModel, _ = m.Update(modelsMsg)
	m = newModel.(model)

	// 3. Render
	rawView := m.View()

	fmt.Println("=== START TUI MODEL LIST SNAPSHOT (80x24) ===")
	fmt.Println(rawView)
	fmt.Println("=== END TUI MODEL LIST SNAPSHOT ===")
}

func TestSnapshotFileAutocomplete(t *testing.T) {
	m := initialModel(false, false)
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(model)

	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	m = newModel.(model)

	rawView := m.View()
	fmt.Println("=== START TUI FILE AUTOCOMPLETE SNAPSHOT (80x24) ===")
	fmt.Println(rawView)
	fmt.Println("=== END TUI FILE AUTOCOMPLETE SNAPSHOT ===")
}
