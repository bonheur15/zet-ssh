package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"zet-ssh/internal/core/profiles"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
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
	list       list.Model
	store      *profiles.Store
	form       Form
	statusMsg  string
	pasteOpen  bool
	pasteInput textinput.Model
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
	m.pasteInput = textinput.New()
	m.pasteInput.Placeholder = "Paste ssh command (e.g. ssh user@host -p 22 -i ~/.ssh/id_ed25519)"
	m.pasteInput.CharLimit = 1024
	m.reloadItems()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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

	if m.form.active {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "esc" {
				m.form.active = false
				m.form.errMsg = ""
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.form, cmd = m.form.Update(msg)
		return m, cmd
	}
	if m.pasteOpen {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.pasteOpen = false
				m.pasteInput.SetValue("")
				return m, nil
			case "enter":
				cmdText := strings.TrimSpace(m.pasteInput.Value())
				if cmdText == "" {
					m.statusMsg = "Paste import failed: empty command"
					return m, nil
				}
				p, err := profiles.ParseSSHCommandProfile(cmdText)
				if err != nil {
					m.statusMsg = "Paste import failed: " + err.Error()
					return m, nil
				}
				if err := m.store.Upsert(p); err != nil {
					m.statusMsg = "Paste import failed: " + err.Error()
					return m, nil
				}
				m.reloadItems()
				m.selectProfileByID(p.ID)
				m.pasteOpen = false
				m.pasteInput.SetValue("")
				m.statusMsg = "Imported profile from ssh command"
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.pasteInput, cmd = m.pasteInput.Update(msg)
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
		case "p":
			m.pasteOpen = true
			m.pasteInput.SetValue("")
			m.pasteInput.Focus()
			m.statusMsg = "Paste an ssh command and press Enter"
			return m, nil
		case "i":
			home, err := os.UserHomeDir()
			if err != nil {
				m.statusMsg = "Import failed: could not resolve home directory"
				return m, nil
			}
			configPath := filepath.Join(home, ".ssh", "config")
			imported, err := profiles.ImportSSHConfig(configPath)
			if err != nil {
				m.statusMsg = "Import failed: " + err.Error()
				return m, nil
			}
			for _, p := range imported {
				if upsertErr := m.store.Upsert(p); upsertErr != nil {
					m.statusMsg = "Import failed: " + upsertErr.Error()
					return m, nil
				}
			}
			m.reloadItems()
			m.statusMsg = "Imported profiles from ~/.ssh/config"
			return m, nil
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
	if m.pasteOpen {
		content := lipgloss.JoinVertical(lipgloss.Left,
			"Import SSH Command",
			"",
			m.pasteInput.View(),
			"",
			"(Enter import, Esc cancel)",
		)
		return lipgloss.Place(m.list.Width(), m.list.Height(),
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				Padding(1).
				Width(max(60, m.list.Width()-10)).
				Render(content))
	}

	view := m.list.View()
	if m.statusMsg != "" {
		view = lipgloss.JoinVertical(lipgloss.Left, view, m.statusMsg)
	}
	view = lipgloss.JoinVertical(lipgloss.Left, view, "[n] New  [e] Edit  [p] Paste ssh cmd  [i] Import ~/.ssh/config  [enter] Connect  (in form: Ctrl+S save, Ctrl+G save+connect)")

	return lipgloss.NewStyle().Margin(1, 2).Render(view)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
