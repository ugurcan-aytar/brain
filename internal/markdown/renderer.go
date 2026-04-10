// Package markdown provides a streaming terminal renderer that lights up
// markdown as it arrives from the LLM — no need to buffer the entire response.
//
// Lines are only rendered once a trailing newline is seen; a partial tail is
// held until the next write or flush, so styled lines never split mid-word.
package markdown

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleDim          = lipgloss.NewStyle().Faint(true)
	styleBold         = lipgloss.NewStyle().Bold(true)
	styleItalic       = lipgloss.NewStyle().Italic(true)
	styleBoldItalic   = lipgloss.NewStyle().Bold(true).Italic(true)
	styleCyan         = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleBlue         = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	styleDimItalic    = lipgloss.NewStyle().Faint(true).Italic(true)
	styleH1           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	styleH2           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleH3           = lipgloss.NewStyle().Bold(true)
	styleH4           = lipgloss.NewStyle().Bold(true).Faint(true)
)

var (
	reInlineCode      = regexp.MustCompile("`([^`]+)`")
	reBoldItalic      = regexp.MustCompile(`\*{3}([^*]+)\*{3}`)
	reBold            = regexp.MustCompile(`\*{2}([^*]+)\*{2}`)
	reItalic          = regexp.MustCompile(`(^|[^*])\*([^*\s][^*]*?[^*\s]|[^*\s])\*([^*]|$)`)
	reFileCitation    = regexp.MustCompile(`\[([^\]]+\.(?:txt|md))\]`)
	reNamedCitation   = regexp.MustCompile(`\[([A-Z][A-Za-z0-9_\- ]+)\]`)
	reTableSeparator  = regexp.MustCompile(`^\|[\s\-:|]+\|$`)
	reHorizontalRule  = regexp.MustCompile(`^(-{3,}|\*{3,})$`)
	reH1              = regexp.MustCompile(`^# (.+)`)
	reH2              = regexp.MustCompile(`^## (.+)`)
	reH3              = regexp.MustCompile(`^### (.+)`)
	reH4              = regexp.MustCompile(`^#{4,} (.+)`)
	reUnorderedList   = regexp.MustCompile(`^(\s*)[-*] (.+)`)
	reOrderedList     = regexp.MustCompile(`^(\s*)(\d+)\. (.+)`)
)

// Renderer is a line-buffered terminal markdown renderer. It is NOT safe for
// concurrent use — one renderer per stream.
type Renderer struct {
	out               io.Writer
	buffer            string
	inCodeBlock       bool
	codeLang          string
	inTable           bool
	lastLineWasEmpty  bool
}

func New() *Renderer {
	return &Renderer{out: os.Stdout}
}

func NewWithWriter(w io.Writer) *Renderer {
	return &Renderer{out: w}
}

// Write accepts a chunk of streamed text, splits it into complete lines, and
// renders each line immediately. Any trailing partial line is held in the
// buffer until the next Write or Flush.
func (r *Renderer) Write(chunk string) {
	r.buffer += chunk
	for {
		idx := strings.Index(r.buffer, "\n")
		if idx == -1 {
			return
		}
		line := r.buffer[:idx]
		r.buffer = r.buffer[idx+1:]
		r.renderLine(line)
	}
}

// Flush renders any buffered partial line and writes a trailing newline so
// the cursor lands on a fresh row.
func (r *Renderer) Flush() {
	if r.buffer != "" {
		r.renderLine(r.buffer)
		r.buffer = ""
	}
	fmt.Fprintln(r.out)
}

func (r *Renderer) renderLine(line string) {
	trimmedStart := strings.TrimLeft(line, " \t")
	trimmed := strings.TrimSpace(line)

	// Code block fence
	if strings.HasPrefix(trimmedStart, "```") {
		if !r.inCodeBlock {
			r.inCodeBlock = true
			r.codeLang = strings.TrimSpace(strings.TrimPrefix(trimmedStart, "```"))
			top := "  ┌─"
			if r.codeLang != "" {
				top += " " + r.codeLang + " "
			}
			dashes := 40 - len(r.codeLang)
			if dashes < 0 {
				dashes = 0
			}
			top += strings.Repeat("─", dashes)
			fmt.Fprintln(r.out, styleDim.Render(top))
		} else {
			r.inCodeBlock = false
			r.codeLang = ""
			fmt.Fprintln(r.out, styleDim.Render("  └─"+strings.Repeat("─", 40)))
		}
		return
	}

	// Inside code block — raw, dimmed border + cyan body.
	if r.inCodeBlock {
		fmt.Fprintln(r.out, styleDim.Render("  │ ")+styleCyan.Render(line))
		return
	}

	// Tables
	if strings.HasPrefix(trimmed, "|") {
		if reTableSeparator.MatchString(trimmed) {
			cols := 0
			for _, seg := range strings.Split(trimmed, "|") {
				if seg != "" {
					cols++
				}
			}
			fmt.Fprintln(r.out, styleDim.Render("  "+strings.Repeat("─", cols*20)))
			r.inTable = true
			return
		}

		cellsRaw := strings.Split(trimmed, "|")
		var cells []string
		for _, c := range cellsRaw {
			if c != "" {
				cells = append(cells, strings.TrimSpace(c))
			}
		}
		if len(cells) == 0 {
			return
		}
		sep := styleDim.Render("  │  ")
		if !r.inTable {
			rendered := make([]string, len(cells))
			for i, c := range cells {
				rendered[i] = styleBold.Render(r.formatInline(c))
			}
			fmt.Fprintln(r.out, "  "+strings.Join(rendered, sep))
			r.inTable = true
		} else {
			rendered := make([]string, len(cells))
			for i, c := range cells {
				rendered[i] = r.formatInline(c)
			}
			fmt.Fprintln(r.out, "  "+strings.Join(rendered, sep))
		}
		return
	}
	r.inTable = false

	// Empty line — collapse runs of blank lines to one.
	if trimmed == "" {
		if !r.lastLineWasEmpty {
			fmt.Fprintln(r.out)
			r.lastLineWasEmpty = true
		}
		return
	}
	r.lastLineWasEmpty = false

	// Horizontal rule
	if reHorizontalRule.MatchString(trimmed) {
		fmt.Fprintln(r.out, styleDim.Render("  "+strings.Repeat("─", 50)))
		return
	}

	// Headers — check longest prefix first so `## x` doesn't match `# x`.
	if m := reH1.FindStringSubmatch(line); m != nil {
		fmt.Fprintln(r.out, "\n"+styleH1.Render("  "+r.formatInline(m[1])))
		return
	}
	if m := reH2.FindStringSubmatch(line); m != nil {
		fmt.Fprintln(r.out, "\n"+styleH2.Render("  "+r.formatInline(m[1])))
		return
	}
	if m := reH3.FindStringSubmatch(line); m != nil {
		fmt.Fprintln(r.out, styleH3.Render("  "+r.formatInline(m[1])))
		return
	}
	if m := reH4.FindStringSubmatch(line); m != nil {
		fmt.Fprintln(r.out, styleH4.Render("  "+r.formatInline(m[1])))
		return
	}

	// Blockquote
	if strings.HasPrefix(trimmedStart, "> ") {
		content := strings.TrimPrefix(trimmedStart, "> ")
		fmt.Fprintln(r.out, styleDim.Render("  ▎ ")+styleDimItalic.Render(r.formatInline(content)))
		return
	}

	// Unordered list
	if m := reUnorderedList.FindStringSubmatch(line); m != nil {
		indent := len(m[1]) / 2
		pad := "  " + strings.Repeat("  ", indent)
		fmt.Fprintln(r.out, pad+styleDim.Render("•")+" "+r.formatInline(m[2]))
		return
	}

	// Ordered list
	if m := reOrderedList.FindStringSubmatch(line); m != nil {
		indent := len(m[1]) / 2
		pad := "  " + strings.Repeat("  ", indent)
		fmt.Fprintln(r.out, pad+styleDim.Render(m[2]+".")+" "+r.formatInline(m[3]))
		return
	}

	// Regular paragraph text
	fmt.Fprintln(r.out, "  "+r.formatInline(line))
}

// formatInline applies bold/italic/code/citation styling to a single line.
// Inline code is handled first so backticks don't swallow emphasis markers.
func (r *Renderer) formatInline(text string) string {
	text = reInlineCode.ReplaceAllStringFunc(text, func(match string) string {
		inner := match[1 : len(match)-1]
		return styleCyan.Render(inner)
	})
	text = reBoldItalic.ReplaceAllStringFunc(text, func(match string) string {
		inner := match[3 : len(match)-3]
		return styleBoldItalic.Render(inner)
	})
	text = reBold.ReplaceAllStringFunc(text, func(match string) string {
		inner := match[2 : len(match)-2]
		return styleBold.Render(inner)
	})
	text = reItalic.ReplaceAllStringFunc(text, func(match string) string {
		// Reconstruct the match preserving leading/trailing non-* capture.
		sub := reItalic.FindStringSubmatch(match)
		if len(sub) != 4 {
			return match
		}
		return sub[1] + styleItalic.Render(sub[2]) + sub[3]
	})
	text = reFileCitation.ReplaceAllStringFunc(text, func(match string) string {
		return styleBlue.Render(match)
	})
	text = reNamedCitation.ReplaceAllStringFunc(text, func(match string) string {
		return styleCyan.Render(match)
	})
	text = strings.ReplaceAll(text, "→", styleDim.Render("→"))
	return text
}
