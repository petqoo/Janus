package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Message Types ---
type StatusMsg string
type AegisLogMsg string
type AppLogMsg string
type ErrorMsg struct{ Err error }

type introDoneMsg struct{}

// --- Model ---

type Model struct {
	Sub   chan any
	ready bool
	introDone bool

	// FIX: Re-added width and height to the model
	width  int
	height int

	appLogsViewport   viewport.Model
	aegisLogsViewport viewport.Model
	spinner           spinner.Model
	status            string
}

func InitialModel(sub chan any) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		Sub:       sub,
		spinner:   s,
		introDone: false,
	}
}

// --- Commands ---

func waitForActivity(sub chan any) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

func endIntroScreen() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return introDoneMsg{}
	})
}

// --- Bubble Tea Methods ---

func (m Model) Init() tea.Cmd {
	return endIntroScreen()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		// Store the window size
		m.width = msg.Width
		m.height = msg.Height

		// Calculate sizes for all components
		bannerHeight := 9
		headerHeight := 1
		footerHeight := 1
		statusHeight := 1
		verticalMargin := bannerHeight + (headerHeight*2) + footerHeight + statusHeight

		if !m.ready {
			m.appLogsViewport = viewport.New(msg.Width-4, (msg.Height-verticalMargin)/2)
			m.aegisLogsViewport = viewport.New(msg.Width-4, (msg.Height-verticalMargin)/2)
			m.ready = true
		} else {
			m.appLogsViewport.Width = msg.Width - 4
			m.aegisLogsViewport.Width = msg.Width - 4
			m.appLogsViewport.Height = (msg.Height - verticalMargin) / 2
			m.aegisLogsViewport.Height = (msg.Height - verticalMargin) / 2
		}
	}

	if !m.introDone {
		if _, ok := msg.(introDoneMsg); ok {
			m.introDone = true
			m.status = "Waiting for file changes..."
			return m, tea.Batch(m.spinner.Tick, waitForActivity(m.Sub))
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "up", "k":
			m.appLogsViewport.LineUp(1)
		case "down", "j":
			m.appLogsViewport.LineDown(1)
		}

	case StatusMsg:
		m.status = string(msg)
	case AegisLogMsg:
		m.aegisLogsViewport.SetContent(m.aegisLogsViewport.View() + "\n" + string(msg))
		m.aegisLogsViewport.GotoBottom()
	case AppLogMsg:
		m.appLogsViewport.SetContent(m.appLogsViewport.View() + "\n" + string(msg))
		m.appLogsViewport.GotoBottom()
	case ErrorMsg:
		m.status = "Error!"
		errorText := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(msg.Err.Error())
		m.aegisLogsViewport.SetContent(m.aegisLogsViewport.View() + "\n" + errorText)
		m.aegisLogsViewport.GotoBottom()

	default:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	cmds = append(cmds, waitForActivity(m.Sub))
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// --- Splash Screen View ---
	if !m.introDone {
		// FIX: Corrected the ASCII art banner
		janusArt := `
***********************************************
*** ***
*** *** * *** * * ***** ***
*** * * * * * * * * * ***
*** * ***** *** * * * *** ***
*** * * * * * * * * * ***
*** *** * * * * * * ***** ***
*** ***
***********************************************
`
		bannerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		banner := bannerStyle.Render(janusArt)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, banner)
	}

	// --- Main Dashboard View ---
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true).Padding(0, 1)
	footerStyle := lipgloss.NewStyle().MarginTop(1).Foreground(lipgloss.Color("240"))
	statusStyle := lipgloss.NewStyle().Padding(0, 1)
	panelStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238"))

	appLogsHeader := headerStyle.Render("APPLICATION LOGS")
	aegisLogsHeader := headerStyle.Render("AEGIS EVENTS")
	status := statusStyle.Render(fmt.Sprintf("%s %s", m.spinner.View(), m.status))
	footer := footerStyle.Render("[↑/↓] scroll [q]uit")

	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		status,
		appLogsHeader,
		panelStyle.Render(m.appLogsViewport.View()),
		aegisLogsHeader,
		panelStyle.Render(m.aegisLogsViewport.View()),
		footer,
	)

	docStyle := lipgloss.NewStyle().Margin(1, 2)
	return docStyle.Render(mainContent)
}