package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Yue2u/marina/internal/core"
	"github.com/Yue2u/marina/internal/core/store"
)

// Messages
type dataLoadedMsg struct {
	folders []core.Folder
	hosts   []core.Host
}
type errMsg struct{ err error }
type sessionEndedMsg struct{ err error }

type model struct {
	st       *store.SQLiteStore
	folders  []core.Folder
	hosts    []core.Host
	nodes    []treeNode
	expanded map[string]bool
	cursor   int
	width    int
	height   int
	err      error
	status   string
}

func New(st *store.SQLiteStore) model {
	return model{
		st:       st,
		expanded: map[string]bool{},
	}
}

func (m model) Init() tea.Cmd {
	return loadData(m.st)
}

func loadData(st *store.SQLiteStore) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		folders, err := st.Folders(ctx)
		if err != nil {
			return errMsg{err}
		}
		hosts, err := st.Hosts(ctx, nil)
		if err != nil {
			return errMsg{err}
		}
		return dataLoadedMsg{folders, hosts}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case dataLoadedMsg:
		m.folders = msg.folders
		m.hosts = msg.hosts
		m.nodes = buildTree(m.folders, m.hosts, nil, 0, m.expanded)
		if m.cursor >= len(m.nodes) {
			m.cursor = max(0, len(m.nodes)-1)
		}

	case errMsg:
		m.err = msg.err

	case sessionEndedMsg:
		m.status = ""
		if msg.err != nil {
			m.status = "disconnected: " + msg.err.Error()
		}
		return m, loadData(m.st)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.nodes)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case " ":
			if len(m.nodes) > 0 && m.nodes[m.cursor].kind == kindFolder {
				id := m.nodes[m.cursor].id
				m.expanded[id] = !m.expanded[id]
				m.nodes = buildTree(m.folders, m.hosts, nil, 0, m.expanded)
			}
		case "enter":
			if len(m.nodes) > 0 && m.nodes[m.cursor].kind == kindHost {
				return m, m.doConnect(m.nodes[m.cursor].id)
			}
		}
	}
	return m, nil
}

func (m model) doConnect(hostID string) tea.Cmd {
	c := exec.Command(os.Args[0], "connect", hostID)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return sessionEndedMsg{err}
	})
}

// --- Styles ---

var (
	styleSelected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleFolder   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleHost     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	styleStatus   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func (m model) View() string {
	if m.err != nil {
		return "Error: " + m.err.Error() + "\n\nPress q to quit."
	}

	treeW := m.width / 2
	if treeW < 28 {
		treeW = 28
	}
	detailW := m.width - treeW - 6 // account for two borders + padding

	panelH := m.height - 3

	left := styleBox.Width(treeW).Height(panelH).Render(m.renderTree(panelH - 2))
	right := styleBox.Width(detailW).Height(panelH).Render(m.renderDetail())

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	status := m.status
	if status == "" {
		status = "j/k move   space fold   ↵ connect   q quit"
	}
	return body + "\n" + styleStatus.Render(status)
}

func (m model) renderTree(visible int) string {
	if len(m.nodes) == 0 {
		return styleDim.Render("(empty — use 'marina add')")
	}

	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}

	var lines []string
	for i := start; i < len(m.nodes) && i < start+visible; i++ {
		n := m.nodes[i]
		indent := strings.Repeat("  ", n.depth)

		var text string
		if n.kind == kindFolder {
			arrow := "▸ "
			if m.expanded[n.id] {
				arrow = "▾ "
			}
			text = arrow + n.label
		} else {
			text = "● " + n.label
		}

		var line string
		if i == m.cursor {
			line = indent + styleSelected.Render(text)
		} else if n.kind == kindFolder {
			line = indent + styleFolder.Render(text)
		} else {
			line = indent + styleHost.Render(text)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m model) renderDetail() string {
	if len(m.nodes) == 0 || m.cursor >= len(m.nodes) {
		return ""
	}
	n := m.nodes[m.cursor]
	if n.kind == kindFolder {
		return styleFolder.Render("[folder]") + "\n" + n.label
	}

	var h core.Host
	for _, host := range m.hosts {
		if host.ID == n.id {
			h = host
			break
		}
	}

	lines := []string{
		fmt.Sprintf("host   %s", h.Addr()),
		fmt.Sprintf("user   %s", h.Username),
		fmt.Sprintf("auth   %s", h.Auth),
	}
	if len(h.Tags) > 0 {
		lines = append(lines, fmt.Sprintf("tags   %s", strings.Join(h.Tags, ", ")))
	}
	lines = append(lines, "", styleDim.Render("↵ connect"))
	return strings.Join(lines, "\n")
}
