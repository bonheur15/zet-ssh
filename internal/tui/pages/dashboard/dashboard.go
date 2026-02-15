package dashboard

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type Model struct {
	list list.Model
}

func New() Model {
	items := []list.Item{
		item{title: "Production DB", desc: "192.168.1.10 - PostgreSQL"},
		item{title: "Staging API", desc: "10.0.0.5 - Docker Swarm"},
		item{title: "Local Dev", desc: "localhost:2222 - Vagrant"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "SSH Profiles"
	l.SetShowHelp(false)

	return Model{list: l}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return lipgloss.NewStyle().Margin(1, 2).Render(m.list.View())
}
