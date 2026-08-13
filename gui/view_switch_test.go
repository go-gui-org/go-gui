package gui

import "testing"

func TestSwitchIDPassthrough(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{ID: "sw1", OnClick: noop}), w)
	// ID lives on the focusable outer row only; the inner pill must
	// not duplicate it.
	if layout.Shape.ID != "sw1" {
		t.Errorf("outer ID: got %s", layout.Shape.ID)
	}
	if len(layout.Children) == 0 {
		t.Fatal("expected children")
	}
	if layout.Children[0].Shape.ID != "" {
		t.Errorf("pill ID: got %s, want empty",
			layout.Children[0].Shape.ID)
	}
}

func TestSwitchUnselectedState(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{ID: "switch_test_test_switch_unselected_state", Selected: false, OnClick: noop}), w)
	if layout.Shape.A11YState != AccessStateNone {
		t.Error("unselected switch should have None state")
	}
}

func TestSwitchOnClickCallback(t *testing.T) {
	fired := false
	w := &Window{}
	v := Switch(SwitchCfg{
		ID: "switch_test_test_switch_on_click_callback",
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

func TestSwitchDisabledFlag(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{ID: "switch_test_test_switch_disabled_flag", Disabled: true, OnClick: noop}), w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

func TestSwitchLabelAddsChild(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{ID: "switch_test_test_switch_label_adds_child", Label: "Dark Mode", OnClick: noop}), w)
	if len(layout.Children) < 2 {
		t.Errorf("expected >= 2 children with label, got %d",
			len(layout.Children))
	}
}

// TestSwitchHoverPressedColor pins the pressed-while-hovered branch of
// the switch's OnHover (view_switch.go): a press-and-hold renders the
// click color on the pill (child 0), release falls back to the hover
// color. The switch takes focus on press; the hover pass runs after
// AmendLayout, so the pill's hover/click paint always wins over the
// focus paint.
func TestSwitchHoverPressedColor(t *testing.T) {
	hover := RGBA(40, 200, 40, 255)
	click := RGBA(200, 40, 40, 255)
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Switch(SwitchCfg{
			ID:      "sw",
			OnClick: func(EventCtx) {},
			Colors: ColorSet{
				Base:   RGBA(10, 10, 10, 255),
				Hover:  hover,
				Click:  click,
				Focus:  RGBA(10, 10, 10, 255),
				Border: RGBA(10, 10, 10, 255),
			},
		})
	})

	pill := func() *Shape {
		ly, ok := w.layout.FindByID("sw")
		if !ok {
			t.Fatal("no switch with effective ID sw")
		}
		if len(ly.Children) == 0 {
			t.Fatal("switch has no pill child")
		}
		return ly.Children[0].Shape
	}

	x, y := hoverOver(t, w, "sw")
	if c := pill().Color; c != hover {
		t.Fatalf("hover: color = %+v, want %+v", c, hover)
	}
	pressAt(w, MouseLeft, x, y)
	if c := pill().Color; c != click {
		t.Fatalf("held: color = %+v, want %+v", c, click)
	}
	releaseAt(w, MouseLeft, x, y)
	if c := pill().Color; c != hover {
		t.Fatalf("released: color = %+v, want %+v", c, hover)
	}
}
