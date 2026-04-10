package markdown

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain forces lipgloss into TrueColor mode so the tests can assert on
// ANSI escape sequences. Without this, writing to a bytes.Buffer (not a TTY)
// causes lipgloss to strip all styling, and any test that checks for "\x1b["
// fails with a false negative.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

// render is a tiny helper that returns the rendered (ANSI-stripped) output
// for a single markdown input. ANSI stripping lets us assert on content
// without pinning exact style escapes — style regressions should be caught
// by explicit ANSI-present tests below.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func render(t *testing.T, md string) string {
	t.Helper()
	var buf bytes.Buffer
	r := NewWithWriter(&buf)
	r.Write(md)
	r.Flush()
	return buf.String()
}

func renderPlain(t *testing.T, md string) string {
	return stripANSI(render(t, md))
}

func TestH1Rendered(t *testing.T) {
	out := renderPlain(t, "# Hello\n")
	if !strings.Contains(out, "Hello") {
		t.Errorf("expected H1 content, got %q", out)
	}
}

func TestH2AndH3Rendered(t *testing.T) {
	out := renderPlain(t, "## Section\n### Subsection\n")
	if !strings.Contains(out, "Section") || !strings.Contains(out, "Subsection") {
		t.Errorf("expected both headers, got %q", out)
	}
}

func TestParagraphIndented(t *testing.T) {
	out := renderPlain(t, "just a paragraph\n")
	if !strings.Contains(out, "just a paragraph") {
		t.Errorf("paragraph missing: %q", out)
	}
}

func TestCodeBlockRendered(t *testing.T) {
	md := "```go\nfmt.Println(\"hi\")\n```\n"
	out := renderPlain(t, md)
	if !strings.Contains(out, `fmt.Println("hi")`) {
		t.Errorf("code block content missing: %q", out)
	}
	if !strings.Contains(out, "go") {
		t.Errorf("code block lang label missing: %q", out)
	}
}

func TestUnorderedList(t *testing.T) {
	out := renderPlain(t, "- first\n- second\n- third\n")
	for _, s := range []string{"first", "second", "third", "•"} {
		if !strings.Contains(out, s) {
			t.Errorf("list item %q missing from %q", s, out)
		}
	}
}

func TestOrderedList(t *testing.T) {
	out := renderPlain(t, "1. alpha\n2. beta\n3. gamma\n")
	for _, s := range []string{"1.", "alpha", "2.", "beta", "3.", "gamma"} {
		if !strings.Contains(out, s) {
			t.Errorf("ordered list element %q missing from %q", s, out)
		}
	}
}

func TestBlockquote(t *testing.T) {
	out := renderPlain(t, "> quoted text here\n")
	if !strings.Contains(out, "quoted text here") {
		t.Errorf("blockquote content missing: %q", out)
	}
	if !strings.Contains(out, "▎") {
		t.Errorf("blockquote marker missing: %q", out)
	}
}

func TestHorizontalRule(t *testing.T) {
	out := renderPlain(t, "---\n")
	if !strings.Contains(out, "─") {
		t.Errorf("horizontal rule not rendered: %q", out)
	}
}

func TestInlineCodeRendered(t *testing.T) {
	// With ANSI stripped the backticks should be gone but the content kept.
	out := renderPlain(t, "use `brain index` to rebuild\n")
	if !strings.Contains(out, "brain index") {
		t.Errorf("inline code content missing: %q", out)
	}
	if strings.Contains(out, "`") {
		t.Errorf("backticks should be stripped: %q", out)
	}
}

func TestBoldRendered(t *testing.T) {
	raw := render(t, "this is **bold** text\n")
	plain := stripANSI(raw)
	if !strings.Contains(plain, "bold") {
		t.Errorf("bold content missing: %q", plain)
	}
	if strings.Contains(plain, "**") {
		t.Errorf("bold markers should be stripped: %q", plain)
	}
	// The raw output should contain an ANSI bold escape.
	if !strings.Contains(raw, "\x1b[") {
		t.Errorf("expected ANSI styling in raw output")
	}
}

func TestCitationRecognized(t *testing.T) {
	raw := render(t, "See [notes.md] for details\n")
	if !strings.Contains(raw, "notes.md") {
		t.Errorf("citation content missing")
	}
	// Styled citation has ANSI color escapes around it.
	if !strings.Contains(raw, "\x1b[") {
		t.Errorf("expected citation to be colored")
	}
}

func TestMultilineStreamingBuffered(t *testing.T) {
	// Simulate a streamed response: split across chunks mid-line.
	var buf bytes.Buffer
	r := NewWithWriter(&buf)

	r.Write("# Head")
	// At this point, the line is incomplete — nothing should be rendered.
	if buf.Len() != 0 {
		t.Errorf("expected nothing rendered for partial line, got %q", buf.String())
	}
	r.Write("er\n")
	if !strings.Contains(buf.String(), "Header") {
		t.Errorf("expected Header to be rendered after newline, got %q", buf.String())
	}

	// Partial paragraph followed by flush.
	r.Write("trailing part")
	before := buf.Len()
	r.Flush()
	if buf.Len() == before {
		t.Errorf("Flush should render the trailing partial line")
	}
	if !strings.Contains(buf.String(), "trailing part") {
		t.Errorf("flushed content missing: %q", buf.String())
	}
}

func TestCollapseRunsOfBlankLines(t *testing.T) {
	// Multiple blank lines should collapse to a single blank.
	var buf bytes.Buffer
	r := NewWithWriter(&buf)
	r.Write("para1\n\n\n\npara2\n")
	r.Flush()

	plain := stripANSI(buf.String())

	// Count blank lines between "para1" and "para2".
	between := strings.SplitN(strings.SplitN(plain, "para1", 2)[1], "para2", 2)[0]
	blankCount := strings.Count(between, "\n") - 1 // subtract the trailing newline after para1
	if blankCount > 2 {
		t.Errorf("expected collapsed blank lines, got %d blank lines between paragraphs: %q", blankCount, between)
	}
}

func TestFlushOnEmpty(t *testing.T) {
	// Flushing a pristine renderer shouldn't panic.
	var buf bytes.Buffer
	r := NewWithWriter(&buf)
	r.Flush()
}

func TestCodeBlockTogglesState(t *testing.T) {
	// Two separate code blocks in a row should toggle in/out correctly.
	md := "```\nblock1\n```\n\n```\nblock2\n```\n"
	out := renderPlain(t, md)
	if !strings.Contains(out, "block1") || !strings.Contains(out, "block2") {
		t.Errorf("two code blocks not both rendered: %q", out)
	}
}
