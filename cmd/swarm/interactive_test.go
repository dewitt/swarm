package main

import (
	"fmt"
	"testing"

	"github.com/dewitt/swarm/pkg/sdk"

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

func TestHistoryNavigation(t *testing.T) {
	m := initialModel(false, false)
	m.history = []string{"first command", "second command"}
	m.historyIdx = 2

	// 1. Type something unsubmitted
	unsubmitted := "unsubmitted text"
	for _, r := range unsubmitted {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(model)
	}
	if m.textArea.Value() != unsubmitted {
		t.Errorf("expected text area value %q, got %q", unsubmitted, m.textArea.Value())
	}

	// 2. Press Up arrow
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(model)
	if m.textArea.Value() != "second command" {
		t.Errorf("expected text area value %q, got %q", "second command", m.textArea.Value())
	}
	if m.historyIdx != 1 {
		t.Errorf("expected historyIdx 1, got %d", m.historyIdx)
	}
	if m.currentInput != unsubmitted {
		t.Errorf("expected currentInput %q, got %q", unsubmitted, m.currentInput)
	}

	// 3. Press Up arrow again
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(model)
	if m.textArea.Value() != "first command" {
		t.Errorf("expected text area value %q, got %q", "first command", m.textArea.Value())
	}
	if m.historyIdx != 0 {
		t.Errorf("expected historyIdx 0, got %d", m.historyIdx)
	}

	// 4. Press Down arrow
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(model)
	if m.textArea.Value() != "second command" {
		t.Errorf("expected text area value %q, got %q", "second command", m.textArea.Value())
	}
	if m.historyIdx != 1 {
		t.Errorf("expected historyIdx 1, got %d", m.historyIdx)
	}

	// 5. Press Down arrow again to return to unsubmitted
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(model)
	if m.historyIdx != 2 {
		t.Errorf("expected historyIdx 2, got %d", m.historyIdx)
	}
	if m.textArea.Value() != unsubmitted {
		t.Errorf("expected text area value %q, got %q", unsubmitted, m.textArea.Value())
	}
}

func TestGitStatusUpdate(t *testing.T) {
	m := initialModel(false, false)

	// Initial state
	m.gitBranch = "old-branch"
	m.gitModified = false

	// Simulate gitStatusMsg
	newBranch := "new-feature"
	newModified := true
	msg := gitStatusMsg{
		branch:   newBranch,
		modified: newModified,
	}

	newModel, cmd := m.Update(msg)
	m = newModel.(model)

	if m.gitBranch != newBranch {
		t.Errorf("expected gitBranch %q, got %q", newBranch, m.gitBranch)
	}
	if m.gitModified != newModified {
		t.Errorf("expected gitModified %v, got %v", newModified, m.gitModified)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd, got %v", cmd)
	}
}

func TestGitTickUpdate(t *testing.T) {
	m := initialModel(false, false)

	// Simulate gitTickMsg
	msg := gitTickMsg{}

	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected non-nil cmd for gitTickMsg")
	}

	// We expect a batch command containing checkGitStatus and doGitTick
	// We can't easily introspect the batch, but we can at least ensure it's not nil.
}

func TestCheckGitStatus(t *testing.T) {
	cmd := checkGitStatus()
	if cmd == nil {
		t.Fatal("expected non-nil cmd for checkGitStatus")
	}

	msg := cmd()
	if _, ok := msg.(gitStatusMsg); !ok {
		t.Errorf("expected gitStatusMsg, got %T", msg)
	}
}

func TestResponseMsgTriggersGitStatusUpdate(t *testing.T) {
	m := initialModel(false, false)
	msg := responseMsg{text: "some output", isShell: true}

	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected non-nil cmd for responseMsg")
	}

	// We can't easily verify it's checkGitStatus, but we can call it and check its type
	innerMsg := cmd()
	if _, ok := innerMsg.(gitStatusMsg); !ok {
		t.Errorf("expected gitStatusMsg, got %T", innerMsg)
	}
}
