package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenCaptureTB records what TestGolden reports instead of failing
// the test, so mismatch paths are assertable. Fatalf only records:
// unlike testing.T it does not stop execution, so these tests
// exercise branches where Fatalf is the last statement.
type goldenCaptureTB struct {
	logs   []string
	errors []string
	fatals []string
}

func (m *goldenCaptureTB) Helper() {}

func (m *goldenCaptureTB) Logf(format string, args ...any) {
	m.logs = append(m.logs, fmt.Sprintf(format, args...))
}

func (m *goldenCaptureTB) Errorf(format string, args ...any) {
	m.errors = append(m.errors, fmt.Sprintf(format, args...))
}

func (m *goldenCaptureTB) Fatalf(format string, args ...any) {
	m.fatals = append(m.fatals, fmt.Sprintf(format, args...))
}

// goldenButtonWindow renders one labelled button for the golden
// regression tests.
func goldenButtonWindow(t *testing.T, label string) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 320, Height: 240})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Button(ButtonCfg{ID: "save", Label: label}),
			},
		})
	})
	return w
}

// A restyled widget must fail with a diff naming the case, in both
// themes, and leave the recording under failures/ for review.
func TestGoldenMismatchReportsAndWritesArtifact(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("GOGUI_UPDATE_GOLDEN", "1")
	w := goldenButtonWindow(t, "Save")
	w.TestGolden(t, GoldenCfg{Dir: dir, Name: "case"})

	t.Setenv("GOGUI_UPDATE_GOLDEN", "0")
	mtb := &goldenCaptureTB{}
	w2 := goldenButtonWindow(t, "Delete")
	w2.TestGolden(mtb, GoldenCfg{Dir: dir, Name: "case"})

	if len(mtb.errors) != 2 {
		t.Fatalf("errors = %d, want 2 (dark and light)", len(mtb.errors))
	}
	for _, err := range mtb.errors {
		if !strings.Contains(err, "golden mismatch for case.") {
			t.Errorf("error names no case: %q", err)
		}
	}
	actual, err := os.ReadFile(filepath.Join(dir, "failures",
		"case.dark.actual.golden"))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if !strings.HasPrefix(string(actual), goldenHeader) {
		t.Error("artifact misses the version header")
	}
	if !strings.Contains(string(actual), `text="Delete"`) {
		t.Error("artifact misses the restyled recording")
	}
}

// A hover point that lands on nothing must fail rather than record
// the resting look under a hovered name, and the window's hover
// state is restored afterwards.
func TestGoldenHoverMissFails(t *testing.T) {
	mtb := &goldenCaptureTB{}
	w := goldenButtonWindow(t, "Save")
	w.TestGolden(mtb, GoldenCfg{Dir: t.TempDir(), Name: "case",
		HoverX: 310, HoverY: 230})

	if len(mtb.fatals) == 0 ||
		!strings.Contains(mtb.fatals[0], "over nothing") {
		t.Errorf("fatals = %q, want a hover-miss failure first", mtb.fatals)
	}
	if w.viewState.hoverTargetID != "" {
		t.Errorf("hoverTargetID = %q, want restored empty", w.viewState.hoverTargetID)
	}
}

// The case name is required: without it every recording lands on
// ".dark.golden".
func TestGoldenEmptyNameNeedsName(t *testing.T) {
	mtb := &goldenCaptureTB{}
	w := goldenButtonWindow(t, "Save")
	w.TestGolden(mtb, GoldenCfg{Dir: t.TempDir()})

	if len(mtb.fatals) == 0 ||
		!strings.Contains(mtb.fatals[0], "GoldenCfg.Name") {
		t.Errorf("fatals = %q, want a missing-name failure first", mtb.fatals)
	}
}

// A recording that predates versioned goldens must fail with a
// re-record hint, not a wall of line diffs.
func TestGoldenLegacyFileNeedsRerecord(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "case.dark.golden"),
		[]byte("Rect xy=0.00,0.00 wh=1.00,1.00\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mtb := &goldenCaptureTB{}
	w := goldenButtonWindow(t, "Save")
	w.TestGolden(mtb, GoldenCfg{Dir: dir, Name: "case"})

	found := false
	for _, f := range mtb.fatals {
		found = found || strings.Contains(f, "has no golden header")
	}
	if !found {
		t.Errorf("fatals = %q, want a no-header re-record hint", mtb.fatals)
	}
}

// A recording from a newer format version must name both versions.
func TestGoldenVersionMismatchNamesVersions(t *testing.T) {
	dir := t.TempDir()
	content := "# go-gui-golden v999\nRect xy=0.00,0.00 wh=1.00,1.00\n"
	if err := os.WriteFile(filepath.Join(dir, "case.dark.golden"),
		[]byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	mtb := &goldenCaptureTB{}
	w := goldenButtonWindow(t, "Save")
	w.TestGolden(mtb, GoldenCfg{Dir: dir, Name: "case"})

	found := false
	for _, f := range mtb.fatals {
		found = found || strings.Contains(f, "v999") &&
			strings.Contains(f, "v1")
	}
	if !found {
		t.Errorf("fatals = %q, want both format versions", mtb.fatals)
	}
}
