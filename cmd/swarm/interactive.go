package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime/debug"
	"time"

	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/sahilm/fuzzy"

	"github.com/dewitt/swarm/pkg/sdk"
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
	borderColor    = lipgloss.AdaptiveColor{Light: "#D9D9D9", Dark: "#333333"}
	statusBg       = lipgloss.AdaptiveColor{Light: "#EBEBEB", Dark: "#1A1A1A"}
	statusFg       = lipgloss.AdaptiveColor{Light: "#555555", Dark: "#888888"}

	// Styles
	appStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderColor)

	logoStyle = lipgloss.NewStyle().
			Bold(true)

	welcomeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 4).
			Height(14).
			MarginRight(2)

	infoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 4).
			Height(14)

	promptStyle = lipgloss.NewStyle().
			Foreground(googleBlue).
			Bold(true)

	agentMsgStyle = lipgloss.NewStyle().
			Foreground(googleGreen).
			Bold(true)

	toolMsgStyle = lipgloss.NewStyle().
			Foreground(googleYellow).
			Italic(true)

	thoughtMsgStyle = lipgloss.NewStyle().
			Foreground(tipColor).
			Italic(true)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	statusMsgStyle = lipgloss.NewStyle().
			Foreground(googleBlue)

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

func renderLogo() string {
	sMainGt := lipgloss.NewStyle().Foreground(lipgloss.Color("#334155")).Bold(true)
	sMainS := lipgloss.NewStyle().Foreground(lipgloss.Color("#1b4332")).Bold(true)
	sMainW := lipgloss.NewStyle().Foreground(lipgloss.Color("#2d6a4f")).Bold(true)
	sMainA := lipgloss.NewStyle().Foreground(lipgloss.Color("#40916c")).Bold(true)
	sMainR := lipgloss.NewStyle().Foreground(lipgloss.Color("#52b788")).Bold(true)
	sMainM := lipgloss.NewStyle().Foreground(lipgloss.Color("#74c69d")).Bold(true)
	sShadow := lipgloss.NewStyle().Foreground(lipgloss.Color("#1a1a1a"))

	// Helper to colorize block characters vs line/shadow characters
	colorize := func(lines []string, mainStyle, shadowStyle lipgloss.Style) []string {
		var res []string
		for _, line := range lines {
			var sb strings.Builder
			for _, r := range line {
				if r == '█' || r == '▄' || r == '▀' {
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

	gt := colorize([]string{
		"██╗    ",
		"╚██╗   ",
		" ╚██╗  ",
		" ██╔╝  ",
		"██╔╝   ",
		"╚═╝    ",
	}, sMainGt, sShadow)

	s := colorize([]string{
		" ██████╗",
		"██╔════╝",
		"╚█████╗ ",
		" ╚═══██╗",
		"██████╔╝",
		"╚═════╝ ",
	}, sMainS, sShadow)

	w := colorize([]string{
		"██╗    ██╗",
		"██║    ██║",
		"██║ █╗ ██║",
		"██║███╗██║",
		"╚███╔███╔╝",
		" ╚══╝╚══╝ ",
	}, sMainW, sShadow)

	a := colorize([]string{
		" █████╗ ",
		"██╔══██╗",
		"███████║",
		"██╔══██║",
		"██║  ██║",
		"╚═╝  ╚═╝",
	}, sMainA, sShadow)

	r := colorize([]string{
		"██████╗ ",
		"██╔══██╗",
		"██████╔╝",
		"██╔══██╗",
		"██║  ██║",
		"╚═╝  ╚═╝",
	}, sMainR, sShadow)

	m := colorize([]string{
		"███╗   ███╗",
		"████╗ ████║",
		"██╔████╔██║",
		"██║╚██╔╝██║",
		"██║ ╚═╝ ██║",
		"╚═╝     ╚═╝",
	}, sMainM, sShadow)

	var sb strings.Builder
	for i := 0; i < 6; i++ {
		sb.WriteString(gt[i])
		sb.WriteString(s[i])
		sb.WriteString(w[i])
		sb.WriteString(a[i])
		sb.WriteString(r[i])
		sb.WriteString(m[i])
		if i < 5 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

type streamMsg struct {
	event sdk.ChatEvent
	ch    <-chan sdk.ChatEvent
}

type streamDoneMsg struct{}

type streamErrMsg struct {
	err error
}

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

type swarmAgent struct {
	name       string
	icon       string
	status     string
	state      string // "idle", "active", "success", "waiting", "error"
	spin       spinner.Model
	lastActive time.Time
	resident   bool
}

func (a *swarmAgent) update(state, status string) {
	if state != "" {
		a.state = state
	}
	if status != "" {
		a.status = status
	}
	a.lastActive = time.Now()
}

func getWorkspaceFiles() []string {
	var items []string
	out, err := exec.Command("git", "ls-files").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				items = append(items, l)
			}
		}
	}
	return items
}

type model struct {
	textArea     textarea.Model
	viewport     viewport.Model
	spinner      spinner.Model
	listModel    list.Model
	messages     []string
	history      []string
	historyIdx   int
	currentInput string
	manager      sdk.AgentManager
	err          error
	width        int
	height       int
	loading      bool
	quitting     bool
	planMode     bool
	state        uiState
	cwd          string
	gitBranch    string
	gitModified  bool
	activeModel  string
	renderer     *glamour.TermRenderer

	statusMsg    string
	lastResponse string
	observeMode  bool
	observeLog   []string
	activeAgent  string

	// Input Queueing and Async HITL
	inputQueue []string
	cancelChat context.CancelFunc

	// Agent Panel state
	agents        []*swarmAgent
	ticks         int
	showAgentPanel bool

	// Autocomplete state
	workspaceFiles []string
	acMatches      []string
	acIndex        int
	acActive       bool
	acPrefix       string
	acMode         string // "file", "command", "history"
	acHasMore      bool
}

func updateAutocomplete(m *model) {
	if m.workspaceFiles == nil {
		m.workspaceFiles = getWorkspaceFiles()
	}
	val := m.textArea.Value()
	m.acHasMore = false

	if m.acMode == "history" {
		m.acActive = true
		m.acPrefix = ""

		// Deduplicate history
		var uniqueHist []string
		seen := make(map[string]bool)
		for i := len(m.history) - 1; i >= 0; i-- {
			h := m.history[i]
			if !seen[h] {
				uniqueHist = append(uniqueHist, h)
				seen[h] = true
			}
		}

		if val == "" {
			m.acMatches = uniqueHist
		} else {
			matches := fuzzy.Find(val, uniqueHist)
			var mstrs []string
			for _, match := range matches {
				mstrs = append(mstrs, match.Str)
			}
			m.acMatches = mstrs
		}

		if len(m.acMatches) > 8 {
			m.acHasMore = true
			m.acMatches = m.acMatches[:8]
		}

		if len(m.acMatches) == 0 {
			m.acActive = false
			m.acIndex = 0
		} else if m.acIndex >= len(m.acMatches) {
			m.acIndex = 0
		}
		return
	}

	// find the last word
	lastSpace := strings.LastIndexAny(val, " \n")
	var lastWord string
	if lastSpace == -1 {
		lastWord = val
	} else {
		lastWord = val[lastSpace+1:]
	}

	slashCommands := []string{
		"help", "clear", "context", "drop", "skills",
		"sessions", "model", "remember", "plan", "act", "exit", "quit",
	}

	if strings.HasPrefix(lastWord, "@") {
		m.acActive = true
		m.acPrefix = "@"
		query := lastWord[1:]
		if query == "" {
			m.acMatches = m.workspaceFiles
		} else {
			matches := fuzzy.Find(query, m.workspaceFiles)
			var mstrs []string
			for _, match := range matches {
				mstrs = append(mstrs, match.Str)
			}
			m.acMatches = mstrs
		}
		if len(m.acMatches) > 8 {
			m.acHasMore = true
			m.acMatches = m.acMatches[:8]
		}
		if len(m.acMatches) == 0 {
			m.acActive = false
			m.acIndex = 0
		} else if m.acIndex >= len(m.acMatches) {
			m.acIndex = 0
		}
	} else if lastSpace == -1 && strings.HasPrefix(lastWord, "/") {
		m.acActive = true
		m.acPrefix = "/"
		query := lastWord[1:]
		if query == "" {
			m.acMatches = slashCommands
		} else {
			matches := fuzzy.Find(query, slashCommands)
			var mstrs []string
			for _, match := range matches {
				mstrs = append(mstrs, match.Str)
			}
			m.acMatches = mstrs
		}
		if len(m.acMatches) > 8 {
			m.acHasMore = true
			m.acMatches = m.acMatches[:8]
		}
		if len(m.acMatches) == 0 {
			m.acActive = false
			m.acIndex = 0
		} else if m.acIndex >= len(m.acMatches) {
			m.acIndex = 0
		}
	} else {
		m.acActive = false
		m.acPrefix = ""
		m.acMatches = nil
		m.acIndex = 0
	}
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

func getAgentBadge(author string) string {
	if author == "router_agent" || author == "agent" {
		return lipgloss.NewStyle().Foreground(googleBlue).Render("❖ Router")
	} else if author == "builder_agent" {
		return lipgloss.NewStyle().Foreground(googleYellow).Render("⚒ Builder")
	} else if author == "gitops_agent" {
		return lipgloss.NewStyle().Foreground(googleRed).Render("🚀 GitOps")
	} else if author == "codebase-investigator" || author == "codebase_investigator" {
		return lipgloss.NewStyle().Foreground(googleGreen).Render("🔍 Investigator")
	}
	return lipgloss.NewStyle().Foreground(agentColor).Render("✦ " + author)
}

func getHistoryFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "swarm", "history.json")
}

func loadHistory() []string {
	file := getHistoryFile()
	if file == "" {
		return []string{}
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return []string{}
	}
	var history []string
	if err := json.Unmarshal(b, &history); err != nil {
		return []string{}
	}
	return history
}

func saveHistory(history []string) {
	file := getHistoryFile()
	if file == "" {
		return
	}
	dir := filepath.Dir(file)
	os.MkdirAll(dir, 0755)

	// Keep only the last 1000 items to prevent the file from growing indefinitely
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}

	b, err := json.MarshalIndent(history, "", "  ")
	if err == nil {
		os.WriteFile(file, b, 0644)
	}
}

func getGitInfo() (string, bool) {
	info, err := sdk.GetGitInfo(".")
	if err != nil {
		return "", false
	}
	return info.Branch, info.Modified
}

type gitStatusMsg struct {
	branch   string
	modified bool
}

func checkGitStatus() tea.Cmd {
	return func() tea.Msg {
		branch, modified := getGitInfo()
		return gitStatusMsg{branch, modified}
	}
}

type gitTickMsg time.Time

func doGitTick() tea.Cmd {
	return tea.Tick(time.Second*10, func(t time.Time) tea.Msg {
		return gitTickMsg(t)
	})
}

func getRecentActivity() string {
	m := sdk.NewManager()
	sessions, _ := m.ListSessions(context.Background())
	commits, _ := sdk.GetRecentCommits(".", 3)

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(googleBlue).Render("Recent Activity") + "\n")

	hasActivity := false
	if len(commits) > 0 {
		for _, c := range commits {
			if len(c) > 30 {
				c = c[:27] + "..."
			}
			sb.WriteString(lipgloss.NewStyle().Foreground(googleGreen).Render(" git ") + c + "\n")
		}
		hasActivity = true
	}

	sessionCount := 0
	for i := len(sessions) - 1; i >= 0 && sessionCount < 3; i-- {
		s := sessions[i]
		summary := s.Summary
		if len(summary) > 30 {
			summary = summary[:27] + "..."
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(googleYellow).Render(" chat ") + summary + "\n")
		sessionCount++
		hasActivity = true
	}

	if !hasActivity {
		sb.WriteString("(none yet)\n")
	}

	// Add spacing to reach a consistent height
	currentLines := strings.Count(sb.String(), "\n")
	for i := currentLines; i < 7; i++ {
		sb.WriteString("\n")
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(googleBlue).Render("Quick Tips") + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(tipColor).Render("^O") + " toggle observe  " + lipgloss.NewStyle().Foreground(tipColor).Render("!") + " run shell\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(tipColor).Render("/plan") + " brainstorm   " + lipgloss.NewStyle().Foreground(tipColor).Render("/skills") + " skills")

	return sb.String()
}

func initialModel(planMode bool, resume bool) model {
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
	greeting := fmt.Sprintf("\n\n%s %s!", lipgloss.NewStyle().Foreground(tipColor).Render("Welcome back,"), lipgloss.NewStyle().Bold(true).Render(getUserName()))
	leftBox := welcomeBoxStyle.Render(renderLogo() + greeting)
	rightBox := infoBoxStyle.Render(getRecentActivity())
	welcomeScreen := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(cwd, home) {
		cwd = strings.Replace(cwd, home, "~", 1)
	}

	activeModel := "auto"
	if cfg, err := sdk.LoadConfig(); err == nil && cfg.Model != "" {
		activeModel = cfg.Model
	}

	style := "dark"
	if !lipgloss.HasDarkBackground() {
		style = "light"
	}
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(0), // Will be updated on WindowSizeMsg
	)

	loadedHist := loadHistory()

	branch, modified := getGitInfo()

	// Initialize resident Agent Panel agents
	agentSpinner := spinner.New()
	agentSpinner.Spinner = spinner.Dot
	agentSpinner.Style = lipgloss.NewStyle().Foreground(colorActive)

	agents := []*swarmAgent{
		{name: "Chat Input Agent", icon: "💠", status: "Idle", state: "idle", spin: agentSpinner, resident: true, lastActive: time.Now()},
		{name: "Router", icon: "🧠", status: "Idle", state: "idle", spin: agentSpinner, resident: true, lastActive: time.Now()},
	}

	return model{
		textArea:      ta,
		viewport:      vp,
		spinner:       s,
		listModel:     l,
		messages:      []string{welcomeScreen},
		history:       loadedHist,
		historyIdx:    len(loadedHist),
		manager:       sdk.NewManager(sdk.ManagerConfig{ResumeLastSession: resume}),
		loading:       false,
		quitting:      false,
		planMode:      planMode,
		state:         stateChat,
		cwd:           cwd,
		gitBranch:     branch,
		gitModified:   modified,
		activeModel:   activeModel,
		renderer:      renderer,
		agents:        agents,
		showAgentPanel: true,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink, m.spinner.Tick, doGitTick()}
	for _, a := range m.agents {
		cmds = append(cmds, a.spin.Tick)
	}
	return tea.Batch(cmds...)
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
			if m.loading {
				if m.cancelChat != nil {
					m.cancelChat()
					m.cancelChat = nil
				}
				m.inputQueue = nil
				m.loading = false
				m.statusMsg = ""
				m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Bold(true).Render("✦ [System] Swarm execution forcefully halted by user."))
				m.updateViewport()
				m.updateInputStyle()
				return m, nil
			}
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
			m.updateInputStyle()
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
						m.activeModel = newModelName
						m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Model preference saved as '%s'.", newModelName))
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
	case gitStatusMsg:
		m.gitBranch = msg.branch
		m.gitModified = msg.modified
		return m, nil

	case gitTickMsg:
		return m, tea.Batch(checkGitStatus(), doGitTick())

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if m.loading {
				if m.cancelChat != nil {
					m.cancelChat()
					m.cancelChat = nil
				}
				m.inputQueue = nil
				m.loading = false
				m.statusMsg = ""
				m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Bold(true).Render("✦ [System] Swarm execution forcefully halted by user."))
				m.updateViewport()
				m.updateInputStyle()
				return m, nil
			}
			if m.acActive {
				m.acActive = false
				m.acMode = ""
				return m, nil
			}
			if m.textArea.Value() != "" {
				m.textArea.SetValue("")
				m.historyIdx = len(m.history)
				return m, nil
			}
			return m, nil
		case tea.KeyCtrlJ:
			m.textArea.InsertString("\n")
			return m, nil
		case tea.KeyCtrlO:
			m.observeMode = !m.observeMode
			state := "disabled"
			if m.observeMode {
				state = "enabled"
			}
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Observe mode %s.", state))
			m.updateViewport()
			return m, nil
		case tea.KeyCtrlL:
			return m, tea.ClearScreen
		case tea.KeyCtrlR:
			if m.state == stateChat && !m.loading {
				m.acActive = true
				m.acMode = "history"
				m.acPrefix = ""
				m.textArea.SetValue("")
				updateAutocomplete(&m)
				return m, tea.ClearScreen
			}
		case tea.KeyEnter:
			if msg.Alt {
				m.textArea.InsertString("\n")
				return m, nil
			}
			if m.loading {
				input := m.textArea.Value()
				trimmedInput := strings.TrimSpace(input)
				if trimmedInput != "" {
					m.inputQueue = append(m.inputQueue, trimmedInput)
					m.textArea.Reset()

					queuedIcon := lipgloss.NewStyle().Foreground(googleYellow).Render("⧖ ")
					m.messages = append(m.messages, queuedIcon+trimmedInput)
					m.updateViewport()
				}
				return m, nil
			}

			if m.acActive && len(m.acMatches) > 0 {
				if m.acMode == "history" {
					m.textArea.SetValue(m.acMatches[m.acIndex])
				} else {
					val := m.textArea.Value()
					lastSpace := strings.LastIndexAny(val, " \n")
					m.textArea.SetValue(val[:lastSpace+1] + m.acPrefix + m.acMatches[m.acIndex] + " ")
				}
				m.textArea.CursorEnd()
				m.acActive = false
				m.acMode = ""
				// Continue to submit
			}

			input := m.textArea.Value()
			trimmedInput := strings.TrimSpace(input)

			if m.state == stateShell && (trimmedInput == "exit" || trimmedInput == "quit") {
				m.state = stateChat
				m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Exited shell execution mode.")
				m.textArea.Reset()
				m.updateViewport()
				m.updateInputStyle()
				return m, nil
			}

			if input != "" {
				if m.state == stateShell {
					m.messages = append(m.messages, lipgloss.NewStyle().Width(m.viewport.Width).Render(lipgloss.NewStyle().Foreground(googleYellow).Bold(true).Render("! ")+input))
				} else {
					m.messages = append(m.messages, lipgloss.NewStyle().Width(m.viewport.Width).Render(promptStyle.Render("> ")+input))
				}

				if len(m.history) == 0 || m.history[len(m.history)-1] != input {
					m.history = append(m.history, input)
					saveHistory(m.history)
				}
				m.historyIdx = len(m.history)

				m.textArea.Reset()

				if m.state == stateShell {
					m.loading = true
					ctx, cancel := context.WithCancel(context.Background())
					m.cancelChat = cancel
					cmds = append(cmds, m.runShellCommand(ctx, input))
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
					ctx, cancel := context.WithCancel(context.Background())
					m.cancelChat = cancel
					cmds = append(cmds, m.runShellCommand(ctx, strings.TrimSpace(strings.TrimPrefix(input, "!"))))
				} else {
					m.loading = true
					ctx, cancel := context.WithCancel(context.Background())
					m.cancelChat = cancel
					cmds = append(cmds, m.callSDK(ctx, input))
				}
				m.updateViewport()
			}
			return m, tea.Batch(cmds...)
		case tea.KeyTab:
			if m.acActive && len(m.acMatches) > 0 {
				if m.acMode == "history" {
					m.textArea.SetValue(m.acMatches[m.acIndex])
				} else {
					val := m.textArea.Value()
					lastSpace := strings.LastIndexAny(val, " \n")
					m.textArea.SetValue(val[:lastSpace+1] + m.acPrefix + m.acMatches[m.acIndex] + " ")
				}
				m.textArea.CursorEnd()
				m.acActive = false
				m.acMode = ""
			}
			return m, nil
		case tea.KeyUp:
			if m.acActive {
				if len(m.acMatches) > 0 {
					m.acIndex--
					if m.acIndex < 0 {
						m.acIndex = len(m.acMatches) - 1
					}
				}
				return m, nil
			}
			if m.textArea.Line() == 0 {
				if len(m.history) > 0 && m.historyIdx > 0 {
					if m.historyIdx == len(m.history) {
						m.currentInput = m.textArea.Value()
					}
					m.historyIdx--
					m.textArea.SetValue(m.history[m.historyIdx])
					m.textArea.CursorEnd()
				}
				return m, nil
			}
		case tea.KeyDown:
			if m.acActive {
				if len(m.acMatches) > 0 {
					m.acIndex++
					if m.acIndex >= len(m.acMatches) {
						m.acIndex = 0
					}
				}
				return m, nil
			}
			if m.textArea.Line() == m.textArea.LineCount()-1 {
				if len(m.history) > 0 && m.historyIdx < len(m.history) {
					m.historyIdx++
					if m.historyIdx == len(m.history) {
						m.textArea.SetValue(m.currentInput)
						m.textArea.CursorEnd()
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

	case tea.MouseMsg:
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		return m, vpCmd
	case streamMsg:
		event := msg.event
		var agentCmd tea.Cmd

		switch event.Type {
		case sdk.ChatEventHandoff:
			newAgentName := strings.TrimSpace(event.Content)
			if m.observeMode {
				m.observeLog = append(m.observeLog, fmt.Sprintf("[%s] ➡️ Delegated task to: %s", m.activeAgent, lipgloss.NewStyle().Bold(true).Render(newAgentName)))
			}

			// Update AgentPanel
			if oldA := m.findAgent(m.activeAgent); oldA != nil {
				oldA.update("success", "Task completed")
			}
			var newA *swarmAgent
			newA, agentCmd = m.ensureAgent(newAgentName)
			newA.update("active", "Analyzing context…")

			m.activeAgent = newAgentName
			m.statusMsg = ""
			m.updateViewport()
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.ChatEventToolCall:
			toolInfo := event.Content
			// Split name and args
			parts := strings.SplitN(toolInfo, " ", 2)
			toolName := parts[0]
			toolArgs := ""
			if len(parts) > 1 {
				toolArgs = parts[1]
			}

			m.statusMsg = "Running " + toolName + "…"

			// Update AgentPanel
			a, cmd := m.ensureAgent(m.activeAgent)
			agentCmd = cmd
			a.update("active", "Tool: "+toolName)

			if m.observeMode {
				logEntry := fmt.Sprintf("[%s] Executing %s", m.activeAgent, toolName)
				if toolArgs != "" {
					logEntry += " " + lipgloss.NewStyle().Foreground(tipColor).Render(toolArgs)
				}
				m.observeLog = append(m.observeLog, logEntry)
			}
			m.updateViewport()
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.ChatEventToolResult:
			resultInfo := event.Content
			parts := strings.SplitN(resultInfo, " ", 2)
			toolName := parts[0]

			// Update AgentPanel
			a, cmd := m.ensureAgent(m.activeAgent)
			agentCmd = cmd
			a.update("", "Completed "+toolName)

			if m.observeMode {
				toolResult := ""
				if len(parts) > 1 {
					toolResult = parts[1]
				}
				logEntry := fmt.Sprintf("[%s] Completed %s", m.activeAgent, toolName)
				if toolResult != "" {
					logEntry += " " + lipgloss.NewStyle().Foreground(tipColor).Render(toolResult)
				}
				m.observeLog = append(m.observeLog, logEntry)
				m.updateViewport()
			}
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.ChatEventThought:
			// Update AgentPanel
			a, cmd := m.ensureAgent(event.Agent)
			agentCmd = cmd
			a.update("active", event.Content)

			if m.observeMode {
				thought := event.Content
				m.observeLog = append(m.observeLog, fmt.Sprintf("[%s] 🤔 %s", event.Agent, lipgloss.NewStyle().Foreground(tipColor).Italic(true).Render(thought)))
				m.updateViewport()
			}
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.ChatEventFinalResponse:
			m.statusMsg = ""
			author := event.Agent
			if author == "" {
				author = "agent"
			}

			// Update AgentPanel
			a, cmd := m.ensureAgent(author)
			agentCmd = cmd
			a.update("success", "Response ready")

			text := event.Content
			out := text
			if m.renderer != nil {
				if rOut, err := m.renderer.Render(text); err == nil {
					out = rOut
				}
			}

			badge := getAgentBadge(author)
			m.messages = append(m.messages, badge+"\n"+strings.TrimSpace(out))
			m.loading = false
			m.updateViewport()
			return m, tea.Batch(m.dequeueAndRun(), agentCmd)

		case sdk.ChatEventError:
			m.statusMsg = ""

			// Update AgentPanel
			a, cmd := m.ensureAgent(m.activeAgent)
			agentCmd = cmd
			a.update("error", "Failed")

			m.messages = append(m.messages, errorMsgStyle.Render(fmt.Sprintf("Error: %s", event.Content)))
			m.loading = false
			m.updateViewport()
			return m, tea.Batch(m.dequeueAndRun(), agentCmd)
		}

		return m, listenForStream(msg.ch)

	case streamDoneMsg:
		m.loading = false
		m.statusMsg = ""
		m.activeAgent = "Router"
		m.observeLog = nil
		m.updateViewport()
		return m, m.dequeueAndRun()

	case streamErrMsg:
		m.loading = false
		m.statusMsg = ""
		m.observeLog = nil
		m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Width(m.viewport.Width).Render("Error: "+msg.err.Error()))
		m.updateViewport()
		return m, m.dequeueAndRun()

	case responseMsg:
		m.loading = false
		if msg.err != nil {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Width(m.viewport.Width).Render("Error: "+msg.err.Error()))
		} else if msg.isShell {
			// Style for shell output - slightly indented and perhaps a different color
			shellStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).PaddingLeft(2).Width(m.viewport.Width)
			m.messages = append(m.messages, shellStyle.Render(msg.text))
		} else {
			out := msg.text
			if m.renderer != nil {
				if rOut, err := m.renderer.Render(msg.text); err == nil {
					out = rOut
				}
			}
			m.messages = append(m.messages, agentMsgStyle.Render("✦\n")+strings.TrimSpace(out))
		}
		m.updateViewport()
		return m, m.dequeueAndRun()

	case modelsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.state = stateChat
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Width(m.viewport.Width).Render("Error fetching models: "+msg.err.Error()))
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
		}
		for i, a := range m.agents {
			if a.state == "active" {
				var cmd tea.Cmd
				m.agents[i].spin, cmd = a.spin.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
		m.updateViewport()
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		// Account for the outer border (2 lines) and the status bar (1 line)
		m.width = msg.Width - 2
		m.height = msg.Height - 3

		m.textArea.SetWidth(m.width - 4)

		agentPanelHeight := 0
		if m.showAgentPanel {
			agentPanelHeight = lipgloss.Height(m.renderAgentPanel())
		}

		// Viewport height: Inner box minus input height, borders, and agentPanel
		m.viewport.Width = m.width - 4
		m.viewport.Height = m.height - m.textArea.Height() - 3 - agentPanelHeight

		// Update glamour word wrap
		if m.renderer != nil {
			style := "dark"
			if !lipgloss.HasDarkBackground() {
				style = "light"
			}
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithStandardStyle(style),
				glamour.WithWordWrap(m.viewport.Width-4),
			)
		}

		// List Model: account for outer border (2) and padding (4 horizontal, 2 vertical)
		m.listModel.SetSize(m.width-6, m.height-4)
		m.updateViewport()
		return m, nil
	}

	m.textArea, tiCmd = m.textArea.Update(msg)
	cmds = append(cmds, tiCmd)

	if m.state == stateChat {
		updateAutocomplete(&m)
	}

	// Check for automatic shell mode toggling
	val := m.textArea.Value()
	if m.state == stateChat && strings.HasPrefix(val, "!") {
		m.state = stateShell
		m.textArea.SetValue(strings.TrimPrefix(val, "!"))
	} else if m.state == stateShell && strings.HasPrefix(val, "!") {
		m.state = stateChat
		m.textArea.SetValue(strings.TrimPrefix(val, "!"))
	}

	if m.loading {
		m.spinner, spCmd = m.spinner.Update(msg)
		cmds = append(cmds, spCmd)
	}

	m.updateInputStyle()
	return m, tea.Batch(cmds...)
}

func (m model) callSDK(ctx context.Context, input string) tea.Cmd {
	if m.planMode {
		input = "[SYSTEM: You are in PLAN MODE. You must strictly act as a read-only architectural advisor. Under NO circumstances should you use tools to write files, execute bash commands, or alter git state. Only use tools to read and list files.]\n\nUser: " + input
	}
	ch, err := m.manager.Chat(ctx, input)
	if err != nil {
		return func() tea.Msg { return streamErrMsg{err: err} }
	}
	return listenForStream(ch)
}

func (m *model) dequeueAndRun() tea.Cmd {
	if len(m.inputQueue) == 0 {
		m.loading = false
		return checkGitStatus()
	}

	// Reset AgentPanel for new task
	for _, a := range m.agents {
		a.update("idle", "")
		a.update("", "Idle")
	}
	if r := m.findAgent("Router"); r != nil {
		r.update("active", "")
		r.update("", "Processing input…")
	}

	nextInput := m.inputQueue[0]
	m.inputQueue = m.inputQueue[1:]

	if m.state == stateShell {
		m.loading = true
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelChat = cancel
		return m.runShellCommand(ctx, nextInput)
	} else if strings.HasPrefix(nextInput, "!") {
		m.loading = true
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelChat = cancel
		return m.runShellCommand(ctx, strings.TrimSpace(strings.TrimPrefix(nextInput, "!")))
	} else {
		m.loading = true
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelChat = cancel
		return m.callSDK(ctx, nextInput)
	}
}

func listenForStream(ch <-chan sdk.ChatEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return streamMsg{event: event, ch: ch}
	}
}

func (m model) runShellCommand(ctx context.Context, command string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.CommandContext(ctx, "bash", "-c", command)
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
	case "/about":
		var buildInfoStr string
		if info, ok := debug.ReadBuildInfo(); ok {
			buildInfoStr = fmt.Sprintf("Go Version: %s\nPath: %s\nMain: %s %s\n", info.GoVersion, info.Path, info.Main.Path, info.Main.Version)
			var revision string
			var timeStr string
			var modified bool
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					revision = setting.Value
				}
				if setting.Key == "vcs.time" {
					timeStr = setting.Value
				}
				if setting.Key == "vcs.modified" {
					modified = setting.Value == "true"
				}
			}
			if revision != "" {
				modStr := ""
				if modified {
					modStr = " (dirty)"
				}
				buildInfoStr += fmt.Sprintf("Revision: %s%s\nBuild Time: %s", revision, modStr, timeStr)
			}
		} else {
			buildInfoStr = "Build information not available."
		}
		aboutText := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Swarm CLI"),
			"",
			buildInfoStr,
		)
		icon := agentMsgStyle.Render("✦ ")
		m.messages = append(m.messages, lipgloss.JoinHorizontal(lipgloss.Top, icon, aboutText))
	case "/help":
		helpText := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Swarm CLI Help Menu"),
			"",
			"  /about       Displays version and build information.",

			"  /help        Shows this menu.",
			"  /clear       Clears the conversation history.",
			"  /context     Displays the current files and metadata loaded in memory.",
			"  /drop [file] Removes a specific file from the active context window.",
			"  /skills      Lists dynamically loaded agent skills.",
			"  /skills reload Dynamically reloads all agent skills.",
			"  /sessions    Lists all persisted interactive sessions in the SQLite database.",
			"  /model       Set the active LLM provider (e.g. /model auto).",
			"  /model list  Open an interactive list of all available models.",
			"  /config      Prints the current global configuration.",
			"  /remember    Saves a global preference (e.g. /remember I use tabs).",
			"  /copy        Copies the last agent response to the system clipboard.",
			"  /observe     Toggles observe mode to see real-time agent activity.",
			"  /rewind [n]  Rewinds the conversation history by n turns (default 1).",
			"  /plan        Enter read-only plan mode to brainstorm safely.",
			"  /act         Exit plan mode and allow the agent to execute actions.",
			"  ! [command]  Execute a shell command directly.",
			"  /exit        Gracefully terminates the session.",
		)

		icon := agentMsgStyle.Render("✦ ")
		m.messages = append(m.messages, lipgloss.JoinHorizontal(lipgloss.Top, icon, helpText))
	case "/sessions":
		sessions, err := m.manager.ListSessions(context.Background())
		if err != nil {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to list sessions: "+err.Error()))
			return nil
		}
		if len(sessions) == 0 {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"No sessions found in the database.")
			return nil
		}

		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Stored Sessions (%d)", len(sessions))))
		lines = append(lines, "")

		for _, s := range sessions {
			id := lipgloss.NewStyle().Foreground(primaryColor).Render(s.ID)
			content := fmt.Sprintf("- %s (Last Updated: %s)", id, s.UpdatedAt)
			lines = append(lines, content)
		}

		icon := agentMsgStyle.Render("✦ ")
		m.messages = append(m.messages, lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, lines...)))
	case "/skills":
		if len(parts) > 1 && parts[1] == "reload" {
			if err := m.manager.Reload(); err != nil {
				m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to reload skills: "+err.Error()))
			} else {
				m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Skills and agents reloaded successfully.")
			}
			return nil
		}

		skills := m.manager.Skills()
		if len(skills) == 0 {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"No dynamic skills are currently loaded.")
			return nil
		}

		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Loaded Skills"))
		lines = append(lines, "")

		// Use the actual viewport width for text wrapping (minus icon and padding)
		wrapWidth := m.viewport.Width - 4
		if wrapWidth < 20 {
			wrapWidth = 20
		}

		for _, s := range skills {
			name := lipgloss.NewStyle().Foreground(primaryColor).Render(s.Manifest.Name)
			desc := s.Manifest.Description
			content := fmt.Sprintf("- %s: %s", name, desc)
			lines = append(lines, lipgloss.NewStyle().Width(wrapWidth).Render(content))
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
			m.activeModel = newModelName
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Model preference saved as '%s'.", newModelName))
		} else {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to load config: "+err.Error()))
		}
	case "/copy":
		if m.lastResponse != "" {
			if err := clipboard.WriteAll(m.lastResponse); err != nil {
				m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to copy to clipboard: "+err.Error()))
			} else {
				m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Copied last response to clipboard.")
			}
		} else {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"No response available to copy.")
		}
	case "/clear":
		// Clear everything except the welcome screen
		if len(m.messages) > 0 {
			m.messages = []string{m.messages[0]}
		}
		m.manager.Reset()
		for _, a := range m.agents {
			a.update("idle", "")
			a.update("", "Idle")
		}
		m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Screen and conversation history cleared. Context window reset.")
	case "/rewind":
		n := 1
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &n)
		}
		if err := m.manager.Rewind(n); err != nil {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to rewind: "+err.Error()))
		} else {
			for _, a := range m.agents {
				a.update("idle", "")
				a.update("", "Idle")
			}
			// Wipe the local messages to reflect the rewound state (or just append a notice)
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Rewound the conversation history by %d turn(s).", n))
		}
	case "/plan":
		m.planMode = true
		m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Plan Mode enabled. I will only read files and brainstorm. I will not modify files or execute shell commands.")
	case "/act":
		m.planMode = false
		m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Act Mode enabled. I am fully capable of writing files and executing commands.")
	case "/observe":
		m.observeMode = !m.observeMode
		state := "disabled"
		if m.observeMode {
			state = "enabled"
		}
		m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Observe mode %s.", state))
		m.updateViewport()

	case "/config":
		cfg, err := sdk.LoadConfig()
		if err != nil {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to load config: "+err.Error()))
		} else {
			var lines []string
			lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Global Configuration"))
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("  - Active Model: %s", lipgloss.NewStyle().Foreground(primaryColor).Render(cfg.Model)))

			configPath, _ := sdk.DefaultConfigPath()
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("Configuration stored at: %s", lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(configPath)))

			icon := agentMsgStyle.Render("✦ ")
			m.messages = append(m.messages, lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, lines...)))
		}
	case "/context":
		if len(parts) == 1 {
			// List context
			files := m.manager.ListContext()
			if len(files) == 0 {
				m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"No files are currently pinned to the context window.")
				return nil
			}

			var lines []string
			lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Pinned Context Files (%d)", len(files))))
			lines = append(lines, "")
			for _, file := range files {
				lines = append(lines, "  - "+lipgloss.NewStyle().Foreground(primaryColor).Render(file))
			}
			icon := agentMsgStyle.Render("✦ ")
			m.messages = append(m.messages, lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, lines...)))
		} else if parts[1] == "add" {
			if len(parts) < 3 {
				m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Usage: /context add <file_path>")
				return nil
			}
			filePath := parts[2]
			if err := m.manager.AddContext(filePath); err != nil {
				m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Error pinning file: "+err.Error()))
			} else {
				m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Pinned `%s` to the active context window.", filePath))
			}
		} else {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Usage: /context OR /context add <file_path>")
		}
	case "/drop":
		if len(parts) < 2 {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Usage: /drop <file_path> OR /drop all")
			return nil
		}
		filePath := parts[1]
		m.manager.DropContext(filePath)
		if filePath == "all" {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Cleared all pinned files from the context window.")
		} else {
			m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Dropped `%s` from the context window.", filePath))
		}
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

func (m *model) updateViewport() {
	var s strings.Builder
	for _, msg := range m.messages {
		s.WriteString(msg)
		s.WriteString("\n\n")
	}
	if m.loading {
		if m.observeMode && len(m.observeLog) > 0 {
			// Only show the last 10 log entries to prevent the box from taking over the screen
			displayLogs := m.observeLog
			if len(displayLogs) > 10 {
				displayLogs = displayLogs[len(displayLogs)-10:]
			}

			observeBox := lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(googleYellow).
				Padding(0, 1).
				Width(m.viewport.Width - 2).
				Render(lipgloss.JoinVertical(lipgloss.Left,
					lipgloss.NewStyle().Foreground(googleYellow).Bold(true).Render("👀 Observing Agent Execution:"),
					strings.Join(displayLogs, "\n"),
				))
			s.WriteString(observeBox)
			s.WriteString("\n\n")
		}

		status := "Thinking…"
		if m.statusMsg != "" {
			status = m.statusMsg
		}
		agentLabel := m.activeAgent
		if agentLabel == "" {
			agentLabel = "Router"
		}
		s.WriteString(agentMsgStyle.Render(fmt.Sprintf("✦ [%s] ", agentLabel)) + m.spinner.View() + " " + status)
		s.WriteString("\n\n")
	}
	isAtBottom := m.viewport.AtBottom()
	currentY := m.viewport.YOffset

	m.viewport.SetContent(s.String())

	if isAtBottom {
		m.viewport.GotoBottom()
	} else {
		// Retain scroll position
		m.viewport.YOffset = currentY
	}
}

func (m *model) updateInputStyle() {
	if m.quitting {
		if m.state == stateShell {
			m.textArea.Placeholder = "Press ^C again to quit, or ! to exit shell mode."
		} else {
			m.textArea.Placeholder = "Press ^C again to quit."
		}
		return
	}

	if m.loading {
		m.textArea.Placeholder = "Agents are working. Type to queue a message or press Esc to interrupt..."
		m.textArea.SetPromptFunc(2, func(lineIdx int) string {
			if lineIdx == 0 {
				return lipgloss.NewStyle().Foreground(googleYellow).Render("⧖ ")
			}
			return "  "
		})
	} else if m.state == stateShell {
		m.textArea.Placeholder = "Type your shell command"
		m.textArea.SetPromptFunc(2, func(lineIdx int) string {
			if lineIdx == 0 {
				return lipgloss.NewStyle().Foreground(googleYellow).Bold(true).Render("! ")
			}
			return "  "
		})
	} else {
		m.textArea.Placeholder = "Type your message or /help (Alt+Enter or ^J for newline)"
		m.textArea.SetPromptFunc(2, func(lineIdx int) string {
			if lineIdx == 0 {
				return promptStyle.Render("> ")
			}
			return "  "
		})
	}
}

func (m model) renderAgentPanel() string {
	var visibleAgents []*swarmAgent
	now := time.Now()
	for _, a := range m.agents {
		if a.resident || a.state == "active" || a.state == "waiting" || now.Sub(a.lastActive) < 30*time.Second {
			visibleAgents = append(visibleAgents, a)
		}
	}

	if len(visibleAgents) == 0 {
		return ""
	}

	fidelity := "medium"
	if len(visibleAgents) <= 8 {
		fidelity = "high"
	} else if len(visibleAgents) > 16 {
		fidelity = "low"
	}

	cols := 4
	if fidelity == "low" {
		cols = (m.width - 4) / 8
		if cols < 1 {
			cols = 1
		}
	}
	availableWidth := m.width - 4
	cardWidth := availableWidth / cols

	// Helper to ensure an icon or spinner is exactly 2 cells wide
	padPrefix := func(s string) string {
		w := runewidth.StringWidth(s)
		if w == 0 {
			return "  "
		}
		if w == 1 {
			return s + " "
		}
		return s
	}

	// Helper to render a perfectly aligned line using two fixed-width columns.
	// This approach is robust across modern and legacy terminals.
	renderLine := func(prefix string, text string, style lipgloss.Style, width int) string {
		prefixComp := padPrefix(prefix)
		
		contentWidth := width - 3 // 2 cells for prefix, 1 for spacer
		if runewidth.StringWidth(text) > contentWidth {
			text = runewidth.Truncate(text, contentWidth-1, "…")
		}
		contentComp := style.Width(contentWidth).Render(text)

		return lipgloss.JoinHorizontal(lipgloss.Left, prefixComp+" ", contentComp)
	}

	var cards []string
	for _, a := range visibleAgents {
		color := colorIdle
		border := lipgloss.NormalBorder()
		if a.state == "active" {
			color = colorActive
			border = lipgloss.ThickBorder()
		} else if a.state == "success" {
			color = colorSuccess
		} else if a.state == "waiting" {
			color = colorWaiting
		} else if a.state == "error" {
			color = colorError
		}

		cardStyle := lipgloss.NewStyle().Border(border).BorderForeground(color).Padding(0, 1).Width(cardWidth - 2)

		iconStr := "  "
		if a.state == "active" {
			iconStr = a.spin.View()
		} else if a.state == "success" {
			iconStr = "✓"
		} else if a.state == "error" {
			iconStr = "✗"
		} else if a.state == "waiting" {
			iconStr = "⧖"
		}

		stateLabel := strings.Title(a.state)
		if a.state == "idle" {
			stateLabel = "Ready"
		} else if a.state == "success" {
			stateLabel = "Complete"
		}

		contentWidth := cardWidth - 4
		line1 := renderLine(a.icon, a.name, lipgloss.NewStyle().Foreground(color).Bold(true), contentWidth)
		line2 := renderLine(iconStr, a.status, lipgloss.NewStyle().Foreground(tipColor), contentWidth)

		var cardContent string
		if fidelity == "high" {
			line3 := renderLine("", stateLabel, lipgloss.NewStyle().Foreground(tipColor).Italic(true).Faint(true), contentWidth)
			cardContent = lipgloss.JoinVertical(lipgloss.Left, line1, line2, line3)
			cardStyle = cardStyle.Height(3)
		} else if fidelity == "medium" {
			cardContent = lipgloss.JoinVertical(lipgloss.Left, line1, line2)
			cardStyle = cardStyle.Height(2)
		} else {
			// Low fidelity
			cardStyle = lipgloss.NewStyle().Border(border).BorderForeground(color).Width(6).Height(1).Padding(0, 1)
			cardContent = lipgloss.NewStyle().Width(4).Align(lipgloss.Center).Render(a.icon)
		}

		cards = append(cards, cardStyle.Render(cardContent))
	}

	var rows []string
	for i := 0; i < len(cards); i += cols {
		end := i + cols
		if end > len(cards) {
			end = len(cards)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...))
	}

	grid := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		MarginBottom(1).
		Width(m.width - 2).
		Render(grid)
}

func (m *model) findAgent(name string) *swarmAgent {
	name = strings.ToLower(name)
	for _, a := range m.agents {
		if strings.Contains(strings.ToLower(a.name), name) || strings.Contains(name, strings.ToLower(a.name)) {
			return a
		}
	}
	return nil
}

func getAgentIcon(name string) string {
	name = strings.ToLower(name)
	switch {
	case strings.Contains(name, "chat input") || strings.Contains(name, "chat_input"):
		return "💠"
	case strings.Contains(name, "router"):
		return "🧠"
	case strings.Contains(name, "investigator") || strings.Contains(name, "codebase"):
		return "🔍"
	case strings.Contains(name, "builder") || strings.Contains(name, "generator"):
		return "🛠️"
	case strings.Contains(name, "gitops") || strings.Contains(name, "github"):
		return "🐙"
	case strings.Contains(name, "researcher") || strings.Contains(name, "web"):
		return "🌐"
	case strings.Contains(name, "test"):
		return "🧪"
	case strings.Contains(name, "security") || strings.Contains(name, "audit"):
		return "🔐"
	case strings.Contains(name, "db") || strings.Contains(name, "architect"):
		return "💾"
	case strings.Contains(name, "codex"):
		return "📖"
	default:
		return "🤖"
	}
}

func (m *model) ensureAgent(name string) (*swarmAgent, tea.Cmd) {
	if a := m.findAgent(name); a != nil {
		return a, nil
	}

	// Create new dynamic agent
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorActive)

	newA := &swarmAgent{
		name:       name,
		icon:       getAgentIcon(name),
		status:     "Starting…",
		state:      "active",
		spin:       s,
		lastActive: time.Now(),
	}
	m.agents = append(m.agents, newA)
	return newA, s.Tick
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	var mainBody string
	agentPanelView := ""
	if m.showAgentPanel {
		agentPanelView = m.renderAgentPanel()
	}
	agentPanelHeight := lipgloss.Height(agentPanelView)

	if m.state == stateModelList {
		if m.loading {
			m.viewport.Height = m.height - 6 - agentPanelHeight
			mainBody = lipgloss.JoinVertical(lipgloss.Left, agentPanelView, m.viewport.View(), inputBoxStyle.Render(m.spinner.View()+" Fetching models…"))
		} else {
			mainBody = lipgloss.JoinVertical(lipgloss.Left, agentPanelView, lipgloss.NewStyle().Padding(1, 2).Render(m.listModel.View()))
		}
	} else {
		// 1.5. Autocomplete overlay
		var acView string
		if m.acActive && len(m.acMatches) > 0 {
			var lines []string

			// Account for borders (2), padding (2), and spacing (2) to calculate max width
			maxMatchWidth := m.width - 6

			for i, match := range m.acMatches {
				displayMatch := strings.ReplaceAll(match, "\n", " ")
				if len(displayMatch) > maxMatchWidth && maxMatchWidth > 3 {
					displayMatch = displayMatch[:maxMatchWidth-3] + "…"
				}

				if i == m.acIndex {
					lines = append(lines, lipgloss.NewStyle().Background(borderColor).Render(" "+displayMatch+" "))
				} else {
					lines = append(lines, lipgloss.NewStyle().Render(" "+displayMatch+" "))
				}
			}
			if m.acHasMore {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(" ▼ more"))
			}
			acBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderColor).Render(strings.Join(lines, "\n"))
			acView = acBox
		}

		// 2. Input
		inputView := inputBoxStyle.Render(m.textArea.View())

		if acView != "" {
			// Adjust viewport height to account for autocomplete overlay
			acHeight := lipgloss.Height(acView)
			m.viewport.Height = m.height - m.textArea.Height() - 3 - acHeight - agentPanelHeight
			vpView := m.viewport.View()
			mainBody = lipgloss.JoinVertical(lipgloss.Left, agentPanelView, vpView, lipgloss.NewStyle().PaddingLeft(1).Render(acView), inputView)
		} else {
			m.viewport.Height = m.height - m.textArea.Height() - 3 - agentPanelHeight
			vpView := m.viewport.View()
			mainBody = lipgloss.JoinVertical(lipgloss.Left, agentPanelView, vpView, inputView)
		}
	}

	// 3. Status
	w1 := m.width / 3
	w2 := m.width / 3
	w3 := m.width - w1 - w2

	baseStyle := statusBarStyle.Copy()

	p1Style := baseStyle.Copy().Width(w1).Align(lipgloss.Left)
	p2Style := baseStyle.Copy().Width(w2).Align(lipgloss.Center)
	p3Style := baseStyle.Copy().Width(w3).Align(lipgloss.Right)

	cwdText := " " + m.cwd
	if m.gitBranch != "" {
		mod := ""
		if m.gitModified {
			mod = "*"
		}
		cwdText += fmt.Sprintf(" (%s%s)", m.gitBranch, mod)
	}
	p1 := p1Style.Render(cwdText)

	modeText := "local mode"
	if m.state == stateShell {
		modeText = "shell mode"
		p2Style = p2Style.Foreground(googleYellow)
	} else if m.planMode {
		modeText = "plan mode"
		p2Style = p2Style.Foreground(googleYellow)
	}
	p2 := p2Style.Render(modeText)

	ctxCount := 0
	if m.manager != nil {
		ctxCount = len(m.manager.ListContext())
	}
	ctxStr := ""
	if ctxCount > 0 {
		ctxStr = fmt.Sprintf(" [%d pinned] ", ctxCount)
	}

	p3 := p3Style.Render(ctxStr + m.activeModel + " ")

	statusView := lipgloss.JoinHorizontal(lipgloss.Top, p1, p2, p3)
	// Apply Outer Border to main body
	boxedBody := appStyle.Width(m.width).Height(m.height).Render(mainBody)

	// Final layout: Boxed Body + Status Bar (outside border)
	return lipgloss.JoinVertical(lipgloss.Left, boxedBody, statusView)
}

func launchInteractiveShell(planMode bool, resume bool) error {
	p := tea.NewProgram(initialModel(planMode, resume), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("could not start interactive shell: %w", err)
	}
	return nil
}
