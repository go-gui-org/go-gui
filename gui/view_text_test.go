package gui

import "testing"

func TestTextGeneratesLayout(t *testing.T) {
	w := &Window{}
	v := Text(TextCfg{ID: "t1", Text: "Hello"})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
	if layout.Shape.ID != "t1" {
		t.Errorf("ID: got %s", layout.Shape.ID)
	}
	if layout.Shape.shapeType != shapeText {
		t.Errorf("type: got %d, want %d",
			layout.Shape.shapeType, shapeText)
	}
}

func TestTextDefaultSizing(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Text(TextCfg{Text: "abc"}), w)
	if layout.Shape.Sizing != FitFit {
		t.Errorf("sizing: got %+v, want FitFit",
			layout.Shape.Sizing)
	}
	if layout.Shape.Width <= 0 {
		t.Error("expected positive width")
	}
	if layout.Shape.Height <= 0 {
		t.Error("expected positive height")
	}
}

func TestTextWrapModeSizing(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Text(TextCfg{Text: "wrap me", Mode: TextModeWrap}), w)
	if layout.Shape.Sizing != FillFit {
		t.Errorf("wrap mode sizing: got %+v, want FillFit",
			layout.Shape.Sizing)
	}
}

func TestTextPasswordMode(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Text(TextCfg{Text: "secret", IsPassword: true}), w)
	if layout.Shape.TC == nil {
		t.Fatal("expected TC")
	}
	if !layout.Shape.TC.textIsPassword {
		t.Error("expected password mode")
	}
}

func TestTextInvisible(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Text(TextCfg{Text: "hidden", Invisible: true}), w)
	if !layout.Shape.Disabled || !layout.Shape.OverDraw {
		t.Error("invisible text should be disabled+overdraw")
	}
}

func TestTextA11Y(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Text(TextCfg{Text: "label"}), w)
	if layout.Shape.A11YRole != AccessRoleStaticText {
		t.Errorf("a11y role: got %d", layout.Shape.A11YRole)
	}
}

// A Text's accessible name comes from its own prose. Building that name
// must not cost a heap allocation per frame: every label in an app is a
// Text, so the cost scales with the screen. The empty text builds no
// accessInfo at all, so it is the baseline.
func TestTextA11YInfoNoAlloc(t *testing.T) {
	w := newTestWindow()
	w.viewState.genDepth = 1
	empty := TextCfg{Text: ""}
	prose := TextCfg{Text: "Save changes", A11YCfg: A11YCfg{A11YDescription: "Writes the file"}}

	// Reset the view-phase arenas each run, as a frame does: the name
	// comes from one, and an arena never reset grows on every call.
	base := testing.AllocsPerRun(50, func() {
		w.scratch.resetViewPools()
		_ = Text(empty).GenerateLayout(w)
	})
	got := testing.AllocsPerRun(50, func() {
		w.scratch.resetViewPools()
		_ = Text(prose).GenerateLayout(w)
	})
	if got > base {
		t.Errorf("allocs = %v, want no more than the empty text's %v",
			got, base)
	}

	// The name and description still reach the shape.
	s := Text(prose).GenerateLayout(w).Shape
	if s.a11Y == nil || s.a11Y.Label != "Save changes" ||
		s.a11Y.Description != "Writes the file" {
		t.Fatalf("a11Y = %+v, want label and description from cfg", s.a11Y)
	}
}
