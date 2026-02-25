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
	primaryColor   = lipgloss.Color("#FF7F50") // Coral
	secondaryColor = lipgloss.Color("#4169E1") // Royal Blue
	tipColor       = lipgloss.Color("#666666") 
	agentColor     = lipgloss.Color("#00FA9A")
	errorColor     = lipgloss.Color("#FF4500") // Orange Red
	borderColor    = lipgloss.Color("#333333")
	statusBg       = lipgloss.Color("#1A1A1A")
	statusFg       = lipgloss.Color("#888888")

	// Styles
	appStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderColor)

	logoStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	welcomeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 2).
			MarginRight(2)

	infoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 2)

	promptStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	agentMsgStyle = lipgloss.NewStyle().
			Foreground(agentColor).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(statusFg).
			Background(statusBg).
			Height(1)

	// Layout padding
	inputBoxStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Height(1)
)

const splashLogo = `
   ▄▄▄▄▀ ▄███▄      ▄      ▄▄▄▄▀ ▄▄▄▄▄   
▀▀▀ █    █▀   ▀      █  ▀▀▀ █   █     ▀▄ 
    █    ██▄▄    ██   █     █ ▄  ▀▀▀▀▄   
   █     █▄   ▄▀ █ █  █    █   ▀▄▄▄▄▀    
  ▀      ▀███▀   █  █ █   ▀              
                 █   ██                  
`

const initialTips = `Recent activity
1m ago    Initialized project
8m ago    Updated memory
2d ago    Added new skills

What's new
/agents to create subagents
/docs for API references
ctrl+c to background or exit
`

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
	loading   bool
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

	// Create a beautiful splash screen
	leftBox := welcomeBoxStyle.Render(fmt.Sprintf("%s\n\nWelcome back, Developer!", logoStyle.Render(splashLogo)))
	rightBox := infoBoxStyle.Render(initialTips)
	welcomeScreen := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

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
				return m, nil
			}

			input := m.textInput.Value()
			if input != "" {
				m.messages = append(m.messages, promptStyle.Render("> ")+input)
				m.textInput.SetValue("")
				m.loading = true
				m.updateViewport()
				cmds = append(cmds, m.callSDK(input))
			}
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd
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
		// Account for the outer border
		m.width = msg.Width - 2
		m.height = msg.Height - 2

		m.textInput.Width = m.width - 4
		
		// Viewport height: Height minus input (1) minus status (1) minus prompt/padding
		m.viewport.Width = m.width
		m.viewport.Height = m.height - 3

		m.updateViewport()
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

func (m model) callSDK(input string) tea.Cmd {
	return func() tea.Msg {
		ch, err := m.manager.Chat(context.Background(), input)
		if err != nil {
			return responseMsg{err: err}
		}
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

	// 1. History
	vpView := m.viewport.View()

	// 2. Input
	var inputView string
	if m.loading {
		inputView = inputBoxStyle.Render(m.spinner.View() + " Thinking...")
	} else {
		inputView = inputBoxStyle.Render(m.textInput.View())
	}

	// 3. Status
	w1 := m.width / 3
	w2 := m.width / 3
	w3 := m.width - w1 - w2
	
	p1 := lipgloss.NewStyle().Width(w1).Align(lipgloss.Left).Render(" ~/Agents")
	p2 := lipgloss.NewStyle().Width(w2).Align(lipgloss.Center).Render("local mode")
	p3 := lipgloss.NewStyle().Width(w3).Align(lipgloss.Right).Render("auto ")

	statusView := statusBarStyle.Width(m.width).Render(lipgloss.JoinHorizontal(lipgloss.Top, p1, p2, p3))

	// Assemble Main Body
	mainBody := lipgloss.JoinVertical(lipgloss.Left, vpView, inputView)
	
	// Apply Outer Border to main body
	boxedBody := appStyle.Width(m.width).Height(m.height).Render(mainBody)

	// Final layout: Boxed Body + Status Bar (outside border)
	return lipgloss.JoinVertical(lipgloss.Left, boxedBody, statusView)
}

func launchInteractiveShell() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("could not start interactive shell: %w", err)
	}
	return nil
}
