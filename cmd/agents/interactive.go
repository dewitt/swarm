package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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
 █████╗  ██████╗ ███████╗███╗   ██╗████████╗███████╗
██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝██╔════╝
███████║██║  ███╗█████╗  ██╔██╗ ██║   ██║   ███████╗
██╔══██║██║   ██║██╔══╝  ██║╚██╗██║   ██║   ╚════██║
██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║   ███████║
╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝   ╚══════╝
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

type modelsLoadedMsg struct {
	models []string
	err    error
}

type item string

func (i item) Title() string       { return string(i) }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return string(i) }

type uiState int

const (
	stateChat uiState = iota
	stateModelList
)

type model struct {
	textInput textinput.Model
	viewport  viewport.Model
	spinner   spinner.Model
	listModel list.Model
	messages  []string
	manager   sdk.AgentManager
	err       error
	width     int
	height    int
	loading   bool
	state     uiState
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

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select an LLM Provider"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	// Create a beautiful splash screen
	leftBox := welcomeBoxStyle.Render(fmt.Sprintf("%s\n\nWelcome back, Developer!", logoStyle.Render(splashLogo)))
	rightBox := infoBoxStyle.Render(initialTips)
	welcomeScreen := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	return model{
		textInput: ti,
		viewport:  vp,
		spinner:   s,
		listModel: l,
		messages:  []string{welcomeScreen},
		manager:   sdk.NewManager(),
		loading:   false,
		state:     stateChat,
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

	// If we are in the model list state, hijack the keys
	if m.state == stateModelList {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			// Let it pass through to the main handler below to update sizes
		case modelsLoadedMsg:
			// Let it pass through to the main handler below
		case tea.KeyMsg:
			if msg.Type == tea.KeyEsc {
				m.state = stateChat
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				if i, ok := m.listModel.SelectedItem().(item); ok {
					newModelName := string(i)
					cfg, err := sdk.LoadConfig()
					if err == nil {
						cfg.Model = newModelName
						sdk.SaveConfig(cfg)
						m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Model preference saved as '%s'. It will be used on the next launch.", newModelName))
					}
				}
				m.state = stateChat
				m.updateViewport()
				return m, nil
			}
			
			var listCmd tea.Cmd
			m.listModel, listCmd = m.listModel.Update(msg)
			return m, listCmd
		}
	}

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

				if strings.HasPrefix(input, "/") {
					parts := strings.Fields(input)
					if len(parts) > 0 && (parts[0] == "/exit" || parts[0] == "/quit") {
						return m, tea.Quit
					}
					cmd := m.handleSlashCommand(input)
					if cmd != nil {
						cmds = append(cmds, cmd)
					}
				} else {
					m.loading = true
					cmds = append(cmds, m.callSDK(input))
				}
				m.updateViewport()
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

	case modelsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.state = stateChat
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Error fetching models: "+msg.err.Error()))
			m.updateViewport()
			return m, nil
		}
		
		var items []list.Item
		items = append(items, item("auto")) // Add auto as default top choice
		for _, name := range msg.models {
			items = append(items, item(name))
		}
		m.listModel.SetItems(items)
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			m.spinner, spCmd = m.spinner.Update(msg)
			cmds = append(cmds, spCmd)
		}

	case tea.WindowSizeMsg:
		// Account for the outer border (2 lines) and the status bar (1 line)
		m.width = msg.Width - 2
		m.height = msg.Height - 3

		m.textInput.Width = m.width - 4
		
		// Viewport height: Inner box minus input height (1)
		m.viewport.Width = m.width
		m.viewport.Height = m.height - 1

		m.listModel.SetSize(m.width, m.height)
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

func (m model) fetchModels() tea.Cmd {
	return func() tea.Msg {
		models, err := m.manager.ListModels(context.Background())
		return modelsLoadedMsg{models: models, err: err}
	}
}

func (m *model) handleSlashCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}
	cmd := parts[0]

	switch cmd {
	case "/help":
		helpText := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Agents CLI Help Menu"),
			"",
			"  /help        Shows this menu.",
			"  /clear       Clears the conversation history.",
			"  /context     Displays the current files and metadata loaded in memory.",
			"  /drop [file] Removes a specific file from the active context window.",
			"  /skills      Lists dynamically loaded agent skills.",
			"  /model       Set the active LLM provider (e.g. /model auto).",
			"  /models list Open an interactive list of all available models.",
			"  /exit        Gracefully terminates the session.",
		)
		
		icon := agentMsgStyle.Render("✦ ")
		m.messages = append(m.messages, lipgloss.JoinHorizontal(lipgloss.Top, icon, helpText))
	case "/skills":
		skills := m.manager.Skills()
		if len(skills) == 0 {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"No dynamic skills are currently loaded.")
			return nil
		}
		
		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Loaded Skills"))
		lines = append(lines, "")
		for _, s := range skills {
			lines = append(lines, fmt.Sprintf("  - %s: %s", lipgloss.NewStyle().Foreground(primaryColor).Render(s.Manifest.Name), s.Manifest.Description))
		}
		
		icon := agentMsgStyle.Render("✦ ")
		m.messages = append(m.messages, lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, lines...)))
	case "/models":
		if len(parts) > 1 && parts[1] == "list" {
			m.state = stateModelList
			m.loading = true
			return tea.Batch(m.fetchModels(), m.spinner.Tick)
		}
		m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Usage: /models list")
	case "/model":
		if len(parts) < 2 {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Usage: /model <name>\nExample: /model gemini-2.5-pro\nCurrent mode is: auto")
			return nil
		}
		
		newModelName := parts[1]
		
		// In a real CLI, we would want to reload the AgentManager here, 
		// but since BubbleTea is running, we just persist the preference for the next run.
		cfg, err := sdk.LoadConfig()
		if err == nil {
			cfg.Model = newModelName
			if err := sdk.SaveConfig(cfg); err != nil {
				m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to save config: "+err.Error()))
				return nil
			}
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Model preference saved as '%s'. It will be used on the next launch.", newModelName))
		} else {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to load config: "+err.Error()))
		}
	case "/clear":
		// Clear everything except the welcome screen
		if len(m.messages) > 0 {
			m.messages = []string{m.messages[0]}
		}
	case "/context":
		m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Context management is coming in a future update.")
	case "/drop":
		m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Context management is coming in a future update.")
	default:
		m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Unknown command: "+cmd))
	}
	return nil
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

	var mainBody string

	if m.state == stateModelList {
		if m.loading {
			mainBody = lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), inputBoxStyle.Render(m.spinner.View()+" Fetching models..."))
		} else {
			mainBody = lipgloss.NewStyle().Padding(1, 2).Render(m.listModel.View())
		}
	} else {
		// 1. History
		vpView := m.viewport.View()

		// 2. Input
		var inputView string
		if m.loading {
			inputView = inputBoxStyle.Render(m.spinner.View() + " Thinking...")
		} else {
			inputView = inputBoxStyle.Render(m.textInput.View())
		}
		mainBody = lipgloss.JoinVertical(lipgloss.Left, vpView, inputView)
	}

	// 3. Status
	w1 := m.width / 3
	w2 := m.width / 3
	w3 := m.width - w1 - w2
	
	p1 := lipgloss.NewStyle().Width(w1).Align(lipgloss.Left).Render(" ~/Agents")
	p2 := lipgloss.NewStyle().Width(w2).Align(lipgloss.Center).Render("local mode")
	
	cfg, err := sdk.LoadConfig()
	activeModel := "auto"
	if err == nil && cfg.Model != "" {
		activeModel = cfg.Model
	}
	p3 := lipgloss.NewStyle().Width(w3).Align(lipgloss.Right).Render(activeModel + " ")

	statusView := statusBarStyle.Width(m.width).Render(lipgloss.JoinHorizontal(lipgloss.Top, p1, p2, p3))
	
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
