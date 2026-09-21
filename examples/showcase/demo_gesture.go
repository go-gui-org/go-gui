package main

import (
	"fmt"
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

func demoGesture(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)

	if app.GesturePadLabel == "" {
		app.GesturePadLabel = "Touch or click the pad"
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SomeF(8),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				// Gestures come only from touch input (web, iOS, Android).
				// The desktop backends send mouse and scroll events, so say
				// what works there instead of implying pinch/rotate do.
				Text: "Touch: tap, pan, pinch, rotate, swipe, or long-press. " +
					"Double-tap clears markers. " +
					"Use a touchscreen or Chrome DevTools touch emulation. " +
					"Mouse or trackpad: click adds a marker, scroll pans.",
				TextStyle: t.TextStyleBody,
				Mode:      gui.TextModeWrap,
			}),
			gui.DrawCanvas(gui.DrawCanvasCfg{
				ID:      "gesture-pad",
				Version: app.GesturePadVersion,
				Sizing:  gui.FillFit,
				Width:   400,
				Height:  300,
				Color:   gui.RGBA(30, 30, 40, 255),
				Radius:  8,
				Clip:    true,
				OnDraw:  gestureOnDraw(app),
				OnClick: func(ctx gui.EventCtx) {
					a := appState(ctx.Window)
					a.GesturePadMarkers = append(
						a.GesturePadMarkers,
						GestureMarker{X: ctx.Event.MouseX, Y: ctx.Event.MouseY},
					)
					a.GesturePadLabel = fmt.Sprintf(
						"Click at (%.0f, %.0f)", ctx.Event.MouseX, ctx.Event.MouseY)
					a.GesturePadVersion++
				},
				OnGesture: func(ctx gui.EventCtx) {
					a := appState(ctx.Window)
					gestureOnGesture(a, ctx.Event)
				},
				// Desktop stand-in for a pan: a two-finger trackpad drag
				// arrives as scroll, not as a gesture. Consume it so the
				// showcase page does not scroll while the pad pans.
				OnMouseScroll: func(ctx gui.EventCtx) {
					gestureOnScroll(appState(ctx.Window), ctx.Event)
					ctx.Consume()
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      app.GesturePadLabel,
				TextStyle: t.TextStyleTitleSmall,
			}),
		},
	})
}

// gestureOnScroll pans the square by the scroll delta. With macOS
// natural scrolling the square follows the fingers.
func gestureOnScroll(app *ShowcaseApp, e *gui.Event) {
	app.GesturePadOffsetX += e.ScrollX
	app.GesturePadOffsetY += e.ScrollY
	app.GesturePadLabel = fmt.Sprintf(
		"Scroll  offset (%.0f, %.0f)",
		app.GesturePadOffsetX, app.GesturePadOffsetY)
	app.GesturePadVersion++
}

func gestureOnGesture(app *ShowcaseApp, e *gui.Event) {
	switch e.GestureType {
	case gui.GestureTap:
		app.GesturePadMarkers = append(app.GesturePadMarkers,
			GestureMarker{X: e.CentroidX, Y: e.CentroidY})
		app.GesturePadLabel = fmt.Sprintf(
			"Tap at (%.0f, %.0f)", e.CentroidX, e.CentroidY)

	case gui.GestureDoubleTap:
		app.GesturePadMarkers = app.GesturePadMarkers[:0]
		app.GesturePadOffsetX = 0
		app.GesturePadOffsetY = 0
		app.GesturePadScale = 1
		app.GesturePadRotation = 0
		app.GesturePadLabel = "Double tap — reset"

	case gui.GestureLongPress:
		app.GesturePadMarkers = append(app.GesturePadMarkers,
			GestureMarker{
				X: e.CentroidX, Y: e.CentroidY,
				LongPress: true,
			})
		app.GesturePadLabel = fmt.Sprintf(
			"Long press at (%.0f, %.0f)",
			e.CentroidX, e.CentroidY)

	case gui.GesturePan:
		app.GesturePadOffsetX += e.GestureDX
		app.GesturePadOffsetY += e.GestureDY
		app.GesturePadLabel = fmt.Sprintf(
			"Pan  offset (%.0f, %.0f)",
			app.GesturePadOffsetX, app.GesturePadOffsetY)

	case gui.GestureSwipe:
		dir := "right"
		if absF(e.VelocityY) > absF(e.VelocityX) {
			if e.VelocityY > 0 {
				dir = "down"
			} else {
				dir = "up"
			}
		} else if e.VelocityX < 0 {
			dir = "left"
		}
		app.GesturePadLabel = "Swipe " + dir

	case gui.GesturePinch:
		app.GesturePadScale = e.PinchScale
		app.GesturePadLabel = fmt.Sprintf(
			"Pinch  scale %.2f", app.GesturePadScale)

	case gui.GestureRotate:
		app.GesturePadRotation = e.GestureRotation
		deg := app.GesturePadRotation * 180 / math.Pi
		app.GesturePadLabel = fmt.Sprintf("Rotate  %.1f\u00b0", deg)
	}
	app.GesturePadVersion++
}

func gestureOnDraw(app *ShowcaseApp) func(*gui.DrawContext) {
	ox := app.GesturePadOffsetX
	oy := app.GesturePadOffsetY
	sc := app.GesturePadScale
	rot := app.GesturePadRotation
	markers := append([]GestureMarker(nil), app.GesturePadMarkers...)

	return func(dc *gui.DrawContext) {
		cw := dc.Width
		ch := dc.Height
		grid := gui.RGBA(60, 60, 80, 255)

		// Grid.
		const step float32 = 40
		for x := step; x < cw; x += step {
			dc.Line(x, 0, x, ch, grid, 1)
		}
		for y := step; y < ch; y += step {
			dc.Line(0, y, cw, y, grid, 1)
		}

		// Central shape: a square with directional arrow,
		// transformed by pan/scale/rotate.
		cx := cw/2 + ox
		cy := ch/2 + oy
		half := float32(40) * sc

		// Draw rotated square as 4-point polygon.
		sin := float32(math.Sin(float64(rot)))
		cos := float32(math.Cos(float64(rot)))
		corners := [4][2]float32{
			{-half, -half}, {half, -half},
			{half, half}, {-half, half},
		}
		pts := make([]float32, 0, 10)
		for _, c := range corners {
			rx := c[0]*cos - c[1]*sin + cx
			ry := c[0]*sin + c[1]*cos + cy
			pts = append(pts, rx, ry)
		}
		// Close polygon.
		pts = append(pts, pts[0], pts[1])
		dc.Polyline(pts, gui.RGBA(100, 160, 255, 255), 2)

		// Arrow pointing "up" (before rotation) from center.
		ax := float32(0)*cos - (-half*0.7)*sin + cx
		ay := float32(0)*sin + (-half*0.7)*cos + cy
		dc.Line(cx, cy, ax, ay, gui.RGBA(100, 160, 255, 200), 2)
		dc.FilledCircle(ax, ay, 4*sc, gui.RGBA(100, 160, 255, 255))

		// Markers.
		for _, m := range markers {
			if m.LongPress {
				dc.FilledCircle(m.X, m.Y, 8,
					gui.RGBA(255, 160, 50, 220))
				dc.Circle(m.X, m.Y, 12,
					gui.RGBA(255, 160, 50, 120), 2)
			} else {
				dc.FilledCircle(m.X, m.Y, 6,
					gui.RGBA(80, 180, 255, 220))
			}
		}
	}
}

func absF(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
