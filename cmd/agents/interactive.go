package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"github.com/dewitt/agents/pkg/sdk"
)

// Define styles for rich text polish as per docs/design/02-cli-ux.md
var (
	promptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	agentMsgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
)

type model struct {
	textInput textinput.Model
	messages  []string
	manager   sdk.AgentManager
	err       error
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Ask the Router Agent something..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return model{
		textInput: ti,
		messages:  []string{"Welcome to Agents! I'm the internal Router Agent."},
		manager:   sdk.NewManager(),
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			input := m.textInput.Value()
			if input != "" {
				m.messages = append(m.messages, promptStyle.Render("> ")+input)
				
				// Call the SDK to handle the business logic
				ch, err := m.manager.Chat(context.Background(), input)
				if err != nil {
					m.messages = append(m.messages, "Error: "+err.Error())
				} else {
					// In a real implementation we would stream this cleanly,
					// but for Phase 1 we just block on the stub response.
					resp := <-ch
					m.messages = append(m.messages, agentMsgStyle.Render("[Router] ")+resp)
				}
				
				m.textInput.SetValue("")
			}
		}

	case error:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var s strings.Builder
	
	s.WriteString("\n")
	for _, msg := range m.messages {
		s.WriteString(msg)
		s.WriteString("\n\n")
	}

	s.WriteString(m.textInput.View())
	s.WriteString("\n\n(esc or ctrl+c to quit)\n")
	
	return s.String()
}

func launchInteractiveShell() error {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("could not start interactive shell: %w", err)
	}
	return nil
}
