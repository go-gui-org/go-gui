package gui

import "testing"

func TestTextGeneratesLayout(t *testing.T) {
	w := &Window{}
	v := Text(TextCfg{ID: "t1", Text: "Hello"})
	layout := GenerateViewLayout(v, w)
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
	layout := GenerateViewLayout(
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
	layout := GenerateViewLayout(
		Text(TextCfg{Text: "wrap me", Mode: TextModeWrap}), w)
	if layout.Shape.Sizing != FillFit {
		t.Errorf("wrap mode sizing: got %+v, want FillFit",
			layout.Shape.Sizing)
	}
}

func TestTextPasswordMode(t *testing.T) {
	w := &Window{}
	layout := GenerateViewLayout(
		Text(TextCfg{Text: "secret", IsPassword: true}), w)
	if layout.Shape.TC == nil {
		t.Fatal("expected TC")
	}
	if !layout.Shape.TC.TextIsPassword {
		t.Error("expected password mode")
	}
}

func TestTextInvisible(t *testing.T) {
	w := &Window{}
	layout := GenerateViewLayout(
		Text(TextCfg{Text: "hidden", Invisible: true}), w)
	if !layout.Shape.Disabled || !layout.Shape.OverDraw {
		t.Error("invisible text should be disabled+overdraw")
	}
}

func TestTextA11Y(t *testing.T) {
	w := &Window{}
	layout := GenerateViewLayout(
		Text(TextCfg{Text: "label"}), w)
	if layout.Shape.A11YRole != AccessRoleStaticText {
		t.Errorf("a11y role: got %d", layout.Shape.A11YRole)
	}
}
