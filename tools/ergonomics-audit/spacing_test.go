package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scanSpacingSrc runs the spacing-mode rules over one in-memory file and
// returns the findings as "fn:verb:expr", with a "[deferred]" suffix when
// the line carried the marker.
func scanSpacingSrc(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	marked := markedLines(fset, f, spacingMarker)
	var out []string
	inspectSpacing(fset, f, func(fn string, line int, verb, expr string) {
		s := fn + ":" + verb + ":" + expr
		if marked[line] {
			s += " [deferred]"
		}
		out = append(out, s)
	})
	return out
}

func TestSpacingFlagsGapLiterals(t *testing.T) {
	t.Parallel()
	const src = `package main

func gaps(t gui.Theme) {
	_ = gui.Column(gui.ContainerCfg{Spacing: gui.SpacingPx(8)})
	_ = gui.Row(gui.ContainerCfg{Spacing: gui.SpacingPx(2.5)})
	// Every spacing field, not only Spacing.
	_ = gui.DatePicker(gui.DatePickerCfg{CellSpacing: gui.SpacingPx(3)})
	// Outside a Cfg literal: SpacingPx builds nothing but a gap.
	gap := gui.SpacingPx(12)
	// Zero is a real choice, not an off-ladder gap.
	_ = gui.Row(gui.ContainerCfg{Spacing: gui.SpacingPx(0)})
	// A role passes, and so does a gap computed from one.
	_ = gui.Row(gui.ContainerCfg{Spacing: gui.SpacingMedium})
	_ = gui.Row(gui.ContainerCfg{Spacing: gui.SpacingPx(2 * t.SpacingLarge)})
	_ = gui.Row(gui.ContainerCfg{Spacing: gui.NoSpacing, Content: gap})
}
`
	got := scanSpacingSrc(t, src)
	want := []string{
		"gaps:" + verbGap + ":gui.SpacingPx(8)",
		"gaps:" + verbGap + ":gui.SpacingPx(2.5)",
		"gaps:" + verbGap + ":gui.SpacingPx(3)",
		"gaps:" + verbGap + ":gui.SpacingPx(12)",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestSpacingFlagsInsetLiterals(t *testing.T) {
	t.Parallel()
	const src = `package main

var pad = gui.PadAll(12)

func insets(t gui.Theme) {
	_ = gui.Column(gui.ContainerCfg{Padding: gui.NewPadding(8, 8, 8, 8)})
	_ = gui.Column(gui.ContainerCfg{Padding: gui.PadVH(4, 10)})
	// Mixed args already name a role for the non-zero sides.
	_ = gui.Column(gui.ContainerCfg{Padding: gui.NewPadding(0, t.SpacingSmall, 0, t.SpacingSmall)})
	// All zero is no inset at all.
	_ = gui.Column(gui.ContainerCfg{Padding: gui.PadAll(0)})
	_ = gui.Column(gui.ContainerCfg{Padding: t.PaddingMedium})
}
`
	got := scanSpacingSrc(t, src)
	want := []string{
		":" + verbInset + ":gui.PadAll(12)",
		"insets:" + verbInset + ":gui.NewPadding(8, 8, 8, 8)",
		"insets:" + verbInset + ":gui.PadVH(4, 10)",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestSpacingButtonPaddingSaysDelete(t *testing.T) {
	t.Parallel()
	const src = `package main

func buttons() {
	_ = gui.Button(gui.ButtonCfg{Padding: gui.NewPadding(8, 16, 8, 16)})
	// A container inside a button content is not the button inset.
	_ = gui.Button(gui.ButtonCfg{Content: []gui.View{
		gui.Row(gui.ContainerCfg{Padding: gui.PadAll(4)}),
	}})
}
`
	got := scanSpacingSrc(t, src)
	want := []string{
		"buttons:" + verbButtonInset + ":gui.NewPadding(8, 16, 8, 16)",
		"buttons:" + verbInset + ":gui.PadAll(4)",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestSpacingExemptsCanvasMargins(t *testing.T) {
	t.Parallel()
	const src = `package main

func chart() {
	_ = gui.DrawCanvas(gui.DrawCanvasCfg{Padding: gui.PadAll(24)})
	// Nearest enclosing literal decides: a container nested in a canvas
	// cfg is still a container inset.
	_ = gui.DrawCanvas(gui.DrawCanvasCfg{Content: []gui.View{
		gui.Column(gui.ContainerCfg{Padding: gui.PadAll(8)}),
	}})
}
`
	got := scanSpacingSrc(t, src)
	want := []string{"chart:" + verbInset + ":gui.PadAll(8)"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

func TestSpacingMarkerDefersFinding(t *testing.T) {
	t.Parallel()
	const src = `package gui

func hairline() {
	_ = ContainerCfg{Spacing: SpacingPx(1)} // ergonomics-audit:spacing — 1px rule
}
`
	got := scanSpacingSrc(t, src)
	want := []string{"hairline:" + verbGap + ":SpacingPx(1) [deferred]"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

// writeRepoFile writes src at rel under repo, creating directories.
func writeRepoFile(t *testing.T, repo, rel, src string) {
	t.Helper()
	path := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

const spacingLiteralSrc = `package x

var pad = PadAll(12)
`

func TestSpacingGoGuiScope(t *testing.T) {
	t.Parallel()
	// In go-gui, only gui/view_*.go and examples/ are call sites; the
	// theme and padding files define the roles.
	repo := t.TempDir()
	writeRepoFile(t, repo, "go.mod", "module github.com/go-gui-org/go-gui\n")
	writeRepoFile(t, repo, "gui/padding.go", spacingLiteralSrc)
	writeRepoFile(t, repo, "gui/view_row.go", spacingLiteralSrc)
	writeRepoFile(t, repo, "gui/view_row_test.go", spacingLiteralSrc)
	writeRepoFile(t, repo, "examples/demo/main.go", spacingLiteralSrc)
	writeRepoFile(t, repo, "tools/x/main.go", spacingLiteralSrc)

	found, err := scanSpacing(repo)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range found {
		paths = append(paths, f.path)
	}
	want := "examples/demo/main.go|gui/view_row.go"
	if strings.Join(paths, "|") != want {
		t.Errorf("scanned = %v, want %s", paths, want)
	}
}

func TestSpacingSiblingScope(t *testing.T) {
	t.Parallel()
	// A consumer repo is all call sites: every non-test file is scanned.
	repo := t.TempDir()
	writeRepoFile(t, repo, "go.mod", "module github.com/go-gui-org/go-charts\n")
	writeRepoFile(t, repo, "chart/axis.go", spacingLiteralSrc)
	writeRepoFile(t, repo, "gui/padding.go", spacingLiteralSrc)
	writeRepoFile(t, repo, "chart/axis_test.go", spacingLiteralSrc)

	found, err := scanSpacing(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Errorf("findings = %+v, want 2 (chart/axis.go, gui/padding.go)", found)
	}
}

func TestRunSpacingGates(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, repo, "go.mod", "module example.com/app\n")
	writeRepoFile(t, repo, "main.go", spacingLiteralSrc)
	if err := runSpacing([]string{repo}); err == nil {
		t.Fatal("runSpacing passed with an unmarked literal")
	}
	writeRepoFile(t, repo, "main.go", `package x

var pad = PadAll(12) // ergonomics-audit:spacing
`)
	if err := runSpacing([]string{repo}); err != nil {
		t.Fatalf("runSpacing failed with a marked literal: %v", err)
	}
}

// RadiusPx and BorderPx with a literal > 0 spell a value the theme names
// as a role (issue #867). Zero, a role and a computed value pass.
func TestSpacingFlagsRadiusAndBorderLiterals(t *testing.T) {
	t.Parallel()
	const src = `package main

func shapes(t gui.Theme, h float32) {
	_ = gui.Column(gui.ContainerCfg{Radius: gui.RadiusPx(8), SizeBorder: gui.BorderPx(2)})
	_ = gui.TabControl(gui.TabControlCfg{RadiusTab: gui.RadiusPx(3)})
	_ = gui.Column(gui.ContainerCfg{Radius: gui.RadiusPx(0), SizeBorder: gui.BorderPx(0)})
	_ = gui.Column(gui.ContainerCfg{Radius: gui.RadiusMedium, SizeBorder: gui.BorderThin})
	_ = gui.Column(gui.ContainerCfg{Radius: gui.RadiusPx(h / 2), SizeBorder: gui.NoBorder})
	_ = gui.Column(gui.ContainerCfg{Radius: gui.RadiusPx(50)}) // ergonomics-audit:spacing — circle
}
`
	got := scanSpacingSrc(t, src)
	want := []string{
		"shapes:" + verbRadius + ":gui.RadiusPx(8)",
		"shapes:" + verbBorder + ":gui.BorderPx(2)",
		"shapes:" + verbRadius + ":gui.RadiusPx(3)",
		"shapes:" + verbRadius + ":gui.RadiusPx(50) [deferred]",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}
