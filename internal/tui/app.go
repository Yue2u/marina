package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Yue2u/marina/internal/core"
	"github.com/Yue2u/marina/internal/core/store"
)

type focusMode int

const (
	focusTree focusMode = iota
	focusSearch
	focusForm
	focusFolder
	focusShare
)

// ── Messages ──────────────────────────────────────────────────────────────────

type dataLoadedMsg struct {
	folders []core.Folder
	hosts   []core.Host
}
type errMsg struct{ err error }
type sessionEndedMsg struct{ err error }

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	st         *store.SQLiteStore
	folders    []core.Folder
	hosts      []core.Host
	nodes      []treeNode
	expanded   map[string]bool
	cursor     int
	width      int
	height     int
	err        error
	status     string
	focus      focusMode
	search     textinput.Model
	form       hostForm
	folderForm folderForm
	shareCode  string // показывается в focusShare
}

func New(st *store.SQLiteStore) model {
	search := textinput.New()
	search.Placeholder = "search..."
	search.Prompt = "/ "

	return model{
		st:       st,
		expanded: map[string]bool{},
		search:   search,
	}
}

func (m model) Init() tea.Cmd { return loadData(m.st) }

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

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case dataLoadedMsg:
		m.folders = msg.folders
		m.hosts = msg.hosts
		m.rebuildNodes()
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case sessionEndedMsg:
		m.status = ""
		if msg.err != nil {
			m.status = "disconnected: " + msg.err.Error()
		}
		return m, loadData(m.st)

	case hostSavedMsg:
		ctx := context.Background()
		if err := m.st.SaveHost(ctx, msg.host); err != nil {
			m.status = "save error: " + err.Error()
		}
		m.focus = focusTree
		return m, loadData(m.st)

	case formCancelledMsg:
		m.focus = focusTree
		return m, nil

	case folderSavedMsg:
		ctx := context.Background()
		if err := m.st.SaveFolder(ctx, msg.folder); err != nil {
			m.status = "folder save error: " + err.Error()
		}
		m.focus = focusTree
		return m, loadData(m.st)

	case folderCancelledMsg:
		m.focus = focusTree
		return m, nil
	}

	switch m.focus {
	case focusSearch:
		return m.updateSearch(msg)
	case focusForm:
		return m.updateForm(msg)
	case focusFolder:
		return m.updateFolderForm(msg)
	case focusShare:
		return m.updateShare(msg)
	default:
		return m.updateTree(msg)
	}
}

func (m model) updateTree(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
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
			m.rebuildNodes()
		}

	case "enter":
		if len(m.nodes) > 0 && m.nodes[m.cursor].kind == kindHost {
			return m, m.doConnect(m.nodes[m.cursor].id)
		}
		if len(m.nodes) > 0 && m.nodes[m.cursor].kind == kindFolder {
			id := m.nodes[m.cursor].id
			m.expanded[id] = !m.expanded[id]
			m.rebuildNodes()
		}

	case "/":
		m.focus = focusSearch
		m.search.SetValue("")
		m.search.Focus()
		return m, textinput.Blink

	case "a":
		m.focus = focusForm
		m.form = newHostForm()
		return m, textinput.Blink

	case "e":
		if len(m.nodes) > 0 && m.nodes[m.cursor].kind == kindHost {
			if h, ok := m.findHost(m.nodes[m.cursor].id); ok {
				m.focus = focusForm
				m.form = newHostFormEdit(h)
				return m, textinput.Blink
			}
		}

	case "f":
		parentID := m.currentFolderID()
		m.focus = focusFolder
		m.folderForm = newFolderForm(parentID)
		return m, textinput.Blink

	case "s":
		if len(m.nodes) > 0 && m.nodes[m.cursor].kind == kindHost {
			if h, ok := m.findHost(m.nodes[m.cursor].id); ok {
				m.shareCode = core.EncodeShare(h)
				m.focus = focusShare
			}
		}

	case "d":
		if len(m.nodes) > 0 && m.nodes[m.cursor].kind == kindHost {
			ctx := context.Background()
			m.st.DeleteHost(ctx, m.nodes[m.cursor].id)
			return m, loadData(m.st)
		}
	}
	return m, nil
}

func (m model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			m.focus = focusTree
			m.search.Blur()
			m.rebuildNodes()
			return m, nil
		case "enter":
			if len(m.nodes) > 0 && m.nodes[0].kind == kindHost {
				m.focus = focusTree
				m.search.Blur()
				return m, m.doConnect(m.nodes[0].id)
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.rebuildNodes()
	return m, cmd
}

func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

func (m model) updateFolderForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.folderForm, cmd = m.folderForm.Update(msg)
	return m, cmd
}

func (m model) updateShare(msg tea.Msg) (tea.Model, tea.Cmd) {
	// любая клавиша закрывает оверлей
	if _, ok := msg.(tea.KeyMsg); ok {
		m.focus = focusTree
		m.shareCode = ""
	}
	return m, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m *model) rebuildNodes() {
	if m.focus == focusSearch && m.search.Value() != "" {
		m.nodes = m.filteredNodes(m.search.Value())
	} else {
		m.nodes = buildTree(m.folders, m.hosts, nil, 0, m.expanded)
	}
	if m.cursor >= len(m.nodes) {
		m.cursor = max(0, len(m.nodes)-1)
	}
}

func (m model) filteredNodes(query string) []treeNode {
	q := strings.ToLower(query)
	var nodes []treeNode
	for _, h := range m.hosts {
		if strings.Contains(strings.ToLower(h.Label), q) ||
			strings.Contains(strings.ToLower(h.Hostname), q) {
			nodes = append(nodes, treeNode{kindHost, h.ID, h.Label, 0})
		}
	}
	return nodes
}

func (m model) findHost(id string) (core.Host, bool) {
	for _, h := range m.hosts {
		if h.ID == id {
			return h, true
		}
	}
	return core.Host{}, false
}

// currentFolderID возвращает ID папки под курсором (или родителя хоста), nil = корень.
func (m model) currentFolderID() *string {
	if len(m.nodes) == 0 {
		return nil
	}
	n := m.nodes[m.cursor]
	if n.kind == kindFolder {
		return &n.id
	}
	// хост — берём его FolderID
	if h, ok := m.findHost(n.id); ok {
		return h.FolderID
	}
	return nil
}

func (m model) doConnect(hostID string) tea.Cmd {
	c := exec.Command(os.Args[0], "connect", hostID)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return sessionEndedMsg{err}
	})
}

// ── View ──────────────────────────────────────────────────────────────────────

var (
	styleSelected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleFolder   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleHost     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	styleStatus   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleShareBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).
			BorderForeground(lipgloss.Color("10"))
)

func (m model) View() string {
	if m.err != nil {
		return "Error: " + m.err.Error() + "\n\nPress q to quit."
	}

	// модальные оверлеи
	switch m.focus {
	case focusForm:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.form.View())
	case focusFolder:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.folderForm.View())
	case focusShare:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.shareView())
	}

	treeW := m.width / 2
	if treeW < 28 {
		treeW = 28
	}
	detailW := m.width - treeW - 6
	panelH := m.height - 3

	left := styleBox.Width(treeW).Height(panelH).Render(m.renderTree(panelH - 2))
	right := styleBox.Width(detailW).Height(panelH).Render(m.renderDetail())
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	var bottomBar string
	if m.focus == focusSearch {
		bottomBar = m.search.View()
	} else if m.status != "" {
		bottomBar = styleStatus.Render(m.status)
	} else {
		bottomBar = styleStatus.Render(
			"j/k move   space/↵ fold   / search   a add   e edit   f folder   s share   d del   ↵ connect   q quit",
		)
	}

	return body + "\n" + bottomBar
}

func (m model) shareView() string {
	var sb strings.Builder
	sb.WriteString(styleFormFocused.Render("Share host") + "\n\n")
	sb.WriteString(styleFormLabel.Render("Отправь этот код получателю:\n\n"))
	sb.WriteString(styleAuthActive.Render(m.shareCode) + "\n\n")
	sb.WriteString(styleFormLabel.Render("Получатель: marina receive <код>\n\nНажми любую клавишу чтобы закрыть"))
	return styleShareBox.Render(sb.String())
}

func (m model) renderTree(visible int) string {
	if len(m.nodes) == 0 {
		return styleDim.Render("(empty — press 'a' to add a host)")
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
		// считаем хосты в папке
		count := 0
		for _, h := range m.hosts {
			if h.FolderID != nil && *h.FolderID == n.id {
				count++
			}
		}
		return styleFolder.Render("📁 "+n.label) + "\n" +
			styleDim.Render(fmt.Sprintf("%d host(s)", count)) + "\n\n" +
			styleDim.Render("space/↵ fold   f new subfolder")
	}

	h, ok := m.findHost(n.id)
	if !ok {
		return ""
	}

	lines := []string{
		fmt.Sprintf("host   %s", h.Addr()),
		fmt.Sprintf("user   %s", h.Username),
		fmt.Sprintf("auth   %s", h.Auth),
	}
	if h.Auth == core.AuthKey && h.SecretRef != "" {
		lines = append(lines, fmt.Sprintf("key    %s", h.SecretRef))
	}
	if len(h.Tags) > 0 {
		lines = append(lines, fmt.Sprintf("tags   %s", strings.Join(h.Tags, ", ")))
	}
	lines = append(lines, "", styleDim.Render("↵ connect   e edit   s share   d delete"))
	return strings.Join(lines, "\n")
}
