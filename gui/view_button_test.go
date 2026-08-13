package gui

import "testing"

func TestButtonGeneratesLayout(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{ID: "b1"})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
	if layout.Shape.ID != "b1" {
		t.Errorf("ID: got %s, want b1", layout.Shape.ID)
	}
	if layout.Shape.A11YRole != AccessRoleButton {
		t.Errorf("a11y role: got %d, want %d",
			layout.Shape.A11YRole, AccessRoleButton)
	}
}

func TestButtonOnClickFires(t *testing.T) {
	fired := false
	w := &Window{}
	v := Button(ButtonCfg{
		ID: "b2",
		OnClick: func(ctx EventCtx) {
			fired = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.OnClick == nil {
		t.Fatal("expected OnClick handler")
	}
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnClick(EventCtx{&layout, e, w})
	if !fired {
		t.Error("OnClick did not fire")
	}
}

func TestButtonDisabledFlag(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{ID: "b3", Disabled: true})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

func TestButtonFocusable(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{ID: "b4"})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Focusable {
		t.Error("Focusable: want true")
	}
	if layout.Shape.ID != "b4" {
		t.Errorf("ID: got %q, want b4", layout.Shape.ID)
	}
}

func TestButtonWithContent(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{
		ID:      "b5",
		Content: []View{Text(TextCfg{Text: "Click"})},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) == 0 {
		t.Error("expected children from content")
	}
}

func TestButtonNoOnClickNoHandler(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{ID: "b6"})
	layout := generateViewLayout(v, w)
	if layout.Shape.events != nil &&
		layout.Shape.events.OnClick != nil {
		t.Error("expected no OnClick without handler")
	}
}

func TestButtonAmendLayoutChains(t *testing.T) {
	w := &Window{}
	called := false
	v := Button(ButtonCfg{
		ID:      "b7",
		OnClick: func(ctx EventCtx) {},
		AmendLayout: func(ctx EventCtx) {
			called = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.AmendLayout == nil {
		t.Fatal("expected AmendLayout handler")
	}
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if !called {
		t.Error("AmendLayout did not fire")
	}
}

func TestButtonAmendLayoutNotCalledWhenDisabled(t *testing.T) {
	w := &Window{}
	called := false
	v := Button(ButtonCfg{
		ID:       "b8",
		Disabled: true,
		OnClick:  func(ctx EventCtx) {},
		AmendLayout: func(ctx EventCtx) {
			called = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.AmendLayout == nil {
		t.Fatal("expected AmendLayout handler")
	}
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if called {
		t.Error("AmendLayout should not fire when disabled")
	}
}

func TestButtonAmendLayoutSuppressedWhenNoOnClick(t *testing.T) {
	// When OnClick is nil, the button creates no event handlers at
	// all — AmendLayout cannot fire because it's never wired.
	w := &Window{}
	v := Button(ButtonCfg{
		ID:          "b9",
		AmendLayout: func(ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events != nil {
		t.Error("expected nil events when OnClick is nil")
	}
}

// TestButtonHoverPressedColor pins the pressed-while-hovered branch of
// buttonOnHover (view_button.go): a press-and-hold renders the click
// color, release falls back to the hover color. FocusDisabled keeps the
// press from taking focus — while focused, buttonAmendLayout paints the
// focus color and buttonOnHover deliberately skips the hover color, so
// the release assertion must observe the un-focused path.
func TestButtonHoverPressedColor(t *testing.T) {
	hover := RGBA(40, 200, 40, 255)
	click := RGBA(200, 40, 40, 255)
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Button(ButtonCfg{
			ID: "b", Width: 100, Height: 40,
			FocusDisabled: true,
			OnClick:       func(EventCtx) {},
			Colors: ColorSet{
				Base:  RGBA(10, 10, 10, 255),
				Hover: hover,
				Click: click,
			},
		})
	})

	x, y := hoverOver(t, w, "b")
	if c := mustShape(t, w, "b").Color; c != hover {
		t.Fatalf("hover: color = %+v, want %+v", c, hover)
	}
	pressAt(w, MouseLeft, x, y)
	if c := mustShape(t, w, "b").Color; c != click {
		t.Fatalf("held: color = %+v, want %+v", c, click)
	}
	releaseAt(w, MouseLeft, x, y)
	if c := mustShape(t, w, "b").Color; c != hover {
		t.Fatalf("released: color = %+v, want %+v", c, hover)
	}
}

// TestButtonHoverRightButtonStaysHoverColor pins D5: while the right
// button is held, hover events carry MouseRight and the == MouseLeft
// branch must not fire — the button keeps the hover color, as a
// right-button hold is not a press.
func TestButtonHoverRightButtonStaysHoverColor(t *testing.T) {
	hover := RGBA(40, 200, 40, 255)
	click := RGBA(200, 40, 40, 255)
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Button(ButtonCfg{
			ID: "b", Width: 100, Height: 40,
			FocusDisabled: true,
			OnClick:       func(EventCtx) {},
			Colors: ColorSet{
				Base:  RGBA(10, 10, 10, 255),
				Hover: hover,
				Click: click,
			},
		})
	})

	x, y := hoverOver(t, w, "b")
	if c := mustShape(t, w, "b").Color; c != hover {
		t.Fatalf("hover: color = %+v, want %+v", c, hover)
	}
	pressAt(w, MouseRight, x, y)
	if c := mustShape(t, w, "b").Color; c != hover {
		t.Fatalf("right held: color = %+v, want %+v", c, hover)
	}
	releaseAt(w, MouseRight, x, y)
}
