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

	// Agent Panel Colors
	colorIdle    = lipgloss.Color("#888888") // Lighter Gray
	colorActive  = lipgloss.Color("#4169E1") // Royal Blue
	colorSuccess = lipgloss.Color("#34A853") // Green
	colorWaiting = lipgloss.Color("#FBBC05") // Yellow
	colorError   = lipgloss.Color("#EA4335") // Red

	// Styles
	logoStyle = lipgloss.NewStyle().
			Bold(true)

	welcomeBoxStyle = lipgloss.NewStyle().
			Padding(1, 2)

	infoBoxStyle = lipgloss.NewStyle().
			Padding(1, 2)

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

	viewportStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

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
	telemetry  []string
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
	swarm        sdk.Swarm
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
	agents         []*swarmAgent
	ticks          int
	showAgentPanel bool

	welcomeScreen       []string
	cachedActivity      string
	lastActivityRefresh time.Time

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
	author = strings.ToLower(author)
	if author == "swarm_agent" || author == "swarm" || author == "agent" || author == "router_agent" {
		return lipgloss.NewStyle().Foreground(googleBlue).Render("❖ Swarm")
	} else if author == "input_agent" || author == "input" {
		return lipgloss.NewStyle().Foreground(googleYellow).Render("⚙ Input")
	} else if author == "output_agent" || author == "output" {
		return lipgloss.NewStyle().Foreground(googleRed).Render("🛡 Output")
	} else if author == "planning_agent" || author == "planning" {
		return lipgloss.NewStyle().Foreground(googleGreen).Render("📋 Planning")
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

func getRecentActivity(swarm sdk.Swarm, width int) string {
	sessions, _ := swarm.ListSessions(context.Background())
	commits, _ := sdk.GetRecentCommits(".", 3)

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(googleBlue).Render("Recent Activity") + "\n")

	// Calculate max content width (width minus borders(2), padding(8), and " git " prefix(5))
	maxContentWidth := width - 10 - 5
	if maxContentWidth < 5 {
		maxContentWidth = 5
	}

	hasActivity := false
	if len(commits) > 0 {
		for _, c := range commits {
			// Remove any newlines to prevent wrapping
			c = strings.ReplaceAll(c, "\n", " ")
			if runewidth.StringWidth(c) > maxContentWidth {
				c = runewidth.Truncate(c, maxContentWidth-1, "…")
			}
			sb.WriteString(lipgloss.NewStyle().Foreground(googleGreen).Render(" git ") + c + "\n")
		}
		hasActivity = true
	}

	sessionCount := 0
	for i := len(sessions) - 1; i >= 0 && sessionCount < 3; i-- {
		s := sessions[i]
		summary := strings.ReplaceAll(s.Summary, "\n", " ")
		if runewidth.StringWidth(summary) > maxContentWidth {
			summary = runewidth.Truncate(summary, maxContentWidth-1, "…")
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
	if width > 50 {
		sb.WriteString(lipgloss.NewStyle().Foreground(tipColor).Render("^O") + " toggle observe  " + lipgloss.NewStyle().Foreground(tipColor).Render("!") + " run shell\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(tipColor).Render("/plan") + " brainstorm   " + lipgloss.NewStyle().Foreground(tipColor).Render("/skills") + " skills")
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(tipColor).Render("^O") + " observe  " + lipgloss.NewStyle().Foreground(tipColor).Render("!") + " shell\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(tipColor).Render("/plan") + " plan   " + lipgloss.NewStyle().Foreground(tipColor).Render("/skills") + " skills")
	}

	return sb.String()
}

func initialModel(planMode bool, resume bool) (model, error) {
	ta := textarea.New()
	ta.Placeholder = "Type your message or /help (Alt+Enter or ^J for newline)"
	ta.Focus()
	ta.CharLimit = 2000
	ta.ShowLineNumbers = false
	ta.SetWidth(0) // Will be properly set in WindowSizeMsg
	ta.SetHeight(3)
	ta.SetPromptFunc(2, func(lineIdx int) string {
		if lineIdx == 0 {
			return "> "
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
		{name: "Input Agent", icon: "⚙", status: "Idle", state: "idle", spin: agentSpinner, resident: true, lastActive: time.Now()},
		{name: "Swarm Agent", icon: "◈", status: "Idle", state: "idle", spin: agentSpinner, resident: true, lastActive: time.Now()},
	}

	swarm, err := sdk.NewSwarm(sdk.SwarmConfig{ResumeLastSession: resume})
	if err != nil {
		return model{}, err
	}

	// Prepare splash screen components for dynamic rendering in View()
	greeting := fmt.Sprintf("\n\n%s %s!", lipgloss.NewStyle().Foreground(tipColor).Render("Welcome back,"), lipgloss.NewStyle().Bold(true).Render(getUserName()))
	logoAndGreeting := renderLogo() + greeting
	recentActivity := getRecentActivity(swarm, 40) // Default width, will be updated in View()

	return model{
		textArea:       ta,
		viewport:       vp,
		spinner:        s,
		listModel:      l,
		messages:       []string{"SPLASH_SCREEN"},
		history:        loadedHist,
		historyIdx:     len(loadedHist),
		swarm:          swarm,
		loading:        false,
		quitting:       false,
		planMode:       planMode,
		state:          stateChat,
		cwd:            cwd,
		gitBranch:      branch,
		gitModified:    modified,
		activeModel:    activeModel,
		renderer:       renderer,
		agents:         agents,
		showAgentPanel: true,
		welcomeScreen:  []string{logoAndGreeting, recentActivity},
	}, nil
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
			m.messages = nil
			m.updateViewport()
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
			oldAgentName := event.Agent
			if oldAgentName == "" {
				oldAgentName = m.activeAgent
			}

			if m.observeMode {
				m.observeLog = append(m.observeLog, fmt.Sprintf("[%s] ➡️ Delegated span to: %s", oldAgentName, lipgloss.NewStyle().Bold(true).Render(newAgentName)))
			}

			// Update AgentPanel
			if oldA := m.findAgent(oldAgentName); oldA != nil {
				oldA.update("success", "Span delegated")
			}
			var newA *swarmAgent
			newA, agentCmd = m.ensureAgent(newAgentName)
			newA.update("active", "Analyzing context…")

			// Sync active agent
			if !isMediationAgent(newAgentName) {
				m.activeAgent = newAgentName
			}
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

			// Identify the correct agent from the event
			targetAgentName := event.Agent
			if targetAgentName == "" {
				targetAgentName = m.activeAgent
			}

			m.statusMsg = "Running " + toolName + "…"

			// Update AgentPanel
			a, cmd := m.ensureAgent(targetAgentName)
			agentCmd = cmd
			a.update("active", "Tool: "+toolName)

			// Sync active agent
			if targetAgentName != "" && !isMediationAgent(targetAgentName) {
				m.activeAgent = targetAgentName
			}

			if m.observeMode {
				logEntry := fmt.Sprintf("[%s] Executing %s", targetAgentName, toolName)
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

			// Identify the correct agent from the event
			targetAgentName := event.Agent
			if targetAgentName == "" {
				targetAgentName = m.activeAgent
			}

			// Update AgentPanel
			a, cmd := m.ensureAgent(targetAgentName)
			agentCmd = cmd
			// Set back to idle to stop spinner, and keep tool name as status
			a.update("idle", "Completed "+toolName)
			a.telemetry = nil

			if m.observeMode {
				toolResult := ""
				if len(parts) > 1 {
					toolResult = parts[1]
				}
				logEntry := fmt.Sprintf("[%s] Completed %s", targetAgentName, toolName)
				if toolResult != "" {
					logEntry += " " + lipgloss.NewStyle().Foreground(tipColor).Render(toolResult)
				}
				m.observeLog = append(m.observeLog, logEntry)
				m.updateViewport()
			}
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.ChatEventTelemetry:
			targetAgentName := event.Agent
			if targetAgentName == "" {
				targetAgentName = m.activeAgent
			}
			// Update AgentPanel telemetry for active agent
			if a := m.findAgent(targetAgentName); a != nil {
				a.telemetry = append(a.telemetry, event.Content)
				if len(a.telemetry) > 3 {
					a.telemetry = a.telemetry[len(a.telemetry)-3:]
				}
				a.lastActive = time.Now()
			}

			// Sync active agent
			if targetAgentName != "" && !isMediationAgent(targetAgentName) {
				m.activeAgent = targetAgentName
			}
			return m, listenForStream(msg.ch)

		case sdk.ChatEventThought:
			// Update AgentPanel
			a, cmd := m.ensureAgent(event.Agent)
			agentCmd = cmd
			a.update("active", event.Content)

			// Sync active agent for error attribution
			if event.Agent != "" && !isMediationAgent(event.Agent) {
				m.activeAgent = event.Agent
			}

			if m.observeMode {
				thought := event.Content
				m.observeLog = append(m.observeLog, fmt.Sprintf("[%s] 🤔 %s", event.Agent, lipgloss.NewStyle().Foreground(tipColor).Italic(true).Render(thought)))
				m.updateViewport()
			}
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.ChatEventObserver:
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(googleBlue).Italic(true).Width(m.viewport.Width).Render("👀 "+event.Content))
			m.updateViewport()
			return m, listenForStream(msg.ch)

		case sdk.ChatEventReplan:
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(googleYellow).Bold(true).Width(m.viewport.Width).Render("🔄 "+event.Content))
			m.updateViewport()
			return m, listenForStream(msg.ch)

		case sdk.ChatEventDebug:
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Width(m.viewport.Width).Render(event.Content))
			m.updateViewport()
			return m, listenForStream(msg.ch)

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

			// Sync active agent
			if author != "" && !isMediationAgent(author) {
				m.activeAgent = author
			}

			text := event.Content

			// Fulfill Requirement 1 & 2: Invisible intermediaries
			if isMediationAgent(author) {
				// Still update viewport and panel, but don't show text to user
				m.updateViewport()
				return m, tea.Batch(listenForStream(msg.ch), agentCmd)
			}

			out := text
			if m.renderer != nil {
				if rOut, err := m.renderer.Render(text); err == nil {
					out = rOut
				}
			}

			badge := getAgentBadge(author)
			m.messages = append(m.messages, badge+"\n"+strings.TrimSpace(out))
			m.updateViewport()
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.ChatEventError:
			m.statusMsg = ""

			targetAgentName := event.Agent
			if targetAgentName == "" {
				targetAgentName = m.activeAgent
			}

			// Update AgentPanel
			a, cmd := m.ensureAgent(targetAgentName)
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
		m.activeAgent = "Swarm"
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
		m.width = msg.Width
		m.height = msg.Height
		m.cachedActivity = "" // Force refresh on next updateViewport

		m.textArea.SetWidth(m.width - 4)

		agentPanelHeight := 0
		if m.showAgentPanel {
			agentPanelHeight = lipgloss.Height(m.renderAgentPanel())
		}

		// Viewport height: Screen minus (Agent Panel + Input Area + Status Line + Borders)
		// Borders: 2 for Agent Panel, 2 for Viewport, 2 for Input = 6 lines
		// Status Line: 1 line
		// TextArea height: 3 lines
		m.viewport.Width = m.width - 4
		m.viewport.Height = m.height - m.textArea.Height() - agentPanelHeight - 7

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
	ch, err := m.swarm.Chat(ctx, input)
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

	// Reset AgentPanel for new span
	for _, a := range m.agents {
		a.update("idle", "Idle")
	}
	if r := m.findAgent("Swarm"); r != nil {
		r.update("active", "Processing input…")
	}

	nextInput := m.inputQueue[0]
	m.inputQueue = m.inputQueue[1:]

	m.loading = true
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelChat = cancel
	return m.callSDK(ctx, nextInput)
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
		models, err := m.swarm.ListModels(context.Background())
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
			"  /debug       Toggles debug mode to see full swarm trajectories.",
			"  /rewind [n]  Rewinds the conversation history by n turns (default 1).",
			"  /plan        Enter read-only plan mode to brainstorm safely.",
			"  /act         Exit plan mode and allow the agent to execute actions.",
			"  ! [command]  Execute a shell command directly.",
			"  /exit        Gracefully terminates the session.",
		)

		icon := agentMsgStyle.Render("✦ ")
		m.messages = append(m.messages, lipgloss.JoinHorizontal(lipgloss.Top, icon, helpText))
	case "/sessions":
		sessions, err := m.swarm.ListSessions(context.Background())
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
			if err := m.swarm.Reload(); err != nil {
				m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to reload skills: "+err.Error()))
			} else {
				m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Skills and agents reloaded successfully.")
			}
			return nil
		}

		skills := m.swarm.Skills()
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

		// In a real CLI, we would want to reload the Swarm here,
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
	case "/debug":
		enabled := !m.swarm.IsDebug()
		m.swarm.SetDebug(enabled)
		status := "enabled"
		if !enabled {
			status = "disabled"
		}
		m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+fmt.Sprintf("Debug mode (trajectories) %s.", status))
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
		m.messages = nil
		m.messages = append(m.messages, agentMsgStyle.Render("✦ ")+"Screen cleared.")
	case "/rewind":
		n := 1
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &n)
		}
		if err := m.swarm.Rewind(n); err != nil {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(errorColor).Render("Failed to rewind: "+err.Error()))
		} else {
			for _, a := range m.agents {
				a.update("idle", "Idle")
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
			files := m.swarm.ListContext()
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
			if err := m.swarm.AddContext(filePath); err != nil {
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
		m.swarm.DropContext(filePath)
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
	if m.width == 0 {
		return
	}

	// Prepare the dynamic message list
	var renderedMessages []string
	for _, msg := range m.messages {
		if msg == "SPLASH_SCREEN" {
			// SPLASH_SCREEN split: 2/3 Logo, 1/3 Recent Activity
			leftW := (m.width * 2) / 3
			rightW := m.width - leftW - 2

			// Refresh activity cache if needed (every 30s or if empty)
			if m.cachedActivity == "" || time.Since(m.lastActivityRefresh) > 30*time.Second {
				m.cachedActivity = getRecentActivity(m.swarm, rightW)
				m.lastActivityRefresh = time.Now()
			}

			left := welcomeBoxStyle.Copy().Width(leftW - 4).Render(m.welcomeScreen[0])
			right := infoBoxStyle.Copy().Width(rightW - 4).Render(m.cachedActivity)

			renderedMessages = append(renderedMessages, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
		} else {
			renderedMessages = append(renderedMessages, msg)
		}
	}

	// Add dynamic status if loading
	if m.loading {
		if m.observeMode && len(m.observeLog) > 0 {
			displayLogs := m.observeLog
			if len(displayLogs) > 10 {
				displayLogs = displayLogs[len(displayLogs)-10:]
			}

			observeBox := lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(googleYellow).
				Padding(0, 1).
				Width(m.width - 6).
				Render(lipgloss.JoinVertical(lipgloss.Left,
					lipgloss.NewStyle().Foreground(googleYellow).Bold(true).Render("👀 Observing Agent Execution:"),
					strings.Join(displayLogs, "\n"),
				))
			renderedMessages = append(renderedMessages, observeBox)
		}

		status := "Thinking…"
		if m.statusMsg != "" {
			status = m.statusMsg
		}
		agentLabel := m.activeAgent
		if agentLabel == "" {
			agentLabel = "Swarm"
		}
		renderedMessages = append(renderedMessages, agentMsgStyle.Render(fmt.Sprintf("✦ [%s] ", agentLabel))+m.spinner.View()+" "+status)
	}

	m.viewport.SetContent(strings.Join(renderedMessages, "\n\n"))
	m.viewport.GotoBottom()
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
				return "⧖ "
			}
			return "  "
		})
	} else if m.state == stateShell {
		m.textArea.Placeholder = "Type your shell command"
		m.textArea.SetPromptFunc(2, func(lineIdx int) string {
			if lineIdx == 0 {
				return "! "
			}
			return "  "
		})
	} else {
		m.textArea.Placeholder = "Type your message or /help (Alt+Enter or ^J for newline)"
		m.textArea.SetPromptFunc(2, func(lineIdx int) string {
			if lineIdx == 0 {
				return "> "
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

	// Calculate optimal columns based on count and width
	cols := 4
	if m.width < 100 {
		cols = 2
	} else if m.width < 140 {
		cols = 3
	}

	fidelity := "high"
	if len(visibleAgents) > cols*2 {
		fidelity = "medium"
	}
	if len(visibleAgents) > 16 {
		fidelity = "low"
	}

	availableWidth := m.width - 4
	if availableWidth < 20 {
		availableWidth = 20
	}

	cardWidth := availableWidth / cols
	if fidelity == "low" {
		cols = availableWidth / 8
		if cols < 1 {
			cols = 1
		}
		cardWidth = 8
	}

	// Helper to render a perfectly aligned line using two columns.
	// Prefix is exactly 3 cells wide.
	renderLine := func(prefix string, text string, style lipgloss.Style, width int) string {
		prefixComp := lipgloss.NewStyle().Width(3).Render(prefix)
		contentWidth := width - 3
		if runewidth.StringWidth(text) > contentWidth {
			text = runewidth.Truncate(text, contentWidth-1, "…")
		}
		contentComp := style.Width(contentWidth).Render(text)
		return lipgloss.JoinHorizontal(lipgloss.Left, prefixComp, contentComp)
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

		cardStyle := lipgloss.NewStyle().
			Border(border).
			BorderForeground(color).
			BorderBottom(false).
			Padding(0, 1).
			Width(cardWidth - 2)

		iconStr := "  "
		if a.state == "active" {
			iconStr = a.spin.View()
		} else if a.state == "success" {
			iconStr = "⚒" // Use 1-cell glyph that runewidth understands
		} else if a.state == "error" {
			iconStr = "✖"
		} else if a.state == "waiting" {
			iconStr = "⌚"
		}

		stateLabel := strings.Title(a.state)
		if a.state == "idle" {
			stateLabel = "Ready"
		} else if a.state == "success" {
			stateLabel = "Complete"
		}

		contentWidth := cardWidth - 4
		line1 := renderLine(a.icon, a.name, lipgloss.NewStyle().Foreground(color).Bold(true), contentWidth)

		mainText := a.status
		if len(a.telemetry) > 0 && a.state == "active" {
			mainText = a.telemetry[len(a.telemetry)-1]
		}
		line2 := renderLine(iconStr, mainText, lipgloss.NewStyle().Foreground(tipColor), contentWidth)

		var cardContent string
		if fidelity == "high" || fidelity == "medium" {
			cardContent = lipgloss.JoinVertical(lipgloss.Left, line1, line2)
			cardStyle = cardStyle.Height(2)
		} else {
			cardStyle = lipgloss.NewStyle().Border(border).BorderForeground(color).Width(6).Height(1).Padding(0, 1)
			cardContent = lipgloss.NewStyle().Width(4).Align(lipgloss.Center).Render(a.icon)
		}

		renderedCard := cardStyle.Render(cardContent)

		if fidelity == "high" || fidelity == "medium" {
			label := " " + stateLabel + " "
			labelLen := runewidth.StringWidth(label)
			remaining := cardWidth - 2 - labelLen
			rightDashCount := 2
			leftDashCount := remaining - rightDashCount
			if leftDashCount < 1 {
				leftDashCount = 1
				rightDashCount = remaining - leftDashCount
				if rightDashCount < 0 {
					rightDashCount = 0
				}
			}

			bottomLine := lipgloss.NewStyle().Foreground(color).Render(
				border.BottomLeft +
					strings.Repeat(border.Bottom, leftDashCount) +
					label +
					strings.Repeat(border.Bottom, rightDashCount) +
					border.BottomRight,
			)
			renderedCard = lipgloss.JoinVertical(lipgloss.Left, renderedCard, bottomLine)
		}
		cards = append(cards, renderedCard)
	}

	var rows []string
	rowStyle := lipgloss.NewStyle().Width(availableWidth)
	for i := 0; i < len(cards); i += cols {
		end := i + cols
		if end > len(cards) {
			end = len(cards)
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...)
		rows = append(rows, rowStyle.Render(row))
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
	// 1. Try exact match
	for _, a := range m.agents {
		if strings.ToLower(a.name) == name {
			return a
		}
	}
	// 2. Try suffix match (e.g., "input_agent" matches "input")
	for _, a := range m.agents {
		if strings.HasSuffix(strings.ToLower(a.name), name) || strings.HasSuffix(name, strings.ToLower(a.name)) {
			return a
		}
	}
	// 3. Try substring match (fallback)
	for _, a := range m.agents {
		if strings.Contains(strings.ToLower(a.name), name) || strings.Contains(name, strings.ToLower(a.name)) {
			return a
		}
	}
	return nil
}

func isMediationAgent(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "input") ||
		strings.Contains(name, "output") ||
		strings.Contains(name, "planning")
}

func getAgentIcon(name string) string {
	name = strings.ToLower(name)
	switch {
	case strings.Contains(name, "input"):
		return "⚙" // Gear (1-cell reliable)
	case strings.Contains(name, "swarm"):
		return "◈" // Diamond (1-cell reliable)
	case strings.Contains(name, "output"):
		return "🛡" // Shield
	case strings.Contains(name, "planning"):
		return "📋" // Clipboard
	case strings.Contains(name, "investigator") || strings.Contains(name, "codebase"):
		return "🔎" // Magnifying glass (usually 2-cell)
	default:
		return "○" // Circle (1-cell reliable)
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

	agentPanelView := ""
	if m.showAgentPanel {
		agentPanelView = m.renderAgentPanel()
	}
	agentPanelHeight := lipgloss.Height(agentPanelView)

	// Input Box with border
	inputView := inputBoxStyle.Width(m.width - 2).Render(m.textArea.View())
	inputHeight := lipgloss.Height(inputView)

	// Status line height
	statusHeight := 1

	// Recalculate viewport height to fill remaining space
	// Subtract 2 for the viewportStyle's own top/bottom borders
	m.viewport.Height = m.height - agentPanelHeight - inputHeight - statusHeight - 2
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}

	// Output Box (Viewport) with border
	vpView := viewportStyle.Width(m.width - 2).Height(m.viewport.Height).Render(m.viewport.View())

	// Main body is just the vertical stack of the three bordered sections
	mainBody := lipgloss.JoinVertical(lipgloss.Left, agentPanelView, vpView, inputView)

	// Bottom Status Line (no border, full width)
	w1, w2 := m.width/3, m.width/3
	w3 := m.width - w1 - w2
	p1 := statusBarStyle.Copy().Width(w1).Align(lipgloss.Left).Render(" " + m.cwd)
	p2 := statusBarStyle.Copy().Width(w2).Align(lipgloss.Center).Render("swarm mode")
	p3 := statusBarStyle.Copy().Width(w3).Align(lipgloss.Right).Render(m.activeModel + " ")
	statusView := lipgloss.JoinHorizontal(lipgloss.Top, p1, p2, p3)

	return lipgloss.JoinVertical(lipgloss.Left, mainBody, statusView)
}

func launchInteractiveShell(planMode bool, resume bool) error {
	m, err := initialModel(planMode, resume)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error: %w", err)
	}
	return nil
}
