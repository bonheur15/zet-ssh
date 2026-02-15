package session

import (
	"fmt"
	"zet-ssh/internal/core/profiles"
	"zet-ssh/internal/core/ssh"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	profile    profiles.Profile
	viewport   viewport.Model
	sshSession *ssh.Session
	err        error
	width      int
	height     int
	output     chan string
}

func New(p profiles.Profile, width, height int) Model {
	vp := viewport.New(width, height-2)
	vp.SetContent("Connecting...")
	
	return Model{
		profile:  p,
		viewport: vp,
		width:    width,
		height:   height,
		output:   make(chan string),
	}
}

type sshOutputMsg string
type sshErrorMsg error

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		// This is a placeholder for actual connection logic
		// In a real app, we'd handle auth methods here
		return sshErrorMsg(fmt.Errorf("SSH interaction in TUI requires complex PTY handling (WIP)"))
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 2
	case sshOutputMsg:
		m.viewport.SetContent(m.viewport.View() + string(msg))
		m.viewport.GotoBottom()
	case sshErrorMsg:
		m.err = msg
		m.viewport.SetContent(fmt.Sprintf("Error: %v", m.err))
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	header := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1).
		Render(fmt.Sprintf("Session: %s (%s)", m.profile.Name, m.profile.Host))

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.viewport.View(),
		"Press 'q' to return to dashboard",
	)
}
