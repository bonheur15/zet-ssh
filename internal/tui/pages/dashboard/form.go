package dashboard

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"zet-ssh/internal/core/profiles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Form struct {
	id         string
	name       textinput.Model
	host       textinput.Model
	user       textinput.Model
	port       textinput.Model
	focusIndex int
	active     bool
	editing    bool
	errMsg     string
}

type saveAction int

const (
	saveOnly saveAction = iota
	saveAndConnect
)

type formSaveMsg struct {
	profile profiles.Profile
	connect bool
}

func NewForm() Form {
	f := Form{}
	f.initInputs()
	return f
}

func NewFormForProfile(p profiles.Profile) Form {
	f := NewForm()
	f.id = p.ID
	f.editing = true
	f.active = true
	f.name.SetValue(p.Name)
	f.host.SetValue(p.Host)
	f.user.SetValue(p.User)
	if p.Port > 0 {
		f.port.SetValue(strconv.Itoa(p.Port))
	}
	return f
}

func (f *Form) OpenNew() {
	f.id = ""
	f.editing = false
	f.errMsg = ""
	f.focusIndex = 0
	f.initInputs()
	f.active = true
}

func (f *Form) initInputs() {
	n := textinput.New()
	n.Placeholder = "Profile Name (e.g. Prod)"
	n.Focus()
	n.CharLimit = 80

	h := textinput.New()
	h.Placeholder = "Host (e.g. 100.77.94.51)"
	h.CharLimit = 255

	u := textinput.New()
	u.Placeholder = "User (e.g. root)"
	u.CharLimit = 80

	p := textinput.New()
	p.Placeholder = "Port (default 22)"
	p.CharLimit = 5

	f.name = n
	f.host = h
	f.user = u
	f.port = p
}

func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	if !f.active {
		return f, nil
	}

	var cmds []tea.Cmd

	submit := func(action saveAction) (Form, tea.Cmd) {
		profile, err := f.buildProfile()
		if err != nil {
			f.errMsg = err.Error()
			return f, nil
		}
		f.active = false
		f.errMsg = ""
		return f, func() tea.Msg {
			return formSaveMsg{
				profile: profile,
				connect: action == saveAndConnect,
			}
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			return submit(saveOnly)
		case "ctrl+g":
			return submit(saveAndConnect)
		case "tab", "shift+tab", "up", "down":
			if msg.String() == "up" || msg.String() == "shift+tab" {
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
		case "enter":
			if f.focusIndex == 3 {
				return submit(saveOnly)
			}
			f.focusIndex++
			if f.focusIndex > 3 {
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

func (f Form) buildProfile() (profiles.Profile, error) {
	name := strings.TrimSpace(f.name.Value())
	host := strings.TrimSpace(f.host.Value())
	user := strings.TrimSpace(f.user.Value())
	portStr := strings.TrimSpace(f.port.Value())

	if name == "" {
		return profiles.Profile{}, fmt.Errorf("name is required")
	}
	if host == "" {
		return profiles.Profile{}, fmt.Errorf("host is required")
	}
	if user == "" {
		return profiles.Profile{}, fmt.Errorf("user is required")
	}

	port := 22
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p <= 0 || p > 65535 {
			return profiles.Profile{}, fmt.Errorf("port must be a number between 1 and 65535")
		}
		port = p
	}

	id := f.id
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	return profiles.Profile{
		ID:       id,
		Name:     name,
		Host:     host,
		User:     user,
		Port:     port,
		AuthType: profiles.AuthAgent,
	}, nil
}

func (f Form) View() string {
	if !f.active {
		return ""
	}

	title := "Add New Profile"
	if f.editing {
		title = "Edit Profile"
	}

	errLine := ""
	if f.errMsg != "" {
		errLine = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(f.errMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		f.name.View(),
		f.host.View(),
		f.user.View(),
		f.port.View(),
		"",
		errLine,
		"(Ctrl+S save, Ctrl+G save & connect, Enter on port saves, Esc cancels)",
	)
}
