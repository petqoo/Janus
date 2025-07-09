package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)


type StatusMsg string
type AegisLogMsg string
type AppLogMsg string
type ErrorMsg struct{ Err error }


type Model struct {
	width     int
	height    int
	Sub       chan any      
	spinner   spinner.Model
	status    string
	appLogs   []string
	aegisLogs []string
}

func InitialModel(sub chan any) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return Model{
		Sub:       sub,
		spinner:   s,
		status:    "Initializing...",
		aegisLogs: []string{"Aegis is waiting for file changes..."},
	}
}


func waitForActivity(sub chan any) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForActivity(m.Sub))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case StatusMsg:
		m.status = string(msg)
	case AegisLogMsg:
		m.aegisLogs = append(m.aegisLogs, string(msg))
	case AppLogMsg:
		m.appLogs = append(m.appLogs, string(msg))
	case ErrorMsg:
		m.status = "Error!"
		m.aegisLogs = append(m.aegisLogs, msg.Err.Error())

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, waitForActivity(m.Sub)
}

func (m Model) View() string {
	docStyle := lipgloss.NewStyle().Margin(1, 2)
	status := fmt.Sprintf("%s %s", m.spinner.View(), m.status)
	aegisLogs := "Aegis Events:\n" + strings.Join(m.aegisLogs, "\n")
	mainContent := lipgloss.JoinVertical(lipgloss.Left, status, aegisLogs)
	return docStyle.Render(mainContent)
}