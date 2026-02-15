package tunnels

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TunnelItem struct {
	Name string
	Type string // L, R, D
	Port string
}

func (i TunnelItem) Title() string       { return i.Name }
func (i TunnelItem) Description() string { return i.Type + " " + i.Port }
func (i TunnelItem) FilterValue() string { return i.Name }

type Model struct {
	list     list.Model
	active   bool
	creating bool
}

func New() Model {
	items := []list.Item{
		TunnelItem{Name: "DB Tunnel", Type: "L", Port: "5432:localhost:5432"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Tunnel Builder"

	return Model{
		list: l,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	}
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return lipgloss.NewStyle().Margin(1, 2).Render(m.list.View())
}
