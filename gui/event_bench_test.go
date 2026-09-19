package gui

import "testing"

func BenchmarkEventFnMouseMove(b *testing.B) {
	w := newEventTestWindow()
	w.layout = Layout{
		Shape: &Shape{shapeClip: w.windowRect()},
		Children: []Layout{
			{Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0, Width: 200, Height: 200},
				events: &eventHandlers{
					OnMouseMove: func(ctx EventCtx) {
						ctx.Consume()
					},
				},
			}},
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	e := &Event{
		Type:      EventMouseMove,
		MouseX:    25,
		MouseY:    25,
		Modifiers: ModNone,
	}
	benchRequireHandled(b, w, e)
	for b.Loop() {
		e.IsHandled = false
		w.EventFn(e)
	}
}

func BenchmarkEventFnMouseScrollFocused(b *testing.B) {
	w := newEventTestWindow()
	w.layout = Layout{
		Shape: &Shape{shapeClip: w.windowRect()},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true, ID: "f77",
				events: &eventHandlers{
					OnMouseScroll: func(ctx EventCtx) {},
				},
			}},
			{Shape: &Shape{
				Scrollable: true,
				ID:         "1",
				Width:      100,
				Height:     50,
				shapeClip: drawClip{
					X: 0, Y: 0, Width: 100, Height: 50,
				},
			}, Children: []Layout{
				// shapeRectangle: a shapeNone child is skipped by contentHeight, which
				// leaves nothing to scroll, so the fallback never handled the event.
				{Shape: &Shape{shapeType: shapeRectangle, Height: 200}},
			}},
		},
	}
	w.SetFocus("f77")
	b.ReportAllocs()
	b.ResetTimer()
	e := &Event{
		Type:      EventMouseScroll,
		MouseX:    10,
		MouseY:    10,
		ScrollY:   -1,
		Modifiers: ModNone,
	}
	benchRequireHandled(b, w, e)
	for b.Loop() {
		e.IsHandled = false
		w.EventFn(e)
	}
}

func BenchmarkEventFnDeepNesting(b *testing.B) {
	w := newEventTestWindow()
	// Build 12-level deep nesting; handler at the leaf.
	leaf := Layout{Shape: &Shape{
		shapeClip: drawClip{X: 0, Y: 0, Width: 800, Height: 600},
		events: &eventHandlers{
			OnMouseMove: func(ctx EventCtx) {
				ctx.Consume()
			},
		},
	}}
	for range 11 {
		leaf = Layout{
			Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0, Width: 800, Height: 600},
			},
			Children: []Layout{leaf},
		}
	}
	w.layout = Layout{
		Shape:    &Shape{shapeClip: w.windowRect()},
		Children: []Layout{leaf},
	}

	b.ReportAllocs()
	b.ResetTimer()
	e := &Event{
		Type:      EventMouseMove,
		MouseX:    25,
		MouseY:    25,
		Modifiers: ModNone,
	}
	benchRequireHandled(b, w, e)
	for b.Loop() {
		e.IsHandled = false
		w.EventFn(e)
	}
}

func BenchmarkExecuteMouseCallback(b *testing.B) {
	layout := &Layout{
		Shape: &Shape{
			shapeClip: drawClip{X: 10, Y: 10, Width: 100, Height: 100},
			X:         10,
			Y:         10,
		},
	}
	w := newEventTestWindow()
	cb := func(ctx EventCtx) {
		if ctx.Event.MouseX >= 0 && ctx.Event.MouseY >= 0 {
			ctx.Consume()
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	e := &Event{MouseX: 25, MouseY: 25}
	for b.Loop() {
		e.IsHandled = false
		e.MouseX = 25
		e.MouseY = 25
		executeMouseCallback(layout, e, w, cb, evNotify)
	}
}

func BenchmarkEventFnMouseMoveDisjointPanels(b *testing.B) {
	const (
		panelCols   = 4
		panelRows   = 3
		panelWidth  = float32(200)
		panelHeight = float32(200)
		controlCols = 8
		controlRows = 4
		controlW    = panelWidth / controlCols
		controlH    = panelHeight / controlRows
	)

	w := newEventTestWindow()
	w.layout = Layout{
		Shape: &Shape{shapeClip: drawClip{
			Width: float32(w.windowWidth), Height: float32(w.windowHeight),
		}},
	}
	w.layout.Children = make([]Layout, 0, panelCols*panelRows)
	// Consume so benchRequireHandled can see that the walk reached a control.
	onMouseMove := func(ctx EventCtx) { ctx.Consume() }
	for panelY := range panelRows {
		for panelX := range panelCols {
			x := float32(panelX) * panelWidth
			y := float32(panelY) * panelHeight
			panel := Layout{
				Shape: &Shape{
					shapeClip: drawClip{
						X: x, Y: y, Width: panelWidth, Height: panelHeight,
					},
				},
				Children: make([]Layout, 0, controlCols*controlRows),
			}
			for controlY := range controlRows {
				for controlX := range controlCols {
					panel.Children = append(panel.Children, Layout{Shape: &Shape{
						shapeClip: drawClip{
							X:      x + float32(controlX)*controlW,
							Y:      y + float32(controlY)*controlH,
							Width:  controlW,
							Height: controlH,
						},
						events: &eventHandlers{OnMouseMove: onMouseMove},
					}})
				}
			}
			w.layout.Children = append(w.layout.Children, panel)
		}
	}

	e := &Event{Type: EventMouseMove, MouseX: 12.5, MouseY: 25}
	benchRequireHandled(b, w, e)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		e.IsHandled = false
		w.EventFn(e)
	}
}

// benchRequireHandled sends one event and fails the benchmark when nothing
// handles it. A dispatch benchmark whose fixture stops reaching its callback
// still runs, and then reports the cost of an early return as a speedup. That
// happened when hit testing began to prune at a root with an empty shapeClip.
func benchRequireHandled(b *testing.B, w *Window, e *Event) {
	b.Helper()
	e.IsHandled = false
	w.EventFn(e)
	if !e.IsHandled {
		b.Fatal("benchmark event was not handled; the fixture no longer dispatches")
	}
}
