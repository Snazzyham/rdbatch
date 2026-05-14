package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/soham/rdbatch/internal/api"
)

var (
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	unselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusReady     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	statusWaiting   = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	statusError     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	headerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cursorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	selectedCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

type item struct {
	torrent  api.Torrent
	selected bool
}

func (i item) FilterValue() string { return i.torrent.Name }

type Model struct {
	list     list.Model
	items    []item
	selected map[int]bool
	quitting bool
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func statusColor(status string) lipgloss.Style {
	switch status {
	case "downloaded", "magnet_conversion", "cached", "completed":
		return statusReady
	case "waiting_files_selection", "queued", "downloading", "uploading":
		return statusWaiting
	case "error", "magnet_error", "virus", "dead", "stalled":
		return statusError
	case "paused":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	}
}

// Custom delegate so we control rendering per item
type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	it, ok := listItem.(item)
	if !ok {
		return
	}

	checkbox := "[ ]"
	if it.selected {
		checkbox = "[x]"
	}

	t := it.torrent
	name := t.Name
	if len(name) > 45 {
		name = name[:42] + "..."
	}

	status := statusColor(t.Status).Render(t.Status)
	size := formatSize(t.Size)
	added := t.Added.Format("2006-01-02")

	// Cursor indicator
	cursor := "  "
	if index == m.Index() {
		cursor = "> "
	}

	line := fmt.Sprintf("%s%s %s | %s | %s | %s", cursor, checkbox, name, status, size, added)

	if it.selected {
		line = selectedStyle.Render(line)
	} else if index == m.Index() {
		line = unselectedStyle.Render(line)
	} else {
		line = dimStyle.Render(line)
	}

	fmt.Fprint(w, line)
}

func New(torrents []api.Torrent) Model {
	items := make([]item, len(torrents))
	for i, t := range torrents {
		items[i] = item{torrent: t}
	}

	var listItems []list.Item
	for _, it := range items {
		listItems = append(listItems, it)
	}

	delegate := itemDelegate{}
	l := list.New(listItems, delegate, 0, 0)
	l.Title = "Torrents"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = headerStyle

	return Model{
		list:     l,
		items:    items,
		selected: make(map[int]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 3)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case " ":
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.items) {
				m.items[idx].selected = !m.items[idx].selected
				m.selected[idx] = m.items[idx].selected
			}
			// Update the list item reference so delegate re-renders
			m.list.SetItem(idx, m.items[idx])
			return m, nil

		case "a":
			for i := range m.items {
				m.items[i].selected = true
				m.selected[i] = true
				m.list.SetItem(i, m.items[i])
			}
			return m, nil

		case "n":
			for i := range m.items {
				m.items[i].selected = false
				delete(m.selected, i)
				m.list.SetItem(i, m.items[i])
			}
			return m, nil

		case "enter":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	help := helpStyle.Render("↑/k up  ↓/j down  SPACE toggle  A select all  N select none  ENTER download  Q quit")
	return m.list.View() + "\n" + help
}

func (m Model) SelectedTorrents() []api.Torrent {
	var selected []api.Torrent
	for i, it := range m.items {
		if m.selected[i] {
			selected = append(selected, it.torrent)
		}
	}
	return selected
}

func Run(torrents []api.Torrent) ([]api.Torrent, error) {
	m := New(torrents)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	model := finalModel.(Model)
	return model.SelectedTorrents(), nil
}
