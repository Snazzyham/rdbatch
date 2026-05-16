package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/soham/rdbatch/internal/api"
	"github.com/soham/rdbatch/internal/player"
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

type viewMode int

const (
	modeTorrents viewMode = iota
	modeFiles
)

type Selection struct {
	TorrentID string
	FileIDs   []string // empty means all files
	Name      string   // for logging/display
}

type item struct {
	torrent  api.Torrent
	file     api.File
	isFile   bool
	selected bool
}

func (i item) FilterValue() string {
	if i.isFile {
		return i.file.Name
	}
	return i.torrent.Name
}

type Model struct {
	list           list.Model
	items          []item
	selected       map[int]bool
	quitting       bool
	mode           viewMode
	activeTorrent  *api.Torrent
	provider       api.Provider
	torrents       []api.Torrent
	finalSelection []Selection
	err            error
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

	var name, status, size, added string
	if it.isFile {
		name = it.file.Name
		status = "" // files don't have separate status in this view
		size = formatSize(it.file.Size)
		added = ""
	} else {
		t := it.torrent
		name = t.Name
		status = statusColor(t.Status).Render(t.Status)
		size = formatSize(t.Size)
		added = t.Added.Format("2006-01-02")
	}

	if len(name) > 45 {
		name = name[:42] + "..."
	}

	// Cursor indicator
	cursor := "  "
	if index == m.Index() {
		cursor = "> "
	}

	var line string
	if it.isFile {
		line = fmt.Sprintf("%s%s %s | %s", cursor, checkbox, name, size)
	} else {
		line = fmt.Sprintf("%s%s %s | %s | %s | %s", cursor, checkbox, name, status, size, added)
	}

	if it.selected {
		line = selectedStyle.Render(line)
	} else if index == m.Index() {
		line = unselectedStyle.Render(line)
	} else {
		line = dimStyle.Render(line)
	}

	fmt.Fprint(w, line)
}

func New(p api.Provider, torrents []api.Torrent) Model {
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
		provider: p,
		torrents: torrents,
		mode:     modeTorrents,
	}
}

type filesMsg struct {
	files []api.File
	t     api.Torrent
}
type watchMsg struct {
	url string
}
type errMsg struct{ err error }

func (m Model) fetchFiles(t api.Torrent) tea.Cmd {
	return func() tea.Msg {
		files, err := m.provider.ListFiles(t.ID)
		if err != nil {
			return errMsg{err}
		}
		return filesMsg{files, t}
	}
}

func (m Model) fetchStreamLink(torrentID, fileID string) tea.Cmd {
	return func() tea.Msg {
		url, err := m.provider.GetStreamLink(torrentID, fileID)
		if err != nil {
			return errMsg{err}
		}
		return watchMsg{url}
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

	case filesMsg:
		m.mode = modeFiles
		m.activeTorrent = &msg.t
		m.items = make([]item, len(msg.files))
		var listItems []list.Item
		for i, f := range msg.files {
			m.items[i] = item{file: f, isFile: true}
			listItems = append(listItems, m.items[i])
		}
		m.list.SetItems(listItems)
		m.list.Title = fmt.Sprintf("Files: %s", msg.t.Name)
		m.list.Select(0)
		m.selected = make(map[int]bool)
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case watchMsg:
		c, _, err := player.GetPlayerCmd(msg.url)
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return errMsg{err}
			}
			return nil
		})

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "f":
			if m.mode == modeTorrents {
				idx := m.list.Index()
				if idx >= 0 && idx < len(m.items) {
					t := m.items[idx].torrent
					return m, m.fetchFiles(t)
				}
			}

		case "w":
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.items) {
				if m.mode == modeTorrents {
					t := m.items[idx].torrent
					return m, m.fetchStreamLink(t.ID, "")
				} else {
					f := m.items[idx].file
					return m, m.fetchStreamLink(m.activeTorrent.ID, f.ID)
				}
			}

		case "esc", "backspace":
			if m.mode == modeFiles {
				m.mode = modeTorrents
				m.activeTorrent = nil
				m.items = make([]item, len(m.torrents))
				var listItems []list.Item
				for i, t := range m.torrents {
					m.items[i] = item{torrent: t}
					listItems = append(listItems, m.items[i])
				}
				m.list.SetItems(listItems)
				m.list.Title = "Torrents"
				m.list.Select(0)
				m.selected = make(map[int]bool)
				return m, nil
			}

		case " ":
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.items) {
				m.items[idx].selected = !m.items[idx].selected
				m.selected[idx] = m.items[idx].selected
			}
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
			if m.mode == modeFiles {
				var fileIDs []string
				for i, it := range m.items {
					if m.selected[i] {
						fileIDs = append(fileIDs, it.file.ID)
					}
				}
				// If none selected but enter pressed, we could either do nothing or download the hovered one.
				// For now, if none selected, we do nothing to avoid accidental downloads of huge torrents.
				if len(fileIDs) > 0 {
					m.finalSelection = []Selection{{
						TorrentID: m.activeTorrent.ID,
						FileIDs:   fileIDs,
						Name:      m.activeTorrent.Name,
					}}
					m.quitting = true
					return m, tea.Quit
				}
			} else {
				// Torrents mode
				for i, it := range m.items {
					if m.selected[i] {
						m.finalSelection = append(m.finalSelection, Selection{
							TorrentID: it.torrent.ID,
							Name:      it.torrent.Name,
						})
					}
				}
				if len(m.finalSelection) > 0 {
					m.quitting = true
					return m, tea.Quit
				}
			}
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
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress Q to quit", m.err)
	}

	var help string
	if m.mode == modeTorrents {
		help = helpStyle.Render("↑/k up  ↓/j down  SPACE toggle  F files  W watch  A all  N none  ENTER download  Q quit")
	} else {
		help = helpStyle.Render("↑/k up  ↓/j down  SPACE toggle  ESC back  W watch  A all  N none  ENTER download  Q quit")
	}
	return m.list.View() + "\n" + help
}

func Run(p api.Provider, torrents []api.Torrent) ([]Selection, error) {
	m := New(p, torrents)
	pg := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := pg.Run()
	if err != nil {
		return nil, err
	}

	model := finalModel.(Model)
	return model.finalSelection, nil
}
