package gui

import "testing"

// ID scoping for the splitter (issue #264). The splitter composes its
// pane, handle and collapse-button IDs from its own effective ID inside
// GenerateLayout, so the whole subtree scopes with the root instead of
// staying window-global. These tests pin both halves of that contract:
// the parts scope under an ID-bearing ancestor, and with no such
// ancestor they keep the historical flat spellings.

// splitterScopedPanel wraps a splitter in an ID-bearing Column, which is
// what opens a scope for the splitter's effective ID.
func splitterScopedPanel(id string, v View) View {
	return Column(ContainerCfg{
		ID:         id,
		Sizing:     FillFill,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    []View{v},
	})
}

// TestSplitterPartsResolveUnderAncestorScope walks every part of a
// nested splitter by its scoped public ID.
func TestSplitterPartsResolveUnderAncestorScope(t *testing.T) {
	w := NewTestWindow(WindowCfg{
		Width:  splitterTestW,
		Height: splitterTestH,
	})
	w.TestRender(func(_ *Window) View {
		return splitterScopedPanel("outer",
			Splitter(splitterCollapsibleCfg("sp")))
	})

	for _, id := range []string{
		"outer:sp",
		"outer:sp:pane:first",
		"outer:sp:handle",
		"outer:sp:pane:second",
		"outer:sp:button:1",
		"outer:sp:button:2",
	} {
		if _, ok := w.layout.FindByID(id); !ok {
			t.Errorf("FindByID(%q) found nothing; part did not scope "+
				"under its ancestor", id)
		}
	}
	// The pre-#264 window-global spellings must be gone entirely.
	for _, id := range []string{
		"sp:pane:first", "sp:handle", "sp:pane:second", "sp:button:1",
		"sp:button:2",
	} {
		if _, ok := w.layout.FindByID(id); ok {
			t.Errorf("FindByID(%q) still resolves; the part is still "+
				"window-global", id)
		}
	}
}

// TestSplitterSameLeafIDInTwoScopesDoesNotCollide is the composability
// case scoping exists for: one splitter leaf ID reused under two
// different ID-bearing panels in the same frame.
func TestSplitterSameLeafIDInTwoScopesDoesNotCollide(t *testing.T) {
	w := NewTestWindow(WindowCfg{
		Width:  splitterTestW,
		Height: splitterTestH,
	})
	w.TestRender(func(_ *Window) View {
		return Row(ContainerCfg{
			Sizing:     FillFill,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Content: []View{
				splitterScopedPanel("settings",
					Splitter(splitterCollapsibleCfg("sp"))),
				splitterScopedPanel("profile",
					Splitter(splitterCollapsibleCfg("sp"))),
			},
		})
	})

	if dups := w.TestDuplicateIDs(); len(dups) != 0 {
		t.Errorf("duplicate effective IDs across two scopes: %v", dups)
	}

	settings, ok := w.layout.FindByID("settings:sp:handle")
	if !ok {
		t.Fatal("settings:sp:handle found nothing")
	}
	profile, ok := w.layout.FindByID("profile:sp:handle")
	if !ok {
		t.Fatal("profile:sp:handle found nothing")
	}
	// Distinct shapes, not one slot resolved twice. The panels sit
	// side by side in a Row, so the second handle is to the right.
	if settings.Shape == profile.Shape {
		t.Fatal("both handles resolved to the same shape")
	}
	if settings.Shape.X >= profile.Shape.X {
		t.Errorf("handle X: settings = %g, profile = %g; want the "+
			"settings handle left of the profile one",
			settings.Shape.X, profile.Shape.X)
	}
	for _, id := range []string{
		"settings:sp:pane:first", "settings:sp:button:1",
		"profile:sp:pane:first", "profile:sp:button:1",
	} {
		if _, found := w.layout.FindByID(id); !found {
			t.Errorf("FindByID(%q) found nothing", id)
		}
	}
}

// TestSplitterUnscopedKeepsFlatIDs guards the no-collateral-break half:
// with no ID-bearing ancestor the effective ID equals cfg.ID, so every
// part keeps the exact spelling it had before #264.
func TestSplitterUnscopedKeepsFlatIDs(t *testing.T) {
	h := newSplitterHarness(t, splitterCollapsibleCfg("sp"))
	// splitterFillParent's Row carries no ID, so it opens no scope.
	for _, id := range []string{
		"sp",
		"sp:pane:first",
		"sp:handle",
		"sp:pane:second",
		"sp:button:1",
		"sp:button:2",
	} {
		if _, ok := h.w.layout.FindByID(id); !ok {
			t.Errorf("FindByID(%q) found nothing; the legacy flat "+
				"spelling changed", id)
		}
	}
}
