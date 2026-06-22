package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 90 * time.Millisecond

// phase runs a long operation with progress feedback.
//
//   - json/quiet: silent, just runs fn.
//   - interactive terminal: an animated spinner that resolves to ✓/✗ with the
//     elapsed time.
//   - piped/verbose human output: a single start line, plus a ✓ line when fn
//     returns a closing message.
//
// fn returns an optional success message; when empty, the label is reused.
func (a *app) phase(label string, fn func() (string, error)) error {
	if a.opts.output != "human" || a.opts.quiet {
		_, err := fn()
		return err
	}

	if !a.interactive() {
		s := a.styler()
		fmt.Fprintf(a.opts.out, "%s %s\n", s.cyan(iconArrow), label)
		done, err := fn()
		if err == nil && done != "" {
			fmt.Fprintf(a.opts.out, "  %s %s\n", s.green(iconOK), done)
		}
		return err
	}

	sp := a.newSpinner(label)
	sp.start()
	done, err := fn()
	final := label
	if done != "" {
		final = done
	}
	if errors.Is(err, context.Canceled) {
		sp.clear()
	} else {
		sp.finish(err == nil, final)
	}
	return err
}

type spinner struct {
	out     io.Writer
	style   styler
	label   string
	stop    chan struct{}
	done    chan struct{}
	started time.Time
}

func (a *app) newSpinner(label string) *spinner {
	return &spinner{out: a.opts.out, style: a.styler(), label: label}
}

func (s *spinner) start() {
	s.started = time.Now()
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.loop()
}

func (s *spinner) loop() {
	defer close(s.done)
	ticker := time.NewTicker(spinnerInterval)
	defer ticker.Stop()
	i := 0
	s.frame(i)
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			i++
			s.frame(i)
		}
	}
}

func (s *spinner) frame(i int) {
	frame := spinnerFrames[i%len(spinnerFrames)]
	fmt.Fprintf(s.out, "\r\x1b[2K%s %s", s.style.cyan(frame), s.label)
}

func (s *spinner) finish(ok bool, final string) {
	close(s.stop)
	<-s.done
	icon := s.style.green(iconOK)
	if !ok {
		icon = s.style.red(iconFail)
	}
	elapsed := s.style.dim("(" + durationShort(time.Since(s.started)) + ")")
	fmt.Fprintf(s.out, "\r\x1b[2K%s %s  %s\n", icon, final, elapsed)
}

func (s *spinner) clear() {
	close(s.stop)
	<-s.done
	fmt.Fprint(s.out, "\r\x1b[2K")
}
