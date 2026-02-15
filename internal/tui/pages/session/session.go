package session

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"zet-ssh/internal/core/profiles"
	coreSFTP "zet-ssh/internal/core/sftp"
	"zet-ssh/internal/core/ssh"
	"zet-ssh/internal/tui/components"
	"zet-ssh/internal/tui/theme"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	sshlib "golang.org/x/crypto/ssh"
)

type sshOutputMsg string
type sshErrorMsg error

type sessionReadyMsg struct {
	session    *ssh.Session
	sftpClient *coreSFTP.Client
	remotePath string
	status     string
}

type transferResultMsg struct {
	status string
	err    error
}

type Model struct {
	profile        profiles.Profile
	viewport       viewport.Model
	sshSession     *ssh.Session
	sftpClient     *coreSFTP.Client
	err            error
	width          int
	height         int
	terminalBuffer string
	status         string

	fileMode    bool
	activePane  int // 0 local, 1 remote
	remoteReady bool

	localBrowser  components.FileBrowser
	remoteBrowser components.FileBrowser
}

func New(p profiles.Profile, width, height int) Model {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	vp := viewport.New(width, max(1, height-3))
	vp.SetContent("Establishing connection...")

	cwd, err := os.Getwd()
	if err != nil || strings.TrimSpace(cwd) == "" {
		cwd = "."
	}

	paneW := max(20, width/2-1)
	paneH := max(5, height-4)
	local := components.NewFileBrowser(components.LocalSource{}, cwd, paneW, paneH)
	remote := components.NewFileBrowser(components.LocalSource{}, "/", paneW, paneH)
	local.Active = true

	return Model{
		profile:       p,
		viewport:      vp,
		width:         width,
		height:        height,
		status:        "Connecting...",
		localBrowser:  local,
		remoteBrowser: remote,
	}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		auth, warnings := buildAuthMethods(m.profile)
		if len(auth) == 0 {
			return sshErrorMsg(fmt.Errorf("no authentication methods available (try SSH agent, key path, or set ZET_SSH_PASSWORD)"))
		}

		s, err := ssh.Connect(m.profile.User, m.profile.Host, m.profile.Port, auth)
		if err != nil {
			return sshErrorMsg(err)
		}

		if err := s.StartShell(m.width, m.height-3); err != nil {
			s.Close()
			return sshErrorMsg(err)
		}

		sftpClient, err := coreSFTP.NewClient(s.Client())
		if err != nil {
			s.Close()
			return sshErrorMsg(err)
		}

		remotePath := "/"
		if pwd, pwdErr := sftpClient.Pwd(); pwdErr == nil && strings.TrimSpace(pwd) != "" {
			remotePath = pwd
		}

		status := "Connected"
		if len(warnings) > 0 {
			status = status + " (" + strings.Join(warnings, "; ") + ")"
		}

		return sessionReadyMsg{
			session:    s,
			sftpClient: sftpClient,
			remotePath: remotePath,
			status:     status,
		}
	}
}

func waitForOutput(s *ssh.Session) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 2048)
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
	case sessionReadyMsg:
		m.sshSession = msg.session
		m.sftpClient = msg.sftpClient
		m.remoteBrowser = components.NewFileBrowser(m.sftpClient, msg.remotePath, max(20, m.width/2-1), max(5, m.height-4))
		m.remoteReady = true
		m.status = msg.status
		m.viewport.SetContent("Connected.\n")
		m.resizePanes()
		return m, waitForOutput(m.sshSession)

	case sshOutputMsg:
		m.terminalBuffer += string(msg)
		m.viewport.SetContent(m.terminalBuffer)
		m.viewport.GotoBottom()
		return m, waitForOutput(m.sshSession)

	case sshErrorMsg:
		m.err = msg
		m.status = fmt.Sprintf("Connection error: %v", m.err)
		m.viewport.SetContent("\n" + m.status)

	case transferResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Transfer failed: %v", msg.err)
			return m, nil
		}
		m.status = msg.status
		m.localBrowser.Refresh()
		if m.remoteReady {
			m.remoteBrowser.Refresh()
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = max(1, msg.Height-3)
		m.resizePanes()
		if m.sshSession != nil {
			_ = m.sshSession.Resize(m.width, m.height-3)
		}

	case tea.KeyMsg:
		if m.fileMode {
			return m.handleFileModeKey(msg)
		}

		switch msg.String() {
		case "ctrl+c":
			if m.sshSession != nil {
				_, _ = m.sshSession.Write([]byte{3})
			}
			return m, nil
		case "ctrl+f":
			if m.remoteReady {
				m.fileMode = true
				m.activePane = 0
				m.localBrowser.Active = true
				m.remoteBrowser.Active = false
				m.status = "File mode: Tab switches pane, Enter opens directory, c copies"
			}
			return m, nil
		}

		if m.sshSession != nil {
			if bytes := keyToBytes(msg); len(bytes) > 0 {
				_, _ = m.sshSession.Write(bytes)
			}
		}
	}

	if !m.fileMode {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	header := theme.Title.Render(fmt.Sprintf("SSH: %s@%s", m.profile.User, m.profile.Host))

	main := m.viewport.View()
	if m.fileMode {
		main = lipgloss.JoinHorizontal(lipgloss.Top,
			m.localBrowser.View(),
			m.remoteBrowser.View(),
		)
	}

	hints := "[Esc/q] Dashboard  [Ctrl+C] Send SIGINT  [Ctrl+F] Toggle File Mode"
	if m.fileMode {
		hints = "[Tab] Switch pane  [Enter] Open dir  [Backspace] Up  [c] Copy file  [r] Refresh  [Ctrl+F] Back"
	}
	footer := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(hints),
		theme.Header.Render(m.status),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		main,
		footer,
	)
}

func (m Model) Close() {
	if m.sftpClient != nil {
		_ = m.sftpClient.Close()
	}
	if m.sshSession != nil {
		m.sshSession.Close()
	}
}

func (m *Model) resizePanes() {
	paneW := max(20, m.width/2-1)
	paneH := max(5, m.height-4)
	m.localBrowser.SetSize(paneW, paneH)
	m.remoteBrowser.SetSize(paneW, paneH)
}

func (m Model) handleFileModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+f":
		m.fileMode = false
		m.status = "Terminal mode"
		return m, nil
	case "tab", "left", "right":
		if m.activePane == 0 {
			m.activePane = 1
		} else {
			m.activePane = 0
		}
		m.localBrowser.Active = m.activePane == 0
		m.remoteBrowser.Active = m.activePane == 1
		return m, nil
	case "enter", "l":
		var err error
		if m.activePane == 0 {
			err = m.localBrowser.EnterSelected()
		} else {
			err = m.remoteBrowser.EnterSelected()
		}
		if err != nil {
			m.status = fmt.Sprintf("Browse error: %v", err)
		}
		return m, nil
	case "backspace", "h":
		var err error
		if m.activePane == 0 {
			err = m.localBrowser.UpDir()
		} else {
			err = m.remoteBrowser.UpDir()
		}
		if err != nil {
			m.status = fmt.Sprintf("Browse error: %v", err)
		}
		return m, nil
	case "c":
		return m, m.transferSelected()
	case "r":
		m.localBrowser.Refresh()
		m.remoteBrowser.Refresh()
		m.status = "Refreshed"
		return m, nil
	}

	var cmd tea.Cmd
	if m.activePane == 0 {
		m.localBrowser, cmd = m.localBrowser.Update(msg)
	} else {
		m.remoteBrowser, cmd = m.remoteBrowser.Update(msg)
	}
	return m, cmd
}

func (m Model) transferSelected() tea.Cmd {
	if !m.remoteReady || m.sftpClient == nil {
		return func() tea.Msg {
			return transferResultMsg{err: fmt.Errorf("remote SFTP is not ready")}
		}
	}

	if m.activePane == 0 {
		item, ok := m.localBrowser.SelectedItem()
		if !ok || item.IsParent {
			return func() tea.Msg { return transferResultMsg{err: fmt.Errorf("select a local file first")} }
		}
		if item.Info.IsDir() {
			return func() tea.Msg { return transferResultMsg{err: fmt.Errorf("directory copy is not implemented yet")} }
		}

		localPath, _ := m.localBrowser.SelectedPath()
		remotePath := path.Join(m.remoteBrowser.CurrentPath(), item.Info.Name())

		return func() tea.Msg {
			err := m.sftpClient.Upload(localPath, remotePath)
			if err != nil {
				return transferResultMsg{err: err}
			}
			return transferResultMsg{status: fmt.Sprintf("Uploaded %s -> %s", filepath.Base(localPath), remotePath)}
		}
	}

	item, ok := m.remoteBrowser.SelectedItem()
	if !ok || item.IsParent {
		return func() tea.Msg { return transferResultMsg{err: fmt.Errorf("select a remote file first")} }
	}
	if item.Info.IsDir() {
		return func() tea.Msg { return transferResultMsg{err: fmt.Errorf("directory copy is not implemented yet")} }
	}

	remotePath, _ := m.remoteBrowser.SelectedPath()
	localPath := filepath.Join(m.localBrowser.CurrentPath(), item.Info.Name())

	return func() tea.Msg {
		err := m.sftpClient.Download(remotePath, localPath)
		if err != nil {
			return transferResultMsg{err: err}
		}
		return transferResultMsg{status: fmt.Sprintf("Downloaded %s -> %s", path.Base(remotePath), localPath)}
	}
}

func buildAuthMethods(profile profiles.Profile) ([]sshlib.AuthMethod, []string) {
	var methods []sshlib.AuthMethod
	var warnings []string

	password := os.Getenv("ZET_SSH_PASSWORD")
	keyPassphrase := os.Getenv("ZET_SSH_KEY_PASSPHRASE")

	addPassword := func() {
		if strings.TrimSpace(password) == "" {
			warnings = append(warnings, "password missing (set ZET_SSH_PASSWORD)")
			return
		}
		methods = append(methods, ssh.PasswordAuth(password))
	}

	addAgent := func() {
		agentAuth, err := ssh.AgentAuth()
		if err != nil {
			warnings = append(warnings, "ssh-agent unavailable")
			return
		}
		methods = append(methods, agentAuth)
	}

	addProfileKey := func() {
		if strings.TrimSpace(profile.KeyPath) == "" {
			warnings = append(warnings, "profile key path not set")
			return
		}
		auth, err := ssh.PublicKeyAuthFromFile(profile.KeyPath, []byte(keyPassphrase))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("key failed: %s", profile.KeyPath))
			return
		}
		methods = append(methods, auth)
	}

	addDefaultKeys := func() {
		for _, keyPath := range ssh.DefaultPrivateKeyPaths() {
			auth, err := ssh.PublicKeyAuthFromFile(keyPath, []byte(keyPassphrase))
			if err == nil {
				methods = append(methods, auth)
			}
		}
	}

	switch profile.AuthType {
	case profiles.AuthPassword:
		addPassword()
		addAgent()
		addDefaultKeys()
	case profiles.AuthKey:
		addProfileKey()
		addDefaultKeys()
		addAgent()
		if strings.TrimSpace(password) != "" {
			methods = append(methods, ssh.PasswordAuth(password))
		}
	case profiles.AuthAgent:
		addAgent()
		addDefaultKeys()
		if strings.TrimSpace(password) != "" {
			methods = append(methods, ssh.PasswordAuth(password))
		}
	default:
		if strings.TrimSpace(profile.KeyPath) != "" {
			addProfileKey()
		}
		addDefaultKeys()
		addAgent()
		if strings.TrimSpace(password) != "" {
			methods = append(methods, ssh.PasswordAuth(password))
		}
	}

	if len(methods) == 0 {
		warnings = append(warnings, "no auth methods built")
	}

	return methods, warnings
}

func keyToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyEnter:
		return []byte{13}
	case tea.KeyTab:
		return []byte{9}
	case tea.KeyBackspace:
		return []byte{127}
	case tea.KeySpace:
		return []byte(" ")
	case tea.KeyEsc:
		return []byte{27}
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	case tea.KeyCtrlD:
		return []byte{4}
	case tea.KeyCtrlZ:
		return []byte{26}
	}

	if len(msg.Runes) > 0 {
		return []byte(string(msg.Runes))
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
