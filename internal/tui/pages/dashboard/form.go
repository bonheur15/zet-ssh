package dashboard

import (
	"zet-ssh/internal/core/profiles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Form struct {
	name     textinput.Model
	host     textinput.Model
	user     textinput.Model
	port     textinput.Model
	focusIndex int
	active   bool
}

func NewForm() Form {
	n := textinput.New()
	n.Placeholder = "Profile Name (e.g. Prod)"
	n.Focus()

	h := textinput.New()
	h.Placeholder = "Host (e.g. 1.2.3.4)"

	u := textinput.New()
	u.Placeholder = "User (e.g. root)"

	p := textinput.New()
	p.Placeholder = "Port (default 22)"

	return Form{
		name: n,
		host: h,
		user: u,
		port: p,
		active: false,
	}
}

func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	if !f.active {
		return f, nil
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			if s == "enter" && f.focusIndex == 3 {
				// Submit
				return f, func() tea.Msg {
					return profiles.Profile{
						Name: f.name.Value(),
						Host: f.host.Value(),
						User: f.user.Value(),
						Port: 22, // Should parse f.port.Value()
					}
				}
			}

			if s == "up" || s == "shift+tab" {
				f.focusIndex--
			} else {
				f.focusIndex++
			}

			if f.focusIndex > 3 {
				f.focusIndex = 0
			} else if f.focusIndex < 0 {
				f.focusIndex = 3
			}

			cmds = append(cmds, f.updateFocus())
		}
	}

	var cmd tea.Cmd
	f.name, cmd = f.name.Update(msg)
	cmds = append(cmds, cmd)
	f.host, cmd = f.host.Update(msg)
	cmds = append(cmds, cmd)
	f.user, cmd = f.user.Update(msg)
	cmds = append(cmds, cmd)
	f.port, cmd = f.port.Update(msg)
	cmds = append(cmds, cmd)

	return f, tea.Batch(cmds...)
}

func (f *Form) updateFocus() tea.Cmd {
	f.name.Blur()
	f.host.Blur()
	f.user.Blur()
	f.port.Blur()

	switch f.focusIndex {
	case 0:
		return f.name.Focus()
	case 1:
		return f.host.Focus()
	case 2:
		return f.user.Focus()
	case 3:
		return f.port.Focus()
	}
	return nil
}

func (f Form) View() string {
	if !f.active {
		return ""
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		"Add New Profile",
		"",
		f.name.View(),
		f.host.View(),
		f.user.View(),
		f.port.View(),
		"",
		"(Enter to submit, Esc to cancel)",
	)
}
