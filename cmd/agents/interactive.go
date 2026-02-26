package main

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dewitt/agents/pkg/sdk"
)

var (
	// Brand Colors
	googleBlue   = lipgloss.Color("#4285F4")
	googleRed    = lipgloss.Color("#EA4335")
	googleYellow = lipgloss.Color("#FBBC05")
	googleGreen  = lipgloss.Color("#34A853")

	primaryColor   = googleBlue
	secondaryColor = lipgloss.Color("#4169E1") // Royal Blue
	tipColor       = lipgloss.Color("#666666")
	agentColor     = googleGreen
	errorColor     = googleRed
	borderColor    = lipgloss.Color("#333333")
	statusBg       = lipgloss.Color("#1A1A1A")
	statusFg       = lipgloss.Color("#888888")

	// Styles
	appStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderColor)

	logoStyle = lipgloss.NewStyle().
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
			Foreground(googleBlue).
			Bold(true)

	agentMsgStyle = lipgloss.NewStyle().
			Foreground(googleGreen).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(statusFg).
			Background(statusBg).
			Height(1)

	// Layout padding
	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)
)

func colorize(lines []string, mainStyle, shadowStyle lipgloss.Style) []string {
	var res []string
	for _, line := range lines {
		var sb strings.Builder
		for _, r := range line {
			if r == '█' {
				sb.WriteString(mainStyle.Render(string(r)))
			} else if r != ' ' {
				sb.WriteString(shadowStyle.Render(string(r)))
			} else {
				sb.WriteRune(r)
			}
		}
		res = append(res, sb.String())
	}
	return res
}

func renderLogo() string {
	shadowColor := lipgloss.Color("#888888") // Grey
	gtColor := lipgloss.Color("#555555")     // Off-black

	sMainGt := lipgloss.NewStyle().Foreground(gtColor).Bold(true)
	sMainA := lipgloss.NewStyle().Foreground(googleBlue).Bold(true)
	sMainG := lipgloss.NewStyle().Foreground(googleRed).Bold(true)
	sMainE := lipgloss.NewStyle().Foreground(googleYellow).Bold(true)
	sMainN := lipgloss.NewStyle().Foreground(googleBlue).Bold(true)
	sMainT := lipgloss.NewStyle().Foreground(googleGreen).Bold(true)
	sMainS := lipgloss.NewStyle().Foreground(googleRed).Bold(true)
	sShadow := lipgloss.NewStyle().Foreground(shadowColor).Bold(true)

	gt := colorize([]string{
		"██╗    ",
		"╚██╗   ",
		" ╚██╗  ",
		" ██╔╝  ",
		"██╔╝   ",
		"╚═╝    ",
  	"       ",
  	"       ",
	}, sMainGt, sShadow)

	a := colorize([]string{
		" █████╗  ",
		"██╔══██╗ ",
		"███████║ ",
		"██╔══██║ ",
		"██║  ██║ ",
		"╚═╝  ╚═╝ ",
  	"         ",
		"         ",
	}, sMainA, sShadow)

	// Lowercase g
	g := colorize([]string{
		"        ",
		" █████╗ ",
		"██╔══██╗",
		"██║  ██║",
		"╚██████║",
		"     ██║",
		" █████╔╝",
  	" ╚════╝",
	}, sMainG, sShadow)

	// Lowercase e
	e := colorize([]string{
		"        ",
		" █████╗ ",
		"██╔══██╗",
		"███████║",
		"██╔════╝",
		"╚██████╗",
		" ╚═════╝",
  	"        ",
	}, sMainE, sShadow)

	// Lowercase n
	n := colorize([]string{
		"        ",
		"██████╗ ",
		"██╔══██╗",
		"██║  ██║",
		"██║  ██║",
		"╚═╝  ╚═╝",
		"        ",
		"        ",					
	}, sMainN, sShadow)

	// Lowercase t
	t := colorize([]string{
		"  ██╗  ",
		"██████╗",
		"╚═██╔═╝",
		"  ██║  ",
		"  ██║  ",
		"  ╚═╝  ",
		"       ",
  	"       ",
	}, sMainT, sShadow)

	// Lowercase s
	s := colorize([]string{
		"        ",
		" ██████╗",
		"██╔════╝",
		"╚█████╗ ",
		" ╚═══██╗",
		"██████╔╝",
		"╚═════╝ ",
		"        ",
	}, sMainS, sShadow)

	var sb strings.Builder
	for i := 0; i < 8; i++ {
		sb.WriteString(gt[i])
		sb.WriteString(a[i])
		sb.WriteString(g[i])
		sb.WriteString(e[i])
		sb.WriteString(n[i])
		sb.WriteString(t[i])
		sb.WriteString(s[i])
		if i < 7 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

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
	text    string
	err     error
	isShell bool
}

type modelsLoadedMsg struct {
	models []sdk.ModelInfo
	err    error
}

type item struct {
	name        string
	description string
}

func (i item) Title() string       { return i.name }
func (i item) Description() string { return i.description }
func (i item) FilterValue() string { return i.name }

type uiState int

const (
	stateChat uiState = iota
	stateModelList
	stateShell
)

type model struct {
	textArea   textarea.Model
	viewport   viewport.Model
	spinner    spinner.Model
	listModel  list.Model
	messages   []string
	history    []string
	historyIdx int
	manager    sdk.AgentManager
	err        error
	width      int
	height     int
	loading    bool
	quitting   bool
	state      uiState
}

func getUserName() string {
	u, err := user.Current()
	if err == nil {
		if u.Name != "" {
			// Return only the first name for a less formal greeting
			parts := strings.Fields(u.Name)
			if len(parts) > 0 {
				return parts[0]
			}
		}
		if u.Username != "" {
			return u.Username
		}
	}
	return "Developer"
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Type your message or /help (Alt+Enter or ^J for newline)"
	ta.Focus()
	ta.CharLimit = 2000
	ta.ShowLineNumbers = false
	ta.SetWidth(0) // Will be properly set in WindowSizeMsg
	ta.SetHeight(3)
	ta.SetPromptFunc(2, func(lineIdx int) string {
		if lineIdx == 0 {
			return promptStyle.Render("> ")
		}
		return "  "
	})

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
	leftBox := welcomeBoxStyle.Render(fmt.Sprintf("%s\n\nWelcome back, %s!", renderLogo(), getUserName()))
	rightBox := infoBoxStyle.Render(initialTips)
	welcomeScreen := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	return model{
		textArea:   ta,
		viewport:   vp,
		spinner:    s,
		listModel:  l,
		messages:   []string{welcomeScreen},
		history:    []string{},
		historyIdx: 0,
		manager:    sdk.NewManager(),
		loading:    false,
		quitting:   false,
		state:      stateChat,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
		cmds  []tea.Cmd
	)

	// Global intercept for double Ctrl+C to quit
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.Type == tea.KeyCtrlC {
			if m.quitting {
				return m, tea.Quit
			}
			m.quitting = true
			m.textArea.Reset()
			if m.state == stateShell {
				m.textArea.Placeholder = "Press ^C again to quit, or ! to exit shell mode."
			} else {
				m.textArea.Placeholder = "Press ^C again to quit."
			}
			return m, nil
		}

		// Reset quitting state on any other keypress
		if m.quitting {
			m.quitting = false
			if m.state == stateShell {
				m.textArea.Placeholder = "Type your shell command"
			} else {
				m.textArea.Placeholder = "Type your message or /help (Alt+Enter or ^J for newline)"
			}
		}
	}

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
				return m, tea.ClearScreen
			}
			if msg.Type == tea.KeyEnter && m.listModel.FilterState() != list.Filtering {
				if i, ok := m.listModel.SelectedItem().(item); ok {
					newModelName := i.name
					cfg, err := sdk.LoadConfig()
					if err == nil {
						cfg.Model = newModelName
						sdk.SaveConfig(cfg)
						m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Model preference saved as '%s'. It will be used on the next launch.", newModelName))
					}
				}
				m.state = stateChat
				m.updateViewport()
				return m, tea.ClearScreen
			}
			
			var listCmd tea.Cmd
			m.listModel, listCmd = m.listModel.Update(msg)
			
			// Intercept the quit command that the list component returns on 'q' or 'ctrl+c'
			if listCmd != nil {
				// If the list tells us to quit, just return to the chat state instead of exiting the app.
				// We can't easily introspect the command, but we know if they pressed 'q' while not filtering.
				if msg.String() == "q" && m.listModel.FilterState() != list.Filtering {
					m.state = stateChat
					return m, tea.ClearScreen
				}
			}
			return m, listCmd
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyCtrlJ:
			m.textArea.InsertString("\n")
			return m, nil
		case tea.KeyEnter:
			if msg.Alt {
				m.textArea.InsertString("\n")
				return m, nil
			}
			if m.loading {
				return m, nil
			}

			input := m.textArea.Value()
			trimmedInput := strings.TrimSpace(input)

			if m.state == stateShell && (trimmedInput == "exit" || trimmedInput == "quit") {
				m.state = stateChat
				m.textArea.Placeholder = "Type your message or /help (Alt+Enter or ^J for newline)"
				m.textArea.SetPromptFunc(2, func(lineIdx int) string {
					if lineIdx == 0 {
						return promptStyle.Render("> ")
					}
					return "  "
				})
				m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Exited shell execution mode.")
				m.textArea.Reset()
				m.updateViewport()
				return m, nil
			}

			if input != "" {
				if m.state == stateShell {
					m.messages = append(m.messages, lipgloss.NewStyle().Foreground(googleYellow).Bold(true).Render("! ")+input)
				} else {
					m.messages = append(m.messages, promptStyle.Render("> ")+input)
				}

				if len(m.history) == 0 || m.history[len(m.history)-1] != input {
					m.history = append(m.history, input)
				}
				m.historyIdx = len(m.history)

				m.textArea.Reset()

				if m.state == stateShell {
					m.loading = true
					cmds = append(cmds, m.runShellCommand(input))
				} else if strings.HasPrefix(input, "/") {
					parts := strings.Fields(input)
					if len(parts) > 0 && (parts[0] == "/exit" || parts[0] == "/quit") {
						return m, tea.Quit
					}
					cmd := m.handleSlashCommand(input)
					if cmd != nil {
						cmds = append(cmds, cmd, tea.ClearScreen)
					}
				} else if strings.HasPrefix(input, "!") {
					m.loading = true
					cmds = append(cmds, m.runShellCommand(strings.TrimSpace(strings.TrimPrefix(input, "!"))))
				} else {
					m.loading = true
					cmds = append(cmds, m.callSDK(input))
				}
				m.updateViewport()
			}
			return m, tea.Batch(cmds...)
		case tea.KeyTab:
			if !m.loading {
				m.textArea.SetValue(autocompleteCommand(m.textArea.Value()))
				m.textArea.CursorEnd()
			}
		case tea.KeyUp:
			if m.textArea.Line() == 0 {
				if len(m.history) > 0 && m.historyIdx > 0 {
					m.historyIdx--
					m.textArea.SetValue(m.history[m.historyIdx])
					m.textArea.CursorEnd()
				}
				return m, nil
			}
		case tea.KeyDown:
			if m.textArea.Line() == m.textArea.LineCount()-1 {
				if len(m.history) > 0 && m.historyIdx < len(m.history) {
					m.historyIdx++
					if m.historyIdx == len(m.history) {
						m.textArea.Reset()
					} else {
						m.textArea.SetValue(m.history[m.historyIdx])
						m.textArea.CursorEnd()
					}
				}
				return m, nil
			}
		case tea.KeyPgUp, tea.KeyPgDown:
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd
		}

	case responseMsg:
		m.loading = false
		if msg.err != nil {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Error: "+msg.err.Error()))
		} else if msg.isShell {
			// Style for shell output - slightly indented and perhaps a different color
			shellStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).PaddingLeft(2)
			m.messages = append(m.messages, shellStyle.Render(msg.text))
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
			return m, tea.ClearScreen
		}
		
		var items []list.Item
		items = append(items, item{name: "auto", description: "Automatically select the best model"}) // Add auto as default top choice
		for _, mInfo := range msg.models {
			desc := mInfo.Description
			if desc == "" && mInfo.DisplayName != "" {
				desc = mInfo.DisplayName
			} else if desc == "" {
				desc = "Standard generation model"
			}
			items = append(items, item{name: mInfo.Name, description: desc})
		}
		m.listModel.SetItems(items)
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			m.spinner, spCmd = m.spinner.Update(msg)
			cmds = append(cmds, spCmd)
			m.updateViewport()
		}

	case tea.WindowSizeMsg:
		// Account for the outer border (2 lines) and the status bar (1 line)
		m.width = msg.Width - 2
		m.height = msg.Height - 3

		m.textArea.SetWidth(m.width - 4)
		
		// Viewport height: Inner box minus input height and borders
		m.viewport.Width = m.width - 4
		m.viewport.Height = m.height - m.textArea.Height() - 2

		// List Model: account for outer border (2) and padding (4 horizontal, 2 vertical)
		m.listModel.SetSize(m.width-6, m.height-4)
		m.updateViewport()
		return m, nil
	}

	m.textArea, tiCmd = m.textArea.Update(msg)
	cmds = append(cmds, tiCmd)

	// Check for automatic shell mode toggling
	val := m.textArea.Value()
	if m.state == stateChat && strings.HasPrefix(val, "!") {
		m.state = stateShell
		m.textArea.SetValue(strings.TrimPrefix(val, "!"))
		m.textArea.Placeholder = "Type your shell command"
		m.textArea.SetPromptFunc(2, func(lineIdx int) string {
			if lineIdx == 0 {
				return lipgloss.NewStyle().Foreground(googleYellow).Bold(true).Render("! ")
			}
			return "  "
		})
	} else if m.state == stateShell && strings.HasPrefix(val, "!") {
		m.state = stateChat
		m.textArea.SetValue(strings.TrimPrefix(val, "!"))
		m.textArea.Placeholder = "Type your message or /help (Alt+Enter or ^J for newline)"
		m.textArea.SetPromptFunc(2, func(lineIdx int) string {
			if lineIdx == 0 {
				return promptStyle.Render("> ")
			}
			return "  "
		})
	}

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

func (m model) runShellCommand(command string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("bash", "-c", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return responseMsg{text: string(out), err: err, isShell: true}
		}
		return responseMsg{text: string(out), isShell: true}
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
			"  /model list  Open an interactive list of all available models.",
			"  /remember    Saves a global preference (e.g. /remember I use tabs).",
			"  ! [command]  Execute a shell command directly.",
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
	case "/model":
		if len(parts) < 2 {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Usage: /model <name> OR /model list\nCurrent mode is: auto")
			return nil
		}
		
		if parts[1] == "list" {
			m.state = stateModelList
			m.loading = true
			return tea.Batch(m.fetchModels(), m.spinner.Tick)
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
	case "/remember":
		if len(parts) < 2 {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Usage: /remember <fact or preference>")
			return nil
		}
		fact := strings.Join(parts[1:], " ")
		if err := sdk.SaveMemory(fact); err != nil {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to save memory: "+err.Error()))
		} else {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Got it. I'll remember that for all future sessions.")
		}
	default:
		m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Unknown command: "+cmd))
	}
	return nil
}

func autocompleteCommand(input string) string {
	if !strings.HasPrefix(input, "/") {
		return input
	}

	commands := []string{
		"/help",
		"/clear",
		"/context",
		"/drop",
		"/skills",
		"/model",
		"/model list",
		"/remember",
		"/exit",
		"/quit",
	}

	for _, cmd := range commands {
		if strings.HasPrefix(cmd, input) {
			// If we matched exactly or typed space after (e.g. "/models " expecting list), don't aggressively replace.
			// But for simplicity, just return the first match that is longer than what's typed.
			return cmd
		}
	}
	return input
}

func (m *model) updateViewport() {
	var s strings.Builder
	for _, msg := range m.messages {
		s.WriteString(msg)
		s.WriteString("\n\n")
	}
	if m.loading {
		s.WriteString(agentMsgStyle.Render("✦ ") + m.spinner.View() + " Thinking...")
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
		inputView := inputBoxStyle.Render(m.textArea.View())
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
