package main

import (
	"fmt"
	"os"
	"strings"
	"zet-ssh/internal/tui"
	"zet-ssh/internal/update"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if handled := runCLICommand(os.Args[1:]); handled {
		return
	}
	runAutoUpdateIfEnabled()

	p := tea.NewProgram(tui.NewAppModel(), tea.WithAltScreen())
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
		repo = "bonheur/zet-ssh-4"
	}

	_ = update.Run(update.Options{
		Repository: repo,
		Yes:        true,
	})
}

func runCLICommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("zet %s (commit=%s date=%s)\n", update.Version, update.Commit, update.Date)
		return true
	case "update":
		repo := os.Getenv("ZET_SSH_REPOSITORY")
		if strings.TrimSpace(repo) == "" {
			repo = "bonheur/zet-ssh-4"
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
		return true
	case "help", "--help", "-h":
		fmt.Println("zet - terminal SSH workspace")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Println("  zet                # open TUI")
		fmt.Println("  zet version        # show version")
		fmt.Println("  zet update         # update from latest GitHub release")
		fmt.Println("  zet update --check # only check for newer release")
		fmt.Println("")
		fmt.Println("Environment:")
		fmt.Println("  ZET_SSH_REPOSITORY=<owner/repo>  release repository (default: bonheur/zet-ssh-4)")
		return true
	default:
		return false
	}
}
