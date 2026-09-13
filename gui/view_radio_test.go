package gui

import "testing"

func TestRadioIDPassthrough(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{ID: "r1", Label: "A"}), w)
	if layout.Shape.ID != "r1" {
		t.Errorf("ID: got %s", layout.Shape.ID)
	}
}

func TestRadioUnselectedStateNone(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{ID: "r2", Selected: false, OnClick: noop}), w)
	if layout.Shape.A11YState != AccessStateNone {
		t.Error("unselected radio should have None state")
	}
}

func TestRadioOnClickCallback(t *testing.T) {
	fired := false
	w := &Window{}
	v := Radio(RadioCfg{
		ID: "r3",
		OnClick: func(ctx EventCtx) {
			fired = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.OnClick == nil {
		t.Fatal("expected OnClick")
	}
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnClick(EventCtx{&layout, e, w})
	if !fired {
		t.Error("OnClick did not fire")
	}
}

func TestRadioLabelUsesTextStyleLabel(t *testing.T) {
	custom := TextStyle{
		Color: RGBA(200, 40, 40, 255),
		Size:  31,
	}
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{
			ID:             "r_label_style",
			Label:          "Option A",
			TextStyleLabel: custom,
			OnClick:        noop,
		}), w)
	if len(layout.Children) < 2 {
		t.Fatal("expected trailing label child")
	}
	tc := layout.Children[1].Children[0].Shape.TC
	if tc == nil || tc.TextStyle == nil {
		t.Fatal("trailing label has no text style")
	}
	if tc.TextStyle.Color != custom.Color {
		t.Errorf("label color = %+v, want %+v",
			tc.TextStyle.Color, custom.Color)
	}
	if tc.TextStyle.Size != custom.Size {
		t.Errorf("label size = %f, want %f",
			tc.TextStyle.Size, custom.Size)
	}
}

func TestRadioLabelDefaultsToTheme(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{
			ID:      "r_label_default",
			Label:   "Option A",
			OnClick: noop,
		}), w)
	if len(layout.Children) < 2 {
		t.Fatal("expected trailing label child")
	}
	tc := layout.Children[1].Children[0].Shape.TC
	if tc == nil || tc.TextStyle == nil {
		t.Fatal("trailing label has no text style")
	}
	if *tc.TextStyle != defaultRadioStyle.textStyleLabel {
		t.Errorf("label style = %+v, want theme %+v",
			*tc.TextStyle, defaultRadioStyle.textStyleLabel)
	}
}

func TestRadioFocusablePassthrough(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{ID: "r4", OnClick: noop}), w)
	if !layout.Shape.Focusable {
		t.Error("Focusable: want true")
	}
	if layout.Shape.ID != "r4" {
		t.Errorf("ID: got %q, want r4", layout.Shape.ID)
	}
}
