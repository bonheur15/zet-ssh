package ssh

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	sshlib "golang.org/x/crypto/ssh"
)

type TunnelType string

const (
	TunnelLocal   TunnelType = "L"
	TunnelRemote  TunnelType = "R"
	TunnelDynamic TunnelType = "D"
)

type TunnelSpec struct {
	Name       string
	Type       TunnelType
	ListenHost string
	ListenPort int
	DestHost   string
	DestPort   int
}

type TunnelRuntime struct {
	Spec      TunnelSpec
	running   bool
	lastError string
	listener  net.Listener
	stopCh    chan struct{}
	mu        sync.RWMutex
}

func NewTunnelRuntime(spec TunnelSpec) *TunnelRuntime {
	return &TunnelRuntime{Spec: spec}
}

func (t *TunnelRuntime) Addr() string {
	return net.JoinHostPort(t.Spec.ListenHost, strconv.Itoa(t.Spec.ListenPort))
}

func (t *TunnelRuntime) Destination() string {
	if t.Spec.Type == TunnelDynamic {
		return "SOCKS5 dynamic"
	}
	return net.JoinHostPort(t.Spec.DestHost, strconv.Itoa(t.Spec.DestPort))
}

func (t *TunnelRuntime) Running() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running
}

func (t *TunnelRuntime) LastError() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastError
}

func (t *TunnelRuntime) setError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err == nil {
		t.lastError = ""
		return
	}
	t.lastError = err.Error()
}

func (t *TunnelRuntime) Start(client *sshlib.Client) error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return nil
	}
	t.stopCh = make(chan struct{})
	t.mu.Unlock()

	var ln net.Listener
	var err error
	switch t.Spec.Type {
	case TunnelLocal, TunnelDynamic:
		ln, err = net.Listen("tcp", t.Addr())
	case TunnelRemote:
		ln, err = client.Listen("tcp", t.Addr())
	default:
		return fmt.Errorf("unsupported tunnel type: %s", t.Spec.Type)
	}
	if err != nil {
		t.setError(err)
		return err
	}

	t.mu.Lock()
	t.listener = ln
	t.running = true
	t.lastError = ""
	t.mu.Unlock()

	go t.acceptLoop(client, ln)
	return nil
}

func (t *TunnelRuntime) Stop() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil
	}
	t.running = false
	if t.stopCh != nil {
		close(t.stopCh)
	}
	ln := t.listener
	t.listener = nil
	t.mu.Unlock()
	if ln != nil {
		return ln.Close()
	}
	return nil
}

func (t *TunnelRuntime) acceptLoop(client *sshlib.Client, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if t.Running() {
				t.setError(err)
			}
			return
		}
		go t.handleConn(client, conn)
	}
}

func (t *TunnelRuntime) handleConn(client *sshlib.Client, src net.Conn) {
	switch t.Spec.Type {
	case TunnelLocal:
		dst, err := client.Dial("tcp", t.Destination())
		if err != nil {
			t.setError(err)
			_ = src.Close()
			return
		}
		pipeBoth(src, dst)
	case TunnelRemote:
		dst, err := net.Dial("tcp", t.Destination())
		if err != nil {
			t.setError(err)
			_ = src.Close()
			return
		}
		pipeBoth(src, dst)
	case TunnelDynamic:
		if err := handleSOCKS5(client, src); err != nil {
			t.setError(err)
			_ = src.Close()
		}
	}
}

func pipeBoth(a net.Conn, b net.Conn) {
	go func() {
		_, _ = io.Copy(a, b)
		_ = a.Close()
		_ = b.Close()
	}()
	go func() {
		_, _ = io.Copy(b, a)
		_ = a.Close()
		_ = b.Close()
	}()
}

func handleSOCKS5(client *sshlib.Client, conn net.Conn) error {
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetDeadline(time.Time{})

	buf := make([]byte, 262)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return err
	}
	if buf[0] != 0x05 {
		return fmt.Errorf("unsupported socks version")
	}
	nMethods := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:nMethods]); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return err
	}

	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return err
	}
	if buf[0] != 0x05 || buf[1] != 0x01 {
		return fmt.Errorf("unsupported socks command")
	}
	atyp := buf[3]
	host := ""
	switch atyp {
	case 0x01:
		if _, err := io.ReadFull(conn, buf[:4]); err != nil {
			return err
		}
		host = net.IP(buf[:4]).String()
	case 0x03:
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return err
		}
		l := int(buf[0])
		if _, err := io.ReadFull(conn, buf[:l]); err != nil {
			return err
		}
		host = string(buf[:l])
	case 0x04:
		if _, err := io.ReadFull(conn, buf[:16]); err != nil {
			return err
		}
		host = net.IP(buf[:16]).String()
	default:
		return fmt.Errorf("unsupported address type")
	}
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return err
	}
	port := binary.BigEndian.Uint16(buf[:2])
	target := net.JoinHostPort(host, strconv.Itoa(int(port)))

	dst, err := client.Dial("tcp", target)
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return err
	}

	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		_ = dst.Close()
		return err
	}

	pipeBoth(conn, dst)
	return nil
}
