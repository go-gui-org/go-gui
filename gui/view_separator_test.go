package gui

import "testing"

// separatorWindow renders a separator inside a fill root so its
// arranged geometry can be asserted. The root is pinned borderless and
// padding-free so the fill area is exactly the window.
func separatorWindow(t *testing.T, vertical bool, cfg SeparatorCfg) *Window {
	t.Helper()
	w := NewTestWindow(WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(win *Window) View {
		root := ContainerCfg{
			Sizing:     FillFill,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Content:    []View{Separator(cfg)},
		}
		if vertical {
			return Row(root)
		}
		return Column(root)
	})
	return w
}

func TestSeparatorHorizontalFillsParentWidth(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	w := separatorWindow(t, false, SeparatorCfg{ID: "sep"})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if !f32AreClose(sep.Shape.Width, 400) {
		t.Errorf("width = %f, want 400 (fill)", sep.Shape.Width)
	}
	if !f32AreClose(sep.Shape.Height, 1) {
		t.Errorf("height = %f, want theme thickness 1", sep.Shape.Height)
	}
	if sep.Shape.SizeBorder != 0 {
		t.Errorf("SizeBorder = %f, want 0 (a separator never carries a border)",
			sep.Shape.SizeBorder)
	}
}

func TestSeparatorVerticalFillsParentHeight(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	w := separatorWindow(t, true, SeparatorCfg{ID: "sep", Vertical: true})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if !f32AreClose(sep.Shape.Height, 300) {
		t.Errorf("height = %f, want 300 (fill)", sep.Shape.Height)
	}
	if !f32AreClose(sep.Shape.Width, 1) {
		t.Errorf("width = %f, want theme thickness 1", sep.Shape.Width)
	}
}

// A bordered theme must not add a border around the rule or change its
// thickness — that is the silent collapse bug the widget exists to kill.
func TestSeparatorThicknessUnaffectedByBorders(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark.WithBorders(true))
	w := separatorWindow(t, false, SeparatorCfg{ID: "sep"})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if !f32AreClose(sep.Shape.Height, ThemeDark.separatorStyle.Size) {
		t.Errorf("height = %f, want theme thickness %f under a bordered theme",
			sep.Shape.Height, ThemeDark.separatorStyle.Size)
	}
	if sep.Shape.SizeBorder != 0 {
		t.Errorf("SizeBorder = %f, want 0 under a bordered theme",
			sep.Shape.SizeBorder)
	}
}

func TestSeparatorCustomSizeAndColor(t *testing.T) {
	w := separatorWindow(t, false, SeparatorCfg{
		ID:    "sep",
		Size:  3,
		Color: Red,
	})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if !f32AreClose(sep.Shape.Height, 3) {
		t.Errorf("height = %f, want 3", sep.Shape.Height)
	}
	if sep.Shape.Color != Red {
		t.Errorf("color = %v, want %v", sep.Shape.Color, Red)
	}
}

func TestSeparatorZeroColorFallsBackToTheme(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	w := separatorWindow(t, false, SeparatorCfg{ID: "sep"})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if sep.Shape.Color != ThemeDark.separatorStyle.Color {
		t.Errorf("color = %v, want theme %v",
			sep.Shape.Color, ThemeDark.separatorStyle.Color)
	}
}

func TestSeparatorLengthFixesLongAxis(t *testing.T) {
	w := separatorWindow(t, false, SeparatorCfg{
		ID:     "sep",
		Length: 100,
	})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if !f32AreClose(sep.Shape.Width, 100) {
		t.Errorf("width = %f, want 100 (fixed length)", sep.Shape.Width)
	}
}

func TestSeparatorVerticalLength(t *testing.T) {
	w := separatorWindow(t, true, SeparatorCfg{
		ID:       "sep",
		Vertical: true,
		Length:   50,
	})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if !f32AreClose(sep.Shape.Height, 50) {
		t.Errorf("height = %f, want 50 (fixed length)", sep.Shape.Height)
	}
}

// Inset is transparent margin: the rule shrinks by the inset on both
// sides of the long axis, and the widget still spans the parent.
func TestSeparatorInsetShrinksRule(t *testing.T) {
	w := separatorWindow(t, false, SeparatorCfg{
		ID:    "sep",
		Inset: NewPadding(0, 10, 0, 10),
	})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if !f32AreClose(sep.Shape.Width, 380) {
		t.Errorf("width = %f, want 380 (400 minus 10+10 inset)", sep.Shape.Width)
	}
	if !f32AreClose(sep.Shape.Height, 1) {
		t.Errorf("height = %f, want 1", sep.Shape.Height)
	}
}

// A vertical separator with an inset wraps in a row: the rule keeps the
// theme thickness and still fills the parent's height.
func TestSeparatorVerticalInset(t *testing.T) {
	w := separatorWindow(t, true, SeparatorCfg{
		ID:       "sep",
		Vertical: true,
		Inset:    NewPadding(10, 0, 10, 0),
	})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if !f32AreClose(sep.Shape.Width, 1) {
		t.Errorf("width = %f, want theme thickness 1", sep.Shape.Width)
	}
	if !f32AreClose(sep.Shape.Height, 280) {
		t.Errorf("height = %f, want 280 (300 minus 10+10 inset)", sep.Shape.Height)
	}
}

func TestSeparatorA11YLabel(t *testing.T) {
	w := separatorWindow(t, false, SeparatorCfg{
		ID:      "sep",
		A11YCfg: A11YCfg{A11YLabel: "divider"},
	})

	sep := findShapeByID(&w.layout, "sep")
	if sep == nil {
		t.Fatal("separator shape not found")
	}
	if sep.Shape.a11Y == nil {
		t.Fatal("a11y nil")
	}
	if sep.Shape.a11Y.Label != "divider" {
		t.Errorf("label = %q, want %q", sep.Shape.a11Y.Label, "divider")
	}
}
