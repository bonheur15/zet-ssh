package profiles

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type sshHostBlock struct {
	aliases  []string
	hostName string
	user     string
	port     int
	identity string
}

func ImportSSHConfig(path string) ([]Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var blocks []sshHostBlock
	var current *sshHostBlock

	flushCurrent := func() {
		if current == nil || len(current.aliases) == 0 {
			return
		}
		blocks = append(blocks, *current)
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.TrimSpace(line[len(fields[0]):])
		value = strings.TrimSpace(value)

		switch key {
		case "host":
			flushCurrent()
			aliases := filterExplicitAliases(strings.Fields(value))
			current = &sshHostBlock{aliases: aliases, port: 22}
		case "hostname":
			if current != nil {
				current.hostName = value
			}
		case "user":
			if current != nil {
				current.user = value
			}
		case "port":
			if current != nil {
				if p, convErr := strconv.Atoi(value); convErr == nil && p > 0 && p < 65536 {
					current.port = p
				}
			}
		case "identityfile":
			if current != nil && current.identity == "" {
				current.identity = expandHome(value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flushCurrent()

	var imported []Profile
	for _, block := range blocks {
		for _, alias := range block.aliases {
			host := block.hostName
			if host == "" {
				host = alias
			}
			if strings.TrimSpace(host) == "" {
				continue
			}

			user := block.user
			if user == "" {
				user = os.Getenv("USER")
			}
			if user == "" {
				user = "root"
			}

			p := Profile{
				ID:       "sshcfg-" + alias,
				Name:     alias,
				Host:     host,
				Port:     max(1, block.port),
				User:     user,
				AuthType: AuthAgent,
			}
			if block.identity != "" {
				p.AuthType = AuthKey
				p.KeyPath = block.identity
			}
			imported = append(imported, p)
		}
	}

	if len(imported) == 0 {
		return nil, fmt.Errorf("no explicit host entries found in %s", path)
	}
	return imported, nil
}

func filterExplicitAliases(aliases []string) []string {
	out := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if strings.ContainsAny(alias, "*?!") {
			continue
		}
		if alias == "" {
			continue
		}
		out = append(out, alias)
	}
	return out
}

func expandHome(v string) string {
	if !strings.HasPrefix(v, "~") {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return v
	}
	if v == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(v, "~/"))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
