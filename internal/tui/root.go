package tui

import (
	"os"
	"path/filepath"
	"zet-ssh/internal/core/profiles"
	"zet-ssh/internal/tui/components"
	"zet-ssh/internal/tui/pages/dashboard"
	"zet-ssh/internal/tui/pages/session"

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
	state          sessionState
	dashboard      tea.Model
	session        tea.Model
	palette        components.Palette
	vaultUnlock    components.VaultUnlock
	profilesStore  *profiles.Store
	vaultPassword  string
	width          int
	height         int
}

func NewAppModel() AppModel {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "zet-ssh")
	store, _ := profiles.NewStore(configDir)

	if len(store.List()) == 0 {
		store.Add(profiles.Profile{ID: "1", Name: "Local Machine", Host: "localhost", Port: 22, User: "user"})
		store.Add(profiles.Profile{ID: "2", Name: "Staging Server", Host: "10.0.0.50", Port: 22, User: "deploy"})
	}

	return AppModel{
		state:         viewDashboard,
		dashboard:     dashboard.New(store),
		palette:       components.NewPalette(),
		vaultUnlock:   components.NewVaultUnlock(),
		profilesStore: store,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.dashboard.Init())
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.vaultUnlock.IsActive() {
		m.vaultUnlock, cmd = m.vaultUnlock.Update(msg)
		return m, cmd
	}

	if m.palette.IsActive() {
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case components.VaultUnlockedMsg:
		m.vaultPassword = msg.Password
		// Here we could try to load the vault to verify password
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+v":
			m.vaultUnlock.Open()
			return m, nil
		case "ctrl+k":
			m.palette.Toggle()
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state == viewSession {
				m.state = viewDashboard
				return m, nil
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate resize
		if m.state == viewDashboard {
			dash, cmd := m.dashboard.Update(msg)
			m.dashboard = dash
			cmds = append(cmds, cmd)
		} else if m.state == viewSession {
			sess, cmd := m.session.Update(msg)
			m.session = sess
			cmds = append(cmds, cmd)
		}
	case profiles.Profile:
		// Profile selected from dashboard
		m.state = viewSession
		m.session = session.New(msg, m.width, m.height)
		return m, m.session.Init()
	}

	// Update active view
	if m.state == viewDashboard {
		newDash, newCmd := m.dashboard.Update(msg)
		m.dashboard = newDash
		cmds = append(cmds, newCmd)
	} else if m.state == viewSession {
		newSess, newCmd := m.session.Update(msg)
		m.session = newSess
		cmds = append(cmds, newCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	var view string
	switch m.state {
	case viewDashboard:
		view = m.dashboard.View()
	case viewSession:
		view = m.session.View()
	case viewSettings:
		view = "Settings"
	default:
		view = "Unknown State"
	}

	if m.palette.IsActive() {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.palette.View())
	}

	if m.vaultUnlock.IsActive() {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.vaultUnlock.View())
	}

	return view
}
