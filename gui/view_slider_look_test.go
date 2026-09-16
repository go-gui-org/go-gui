package gui

import "testing"

// lookSlider builds a 200x20 slider at 25 of 0..100 whose Look has a 6 px
// track, a 6 px fill and a 10x20 handle holding a 4x4 grip 3 px in. The
// state the Look got is stored in *got.
func lookSlider(value *float32, vertical bool, got *SliderLookState) func(*Window) View {
	return func(*Window) View {
		width, height := float32(200), float32(20)
		trackSizing, fillW, fillH := FillFixed, float32(0), float32(6)
		trackW, trackH := float32(0), float32(6)
		handleW, handleH := float32(10), float32(20)
		if vertical {
			width, height = height, width
			trackSizing, fillW, fillH = FixedFill, 6, 0
			trackW, trackH = 6, 0
			handleW, handleH = handleH, handleW
		}
		return Column(ContainerCfg{
			SizeBorder: NoBorder,
			Padding:    PadAll(10),
			Content: []View{Slider(SliderCfg{
				ID:       "vol",
				Value:    *value,
				Min:      0,
				Max:      100,
				Width:    width,
				Height:   height,
				Vertical: vertical,
				OnChange: func(v float32, ctx EventCtx) {
					*value = v
					ctx.Consume()
				},
				Look: func(s SliderLookState) SliderParts {
					*got = s
					return SliderParts{
						Track: Row(ContainerCfg{ID: "track", Width: trackW,
							Height: trackH, Sizing: trackSizing,
							SizeBorder: NoBorder, Padding: NoPadding}),
						Fill: Rectangle(RectangleCfg{ID: "fill", Width: fillW,
							Height: fillH, Sizing: FixedFixed}),
						Handle: Column(ContainerCfg{ID: "handle", Width: handleW,
							Height: handleH, Sizing: FixedFixed,
							SizeBorder: NoBorder, Padding: PadAll(3),
							Content: []View{Rectangle(RectangleCfg{ID: "grip",
								Width: 4, Height: 4, Sizing: FixedFixed})}}),
					}
				},
			})},
		})
	}
}

func lookShape(t *testing.T, w *Window, id string) *Shape {
	t.Helper()
	ly, ok := w.layout.FindLayout(func(n Layout) bool { return n.Shape.idKey() == id })
	if !ok {
		t.Fatalf("no shape %q", id)
	}
	return ly.Shape
}

// The track keeps its full width; Fill and Handle take no room from it.
// The handle center travels the track inset by half a handle, the fill
// ends at that center, and the handle's children move with it.
func TestSliderLookPlacesParts(t *testing.T) {
	value := float32(25)
	var got SliderLookState
	w := NewTestWindow(WindowCfg{})
	w.TestRender(lookSlider(&value, false, &got))

	slider := lookShape(t, w, "vol")
	track := lookShape(t, w, "vol:track")
	fill := lookShape(t, w, "vol:fill")
	handle := lookShape(t, w, "vol:handle")
	grip := lookShape(t, w, "vol:handle:grip")

	if track.Width != 200 || track.Height != 6 {
		t.Fatalf("track %vx%v, want 200x6", track.Width, track.Height)
	}
	if track.Y != slider.Y+7 {
		t.Fatalf("track y %v, want centered at %v", track.Y, slider.Y+7)
	}
	center := track.X + 5 + 190*0.25
	if handle.X != center-5 || handle.Y != slider.Y {
		t.Fatalf("handle at (%v, %v), want (%v, %v)", handle.X, handle.Y, center-5, slider.Y)
	}
	if grip.X != handle.X+3 || grip.Y != handle.Y+3 {
		t.Fatalf("grip at (%v, %v), want (%v, %v): children must move with the handle",
			grip.X, grip.Y, handle.X+3, handle.Y+3)
	}
	if fill.X != track.X || fill.Y != track.Y || fill.Width != center-track.X || fill.Height != 6 {
		t.Fatalf("fill %v,%v %vx%v, want %v,%v %vx6",
			fill.X, fill.Y, fill.Width, fill.Height, track.X, track.Y, center-track.X)
	}
	if got.Pct != 0.25 {
		t.Fatalf("Pct %v, want 0.25", got.Pct)
	}
}

func TestSliderLookPlacesPartsVertical(t *testing.T) {
	value := float32(50)
	var got SliderLookState
	w := NewTestWindow(WindowCfg{})
	w.TestRender(lookSlider(&value, true, &got))

	slider := lookShape(t, w, "vol")
	track := lookShape(t, w, "vol:track")
	fill := lookShape(t, w, "vol:fill")
	handle := lookShape(t, w, "vol:handle")

	if track.Height != 200 || track.X != slider.X+7 {
		t.Fatalf("track x %v h %v, want x %v h 200", track.X, track.Height, slider.X+7)
	}
	center := track.Y + 5 + 190*0.5
	if handle.Y != center-5 || handle.X != slider.X {
		t.Fatalf("handle at (%v, %v), want (%v, %v)", handle.X, handle.Y, slider.X, center-5)
	}
	if fill.Y != track.Y || fill.X != track.X || fill.Height != center-track.Y {
		t.Fatalf("fill %v,%v h %v, want %v,%v h %v",
			fill.X, fill.Y, fill.Height, track.X, track.Y, center-track.Y)
	}
}

// A press maps over the span the handle center travels: half a handle in
// from the track start is Min, and the drag keeps mapping the same way.
func TestSliderLookPressAndDrag(t *testing.T) {
	value := float32(25)
	var got SliderLookState
	w := NewTestWindow(WindowCfg{})
	w.TestRender(lookSlider(&value, false, &got))
	track := lookShape(t, w, "vol:track")
	y := track.Y + 3
	left := track.X + 5

	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: left + 2, MouseY: y})
	w.TestRender(nil)
	if value < 1.0 || value > 1.1 {
		t.Fatalf("press value %v, want 2/190 of 100", value)
	}
	if !got.Pressed {
		t.Fatal("Look did not see the press")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: left + 95, MouseY: y})
	w.TestRender(nil)
	if value != 50 {
		t.Fatalf("drag value %v, want 50", value)
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: track.X + 400, MouseY: y})
	if value != 100 {
		t.Fatalf("drag past the end: value %v, want 100", value)
	}
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: track.X + 400, MouseY: y})
}

// Keys, wheel and focus come from the stock slider.
func TestSliderLookKeysWheelFocus(t *testing.T) {
	value := float32(25)
	var got SliderLookState
	w := NewTestWindow(WindowCfg{})
	w.TestRender(lookSlider(&value, false, &got))

	if err := w.TestKey("vol", KeyEnd, ModNone); err != nil {
		t.Fatal(err)
	}
	if value != 100 {
		t.Fatalf("End: value %v, want 100", value)
	}
	w.TestRender(nil)
	if !got.Focused || got.Pct != 1 {
		t.Fatalf("state %+v, want Focused and Pct 1", got)
	}
	if err := w.TestScroll("vol", 0, -3); err != nil {
		t.Fatal(err)
	}
	if value != 97 {
		t.Fatalf("wheel: value %v, want 97", value)
	}
}

// A nil part is left out. With no track and no handle, the fill runs from
// the slider's start to the value over the whole width.
func TestSliderLookNilParts(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return Slider(SliderCfg{ID: "s", Value: 50, Width: 100, Height: 10,
			Look: func(SliderLookState) SliderParts {
				return SliderParts{Fill: Rectangle(RectangleCfg{ID: "f", Height: 4, Sizing: FixedFixed})}
			}})
	})
	s := lookShape(t, w, "s")
	f := lookShape(t, w, "s:f")
	if len(w.layout.Children) == 0 {
		t.Fatal("empty tree")
	}
	if f.X != s.X || f.Width != 50 || f.Y != s.Y+3 {
		t.Fatalf("fill %v,%v w %v, want %v,%v w 50", f.X, f.Y, f.Width, s.X, s.Y+3)
	}
	if found := w.TestFindings(DebugAll); len(found) != 0 {
		t.Fatalf("findings: %v", found)
	}
}
