package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type VaultUnlock struct {
	password textinput.Model
	active   bool
	err      error
}

func NewVaultUnlock() VaultUnlock {
	ti := textinput.New()
	ti.Placeholder = "Master Password"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.Focus()

	return VaultUnlock{
		password: ti,
		active:   false,
	}
}

type VaultUnlockedMsg struct {
	Password string
}

func (m VaultUnlock) Update(msg tea.Msg) (VaultUnlock, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			pass := m.password.Value()
			m.password.SetValue("")
			m.active = false
			return m, func() tea.Msg {
				return VaultUnlockedMsg{Password: pass}
			}
		case "esc":
			m.active = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.password, cmd = m.password.Update(msg)
	return m, cmd
}

func (m VaultUnlock) View() string {
	if !m.active {
		return ""
	}

	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1).
		Width(40)

	content := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("VAULT LOCKED"),
		"Please enter master password to unlock",
		"",
		m.password.View(),
	)

	return style.Render(content)
}

func (m *VaultUnlock) Open() {
	m.active = true
	m.password.Focus()
}

func (m VaultUnlock) IsActive() bool {
	return m.active
}
