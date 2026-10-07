package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/soham/rdbatch/internal/download"
)

type downloadUpdate []download.Progress
type downloadDone struct{ err error }
type downloadModel struct {
	rows     []download.Progress
	viewport viewport.Model
	cancel   context.CancelFunc
	done     bool
	err      error
}

func (m downloadModel) Init() tea.Cmd { return nil }
func (m downloadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = max(1, msg.Height-5)
	case downloadUpdate:
		m.rows = []download.Progress(msg)
	case downloadDone:
		m.done = true
		m.err = msg.err
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit
		case "enter":
			if m.done {
				return m, tea.Quit
			}
		}
	}
	offset := m.viewport.YOffset
	m.viewport.SetContent(m.content())
	m.viewport.SetYOffset(offset)
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}
func bytesLabel(n int64) string { return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024)) }
func (m downloadModel) content() string {
	var b strings.Builder
	for _, r := range m.rows {
		fraction := float64(0)
		if r.Total > 0 {
			fraction = min(1, float64(r.Completed)/float64(r.Total))
		}
		if r.Status == "completed" {
			fraction = 1
		}
		filled := int(fraction * 24)
		eta := ""
		if r.Speed > 0 && r.Total > r.Completed {
			eta = " ETA " + (time.Duration((r.Total-r.Completed)/r.Speed) * time.Second).String()
		}
		fmt.Fprintf(&b, "%s\n[%s%s] %3.0f%%  %s / %s  %s/s%s  %s\n", r.Name, strings.Repeat("=", filled), strings.Repeat(" ", 24-filled), fraction*100, bytesLabel(r.Completed), bytesLabel(r.Total), bytesLabel(r.Speed), eta, r.Status)
		if r.Error != "" {
			fmt.Fprintf(&b, "  Error: %s\n", r.Error)
		}
		if r.Notice != "" {
			fmt.Fprintf(&b, "  %s\n", r.Notice)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
func (m downloadModel) View() string {
	footer := "Up/down: scroll | q: cancel downloads"
	if m.done {
		footer = "Downloads finished. Enter or q: close"
		if m.err != nil {
			footer = m.err.Error() + "\n" + footer
		}
	}
	return "Downloads\n\n" + m.viewport.View() + "\n" + footer + "\n"
}

func RunDownloads(ctx context.Context, manager *download.Manager, items []download.Item, dir string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	model := downloadModel{cancel: cancel, viewport: viewport.New(80, 20)}
	for _, item := range items {
		model.rows = append(model.rows, download.Progress{Name: item.Name, Status: "queued"})
	}
	model.viewport.SetContent(model.content())
	program := tea.NewProgram(model, tea.WithAltScreen())
	result := make(chan error, 1)
	go func() {
		err := manager.Download(ctx, items, dir, func(rows []download.Progress) { program.Send(downloadUpdate(rows)) })
		result <- err
		program.Send(downloadDone{err})
	}()
	final, err := program.Run()
	cancel()
	downloadErr := <-result
	if err != nil {
		return err
	}
	if finished, ok := final.(downloadModel); ok {
		fmt.Print(finished.content())
	}
	return downloadErr
}
