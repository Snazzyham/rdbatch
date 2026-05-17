package ui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/soham/rdbatch/internal/api"
	"github.com/soham/rdbatch/internal/player"
	"github.com/soham/rdbatch/internal/search"
)

// Screen states
type screen int

const (
	screenSearch screen = iota
	screenSeasons
	screenEpisodes
	screenTorrents
)

// UI Styles
var (
	searchTitleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	searchPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	searchHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	searchErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	searchToastStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	searchFlashStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	searchSelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	searchUnselStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	searchDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	searchCursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	searchCachedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	searchUncachedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// Messages
type titleResultsMsg struct {
	results []search.TitleResult
}

type metaMsg struct {
	meta search.Meta
}

type torrentsMsg struct {
	torrents []search.TorrentResult
}

type searchErrMsg struct {
	err error
}

type toastMsg struct {
	msg string
}

type flashMsg struct {
	msg string
}

type clearMsg struct{}

// SearchModel is the Bubble Tea model for the search TUI
type SearchModel struct {
	// Clients
	cinemeta *search.Cinemeta
	comet    *search.Comet
	provider api.Provider

	// Screen state
	screen screen

	// Search state
	query         string
	titleResults  []search.TitleResult
	selectedTitle *search.TitleResult

	// Series state
	meta            search.Meta
	seasons         []int
	selectedSeason  int
	episodes        []search.Episode
	selectedEpisode *search.Episode

	// Torrents state
	torrents     []search.TorrentResult
	showUncached bool

	// UI components
	input  textinput.Model
	list   list.Model
	width  int
	height int

	// Status
	loading   bool
	errMsg    string
	toastMsg  string
	flashMsg  string
	flashTime time.Time
}

// searchItem represents an item in the search results list
type searchItem struct {
	title search.TitleResult
}

func (i searchItem) FilterValue() string { return i.title.Name }

// seasonItem represents a season in the season list
type seasonItem struct {
	season int
}

func (i seasonItem) FilterValue() string { return fmt.Sprintf("Season %d", i.season) }

// episodeItem represents an episode in the episode list
type episodeItem struct {
	episode search.Episode
}

func (i episodeItem) FilterValue() string { return i.episode.Name }

// torrentItem represents a torrent in the torrents list
type torrentItem struct {
	torrent search.TorrentResult
}

func (i torrentItem) FilterValue() string { return i.torrent.Name }

// Delegate for rendering list items
type searchItemDelegate struct{}

func (d searchItemDelegate) Height() int {
	// Return 2 to provide visual spacing between items
	return 2
}

func (d searchItemDelegate) Spacing() int                              { return 0 }
func (d searchItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d searchItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	var line string
	switch item := listItem.(type) {
	case searchItem:
		cursor := "  "
		if index == m.Index() {
			cursor = "> "
		}
		typeStr := "movie"
		if item.title.Type == "series" {
			typeStr = "series"
		}
		line = fmt.Sprintf("%s%s (%s) · %s", cursor, item.title.Name, item.title.ReleaseInfo, typeStr)
		if index == m.Index() {
			line = searchSelStyle.Render(line)
		} else {
			line = searchUnselStyle.Render(line)
		}
		fmt.Fprintln(w, line)
		fmt.Fprint(w, searchDimStyle.Render("")) // Second line for spacing
		return
	case seasonItem:
		cursor := "  "
		if index == m.Index() {
			cursor = "> "
		}
		line = fmt.Sprintf("%sSeason %d", cursor, item.season)
		if index == m.Index() {
			line = searchSelStyle.Render(line)
		} else {
			line = searchUnselStyle.Render(line)
		}
		fmt.Fprintln(w, line)
		fmt.Fprint(w, searchDimStyle.Render("")) // Second line for spacing
		return
	case episodeItem:
		cursor := "  "
		if index == m.Index() {
			cursor = "> "
		}
		line = fmt.Sprintf("%sS%02dE%02d · %s", cursor, item.episode.Season, item.episode.Episode, item.episode.Name)
		if index == m.Index() {
			line = searchSelStyle.Render(line)
		} else {
			line = searchUnselStyle.Render(line)
		}
		fmt.Fprintln(w, line)
		fmt.Fprint(w, searchDimStyle.Render("")) // Second line for spacing
		return
	case torrentItem:
		cursor := "  "
		if index == m.Index() {
			cursor = "> "
		}
		cacheIcon := "[⬇]"
		style := searchUncachedStyle
		if item.torrent.Cache == search.Cached {
			cacheIcon = "[⚡]"
			style = searchCachedStyle
		}

		// Format the description nicely - take just the first line for the main display
		desc := item.torrent.Description
		if idx := strings.Index(desc, "\n"); idx > 0 {
			desc = desc[:idx]
		}

		// Truncate if too long
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}

		line = fmt.Sprintf("%s%s %s", cursor, cacheIcon, desc)
		if index == m.Index() {
			line = searchSelStyle.Render(line)
		} else {
			line = style.Render(line)
		}
		fmt.Fprintln(w, line)

		// Second line: separator for visual spacing
		sepStyle := searchDimStyle
		if index == m.Index() {
			sepStyle = searchSelStyle
		}
		fmt.Fprint(w, sepStyle.Render("  ──────────────────────────────────────────"))
		return
	}
	fmt.Fprint(w, line)
}

// NewSearchModel creates a new search model
func NewSearchModel(cinemeta *search.Cinemeta, comet *search.Comet, provider api.Provider) SearchModel {
	// Setup input
	input := textinput.New()
	input.Placeholder = "Search for a movie or series..."
	input.Prompt = "> "
	input.Focus()
	input.CharLimit = 156
	input.Width = 50

	// Setup list
	delegate := searchItemDelegate{}
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Title = ""

	return SearchModel{
		cinemeta: cinemeta,
		comet:    comet,
		provider: provider,
		screen:   screenSearch,
		input:    input,
		list:     l,
	}
}

func (m SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 6)
		return m, nil

	case titleResultsMsg:
		m.loading = false
		m.titleResults = msg.results
		m.screen = screenSearch

		// Update list items
		items := make([]list.Item, len(m.titleResults))
		for i, r := range m.titleResults {
			items[i] = searchItem{title: r}
		}
		m.list.SetItems(items)
		m.list.Select(0)
		return m, nil

	case metaMsg:
		m.loading = false
		m.meta = msg.meta

		// Build seasons list (skip season 0 unless it's the only one)
		seasonMap := make(map[int]bool)
		for _, v := range m.meta.Videos {
			seasonMap[v.Season] = true
		}

		m.seasons = make([]int, 0, len(seasonMap))
		for s := range seasonMap {
			if s != 0 {
				m.seasons = append(m.seasons, s)
			}
		}

		// If no seasons found except 0, include it
		if len(m.seasons) == 0 && seasonMap[0] {
			m.seasons = append(m.seasons, 0)
		}

		sort.Ints(m.seasons)

		// Update list for seasons
		items := make([]list.Item, len(m.seasons))
		for i, s := range m.seasons {
			items[i] = seasonItem{season: s}
		}
		m.list.SetItems(items)
		m.list.Select(0)
		m.screen = screenSeasons
		return m, nil

	case torrentsMsg:
		m.loading = false
		m.torrents = msg.torrents

		// Filter and sort torrents
		m.updateTorrentList()
		return m, nil

	case searchErrMsg:
		m.loading = false
		m.errMsg = msg.err.Error()
		return m, nil

	case toastMsg:
		m.toastMsg = msg.msg
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearMsg{}
		})

	case flashMsg:
		m.flashMsg = msg.msg
		m.flashTime = time.Now()
		return m, tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return clearMsg{}
		})

	case clearMsg:
		m.toastMsg = ""
		m.flashMsg = ""
		return m, nil

	case tea.KeyMsg:
		// Always update input first if on search screen and focused
		if m.screen == screenSearch && m.input.Focused() {
			newInput, inputCmd := m.input.Update(msg)
			m.input = newInput
			cmds = append(cmds, inputCmd)
		}

		// Then handle special keys
		model, cmd := m.handleKeyMsg(msg)
		if cmd != nil {
			return model, tea.Batch(append(cmds, cmd)...)
		}
	}

	// Update list
	newList, cmd := m.list.Update(msg)
	m.list = newList
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *SearchModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc":
		return m.handleEsc()

	case "enter":
		return m.handleEnter()

	case "down", "j":
		// If on search screen and in input, move to list
		if m.screen == screenSearch && m.input.Focused() {
			m.input.Blur()
			return m, nil
		}

	case "up", "k":
		// If on search screen and at top of list, move to input
		if m.screen == screenSearch && !m.input.Focused() && m.list.Index() == 0 {
			m.input.Focus()
			return m, nil
		}

	case "/":
		// Focus input on search screen
		if m.screen == screenSearch {
			m.input.Focus()
			return m, nil
		}

	case "w":
		return m.handleWatch()

	case "u":
		if m.screen == screenTorrents {
			m.showUncached = !m.showUncached
			m.updateTorrentList()
			return m, nil
		}
	}

	return m, nil
}

func (m *SearchModel) handleEsc() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenSearch:
		return m, tea.Quit
	case screenSeasons:
		// Go back to search
		m.screen = screenSearch
		m.selectedTitle = nil
		items := make([]list.Item, len(m.titleResults))
		for i, r := range m.titleResults {
			items[i] = searchItem{title: r}
		}
		m.list.SetItems(items)
		m.input.Focus()
		return m, nil
	case screenEpisodes:
		// Go back to seasons
		m.screen = screenSeasons
		m.selectedSeason = 0
		items := make([]list.Item, len(m.seasons))
		for i, s := range m.seasons {
			items[i] = seasonItem{season: s}
		}
		m.list.SetItems(items)
		return m, nil
	case screenTorrents:
		// Go back to episodes (TV) or search (movie)
		if m.selectedTitle.Type == "series" {
			m.screen = screenEpisodes
			items := make([]list.Item, len(m.episodes))
			for i, e := range m.episodes {
				items[i] = episodeItem{episode: e}
			}
			m.list.SetItems(items)
		} else {
			m.screen = screenSearch
			items := make([]list.Item, len(m.titleResults))
			for i, r := range m.titleResults {
				items[i] = searchItem{title: r}
			}
			m.list.SetItems(items)
			m.input.Focus()
		}
		return m, nil
	}
	return m, nil
}

func (m *SearchModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenSearch:
		if m.input.Focused() {
			// Execute search
			query := m.input.Value()
			if query == "" {
				return m, nil
			}
			m.loading = true
			m.errMsg = ""
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), cinemetaTimeout)
				defer cancel()
				results, err := m.cinemeta.Search(ctx, query)
				if err != nil {
					return searchErrMsg{err: err}
				}
				return titleResultsMsg{results: results}
			}
		}

		// Select a title
		idx := m.list.Index()
		if idx < 0 || idx >= len(m.titleResults) {
			return m, nil
		}

		m.selectedTitle = &m.titleResults[idx]

		if m.selectedTitle.Type == "movie" {
			// Fetch torrents directly
			m.loading = true
			return m, m.fetchTorrents(m.selectedTitle.ID, "", "")
		} else {
			// Fetch series meta first
			m.loading = true
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), cinemetaTimeout)
				defer cancel()
				meta, err := m.cinemeta.Meta(ctx, m.selectedTitle.ID)
				if err != nil {
					return searchErrMsg{err: err}
				}
				return metaMsg{meta: meta}
			}
		}

	case screenSeasons:
		idx := m.list.Index()
		if idx < 0 || idx >= len(m.seasons) {
			return m, nil
		}
		m.selectedSeason = m.seasons[idx]

		// Build episodes list for this season
		m.episodes = make([]search.Episode, 0)
		for _, v := range m.meta.Videos {
			if v.Season == m.selectedSeason {
				m.episodes = append(m.episodes, search.Episode{
					ID:      v.ID,
					Name:    v.Name,
					Season:  v.Season,
					Episode: v.Episode,
				})
			}
		}

		// Sort by episode number
		sort.Slice(m.episodes, func(i, j int) bool {
			return m.episodes[i].Episode < m.episodes[j].Episode
		})

		items := make([]list.Item, len(m.episodes))
		for i, e := range m.episodes {
			items[i] = episodeItem{episode: e}
		}
		m.list.SetItems(items)
		m.list.Select(0)
		m.screen = screenEpisodes
		return m, nil

	case screenEpisodes:
		idx := m.list.Index()
		if idx < 0 || idx >= len(m.episodes) {
			return m, nil
		}
		m.selectedEpisode = &m.episodes[idx]

		// Fetch torrents for this episode
		m.loading = true
		mediaID := fmt.Sprintf("%s:%d:%d", m.selectedTitle.ID, m.selectedEpisode.Season, m.selectedEpisode.Episode)
		return m, m.fetchTorrents(mediaID, "", "")

	case screenTorrents:
		idx := m.list.Index()
		visibleTorrents := m.getVisibleTorrents()
		if idx < 0 || idx >= len(visibleTorrents) {
			return m, nil
		}

		torrent := visibleTorrents[idx]

		// Reconstruct magnet link and add to provider
		magnet := fmt.Sprintf("magnet:?xt=urn:btih:%s", torrent.InfoHash)

		return m, func() tea.Msg {
			_, name, status, err := m.provider.AddMagnet(magnet)
			if err != nil {
				return searchErrMsg{err: fmt.Errorf("failed to add magnet: %w", err)}
			}
			return toastMsg{msg: fmt.Sprintf("added · %s (%s)", name, status)}
		}
	}

	return m, nil
}

func (m *SearchModel) handleWatch() (tea.Model, tea.Cmd) {
	if m.screen != screenTorrents {
		return m, nil
	}

	idx := m.list.Index()
	visibleTorrents := m.getVisibleTorrents()
	if idx < 0 || idx >= len(visibleTorrents) {
		return m, nil
	}

	torrent := visibleTorrents[idx]

	// Only allow watching cached torrents
	if torrent.Cache != search.Cached {
		return m, func() tea.Msg {
			return flashMsg{msg: "W disabled — torrent is not cached"}
		}
	}

	// Play the stream
	return m, func() tea.Msg {
		cmd, _, err := player.GetPlayerCmd(torrent.StreamURL)
		if err != nil {
			return searchErrMsg{err: err}
		}
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return searchErrMsg{err: err}
			}
			return nil
		})()
	}
}

func (m *SearchModel) fetchTorrents(mediaID, season, episode string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var mediaType string
		if m.selectedTitle.Type == "movie" {
			mediaType = "movie"
		} else {
			mediaType = "series"
		}

		torrents, err := m.comet.Streams(ctx, mediaType, mediaID)
		if err != nil {
			return searchErrMsg{err: err}
		}

		return torrentsMsg{torrents: torrents}
	}
}

func (m *SearchModel) updateTorrentList() {
	visible := m.getVisibleTorrents()
	items := make([]list.Item, len(visible))
	for i, t := range visible {
		items[i] = torrentItem{torrent: t}
	}
	m.list.SetItems(items)
	m.list.Select(0)
	m.screen = screenTorrents
}

func (m *SearchModel) getVisibleTorrents() []search.TorrentResult {
	var visible []search.TorrentResult
	for _, t := range m.torrents {
		if t.Cache == search.Cached || m.showUncached {
			visible = append(visible, t)
		}
	}

	// Sort: cached first, then by size descending
	sort.Slice(visible, func(i, j int) bool {
		if visible[i].Cache != visible[j].Cache {
			return visible[i].Cache == search.Cached
		}
		return visible[i].SizeBytes > visible[j].SizeBytes
	})

	return visible
}

const cinemetaTimeout = 10 * time.Second

func (m SearchModel) View() string {
	if m.loading {
		return m.renderLoading()
	}

	var b strings.Builder

	// Render screen-specific content
	switch m.screen {
	case screenSearch:
		b.WriteString(m.renderSearchScreen())
	case screenSeasons:
		b.WriteString(m.renderSeasonsScreen())
	case screenEpisodes:
		b.WriteString(m.renderEpisodesScreen())
	case screenTorrents:
		b.WriteString(m.renderTorrentsScreen())
	}

	// Render messages
	if m.errMsg != "" {
		b.WriteString("\n" + searchErrorStyle.Render(m.errMsg))
	}
	if m.toastMsg != "" {
		b.WriteString("\n" + searchToastStyle.Render(m.toastMsg))
	}
	if m.flashMsg != "" && time.Since(m.flashTime) < time.Second {
		b.WriteString("\n" + searchFlashStyle.Render(m.flashMsg))
	}

	return b.String()
}

func (m SearchModel) renderLoading() string {
	return "\n\n" + dimStyle.Render("    Loading...")
}

func (m SearchModel) renderSearchScreen() string {
	var b strings.Builder

	b.WriteString(searchTitleStyle.Render("Search") + "\n")
	b.WriteString(m.input.View() + "\n\n")

	if len(m.titleResults) == 0 && m.query != "" {
		b.WriteString(dimStyle.Render("  no results — try a different query") + "\n")
	} else if len(m.titleResults) > 0 {
		b.WriteString(m.list.View() + "\n")
	}

	// Help text
	if m.input.Focused() {
		b.WriteString(searchHelpStyle.Render("  [Enter to search · Esc quit]"))
	} else {
		b.WriteString(searchHelpStyle.Render("  [↑/↓ navigate · Enter view torrents · / focus search · Esc quit]"))
	}

	return b.String()
}

func (m SearchModel) renderSeasonsScreen() string {
	var b strings.Builder

	if m.selectedTitle != nil {
		b.WriteString(searchTitleStyle.Render(m.selectedTitle.Name) + "\n")
	}
	b.WriteString(searchPromptStyle.Render("Select Season") + "\n")
	b.WriteString(m.list.View() + "\n")
	b.WriteString(searchHelpStyle.Render("  [↑/↓ navigate · Enter pick · Esc back]"))

	return b.String()
}

func (m SearchModel) renderEpisodesScreen() string {
	var b strings.Builder

	if m.selectedTitle != nil {
		b.WriteString(searchTitleStyle.Render(fmt.Sprintf("%s · Season %d", m.selectedTitle.Name, m.selectedSeason)) + "\n")
	}
	b.WriteString(searchPromptStyle.Render("Select Episode") + "\n")
	b.WriteString(m.list.View() + "\n")
	b.WriteString(searchHelpStyle.Render("  [↑/↓ navigate · Enter view torrents · Esc back]"))

	return b.String()
}

func (m SearchModel) renderTorrentsScreen() string {
	var b strings.Builder

	// Title
	if m.selectedTitle != nil {
		if m.selectedTitle.Type == "series" && m.selectedEpisode != nil {
			b.WriteString(searchTitleStyle.Render(fmt.Sprintf("%s · S%02dE%02d", m.selectedTitle.Name, m.selectedEpisode.Season, m.selectedEpisode.Episode)) + "\n")
		} else {
			b.WriteString(searchTitleStyle.Render(m.selectedTitle.Name) + "\n")
		}
	}

	// Toggle indicator
	if m.showUncached {
		b.WriteString(dimStyle.Render("  [showing all torrents]") + "\n")
	} else {
		b.WriteString(dimStyle.Render("  [showing cached only — press U to toggle]") + "\n")
	}

	b.WriteString("\n")

	// List
	if len(m.getVisibleTorrents()) == 0 {
		b.WriteString(dimStyle.Render("  no torrents found for this title") + "\n")
	} else {
		b.WriteString(m.list.View() + "\n")
	}

	// Help
	b.WriteString(searchHelpStyle.Render("  [Enter add · W stream · U toggle uncached · Esc back]"))

	return b.String()
}

// RunSearch starts the search TUI
func RunSearch(cinemeta *search.Cinemeta, comet *search.Comet, provider api.Provider) error {
	m := NewSearchModel(cinemeta, comet, provider)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
