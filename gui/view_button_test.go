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
	// A click-less button still carries handlers, because every button
	// wires the optical-centring correction (issue #346) — its label is
	// centred whether or not the button can be pressed. What stays
	// suppressed is the caller's own hook: buttonAmendLayout returns
	// before OnAmend when there is no OnClick.
	w := &Window{}
	called := false
	v := Button(ButtonCfg{
		ID:          "b9",
		Content:     []View{Text(TextCfg{Text: "Save"})},
		AmendLayout: func(ctx EventCtx) { called = true },
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.AmendLayout == nil {
		t.Fatal("expected the correction to be wired")
	}
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if called {
		t.Error("caller AmendLayout should not fire without OnClick")
	}
}

// The correction must not depend on the button being interactive: a
// disabled button's label sits beside an enabled one, and
// buttonAmendLayout returns early for both the disabled and the
// click-less case (issue #346).
func TestButtonOpticalCorrectionIgnoresButtonState(t *testing.T) {
	labelY := func(cfg ButtonCfg) float32 {
		w := &Window{}
		w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 19}
		cfg.ID = "b"
		cfg.Content = []View{Text(TextCfg{Text: "Save"})}
		layout := generateViewLayout(Button(cfg), w)
		layout.Children[0].Shape.Y = 100
		layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
		return layout.Children[0].Shape.Y
	}

	enabled := labelY(ButtonCfg{OnClick: func(EventCtx) {}})
	disabled := labelY(ButtonCfg{Disabled: true, OnClick: func(EventCtx) {}})
	clickless := labelY(ButtonCfg{})

	if enabled <= 100 {
		t.Fatalf("enabled label not corrected: y=%v, want > 100", enabled)
	}
	if disabled != enabled || clickless != enabled {
		t.Errorf("label y differs by state: enabled=%v disabled=%v "+
			"clickless=%v, want all equal", enabled, disabled, clickless)
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

// Variant fills come from the installed theme's derived styles; the
// zero-value variant is today's button (visual-refresh §6).
func TestButtonVariantFills(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Spacing: SomeF(8),
			Content: []View{
				Button(ButtonCfg{ID: "sec", OnClick: noop}),
				Button(ButtonCfg{ID: "pri", Variant: ButtonPrimary, OnClick: noop}),
				Button(ButtonCfg{ID: "gho", Variant: ButtonGhost, OnClick: noop}),
				Button(ButtonCfg{ID: "dng", Variant: ButtonDanger, OnClick: noop}),
			},
		})
	})
	if c := mustShape(t, w, "sec").Color; c != defaultButtonStyle.Color {
		t.Errorf("secondary fill = %v, want the plain button fill %v", c, defaultButtonStyle.Color)
	}
	if c := mustShape(t, w, "pri").Color; c != ThemeDark.ButtonStylePrimary.Color {
		t.Errorf("primary fill = %v, want the accent %v", c, ThemeDark.ButtonStylePrimary.Color)
	}
	if c := mustShape(t, w, "gho").Color; !c.IsSet() {
		t.Errorf("ghost fill = %v, want transparent", c)
	}
	if c := mustShape(t, w, "dng").Color; c != ThemeDark.ButtonStyleDanger.Color {
		t.Errorf("danger fill = %v, want the error color %v", c, ThemeDark.ButtonStyleDanger.Color)
	}
}

// Caller colors keep precedence over the variant, state by state: the
// Color shorthand sets the resting fill and leaves hover to the
// variant's ramp (visual-refresh §6).
func TestButtonVariantColorsKeepPrecedence(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{Button(ButtonCfg{ID: "ovr", Variant: ButtonPrimary, Color: Blue, OnClick: noop})},
		})
	})
	if c := mustShape(t, w, "ovr").Color; c != Blue {
		t.Errorf("resting fill = %v, want the assigned Blue", c)
	}
	ly, ok := w.layout.FindByID("ovr")
	if !ok || ly.Shape.bc == nil {
		t.Fatal("button colors record missing")
	}
	if c := ly.Shape.bc.ColorHover; c != ThemeDark.ButtonStylePrimary.ColorHover {
		t.Errorf("hover = %v, want the variant's accent hover %v", c, ThemeDark.ButtonStylePrimary.ColorHover)
	}
}

// The Label path builds the button's text with the variant's label
// color already applied — the one way a filled variant's label can be
// recolored at construction (visual-refresh §6).
func TestButtonVariantLabelPath(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Spacing: SomeF(8),
			Content: []View{
				Button(ButtonCfg{ID: "pri", Variant: ButtonPrimary, Label: "Save", OnClick: noop}),
				Button(ButtonCfg{ID: "gho", Variant: ButtonGhost, Label: "Discard", OnClick: noop}),
				Button(ButtonCfg{ID: "sec", Label: "Plain", OnClick: noop}),
			},
		})
	})
	if c := buttonTextColor(t, w, "pri"); c != ThemeDark.ColorTextOnAccent {
		t.Errorf("primary label = %v, want ColorTextOnAccent %v", c, ThemeDark.ColorTextOnAccent)
	}
	if c := buttonTextColor(t, w, "gho"); c != DefaultTextStyle.Color {
		t.Errorf("ghost label = %v, want the default text color", c)
	}
	if c := buttonTextColor(t, w, "sec"); c != DefaultTextStyle.Color {
		t.Errorf("secondary label = %v, want the default text color", c)
	}
}

// A Content-built label that took the default style is recolored by
// the filled variants' amend pass; an explicitly colored label is the
// caller's choice and is left alone. Only the filled variants stamp —
// the ghost keeps the default color (visual-refresh §6).
func TestButtonVariantRecolorsDefaultedLabel(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Spacing: SomeF(8),
			Content: []View{
				Button(ButtonCfg{ID: "pri", Variant: ButtonPrimary, OnClick: noop, Content: []View{
					Text(TextCfg{Text: "defaulted"}),
					Text(TextCfg{Text: "explicit", TextStyle: TextStyle{Color: Red}}),
				}}),
				Button(ButtonCfg{ID: "gho", Variant: ButtonGhost, OnClick: noop, Content: []View{
					Text(TextCfg{Text: "keep"}),
				}}),
			},
		})
	})
	if c := buttonTextColorAt(t, w, "pri", 0); c != ThemeDark.ColorTextOnAccent {
		t.Errorf("defaulted label = %v, want ColorTextOnAccent", c)
	}
	if c := buttonTextColorAt(t, w, "pri", 1); c != Red {
		t.Errorf("explicit label = %v, want Red — a caller's color must win", c)
	}
	if c := buttonTextColorAt(t, w, "gho", 0); c != DefaultTextStyle.Color {
		t.Errorf("ghost label = %v, want the default text color", c)
	}
}

// An out-of-range variant value degrades to the zero-value button
// rather than panic or render nothing.
func TestButtonVariantUnknownDegradesToSecondary(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Button(ButtonCfg{
				ID: "weird", Variant: ButtonVariant(255), OnClick: noop,
			})},
		})
	})
	if c := mustShape(t, w, "weird").Color; c != defaultButtonStyle.Color {
		t.Errorf("unknown variant fill = %v, want the plain button fill %v",
			c, defaultButtonStyle.Color)
	}
}

// Label and Content compose: the built label comes first, then the
// caller's content (an icon under the label, say) — Content is "used
// as-is" either way.
func TestButtonVariantLabelWithContent(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Button(ButtonCfg{
				ID:      "combo",
				Variant: ButtonPrimary,
				Label:   "Save",
				Content: []View{Text(TextCfg{Text: "extra"})},
				OnClick: noop,
			})},
		})
	})
	if got := buttonTextAt(t, w, "combo", 0); got != "Save" {
		t.Errorf("first text = %q, want the built label %q", got, "Save")
	}
	if got := buttonTextAt(t, w, "combo", 1); got != "extra" {
		t.Errorf("second text = %q, want the caller content %q", got, "extra")
	}
}

// buttonTextAt returns the text of the i-th text shape under the
// named shape.
func buttonTextAt(t *testing.T, w *Window, id string, i int) string {
	t.Helper()
	text, _ := textShapeAt(t, w, id, i)
	return text
}

// buttonTextColor returns the color of the first text shape under the
// named shape, or the zero color when there is none.
func buttonTextColor(t *testing.T, w *Window, id string) Color {
	t.Helper()
	return buttonTextColorAt(t, w, id, 0)
}

// buttonTextColorAt returns the color of the i-th text shape under the
// named shape.
func buttonTextColorAt(t *testing.T, w *Window, id string, i int) Color {
	t.Helper()
	_, c := textShapeAt(t, w, id, i)
	return c
}

// textShapeAt returns the text and color of the i-th text shape under
// the named shape.
func textShapeAt(t *testing.T, w *Window, id string, i int) (string, Color) {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("no shape with effective ID %s", id)
	}
	found := 0
	for j := range ly.Children {
		text, c, ok := textShapeIn(&ly.Children[j])
		if !ok {
			continue
		}
		if found == i {
			return text, c
		}
		found++
	}
	t.Fatalf("no %d-th text child under %s", i, id)
	return "", Color{}
}

// textShapeIn returns the text and color of the first text shape in
// the subtree rooted at l.
func textShapeIn(l *Layout) (string, Color, bool) {
	if l.Shape != nil && l.Shape.shapeType == shapeText &&
		l.Shape.TC != nil {
		c := Color{}
		if l.Shape.TC.TextStyle != nil {
			c = l.Shape.TC.TextStyle.Color
		}
		return l.Shape.TC.Text, c, true
	}
	for i := range l.Children {
		if text, c, ok := textShapeIn(&l.Children[i]); ok {
			return text, c, true
		}
	}
	return "", Color{}, false
}
