package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/glamour"
	"github.com/mattn/go-runewidth"
	"github.com/sahilm/fuzzy"

	"github.com/dewitt/swarm/pkg/eval"
	"github.com/dewitt/swarm/pkg/sdk"
	"github.com/dewitt/swarm/pkg/web"
)

var (
	// Brand Colors
	googleBlue   = lipgloss.Color("#4285F4")
	googleRed    = lipgloss.Color("#EA4335")
	googleYellow = lipgloss.Color("#FBBC05")
	googleGreen  = lipgloss.Color("#34A853")

	primaryColor = googleBlue
	tipColor     = lipgloss.Color("#666666")
	errorColor   = googleRed
	// Agent Panel Colors
	colorIdle    = lipgloss.Color("#888888") // Lighter Gray
	colorActive  = lipgloss.Color("#4169E1") // Royal Blue
	colorSuccess = lipgloss.Color("#34A853") // Green
	colorWaiting = lipgloss.Color("#FBBC05") // Yellow
	colorError   = lipgloss.Color("#EA4335") // Red

	promptStyle = lipgloss.NewStyle().
			Foreground(googleBlue).
			Bold(true)

	agentMsgStyle = lipgloss.NewStyle().
			Foreground(googleGreen).
			Bold(true)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)
)

func renderScrollbar(height int, scrollPercent float64, color color.Color) []string {
	if height <= 0 {
		return nil
	}

	style := lipgloss.NewStyle().Foreground(color)
	handle := "┃"
	track := "│"

	cursorPos := int(scrollPercent * float64(height-1))
	if cursorPos < 0 {
		cursorPos = 0
	}
	if cursorPos >= height {
		cursorPos = height - 1
	}

	var lines []string
	for i := 0; i < height; i++ {
		if i == cursorPos {
			lines = append(lines, style.Render(handle))
		} else {
			lines = append(lines, style.Faint(true).Render(track))
		}
	}
	return lines
}

type streamMsg struct {
	event sdk.ObservableEvent
	ch    <-chan sdk.ObservableEvent
}

type streamDoneMsg struct{}

type streamErrMsg struct {
	err error
}

type responseMsg struct {
	text    string
	err     error
	isShell bool
	pgid    int
}

type (
	globalSummaryMsg        string
	triggerGlobalSummaryMsg struct{}
)

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

type workspaceFilesMsg struct {
	files []string
}

func fetchWorkspaceFiles() tea.Cmd {
	return func() tea.Msg {
		var items []string
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "git", "ls-files").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l != "" {
					items = append(items, l)
				}
			}
		}
		return workspaceFilesMsg{files: items}
	}
}

type uiSpan struct {
	ID       string
	Name     string
	ParentID string
	Agent    string
	Status   string
	Thought  string
	ToolName string
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
	webServer    *web.Server

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
	spans          map[string]*uiSpan
	showAgentPanel bool
	globalSummary  string

	// Autocomplete state
	workspaceFiles []string
	acMatches      []string
	acIndex        int
	acActive       bool
	acPrefix       string
	acMode         string // "file", "command", "history"
	acHasMore      bool

	isDark bool

	// Background processes
	bgPGIDs []int

	// Boot Logo Animation
	logoFrame   int
	hasRunTasks bool
}

type themeColors struct {
	borderColor   color.Color
	statusBg      color.Color
	statusFg      color.Color
	placeholderFg color.Color
	labelFg       color.Color
	logoMutedFg   color.Color
	logoCaretFg   color.Color
	logoForestFgs []color.Color
}

func defaultTheme(isDark bool) themeColors {
	ld := lipgloss.LightDark(isDark)
	return themeColors{
		borderColor:   ld(lipgloss.Color("#D9D9D9"), lipgloss.Color("#444444")),
		statusBg:      ld(lipgloss.Color("#EBEBEB"), lipgloss.Color("#1A1A1A")),
		statusFg:      ld(lipgloss.Color("#555555"), lipgloss.Color("#888888")),
		placeholderFg: ld(lipgloss.Color("#E0E0E0"), lipgloss.Color("#262626")),
		labelFg:       ld(lipgloss.Color("#777777"), lipgloss.Color("#AAAAAA")),
		logoMutedFg:   ld(lipgloss.Color("#D4D4D4"), lipgloss.Color("#404040")),
		logoCaretFg:   ld(lipgloss.Color("#1E3A8A"), lipgloss.Color("#60A5FA")), // Dark blue for >, light blue for dark mode
		logoForestFgs: []color.Color{
			ld(lipgloss.Color("#153324"), lipgloss.Color("#86EFAC")), // Light: Dark, Dark: Light
			ld(lipgloss.Color("#2F7452"), lipgloss.Color("#4AB57F")), // Light: Med, Dark: Med
			ld(lipgloss.Color("#4AB57F"), lipgloss.Color("#3D9469")), // Light: Light, Dark: Dark
			ld(lipgloss.Color("#2F7452"), lipgloss.Color("#4AB57F")), // Light: Med, Dark: Med
			ld(lipgloss.Color("#153324"), lipgloss.Color("#86EFAC")), // Light: Dark, Dark: Light
		},
	}
}

func (m model) theme() themeColors {
	return defaultTheme(m.isDark)
}

func updateAutocomplete(m *model) {
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

	// Do not trigger standard autocomplete while traversing history via up/down arrows
	if m.historyIdx < len(m.history) {
		m.acActive = false
		m.acMode = ""
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

func getHistoryFile() string {
	dir, err := sdk.GetConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "history.json")
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
	_ = os.MkdirAll(dir, 0o755)

	// Keep only the last 1000 items to prevent the file from growing indefinitely
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}

	b, err := json.MarshalIndent(history, "", "  ")
	if err == nil {
		_ = os.WriteFile(file, b, 0o600)
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

type logoTickMsg time.Time

func doLogoTick() tea.Cmd {
	return tea.Tick(time.Millisecond*30, func(t time.Time) tea.Msg {
		return logoTickMsg(t)
	})
}

func initialModel(planMode bool, resume bool) (model, error) {
	isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)

	ta := textarea.New()
	ta.SetStyles(textarea.DefaultStyles(isDark))
	ta.Placeholder = "Type your message or /help (Alt+Enter or ^J for newline)"
	ta.Focus()
	ta.CharLimit = 2000
	ta.ShowLineNumbers = false
	ta.SetWidth(0) // Will be properly set in WindowSizeMsg
	ta.SetHeight(3)
	ta.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "> "
		}
		return "  "
	})

	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
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
	if !isDark {
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

	var webServer *web.Server
	if !strings.HasSuffix(os.Args[0], ".test") {
		webServer = web.NewServer(":5050")
		go func() {
			_ = webServer.Start()
		}()
	}

	// Use in-memory DB for tests to prevent SQLite file locks
	swarmConfig := sdk.SwarmConfig{ResumeLastSession: resume}
	if strings.HasSuffix(os.Args[0], ".test") {
		swarmConfig.DatabaseURI = "file::memory:?cache=shared"
	}
	swarm, err := sdk.NewSwarm(swarmConfig)
	if err != nil {
		return model{}, err
	}

	contextFiles := swarm.ListContext()
	return model{
		textArea:       ta,
		viewport:       vp,
		spinner:        s,
		listModel:      l,
		messages:       []string{buildBootMessage(cwd, branch, modified, isDark, activeModel, contextFiles, swarm.SessionID(), resume)},
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
		webServer:      webServer,
		agents:         agents,
		spans:          make(map[string]*uiSpan),
		showAgentPanel: true,
		isDark:         isDark,
		hasRunTasks:    resume,
	}, nil
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink, m.spinner.Tick, doGitTick(), tea.RequestBackgroundColor, doLogoTick()}
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
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if keyMsg.String() == "ctrl+c" {
			if m.loading {
				if m.cancelChat != nil {
					m.cancelChat()
					m.cancelChat = nil
				}
				m.inputQueue = nil
				m.loading = false
				m.statusMsg = ""
				m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Bold(true).Render("✦ [System] Swarm execution forcefully halted by user."))
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
		case tea.KeyPressMsg:
			if msg.String() == "esc" {
				m.state = stateChat
				return m, tea.ClearScreen
			}
			if msg.String() == "enter" && m.listModel.FilterState() != list.Filtering {
				if i, ok := m.listModel.SelectedItem().(item); ok {
					newModelName := i.name
					cfg, err := sdk.LoadConfig()
					if err == nil {
						cfg.Model = newModelName
						_ = sdk.SaveConfig(cfg)
						m.activeModel = newModelName
						m.appendMessage(agentMsgStyle.Render("✦ ") + fmt.Sprintf("Model preference saved as '%s'.", newModelName))
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
	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.textArea.SetStyles(textarea.DefaultStyles(m.isDark))
		// Update glamour style based on background
		style := "dark"
		if !m.isDark {
			style = "light"
		}
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithStandardStyle(style),
				glamour.WithWordWrap(m.viewport.Width()),
			)
		}
		return m, nil

	case workspaceFilesMsg:
		m.workspaceFiles = msg.files
		updateAutocomplete(&m)
		return m, nil

	case gitStatusMsg:
		m.gitBranch = msg.branch
		m.gitModified = msg.modified
		return m, nil

	case gitTickMsg:
		return m, tea.Batch(checkGitStatus(), doGitTick())

	case logoTickMsg:
		if !m.hasRunTasks && m.logoFrame < 60 {
			m.logoFrame++
			return m, doLogoTick()
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if m.loading {
				if m.cancelChat != nil {
					m.cancelChat()
					m.cancelChat = nil
				}
				m.inputQueue = nil
				m.loading = false
				m.statusMsg = ""
				m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Bold(true).Render("✦ [System] Swarm execution forcefully halted by user."))
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
		case "ctrl+j":
			m.textArea.InsertString("\n")
			return m, nil
		case "ctrl+o":
			m.observeMode = !m.observeMode
			state := "disabled"
			if m.observeMode {
				state = "enabled"
			}
			m.appendMessage(agentMsgStyle.Render("✦ ") + fmt.Sprintf("Observe mode %s.", state))
			m.updateViewport()
			return m, nil
		case "ctrl+l":
			m.messages = nil
			m.updateViewport()
			return m, tea.ClearScreen
		case "ctrl+r":
			if m.state == stateChat && !m.loading {
				m.acActive = true
				m.acMode = "history"
				m.acPrefix = ""
				m.textArea.SetValue("")
				updateAutocomplete(&m)
				return m, tea.ClearScreen
			}
		case "alt+enter":
			m.textArea.InsertString("\n")
			return m, nil
		case "enter":
			if m.loading {
				input := m.textArea.Value()
				trimmedInput := strings.TrimSpace(input)
				if trimmedInput != "" {
					m.inputQueue = append(m.inputQueue, trimmedInput)
					m.textArea.Reset()

					queuedIcon := lipgloss.NewStyle().Foreground(googleYellow).Render("⧖ ")
					m.appendMessage(queuedIcon + trimmedInput)
					m.updateViewport()
				}
				return m, nil
			}

			if m.acActive && len(m.acMatches) > 0 {
				val := m.textArea.Value()
				lastSpace := strings.LastIndexAny(val, " \n")
				var lastWord string
				if lastSpace == -1 {
					lastWord = val
				} else {
					lastWord = val[lastSpace+1:]
				}

				if m.acMode == "history" {
					if val == m.acMatches[m.acIndex] {
						m.acActive = false
						m.acMode = ""
						// Fall through to submit
					} else {
						m.textArea.SetValue(m.acMatches[m.acIndex])
						m.textArea.CursorEnd()
						m.acActive = false
						m.acMode = ""
						return m, nil
					}
				} else {
					if lastWord == m.acPrefix+m.acMatches[m.acIndex] {
						// If the user fully typed the exact suggestion, let Enter submit it
						// (Optionally ensure space is added if they want to keep typing, but for Enter, they want to submit)
						m.acActive = false
						m.acMode = ""
						// Fall through to submit
					} else {
						m.textArea.SetValue(val[:lastSpace+1] + m.acPrefix + m.acMatches[m.acIndex] + " ")
						m.textArea.CursorEnd()
						m.acActive = false
						m.acMode = ""
						return m, nil
					}
				}
			}

			input := m.textArea.Value()
			trimmedInput := strings.TrimSpace(input)

			if m.state == stateShell && (trimmedInput == "exit" || trimmedInput == "quit") {
				m.state = stateChat
				m.appendMessage(agentMsgStyle.Render("✦ ") + "Exited shell execution mode.")
				m.textArea.Reset()
				m.updateViewport()
				m.updateInputStyle()
				return m, nil
			}

			if input != "" {
				if m.state == stateShell {
					m.appendMessage(lipgloss.NewStyle().Width(m.viewport.Width()).Render(lipgloss.NewStyle().Foreground(googleYellow).Bold(true).Render("! ") + input))
				} else {
					m.appendMessage(lipgloss.NewStyle().Width(m.viewport.Width()).Render(promptStyle.Render("> ") + input))
				}

				if len(m.history) == 0 || m.history[len(m.history)-1] != input {
					m.history = append(m.history, input)
					if len(m.history) > 1000 {
						m.history = m.history[len(m.history)-1000:]
					}
					saveHistory(m.history)
				}
				m.historyIdx = len(m.history)

				m.textArea.Reset()

				if m.state == stateShell {
					m.loading = true
					m.globalSummary = "Running shell command..."
					ctx, cancel := context.WithCancel(context.Background())
					m.cancelChat = cancel
					cmds = append(cmds, m.runShellCommand(ctx, input), m.spinner.Tick)
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
					m.globalSummary = "Running shell command..."
					ctx, cancel := context.WithCancel(context.Background())
					m.cancelChat = cancel
					cmds = append(cmds, m.runShellCommand(ctx, strings.TrimSpace(strings.TrimPrefix(input, "!"))), m.spinner.Tick)
				} else {
					m.inputQueue = append(m.inputQueue, trimmedInput)
					cmds = append(cmds, m.dequeueAndRun())
				}
				m.updateViewport()
			}
			return m, tea.Batch(cmds...)
		case "tab":
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
		case "up":
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
		case "down":
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
		case "pgup", "pgdown":
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd
		}

	case tea.MouseMsg:
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		return m, vpCmd
	case triggerGlobalSummaryMsg:
		if m.loading {
			return m, m.fetchGlobalSummary()
		}
		return m, nil

	case globalSummaryMsg:
		if m.loading {
			m.globalSummary = string(msg)
			m.updateViewport()
			return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return triggerGlobalSummaryMsg{}
			})
		}
		return m, nil

	case streamMsg:
		event := msg.event
		var agentCmd tea.Cmd

		if m.webServer != nil {
			m.webServer.Broadcast(event)
		}

		if event.SpanID != "" {
			m.hasRunTasks = true
			span, ok := m.spans[event.SpanID]
			if !ok {
				span = &uiSpan{
					ID:       event.SpanID,
					Name:     event.TaskName,
					ParentID: event.ParentID,
					Agent:    event.AgentName,
				}
				if span.Name == "" {
					span.Name = "Task"
				}
				m.spans[event.SpanID] = span
			}
			span.Status = string(event.State)
			if event.Thought != "" {
				span.Thought = event.Thought
			}
			if event.ToolName != "" {
				span.ToolName = event.ToolName
			}
			if event.ObserverSummary != "" {
				span.Thought = event.ObserverSummary
			}
		}

		switch event.State {
		case sdk.AgentStatePending:
			m.updateViewport()
			return m, listenForStream(msg.ch)

		case sdk.AgentStateSpawning:
			targetAgentName := event.AgentName
			if targetAgentName == "" {
				targetAgentName = m.activeAgent
			}
			a, cmd := m.ensureAgent(targetAgentName)
			agentCmd = cmd
			a.update("active", "Initializing...")

			if !isMediationAgent(targetAgentName) {
				m.activeAgent = targetAgentName
			}
			m.statusMsg = ""
			m.updateViewport()
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.AgentStateThinking:
			a, cmd := m.ensureAgent(event.AgentName)
			agentCmd = cmd
			if event.Thought != "" {
				a.update("active", event.Thought)
			} else {
				a.update("active", "Thinking...")
			}

			if event.PGID > 0 {
				m.bgPGIDs = append(m.bgPGIDs, event.PGID)
			}

			if event.AgentName != "" && !isMediationAgent(event.AgentName) {
				m.activeAgent = event.AgentName
			}

			if m.observeMode && event.Thought != "" {
				m.observeLog = append(m.observeLog, fmt.Sprintf("[%s] 🤔 %s", event.AgentName, lipgloss.NewStyle().Foreground(tipColor).Italic(true).Render(event.Thought)))
				m.updateViewport()
			}
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.AgentStateExecuting:
			targetAgentName := event.AgentName
			if targetAgentName == "" {
				targetAgentName = m.activeAgent
			}

			a, cmd := m.ensureAgent(targetAgentName)
			agentCmd = cmd

			if event.ObserverSummary != "" {
				a.update("active", event.ObserverSummary)
				m.statusMsg = event.ObserverSummary
				if m.observeMode {
					m.observeLog = append(m.observeLog, fmt.Sprintf("[%s] 💡 %s", targetAgentName, lipgloss.NewStyle().Foreground(googleBlue).Render(event.ObserverSummary)))
					m.updateViewport()
				}
			} else if event.ToolName != "" {
				a.update("active", "Tool: "+event.ToolName)
				m.statusMsg = "Running " + event.ToolName + "…"

				if m.observeMode {
					logEntry := fmt.Sprintf("[%s] Executing %s", targetAgentName, event.ToolName)
					if len(event.ToolArgs) > 0 {
						argsJSON, _ := json.Marshal(event.ToolArgs)
						logEntry += " " + lipgloss.NewStyle().Foreground(tipColor).Render(string(argsJSON))
					}
					m.observeLog = append(m.observeLog, logEntry)
					m.updateViewport()
				}
			}

			if targetAgentName != "" && !isMediationAgent(targetAgentName) {
				m.activeAgent = targetAgentName
			}
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.AgentStateWaiting:
			m.appendMessage(lipgloss.NewStyle().Foreground(googleBlue).Italic(true).Width(m.viewport.Width()).Render("👀 " + event.ObserverSummary))
			m.updateViewport()
			return m, listenForStream(msg.ch)

		case sdk.AgentStateComplete:
			m.statusMsg = ""
			author := event.AgentName
			if author == "" {
				author = "agent"
			}
			a, cmd := m.ensureAgent(author)
			agentCmd = cmd

			if event.FinalContent != "" {
				a.update("success", "Response ready")
				if author != "" && !isMediationAgent(author) {
					m.activeAgent = author
				}

				if isMediationAgent(author) {
					m.updateViewport()
					return m, tea.Batch(listenForStream(msg.ch), agentCmd)
				}

				m.lastResponse = event.FinalContent
				out := event.FinalContent
				if m.renderer != nil {
					if rOut, err := m.renderer.Render(out); err == nil {
						out = rOut
					}
				}

				// Fix alignment: join the icon and the rendered text horizontally so they line up
				icon := agentMsgStyle.Render("✦ ")
				m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, strings.TrimSpace(out)))
			} else {
				a.update("idle", "Completed task")
			}
			m.updateViewport()
			return m, tea.Batch(listenForStream(msg.ch), agentCmd)

		case sdk.AgentStateError:
			m.statusMsg = ""
			targetAgentName := event.AgentName
			if targetAgentName == "" {
				targetAgentName = m.activeAgent
			}
			a, cmd := m.ensureAgent(targetAgentName)
			agentCmd = cmd
			a.update("error", "Failed")

			errMsg := event.FinalContent
			if event.Error != nil {
				errMsg = event.Error.Error()
			}
			m.appendMessage(errorMsgStyle.Width(m.viewport.Width()).Render(fmt.Sprintf("Error: %s", errMsg)))
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
		m.spans = make(map[string]*uiSpan)
		m.updateViewport()
		return m, m.dequeueAndRun()

	case streamErrMsg:
		m.loading = false
		m.statusMsg = ""
		m.observeLog = nil
		m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Width(m.viewport.Width()).Render("Error: " + msg.err.Error()))
		m.updateViewport()
		return m, m.dequeueAndRun()

	case responseMsg:
		m.loading = false
		if msg.pgid > 0 {
			m.bgPGIDs = append(m.bgPGIDs, msg.pgid)
		}
		if msg.err != nil {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Width(m.viewport.Width()).Render("Error: " + msg.err.Error()))
		} else if msg.isShell {
			// Style for shell output - slightly indented and perhaps a different color
			shellStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).PaddingLeft(2).Width(m.viewport.Width())
			m.appendMessage(shellStyle.Render(msg.text))
		} else {
			m.lastResponse = msg.text
			out := msg.text
			if m.renderer != nil {
				if rOut, err := m.renderer.Render(msg.text); err == nil {
					out = rOut
				}
			}
			icon := agentMsgStyle.Render("✦ ")
			m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, strings.TrimSpace(out)))
		}
		m.updateViewport()
		return m, m.dequeueAndRun()

	case modelsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.state = stateChat
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Width(m.viewport.Width()).Render("Error fetching models: " + msg.err.Error()))
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

		m.textArea.SetWidth(m.width - 4)

		agentPanelHeight := 0
		if m.showAgentPanel {
			agentPanelHeight = lipgloss.Height(m.renderAgentPanel())
		}

		// Viewport height: Screen minus (Agent Panel + Input Area + Status Line + Borders)
		// Borders: 2 for Agent Panel, 2 for Viewport, 2 for Input = 6 lines
		// Status Line: 1 line
		// TextArea height: 3 lines
		m.viewport.SetWidth(m.width - 6)
		m.viewport.SetHeight(m.height - m.textArea.Height() - agentPanelHeight - 7)

		// Update glamour word wrap
		if m.renderer != nil {
			style := "dark"
			if !m.isDark {
				style = "light"
			}
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithStandardStyle(style),
				glamour.WithWordWrap(m.viewport.Width()),
			)
		}

		// List Model: account for outer border (2) and padding (4 horizontal, 2 vertical)
		m.listModel.SetSize(m.width-6, m.height-4)
		m.updateViewport()
		return m, nil
	}

	m.textArea, tiCmd = m.textArea.Update(msg)
	cmds = append(cmds, tiCmd)

	// Filter out leaked SGR mouse sequences due to dropped escape bytes (Issue #3)
	val := m.textArea.Value()
	if strings.Contains(val, "[<") {
		// e.g., [<65;74; 31M or [<65;44; 34m
		re := regexp.MustCompile(`\[<\d+;\s*\d+;\s*\d+[Mm]`)
		newVal := re.ReplaceAllString(val, "")
		if newVal != val {
			m.textArea.SetValue(newVal)
			m.textArea.CursorEnd()
		}
	}

	if m.state == stateChat {
		if strings.Contains(val, "@") && m.workspaceFiles == nil {
			cmds = append(cmds, fetchWorkspaceFiles())
		}
		updateAutocomplete(&m)
	}

	// Check for automatic shell mode toggling
	val = m.textArea.Value()
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
	m.spans = make(map[string]*uiSpan)
	for _, a := range m.agents {
		a.update("idle", "Idle")
	}
	if r := m.findAgent("Swarm"); r != nil {
		r.update("active", "Processing input…")
	}

	nextInput := m.inputQueue[0]
	m.inputQueue = m.inputQueue[1:]

	m.loading = true
	m.globalSummary = "Starting execution..."
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelChat = cancel
	return tea.Batch(
		m.callSDK(ctx, nextInput),
		tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return triggerGlobalSummaryMsg{} }),
		m.spinner.Tick,
	)
}

func listenForStream(ch <-chan sdk.ObservableEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return streamMsg{event: event, ch: ch}
	}
}

func (m *model) fetchGlobalSummary() tea.Cmd {
	return func() tea.Msg {
		if len(m.spans) == 0 {
			return globalSummaryMsg("Waiting for tasks to start...")
		}
		var lines []string
		for _, s := range m.spans {
			if s.Status == "executing" || s.Status == "thinking" || s.Status == "spawning" {
				thought := s.Thought
				if s.ToolName != "" {
					thought = "Running " + s.ToolName
				}
				lines = append(lines, fmt.Sprintf("- [%s] %s: %s", s.Agent, s.Name, thought))
			}
		}
		if len(lines) == 0 {
			return globalSummaryMsg("Waiting for tasks to start...")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		summary, err := m.swarm.SummarizeState(ctx, strings.Join(lines, "\n"))
		if err != nil {
			return globalSummaryMsg("Waiting for tasks... (" + err.Error() + ")")
		}
		return globalSummaryMsg(summary)
	}
}

func (m model) runShellCommand(ctx context.Context, command string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		out, err := cmd.CombinedOutput()
		pgid, _ := syscall.Getpgid(cmd.Process.Pid)
		if err != nil {
			return responseMsg{text: string(out), err: err, isShell: true, pgid: pgid}
		}
		return responseMsg{text: string(out), isShell: true, pgid: pgid}
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
		m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, aboutText))
	case "/help":
		helpText := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Swarm CLI Help Menu"),
			"",
			"  /about       Displays version and build information.",
			"  /copy        Copies the most recent unformatted AI response to your clipboard.",

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
			"  /web         Launch the Web Agent Panel in your browser.",
			"  /rewind [n]  Rewinds the conversation history by n turns (default 1).",
			"  /plan        Enter read-only plan mode to brainstorm safely.",
			"  /act         Exit plan mode and allow the agent to execute actions.",
			"  ! [command]  Execute a shell command directly.",
			"  /exit        Gracefully terminates the session.",
		)

		icon := agentMsgStyle.Render("✦ ")
		m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, helpText))
	case "/sessions":
		sessions, err := m.swarm.ListSessions(context.Background())
		if err != nil {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to list sessions: " + err.Error()))
			return nil
		}
		if len(sessions) == 0 {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "No sessions found in the database.")
			return nil
		}

		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Stored Sessions (%d)", len(sessions))))
		lines = append(lines, "")

		for _, s := range sessions {
			id := lipgloss.NewStyle().Foreground(primaryColor).Render(s.ID)
			summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Italic(true)
			content := fmt.Sprintf("- %s (Last Updated: %s)", id, s.UpdatedAt)
			if s.Summary != "" && s.Summary != s.ID {
				// Clean up the summary text
				cleanSummary := strings.ReplaceAll(s.Summary, "\n", " ")
				content += fmt.Sprintf("\n  %s", summaryStyle.Render(cleanSummary))
			}
			lines = append(lines, content)
			lines = append(lines, "") // Add a blank line between sessions for readability
		}

		icon := agentMsgStyle.Render("✦ ")
		m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, lines...)))
	case "/web":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := exec.CommandContext(ctx, "open", "http://localhost:5050").Start()
		if err != nil {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to open browser: " + err.Error()))
		} else {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "Opened Web Agent Panel at http://localhost:5050")
		}
	case "/eval":
		scenarios, err := eval.GetScenarios()
		if err != nil {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to load scenarios: " + err.Error()))
			return nil
		}

		if len(parts) == 1 || parts[1] == "list" {
			var lines []string
			lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Evaluation Scenarios (%d)", len(scenarios))))
			lines = append(lines, "")
			wrapWidth := m.viewport.Width() - 4
			if wrapWidth < 20 {
				wrapWidth = 20
			}
			for _, s := range scenarios {
				content := fmt.Sprintf("- %s: %s", lipgloss.NewStyle().Foreground(primaryColor).Render(s.ID), s.Name)
				lines = append(lines, lipgloss.NewStyle().Width(wrapWidth).Render(content))
			}
			icon := agentMsgStyle.Render("✦ ")
			m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, lines...)))
			return nil
		}

		target := parts[1]
		var found *eval.Scenario
		for _, s := range scenarios {
			if s.ID == target {
				found = &s
				break
			}
		}

		if found == nil {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render(fmt.Sprintf("Scenario '%s' not found.", target)))
			return nil
		}

		// Print context to chat
		var ctxLines []string
		ctxLines = append(ctxLines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Running Evaluation: %s", found.Name)))
		ctxLines = append(ctxLines, "")
		ctxLines = append(ctxLines, lipgloss.NewStyle().Italic(true).Render("Prompt: "+found.Prompt))
		ctxLines = append(ctxLines, "")
		ctxLines = append(ctxLines, lipgloss.NewStyle().Italic(true).Render("Rubric: "+found.Rubric))

		icon := agentMsgStyle.Render("✦ ")
		m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, ctxLines...)))

		// Set up the evaluation runner
		evalChan := make(chan sdk.ObservableEvent, 100)
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Error: GOOGLE_API_KEY environment variable is required."))
			return nil
		}

		m.loading = true
		m.globalSummary = fmt.Sprintf("Evaluating %s...", found.ID)

		// Reset AgentPanel
		m.spans = make(map[string]*uiSpan)
		for _, a := range m.agents {
			a.update("idle", "Idle")
		}

		go func(scenario eval.Scenario) {
			evaluator, err := eval.NewEvaluator(apiKey)
			if err != nil {
				evalChan <- sdk.ObservableEvent{AgentName: "Swarm", State: sdk.AgentStateError, Error: err}
				close(evalChan)
				return
			}

			res, err := evaluator.Run(context.Background(), scenario, eval.WithProgress(func(e sdk.ObservableEvent) {
				evalChan <- e
			}))

			if err != nil {
				evalChan <- sdk.ObservableEvent{AgentName: "Swarm", State: sdk.AgentStateError, Error: err}
			} else {
				status := "FAIL"
				if res.Passed {
					status = "PASS"
				}
				msg := fmt.Sprintf("Evaluation Complete: %s\n\nReasoning: %s", status, res.Reasoning)
				evalChan <- sdk.ObservableEvent{AgentName: "Evaluator", State: sdk.AgentStateComplete, FinalContent: msg}
			}
			close(evalChan)
		}(*found)

		return tea.Batch(
			listenForStream(evalChan),
			tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return triggerGlobalSummaryMsg{} }),
			m.spinner.Tick,
		)

	case "/skills":
		if len(parts) > 1 && parts[1] == "reload" {
			if err := m.swarm.Reload(); err != nil {
				m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to reload skills: " + err.Error()))
			} else {
				m.appendMessage(agentMsgStyle.Render("✦ ") + "Skills and agents reloaded successfully.")
			}
			return nil
		}

		skills := m.swarm.Skills()
		if len(skills) == 0 {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "No dynamic skills are currently loaded.")
			return nil
		}

		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Loaded Skills"))
		lines = append(lines, "")

		// Use the actual viewport width for text wrapping (minus icon and padding)
		wrapWidth := m.viewport.Width() - 4
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
		m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, lines...)))
	case "/model":
		if len(parts) < 2 {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "Usage: /model <name> OR /model list\nCurrent mode is: auto")
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
				m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to save config: " + err.Error()))
				return nil
			}
			m.activeModel = newModelName
			m.appendMessage(agentMsgStyle.Render("✦ ") + fmt.Sprintf("Model preference saved as '%s'.", newModelName))
		} else {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to load config: " + err.Error()))
		}
	case "/debug":
		enabled := !m.swarm.IsDebug()
		m.swarm.SetDebug(enabled)
		status := "enabled"
		if !enabled {
			status = "disabled"
		}
		m.appendMessage(agentMsgStyle.Render("✦ ") + fmt.Sprintf("Debug mode (trajectories) %s.", status))
	case "/copy":
		if m.lastResponse != "" {
			if err := clipboard.WriteAll(m.lastResponse); err != nil {
				m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to copy to clipboard: " + err.Error()))
			} else {
				m.appendMessage(agentMsgStyle.Render("✦ ") + "Copied last response to clipboard.")
			}
		} else {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "No response available to copy.")
		}
	case "/clear":
		m.messages = nil
		m.appendMessage(agentMsgStyle.Render("✦ ") + "Screen cleared.")
	case "/rewind":
		n := 1
		if len(parts) > 1 {
			_, _ = fmt.Sscanf(parts[1], "%d", &n)
		}
		if err := m.swarm.Rewind(n); err != nil {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to rewind: " + err.Error()))
		} else {
			for _, a := range m.agents {
				a.update("idle", "Idle")
			}
			// Wipe the local messages to reflect the rewound state (or just append a notice)
			m.appendMessage(agentMsgStyle.Render("✦ ") + fmt.Sprintf("Rewound the conversation history by %d turn(s).", n))
		}
	case "/plan":
		m.planMode = true
		m.appendMessage(agentMsgStyle.Render("✦ ") + "Plan Mode enabled. I will only read files and brainstorm. I will not modify files or execute shell commands.")
	case "/act":
		m.planMode = false
		m.appendMessage(agentMsgStyle.Render("✦ ") + "Act Mode enabled. I am fully capable of writing files and executing commands.")
	case "/observe":
		m.observeMode = !m.observeMode
		state := "disabled"
		if m.observeMode {
			state = "enabled"
		}
		m.appendMessage(agentMsgStyle.Render("✦ ") + fmt.Sprintf("Observe mode %s.", state))
		m.updateViewport()

	case "/config":
		cfg, err := sdk.LoadConfig()
		if err != nil {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to load config: " + err.Error()))
		} else {
			var lines []string
			lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Global Configuration"))
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("  - Active Model: %s", lipgloss.NewStyle().Foreground(primaryColor).Render(cfg.Model)))

			configPath, _ := sdk.DefaultConfigPath()
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("Configuration stored at: %s", lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(configPath)))

			icon := agentMsgStyle.Render("✦ ")
			m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, lines...)))
		}
	case "/context":
		if len(parts) == 1 {
			// List context
			files := m.swarm.ListContext()
			if len(files) == 0 {
				m.appendMessage(agentMsgStyle.Render("✦ ") + "No files are currently pinned to the context window.")
				return nil
			}

			var lines []string
			lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Pinned Context Files (%d)", len(files))))
			lines = append(lines, "")
			for _, file := range files {
				lines = append(lines, "  - "+lipgloss.NewStyle().Foreground(primaryColor).Render(file))
			}
			icon := agentMsgStyle.Render("✦ ")
			m.appendMessage(lipgloss.JoinHorizontal(lipgloss.Top, icon, lipgloss.JoinVertical(lipgloss.Left, lines...)))
		} else if parts[1] == "add" {
			if len(parts) < 3 {
				m.appendMessage(agentMsgStyle.Render("✦ ") + "Usage: /context add <file_path>")
				return nil
			}
			filePath := parts[2]
			if err := m.swarm.AddContext(filePath); err != nil {
				m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Error pinning file: " + err.Error()))
			} else {
				m.appendMessage(agentMsgStyle.Render("✦ ") + fmt.Sprintf("Pinned `%s` to the active context window.", filePath))
			}
		} else {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "Usage: /context OR /context add <file_path>")
		}
	case "/drop":
		if len(parts) < 2 {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "Usage: /drop <file_path> OR /drop all")
			return nil
		}
		filePath := parts[1]
		m.swarm.DropContext(filePath)
		if filePath == "all" {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "Cleared all pinned files from the context window.")
		} else {
			m.appendMessage(agentMsgStyle.Render("✦ ") + fmt.Sprintf("Dropped `%s` from the context window.", filePath))
		}
	case "/remember":
		if len(parts) < 2 {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "Usage: /remember <fact or preference>")
			return nil
		}
		fact := strings.Join(parts[1:], " ")
		if err := sdk.SaveMemory(fact); err != nil {
			m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Failed to save memory: " + err.Error()))
		} else {
			m.appendMessage(agentMsgStyle.Render("✦ ") + "Got it. I'll remember that for all future sessions.")
		}
	default:
		m.appendMessage(lipgloss.NewStyle().Foreground(errorColor).Render("Unknown command: " + cmd))
	}
	return nil
}

func (m *model) appendMessage(msg string) {
	m.messages = append(m.messages, msg)
	if len(m.messages) > 500 {
		m.messages = m.messages[len(m.messages)-500:]
	}
}

func (m *model) updateViewport() {
	if m.width == 0 {
		return
	}

	wasAtBottom := m.viewport.AtBottom() || m.viewport.YOffset() == 0

	// Prepare the dynamic message list
	var renderedMessages []string
	for _, msg := range m.messages {
		// Ensure each message is wrapped to the current viewport width.
		// This prevents terminal-level wrapping that breaks viewport scrolling calculations
		// and handles terminal resizing correctly.
		renderedMessages = append(renderedMessages, lipgloss.NewStyle().Width(m.viewport.Width()).Render(msg))
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

		status := "Thinking..."
		if m.globalSummary != "" {
			status = m.globalSummary
		}
		// The user requested '⣾' or a braille spinner. The default spinner in bubbles is dot/braille.
		// So we will just use m.spinner.View().
		renderedMessages = append(renderedMessages, m.spinner.View()+" "+lipgloss.NewStyle().Foreground(googleBlue).Italic(true).Render(status))
	}

	m.viewport.SetContent(strings.Join(renderedMessages, "\n\n"))

	if wasAtBottom {
		m.viewport.GotoBottom()
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
		m.textArea.SetPromptFunc(2, func(info textarea.PromptInfo) string {
			if info.LineNumber == 0 {
				return "⧖ "
			}
			return "  "
		})
	} else if m.state == stateShell {
		m.textArea.Placeholder = "Type your shell command"
		m.textArea.SetPromptFunc(2, func(info textarea.PromptInfo) string {
			if info.LineNumber == 0 {
				return "! "
			}
			return "  "
		})
	} else {
		m.textArea.Placeholder = "Type your message or /help (Alt+Enter or ^J for newline)"
		m.textArea.SetPromptFunc(2, func(info textarea.PromptInfo) string {
			if info.LineNumber == 0 {
				return "> "
			}
			return "  "
		})
	}
}

func buildBootMessage(cwd, branch string, modified bool, isDark bool, activeModel string, contextFiles []string, sessionID string, isResume bool) string {
	version := "Unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
		if version == "(devel)" {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					version = setting.Value
					if len(version) > 7 {
						version = version[:7]
					}
					break
				}
			}
		}
	}

	modStr := ""
	if modified {
		modStr = " (modified)"
	}

	headHash := "Unknown"
	if commits, err := sdk.GetRecentCommits(".", 1); err == nil && len(commits) > 0 {
		// Try to extract just the short hash and the first few words of the message
		msg := strings.ReplaceAll(commits[0], "\n", " ")
		if runewidth.StringWidth(msg) > 40 {
			msg = runewidth.Truncate(msg, 40, "…")
		}
		headHash = msg
	}

	sessionState := "New session (/sessions to resume)"
	if isResume {
		shortID := sessionID
		if strings.HasPrefix(shortID, "session_") && len(shortID) > 16 {
			shortID = shortID[8:16]
		}
		sessionState = "Resuming " + shortID
	}

	// Shorten home dir
	homeDir, _ := os.UserHomeDir()
	displayDir := cwd
	if homeDir != "" && strings.HasPrefix(cwd, homeDir) {
		displayDir = "~" + strings.TrimPrefix(cwd, homeDir)
	}

	contextStr := "None"
	if len(contextFiles) > 0 {
		var shortFiles []string
		for _, f := range contextFiles {
			// Try to show relative path if in cwd
			if rel, err := filepath.Rel(cwd, f); err == nil && !strings.HasPrefix(rel, "..") {
				shortFiles = append(shortFiles, rel)
			} else if homeDir != "" && strings.HasPrefix(f, homeDir) {
				shortFiles = append(shortFiles, "~"+strings.TrimPrefix(f, homeDir))
			} else {
				shortFiles = append(shortFiles, filepath.Base(f))
			}
		}
		contextStr = strings.Join(shortFiles, ", ")
	}

	// Styles
	t := defaultTheme(isDark)
	titleStyle := lipgloss.NewStyle().Foreground(googleBlue).Bold(true)
	versionStyle := lipgloss.NewStyle().Foreground(t.labelFg)
	headerStyle := lipgloss.NewStyle().Foreground(t.labelFg).Bold(true).MarginBottom(1)
	keyStyle := lipgloss.NewStyle().Foreground(t.labelFg).Width(9)
	valStyle := lipgloss.NewStyle().Foreground(t.statusFg)
	valModifiedStyle := lipgloss.NewStyle().Foreground(googleYellow)

	displayVersion := version
	if !strings.HasPrefix(displayVersion, "v") && displayVersion != "Unknown" {
		displayVersion = "v" + displayVersion
	}

	// Top row
	topRow := lipgloss.JoinHorizontal(lipgloss.Bottom,
		titleStyle.Render("🤖 Swarm CLI "),
		versionStyle.Render(displayVersion),
	)

	// Environment Column
	envBranchVal := valStyle.Render(branch)
	if modified {
		envBranchVal += valModifiedStyle.Render(modStr)
	}

	envCol := lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Render("[ Environment ]"),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("Dir:"), valStyle.Render(displayDir)),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("Branch:"), envBranchVal),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("HEAD:"), valStyle.Render(headHash)),
	)

	// Session Column
	sessCol := lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Render("[ Session ]"),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("State:"), valStyle.Render(sessionState)),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("Model:"), valStyle.Render(activeModel)),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("Context:"), valStyle.Render(contextStr)),
	)

	// Combine columns with spacing
	dashboard := lipgloss.JoinHorizontal(lipgloss.Top, envCol, lipgloss.NewStyle().Width(4).Render(""), sessCol)

	// Tips footer
	tipsLabel := lipgloss.NewStyle().Foreground(googleYellow).Bold(true).Render("💡 Tips: ")
	tipsContent := lipgloss.NewStyle().Foreground(t.labelFg).Render(
		"[/help] commands   [/skills] view skills   [/debug] toggle debug   [!] shell mode",
	)
	footer := lipgloss.JoinHorizontal(lipgloss.Top, tipsLabel, tipsContent)

	// Assemble final block
	return lipgloss.JoinVertical(lipgloss.Left,
		topRow,
		"",
		dashboard,
		"",
		footer,
	)
}

func (m model) renderAgentPanel() string {
	t := m.theme()
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.borderColor).
		Padding(0, 1).
		MarginBottom(1).
		Width(m.width - 2)

	if len(m.spans) == 0 {
		var watermark string
		rawLogo := []string{
			"   `Yb.      .dP\"Y8 Yb        dP  db    88\"\"Yb 8b    d8",
			"     `Yb.    `Ybo.\"  Yb  db  dP  dPYb   88__dP 88b  d88",
			"     .dP'    o.`Y8b   YbdPYbdP  dP__Yb  88\"Yb  88YbdP88",
			"   .dP'      8bodP'    YP  YP  dP\"\"\"\"Yb 88  Yb 88 YY 88",
		}

		if !m.hasRunTasks && m.logoFrame < 60 {
			// Animated "paint" from left to right on boot

			var sb strings.Builder
			for _, line := range rawLogo {
				for x, ch := range line {
					if string(ch) == " " {
						sb.WriteRune(ch)
						continue
					}

					c := t.logoMutedFg
					if x <= m.logoFrame {
						if x < 12 {
							c = t.logoCaretFg
						} else {
							cIdx := ((x - 12) * len(t.logoForestFgs)) / 43 // 55 - 12 = 43
							if cIdx < 0 {
								cIdx = 0
							}
							if cIdx >= len(t.logoForestFgs) {
								cIdx = len(t.logoForestFgs) - 1
							}
							c = t.logoForestFgs[cIdx]
						}
					}

					style := lipgloss.NewStyle().Foreground(c).Bold(true)
					sb.WriteString(style.Render(string(ch)))
				}
				sb.WriteString("\n")
			}
			watermark = strings.TrimRight(sb.String(), "\n")
		} else {
			// Static, fully painted logo after animation, until tasks run
			// Or muted grey if tasks have run. The user requested:
			// "The logo should start out as grey (the same grey it will be after tasks have run), but the first time it launches the logo will be "painted"...
			// If m.hasRunTasks is true, we display the muted grey.
			// If not, we display the fully painted logo.
			if !m.hasRunTasks {
				var sb strings.Builder
				for _, line := range rawLogo {
					for x, ch := range line {
						if string(ch) == " " {
							sb.WriteRune(ch)
							continue
						}
						var c color.Color
						if x < 12 {
							c = t.logoCaretFg
						} else {
							cIdx := ((x - 12) * len(t.logoForestFgs)) / 43
							if cIdx < 0 {
								cIdx = 0
							}
							if cIdx >= len(t.logoForestFgs) {
								cIdx = len(t.logoForestFgs) - 1
							}
							c = t.logoForestFgs[cIdx]
						}
						style := lipgloss.NewStyle().Foreground(c).Bold(true)
						sb.WriteString(style.Render(string(ch)))
					}
					sb.WriteString("\n")
				}
				watermark = strings.TrimRight(sb.String(), "\n")
			} else {
				watermarkStyle := lipgloss.NewStyle().Foreground(t.logoMutedFg).Bold(true)
				watermark = watermarkStyle.Render(strings.Join(rawLogo, "\n"))
			}
		}

		// panelStyle.Height(6) results in exactly 7 total layout lines (1 Top + 4 Inner + 1 Bottom + 1 Margin)
		// This perfectly matches the height of 1 row of active cards, ensuring zero jump.
		return panelStyle.Height(6).Align(lipgloss.Center).Render(lipgloss.JoinVertical(lipgloss.Center, watermark))
	}
	// Build tree
	type treeNode struct {
		span     *uiSpan
		children []*treeNode
		depth    int
	}

	nodes := make(map[string]*treeNode)
	var roots []*treeNode

	for _, s := range m.spans {
		nodes[s.ID] = &treeNode{span: s}
	}

	for _, s := range m.spans {
		node := nodes[s.ID]
		if s.ParentID != "" && nodes[s.ParentID] != nil {
			parent := nodes[s.ParentID]
			parent.children = append(parent.children, node)
		} else {
			roots = append(roots, node)
		}
	}

	// Sort to stabilize UI rendering
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].span.ID < roots[j].span.ID
	})

	var linearSpans []*treeNode
	var traverse func(node *treeNode, depth int)
	traverse = func(node *treeNode, depth int) {
		node.depth = depth
		linearSpans = append(linearSpans, node)
		sort.Slice(node.children, func(i, j int) bool {
			return node.children[i].span.ID < node.children[j].span.ID
		})
		for _, child := range node.children {
			traverse(child, depth+1)
		}
	}

	for _, root := range roots {
		traverse(root, 0)
	}

	// Calculate optimal columns based on count and width
	cols := 4
	if m.width < 100 {
		cols = 2
	} else if m.width < 140 {
		cols = 3
	}

	fidelity := "high"
	if len(linearSpans) > cols*2 {
		fidelity = "medium"
	}
	if len(linearSpans) > 16 {
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

	renderLine := func(prefix string, text string, style lipgloss.Style, width int) string {
		text = strings.ReplaceAll(text, "\n", " ")
		text = strings.ReplaceAll(text, "\r", "")
		prefixComp := lipgloss.NewStyle().Width(3).Render(prefix)
		contentWidth := width - 3
		if contentWidth < 1 {
			contentWidth = 1
		}
		if runewidth.StringWidth(text) > contentWidth {
			text = runewidth.Truncate(text, contentWidth-1, "…")
		}
		contentComp := style.Render(lipgloss.PlaceHorizontal(contentWidth, lipgloss.Left, text))
		return lipgloss.JoinHorizontal(lipgloss.Left, prefixComp, contentComp)
	}

	var cards []string
	for _, node := range linearSpans {
		s := node.span

		color := colorIdle
		border := lipgloss.NormalBorder()
		iconStr := "⚪"
		statusLabel := "Pending"

		isActive := s.Status == "spawning" || s.Status == "thinking" || s.Status == "executing"
		if isActive {
			color = colorActive
			border = lipgloss.ThickBorder()
			iconStr = m.spinner.View()
			statusLabel = "Active"
		} else if s.Status == "complete" {
			color = colorSuccess
			iconStr = "⚒" // 1-cell glyph
			statusLabel = "Complete"
		} else if s.Status == "error" {
			color = colorError
			iconStr = "✖"
			statusLabel = "Failed"
		} else if s.Status == "waiting" {
			color = colorWaiting
			iconStr = "⌚"
			statusLabel = "Waiting"
		}

		agentIcon := "🤖"
		if a := m.findAgent(s.Agent); a != nil {
			agentIcon = a.icon
		}

		cardStyle := lipgloss.NewStyle().
			Border(border).
			BorderForeground(color).
			BorderBottom(false).
			Padding(0, 1).
			MarginRight(2).
			Width(cardWidth - 2)

		contentWidth := cardWidth - 6
		if contentWidth < 1 {
			contentWidth = 1
		}

		// Depth indentation for title
		indent := strings.Repeat(" ", node.depth)
		if node.depth > 0 {
			indent += "↳ "
		}
		// Let's ensure the indent doesn't consume all the width
		if runewidth.StringWidth(indent) > contentWidth/2 {
			indent = strings.Repeat(" ", contentWidth/2)
		}

		titleText := indent + s.Name
		line1 := renderLine(agentIcon, titleText, lipgloss.NewStyle().Foreground(color).Bold(true), contentWidth)

		statusText := s.Thought
		if s.ToolName != "" {
			statusText = "Tool: " + s.ToolName
		}
		if statusText == "" {
			statusText = "..."
		}
		line2 := renderLine(iconStr, statusText, lipgloss.NewStyle().Foreground(tipColor), contentWidth)

		var cardContent string
		if fidelity == "high" || fidelity == "medium" {
			cardContent = lipgloss.JoinVertical(lipgloss.Left, line1, line2)
			cardStyle = cardStyle.Height(2)
		} else {
			cardStyle = lipgloss.NewStyle().Border(border).BorderForeground(color).Width(6).Height(1).Padding(0, 1)
			cardContent = lipgloss.NewStyle().Width(4).Align(lipgloss.Center).Render(agentIcon)
		}

		renderedCard := cardStyle.Render(cardContent)

		if fidelity == "high" || fidelity == "medium" {
			label := " " + statusLabel + " "
			labelLen := runewidth.StringWidth(label)
			visibleWidth := cardWidth - 2
			remaining := visibleWidth - 2 - labelLen
			rightDashCount := 2
			leftDashCount := remaining - rightDashCount
			if leftDashCount < 1 {
				leftDashCount = 1
				rightDashCount = remaining - leftDashCount
				if rightDashCount < 0 {
					rightDashCount = 0
				}
			}

			bottomLine := lipgloss.NewStyle().Foreground(color).MarginRight(2).Render(
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

	return panelStyle.Render(grid)
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

func (m model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if m.width == 0 {
		v.SetContent("Loading…")
		return v
	}

	t := m.theme()

	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.borderColor).
		Padding(0, 1)

	viewportStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.borderColor).
		Padding(0, 1)

	statusBarStyle := lipgloss.NewStyle().
		Foreground(t.statusFg).
		Background(t.statusBg).
		Height(1)

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

	// Autocomplete Box
	acView := ""
	if m.acActive && len(m.acMatches) > 0 {
		var lines []string
		for i, match := range m.acMatches {
			if i == m.acIndex {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(primaryColor).Render(" "+match+" "))
			} else {
				lines = append(lines, " "+match+" ")
			}
		}
		if m.acHasMore {
			lines = append(lines, " ... ")
		}
		acView = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(primaryColor).Padding(0, 1).Render(strings.Join(lines, "\n"))
	}
	acHeight := lipgloss.Height(acView)

	// Recalculate viewport height to fill remaining space
	// Subtract 2 for the viewportStyle's own top/bottom borders
	m.viewport.SetHeight(m.height - agentPanelHeight - inputHeight - acHeight - statusHeight - 2)
	if m.viewport.Height() < 1 {
		m.viewport.SetHeight(1)
	}

	// Output Box (Viewport) with border
	contentWidth := m.width - 6
	vpWidth := contentWidth
	scrollable := m.viewport.TotalLineCount() > m.viewport.Height()
	if scrollable {
		vpWidth = contentWidth - 2
	}
	m.viewport.SetWidth(vpWidth)

	lines := strings.Split(m.viewport.View(), "\n")

	if scrollable {
		scrollbar := renderScrollbar(m.viewport.Height(), m.viewport.ScrollPercent(), t.borderColor)
		var newLines []string
		for i, line := range lines {
			padded := lipgloss.PlaceHorizontal(vpWidth, lipgloss.Left, line)
			if i < len(scrollbar) {
				newLines = append(newLines, padded+" "+scrollbar[i])
			} else {
				newLines = append(newLines, padded)
			}
		}
		lines = newLines
	} else {
		for i, line := range lines {
			lines[i] = lipgloss.PlaceHorizontal(contentWidth, lipgloss.Left, line)
		}
	}

	vpContent := strings.Join(lines, "\n")
	vpView := viewportStyle.Render(vpContent)

	// Main body is just the vertical stack of the sections
	var mainBody string
	if acView != "" {
		mainBody = lipgloss.JoinVertical(lipgloss.Left, agentPanelView, vpView, acView, inputView)
	} else {
		mainBody = lipgloss.JoinVertical(lipgloss.Left, agentPanelView, vpView, inputView)
	}

	// Bottom Status Line (no border, full width)
	w1, w2 := m.width/3, m.width/3
	w3 := m.width - w1 - w2

	cwdText := " " + m.cwd
	if m.gitBranch != "" {
		mod := ""
		if m.gitModified {
			mod = "*"
		}
		cwdText += fmt.Sprintf(" (%s%s)", m.gitBranch, mod)
	}
	p1 := statusBarStyle.Width(w1).Align(lipgloss.Left).Render(cwdText)
	p2 := statusBarStyle.Width(w2).Align(lipgloss.Center).Render("swarm mode")
	p3 := statusBarStyle.Width(w3).Align(lipgloss.Right).Render(m.activeModel + " ")
	statusView := lipgloss.JoinHorizontal(lipgloss.Top, p1, p2, p3)

	v.SetContent(lipgloss.JoinVertical(lipgloss.Left, mainBody, statusView))
	return v
}

func (m *model) cleanup() {
	if m.webServer != nil {
		_ = m.webServer.Stop(context.Background())
	}
	for _, pgid := range m.bgPGIDs {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}

func launchInteractiveShell(planMode bool, resume bool) error {
	m, err := initialModel(planMode, resume)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		m.cleanup()
		return fmt.Errorf("error: %w", err)
	}
	m.cleanup()
	return nil
}
