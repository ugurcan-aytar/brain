package commands

// One-shot bubbletea input component for the chat REPL. Replaces the
// chzyer/readline call that used to drive the prompt.
//
// Two things this fixes over readline:
//   - live slash-command filter dropdown: as soon as the user types `/`,
//     a ranked list of matching commands appears below the input; arrow
//     keys navigate, Enter confirms the highlighted one (or whatever is
//     literally in the input box)
//   - terminal state survives idle periods: bubbletea re-asserts the TTY
//     on every render cycle so readline's "typed chars vanish after a few
//     minutes idle" bug goes away
//
// Every call to readChatInput runs a fresh bubbletea program — there is
// no persistent state between prompts, so the chat loop stays in charge
// of history, Ctrl+C double-tap exit, etc.

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ugurcan-aytar/brain/internal/ui"
)

// slashCommand is a single entry in the slash-command catalog the chat
// REPL exposes via `/help`, the live dropdown, and unique-prefix matching.
type slashCommand struct {
	name string
	help string
}

// chatInputResult describes how a single readChatInput call ended. The
// chat loop translates this into the same branches readline's ErrInterrupt
// / io.EOF paths used to handle.
//
// IMPORTANT: chatInputPending must be the zero value. The model's initial
// state is pending (nothing submitted yet), and View() keys off the
// submitted state to decide whether to render the input line or a blank
// string (the blank is how we hide the input after Enter so the chat loop
// can re-print the question cleanly). If chatInputSubmitted were the zero
// value, the initial View() would render blank and the user would type
// into invisible darkness until Enter finally flipped the state.
type chatInputResult int

const (
	chatInputPending chatInputResult = iota // zero value — model hasn't finished yet
	chatInputSubmitted
	chatInputInterrupted // Ctrl+C pressed
	chatInputEOF         // Ctrl+D on an empty line
)

var inputBorder = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("8")).
	Padding(0, 1)

var inputBorderActive = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("6")).
	Padding(0, 1)

type chatInputModel struct {
	input    textinput.Model
	commands []slashCommand
	filtered []slashCommand
	cursor   int
	result   chatInputResult
	value    string
	width    int
}

func newChatInputModel(commands []slashCommand) chatInputModel {
	ti := textinput.New()
	ti.Prompt = ui.Cyan.Render("❯ ")
	ti.Placeholder = "Ask a question about your notes, or type / for commands"
	ti.PlaceholderStyle = lipgloss.NewStyle().Faint(true)
	ti.Focus()
	ti.CharLimit = 0
	ti.Width = 80
	return chatInputModel{input: ti, commands: commands, width: 80}
}

func (m chatInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m chatInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		innerW := ws.Width - 4 // border + padding
		if innerW < 20 {
			innerW = 20
		}
		m.input.Width = innerW
	}

	var cmd tea.Cmd
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyCtrlC:
			m.result = chatInputInterrupted
			return m, tea.Quit
		case tea.KeyCtrlD:
			if m.input.Value() == "" {
				m.result = chatInputEOF
				return m, tea.Quit
			}
		case tea.KeyEnter:
			// If the dropdown is showing, Enter picks the highlighted
			// suggestion (that's the autocomplete affordance). Otherwise
			// it submits whatever is in the text box verbatim.
			if m.dropdownVisible() && m.cursor < len(m.filtered) {
				m.value = m.filtered[m.cursor].name
			} else {
				m.value = m.input.Value()
			}
			m.result = chatInputSubmitted
			return m, tea.Quit
		case tea.KeyUp:
			if m.dropdownVisible() && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.dropdownVisible() && m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case tea.KeyTab:
			// Tab completes the input to the currently highlighted
			// suggestion, keeping the prompt open so the user can add
			// arguments (e.g. `/mode recall`).
			if m.dropdownVisible() && len(m.filtered) > 0 {
				m.input.SetValue(m.filtered[m.cursor].name + " ")
				m.input.CursorEnd()
			}
			return m, nil
		}
	}
	m.input, cmd = m.input.Update(msg)
	m.filtered = filterSlashCommands(m.commands, m.input.Value())
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
	return m, cmd
}

// dropdownVisible is true when the live slash-command filter should be
// rendered below the input. We only show it while the user is typing the
// command name — once they add arguments (space in the input), the user
// is past the pick step and the dropdown would be a distraction.
func (m chatInputModel) dropdownVisible() bool {
	v := m.input.Value()
	return strings.HasPrefix(v, "/") && !strings.Contains(v, " ") && len(m.filtered) > 0
}

func (m chatInputModel) View() string {
	if m.result == chatInputSubmitted {
		return ""
	}

	borderW := m.width - 2
	if borderW < 20 {
		borderW = 20
	}

	border := inputBorder.Width(borderW)
	if m.input.Value() != "" {
		border = inputBorderActive.Width(borderW)
	}

	var b strings.Builder
	b.WriteString(border.Render(m.input.View()))

	if !m.dropdownVisible() {
		return b.String()
	}
	b.WriteString("\n")
	cursorGlyph := ui.Cyan.Render("▸")
	hl := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	for i, c := range m.filtered {
		if i == m.cursor {
			b.WriteString("  " + cursorGlyph + " " + hl.Render(c.name) + "  " + ui.Dim.Render(c.help))
		} else {
			b.WriteString("    " + ui.Dim.Render(c.name+"  "+c.help))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// filterSlashCommands returns the subset of commands whose name has the
// current input as a prefix. A non-slash input or one with spaces returns
// an empty slice so the dropdown stays hidden while the user is typing
// ordinary questions or adding slash-command arguments.
func filterSlashCommands(commands []slashCommand, input string) []slashCommand {
	if !strings.HasPrefix(input, "/") || strings.Contains(input, " ") {
		return nil
	}
	prefix := strings.ToLower(input)
	var out []slashCommand
	for _, c := range commands {
		if strings.HasPrefix(c.name, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// readChatInput runs a single bubbletea textinput session and returns the
// captured text, how the session ended (submitted / interrupted / EOF),
// and any framework-level error. The caller is expected to handle the
// double-tap Ctrl+C exit pattern and any other REPL-level state — this
// function is intentionally stateless between calls.
func readChatInput(commands []slashCommand) (string, chatInputResult, error) {
	m := newChatInputModel(commands)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return "", chatInputInterrupted, err
	}
	fm, ok := final.(chatInputModel)
	if !ok {
		return "", chatInputInterrupted, nil
	}
	return fm.value, fm.result, nil
}
