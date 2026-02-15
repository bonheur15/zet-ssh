package session

import (
	"fmt"
	"io"
	"zet-ssh/internal/core/profiles"
	"zet-ssh/internal/core/ssh"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	sshlib "golang.org/x/crypto/ssh"
)

type sshOutputMsg string
type sshErrorMsg error

type Model struct {
	profile    profiles.Profile
	viewport   viewport.Model
	sshSession *ssh.Session
	err        error
	width      int
	height     int
	terminalBuffer string
}

func New(p profiles.Profile, width, height int) Model {
	vp := viewport.New(width, height-3)
	vp.SetContent("Establishing connection...")
	
	return Model{
		profile:  p,
		viewport: vp,
		width:    width,
		height:   height,
	}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		// Defaulting to agent auth for now, in a real app we'd check profile.AuthType
		// and use the vault password to decrypt keys if necessary.
		auth := []sshlib.AuthMethod{}
		
		// Note: This is a placeholder for actual auth logic. 
		// For now, it will likely fail if no agent is running.
		s, err := ssh.Connect(m.profile.User, m.profile.Host, m.profile.Port, auth)
		if err != nil {
			return sshErrorMsg(err)
		}

		err = s.StartShell(m.width, m.height-3)
		if err != nil {
			return sshErrorMsg(err)
		}

		return s
	}
}

func waitForOutput(s *ssh.Session) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 1024)
		n, err := s.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return sshErrorMsg(err)
		}
		return sshOutputMsg(string(buf[:n]))
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case *ssh.Session:
		m.sshSession = msg
		m.viewport.SetContent("Connected.\n")
		return m, waitForOutput(m.sshSession)

	case sshOutputMsg:
		m.terminalBuffer += string(msg)
		m.viewport.SetContent(m.terminalBuffer)
		m.viewport.GotoBottom()
		return m, waitForOutput(m.sshSession)

	case sshErrorMsg:
		m.err = msg
		m.viewport.SetContent(fmt.Sprintf("\nConnection Error: %v", m.err))

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3
		if m.sshSession != nil {
			m.sshSession.Resize(m.width, m.height-3)
		}

	case tea.KeyMsg:
		if m.sshSession != nil {
			switch msg.String() {
			case "ctrl+c":
				// Forward ctrl+c to SSH instead of quitting the whole app
				m.sshSession.Write([]byte{3})
				return m, nil
			case "enter":
				m.sshSession.Write([]byte{13})
			default:
				m.sshSession.Write([]byte(msg.String()))
			}
		}
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
		Render(fmt.Sprintf("SSH: %s@%s", m.profile.User, m.profile.Host))

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(" [Esc/q] Dashboard  [Ctrl+C] Send SIGINT")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.viewport.View(),
		footer,
	)
}
