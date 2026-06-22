package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// Icons are used in both interactive and piped output; they carry no ANSI color
// so they stay readable when colors are disabled.
const (
	iconOK    = "✓"
	iconWarn  = "⚠"
	iconFail  = "✗"
	iconArrow = "›"
	iconDot   = "•"
)

// styler wraps text in ANSI escapes. When off, every method is the identity, so
// callers can colorize unconditionally and non-terminal output stays plain.
type styler struct {
	on bool
}

func (s styler) paint(code string, text string) string {
	if !s.on || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (s styler) green(t string) string  { return s.paint("32", t) }
func (s styler) red(t string) string    { return s.paint("31", t) }
func (s styler) yellow(t string) string { return s.paint("33", t) }
func (s styler) cyan(t string) string   { return s.paint("36", t) }
func (s styler) blue(t string) string   { return s.paint("34", t) }
func (s styler) gray(t string) string   { return s.paint("90", t) }
func (s styler) dim(t string) string    { return s.paint("2", t) }
func (s styler) bold(t string) string   { return s.paint("1", t) }
func (s styler) ok(t string) string     { return s.paint("1;32", t) }
func (s styler) bad(t string) string    { return s.paint("1;31", t) }

func (a *app) resolveTTYColor() {
	a.colorOnce.Do(func() {
		a.ttyVal = isTerminalWriter(a.opts.out)
		a.colorVal = a.ttyVal &&
			a.opts.output == "human" &&
			!a.opts.noColor &&
			os.Getenv("NO_COLOR") == ""
	})
}

func (a *app) colorOn() bool {
	a.resolveTTYColor()
	return a.colorVal
}

func (a *app) styler() styler {
	return styler{on: a.colorOn()}
}

// interactive reports whether stdout can host an animated spinner. Verbose mode
// streams tool output to the same writer, so spinners are disabled there.
func (a *app) interactive() bool {
	a.resolveTTYColor()
	return a.ttyVal && a.opts.output == "human" && !a.opts.quiet && !a.opts.verbose
}

// field pads text to a visible width and optionally colors it. Padding is based
// on the plain rune count, so alignment is preserved even with ANSI escapes.
func (a *app) field(text string, width int, fn func(string) string) string {
	pad := width - len([]rune(text))
	if pad < 0 {
		pad = 0
	}
	if fn != nil {
		text = fn(text)
	}
	return text + strings.Repeat(" ", pad)
}

func (a *app) stateColorFn(state string) func(string) string {
	s := a.styler()
	switch state {
	case "ready", "running":
		return s.green
	case "waiting", "stopped":
		return s.yellow
	case "disabled":
		return s.gray
	default:
		return s.dim
	}
}

func (a *app) colorState(state string) string {
	return a.stateColorFn(state)(state)
}

func (a *app) colorPodState(state string, ready bool) string {
	s := a.styler()
	if ready {
		return s.green(state)
	}
	return s.yellow(state)
}

func isTerminalWriter(w io.Writer) bool {
	type fdWriter interface{ Fd() uintptr }
	f, ok := w.(fdWriter)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func stderrColor(opts options) bool {
	return opts.output != "json" &&
		!opts.noColor &&
		os.Getenv("NO_COLOR") == "" &&
		isTerminalWriter(opts.err)
}

func durationShort(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
}
