package gui

import (
	"math"
	"strconv"
	"testing"
)

// buildFuzzEventLayoutTree builds a random layout tree for event fuzzing.
// depth controls recursion depth; childCount controls width.
// Shapes alternate between having events, focus IDs, scroll IDs, and
// being plain containers to exercise different dispatch code paths.
func buildFuzzEventLayoutTree(depth, childCount int, w *Window) Layout {
	root := Layout{
		Shape: w.allocShape(Shape{}),
	}
	if depth <= 0 || childCount <= 0 {
		return root
	}
	root.Children = make([]Layout, childCount)
	for i := range childCount {
		shape := w.allocShape(Shape{})
		switch i % 6 {
		case 0:
			// Shape with events but no focus.
			shape.events = &eventHandlers{
				OnClick: func(_ *Layout, e *Event, _ *Window) {
					e.IsHandled = true
				},
			}
		case 1:
			// Shape with focus ID only.
			shape.Focusable = true
			shape.ID = "fa" + strconv.Itoa(i)
		case 2:
			// Shape with scroll container.
			shape.IDScroll = uint32(200 + i)
			shape.Width = 100
			shape.Height = 100
		case 3:
			// Shape with events and focus.
			shape.Focusable = true
			shape.ID = "fb" + strconv.Itoa(i)
			shape.events = &eventHandlers{
				OnKeyDown: func(_ *Layout, e *Event, _ *Window) {
					e.IsHandled = true
				},
				OnChar: func(_ *Layout, e *Event, _ *Window) {
					e.IsHandled = true
				},
			}
		case 4:
			// Shape with hover events.
			shape.events = &eventHandlers{
				OnHover: func(_ *Layout, _ *Event, _ *Window) {},
			}
		case 5:
			// Plain container (no events).
		}
		root.Children[i] = Layout{Shape: shape}
		if depth > 1 {
			sub := buildFuzzEventLayoutTree(depth-1, childCount/2, w)
			root.Children[i].Children = sub.Children
		}
	}
	return root
}

// clampEventCoord clamps a float32 to a reasonable window coordinate.
func clampEventCoord(v float32, maxVal float32) float32 {
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return 0
	}
	if v < 0 {
		v = -v
	}
	if v > maxVal {
		v = maxVal
	}
	return v
}

// FuzzEventDispatch fuzzes the event dispatch system with random event
// types and coordinates against randomly-generated layout trees. Verifies
// no panics occur during dispatch.
func FuzzEventDispatch(f *testing.F) {
	f.Add(uint8(2), uint8(5), uint32(0x01), float32(400), float32(300))
	f.Add(uint8(0), uint8(1), uint32(0x02), float32(0), float32(0))
	f.Add(uint8(3), uint8(10), uint32(0x04), float32(800), float32(600))
	f.Add(uint8(1), uint8(20), uint32(0x08), float32(1920), float32(1080))
	f.Add(uint8(4), uint8(3), uint32(0x10), float32(-1), float32(-1))

	f.Fuzz(func(t *testing.T, depth, childCount uint8, eventType uint32, mouseX, mouseY float32) {
		d := int(depth % 5)
		n := int(childCount % 15)
		et := EventType(eventType % 26) // covers all defined EventType values

		mx := clampEventCoord(mouseX, 4096)
		my := clampEventCoord(mouseY, 4096)

		w := &Window{
			scratch:      newScratchPools(),
			focused:      true,
			windowWidth:  1024,
			windowHeight: 768,
		}

		eventLayout := buildFuzzEventLayoutTree(d, n, w)
		w.layout = eventLayout

		// Set focus to a random child if one exists with IDFocus.
		if got, ok := FindLayoutByFocusID(&w.layout, "fa0"); ok {
			w.SetFocus(got.Shape.ID)
		}

		e := &Event{
			Type:        et,
			MouseX:      mx,
			MouseY:      my,
			MouseButton: MouseButton(eventType % 3),
			KeyCode:     KeyCode(eventType % 350),
			CharCode:    eventType,
			Modifiers:   Modifier(eventType % 16),
			ScrollX:     (mouseX - 400) / 100,
			ScrollY:     (mouseY - 300) / 100,
		}

		// Dispatch — must not panic.
		w.EventFn(e)

		// Verify layout tree still intact.
		walkLayoutAssertNonNegative(t, &w.layout)
	})
}

// FuzzEventDispatchWithDialog fuzzes event dispatch with modal dialog
// visible. Dialog routing changes the dispatch target to the last child.
func FuzzEventDispatchWithDialog(f *testing.F) {
	f.Add(uint8(2), uint8(3), uint32(0x01), float32(100), float32(100))

	f.Fuzz(func(t *testing.T, depth, childCount uint8, eventType uint32, mouseX, mouseY float32) {
		d := int(depth % 3)
		n := int(childCount%10) + 1
		et := EventType(eventType % 26)

		mx := clampEventCoord(mouseX, 1024)
		my := clampEventCoord(mouseY, 768)

		w := &Window{
			scratch:      newScratchPools(),
			focused:      true,
			windowWidth:  1024,
			windowHeight: 768,
		}

		eventLayout := buildFuzzEventLayoutTree(d, n, w)

		// Add a dialog layer as the last child.
		dialogLayout := Layout{
			Shape: w.allocShape(Shape{
				ID:        reservedDialogID,
				Focusable: true,
				Width:     400,
				Height:    300,
				shapeClip: drawClip{X: 0, Y: 0, Width: 400, Height: 300},
			}),
		}
		eventLayout.Children = append(eventLayout.Children, dialogLayout)
		w.layout = eventLayout
		w.dialogCfg.visible = true

		e := &Event{
			Type:        et,
			MouseX:      mx,
			MouseY:      my,
			MouseButton: MouseButton(eventType % 3),
			KeyCode:     KeyCode(eventType % 350),
			CharCode:    eventType,
			Modifiers:   Modifier(eventType % 16),
			ScrollX:     (mouseX - 400) / 100,
			ScrollY:     (mouseY - 300) / 100,
		}

		w.EventFn(e) // must not panic
	})
}

// FuzzEventDispatchUnfocused fuzzes event dispatch when window is
// unfocused. Only certain event types are allowed through.
func FuzzEventDispatchUnfocused(f *testing.F) {
	f.Add(uint8(2), uint8(5), uint32(0x01), float32(400), float32(300))

	f.Fuzz(func(t *testing.T, depth, childCount uint8, eventType uint32, mouseX, mouseY float32) {
		d := int(depth % 4)
		n := int(childCount % 12)
		et := EventType(eventType % 26)

		mx := clampEventCoord(mouseX, 1024)
		my := clampEventCoord(mouseY, 768)

		w := &Window{
			scratch:      newScratchPools(),
			focused:      false,
			windowWidth:  1024,
			windowHeight: 768,
		}

		eventLayout := buildFuzzEventLayoutTree(d, n, w)
		w.layout = eventLayout

		e := &Event{
			Type:        et,
			MouseX:      mx,
			MouseY:      my,
			MouseButton: MouseRight, // right-click allowed when unfocused
			ScrollY:     -10,
		}

		w.EventFn(e) // must not panic
	})
}
