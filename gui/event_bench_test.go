package gui

import "testing"

func BenchmarkEventFnMouseMove(b *testing.B) {
	w := newEventTestWindow()
	w.layout = Layout{
		Shape: &Shape{},
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
	for b.Loop() {
		e.IsHandled = false
		w.EventFn(e)
	}
}

func BenchmarkEventFnMouseScrollFocused(b *testing.B) {
	w := newEventTestWindow()
	w.layout = Layout{
		Shape: &Shape{},
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
				{Shape: &Shape{Height: 200}},
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
	w.layout = Layout{Shape: &Shape{}, Children: []Layout{leaf}}

	b.ReportAllocs()
	b.ResetTimer()
	e := &Event{
		Type:      EventMouseMove,
		MouseX:    25,
		MouseY:    25,
		Modifiers: ModNone,
	}
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

func BenchmarkMouseDownClippedSubtrees(b *testing.B) {
	const (
		subtreeCount  = 64
		leavesPerTree = 128
	)

	buildLayout := func() Layout {
		clip := drawClip{X: 10, Y: 10, Width: 100, Height: 100}
		root := Layout{
			Shape: &Shape{shapeClip: drawClip{Width: 1000, Height: 1000}},
		}
		root.Children = make([]Layout, subtreeCount)
		for i := range root.Children {
			subtree := &root.Children[i]
			subtree.Shape = &Shape{Clip: true, shapeClip: clip}
			subtree.Children = make([]Layout, leavesPerTree)
			for j := range subtree.Children {
				subtree.Children[j].Shape = &Shape{shapeClip: clip}
			}
		}
		return root
	}

	for _, tc := range []struct {
		name string
		x, y float32
	}{
		{name: "outside_clips", x: 500, y: 500},
		{name: "inside_clips", x: 25, y: 25},
	} {
		b.Run(tc.name, func(b *testing.B) {
			layout := buildLayout()
			w := newEventTestWindow()
			e := &Event{MouseX: tc.x, MouseY: tc.y, MouseButton: MouseLeft}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				e.IsHandled = false
				mouseDownHandler(&layout, false, e, w)
			}
		})
	}
}

func BenchmarkMouseDownDisjointClippedPanels(b *testing.B) {
	const (
		panelCols    = 8
		panelRows    = 8
		panelSize    = float32(100)
		cellCols     = 16
		cellRows     = 8
		cellWidth    = panelSize / cellCols
		cellHeight   = panelSize / cellRows
		windowWidth  = panelCols * panelSize
		windowHeight = panelRows * panelSize
	)

	root := Layout{
		Shape: &Shape{shapeClip: drawClip{Width: windowWidth, Height: windowHeight}},
	}
	root.Children = make([]Layout, 0, panelCols*panelRows)
	for panelY := range panelRows {
		for panelX := range panelCols {
			x := float32(panelX) * panelSize
			y := float32(panelY) * panelSize
			panel := Layout{
				Shape: &Shape{
					Clip: true,
					shapeClip: drawClip{
						X: x, Y: y, Width: panelSize, Height: panelSize,
					},
				},
				Children: make([]Layout, 0, cellCols*cellRows),
			}
			for cellY := range cellRows {
				for cellX := range cellCols {
					panel.Children = append(panel.Children, Layout{Shape: &Shape{
						shapeClip: drawClip{
							X:      x + float32(cellX)*cellWidth,
							Y:      y + float32(cellY)*cellHeight,
							Width:  cellWidth,
							Height: cellHeight,
						},
					}})
				}
			}
			root.Children = append(root.Children, panel)
		}
	}

	w := newEventTestWindow()
	e := &Event{MouseX: 25, MouseY: 25, MouseButton: MouseLeft}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		e.IsHandled = false
		mouseDownHandler(&root, false, e, w)
	}
}
