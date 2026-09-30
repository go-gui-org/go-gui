package gui

import (
	"math"
	"testing"
)

// igTestView is a group with one segment of each kind.
func igTestView(disabled bool) func(*Window) View {
	return func(*Window) View {
		return InputGroup(InputGroupCfg{
			ID:       "grp",
			Disabled: disabled,
			Segments: []InputGroupSegment{
				InputGroupText(InputGroupTextCfg{Text: "@"}),
				InputGroupInput(InputCfg{
					ID: "user", Label: "User", Text: "name",
				}),
				{}, // the zero segment is skipped, with no divider
				InputGroupSelect(SelectCfg{
					ID:      "dom",
					Options: []SelectOption{NewSelectOption("x", "x")},
				}),
				// OnClick matters: Button skips its focus look
				// when it has no click handler.
				InputGroupButton(ButtonCfg{
					ID:      "go",
					Content: []View{Text(TextCfg{Text: "Go"})},
					OnClick: func(ctx EventCtx) { ctx.Consume() },
				}),
			},
		})
	}
}

func igFind(t *testing.T, w *Window, id string) *Layout {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("no shape %q; frame has %v", id, w.EffectiveIDs())
	}
	return ly
}

// The segments draw no border and no radius: the group owns the only
// outline, so no seam doubles (issue #820).
func TestInputGroupSegmentsHaveNoBorderOrRadius(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(igTestView(false))

	grp := igFind(t, w, "grp")
	if grp.Shape.SizeBorder <= 0 || grp.Shape.Radius <= 0 {
		t.Errorf("group border %v radius %v; want the theme's input values",
			grp.Shape.SizeBorder, grp.Shape.Radius)
	}
	if !grp.Shape.clipContents {
		t.Error("group does not clip its segments to its rounded area")
	}
	for _, id := range []string{"grp:user", "grp:dom", "grp:go"} {
		s := igFind(t, w, id).Shape
		if s.SizeBorder != 0 || s.Radius != 0 {
			t.Errorf("%s: border %v radius %v, want 0 and 0",
				id, s.SizeBorder, s.Radius)
		}
		if s.Sizing.Height != sizingFill {
			t.Errorf("%s: height sizing %v, want fill", id, s.Sizing.Height)
		}
	}
	// Four segments, three dividers: the zero segment adds neither.
	if n := len(grp.Children); n != 7 {
		t.Errorf("group has %d children, want 7", n)
	}
}

// A Label cannot render inside the row, so it names the segment for a
// screen reader instead.
func TestInputGroupInputLabelBecomesA11YLabel(t *testing.T) {
	seg := InputGroupInput(InputCfg{ID: "user", Label: "User"})
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View {
		return InputGroup(InputGroupCfg{
			ID: "grp", Segments: []InputGroupSegment{seg},
		})
	})
	s := igFind(t, w, "grp:user").Shape
	if s.a11Y == nil || s.a11Y.Label != "User" {
		t.Errorf("a11y %+v, want label User", s.a11Y)
	}
}

// Focus on any segment lights the group; the focused segment itself
// shows no ring, because the group clips it.
func TestInputGroupFocusWithin(t *testing.T) {
	for _, id := range []string{"grp:user", "grp:dom", "grp:go"} {
		t.Run(id, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(igTestView(false))
			w.SetFocus(id)
			w.TestRender(igTestView(false))

			grp := igFind(t, w, "grp").Shape
			if grp.fx == nil || grp.fx.Shadow == nil {
				t.Error("group has no focus ring")
			}
			if grp.ColorBorder != defaultInputStyle.Colors.BorderFocus {
				t.Errorf("group border %v, want the focus border %v",
					grp.ColorBorder, defaultInputStyle.Colors.BorderFocus)
			}
			seg := igFind(t, w, id).Shape
			if seg.fx != nil && seg.fx.Shadow != nil {
				t.Error("focused segment draws its own ring")
			}
		})
	}
}

func TestInputGroupNoFocusNoRing(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(igTestView(false))
	grp := igFind(t, w, "grp").Shape
	if grp.fx != nil && grp.fx.Shadow != nil {
		t.Error("unfocused group has a ring")
	}
	if grp.ColorBorder != defaultInputStyle.Colors.Border {
		t.Errorf("group border %v, want %v",
			grp.ColorBorder, defaultInputStyle.Colors.Border)
	}
}

// A disabled group disables every segment and shows no focus.
func TestInputGroupDisabled(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(igTestView(true))
	for _, id := range []string{"grp", "grp:user", "grp:dom", "grp:go"} {
		if !igFind(t, w, id).Shape.Disabled {
			t.Errorf("%s not disabled", id)
		}
	}
}

// The group is not a tab stop; its segments keep their own.
func TestInputGroupNotFocusable(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(igTestView(false))
	if igFind(t, w, "grp").Shape.Focusable {
		t.Error("group is focusable")
	}
	for _, id := range []string{"grp:user", "grp:dom", "grp:go"} {
		if !igFind(t, w, id).Shape.Focusable {
			t.Errorf("%s lost its focus", id)
		}
	}
	if f := w.TestDuplicateIDs(); len(f) != 0 {
		t.Errorf("duplicate IDs: %v", f)
	}
}

// A Label is prose, not an ID path: a colon in it is part of the name.
// a11yLabel would keep only the text after the last IDSep.
func TestInputGroupLabelKeepsColon(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View {
		return InputGroup(InputGroupCfg{
			ID: "grp", Segments: []InputGroupSegment{
				InputGroupInput(InputCfg{ID: "in", Label: "Price: USD"}),
				InputGroupSelect(SelectCfg{ID: "sel", Label: "Unit: kg"}),
			},
		})
	})
	for id, want := range map[string]string{
		"grp:in": "Price: USD", "grp:sel": "Unit: kg",
	} {
		s := igFind(t, w, id).Shape
		if s.a11Y == nil || s.a11Y.Label != want {
			t.Errorf("%s: a11y %+v, want label %q", id, s.a11Y, want)
		}
	}
}

// A NaN or negative border or radius falls back to none. The divider
// keeps its 1px floor, so the segments still read as separate.
func TestInputGroupNonFiniteBorderAndRadius(t *testing.T) {
	nan := float32(math.NaN())
	for _, v := range []float32{nan, float32(math.Inf(1)), -3} {
		w := NewTestWindow(t, WindowCfg{})
		w.TestRender(func(*Window) View {
			return InputGroup(InputGroupCfg{
				ID:         "grp",
				SizeBorder: BorderPx(v),
				Radius:     RadiusPx(v),
				Segments: []InputGroupSegment{
					InputGroupText(InputGroupTextCfg{Text: "@"}),
					InputGroupInput(InputCfg{ID: "user"}),
				},
			})
		})
		grp := igFind(t, w, "grp")
		if grp.Shape.SizeBorder != 0 || grp.Shape.Radius != 0 {
			t.Errorf("%v: border %v radius %v, want 0 and 0",
				v, grp.Shape.SizeBorder, grp.Shape.Radius)
		}
		if n := len(grp.Children); n != 3 {
			t.Fatalf("%v: group has %d children, want 3", v, n)
		}
		if dw := grp.Children[1].Shape.Width; dw != 1 {
			t.Errorf("%v: divider width %v, want 1", v, dw)
		}
	}
}

// The group forces the segment height to fill but keeps the caller's
// width, and a Button keeps a Padding the caller set.
func TestInputGroupKeepsCallerWidthAndPadding(t *testing.T) {
	pad := PadAll(3)
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View {
		return InputGroup(InputGroupCfg{
			ID:     "grp",
			Sizing: FillFit,
			Segments: []InputGroupSegment{
				InputGroupInput(InputCfg{ID: "user", Sizing: FillFit}),
				InputGroupButton(ButtonCfg{
					ID:      "go",
					Padding: pad,
					Content: []View{Text(TextCfg{Text: "Go"})},
				}),
			},
		})
	})
	user := igFind(t, w, "grp:user").Shape
	if user.Sizing.Width != sizingFill || user.Sizing.Height != sizingFill {
		t.Errorf("input sizing %+v, want fill width and height", user.Sizing)
	}
	btn := igFind(t, w, "grp:go").Shape
	if btn.Padding != pad {
		t.Errorf("button padding %+v, want %+v", btn.Padding, pad)
	}
}
