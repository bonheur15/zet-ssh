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
	list      list.Model
	store     *profiles.Store
	form      Form
	statusMsg string
}

func New(store *profiles.Store) Model {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Zet-SSH | Connections"
	l.SetShowHelp(true)

	m := Model{
		list:  l,
		store: store,
		form:  NewForm(),
	}
	m.reloadItems()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.form.active {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "esc" {
				m.form.active = false
				m.form.errMsg = ""
				return m, nil
			}
		case formSaveMsg:
			if err := m.store.Upsert(msg.profile); err != nil {
				m.statusMsg = "Save failed: " + err.Error()
				return m, nil
			}
			m.reloadItems()
			m.selectProfileByID(msg.profile.ID)
			if msg.connect {
				m.statusMsg = "Profile saved and connecting..."
				return m, func() tea.Msg {
					return msg.profile
				}
			}
			m.statusMsg = "Profile saved"
			m.form.active = false
			return m, nil
		}
		var cmd tea.Cmd
		m.form, cmd = m.form.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	case tea.KeyMsg:
		switch msg.String() {
		case "n":
			m.form.OpenNew()
			return m, nil
		case "e":
			if i, ok := m.list.SelectedItem().(item); ok {
				m.form = NewFormForProfile(i.profile)
			}
			return m, nil
		case "enter":
			if i, ok := m.list.SelectedItem().(item); ok {
				return m, func() tea.Msg {
					return i.profile
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.form.active {
		return lipgloss.Place(m.list.Width(), m.list.Height(),
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				Padding(1).
				Render(m.form.View()))
	}

	view := m.list.View()
	if m.statusMsg != "" {
		view = lipgloss.JoinVertical(lipgloss.Left, view, m.statusMsg)
	}
	view = lipgloss.JoinVertical(lipgloss.Left, view, "[n] New  [e] Edit  [enter] Connect  (in form: Ctrl+S save, Ctrl+G save+connect)")

	return lipgloss.NewStyle().Margin(1, 2).Render(view)
}

func (m *Model) reloadItems() {
	var items []list.Item
	if m.store != nil {
		for _, p := range m.store.List() {
			items = append(items, item{profile: p})
		}
	}

	if len(items) == 0 {
		items = append(items, item{profile: profiles.Profile{Name: "No Profiles", Host: "Add one to start"}})
	}
	m.list.SetItems(items)
}

func (m *Model) selectProfileByID(id string) {
	items := m.list.Items()
	for idx := range items {
		it, ok := items[idx].(item)
		if ok && it.profile.ID == id {
			m.list.Select(idx)
			return
		}
	}
}
