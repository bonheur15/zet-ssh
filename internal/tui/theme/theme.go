package theme

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

const FileName = "theme.json"

type Config struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Accent    string `json:"accent"`
	Success   string `json:"success"`
	Error     string `json:"error"`
	Warning   string `json:"warning"`
	Highlight string `json:"highlight"`
	Inactive  string `json:"inactive"`
	OnPrimary string `json:"on_primary"`
}

var (
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Success   lipgloss.Color
	Error     lipgloss.Color
	Warning   lipgloss.Color
	Highlight lipgloss.Color
	Inactive  lipgloss.Color
	OnPrimary lipgloss.Color

	Title         lipgloss.Style
	Header        lipgloss.Style
	Border        lipgloss.Style
	Selected      lipgloss.Style
	StatusSuccess lipgloss.Style
	StatusError   lipgloss.Style
	Modal         lipgloss.Style
)

func Center(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func DefaultConfig() Config {
	return Config{
		Primary:   "62",
		Secondary: "211",
		Accent:    "86",
		Success:   "42",
		Error:     "196",
		Warning:   "214",
		Highlight: "235",
		Inactive:  "241",
		OnPrimary: "230",
	}
}

func Apply(cfg Config) {
	Primary = lipgloss.Color(cfg.Primary)
	Secondary = lipgloss.Color(cfg.Secondary)
	Accent = lipgloss.Color(cfg.Accent)
	Success = lipgloss.Color(cfg.Success)
	Error = lipgloss.Color(cfg.Error)
	Warning = lipgloss.Color(cfg.Warning)
	Highlight = lipgloss.Color(cfg.Highlight)
	Inactive = lipgloss.Color(cfg.Inactive)
	OnPrimary = lipgloss.Color(cfg.OnPrimary)

	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(OnPrimary).
		Background(Primary).
		Padding(0, 1)

	Header = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	Border = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Primary)

	Selected = lipgloss.NewStyle().
		Background(Primary).
		Foreground(OnPrimary).
		Bold(true)

	StatusSuccess = lipgloss.NewStyle().Foreground(Success)
	StatusError = lipgloss.NewStyle().Foreground(Error)

	Modal = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Background(Highlight).
		Padding(1)
}

func Load(configDir string) error {
	cfg := DefaultConfig()
	Apply(cfg)

	path := filepath.Join(configDir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var custom Config
	if err := json.Unmarshal(data, &custom); err != nil {
		return err
	}

	merged := mergeConfig(cfg, custom)
	Apply(merged)
	return nil
}

func mergeConfig(base, override Config) Config {
	if override.Primary != "" {
		base.Primary = override.Primary
	}
	if override.Secondary != "" {
		base.Secondary = override.Secondary
	}
	if override.Accent != "" {
		base.Accent = override.Accent
	}
	if override.Success != "" {
		base.Success = override.Success
	}
	if override.Error != "" {
		base.Error = override.Error
	}
	if override.Warning != "" {
		base.Warning = override.Warning
	}
	if override.Highlight != "" {
		base.Highlight = override.Highlight
	}
	if override.Inactive != "" {
		base.Inactive = override.Inactive
	}
	if override.OnPrimary != "" {
		base.OnPrimary = override.OnPrimary
	}
	return base
}
