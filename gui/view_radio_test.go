package gui

import (
	"math"
	"testing"
)

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

// A selected radio reads as a radio only with a dot inside the ring: a
// solid accent disc alone looks like a status light.
func TestRadioSelectedDrawsCenterDot(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{ID: "r_dot", Selected: true, OnClick: noop}), w)
	disc := layout.Children[0]
	if disc.Shape.Color != guiTheme.ColorSelect {
		t.Errorf("disc color: got %v, want ColorSelect", disc.Shape.Color)
	}
	if len(disc.Children) != 1 {
		t.Fatalf("selected disc children: got %d, want 1 dot",
			len(disc.Children))
	}
	dot := disc.Children[0].Shape
	if dot.shapeType != shapeCircle {
		t.Errorf("dot shape: got %v, want circle", dot.shapeType)
	}
	if dot.Color != guiTheme.ColorTextOnSelect {
		t.Errorf("dot color: got %v, want ColorTextOnSelect", dot.Color)
	}
	if dot.Width <= 0 || dot.Width >= disc.Shape.Width {
		t.Errorf("dot width %v not inside disc width %v",
			dot.Width, disc.Shape.Width)
	}
}

// An unselected radio is an empty well, like Checkbox: no dot, and the
// panel fill rather than the darker active grey that read as a filled
// grey dot.
func TestRadioUnselectedIsEmptyWell(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{ID: "r_well", OnClick: noop}), w)
	disc := layout.Children[0]
	if len(disc.Children) != 0 {
		t.Errorf("unselected disc children: got %d, want 0",
			len(disc.Children))
	}
	if disc.Shape.Color != defaultToggleStyle.Colors.Base {
		t.Errorf("unselected fill: got %v, want checkbox well %v",
			disc.Shape.Color, defaultToggleStyle.Colors.Base)
	}
}

// A caller's light ColorSelect must not get the theme's white dot: the
// dot would vanish into the disc. The dot follows the caller's fill.
func TestRadioDotContrastsCustomColorSelect(t *testing.T) {
	w := &Window{}
	light := RGB(255, 230, 80)
	layout := generateViewLayout(
		Radio(RadioCfg{ID: "r_light", Selected: true,
			ColorSelect: light, OnClick: noop}), w)
	dot := layout.Children[0].Children[0].Shape
	if dot.Color != textOnFor(light) {
		t.Errorf("dot on light fill: got %v, want %v",
			dot.Color, textOnFor(light))
	}
}

// With no border, a panel-colored well on a panel draws nothing, so a
// borderless theme keeps the darker ColorActive fill for the off state.
func TestRadioBorderlessUnselectedStaysVisible(t *testing.T) {
	th := ThemeDark.WithBorders(false)
	if th.radioStyle.colorUnselect == th.ColorPanel {
		t.Errorf("borderless unselected fill equals ColorPanel %v: "+
			"radio vanishes on a panel", th.ColorPanel)
	}
	if got := th.WithColors(ColorOverrides{}).radioStyle.colorUnselect; got ==
		th.ColorPanel {
		t.Errorf("WithColors borderless unselected fill = ColorPanel %v", got)
	}
}

// A non-finite or negative Size must not reach the disc or the dot: a
// NaN rect poisons every later f32Max in arrange. Both fall back to
// the theme size.
func TestRadioBadSizeFallsBackToTheme(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, bad := range []float32{nan, inf, -inf, -5} {
		w := &Window{}
		layout := generateViewLayout(
			Radio(RadioCfg{ID: "r_bad", Selected: true,
				Size: Some(bad), OnClick: noop}), w)
		disc := layout.Children[0].Shape
		dot := layout.Children[0].Children[0].Shape
		if disc.Width != defaultRadioStyle.Size {
			t.Errorf("Size %v: disc width %v, want theme %v",
				bad, disc.Width, defaultRadioStyle.Size)
		}
		if !f32IsFinite(dot.Width) || dot.Width <= 0 {
			t.Errorf("Size %v: dot width %v", bad, dot.Width)
		}
	}
}
