package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Session struct {
	client    *ssh.Client
	session   *ssh.Session
	stdin     io.WriteCloser
	stdout    io.Reader
	stderr    io.Reader
	agentConn io.Closer
}

func Connect(user, host string, port int, auth []ssh.AuthMethod) (*Session, error) {
	hostKeyCallback, err := HostKeyCallbackFromKnownHosts(DefaultKnownHostsFiles()...)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}

	return &Session{
		client:  client,
		session: session,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
	}, nil
}

// DefaultKnownHostsFiles returns preferred known_hosts files in search order.
func DefaultKnownHostsFiles() []string {
	var files []string

	home, err := os.UserHomeDir()
	if err != nil {
		return files
	}

	candidates := []string{
		filepath.Join(home, ".config", "zet-ssh", "known_hosts"),
		filepath.Join(home, ".ssh", "known_hosts"),
	}

	for _, p := range candidates {
		if info, statErr := os.Stat(p); statErr == nil && !info.IsDir() {
			files = append(files, p)
		}
	}

	return files
}

// HostKeyCallbackFromKnownHosts creates a strict host-key callback from one or more files.
func HostKeyCallbackFromKnownHosts(files ...string) (ssh.HostKeyCallback, error) {
	var existing []string
	for _, f := range files {
		if strings.TrimSpace(f) == "" {
			continue
		}
		if info, err := os.Stat(f); err == nil && !info.IsDir() {
			existing = append(existing, f)
		}
	}
	if len(existing) == 0 {
		return nil, fmt.Errorf("no known_hosts file found (expected ~/.config/zet-ssh/known_hosts or ~/.ssh/known_hosts)")
	}
	return knownhosts.New(existing...)
}

func (s *Session) StartShell(width, height int) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := s.session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return err
	}

	if err := s.session.Shell(); err != nil {
		return err
	}

	return nil
}

func (s *Session) Write(p []byte) (n int, err error) {
	return s.stdin.Write(p)
}

func (s *Session) Read(p []byte) (n int, err error) {
	return s.stdout.Read(p)
}

func (s *Session) Wait() error {
	return s.session.Wait()
}

func (s *Session) Close() {
	if s.agentConn != nil {
		_ = s.agentConn.Close()
	}
	s.session.Close()
	s.client.Close()
}

func (s *Session) Resize(width, height int) error {
	return s.session.WindowChange(height, width)
}

// Client returns the underlying SSH client for operations like SFTP.
func (s *Session) Client() *ssh.Client {
	return s.client
}

// EnableAgentForwarding forwards the local SSH agent into this remote session.
func (s *Session) EnableAgentForwarding() error {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return fmt.Errorf("SSH_AUTH_SOCK is not set")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return err
	}

	agentClient := agent.NewClient(conn)
	if err := agent.ForwardToAgent(s.client, agentClient); err != nil {
		_ = conn.Close()
		return err
	}
	if err := agent.RequestAgentForwarding(s.session); err != nil {
		_ = conn.Close()
		return err
	}

	s.agentConn = conn
	return nil
}

// PasswordAuth returns an auth method using the provided password.
func PasswordAuth(pass string) ssh.AuthMethod {
	return ssh.Password(pass)
}

// KeyboardInteractiveAuth returns keyboard-interactive auth method using the provided password.
func KeyboardInteractiveAuth(pass string) ssh.AuthMethod {
	return ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range questions {
			answers[i] = pass
		}
		return answers, nil
	})
}

// PublicKeyAuth returns an auth method using the provided private key data.
func PublicKeyAuth(keyData []byte) (ssh.AuthMethod, error) {
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

// PublicKeyAuthWithPassphrase returns an auth method using the provided encrypted private key data.
func PublicKeyAuthWithPassphrase(keyData []byte, passphrase []byte) (ssh.AuthMethod, error) {
	signer, err := ssh.ParsePrivateKeyWithPassphrase(keyData, passphrase)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

// AgentAuth returns an auth method backed by SSH agent signers.
func AgentAuth() (ssh.AuthMethod, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK is not set")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), nil
}

// PublicKeyAuthFromFile loads an auth method from a private key file.
func PublicKeyAuthFromFile(keyPath string, passphrase []byte) (ssh.AuthMethod, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	if len(passphrase) > 0 {
		return PublicKeyAuthWithPassphrase(data, passphrase)
	}
	return PublicKeyAuth(data)
}

// DefaultPrivateKeyPaths returns common key file paths if they exist.
func DefaultPrivateKeyPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	baseCandidates := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
		filepath.Join(home, ".ssh", "id_dsa"),
	}

	var candidates []string
	candidates = append(candidates, baseCandidates...)

	// Also discover custom id_* private keys (for keys with non-default names).
	if matches, globErr := filepath.Glob(filepath.Join(home, ".ssh", "id_*")); globErr == nil {
		sort.Strings(matches)
		candidates = append(candidates, matches...)
	}

	seen := make(map[string]struct{})
	var keys []string
	for _, keyPath := range candidates {
		if _, ok := seen[keyPath]; ok {
			continue
		}
		seen[keyPath] = struct{}{}

		if strings.HasSuffix(keyPath, ".pub") || strings.HasSuffix(keyPath, "-cert.pub") {
			continue
		}
		base := filepath.Base(keyPath)
		switch base {
		case "known_hosts", "known_hosts.old", "config", "authorized_keys":
			continue
		}

		if info, statErr := os.Stat(keyPath); statErr == nil && !info.IsDir() {
			keys = append(keys, keyPath)
		}
	}
	return keys
}
