package tui

import (
	"zet-ssh/internal/tui/pages/dashboard"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sessionState int

const (
	viewDashboard sessionState = iota
	viewSession
	viewSettings
)

type AppModel struct {
	state     sessionState
	dashboard tea.Model
	width     int
	height    int
}

func NewAppModel() AppModel {
	return AppModel{
		state:     viewDashboard,
		dashboard: dashboard.New(),
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.dashboard.Init())
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			// Basic tab switching logic placeholder
			if m.state == viewDashboard {
				m.state = viewSettings
			} else {
				m.state = viewDashboard
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate resize to children
		dash, cmd := m.dashboard.Update(msg)
		m.dashboard = dash
		cmds = append(cmds, cmd)
	}

	// Update the active view
	switch m.state {
	case viewDashboard:
		newDash, newCmd := m.dashboard.Update(msg)
		m.dashboard = newDash
		cmd = newCmd
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	// Simple layout
	if m.width == 0 {
		return "Initializing..."
	}

	switch m.state {
	case viewDashboard:
		return m.dashboard.View()
	case viewSettings:
		return lipgloss.NewStyle().Padding(2).Render("Settings View (Placeholder)")
	default:
		return "Unknown View"
	}
}
