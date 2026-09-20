package gui

import "testing"

// Widget-level regressions for ColorSet.pick (#690).
//
// The golden files record what these states look like; these tests
// record what they mean, and they name the bug each one closes. They
// drive the real pipeline — a mouse event, then a settled frame, which
// is where layoutHover runs — rather than calling a handler directly,
// so the disabled short-circuit in layoutHoverDepth is exercised too.

// pickProbeColors gives each state a color nothing else can produce, so
// a wrong slot cannot pass by matching the theme.
var pickProbeColors = ColorSet{
	Base:        RGBA(10, 10, 10, 255),
	Hover:       RGBA(40, 200, 40, 255),
	Click:       RGBA(200, 40, 40, 255),
	Focus:       RGBA(40, 40, 200, 255),
	Border:      RGBA(60, 60, 60, 255),
	BorderFocus: RGBA(200, 200, 40, 255),
}

// childShape returns the first child of the shape with effective ID id.
// Toggle, Switch and Radio paint their state onto the box, pill or
// circle, not onto the focusable row that carries the ID.
func childShape(t *testing.T, w *Window, id string) *Shape {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("no shape with effective ID %s", id)
	}
	if len(ly.Children) == 0 {
		t.Fatalf("%s has no children", id)
	}
	return ly.Children[0].Shape
}

// TestPickFocusedAndHoveredShowsHoverFill is the precedence decision,
// asserted where it is visible. The fill goes to the pointer; the
// border stays with focus. Before #690 Button kept its focus fill while
// hovered, which reads as stuck after a click focuses it.
func TestPickFocusedAndHoveredShowsHoverFill(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return Button(ButtonCfg{
			ID: "b", Width: 100, Height: 40,
			OnClick: func(EventCtx) {},
			Colors:  pickProbeColors,
		})
	})
	w.SetFocus("b")
	hoverOver(t, w, "b")

	sh := mustShape(t, w, "b")
	if sh.Color != pickProbeColors.Hover {
		t.Errorf("focused and hovered: fill = %+v, want the hover color %+v",
			sh.Color, pickProbeColors.Hover)
	}
	if sh.ColorBorder != pickProbeColors.BorderFocus {
		t.Errorf("focused and hovered: border = %+v, want the focus "+
			"border %+v — hovering must not drop the focus affordance",
			sh.ColorBorder, pickProbeColors.BorderFocus)
	}
}

// pointerAway parks the pointer clear of every widget and settles a
// frame. The pointer starts at the origin, which in a test window is
// on top of the widget under test, so a "not hovered" assertion has to
// move it first — the same trap rule 7 of
// docs/specs/build-time-interaction-state.md describes.
func pointerAway(w *Window) {
	w.EventFn(&Event{Type: EventMouseMove, MouseX: 5000, MouseY: 5000})
	w.settle()
}

// TestPickFocusedNotHoveredShowsFocusFill is the other half: the focus
// fill is not dead, it is what a control reached by the keyboard looks
// like.
func TestPickFocusedNotHoveredShowsFocusFill(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return Button(ButtonCfg{
			ID: "b", Width: 100, Height: 40,
			OnClick: func(EventCtx) {},
			Colors:  pickProbeColors,
		})
	})
	w.SetFocus("b")
	pointerAway(w)

	sh := mustShape(t, w, "b")
	if sh.Color != pickProbeColors.Focus {
		t.Errorf("focused, pointer elsewhere: fill = %+v, want %+v",
			sh.Color, pickProbeColors.Focus)
	}
}

// TestPickPressedBeatsFocusAndHover pins the top of the fill rule:
// whatever else is true, a held button shows the pressed color.
func TestPickPressedBeatsFocusAndHover(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return Button(ButtonCfg{
			ID: "b", Width: 100, Height: 40,
			OnClick: func(EventCtx) {},
			Colors:  pickProbeColors,
		})
	})
	w.SetFocus("b")
	x, y := hoverOver(t, w, "b")
	pressAt(w, MouseLeft, x, y)

	if c := mustShape(t, w, "b").Color; c != pickProbeColors.Click {
		t.Errorf("focused, hovered and held: fill = %+v, want %+v",
			c, pickProbeColors.Click)
	}
	releaseAt(w, MouseLeft, x, y)
	if c := mustShape(t, w, "b").Color; c != pickProbeColors.Hover {
		t.Errorf("after release: fill = %+v, want the hover color %+v",
			c, pickProbeColors.Hover)
	}
}

// TestPickHeldSpaceShowsClick pins #658: a held Space is a press, and
// the amend pass is the only pass that can see it.
func TestPickHeldSpaceShowsClick(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return Button(ButtonCfg{
			ID: "b", Width: 100, Height: 40,
			OnClick: func(EventCtx) {},
			Colors:  pickProbeColors,
		})
	})
	w.SetFocus("b")
	pointerAway(w)
	// Setting the press target by hand invalidates nothing, and settle
	// is a no-op without a pending layout — so ask for the pass that
	// re-runs buttonAmendLayout.
	w.viewState.keyPressTargetID = "b"
	w.InvalidateLayout()
	w.settle()

	if c := mustShape(t, w, "b").Color; c != pickProbeColors.Click {
		t.Errorf("held Space: fill = %+v, want %+v",
			c, pickProbeColors.Click)
	}
}

// TestPickDisabledIgnoresPointer is the bug history, once, for every
// widget that now shares the picker. It was fixed one widget at a time
// in e5ed61d9 (button), cd9a8842 (toggle, radio), 6274186c (switch) and
// 89cbf85a (listbox), each time with another local guard.
func TestPickDisabledIgnoresPointer(t *testing.T) {
	tests := []struct {
		name string
		// build returns the widget; onChild says whether the state is
		// painted on the ID-bearing row or on its first child.
		build   func() View
		onChild bool
	}{
		{"button", func() View {
			return Button(ButtonCfg{
				ID: "w", Width: 100, Height: 40, Disabled: true,
				OnClick: func(EventCtx) {}, Colors: pickProbeColors,
			})
		}, false},
		{"toggle", func() View {
			return Toggle(ToggleCfg{
				ID: "w", Label: "x", Disabled: true,
				OnClick: func(EventCtx) {}, Colors: pickProbeColors,
			})
		}, true},
		{"switch", func() View {
			return Switch(SwitchCfg{
				ID: "w", Label: "x", Disabled: true,
				OnClick: func(EventCtx) {}, Colors: pickProbeColors,
			})
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := NewTestWindow(WindowCfg{})
			w.TestRender(func(*Window) View { return tc.build() })

			shape := mustShape(t, w, "w")
			if tc.onChild {
				shape = childShape(t, w, "w")
			}
			before := shape.Color
			hoverOver(t, w, "w")

			after := mustShape(t, w, "w")
			if tc.onChild {
				after = childShape(t, w, "w")
			}
			if after.Color != before {
				t.Errorf("disabled and hovered: fill = %+v, want the "+
					"resting %+v", after.Color, before)
			}
			if after.Color == pickProbeColors.Hover {
				t.Error("disabled widget took the hover color")
			}
		})
	}
}

// TestPickRadioPaintsBorderNotFill pins the one widget whose channels
// are crossed on purpose: Radio's fill is its selection, so every
// interaction state lands on the circle's border and pick's fill return
// is what carries it.
func TestPickRadioPaintsBorderNotFill(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return Radio(RadioCfg{
			ID: "r", Label: "x",
			OnClick: func(EventCtx) {}, Colors: pickProbeColors,
		})
	})
	fillBefore := childShape(t, w, "r").Color
	hoverOver(t, w, "r")

	circle := childShape(t, w, "r")
	if circle.ColorBorder != pickProbeColors.Hover {
		t.Errorf("hovered: border = %+v, want the hover color %+v",
			circle.ColorBorder, pickProbeColors.Hover)
	}
	if circle.Color != fillBefore {
		t.Errorf("hovered: fill = %+v, want it untouched at %+v",
			circle.Color, fillBefore)
	}
}

// TestAmendRunsBeforeHover pins the ordering the two-pass design rests
// on. buttonAmendLayout picks without the pointer and buttonOnHover
// re-picks with it, which is only correct while hover runs second.
// Nothing else asserts this.
func TestAmendRunsBeforeHover(t *testing.T) {
	var order []string
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return Button(ButtonCfg{
			ID: "b", Width: 100, Height: 40,
			OnClick: func(EventCtx) {},
			AmendLayout: func(EventCtx) {
				order = append(order, "amend")
			},
			OnHover: func(EventCtx) {
				order = append(order, "hover")
			},
		})
	})
	order = order[:0]
	hoverOver(t, w, "b")

	var amendAt, hoverAt = -1, -1
	for i, step := range order {
		if step == "amend" && amendAt < 0 {
			amendAt = i
		}
		if step == "hover" && hoverAt < 0 {
			hoverAt = i
		}
	}
	if amendAt < 0 || hoverAt < 0 {
		t.Fatalf("order = %v, want both passes to run", order)
	}
	if amendAt > hoverAt {
		t.Errorf("order = %v, want amend before hover", order)
	}
}
