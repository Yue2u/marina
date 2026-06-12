package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/Yue2u/marina/internal/core"
)

type folderForm struct {
	input    textinput.Model
	parentID *string // nil = корень
}

type folderSavedMsg struct{ folder core.Folder }
type folderCancelledMsg struct{}

func newFolderForm(parentID *string) folderForm {
	t := textinput.New()
	t.Placeholder = "folder name"
	t.Focus()
	return folderForm{input: t, parentID: parentID}
}

func (f folderForm) Update(msg tea.Msg) (folderForm, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return f, cmd
	}
	switch key.String() {
	case "esc":
		return f, func() tea.Msg { return folderCancelledMsg{} }
	case "enter", "ctrl+s":
		name := strings.TrimSpace(f.input.Value())
		if name == "" {
			return f, nil
		}
		folder := core.Folder{
			ID:       uuid.New().String(),
			ParentID: f.parentID,
			Name:     name,
			Order:    int(time.Now().Unix()),
		}
		return f, func() tea.Msg { return folderSavedMsg{folder} }
	}
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	return f, cmd
}

func (f folderForm) View() string {
	var sb strings.Builder
	sb.WriteString(styleFormFocused.Render("New folder") + "\n\n")
	sb.WriteString(styleFormLabel.Render("name      "))
	sb.WriteString(f.input.View() + "\n")
	sb.WriteString("\n" + styleFormLabel.Render("enter save   esc cancel"))
	return styleFormBorder.Render(sb.String())
}
