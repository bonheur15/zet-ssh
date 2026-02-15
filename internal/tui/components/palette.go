package components

import (
	"zet-ssh/internal/tui/theme"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type PaletteItem struct {
	TitleStr string
	DescStr  string
	Action   func() tea.Cmd
}

func (i PaletteItem) Title() string       { return i.TitleStr }
func (i PaletteItem) Description() string { return i.DescStr }
func (i PaletteItem) FilterValue() string { return i.TitleStr + " " + i.DescStr }

type Palette struct {
	list   list.Model
	active bool
}

func NewPalette() Palette {
	items := []list.Item{
		PaletteItem{TitleStr: "Connect to Profile", DescStr: "Select and connect to a saved SSH profile"},
		PaletteItem{TitleStr: "Add New Profile", DescStr: "Create a new SSH connection profile"},
		PaletteItem{TitleStr: "Manage Vault", DescStr: "Open the encrypted secrets vault"},
		PaletteItem{TitleStr: "Help", DescStr: "Show keyboard shortcuts and help"},
		PaletteItem{TitleStr: "Quit", DescStr: "Exit Zet-SSH"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Command Palette"
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	return Palette{
		list:   l,
		active: false,
	}
}

func (m Palette) Init() tea.Cmd {
	return nil
}

func (m Palette) Update(msg tea.Msg) (Palette, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-4, msg.Height-4)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.active = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Palette) View() string {
	if !m.active {
		return ""
	}

	style := theme.Modal.Copy().
		BorderForeground(theme.Primary).
		Background(theme.Highlight)
	return style.Render(m.list.View())
}

func (m *Palette) Toggle() {
	m.active = !m.active
}

func (m Palette) IsActive() bool {
	return m.active
}
