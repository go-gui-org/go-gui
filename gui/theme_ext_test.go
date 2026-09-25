package gui

import "testing"

// Sibling-owned extension values, simulated locally. Real siblings
// (go-edit, go-charts) declare their own types in their own packages;
// gui never imports them. The slot keys by exact type.
type themeExtSample struct {
	Tag string
}

type themeExtOther struct {
	N int
}

// TestWithExtRoundTrip pins the core contract of issue #733: a value
// stored with WithExt reads back with Ext.
func TestWithExtRoundTrip(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)
	themed := WithExt(base, themeExtSample{Tag: "editor"})

	got, ok := Ext[themeExtSample](themed)
	if !ok {
		t.Fatal("Ext found no value after WithExt")
	}
	if got.Tag != "editor" {
		t.Errorf("Ext.Tag = %q, want %q", got.Tag, "editor")
	}
}

// TestExtMissing pins the fallback half of the locked decision: no
// value stored means the zero value and false, never a panic.
func TestExtMissing(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)

	got, ok := Ext[themeExtSample](base)
	if ok {
		t.Errorf("Ext ok = true on a theme with no extension, value %v", got)
	}
	if got != (themeExtSample{}) {
		t.Errorf("Ext = %v on a theme with no extension, want zero", got)
	}
	if zeroGot, zeroOK := Ext[themeExtSample](Theme{}); zeroOK ||
		zeroGot != (themeExtSample{}) {
		t.Errorf("Ext on zero Theme = (%v, %v), want (zero, false)",
			zeroGot, zeroOK)
	}
}

// TestExtWrongType pins that the slot keys by exact type: a stored
// value under one type is invisible under another.
func TestExtWrongType(t *testing.T) {
	themed := WithExt(ThemeMaker(themeDarkCfg), themeExtSample{Tag: "x"})

	if got, ok := Ext[themeExtOther](themed); ok || got != (themeExtOther{}) {
		t.Errorf("Ext[other] = (%v, %v), want (zero, false)", got, ok)
	}
}

// TestWithExtRestampsID pins that WithExt returns a new theme
// identity and leaves the parent untouched (copy-on-write: the two
// themes share nothing the caller can mutate through).
func TestWithExtRestampsID(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)
	parent := WithExt(base, themeExtSample{Tag: "parent"})
	child := WithExt(parent, themeExtSample{Tag: "child"})

	if child.id == parent.id {
		t.Error("WithExt reused the parent theme id")
	}
	if parent.id == base.id {
		t.Error("WithExt reused the grandparent theme id")
	}
	got, ok := Ext[themeExtSample](parent)
	if !ok || got.Tag != "parent" {
		t.Errorf("parent Ext = (%v, %v), want ({parent}, true)", got, ok)
	}
}

// TestWithExtOverwriteSameType pins that storing twice under one
// type keeps the last value, and the intermediate theme keeps the
// first (clone on write reaches every store).
func TestWithExtOverwriteSameType(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)
	first := WithExt(base, themeExtSample{Tag: "first"})
	second := WithExt(first, themeExtSample{Tag: "second"})

	got, ok := Ext[themeExtSample](second)
	if !ok || got.Tag != "second" {
		t.Errorf("second Ext = (%v, %v), want ({second}, true)", got, ok)
	}
	kept, keptOK := Ext[themeExtSample](first)
	if !keptOK || kept.Tag != "first" {
		t.Errorf("first Ext = (%v, %v), want ({first}, true)", kept, keptOK)
	}
}

// TestWithExtCoexistingTypes pins that values under different types
// share one theme without disturbing each other.
func TestWithExtCoexistingTypes(t *testing.T) {
	themed := WithExt(
		WithExt(ThemeMaker(themeDarkCfg), themeExtSample{Tag: "s"}),
		themeExtOther{N: 7})

	sample, sampleOK := Ext[themeExtSample](themed)
	other, otherOK := Ext[themeExtOther](themed)
	if !sampleOK || sample.Tag != "s" {
		t.Errorf("sample Ext = (%v, %v), want ({s}, true)", sample, sampleOK)
	}
	if !otherOK || other.N != 7 {
		t.Errorf("other Ext = (%v, %v), want ({7}, true)", other, otherOK)
	}
}

// TestWithExtSurvivesPaddingRestore pins the edited true path of
// WithPadding: a strip followed by a restore still carries the
// extension.
func TestWithExtSurvivesPaddingRestore(t *testing.T) {
	base := WithExt(ThemeMaker(themeDarkCfg), themeExtSample{Tag: "keep"})
	restored := base.WithPadding(false).WithPadding(true)

	got, ok := Ext[themeExtSample](restored)
	if !ok || got.Tag != "keep" {
		t.Errorf("restored Ext = (%v, %v), want ({keep}, true)", got, ok)
	}
}

// TestWithExtSurvivesRebuilds pins the derive-at-build rule: every
// rebuild path carries the extension, because siblings re-derive
// from the base theme rather than tracking each path by hand.
func TestWithExtSurvivesRebuilds(t *testing.T) {
	base := WithExt(ThemeMaker(themeDarkCfg), themeExtSample{Tag: "keep"})

	sized, err := base.AdjustFontSize(1, 1, 100)
	if err != nil {
		t.Fatalf("AdjustFontSize = %v", err)
	}
	rebuilt := []struct {
		name  string
		theme Theme
	}{
		{"WithColors", base.WithColors(ColorOverrides{})},
		{"WithPadding", base.WithPadding(false)},
		{"WithBorders", base.WithBorders(false)},
		{"AdjustFontSize", sized},
	}

	for _, r := range rebuilt {
		got, ok := Ext[themeExtSample](r.theme)
		if !ok || got.Tag != "keep" {
			t.Errorf("%s Ext = (%v, %v), want ({keep}, true)",
				r.name, got, ok)
		}
	}
}

// TestExtFollowsThemedScope pins the generation-phase read rule: an
// extension stored on a scoped theme is visible inside the Themed
// subtree through the installed theme, and invisible outside it.
func TestExtFollowsThemedScope(t *testing.T) {
	restoreTheme(t)
	outer := ThemeMaker(themeDarkCfg)
	inner := WithExt(ThemeMaker(themeDarkCfg), themeExtSample{Tag: "in"})

	mark := RGB(0, 200, 0)
	unmarked := RGB(200, 0, 0)
	paint := func() Color {
		if _, ok := Ext[themeExtSample](CurrentTheme()); ok {
			return mark
		}
		return unmarked
	}

	w := NewTestWindow(WindowCfg{})
	w.SetTheme(outer)
	root := w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			ID: "root",
			Content: []View{
				Themed(inner, func(*Window) View {
					return Column(ContainerCfg{
						ID:    "scoped",
						Color: paint(),
					})
				}),
				ViewFunc(func(*Window) View {
					return Column(ContainerCfg{
						ID:    "sibling",
						Color: paint(),
					})
				}),
			},
		})
	})

	if got := findByLeafIDTest(t, root, "scoped").Shape.Color; got != mark {
		t.Errorf("scoped color = %v, want %v (extension visible)",
			got, mark)
	}
	if got := findByLeafIDTest(t, root, "sibling").Shape.Color; got != unmarked {
		t.Errorf("sibling color = %v, want %v (extension not visible)",
			got, unmarked)
	}
}
