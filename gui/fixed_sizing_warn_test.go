package gui

import (
	"strings"
	"testing"
)

// A Fixed axis pins Min = Max = size, so a conflicting stated bound
// never takes effect (issue #635). The DebugSizing category reports
// it at generation time, while the stated bounds are still visible.
func TestFixedSizingConflictWarns(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			ID:       "fixed-warn",
			Sizing:   FixedFixed,
			Width:    100,
			Height:   50,
			MinWidth: 20,
		})
	})

	found := w.TestFindings(DebugAll)
	if len(found) != 1 {
		t.Fatalf("want 1 sizing finding, got %q", found)
	}
	if !strings.Contains(found[0], `"fixed-warn"`) ||
		!strings.Contains(found[0], "width") ||
		!strings.Contains(found[0], "ignored") {
		t.Fatalf("want the finding to name the shape, axis and "+
			"outcome, got %q", found[0])
	}
}

// A bound equal to the size is redundant but harmless: the pin
// changes nothing, so it stays quiet.
func TestFixedSizingRedundantBoundsQuiet(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			ID:        "fixed-redundant",
			Sizing:    FixedFixed,
			Width:     100,
			Height:    50,
			MinWidth:  100,
			MaxWidth:  100,
			MinHeight: 50,
			MaxHeight: 50,
		})
	})

	if found := w.TestFindings(DebugAll); len(found) != 0 {
		t.Fatalf("redundant bounds must stay quiet, got %q", found)
	}
}

// Unset bounds (zero) and non-Fixed axes keep whatever the pass
// computes; there is nothing to report.
func TestFixedSizingUnsetAndNonFixedQuiet(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID:       "fill-minmax",
					Sizing:   FillFill,
					MinWidth: 20,
					MaxWidth: 200,
				}),
				Column(ContainerCfg{
					ID:     "fixed-bare",
					Sizing: FixedFixed,
					Width:  100,
					Height: 50,
				}),
			},
		})
	})

	if found := w.TestFindings(DebugAll); len(found) != 0 {
		t.Fatalf("want no findings, got %q", found)
	}
}

// A Fixed axis with no positive size degrades to content sizing
// (issue #94) and keeps its bounds, so it stays quiet.
func TestFixedSizingZeroSizeQuiet(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			ID:       "fixed-zero",
			Sizing:   FixedFixed,
			MinWidth: 20,
		})
	})

	if found := w.TestFindings(DebugAll); len(found) != 0 {
		t.Fatalf("zero-size Fixed must stay quiet, got %q", found)
	}
}

// The height axis reports independently of the width axis.
func TestFixedSizingHeightAxisWarns(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			ID:        "fixed-height",
			Sizing:    FixedFixed,
			Width:     100,
			Height:    50,
			MaxHeight: 200,
		})
	})

	found := w.TestFindings(DebugAll)
	if len(found) != 1 {
		t.Fatalf("want 1 sizing finding, got %q", found)
	}
	if !strings.Contains(found[0], `"fixed-height"`) ||
		!strings.Contains(found[0], "height") {
		t.Fatalf("want the finding to name the shape and axis, "+
			"got %q", found[0])
	}
}

// A DrawCanvas with Fixed sizing and stated bounds goes through the
// same pin, so it reports through the same category.
func TestFixedSizingDrawCanvasWarns(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return DrawCanvas(DrawCanvasCfg{
			ID:       "fixed-canvas",
			Sizing:   FixedFixed,
			Width:    80,
			Height:   60,
			MinWidth: 10,
		})
	})

	found := w.TestFindings(DebugAll)
	if len(found) != 1 {
		t.Fatalf("want 1 sizing finding, got %q", found)
	}
	if !strings.Contains(found[0], `"fixed-canvas"`) {
		t.Fatalf("want the finding to name the canvas, got %q",
			found[0])
	}
}

// The finding is gated by its own category: a mask without
// DebugSizing stays silent at the site.
func TestFixedSizingGatedByCategory(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{
			ID:       "fixed-gated",
			Sizing:   FixedFixed,
			Width:    100,
			Height:   50,
			MinWidth: 20,
		})
	})

	if found := w.TestFindings(DebugAll &^ DebugSizing); len(found) != 0 {
		t.Fatalf("mask without DebugSizing reported %q", found)
	}
}
