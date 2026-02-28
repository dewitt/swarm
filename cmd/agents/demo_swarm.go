package main

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type demoAgent struct {
	name   string
	color  lipgloss.Color
	status string
	spin   spinner.Model
	active bool
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

	s1 := spinner.New()
	s1.Spinner = spinner.Dot
	s1.Style = lipgloss.NewStyle().Foreground(googleBlue)

	s2 := spinner.New()
	s2.Spinner = spinner.MiniDot
	s2.Style = lipgloss.NewStyle().Foreground(googleGreen)

	s3 := spinner.New()
	s3.Spinner = spinner.Pulse
	s3.Style = lipgloss.NewStyle().Foreground(googleYellow)

	s4 := spinner.New()
	s4.Spinner = spinner.Jump
	s4.Style = lipgloss.NewStyle().Foreground(googleRed)

	agents := []*demoAgent{
		{name: "Router Agent", color: googleBlue, status: "Awaiting input...", spin: s1, active: true},
		{name: "Codebase Investigator", color: googleGreen, status: "Idle", spin: s2, active: false},
		{name: "Web Researcher", color: googleYellow, status: "Idle", spin: s3, active: false},
		{name: "GitOps Practitioner", color: googleRed, status: "Idle", spin: s4, active: false},
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
		m.vp.Height = msg.Height - 12 // Leave room for dashboard
		m.updateVP()

	case spinner.TickMsg:
		for i, a := range m.agents {
			if a.spin.ID() == msg.ID {
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
			m.agents[0].status = "Decomposing task..."
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [Router] ")+"Decomposing the task into sub-objectives: 1. Audit current codebase 2. Research OAuth patterns 3. Create Git Branch.")
		case 4:
			m.agents[0].status = "Provisioning Swarm..."
		case 6:
			m.agents[0].status = "Delegating to Investigator"
			m.agents[0].active = false
			m.agents[1].active = true
			m.agents[1].status = "Running grep_search ('auth.go')"
		case 8:
			m.agents[1].status = "Reading src/legacy/auth.go"
		case 10:
			m.agents[1].active = false
			m.agents[1].status = "Completed"
			m.agents[0].active = true
			m.agents[0].status = "Synthesizing investigation"
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [Investigator] ")+"Found 3 files related to legacy auth in `src/legacy/`. Preparing refactor map.")
		case 12:
			m.agents[0].status = "Delegating to Web Researcher"
			m.agents[0].active = false
			m.agents[2].active = true
			m.agents[2].status = "Running google_search ('OAuth 2.0 Go standards')"
		case 15:
			m.agents[2].status = "Fetching https://datatracker.ietf.org/doc/html/rfc6749"
		case 18:
			m.agents[2].status = "Completed"
			m.agents[2].active = false
			m.agents[0].active = true
			m.agents[0].status = "Delegating to GitOps Practitioner"
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [Web Researcher] ")+"Compiled modern OAuth 2.0 PKCE flow standards for Go implementation.")
		case 20:
			m.agents[0].active = false
			m.agents[3].active = true
			m.agents[3].status = "Running bash_execute ('git checkout -b refactor/oauth')"
		case 22:
			m.agents[3].status = "Executing code changes locally..."
		case 25:
			m.agents[3].status = "Running bash_execute ('go test ./...')"
		case 28:
			m.agents[3].status = "Running bash_execute ('git commit -m \"feat: oauth2\"')"
		case 31:
			m.agents[3].status = "Running bash_execute ('gh pr create')"
		case 34:
			m.agents[3].status = "Completed"
			m.agents[3].active = false
			m.agents[0].active = true
			m.agents[0].status = "Finalizing"
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [GitOps] ")+"Created Pull Request #142 for your review.")
		case 36:
			m.agents[0].status = "Awaiting input..."
			m.messages = append(m.messages, agentMsgStyle.Render("✦ [Router] ")+"The swarm has successfully completed the OAuth 2.0 refactoring task. All tests pass, and PR #142 is ready. What would you like to do next?")
		}
		m.updateVP()
	}

	return m, tea.Batch(cmds...)
}

func (m *demoSwarmModel) updateVP() {
	var s strings.Builder
	for _, msg := range m.messages {
		s.WriteString(msg)
		s.WriteString("\n\n")
	}
	m.vp.SetContent(s.String())
	m.vp.GotoBottom()
}

func (m demoSwarmModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// 1. Render Dashboard
	var cards []string
	for _, a := range m.agents {
		border := lipgloss.NormalBorder()
		color := tipColor
		if a.active {
			border = lipgloss.ThickBorder()
			color = a.color
		}

		style := lipgloss.NewStyle().
			Border(border).
			BorderForeground(color).
			Padding(0, 1).
			Width((m.width - 10) / 4) // Roughly 4 columns

		icon := " "
		if a.active {
			icon = a.spin.View()
		}

		card := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(a.color).Bold(true).Render(a.name),
			icon+" "+a.status,
		)
		cards = append(cards, style.Render(card))
	}

	dashboard := lipgloss.JoinHorizontal(lipgloss.Top, cards...)
	dashboardBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(" Swarm Dashboard - Mission Control"),
			dashboard,
		))

	// 2. Render Viewport
	vpBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(m.width - 4).
		Height(m.height - lipgloss.Height(dashboardBox) - 3).
		Render(m.vp.View())

	// 3. Status Bar
	status := statusBarStyle.Width(m.width).Render(" agents (main*)        [Demo Mode] ")

	return lipgloss.JoinVertical(lipgloss.Left,
		dashboardBox,
		vpBox,
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
