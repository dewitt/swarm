package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dewitt/agents/pkg/sdk"
)

var (
	// Brand Colors
	primaryColor   = lipgloss.Color("#FF7F50") // Coral (similar to Claude Code's accent)
	secondaryColor = lipgloss.Color("#8A2BE2") // Purple (Gemini vibe)
	tipColor       = lipgloss.Color("#888888") // Dim Gray
	agentColor     = lipgloss.Color("#00FA9A") // Medium Spring Green
	errorColor     = lipgloss.Color("#FF4500") // Orange Red
	bgDark         = lipgloss.Color("#111111") // Very dark gray for boxes

	// Styles
	logoStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	infoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(1, 2)

	promptStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	agentMsgStyle = lipgloss.NewStyle().
			Foreground(agentColor).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Background(lipgloss.Color("#222222")).
			Padding(0, 1)

	// Layout padding
	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("#333333")).
			PaddingTop(1).
			PaddingBottom(1)
)

const splashLogo = `
   ___                    __      
  / _ |  ___  ___  ___  / /____ 
 / __ | / _ \/ -_)/ _ \/ __(_-<
/_/ |_| \_, /\__//_//_/\__/___/
       /___/                    
`

const initialTips = `Recent activity
1m ago    Initialized project
8m ago    Updated memory
2d ago    Added new skills
... /resume for more

What's new
/agents to create subagents
/docs for API references
ctrl+c to background or exit
`

// Message struct for async SDK responses
type responseMsg struct {
	text string
	err  error
}

type model struct {
	textInput textinput.Model
	viewport  viewport.Model
	spinner   spinner.Model
	messages  []string
	manager   sdk.AgentManager
	err       error
	width     int
	height    int
	loading   bool // Is the agent thinking?
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Type your message or /help"
	ti.Focus()
	ti.CharLimit = 500
	ti.Prompt = promptStyle.Render("> ")

	vp := viewport.New(0, 0)
	vp.YPosition = 0

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	// Create a beautiful, split-pane splash screen
	leftBox := boxStyle.Render(fmt.Sprintf("%s\n\nWelcome back, Developer!", logoStyle.Render(splashLogo)))
	rightBox := infoBoxStyle.Render(initialTips)
	welcomeScreen := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, "  ", rightBox)

	return model{
		textInput: ti,
		viewport:  vp,
		spinner:   s,
		messages:  []string{welcomeScreen},
		manager:   sdk.NewManager(),
		loading:   false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
		cmds  []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.loading {
				// Don't accept input while loading
				return m, nil
			}

			input := m.textInput.Value()
			if input != "" {
				// Record the user's input
				m.messages = append(m.messages, promptStyle.Render("> ")+input)

				// Clear the input field and set loading state
				m.textInput.SetValue("")
				m.loading = true

				// Update viewport content and scroll to bottom
				m.updateViewport()

				// Launch the async command to call the SDK
				cmds = append(cmds, m.callSDK(input))
			}
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			// Pass navigation keys directly to the viewport so the user can scroll
			m.viewport, vpCmd = m.viewport.Update(msg)
			cmds = append(cmds, vpCmd)
			return m, tea.Batch(cmds...)
		}

	case responseMsg:
		m.loading = false
		if msg.err != nil {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Error: "+msg.err.Error()))
		} else {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+msg.text)
		}
		m.updateViewport()
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			m.spinner, spCmd = m.spinner.Update(msg)
			cmds = append(cmds, spCmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update input width
		m.textInput.Width = msg.Width - 4 // Account for prompt and padding

		// Update viewport dimensions
		// Height minus input box height (approx 4 lines) minus status bar (1 line)
		inputHeight := 3
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - inputHeight - 1

		statusBarStyle.Width(msg.Width)

		m.updateViewport()
		return m, nil

	case error:
		m.err = msg
		return m, nil
	}

	m.textInput, tiCmd = m.textInput.Update(msg)
	cmds = append(cmds, tiCmd)

	if m.loading {
		m.spinner, spCmd = m.spinner.Update(msg)
		cmds = append(cmds, spCmd)
	}

	return m, tea.Batch(cmds...)
}

// callSDK wraps the SDK Chat call in a tea.Cmd so it doesn't block the UI thread.
func (m model) callSDK(input string) tea.Cmd {
	return func() tea.Msg {
		ch, err := m.manager.Chat(context.Background(), input)
		if err != nil {
			return responseMsg{err: err}
		}
		// Block on the channel, but in a background goroutine managed by Bubble Tea
		resp := <-ch
		return responseMsg{text: resp}
	}
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
	// If loading, show the spinner instead of the prompt
	var inputView string
	if m.loading {
		inputView = inputBoxStyle.Render(m.spinner.View() + " Thinking...")
	} else {
		inputView = inputBoxStyle.Render(m.textInput.View())
	}

	// 3. Render Status Bar (Properly aligned)
	leftStatus := "~\\Agents"
	centerStatus := "local mode (see /docs)"
	rightStatus := "auto"

	// Calculate spacing
	w1 := m.width / 3
	w2 := m.width / 3
	w3 := m.width - w1 - w2
	
	p1 := lipgloss.NewStyle().Width(w1).Align(lipgloss.Left).Render(leftStatus)
	p2 := lipgloss.NewStyle().Width(w2).Align(lipgloss.Center).Render(centerStatus)
	p3 := lipgloss.NewStyle().Width(w3).Align(lipgloss.Right).Render(rightStatus)

	statusView := statusBarStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, p1, p2, p3))

	// Combine components
	return lipgloss.JoinVertical(lipgloss.Left, vpView, inputView, statusView)
}

func launchInteractiveShell() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("could not start interactive shell: %w", err)
	}
	return nil
}
