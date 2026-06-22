package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStylerOffIsIdentity(t *testing.T) {
	off := styler{on: false}
	if got := off.green("x"); got != "x" {
		t.Fatalf("off.green = %q, want plain", got)
	}
	if got := off.bold("x"); got != "x" {
		t.Fatalf("off.bold = %q, want plain", got)
	}

	on := styler{on: true}
	if got := on.green("x"); got != "\x1b[32mx\x1b[0m" {
		t.Fatalf("on.green = %q", got)
	}
	if got := on.green(""); got != "" {
		t.Fatalf("empty input must stay empty, got %q", got)
	}
}

func TestFieldPreservesVisibleWidth(t *testing.T) {
	a := &app{}
	on := styler{on: true}

	if got := a.field("ok", 6, nil); got != "ok    " {
		t.Fatalf("plain field = %q", got)
	}
	// Coloring must not change the visible width: padding counts runes, not bytes.
	if got := a.field("ok", 6, on.green); got != "\x1b[32mok\x1b[0m    " {
		t.Fatalf("colored field = %q", got)
	}
	if got := a.field("toolong", 3, nil); got != "toolong" {
		t.Fatalf("over-width field must not pad negatively, got %q", got)
	}
}

func TestDurationShort(t *testing.T) {
	cases := map[time.Duration]string{
		500 * time.Millisecond:  "500ms",
		1500 * time.Millisecond: "1.5s",
		90 * time.Second:        "1m30s",
	}
	for in, want := range cases {
		if got := durationShort(in); got != want {
			t.Fatalf("durationShort(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestPhaseNonInteractiveHumanPrintsResult(t *testing.T) {
	var buf bytes.Buffer
	a := newApp("t", "c", "d", &buf, &buf)

	called := false
	err := a.phase("Doing thing", func() (string, error) {
		called = true
		return "Done thing", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("phase did not run fn")
	}
	out := buf.String()
	if !strings.Contains(out, iconArrow+" Doing thing") {
		t.Fatalf("missing start line:\n%s", out)
	}
	if !strings.Contains(out, iconOK+" Done thing") {
		t.Fatalf("missing done line:\n%s", out)
	}
}

func TestPhaseSilentForJSON(t *testing.T) {
	var buf bytes.Buffer
	a := newApp("t", "c", "d", &buf, &buf)
	a.opts.output = "json"

	called := false
	if err := a.phase("x", func() (string, error) { called = true; return "y", nil }); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("fn must still run in json mode")
	}
	if buf.Len() != 0 {
		t.Fatalf("json mode must not emit progress: %q", buf.String())
	}
}

func TestPhasePropagatesError(t *testing.T) {
	var buf bytes.Buffer
	a := newApp("t", "c", "d", &buf, &buf)

	sentinel := errors.New("boom")
	err := a.phase("x", func() (string, error) { return "", sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("phase swallowed error: %v", err)
	}
}
