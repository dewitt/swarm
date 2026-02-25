package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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
	errorColor     = lipgloss.Color("#FF4500") // Orange Red

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

	statusBarStyle = lipgloss.NewStyle().
			Foreground(tipColor).
			Background(lipgloss.Color("#1E1E1E")).
			Width(100).
			Padding(0, 1)

	// Layout padding
	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("#333333")).
			PaddingTop(1).
			PaddingBottom(1)
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
	viewport  viewport.Model
	messages  []string
	manager   sdk.AgentManager
	err       error
	width     int
	height    int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Type your message or /help"
	ti.Focus()
	ti.CharLimit = 500
	ti.Prompt = promptStyle.Render("> ")

	vp := viewport.New(0, 0)
	vp.YPosition = 0

	// The first "message" in our history is the splash screen and tips
	welcomeScreen := fmt.Sprintf("%s\n\n%s", logoStyle.Render(splashLogo), tipStyle.Render(initialTips))

	return model{
		textInput: ti,
		viewport:  vp,
		messages:  []string{welcomeScreen},
		manager:   sdk.NewManager(),
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

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
					m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Error: "+err.Error()))
				} else {
					// In Phase 2, this will stream. For now, it blocks on the stub.
					resp := <-ch
					m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+resp)
				}

				// Update viewport content and scroll to bottom
				m.updateViewport()
			}
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			// Pass navigation keys directly to the viewport so the user can scroll
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update input width
		m.textInput.Width = msg.Width - 4 // Account for prompt and padding

		// Update viewport dimensions
		// Height minus input box height (approx 4 lines) minus status bar (1 line)
		inputHeight := 4
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - inputHeight - 1

		statusBarStyle.Width(msg.Width)

		m.updateViewport()

	case error:
		m.err = msg
		return m, nil
	}

	m.textInput, tiCmd = m.textInput.Update(msg)
	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *model) updateViewport() {
	var s strings.Builder
	for _, msg := range m.messages {
		s.WriteString(msg)
		s.WriteString("\n\n")
	}
	m.viewport.SetContent(s.String())
	m.viewport.GotoBottom()
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// 1. Render Viewport (History)
	vpView := m.viewport.View()

	// 2. Render Input Box
	inputView := inputBoxStyle.Render(m.textInput.View())

	// 3. Render Status Bar
	statusContent := fmt.Sprintf(" %-20s %-30s %20s ", "~\\Agents", "local mode (see /docs)", "auto")
	statusView := statusBarStyle.Render(statusContent)

	// Combine components
	return lipgloss.JoinVertical(lipgloss.Left, vpView, inputView, statusView)
}

func launchInteractiveShell() error {
	// tea.WithAltScreen() puts the application into full-screen mode, hiding the terminal's native history
	// and taking over the entire window layout, similar to vim or top.
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("could not start interactive shell: %w", err)
	}
	return nil
}
