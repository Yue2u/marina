package cli

import (
	"github.com/Yue2u/marina/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
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
