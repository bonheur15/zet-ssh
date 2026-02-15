package ssh

import (
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/ssh"
)

type Session struct {
	client  *ssh.Client
	session *ssh.Session
}

func Connect(user, host string, port int, auth []ssh.AuthMethod) (*Session, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key management
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

	return &Session{
		client:  client,
		session: session,
	}, nil
}

func (s *Session) StartShell(stdin io.Reader, stdout, stderr io.Writer, width, height int) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := s.session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return err
	}

	s.session.Stdin = stdin
	s.session.Stdout = stdout
	s.session.Stderr = stderr

	if err := s.session.Shell(); err != nil {
		return err
	}

	return nil
}

func (s *Session) Wait() error {
	return s.session.Wait()
}

func (s *Session) Close() {
	s.session.Close()
	s.client.Close()
}

func (s *Session) Resize(width, height int) error {
	return s.session.WindowChange(height, width)
}

// Helper to get local ssh-agent auth
func GetAgentAuth() (ssh.AuthMethod, error) {
	// Not implemented here yet, but standard practice
	return nil, fmt.Errorf("agent auth not implemented")
}
