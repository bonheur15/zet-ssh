package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"zet-ssh/internal/core/profiles"
	coreSSH "zet-ssh/internal/core/ssh"
	"zet-ssh/internal/tui"
	"zet-ssh/internal/update"

	tea "github.com/charmbracelet/bubbletea"
	sshlib "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	model := tui.NewAppModel()
	if custom, handled := runCLICommand(os.Args[1:]); handled {
		if custom != nil {
			model = *custom
		} else {
			return
		}
	}
	if len(os.Args[1:]) == 0 {
		runAutoUpdateIfEnabled()
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func runAutoUpdateIfEnabled() {
	flag := strings.ToLower(strings.TrimSpace(os.Getenv("ZET_SSH_AUTO_UPDATE")))
	if flag != "1" && flag != "true" && flag != "yes" {
		return
	}

	repo := os.Getenv("ZET_SSH_REPOSITORY")
	if strings.TrimSpace(repo) == "" {
		repo = "bonheur15/zet-ssh"
	}

	_ = update.Run(update.Options{
		Repository: repo,
		Yes:        true,
	})
}

func runCLICommand(args []string) (*tui.AppModel, bool) {
	if len(args) == 0 {
		return nil, false
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("zet %s (commit=%s date=%s)\n", update.Version, update.Commit, update.Date)
		return nil, true
	case "update":
		repo := os.Getenv("ZET_SSH_REPOSITORY")
		if strings.TrimSpace(repo) == "" {
			repo = "bonheur15/zet-ssh"
		}

		opts := update.Options{
			Repository: repo,
		}
		for _, a := range args[1:] {
			switch a {
			case "--check":
				opts.CheckOnly = true
			case "--yes", "-y", "--auto":
				opts.Yes = true
			}
		}

		if err := update.Run(opts); err != nil {
			fmt.Println("update failed:", err)
			os.Exit(1)
		}
		return nil, true
	case "connect":
		if len(args) < 2 {
			fmt.Println("usage: zet connect <name>")
			os.Exit(1)
		}
		p, err := loadProfileByName(args[1])
		if err != nil {
			fmt.Println("connect failed:", err)
			os.Exit(1)
		}
		model := tui.NewAppModelWithInitialProfile(&p)
		return &model, true
	case "run":
		if len(args) < 4 {
			fmt.Println("usage: zet run <name> -- <command>")
			os.Exit(1)
		}
		sep := -1
		for i := 2; i < len(args); i++ {
			if args[i] == "--" {
				sep = i
				break
			}
		}
		if sep == -1 || sep == len(args)-1 {
			fmt.Println("usage: zet run <name> -- <command>")
			os.Exit(1)
		}
		p, err := loadProfileByName(args[1])
		if err != nil {
			fmt.Println("run failed:", err)
			os.Exit(1)
		}
		command := strings.Join(args[sep+1:], " ")
		if err := runHeadless(p, command); err != nil {
			fmt.Println("run failed:", err)
			os.Exit(1)
		}
		return nil, true
	case "help", "--help", "-h":
		fmt.Println("zet - terminal SSH workspace")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Println("  zet                # open TUI")
		fmt.Println("  zet connect <name> # open TUI and connect to a named profile")
		fmt.Println("  zet run <name> -- <command> # run remote command and exit")
		fmt.Println("  zet version        # show version")
		fmt.Println("  zet update         # update from latest GitHub release")
		fmt.Println("  zet update --check # only check for newer release")
		fmt.Println("")
		fmt.Println("Environment:")
		fmt.Println("  ZET_SSH_REPOSITORY=<owner/repo>  release repository (default: bonheur15/zet-ssh)")
		return nil, true
	default:
		return nil, false
	}
}

func loadProfileByName(name string) (profiles.Profile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return profiles.Profile{}, err
	}
	store, err := profiles.NewStore(filepath.Join(home, ".config", "zet-ssh"))
	if err != nil {
		return profiles.Profile{}, err
	}

	want := strings.TrimSpace(strings.ToLower(name))
	for _, p := range store.List() {
		if strings.ToLower(strings.TrimSpace(p.Name)) == want {
			return p, nil
		}
	}
	return profiles.Profile{}, fmt.Errorf("profile not found: %s", name)
}

func runHeadless(p profiles.Profile, command string) error {
	auth, warnings := buildAuthMethodsForCLI(p)
	if len(auth) == 0 {
		return fmt.Errorf("no auth methods available (%s)", strings.Join(warnings, "; "))
	}

	client, err := coreSSH.NewClient(p.User, p.Host, p.Port, auth)
	if err != nil {
		return err
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	if p.AgentForward {
		if err := enableAgentForwardingForSession(client, sess); err != nil {
			return err
		}
	}

	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	runErr := sess.Run(command)
	if stdout.Len() > 0 {
		_, _ = os.Stdout.Write(stdout.Bytes())
	}
	if stderr.Len() > 0 {
		_, _ = os.Stderr.Write(stderr.Bytes())
	}
	return runErr
}

func buildAuthMethodsForCLI(profile profiles.Profile) ([]sshlib.AuthMethod, []string) {
	var methods []sshlib.AuthMethod
	var warnings []string

	password := os.Getenv("ZET_SSH_PASSWORD")
	keyPassphrase := os.Getenv("ZET_SSH_KEY_PASSPHRASE")

	addAgent := func() {
		agentAuth, err := coreSSH.AgentAuth()
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
		auth, err := coreSSH.PublicKeyAuthFromFile(profile.KeyPath, []byte(keyPassphrase))
		if err != nil {
			warnings = append(warnings, "profile key failed")
			return
		}
		methods = append(methods, auth)
	}
	addDefaultKeys := func() {
		for _, keyPath := range coreSSH.DefaultPrivateKeyPaths() {
			auth, err := coreSSH.PublicKeyAuthFromFile(keyPath, []byte(keyPassphrase))
			if err == nil {
				methods = append(methods, auth)
			}
		}
	}

	switch profile.AuthType {
	case profiles.AuthPassword:
	case profiles.AuthKey:
		addProfileKey()
		addAgent()
	default:
		addProfileKey()
		addDefaultKeys()
		addAgent()
	}

	if strings.TrimSpace(password) != "" {
		methods = append(methods, coreSSH.PasswordAuth(password))
		methods = append(methods, coreSSH.KeyboardInteractiveAuth(password))
	}
	if len(methods) == 0 {
		warnings = append(warnings, "no auth methods built")
	}
	return methods, warnings
}

func enableAgentForwardingForSession(client *sshlib.Client, session *sshlib.Session) error {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return fmt.Errorf("SSH_AUTH_SOCK is not set")
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return err
	}
	agentClient := agent.NewClient(conn)
	if err := agent.ForwardToAgent(client, agentClient); err != nil {
		_ = conn.Close()
		return err
	}
	if err := agent.RequestAgentForwarding(session); err != nil {
		_ = conn.Close()
		return err
	}
	return nil
}
