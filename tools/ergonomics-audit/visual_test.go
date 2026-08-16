package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunVisualGatesOnLiterals covers runVisual end-to-end: the gating
// contract that make check-all relies on. Unmarked dimming alphas and
// size steps fail the audit; the marker defers them; clean code passes.
func TestRunVisualGatesOnLiterals(t *testing.T) {
	repo := t.TempDir()
	guiDir := filepath.Join(repo, "gui")
	if err := os.MkdirAll(guiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	viewPath := filepath.Join(guiDir, "view_roller.go")

	write := func(src string) {
		t.Helper()
		if err := os.WriteFile(viewPath, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Unmarked dimming literal in a scanned path: gates.
	write(`package gui

func rollerItem(ts TextStyle) TextStyle {
	alpha := uint8(150)
	itemTS := ts
	itemTS.Color = RGBA(ts.Color.R, ts.Color.G, ts.Color.B, alpha)
	return itemTS
}
`)
	if err := runVisual([]string{repo}); err == nil {
		t.Fatal("runVisual passed with an unmarked dimming literal")
	}

	// Same line marked: advisory only, passes.
	write(`package gui

func rollerItem(ts TextStyle) TextStyle {
	alpha := uint8(150) // ergonomics-audit:visual
	itemTS := ts
	itemTS.Color = RGBA(ts.Color.R, ts.Color.G, ts.Color.B, alpha)
	return itemTS
}
`)
	if err := runVisual([]string{repo}); err != nil {
		t.Fatalf("runVisual failed with a marked literal: %v", err)
	}

	// No literal: named roles and opaque colors pass.
	write(`package gui

func ok(ts TextStyle) TextStyle {
	itemTS := withRoleAlpha(ts, guiTheme.TextStyleSecondary)
	itemTS.Color = RGBA(200, 50, 50, 255)
	return itemTS
}
`)
	if err := runVisual([]string{repo}); err != nil {
		t.Fatalf("runVisual failed on clean code: %v", err)
	}

	// Unmarked size step: gates.
	write(`package gui

func stepButtons(cfg NumericInputCfg) float32 {
	return f32Max(cfg.TextStyle.Size-4, 8)
}
`)
	if err := runVisual([]string{repo}); err == nil {
		t.Fatal("runVisual passed with an unmarked size step")
	}
}

// scanVisualSrc runs the visual-mode rules over one in-memory file and
// returns the reported findings as "fn:verb:expr", with a "[deferred]"
// suffix when the line carried the marker.
func scanVisualSrc(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	marked := markedLines(fset, f, visualMarker)
	var out []string
	inspectVisual(fset, f, func(fn string, line int, verb, expr string) {
		s := fn + ":" + verb + ":" + expr
		if marked[line] {
			s += " [deferred]"
		}
		out = append(out, s)
	})
	return out
}

func TestVisualFlagsDimmingLiterals(t *testing.T) {
	t.Parallel()
	const src = `package gui

func ghost(color Color) {
	_ = RGBA(color.R, color.G, color.B, 30)
}

func edges(c Color) {
	_ = c.WithOpacity(0.7)
}

func ramp() {
	alpha := uint8(80)
	_ = alpha
}

func trail(color Color) {
	fade := 0.5
	alpha := uint8(10 + fade*245)
	_ = RGBA(color.R, color.G, color.B, alpha)
}
`
	got := scanVisualSrc(t, src)
	want := []string{
		"ghost:spells a dimming alpha:RGBA(color.R, color.G, color.B, 30)",
		"edges:spells a dimming alpha:c.WithOpacity(0.7)",
		"ramp:spells a dimming alpha:uint8(80)",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestVisualFlagsSizeSteps(t *testing.T) {
	t.Parallel()
	const src = `package gui

func floor(cfg NumericInputCfg) {
	_ = f32Max(cfg.TextStyle.Size-4, 8)
}

func namedFloor(cfg NumericInputCfg) {
	_ = f32Max(cfg.TextStyle.Size-4, guiTheme.N6.Size)
}

func roller(ts TextStyle) TextStyle {
	itemTS := ts
	itemTS.Size = ts.Size + 2
	return itemTS
}

func named(ts TextStyle) TextStyle {
	return TextStyle{Size: ts.Size - 2}
}

func geometry(style TextStyle) float32 {
	return style.Size + 4
}

func geometryMax(s handleSize) float32 {
	return f32Max(14, s*2.0)
}
`
	got := scanVisualSrc(t, src)
	want := []string{
		"floor:sizes a text step inline:f32Max(cfg.TextStyle.Size-4, 8)",
		// A named floor is an improvement, not a resolution: the step
		// itself is still arithmetic, so the rule must still see it.
		"namedFloor:sizes a text step inline:f32Max(cfg.TextStyle.Size-4, guiTheme.N6.Size)",
		"roller:sizes a text step inline:ts.Size + 2",
		"named:sizes a text step inline:TextStyle{Size: ts.Size - 2}",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestVisualMarkerDefersFinding(t *testing.T) {
	t.Parallel()
	const src = `package gui

func ramp() {
	alpha := uint8(150) // ergonomics-audit:visual
	_ = alpha
}
`
	got := scanVisualSrc(t, src)
	want := []string{"ramp:spells a dimming alpha:uint8(150) [deferred]"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestVisualIgnoresNonViewFiles(t *testing.T) {
	t.Parallel()
	// theme_maker.go defines the roles; a literal there is a definition.
	// Files outside gui/view_*.go are not scanned at all.
	repo := t.TempDir()
	guiDir := filepath.Join(repo, "gui")
	if err := os.MkdirAll(guiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	makerPath := filepath.Join(guiDir, "theme_maker.go")
	src := `package gui

func scrim() {
	_ = RGBA(0, 0, 0, 50)
}
`
	if err := os.WriteFile(makerPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := scanVisual(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("findings in non-view file = %+v, want none", found)
	}
}
