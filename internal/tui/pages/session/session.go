package session

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"zet-ssh/internal/core/profiles"
	coreSFTP "zet-ssh/internal/core/sftp"
	"zet-ssh/internal/core/ssh"
	"zet-ssh/internal/tui/components"
	"zet-ssh/internal/tui/theme"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	sshlib "golang.org/x/crypto/ssh"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

type sshOutputMsg string
type sshErrorMsg error

type sessionReadyMsg struct {
	session    *ssh.Session
	sftpClient *coreSFTP.Client
	remotePath string
	status     string
}

type transferUpdateMsg struct {
	label   string
	copied  int64
	total   int64
	done    bool
	err     error
	aborted bool
}

type filePreviewMsg struct {
	title   string
	content string
	err     error
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

	transferInProgress bool
	transferLabel      string
	transferCopied     int64
	transferTotal      int64
	transferProgress   progress.Model
	transferUpdates    chan tea.Msg
	transferCancel     chan struct{}

	previewOpen    bool
	previewTitle   string
	previewContent string

	passwordPromptOpen bool
	passwordInput      textinput.Model
	runtimePassword    string
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
	paneH := max(5, height-6)
	local := components.NewFileBrowser(components.LocalSource{}, cwd, paneW, paneH)
	remote := components.NewFileBrowser(components.LocalSource{}, "/", paneW, paneH)
	local.Active = true

	pb := progress.New(progress.WithScaledGradient("62", "86"))
	pb.Width = 24

	passInput := textinput.New()
	passInput.Placeholder = "SSH Password"
	passInput.EchoMode = textinput.EchoPassword
	passInput.EchoCharacter = '*'

	return Model{
		profile:          p,
		viewport:         vp,
		width:            width,
		height:           height,
		status:           "Connecting...",
		localBrowser:     local,
		remoteBrowser:    remote,
		transferProgress: pb,
		passwordInput:    passInput,
	}
}

func (m Model) Init() tea.Cmd {
	return m.connectCmd("")
}

func (m Model) connectCmd(password string) tea.Cmd {
	return func() tea.Msg {
		auth, warnings := buildAuthMethods(m.profile, password)
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
		buf := make([]byte, 4096)
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

func waitForTransferUpdate(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case sessionReadyMsg:
		m.passwordPromptOpen = false
		m.passwordInput.SetValue("")
		m.runtimePassword = ""
		m.sshSession = msg.session
		m.sftpClient = msg.sftpClient
		m.remoteBrowser = components.NewFileBrowser(m.sftpClient, msg.remotePath, max(20, m.width/2-1), max(5, m.height-6))
		m.remoteReady = true
		m.status = msg.status
		m.viewport.SetContent("Connected.\n")
		m.resizePanes()
		return m, waitForOutput(m.sshSession)

	case sshOutputMsg:
		m.appendTerminalOutput(string(msg))
		return m, waitForOutput(m.sshSession)

	case sshErrorMsg:
		m.err = msg
		m.status = fmt.Sprintf("Connection error: %v", m.err)
		m.viewport.SetContent("\n" + m.status)
		if shouldPromptPassword(m.err) && !m.passwordPromptOpen {
			m.passwordPromptOpen = true
			m.passwordInput.SetValue("")
			m.passwordInput.Focus()
			m.status = "Authentication failed. Enter password and press Enter to retry"
		}

	case transferUpdateMsg:
		m.transferLabel = msg.label
		m.transferCopied = msg.copied
		m.transferTotal = msg.total
		if msg.total > 0 {
			pct := float64(msg.copied) / float64(msg.total)
			if pct < 0 {
				pct = 0
			}
			if pct > 1 {
				pct = 1
			}
			m.transferProgress.SetPercent(pct)
		}

		if msg.done {
			m.transferInProgress = false
			if msg.aborted {
				m.status = "Transfer cancelled"
			} else if msg.err != nil {
				m.status = fmt.Sprintf("Transfer failed: %v", msg.err)
			} else {
				m.status = "Transfer complete"
				m.localBrowser.Refresh()
				if m.remoteReady {
					m.remoteBrowser.Refresh()
				}
			}
			m.transferUpdates = nil
			m.transferCancel = nil
			return m, nil
		}

		if m.transferUpdates != nil {
			return m, waitForTransferUpdate(m.transferUpdates)
		}

	case filePreviewMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Open failed: %v", msg.err)
			return m, nil
		}
		m.previewTitle = msg.title
		m.previewContent = msg.content
		m.previewOpen = true
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = max(1, msg.Height-4)
		m.resizePanes()
		if m.sshSession != nil {
			_ = m.sshSession.Resize(m.width, m.height-4)
		}

	case tea.KeyMsg:
		if m.passwordPromptOpen {
			switch msg.String() {
			case "enter":
				pass := m.passwordInput.Value()
				m.passwordInput.SetValue("")
				m.passwordPromptOpen = false
				m.runtimePassword = pass
				m.status = "Retrying with password..."
				return m, m.connectCmd(pass)
			case "esc":
				m.passwordPromptOpen = false
				m.status = fmt.Sprintf("Connection error: %v", m.err)
				return m, nil
			}
			m.passwordInput, cmd = m.passwordInput.Update(msg)
			return m, cmd
		}

		if m.previewOpen {
			switch msg.String() {
			case "esc", "o", "q":
				m.previewOpen = false
			}
			return m, nil
		}

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
				m.status = "File mode: Tab switches pane, Enter opens directory, c copies, o opens file"
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
		left := m.localBrowser.View()
		right := m.remoteBrowser.View()
		main = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
		if m.transferInProgress {
			main = lipgloss.JoinVertical(lipgloss.Left, main, m.transferView())
		}
	}

	if m.previewOpen {
		preview := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Accent).
			Padding(1).
			Width(max(40, m.width-6)).
			Height(max(10, m.height-8)).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				theme.Header.Render(m.previewTitle),
				"",
				m.previewContent,
				"",
				lipgloss.NewStyle().Foreground(theme.Inactive).Render("[Esc/o] Close"),
			))
		main = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, preview)
	}
	if m.passwordPromptOpen {
		prompt := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Warning).
			Padding(1).
			Width(max(40, m.width/2)).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				theme.Header.Render("SSH Authentication Required"),
				"Enter password for "+m.profile.User+"@"+m.profile.Host,
				"",
				m.passwordInput.View(),
				"",
				lipgloss.NewStyle().Foreground(theme.Inactive).Render("[Enter] Retry  [Esc] Cancel"),
			))
		main = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, prompt)
	}

	hints := "[Esc/q] Dashboard  [Ctrl+C] Send SIGINT  [Ctrl+F] Toggle File Mode"
	if m.fileMode {
		hints = "[Tab] Switch pane  [Enter] Open dir  [Backspace] Up  [c] Copy  [o] Open  [x] Cancel transfer  [r] Refresh  [Ctrl+F] Back"
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
	if m.transferCancel != nil {
		close(m.transferCancel)
	}
	if m.sftpClient != nil {
		_ = m.sftpClient.Close()
	}
	if m.sshSession != nil {
		m.sshSession.Close()
	}
}

func (m *Model) resizePanes() {
	paneW := max(20, m.width/2-1)
	paneH := max(5, m.height-8)
	m.localBrowser.SetSize(paneW, paneH)
	m.remoteBrowser.SetSize(paneW, paneH)
}

func (m Model) handleFileModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.transferInProgress {
		switch msg.String() {
		case "x", "ctrl+c":
			if m.transferCancel != nil {
				close(m.transferCancel)
				m.transferCancel = nil
				m.status = "Cancelling transfer..."
			}
			return m, nil
		}
	}

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
		if m.transferInProgress {
			m.status = "A transfer is already running"
			return m, nil
		}
		return m, m.startTransfer()
	case "o":
		return m, m.openSelectedFile()
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

func (m *Model) startTransfer() tea.Cmd {
	if !m.remoteReady || m.sftpClient == nil {
		return func() tea.Msg {
			return transferUpdateMsg{done: true, err: fmt.Errorf("remote SFTP is not ready")}
		}
	}

	var label string
	var run func(ch chan tea.Msg, cancel <-chan struct{})

	if m.activePane == 0 {
		item, ok := m.localBrowser.SelectedItem()
		if !ok || item.IsParent {
			return func() tea.Msg { return transferUpdateMsg{done: true, err: fmt.Errorf("select a local file first")} }
		}
		if item.Info.IsDir() {
			return func() tea.Msg {
				return transferUpdateMsg{done: true, err: fmt.Errorf("directory copy is not implemented yet")}
			}
		}

		localPath, _ := m.localBrowser.SelectedPath()
		remotePath := path.Join(m.remoteBrowser.CurrentPath(), item.Info.Name())
		label = fmt.Sprintf("Uploading %s", filepath.Base(localPath))

		run = func(ch chan tea.Msg, cancel <-chan struct{}) {
			err := m.sftpClient.UploadWithProgress(localPath, remotePath, func(copied, total int64) {
				ch <- transferUpdateMsg{label: label, copied: copied, total: total}
			}, cancel)

			final := transferUpdateMsg{label: label, done: true, copied: m.transferCopied, total: m.transferTotal}
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "cancel") {
					final.aborted = true
				} else {
					final.err = err
				}
			}
			ch <- final
			close(ch)
		}
	} else {
		item, ok := m.remoteBrowser.SelectedItem()
		if !ok || item.IsParent {
			return func() tea.Msg { return transferUpdateMsg{done: true, err: fmt.Errorf("select a remote file first")} }
		}
		if item.Info.IsDir() {
			return func() tea.Msg {
				return transferUpdateMsg{done: true, err: fmt.Errorf("directory copy is not implemented yet")}
			}
		}

		remotePath, _ := m.remoteBrowser.SelectedPath()
		localPath := filepath.Join(m.localBrowser.CurrentPath(), item.Info.Name())
		label = fmt.Sprintf("Downloading %s", path.Base(remotePath))

		run = func(ch chan tea.Msg, cancel <-chan struct{}) {
			err := m.sftpClient.DownloadWithProgress(remotePath, localPath, func(copied, total int64) {
				ch <- transferUpdateMsg{label: label, copied: copied, total: total}
			}, cancel)

			final := transferUpdateMsg{label: label, done: true, copied: m.transferCopied, total: m.transferTotal}
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "cancel") {
					final.aborted = true
				} else {
					final.err = err
				}
			}
			ch <- final
			close(ch)
		}
	}

	updates := make(chan tea.Msg, 64)
	cancel := make(chan struct{})
	m.transferUpdates = updates
	m.transferCancel = cancel
	m.transferInProgress = true
	m.transferCopied = 0
	m.transferTotal = 0
	m.transferLabel = label
	m.status = label

	go run(updates, cancel)
	return waitForTransferUpdate(updates)
}

func (m Model) openSelectedFile() tea.Cmd {
	const maxPreview = 128 * 1024

	if m.activePane == 0 {
		item, ok := m.localBrowser.SelectedItem()
		if !ok || item.IsParent {
			return func() tea.Msg { return filePreviewMsg{err: fmt.Errorf("select a file first")} }
		}
		if item.Info.IsDir() {
			return func() tea.Msg { return filePreviewMsg{err: fmt.Errorf("cannot open directory")} }
		}
		localPath, _ := m.localBrowser.SelectedPath()
		return func() tea.Msg {
			data, err := os.ReadFile(localPath)
			if err != nil {
				return filePreviewMsg{err: err}
			}
			if len(data) > maxPreview {
				data = data[:maxPreview]
			}
			return filePreviewMsg{title: localPath, content: sanitizeTextPreview(string(data))}
		}
	}

	item, ok := m.remoteBrowser.SelectedItem()
	if !ok || item.IsParent {
		return func() tea.Msg { return filePreviewMsg{err: fmt.Errorf("select a remote file first")} }
	}
	if item.Info.IsDir() {
		return func() tea.Msg { return filePreviewMsg{err: fmt.Errorf("cannot open directory")} }
	}
	remotePath, _ := m.remoteBrowser.SelectedPath()
	if m.sftpClient == nil {
		return func() tea.Msg { return filePreviewMsg{err: fmt.Errorf("remote SFTP is not ready")} }
	}

	return func() tea.Msg {
		f, err := m.sftpClient.OpenRead(remotePath)
		if err != nil {
			return filePreviewMsg{err: err}
		}
		defer f.Close()

		data, err := io.ReadAll(io.LimitReader(f, maxPreview))
		if err != nil {
			return filePreviewMsg{err: err}
		}

		return filePreviewMsg{title: remotePath, content: sanitizeTextPreview(string(data))}
	}
}

func (m Model) transferView() string {
	pct := 0.0
	if m.transferTotal > 0 {
		pct = float64(m.transferCopied) / float64(m.transferTotal)
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	line := fmt.Sprintf("%s %d/%d bytes", m.transferLabel, m.transferCopied, m.transferTotal)
	bar := m.transferProgress.ViewAs(pct)
	cancelHint := lipgloss.NewStyle().Foreground(theme.Warning).Render("Press x to cancel")
	return lipgloss.NewStyle().PaddingTop(1).Render(lipgloss.JoinVertical(lipgloss.Left, line, bar, cancelHint))
}

func (m *Model) appendTerminalOutput(raw string) {
	if strings.Contains(raw, "\x1bc") || strings.Contains(raw, "\x1b[2J") {
		m.terminalBuffer = ""
	}

	clean := ansiEscape.ReplaceAllString(raw, "")
	clean = strings.ReplaceAll(clean, "\r", "")
	if clean == "" {
		return
	}

	m.terminalBuffer += clean
	if len(m.terminalBuffer) > 250000 {
		m.terminalBuffer = m.terminalBuffer[len(m.terminalBuffer)-250000:]
	}

	m.viewport.SetContent(m.terminalBuffer)
	m.viewport.GotoBottom()
}

func buildAuthMethods(profile profiles.Profile, runtimePassword string) ([]sshlib.AuthMethod, []string) {
	var methods []sshlib.AuthMethod
	var warnings []string

	password := os.Getenv("ZET_SSH_PASSWORD")
	if strings.TrimSpace(runtimePassword) != "" {
		password = runtimePassword
	}
	keyPassphrase := os.Getenv("ZET_SSH_KEY_PASSPHRASE")
	passwordAdded := false

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
	// Always prioritize key-based auth first (profile key, local private keys, agent).
	addProfileKey()
	addDefaultKeys()
	addAgent()

	// Password fallback is always appended last.
	if strings.TrimSpace(password) != "" {
		methods = append(methods, ssh.PasswordAuth(password))
		methods = append(methods, ssh.KeyboardInteractiveAuth(password))
		passwordAdded = true
	}

	if len(methods) == 0 {
		warnings = append(warnings, "no auth methods built")
	} else if !passwordAdded {
		warnings = append(warnings, "password fallback not set (prompt will appear on auth failure)")
	}

	return methods, warnings
}

func shouldPromptPassword(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unable to authenticate") || strings.Contains(msg, "no supported methods")
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
	case tea.KeyCtrlL:
		return []byte{12}
	case tea.KeyCtrlZ:
		return []byte{26}
	}

	if len(msg.Runes) > 0 {
		return []byte(string(msg.Runes))
	}
	return nil
}

func sanitizeTextPreview(in string) string {
	in = ansiEscape.ReplaceAllString(in, "")
	in = strings.ReplaceAll(in, "\r", "")
	if strings.TrimSpace(in) == "" {
		return "(empty or binary content)"
	}
	return in
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
