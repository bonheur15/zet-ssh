package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"zet-ssh/internal/core/profiles"
	"zet-ssh/internal/tui/components"
	"zet-ssh/internal/tui/pages/dashboard"
	"zet-ssh/internal/tui/pages/session"
	"zet-ssh/internal/tui/theme"

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
	state         sessionState
	dashboard     tea.Model
	sessions      []tea.Model
	sessionMeta   []profiles.Profile
	activeSession int
	palette       components.Palette
	vaultUnlock   components.VaultUnlock
	profilesStore *profiles.Store
	vaultPassword string
	width         int
	height        int
}

func NewAppModel() AppModel {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "zet-ssh")
	store, _ := profiles.NewStore(configDir)
	_ = theme.Load(configDir)

	if len(store.List()) == 0 {
		_ = store.Add(profiles.Profile{ID: "1", Name: "Local Machine", Host: "localhost", Port: 22, User: "user", AuthType: profiles.AuthAgent})
		_ = store.Add(profiles.Profile{ID: "2", Name: "Staging Server", Host: "10.0.0.50", Port: 22, User: "deploy", AuthType: profiles.AuthAgent})
	}

	return AppModel{
		state:         viewDashboard,
		dashboard:     dashboard.New(store),
		palette:       components.NewPalette(),
		vaultUnlock:   components.NewVaultUnlock(),
		profilesStore: store,
		activeSession: -1,
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
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+k":
			if !m.vaultUnlock.IsActive() {
				m.palette.Toggle()
			}
			return m, nil
		case "ctrl+c":
			if m.state != viewSession {
				m.closeAllSessions()
				return m, tea.Quit
			}
		case "esc", "q":
			if m.state == viewSession && !m.palette.IsActive() && !m.vaultUnlock.IsActive() {
				m.state = viewDashboard
				return m, nil
			}
			if m.state == viewDashboard && !m.palette.IsActive() && !m.vaultUnlock.IsActive() {
				if len(m.sessions) > 0 {
					m.state = viewSession
					return m, nil
				}
				m.closeAllSessions()
				return m, tea.Quit
			}
		case "ctrl+n":
			if m.state == viewSession {
				m.state = viewDashboard
				return m, nil
			}
		case "ctrl+w":
			if m.state == viewSession {
				m.closeActiveSession()
				if len(m.sessions) == 0 {
					m.state = viewDashboard
				}
				return m, nil
			}
		case "[":
			if m.state == viewSession {
				m.switchSession(-1)
				return m, nil
			}
		case "]":
			if m.state == viewSession {
				m.switchSession(1)
				return m, nil
			}
		}

		if m.state == viewSession {
			for i := 1; i <= 9; i++ {
				if msg.String() == "alt+"+strconv.Itoa(i) {
					idx := i - 1
					if idx < len(m.sessions) {
						m.activeSession = idx
						m.resizeActiveSession()
					}
					return m, nil
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == viewDashboard {
			dash, c := m.dashboard.Update(msg)
			m.dashboard = dash
			cmds = append(cmds, c)
		}
		if m.state == viewSession {
			m.resizeActiveSession()
		}

	case profiles.Profile:
		m.state = viewSession
		sess := session.New(msg, m.width, m.height)
		m.sessions = append(m.sessions, sess)
		m.sessionMeta = append(m.sessionMeta, msg)
		m.activeSession = len(m.sessions) - 1
		m.resizeActiveSession()
		return m, m.sessions[m.activeSession].Init()
	}

	if m.state == viewDashboard {
		newDash, newCmd := m.dashboard.Update(msg)
		m.dashboard = newDash
		cmds = append(cmds, newCmd)
	} else if m.state == viewSession {
		if m.activeSession >= 0 && m.activeSession < len(m.sessions) {
			newSess, newCmd := m.sessions[m.activeSession].Update(msg)
			m.sessions[m.activeSession] = newSess
			cmds = append(cmds, newCmd)
		}
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
		if m.activeSession >= 0 && m.activeSession < len(m.sessions) {
			view = m.renderSessionTabs() + "\n" + m.sessions[m.activeSession].View()
		} else {
			view = "No active sessions"
		}
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

func (m *AppModel) switchSession(delta int) {
	if len(m.sessions) == 0 {
		return
	}
	m.activeSession = (m.activeSession + delta + len(m.sessions)) % len(m.sessions)
	m.resizeActiveSession()
}

func (m *AppModel) closeActiveSession() {
	if m.activeSession < 0 || m.activeSession >= len(m.sessions) {
		return
	}

	if closer, ok := m.sessions[m.activeSession].(interface{ Close() }); ok {
		closer.Close()
	}

	m.sessions = append(m.sessions[:m.activeSession], m.sessions[m.activeSession+1:]...)
	m.sessionMeta = append(m.sessionMeta[:m.activeSession], m.sessionMeta[m.activeSession+1:]...)

	if len(m.sessions) == 0 {
		m.activeSession = -1
		return
	}

	if m.activeSession >= len(m.sessions) {
		m.activeSession = len(m.sessions) - 1
	}
	m.resizeActiveSession()
}

func (m *AppModel) closeAllSessions() {
	for _, sess := range m.sessions {
		if closer, ok := sess.(interface{ Close() }); ok {
			closer.Close()
		}
	}
}

func (m *AppModel) resizeActiveSession() {
	if m.activeSession < 0 || m.activeSession >= len(m.sessions) {
		return
	}
	updated, _ := m.sessions[m.activeSession].Update(tea.WindowSizeMsg{Width: m.width, Height: m.height - 2})
	m.sessions[m.activeSession] = updated
}

func (m AppModel) renderSessionTabs() string {
	if len(m.sessions) == 0 {
		return ""
	}

	var tabs []string
	for i, meta := range m.sessionMeta {
		label := strconv.Itoa(i+1) + ":" + meta.Name
		style := lipgloss.NewStyle().Padding(0, 1).Foreground(theme.Inactive)
		if i == m.activeSession {
			style = lipgloss.NewStyle().Padding(0, 1).Foreground(theme.OnPrimary).Background(theme.Primary).Bold(true)
		}
		tabs = append(tabs, style.Render(label))
	}

	hints := lipgloss.NewStyle().Foreground(theme.Inactive).Render(" [ ] switch  [Alt+1..9] jump  [Ctrl+N] new  [Ctrl+W] close")
	return lipgloss.JoinHorizontal(lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Left, tabs...), hints)
}
