package commands

// The history browser is a two-pane bubbletea TUI:
//
//   list  ──enter──▶  viewer
//         ◀───esc────
//
// The list pane uses bubbles/list with its built-in `/` filter (which
// matches on FilterValue — we point that at the question). `f` opens a
// full-text prompt that calls history.Search under the hood and replaces
// the list items with the narrower result set. `r` restores the full set.
//
// The viewer pane is a bubbles/viewport seeded with a fully-rendered
// markdown string (via markdown.NewWithWriter → strings.Builder). Scroll
// bindings are the viewport defaults; `q`/`esc` pop back to the list.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	historypkg "github.com/ugurcan-aytar/brain/internal/history"
	"github.com/ugurcan-aytar/brain/internal/markdown"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewHistoryBrowseCmd constructs the `brain history browse` subcommand
// and is registered as a child of NewHistoryCmd.
func NewHistoryBrowseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "browse",
		Short: "Interactive picker over the saved Q&A history",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistoryBrowse()
		},
	}
}

func runHistoryBrowse() error {
	recs, err := historypkg.List()
	if err != nil {
		return err
	}
	if len(recs) == 0 {
		fmt.Println(ui.Dim.Render("  No history entries yet. Ask a question with `brain ask`."))
		return nil
	}

	m := newBrowserModel(recs)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// ── model ─────────────────────────────────────────────────────────────

type browserState int

const (
	browserStateList browserState = iota
	browserStateViewer
	browserStateFullTextPrompt
	browserStateConfirmDelete
)

type historyItem struct {
	rec historypkg.Record
}

func (i historyItem) Title() string {
	return i.rec.Question
}

func (i historyItem) Description() string {
	date := i.rec.Timestamp.Format("2006-01-02 15:04")
	parts := []string{date}
	if i.rec.Mode != "" {
		parts = append(parts, i.rec.Mode)
	}
	if i.rec.Model != "" {
		parts = append(parts, i.rec.Model)
	}
	if i.rec.Collections != "" {
		parts = append(parts, "["+i.rec.Collections+"]")
	}
	if i.rec.Elapsed != "" {
		parts = append(parts, i.rec.Elapsed)
	}
	return strings.Join(parts, "  ")
}

// FilterValue is what bubbles/list's built-in `/` filter matches against.
// Pointing it at the question gives us "search across questions" for free.
func (i historyItem) FilterValue() string { return i.rec.Question }

type browserKeyMap struct {
	View       key.Binding
	Delete     key.Binding
	FullText   key.Binding
	Reset      key.Binding
	Back       key.Binding
	ConfirmYes key.Binding
	ConfirmNo  key.Binding
}

func newBrowserKeyMap() browserKeyMap {
	return browserKeyMap{
		View: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "view"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		FullText: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "full-text search"),
		),
		Reset: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reset filter"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc", "back"),
		),
		ConfirmYes: key.NewBinding(key.WithKeys("y")),
		ConfirmNo:  key.NewBinding(key.WithKeys("n", "esc")),
	}
}

type browserModel struct {
	allRecords []historypkg.Record
	state      browserState

	list     list.Model
	viewer   viewport.Model
	ftInput  textinput.Model
	keys     browserKeyMap
	viewerOf historypkg.Record

	width  int
	height int

	status      string
	filterLabel string // e.g. "full-text: onboarding" — shown in list title
}

func newBrowserModel(recs []historypkg.Record) *browserModel {
	items := make([]list.Item, 0, len(recs))
	for _, r := range recs {
		items = append(items, historyItem{rec: r})
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("6")).
		BorderForeground(lipgloss.Color("6"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("6")).
		BorderForeground(lipgloss.Color("6")).
		Faint(true)

	l := list.New(items, delegate, 80, 20)
	l.Title = "brain history"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetStatusBarItemName("entry", "entries")
	keys := newBrowserKeyMap()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.View, keys.FullText, keys.Reset, keys.Delete}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.View, keys.FullText, keys.Reset, keys.Delete}
	}

	ft := textinput.New()
	ft.Placeholder = "substring to match in question + answer body"
	ft.Prompt = "  full-text: "
	ft.CharLimit = 120

	return &browserModel{
		allRecords: recs,
		state:      browserStateList,
		list:       l,
		viewer:     viewport.New(80, 20),
		ftInput:    ft,
		keys:       keys,
	}
}

func (m *browserModel) Init() tea.Cmd { return nil }

func (m *browserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Leave a row for the status line.
		m.list.SetSize(msg.Width, msg.Height-1)
		m.viewer.Width = msg.Width
		m.viewer.Height = msg.Height - 2
	}

	switch m.state {
	case browserStateFullTextPrompt:
		return m.updateFullTextPrompt(msg)
	case browserStateConfirmDelete:
		return m.updateConfirmDelete(msg)
	case browserStateViewer:
		return m.updateViewer(msg)
	default:
		return m.updateList(msg)
	}
}

func (m *browserModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	// While the list's own filter input is active, let the list swallow
	// every key — otherwise hitting `d` mid-filter would try to delete.
	if key, ok := msg.(tea.KeyMsg); ok && !m.list.SettingFilter() {
		switch {
		case key.String() == "ctrl+c":
			return m, tea.Quit
		case keyMatches(key, m.keys.View):
			if sel, ok := m.list.SelectedItem().(historyItem); ok {
				m.openViewer(sel.rec)
				return m, nil
			}
		case keyMatches(key, m.keys.Delete):
			if _, ok := m.list.SelectedItem().(historyItem); ok {
				m.state = browserStateConfirmDelete
				return m, nil
			}
		case keyMatches(key, m.keys.FullText):
			m.state = browserStateFullTextPrompt
			m.ftInput.SetValue("")
			m.ftInput.Focus()
			return m, textinput.Blink
		case keyMatches(key, m.keys.Reset):
			m.resetFilter()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *browserModel) updateViewer(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "q":
			m.state = browserStateList
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewer, cmd = m.viewer.Update(msg)
	return m, cmd
}

func (m *browserModel) updateFullTextPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			m.state = browserStateList
			return m, nil
		case "enter":
			query := strings.TrimSpace(m.ftInput.Value())
			m.state = browserStateList
			if query == "" {
				return m, nil
			}
			if err := m.applyFullTextFilter(query); err != nil {
				m.status = "search failed: " + err.Error()
			}
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.ftInput, cmd = m.ftInput.Update(msg)
	return m, cmd
}

func (m *browserModel) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch {
		case k.String() == "ctrl+c":
			return m, tea.Quit
		case keyMatches(k, m.keys.ConfirmYes):
			if sel, ok := m.list.SelectedItem().(historyItem); ok {
				if err := historypkg.Delete(sel.rec); err != nil {
					m.status = "delete failed: " + err.Error()
				} else {
					m.status = "deleted: " + sel.rec.Filename
					m.removeRecord(sel.rec)
				}
			}
			m.state = browserStateList
			return m, nil
		case keyMatches(k, m.keys.ConfirmNo):
			m.state = browserStateList
			return m, nil
		}
	}
	return m, nil
}

func (m *browserModel) openViewer(rec historypkg.Record) {
	content, err := historypkg.Load(rec)
	if err != nil {
		m.status = "load failed: " + err.Error()
		return
	}
	// Pre-render the markdown into a string and hand it to the viewport.
	// The renderer is line-buffered so we have to Flush for the tail.
	var buf strings.Builder
	r := markdown.NewWithWriter(&buf)
	r.Write(content)
	r.Flush()
	m.viewer.SetContent(buf.String())
	m.viewer.GotoTop()
	m.viewerOf = rec
	m.state = browserStateViewer
}

func (m *browserModel) applyFullTextFilter(query string) error {
	matches, err := historypkg.Search(query)
	if err != nil {
		return err
	}
	items := make([]list.Item, 0, len(matches))
	for _, r := range matches {
		items = append(items, historyItem{rec: r})
	}
	m.list.SetItems(items)
	m.list.ResetSelected()
	m.filterLabel = fmt.Sprintf("full-text: %s (%d)", query, len(matches))
	m.list.Title = "brain history — " + m.filterLabel
	if len(matches) == 0 {
		m.status = "no full-text matches"
	} else {
		m.status = ""
	}
	return nil
}

func (m *browserModel) resetFilter() {
	items := make([]list.Item, 0, len(m.allRecords))
	for _, r := range m.allRecords {
		items = append(items, historyItem{rec: r})
	}
	m.list.SetItems(items)
	m.list.ResetFilter()
	m.list.ResetSelected()
	m.filterLabel = ""
	m.list.Title = "brain history"
	m.status = ""
}

func (m *browserModel) removeRecord(rec historypkg.Record) {
	filtered := make([]historypkg.Record, 0, len(m.allRecords))
	for _, r := range m.allRecords {
		if r.Path != rec.Path {
			filtered = append(filtered, r)
		}
	}
	m.allRecords = filtered
	// Drop it from the list's current view too — do this after the
	// allRecords update so the surviving set is consistent if the user
	// then hits `r`.
	idx := m.list.Index()
	if idx >= 0 && idx < len(m.list.Items()) {
		m.list.RemoveItem(idx)
	}
}

func (m *browserModel) View() string {
	switch m.state {
	case browserStateFullTextPrompt:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.list.View(),
			ui.Dim.Render("  enter: apply · esc: cancel"),
			m.ftInput.View(),
		)
	case browserStateConfirmDelete:
		sel, _ := m.list.SelectedItem().(historyItem)
		prompt := ui.Yellow.Render(fmt.Sprintf("  delete \"%s\"? [y/N]", sel.rec.Filename))
		return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), prompt)
	case browserStateViewer:
		footer := ui.Dim.Render(fmt.Sprintf("  esc: back · j/k: scroll · %s", m.viewerOf.Filename))
		return lipgloss.JoinVertical(lipgloss.Left, m.viewer.View(), footer)
	default:
		view := m.list.View()
		if m.status != "" {
			view = lipgloss.JoinVertical(lipgloss.Left, view, ui.Dim.Render("  "+m.status))
		}
		return view
	}
}

// keyMatches is a short-circuit for `key.Matches` that also ignores the
// binding's enabled flag — our bindings are all enabled, but keeping the
// helper lets us swap in conditional enables later without touching every
// call site.
func keyMatches(k tea.KeyMsg, b key.Binding) bool {
	for _, bk := range b.Keys() {
		if k.String() == bk {
			return true
		}
	}
	return false
}
