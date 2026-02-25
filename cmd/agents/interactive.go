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

var (
	// Brand Colors
	primaryColor   = lipgloss.Color("#8A2BE2") // Purple
	secondaryColor = lipgloss.Color("#4169E1") // Royal Blue
	tipColor       = lipgloss.Color("#696969") // Dim Gray
	agentColor     = lipgloss.Color("#00FA9A") // Medium Spring Green
	
	// Styles
	logoStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)
		
	tipStyle = lipgloss.NewStyle().
		Foreground(tipColor).
		Italic(true)
		
	promptStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)
		
	agentMsgStyle = lipgloss.NewStyle().
		Foreground(agentColor).
		Bold(true)
		
	statusBar = lipgloss.NewStyle().
		Foreground(tipColor).
		PaddingTop(1)
)

const splashLogo = `
    ___   ____________   __________
   /   | / ____/ ____/  / | / /_  /
  / /| |/ / __/ __/    /  |/ / / / 
 / ___ / /_/ / /___   / /|  / / /  
/_/  |_\____/_____/  /_/ |_/ /_/   
`

const initialTips = `Tips for getting started:
1. Ask questions, build agents, or run deployments.
2. Be specific for the best results.
3. Type /help for more information.
`

type model struct {
	textInput textinput.Model
	messages  []string
	manager   sdk.AgentManager
	err       error
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Type your message or /help"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80
	ti.Prompt = promptStyle.Render("> ")

	// The first "message" in our history is the splash screen and tips
	welcomeScreen := fmt.Sprintf("%s\n\n%s", logoStyle.Render(splashLogo), tipStyle.Render(initialTips))

	return model{
		textInput: ti,
		messages:  []string{welcomeScreen},
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
				// Record the user's input
				m.messages = append(m.messages, promptStyle.Render("> ")+input)
				
				// Clear the input field while processing
				m.textInput.SetValue("")
				
				// Call the SDK to handle the business logic
				ch, err := m.manager.Chat(context.Background(), input)
				if err != nil {
					m.messages = append(m.messages, "Error: "+err.Error())
				} else {
					// In Phase 2, this will stream. For now, it blocks on the stub.
					resp := <-ch
					m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+resp)
				}
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
	
	// Render history
	for _, msg := range m.messages {
		s.WriteString(msg)
		s.WriteString("\n\n")
	}

	// Render input prompt
	s.WriteString(m.textInput.View())
	
	// Render fake status bar
	s.WriteString(statusBar.Render("~\\Agents        local mode (see /docs)                 auto"))
	s.WriteString("\n")
	
	return s.String()
}

func launchInteractiveShell() error {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("could not start interactive shell: %w", err)
	}
	return nil
}
