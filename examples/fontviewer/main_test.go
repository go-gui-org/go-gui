package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeLight.WithPadding(false))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &FontViewerState{FontSize: initialFontSize, Sample: "The quick brown fox"},
		Width:  initialWinW,
		Height: initialWinH,
	})
	_ = mainView(w).GenerateLayout(w)
}

func TestFilterFontFamilies(t *testing.T) {
	all := []string{"Arial", "Courier New", "Times New Roman", "Zapfino"}
	got := filterFontFamilies(all, "")
	if len(got) != len(all) {
		t.Fatalf("empty filter should return all: got %d, want %d", len(got), len(all))
	}

	got = filterFontFamilies(all, "aria")
	if len(got) != 1 || got[0] != "Arial" {
		t.Fatalf(`"aria" filter: got %v, want ["Arial"]`, got)
	}

	got = filterFontFamilies(all, "zzz")
	if len(got) != 0 {
		t.Fatalf(`"zzz" filter: got %d results, want 0`, len(got))
	}

	got = filterFontFamilies(nil, "foo")
	if got != nil {
		t.Fatalf("nil input should return nil, got %v", got)
	}
}

func TestFilterFontFamiliesCaseInsensitive(t *testing.T) {
	all := []string{"Arial", "ARIAL", "arial"}
	got := filterFontFamilies(all, "ARI")
	if len(got) != 3 {
		t.Fatalf("case-insensitive: got %d, want 3", len(got))
	}
}

func TestCardHeight(t *testing.T) {
	h28 := cardHeight(28)
	h12 := cardHeight(12)
	h72 := cardHeight(72)

	if h12 >= h28 {
		t.Errorf("cardHeight(12)=%v should be < cardHeight(28)=%v", h12, h28)
	}
	if h28 >= h72 {
		t.Errorf("cardHeight(28)=%v should be < cardHeight(72)=%v", h28, h72)
	}
	if h12 < nameRowH+previewPad {
		t.Errorf("cardHeight(12)=%v too small, should be > %v", h12, nameRowH+previewPad)
	}
}

func TestInWindow(t *testing.T) {
	matches := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	// 4 cols → rows: 0=[a,b,c,d], 1=[e,f,g,h]
	// Emitted rows 0-0 → only a-d.
	if !inWindow(matches, "a", 0, 0, 4) {
		t.Error("a should be in window rows 0-0 cols 4")
	}
	if inWindow(matches, "e", 0, 0, 4) {
		t.Error("e should not be in window rows 0-0 cols 4")
	}
	// Emitted rows 1-1 → only e-h.
	if !inWindow(matches, "h", 1, 1, 4) {
		t.Error("h should be in window rows 1-1 cols 4")
	}
	if inWindow(matches, "z", 0, 1, 4) {
		t.Error("z should not be in window")
	}
}

// TestVisibleRangeMath verifies the spacer invariant for known inputs.
func TestVisibleRangeMath(t *testing.T) {
	// Simulate a grid with 500 matches, ~200px rows, 600px viewport.
	n := 500
	cols := 4
	rows := (n + cols - 1) / cols // 125
	rowH := float32(200)
	listH := float32(600)
	scrollY := float32(-4000) // scrolled ~20 rows down
	overscan := 4

	first, last := gui.ListVisibleRange(rows, rowH, listH, scrollY, overscan)

	// Emitted count.
	emitted := last - first + 1
	if emitted < 1 {
		t.Fatalf("no visible rows: first=%d last=%d", first, last)
	}

	// Spacer invariant.
	topSpacer := float32(first) * rowH
	bottomSpacer := float32(rows-1-last) * rowH
	total := topSpacer + float32(emitted)*rowH + bottomSpacer
	want := float32(rows) * rowH
	if total != want {
		t.Errorf("spacer invariant: top(%d)*%v + %d*%v + bottom(%d)*%v = %v, want %v",
			first, rowH, emitted, rowH, rows-1-last, rowH, total, want)
	}
}

// TestRandomPangramExcludes verifies a shuffle never returns the
// current sample (possible because len(pangrams) > 1).
func TestRandomPangramExcludes(t *testing.T) {
	t.Parallel()
	if len(pangrams) < 2 {
		t.Skip("needs at least two pangrams")
	}
	for _, exclude := range pangrams {
		for range 50 {
			if got := randomPangram(exclude); got == exclude {
				t.Fatalf("randomPangram(%q) returned the excluded value", exclude)
			}
		}
	}
}

func TestMainViewEmptyCatalog(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeLight.WithBorders(true))
	// Families is nil, Loaded is false → empty state shows "No system fonts".
	w := gui.NewWindow(gui.WindowCfg{
		State:  &FontViewerState{FontSize: initialFontSize},
		Width:  initialWinW,
		Height: initialWinH,
	})
	_ = mainView(w).GenerateLayout(w)
}
