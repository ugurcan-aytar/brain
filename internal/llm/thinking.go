package llm

// Live progress indicator for the "extended thinking" phase that Claude
// opus (and sonnet with thinking enabled) goes through before producing
// actual response tokens. Both the Anthropic SDK and the claude CLI
// emit thinking_delta events during this phase. Without this indicator
// the terminal looks frozen for anywhere between 10 seconds and several
// minutes while the model reasons internally — see chat.go's
// formatElapsed and the user's 229-second silence bug.
//
// The indicator rewrites a single dim line via \r every ~500ms:
//
//     💭 thinking… 42s
//
// When the model transitions from thinking to real output, Finalize()
// clears the live line and prints a one-line summary:
//
//     ✓ thought for 3m 49s
//
// Then the normal streaming begins on a fresh line. If no thinking
// tokens ever arrive (e.g., haiku, which doesn't do extended thinking),
// nothing is ever printed — the indicator stays dormant.

import (
	"fmt"
	"time"
)

const (
	thinkingUpdateInterval = 500 * time.Millisecond
	thinkingHint           = "\033[2m  💭 thinking… "
	thinkingSummaryDone    = "\033[2m  ✓ thought for "
	ansiReset              = "\033[0m"
	ansiClearLine          = "\r\033[K"
)

type thinkingIndicator struct {
	started   time.Time
	lastPrint time.Time
	active    bool
}

// Note records a thinking_delta arrival. Kicks off the timer on the
// first call. Throttles visible updates to at most one every
// thinkingUpdateInterval so rapid delta events don't churn the
// terminal.
func (t *thinkingIndicator) Note() {
	if t.started.IsZero() {
		t.started = time.Now()
	}
	now := time.Now()
	if t.active && now.Sub(t.lastPrint) < thinkingUpdateInterval {
		return
	}
	t.lastPrint = now
	line := thinkingHint + formatThinkingElapsed(time.Since(t.started)) + ansiReset
	if t.active {
		// Overwrite the previous indicator line in place.
		fmt.Print(ansiClearLine)
	}
	t.active = true
	fmt.Print(line)
}

// Finalize transitions from the thinking indicator to real output.
// Safe to call multiple times — only the first call does anything,
// so streamViaCLI/streamViaSDK can call it defensively from both the
// "first text_delta arrived" path and the "scan loop ended" path.
func (t *thinkingIndicator) Finalize() {
	if !t.active {
		return
	}
	t.active = false
	fmt.Print(ansiClearLine)
	fmt.Println(thinkingSummaryDone + formatThinkingElapsed(time.Since(t.started)) + ansiReset)
	fmt.Println()
}

// formatThinkingElapsed renders a duration in seconds for <60s and
// minutes+seconds beyond that. Deliberately mirrors the chat loop's
// formatElapsed helper — duplicating the 6 lines is cheaper than a
// shared-helper package for a single formatter.
func formatThinkingElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	mins := int(d / time.Minute)
	secs := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%ds", mins, secs)
}
