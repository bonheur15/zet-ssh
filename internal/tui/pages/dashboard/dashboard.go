package dashboard

import (
	"zet-ssh/internal/core/profiles"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type item struct {
	profile profiles.Profile
}

func (i item) Title() string       { return i.profile.Name }
func (i item) Description() string { return i.profile.User + "@" + i.profile.Host }
func (i item) FilterValue() string { return i.profile.Name + " " + i.profile.Host }

type Model struct {
	list  list.Model
	store *profiles.Store
}

func New(store *profiles.Store) Model {
	var items []list.Item
	if store != nil {
		for _, p := range store.List() {
			items = append(items, item{profile: p})
		}
	}

	if len(items) == 0 {
		// Placeholder if empty
		items = append(items, item{profile: profiles.Profile{Name: "No Profiles", Host: "Add one to start"}})
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Zet-SSH | Connections"
	l.SetShowHelp(true)

	return Model{
		list:  l,
		store: store,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if i, ok := m.list.SelectedItem().(item); ok {
				return m, func() tea.Msg {
					return i.profile
				}
			}
		}
	case profiles.Profile: // Signal to add/update profile (placeholder)
		m.list.InsertItem(len(m.list.Items()), item{profile: msg})
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return lipgloss.NewStyle().Margin(1, 2).Render(m.list.View())
}
