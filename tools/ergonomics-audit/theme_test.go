package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunThemeGatesOnFrameCacheReads covers runTheme end-to-end: the
// gating contract that make check-all relies on. Unmarked reads fail
// the audit; the marker defers them; clean code passes.
func TestRunThemeGatesOnFrameCacheReads(t *testing.T) {
	repo := t.TempDir()
	guiDir := filepath.Join(repo, "gui")
	if err := os.MkdirAll(guiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scrollPath := filepath.Join(guiDir, "scroll.go")

	write := func(src string) {
		t.Helper()
		if err := os.WriteFile(scrollPath, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Unmarked frame-cache read in a scanned path: gates.
	write(`package gui

var guiTheme struct{ scrollMultiplier float32 }

func scrollVertical(w *Window, d float32) float32 {
	return d * guiTheme.scrollMultiplier
}
`)
	if err := runTheme([]string{repo}); err == nil {
		t.Fatal("runTheme passed with an unmarked frame-cache read")
	}

	// Same line marked: advisory only, passes.
	write(`package gui

var guiTheme struct{ scrollMultiplier float32 }

func scrollVertical(w *Window, d float32) float32 {
	return d * guiTheme.scrollMultiplier // ergonomics-audit:theme-global
}
`)
	if err := runTheme([]string{repo}); err != nil {
		t.Fatalf("runTheme failed with a marked read: %v", err)
	}

	// Window named: passes.
	write(`package gui

type Theme struct{ scrollMultiplier float32 }

func scrollVertical(w *Window, d float32) float32 {
	return d * w.Theme().scrollMultiplier
}
`)
	if err := runTheme([]string{repo}); err != nil {
		t.Fatalf("runTheme failed on clean code: %v", err)
	}
}

// scanThemeSrc runs the theme-mode rules over one in-memory file and
// returns the reported findings as "fn:line:read", with a "[deferred]"
// suffix when the line carried the marker.
func scanThemeSrc(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	marked := markedLines(fset, f, themeGlobalMarker)
	var out []string
	inspectTheme(fset, f, func(fn string, line int, read string) {
		s := fn + ":" + read
		if marked[line] {
			s += " [deferred]"
		}
		out = append(out, s)
	})
	return out
}

func TestThemeFlagsFrameCacheReadsWithWindowInHand(t *testing.T) {
	t.Parallel()
	const src = `package p

type Window struct{}
type Theme struct{ ColorBackground int }

func CurrentTheme() Theme { return Theme{} }

var guiTheme Theme
var defaultToastStyle struct{ maxVisible int }
var DefaultScrollbarStyle struct{ Size int }

// Parameter form.
func handler(w *Window, delta float32) {
	_ = guiTheme.ColorBackground
}

// Receiver form.
func (w *Window) export() {
	_ = CurrentTheme().ColorBackground
}

// Qualified call, backend style.
func render(w *gui.Window) {
	_ = gui.CurrentTheme().ColorBackground
}

// Style mirrors, both spellings.
func enforce(w *Window) {
	_ = defaultToastStyle.maxVisible
	_ = DefaultScrollbarStyle.Size
}
`
	got := scanThemeSrc(t, src)
	want := []string{
		"handler:guiTheme",
		"export:CurrentTheme()",
		"render:gui.CurrentTheme()",
		"enforce:defaultToastStyle",
		"enforce:DefaultScrollbarStyle",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestThemeIgnoresFunctionsWithoutAWindow(t *testing.T) {
	t.Parallel()
	// A factory has no window to name — the bare read is the design,
	// and it is what makes Themed() scoping work.
	const src = `package p

type Theme struct{ ColorBackground int }

var guiTheme Theme
var defaultButtonStyle struct{ Radius int }

func Button(cfg int) int {
	_ = guiTheme.ColorBackground
	return defaultButtonStyle.Radius
}

func (b *Backend) draw(r int) {
	_ = guiTheme.ColorBackground
}
`
	if got := scanThemeSrc(t, src); len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}

func TestThemeMarkerDefersFinding(t *testing.T) {
	t.Parallel()
	const src = `package p

type Window struct{}
type Theme struct{ ColorBackground int }

var guiTheme Theme

func handler(w *Window) {
	_ = guiTheme.ColorBackground // ergonomics-audit:theme-global
}
`
	got := scanThemeSrc(t, src)
	want := []string{"handler:guiTheme [deferred]"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestThemeScannedPaths(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"gui/scroll.go":                        true,
		"gui/scroll_smooth.go":                 true,
		"gui/event_handlers.go":                true,
		"gui/native_print.go":                  true,
		"gui/window_update.go":                 true,
		"gui/backend/metal/backend.go":         true,
		"gui/backend/internal/framestate/f.go": true,
		// Mixed-phase and generation-time files stay out: their bare
		// reads are correct, and Themed() depends on them.
		"gui/view_toast.go":     false,
		"gui/theme.go":          false,
		"gui/layout_arrange.go": false,
		"examples/x/main.go":    false,
	}
	for path, want := range cases {
		if got := themeScanned(path); got != want {
			t.Errorf("themeScanned(%q) = %v, want %v", path, got, want)
		}
	}
}
