package main

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	colorIdle    = lipgloss.Color("#666666") // Gray
	colorActive  = lipgloss.Color("#4169E1") // Royal Blue
	colorSuccess = lipgloss.Color("#34A853") // Green
	colorWaiting = lipgloss.Color("#FBBC05") // Yellow
	colorError   = lipgloss.Color("#EA4335") // Red
)

type demoAgent struct {
	name   string
	icon   string
	status string
	state  string // "idle", "active", "success", "waiting", "error"
	spin   spinner.Model
}

type demoSwarmModel struct {
	vp       viewport.Model
	agents   []*demoAgent
	messages []string
	width    int
	height   int
	ticks    int
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func initDemoSwarm() demoSwarmModel {
	vp := viewport.New(0, 0)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorActive)

	agents := []*demoAgent{
		{name: "Router", icon: "🧠", status: "Awaiting input…", state: "waiting", spin: s},
		{name: "Investigator", icon: "🔍", status: "Idle", state: "idle", spin: s},
		{name: "Web Researcher", icon: "🌐", status: "Idle", state: "idle", spin: s},
		{name: "GitOps", icon: "🐙", status: "Idle", state: "idle", spin: s},
		{name: "Test Synthesizer", icon: "🧪", status: "Idle", state: "idle", spin: s},
		{name: "Security Auditor", icon: "🔐", status: "Idle", state: "idle", spin: s},
		{name: "DB Architect", icon: "💾", status: "Idle", state: "idle", spin: s},
		{name: "Code Generator", icon: "💻", status: "Idle", state: "idle", spin: s},
	}

	msgs := []string{
		promptStyle.Render("> ") + "Refactor the legacy authentication service to use OAuth 2.0, generate the test cases, and create a Pull Request with the changes.",
	}

	return demoSwarmModel{
		vp:       vp,
		agents:   agents,
		messages: msgs,
	}
}

func (m demoSwarmModel) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	for _, a := range m.agents {
		cmds = append(cmds, a.spin.Tick)
	}
	return tea.Batch(cmds...)
}

func (m demoSwarmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.vp.Width = msg.Width - 4
		dashboardHeight := lipgloss.Height(m.renderDashboard())
		m.vp.Height = msg.Height - dashboardHeight - 4 // Account for status bar and vpBox borders/padding
		m.updateVP()

	case spinner.TickMsg:
		for i, a := range m.agents {
			if a.state == "active" {
				var cmd tea.Cmd
				m.agents[i].spin, cmd = a.spin.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tickMsg:
		m.ticks++
		cmds = append(cmds, tickCmd())

		// Script the demo
		switch m.ticks {
		case 2:
			m.agents[0].state = "active"
			m.agents[0].status = "Decomposing task…"
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [Router] ")+"Decomposing the task into sub-objectives: 1. Audit current codebase 2. Research OAuth patterns 3. Create Git Branch.")
		case 4:
			m.agents[0].status = "Provisioning Swarm…"
		case 6:
			m.agents[0].state = "waiting"
			m.agents[0].status = "Delegated to Investigator"
			m.agents[1].state = "active"
			m.agents[1].status = "Running grep_search ('auth.go')"
		case 8:
			m.agents[1].status = "Reading src/legacy/auth.go"
		case 10:
			m.agents[1].state = "success"
			m.agents[1].status = "Completed"
			m.agents[0].state = "active"
			m.agents[0].status = "Synthesizing investigation"
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [Investigator] ")+"Found 3 files related to legacy auth in `src/legacy/`. Preparing refactor map.")
		case 12:
			m.agents[0].state = "waiting"
			m.agents[0].status = "Delegated to Web Researcher"
			m.agents[2].state = "active"
			m.agents[2].status = "Running google_search"
		case 15:
			m.agents[2].status = "Fetching RFC 6749"
		case 18:
			m.agents[2].state = "success"
			m.agents[2].status = "Completed"
			m.agents[0].state = "active"
			m.agents[0].status = "Delegating to Dev Swarm"
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [Web Researcher] ")+"Compiled modern OAuth 2.0 PKCE flow standards for Go implementation.")
		case 20:
			m.agents[0].state = "waiting"
			m.agents[0].status = "Waiting on Dev Swarm"
			m.agents[6].state = "active"
			m.agents[6].status = "Designing schema migrations…"
		case 22:
			m.agents[6].state = "success"
			m.agents[6].status = "Completed"
			m.agents[7].state = "active"
			m.agents[7].status = "Translating logic to Go…"
		case 24:
			m.agents[7].status = "Refactoring user routes…"
		case 26:
			m.agents[7].state = "success"
			m.agents[7].status = "Completed"
			m.agents[4].state = "active"
			m.agents[5].state = "active"
			m.agents[4].status = "Generating table tests…"
			m.agents[5].status = "Auditing Go code for vulnerabilities…"
		case 28:
			m.agents[4].status = "Executing tests: PASS (14/14)"
			m.agents[5].status = "No vulnerabilities found."
		case 31:
			m.agents[4].state = "success"
			m.agents[4].status = "Completed"
			m.agents[5].state = "success"
			m.agents[5].status = "Completed"
			m.agents[3].state = "active"
			m.agents[3].status = "Running bash ('git commit')"
		case 34:
			m.agents[3].status = "Running bash ('gh pr create')"
		case 37:
			m.agents[3].state = "success"
			m.agents[3].status = "Completed"
			m.agents[0].state = "active"
			m.agents[0].status = "Finalizing"
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [GitOps] ")+"Created Pull Request #142 for your review.")
		case 39:
			m.agents[0].state = "waiting"
			m.agents[0].status = "Awaiting input…"
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [Router] ")+"The swarm has successfully completed the OAuth 2.0 refactoring task. All tests pass, and PR #142 is ready. What would you like to do next?")
		}

		// Dynamically adjust viewport height in case the dashboard grew
		dashboardHeight := lipgloss.Height(m.renderDashboard())
		m.vp.Height = m.height - dashboardHeight - 4

		m.updateVP()
	}

	return m, tea.Batch(cmds...)
}

func (m *demoSwarmModel) updateVP() {
	var s strings.Builder
	wrapStyle := lipgloss.NewStyle().Width(m.vp.Width)
	for _, msg := range m.messages {
		s.WriteString(wrapStyle.Render(msg))
		s.WriteString("\n\n")
	}
	m.vp.SetContent(s.String())
	m.vp.GotoBottom()
}

func (m demoSwarmModel) renderDashboard() string {
	var row1 []string
	var row2 []string
	for i, a := range m.agents {
		border := lipgloss.NormalBorder()
		color := colorIdle

		switch a.state {
		case "active":
			border = lipgloss.ThickBorder()
			color = colorActive
		case "success":
			color = colorSuccess
		case "waiting":
			color = colorWaiting
		case "error":
			color = colorError
		}

		cardWidth := (m.width - 4) / 4

		style := lipgloss.NewStyle().
			Border(border).
			BorderForeground(color).
			Padding(0, 1).
			Width(cardWidth - 2). // Lipgloss width might be content width in this version, so subtract borders
			Height(2)             // Content height 2 lines: Name, status-wrap

		iconStr := "  "
		if a.state == "active" {
			iconStr = a.spin.View() + " "
		} else if a.state == "success" {
			iconStr = "✓ "
		} else if a.state == "error" {
			iconStr = "✗ "
		} else if a.state == "waiting" {
			iconStr = "⧖ "
		}

		// Use Lipgloss for text wrapping instead of manual truncation
		statusText := lipgloss.NewStyle().
			Width(cardWidth - 8).
			MaxHeight(1).
			Render(a.status)

		card := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(color).Bold(true).Render(a.icon+" "+a.name),
			iconStr+lipgloss.NewStyle().Foreground(tipColor).Render(statusText),
		)

		renderedCard := style.Render(card)
		if i < 4 {
			row1 = append(row1, renderedCard)
		} else {
			row2 = append(row2, renderedCard)
		}
	}

	dashboardRow1 := lipgloss.JoinHorizontal(lipgloss.Top, row1...)
	dashboardRow2 := lipgloss.JoinHorizontal(lipgloss.Top, row2...)
	dashboardGrid := lipgloss.JoinVertical(lipgloss.Left, dashboardRow1, dashboardRow2)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(" Swarm Dashboard - Mission Control"),
			dashboardGrid,
		))
}

func (m demoSwarmModel) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	// 1. Render Dashboard
	var row1 []string
	var row2 []string
	for i, a := range m.agents {
		border := lipgloss.NormalBorder()
		color := colorIdle

		switch a.state {
		case "active":
			border = lipgloss.ThickBorder()
			color = colorActive
		case "success":
			color = colorSuccess
		case "waiting":
			color = colorWaiting
		case "error":
			color = colorError
		}

		style := lipgloss.NewStyle().
			Border(border).
			BorderForeground(color).
			Padding(0, 1).
			Width((m.width - 12) / 4). // 4 columns per row, account for borders
			Height(3)                  // Ensure consistent height

		iconStr := "  "
		if a.state == "active" {
			iconStr = a.spin.View() + " "
		} else if a.state == "success" {
			iconStr = "✓ "
		} else if a.state == "error" {
			iconStr = "✗ "
		} else if a.state == "waiting" {
			iconStr = "⧖ "
		}

		// Truncate status if it's too long for the box
		statusText := a.status
		maxLen := ((m.width - 12) / 4) - 6
		if len(statusText) > maxLen && maxLen > 0 {
			statusText = statusText[:maxLen] + "…"
		}

		card := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(color).Bold(true).Render(a.icon+" "+a.name),
			iconStr+lipgloss.NewStyle().Foreground(tipColor).Render(statusText),
		)

		renderedCard := style.Render(card)
		if i < 4 {
			row1 = append(row1, renderedCard)
		} else {
			row2 = append(row2, renderedCard)
		}
	}

	dashboardRow1 := lipgloss.JoinHorizontal(lipgloss.Top, row1...)
	dashboardRow2 := lipgloss.JoinHorizontal(lipgloss.Top, row2...)
	dashboardGrid := lipgloss.JoinVertical(lipgloss.Left, dashboardRow1, dashboardRow2)

	dashboardBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(" Swarm Dashboard - Mission Control"),
			dashboardGrid,
		))

	// 2. Render Viewport
	vpBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false).
		BorderForeground(borderColor).
		Padding(1, 2).
		Height(m.height - lipgloss.Height(dashboardBox) - 1).
		Render(m.vp.View())

	// 3. Status Bar
	status := statusBarStyle.Width(m.width).Render(" agents (main*)        [Demo Mode] ")

	mainBody := lipgloss.JoinVertical(lipgloss.Left, dashboardBox, vpBox)
	// Force the main body to be exactly m.height - 1 lines to prevent terminal scrolling
	mainBody = lipgloss.NewStyle().Width(m.width).Height(m.height - 1).Render(mainBody)

	return lipgloss.JoinVertical(lipgloss.Left,
		mainBody,
		status,
	)
}

func launchDemoSwarm() error {
	p := tea.NewProgram(initDemoSwarm(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
