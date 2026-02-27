package profiles

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ParseSSHCommandProfile(cmd string) (Profile, error) {
	fields := strings.Fields(strings.TrimSpace(cmd))
	if len(fields) < 2 {
		return Profile{}, fmt.Errorf("expected command like: ssh user@host -p 22")
	}
	if fields[0] != "ssh" {
		return Profile{}, fmt.Errorf("command must start with ssh")
	}

	var (
		userArg string
		hostArg string
		port    = 22
		keyPath string
		login   string
	)

	for i := 1; i < len(fields); i++ {
		token := fields[i]
		switch token {
		case "-p":
			if i+1 >= len(fields) {
				return Profile{}, fmt.Errorf("missing value after -p")
			}
			p, err := strconv.Atoi(fields[i+1])
			if err != nil || p < 1 || p > 65535 {
				return Profile{}, fmt.Errorf("invalid port: %s", fields[i+1])
			}
			port = p
			i++
		case "-i":
			if i+1 >= len(fields) {
				return Profile{}, fmt.Errorf("missing value after -i")
			}
			keyPath = fields[i+1]
			i++
		case "-l":
			if i+1 >= len(fields) {
				return Profile{}, fmt.Errorf("missing value after -l")
			}
			login = fields[i+1]
			i++
		default:
			if strings.HasPrefix(token, "-") {
				// Ignore flags not currently mapped to profile fields.
				continue
			}
			if strings.Contains(token, "@") {
				parts := strings.SplitN(token, "@", 2)
				userArg = parts[0]
				hostArg = parts[1]
			} else {
				hostArg = token
			}
		}
	}

	if hostArg == "" {
		return Profile{}, fmt.Errorf("missing host in ssh command")
	}
	user := userArg
	if strings.TrimSpace(user) == "" {
		user = login
	}
	if strings.TrimSpace(user) == "" {
		user = "root"
	}

	authType := AuthAgent
	if strings.TrimSpace(keyPath) != "" {
		authType = AuthKey
	}

	return Profile{
		ID:           fmt.Sprintf("sshcmd-%d", time.Now().UnixNano()),
		Name:         hostArg,
		Host:         hostArg,
		Port:         port,
		User:         user,
		AuthType:     authType,
		KeyPath:      keyPath,
		AgentForward: false,
	}, nil
}
