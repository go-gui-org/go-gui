package gui

import "testing"

// lookList is a 200x100 scrollable column holding rows 20 px tall, with a
// vertical scrollbar drawn by hooks. The thumb is a face that fills the
// thumb and centers a 4x4 grip; the track is a gutter that fills the bar.
// The last state each hook got is stored.
func lookList(rows int, thumb, track *ScrollbarState) func(*Window) View {
	return func(*Window) View {
		content := make([]View, rows)
		for i := range content {
			content[i] = Rectangle(RectangleCfg{Height: 20, Sizing: FillFixed})
		}
		return Column(ContainerCfg{
			ID:         "list",
			Width:      200,
			Height:     100,
			Sizing:     FixedFixed,
			Scrollable: true,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Spacing:    SomeF(0),
			ScrollbarCfgY: &ScrollbarCfg{
				Thumb: func(s ScrollbarState) View {
					*thumb = s
					return Column(ContainerCfg{ID: "face", Sizing: FillFill,
						Padding: NoPadding, SizeBorder: NoBorder,
						HAlign: HAlignCenter, VAlign: VAlignMiddle,
						Content: []View{Rectangle(RectangleCfg{ID: "grip",
							Width: 4, Height: 4, Sizing: FixedFixed})}})
				},
				Track: func(s ScrollbarState) View {
					*track = s
					return Column(ContainerCfg{ID: "gutter", Sizing: FillFill,
						Padding: NoPadding, SizeBorder: NoBorder})
				},
			},
			Content: content,
		})
	}
}

func lookFind(t *testing.T, w *Window, id string) *Layout {
	t.Helper()
	ly, ok := w.layout.FindLayout(func(n Layout) bool { return n.Shape.idKey() == id })
	if !ok {
		t.Fatalf("no shape %q", id)
	}
	return ly
}

// The first pass sizes the hook views to their content; the pass it asks
// for, which one frame runs, builds them at the thumb's size. The face
// then fills the thumb, the grip sits in its middle, and the state
// carries that size.
func TestScrollbarThumbHookFillsThumb(t *testing.T) {
	var thumb, track ScrollbarState
	w := NewTestWindow(WindowCfg{})
	w.TestRender(lookList(20, &thumb, &track))
	if w.refreshLayout {
		t.Fatal("sizes still changing after the second pass")
	}

	bar := lookFind(t, w, "list:scrollbar-y")
	face := lookFind(t, w, "list:scrollbar-y:face").Shape
	grip := lookFind(t, w, "list:scrollbar-y:face:grip").Shape
	gutter := lookFind(t, w, "list:scrollbar-y:gutter").Shape
	box := bar.Children[1].Shape

	if box.Height <= 0 || box.Height >= bar.Shape.Height {
		t.Fatalf("thumb height %v, want between 0 and bar %v", box.Height, bar.Shape.Height)
	}
	if face.X != box.X || face.Y != box.Y || face.Width != box.Width || face.Height != box.Height {
		t.Fatalf("face %v,%v %vx%v, want thumb %v,%v %vx%v",
			face.X, face.Y, face.Width, face.Height, box.X, box.Y, box.Width, box.Height)
	}
	if grip.X != box.X+(box.Width-4)/2 || grip.Y != box.Y+(box.Height-4)/2 {
		t.Fatalf("grip at %v,%v, want centered in thumb %v,%v %vx%v",
			grip.X, grip.Y, box.X, box.Y, box.Width, box.Height)
	}
	if thumb.Width != box.Width || thumb.Height != box.Height || !thumb.Vertical {
		t.Fatalf("thumb state %+v, want size %vx%v, vertical", thumb, box.Width, box.Height)
	}
	if gutter.X != bar.Shape.X || gutter.Y != bar.Shape.Y ||
		gutter.Width != bar.Shape.Width || gutter.Height != bar.Shape.Height {
		t.Fatalf("gutter %v,%v %vx%v, want bar %v,%v %vx%v",
			gutter.X, gutter.Y, gutter.Width, gutter.Height,
			bar.Shape.X, bar.Shape.Y, bar.Shape.Width, bar.Shape.Height)
	}
	if track.Width != bar.Shape.Width || track.Height != bar.Shape.Height {
		t.Fatalf("track state %+v, want bar size", track)
	}
	if found := w.TestFindings(DebugAll); len(found) != 0 {
		t.Fatalf("findings: %v", found)
	}
}

// Scrolling moves the thumb, and the hook view's children move with it.
func TestScrollbarThumbHookFollowsScroll(t *testing.T) {
	var thumb, track ScrollbarState
	w := NewTestWindow(WindowCfg{})
	w.TestRender(lookList(20, &thumb, &track))
	w.TestRender(nil)
	top := lookFind(t, w, "list:scrollbar-y").Children[1].Shape.Y

	if err := w.TestScroll("list", 0, -100); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	box := lookFind(t, w, "list:scrollbar-y").Children[1].Shape
	grip := lookFind(t, w, "list:scrollbar-y:face:grip").Shape
	if box.Y <= top {
		t.Fatalf("thumb y %v after scroll, want below %v", box.Y, top)
	}
	// A tolerance: the shift adds a delta to a position the layout pass
	// computed, so float rounding differs in the last digit.
	if d := grip.Y - (box.Y + (box.Height-4)/2); d > 0.01 || d < -0.01 {
		t.Fatalf("grip y %v, want %v: children must move with the thumb", grip.Y, box.Y+(box.Height-4)/2)
	}
}

// With nothing to scroll the hook thumb is hidden, subtree and all; the
// track stays, as the stock background does.
func TestScrollbarThumbHookHiddenWithoutOverflow(t *testing.T) {
	var thumb, track ScrollbarState
	w := NewTestWindow(WindowCfg{})
	w.TestRender(lookList(2, &thumb, &track))
	w.TestRender(nil)
	box := lookFind(t, w, "list:scrollbar-y").Children[1].Shape
	if box.shapeType != shapeNone || box.Width != 0 || box.Height != 0 || !box.Clip {
		t.Fatalf("thumb box type %v %vx%v clip %v, want hidden", box.shapeType, box.Width, box.Height, box.Clip)
	}
	lookFind(t, w, "list:scrollbar-y:gutter")
}

// Hover over the bar and a held press on the thumb reach the hooks, and a
// drag on the thumb scrolls.
func TestScrollbarThumbHookHoverPressDrag(t *testing.T) {
	var thumb, track ScrollbarState
	w := NewTestWindow(WindowCfg{})
	w.TestRender(lookList(20, &thumb, &track))
	w.TestRender(nil)
	box := lookFind(t, w, "list:scrollbar-y").Children[1].Shape
	x, y := box.X+box.Width/2, box.Y+box.Height/2

	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	w.TestRender(nil)
	w.TestRender(nil)
	if !thumb.Hovered || !track.Hovered {
		t.Fatalf("hover: thumb %+v track %+v", thumb, track)
	}

	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y})
	w.TestRender(nil)
	if !thumb.Pressed {
		t.Fatal("press on the thumb: Pressed false")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y + 20, MouseDY: 20})
	w.TestRender(nil)
	if _, oy, err := w.TestScrollOffset("list"); err != nil || oy >= 0 {
		t.Fatalf("drag: offset %v err %v, want scrolled", oy, err)
	}
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: x, MouseY: y + 20})
	w.TestRender(nil)
	if thumb.Pressed {
		t.Fatal("after release: Pressed still true")
	}
}

// Hover does not repaint a hook thumb with ColorThumb.
func TestScrollbarOnHoverLeavesHookThumbColor(t *testing.T) {
	layout := Layout{
		Shape:    &Shape{},
		Children: []Layout{{Shape: &Shape{}}},
	}
	cfg := ScrollbarCfg{ColorThumb: RGB(255, 0, 0), Overflow: scrollbarOnHover,
		Thumb: func(ScrollbarState) View { return nil }}
	makeScrollbarOnHover(cfg, 0)(EventCtx{&layout, nil, &Window{}})
	if layout.Children[0].Shape.Color.IsSet() {
		t.Fatalf("thumb color %v, want unset", layout.Children[0].Shape.Color)
	}
}
