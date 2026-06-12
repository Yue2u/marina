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

// Fixed field indices (always visible). fldAuth is a selector, not a textinput.
const (
	fldName     = 0
	fldHostname = 1
	fldUser     = 2
	fldPort     = 3
	fldAuth     = 4 // selector
	fldSecret   = 5 // password or key path — only when auth != agent
	numInputs   = 4 // textinputs before the selector
)

var authCycle = []core.AuthType{core.AuthAgent, core.AuthKey, core.AuthPassword}

type hostForm struct {
	inputs  [numInputs]textinput.Model // name, hostname, user, port
	authIdx int                        // index into authCycle
	secret  textinput.Model            // key path or password
	focused int                        // 0..fieldCount()-1
}

type hostSavedMsg struct{ host core.Host }
type formCancelledMsg struct{}

func newHostForm() hostForm {
	placeholders := [numInputs]string{"web-1", "203.0.113.10", "deploy", "22"}

	var inputs [numInputs]textinput.Model
	for i := range inputs {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		inputs[i] = t
	}
	inputs[fldPort].SetValue("22")
	inputs[0].Focus()

	secret := textinput.New()
	secret.Placeholder = "~/.ssh/id_ed25519"

	return hostForm{inputs: inputs, authIdx: 0, secret: secret}
}

func (f hostForm) selectedAuth() core.AuthType { return authCycle[f.authIdx] }
func (f hostForm) hasSecret() bool             { return f.selectedAuth() != core.AuthAgent }

func (f hostForm) fieldCount() int {
	if f.hasSecret() {
		return fldSecret + 1
	}
	return fldAuth + 1
}

// ── Update ───────────────────────────────────────────────────────────────────

func (f hostForm) Update(msg tea.Msg) (hostForm, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return f.forwardToActive(msg)
	}

	switch key.String() {
	case "esc":
		return f, func() tea.Msg { return formCancelledMsg{} }

	case "ctrl+s":
		return f, f.submit()

	case "left", "right":
		if f.focused == fldAuth {
			n := len(authCycle)
			if key.String() == "right" {
				f.authIdx = (f.authIdx + 1) % n
			} else {
				f.authIdx = (f.authIdx - 1 + n) % n
			}
			f.refreshSecret()
			// if we moved to agent while on secret field — jump back to selector
			if !f.hasSecret() && f.focused >= fldSecret {
				f.focused = fldAuth
			}
			return f, nil
		}

	case "tab", "down":
		f.blurActive()
		f.focused = (f.focused + 1) % f.fieldCount()
		f.focusActive()
		return f, textinput.Blink

	case "shift+tab", "up":
		f.blurActive()
		f.focused = (f.focused - 1 + f.fieldCount()) % f.fieldCount()
		f.focusActive()
		return f, textinput.Blink

	case "enter":
		if f.focused == f.fieldCount()-1 {
			return f, f.submit()
		}
		f.blurActive()
		f.focused++
		f.focusActive()
		return f, textinput.Blink
	}

	return f.forwardToActive(msg)
}

func (f hostForm) forwardToActive(msg tea.Msg) (hostForm, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case f.focused < numInputs:
		f.inputs[f.focused], cmd = f.inputs[f.focused].Update(msg)
	case f.focused == fldSecret:
		f.secret, cmd = f.secret.Update(msg)
	}
	return f, cmd
}

func (f *hostForm) blurActive() {
	if f.focused < numInputs {
		f.inputs[f.focused].Blur()
	} else if f.focused == fldSecret {
		f.secret.Blur()
	}
}

func (f *hostForm) focusActive() {
	if f.focused < numInputs {
		f.inputs[f.focused].Focus()
	} else if f.focused == fldSecret {
		f.secret.Focus()
	}
}

func (f *hostForm) refreshSecret() {
	f.secret.SetValue("")
	switch f.selectedAuth() {
	case core.AuthKey:
		f.secret.Placeholder = "~/.ssh/id_ed25519"
		f.secret.EchoMode = textinput.EchoNormal
	case core.AuthPassword:
		f.secret.Placeholder = "password"
		f.secret.EchoMode = textinput.EchoPassword
	}
}

// ── Submit ───────────────────────────────────────────────────────────────────

func (f hostForm) submit() tea.Cmd {
	name := strings.TrimSpace(f.inputs[fldName].Value())
	hostname := strings.TrimSpace(f.inputs[fldHostname].Value())
	user := strings.TrimSpace(f.inputs[fldUser].Value())
	if name == "" || hostname == "" || user == "" {
		return nil
	}

	port := 22
	if p := strings.TrimSpace(f.inputs[fldPort].Value()); p != "" {
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

	auth := f.selectedAuth()
	h := core.Host{
		ID:        uuid.New().String(),
		Label:     name,
		Hostname:  hostname,
		Port:      port,
		Username:  user,
		Auth:      auth,
		UpdatedAt: time.Now(),
	}

	if f.hasSecret() {
		sec := strings.TrimSpace(f.secret.Value())
		if auth == core.AuthKey && sec == "" {
			sec = "~/.ssh/id_ed25519"
		}
		h.SecretRef = sec
	}

	return func() tea.Msg { return hostSavedMsg{h} }
}

// ── View ─────────────────────────────────────────────────────────────────────

var (
	styleFormLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleFormFocused = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleFormBorder  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	styleAuthActive  = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
)

var inputLabels = [numInputs]string{
	"name      ",
	"hostname  ",
	"user      ",
	"port      ",
}

func (f hostForm) View() string {
	var sb strings.Builder
	sb.WriteString(styleFormFocused.Render("Add host") + "\n\n")

	// Static text inputs
	for i := 0; i < numInputs; i++ {
		f.renderLabel(&sb, inputLabels[i], i == f.focused)
		sb.WriteString(f.inputs[i].View() + "\n")
	}

	// Auth selector
	f.renderLabel(&sb, "auth      ", f.focused == fldAuth)
	sb.WriteString(f.authSelectorView() + "\n")

	// Conditional secret field
	if f.hasSecret() {
		var label string
		if f.selectedAuth() == core.AuthKey {
			label = "key path  "
		} else {
			label = "password  "
		}
		f.renderLabel(&sb, label, f.focused == fldSecret)
		sb.WriteString(f.secret.View() + "\n")
	}

	sb.WriteString("\n" + styleFormLabel.Render("tab/↑↓ move   ←/→ auth   ctrl+s save   esc cancel"))
	return styleFormBorder.Render(sb.String())
}

func (f hostForm) renderLabel(sb *strings.Builder, label string, active bool) {
	if active {
		sb.WriteString(styleFormFocused.Render(label))
	} else {
		sb.WriteString(styleFormLabel.Render(label))
	}
}

func (f hostForm) authSelectorView() string {
	var parts []string
	for i, opt := range authCycle {
		s := string(opt)
		if i == f.authIdx {
			parts = append(parts, styleAuthActive.Render(s))
		} else {
			parts = append(parts, styleFormLabel.Render(s))
		}
	}
	body := strings.Join(parts, styleFormLabel.Render(" / "))
	if f.focused == fldAuth {
		return styleFormFocused.Render("< ") + body + styleFormFocused.Render(" >")
	}
	return "  " + body + "  "
}
