package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/Yue2u/marina/internal/tui"
)

func launchTUI() error {
	s, err := openStore()
	if err != nil {
		return err
	}
	p := tea.NewProgram(tui.New(s), tea.WithAltScreen())
	_, err = p.Run()
	s.Close()
	return err
}
