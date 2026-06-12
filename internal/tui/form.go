package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/Yue2u/marina/internal/core"
)

const (
	fieldName     = 0
	fieldHostname = 1
	fieldUser     = 2
	fieldPort     = 3
	fieldAuth     = 4
	fieldKeyPath  = 5
	numFields     = 6
)

var fieldLabels = [numFields]string{
	"name      ",
	"hostname  ",
	"user      ",
	"port      ",
	"auth      ",
	"key path  ",
}

type hostForm struct {
	fields  [numFields]textinput.Model
	focused int
}

type hostSavedMsg struct{ host core.Host }
type formCancelledMsg struct{}

func newHostForm() hostForm {
	placeholders := [numFields]string{
		"web-1", "203.0.113.10", "deploy", "22", "agent/key/password", "~/.ssh/id_ed25519",
	}
	defaults := [numFields]string{"", "", "", "22", "agent", ""}

	var fields [numFields]textinput.Model
	for i := range fields {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.SetValue(defaults[i])
		fields[i] = t
	}
	fields[0].Focus()
	return hostForm{fields: fields, focused: 0}
}

func (f hostForm) Update(msg tea.Msg) (hostForm, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		// forward to focused field
		var cmd tea.Cmd
		f.fields[f.focused], cmd = f.fields[f.focused].Update(msg)
		return f, cmd
	}

	switch key.String() {
	case "tab", "down":
		f.fields[f.focused].Blur()
		f.focused = (f.focused + 1) % numFields
		f.fields[f.focused].Focus()
		return f, textinput.Blink

	case "shift+tab", "up":
		f.fields[f.focused].Blur()
		f.focused = (f.focused - 1 + numFields) % numFields
		f.fields[f.focused].Focus()
		return f, textinput.Blink

	case "enter":
		if f.focused < numFields-1 {
			// advance to next field
			f.fields[f.focused].Blur()
			f.focused++
			f.fields[f.focused].Focus()
			return f, textinput.Blink
		}
		// last field — submit
		return f, f.submit()

	case "ctrl+s":
		return f, f.submit()

	case "esc":
		return f, func() tea.Msg { return formCancelledMsg{} }
	}

	var cmd tea.Cmd
	f.fields[f.focused], cmd = f.fields[f.focused].Update(msg)
	return f, cmd
}

func (f hostForm) submit() tea.Cmd {
	name := strings.TrimSpace(f.fields[fieldName].Value())
	hostname := strings.TrimSpace(f.fields[fieldHostname].Value())
	user := strings.TrimSpace(f.fields[fieldUser].Value())
	if name == "" || hostname == "" || user == "" {
		return nil // ignore — required fields empty
	}

	port := 22
	if p := strings.TrimSpace(f.fields[fieldPort].Value()); p != "" {
		for _, r := range p {
			if r >= '0' && r <= '9' {
				port = port*10 + int(r-'0')
			}
		}
		// reset and re-parse
		port = 0
		for _, r := range p {
			if r >= '0' && r <= '9' {
				port = port*10 + int(r-'0')
			}
		}
		if port == 0 {
			port = 22
		}
	}

	auth := core.AuthType(strings.TrimSpace(f.fields[fieldAuth].Value()))
	if auth != core.AuthPassword && auth != core.AuthKey {
		auth = core.AuthAgent
	}

	h := core.Host{
		ID:        uuid.New().String(),
		Label:     name,
		Hostname:  hostname,
		Port:      port,
		Username:  user,
		Auth:      auth,
		UpdatedAt: time.Now(),
	}

	if auth == core.AuthKey {
		kp := strings.TrimSpace(f.fields[fieldKeyPath].Value())
		if kp == "" {
			kp = "~/.ssh/id_ed25519"
		}
		h.SecretRef = kp
	}

	return func() tea.Msg { return hostSavedMsg{h} }
}

var (
	styleFormLabel    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleFormFocused  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleFormBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
)

func (f hostForm) View() string {
	var sb strings.Builder
	sb.WriteString(styleFormFocused.Render("Add host") + "\n\n")

	for i, field := range f.fields {
		label := fieldLabels[i]
		if i == f.focused {
			sb.WriteString(styleFormFocused.Render(label))
		} else {
			sb.WriteString(styleFormLabel.Render(label))
		}
		sb.WriteString(field.View() + "\n")
	}

	sb.WriteString("\n" + styleFormLabel.Render("tab next   ctrl+s save   esc cancel"))
	return styleFormBorder.Render(sb.String())
}
