package main

import "github.com/go-gui-org/go-gui/gui"

const (
	// scrollZoomPerLine and scrollZoomPerPoint convert the two units
	// Event.ScrollY can arrive in. A discrete wheel notch reports ~3
	// lines on every platform; a trackpad reports points of finger
	// travel. Branching on ScrollPrecise is the only way to tell.
	scrollZoomPerLine  = 0.12
	scrollZoomPerPoint = 0.02
)

// mainView builds the window: a full-bleed canvas with the zoom
// controls and the info panel floated over it. Float children are
// excluded from parent sizing, so the canvas keeps the whole window and
// the camera math never has to react to a panel appearing.
func mainView(w *gui.Window) gui.View {
	a := state(w)
	theme := gui.CurrentTheme()

	// Resolved here, during generation, where the theme read is
	// correct; OnDraw runs after generation and reads them back.
	a.TipStyle = theme.B4
	a.LabelStyle = theme.TextStyleSecondary

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(0),
		Color:      colorSpace,
		Content: []gui.View{
			solarCanvas(a),
			zoomControls(theme),
			infoPanel(a, theme),
		},
	})
}

// solarCanvas is the drawing surface and the whole pointer surface.
// Padding is explicitly none so shape-relative callback coordinates are
// also content coordinates, with no adjustment at any call site.
func solarCanvas(a *App) gui.View {
	return gui.DrawCanvas(gui.DrawCanvasCfg{
		ID:        canvasID,
		Sizing:    gui.FillFill,
		Padding:   gui.NoPadding,
		Version:   a.Version,
		Focusable: true,

		// The tick animation refreshes renderers only, so no view pass
		// runs to carry Version into the redraw gate. AlwaysRedraw is
		// the other half of that pairing; without it the canvas would
		// freeze on frame one.
		AlwaysRedraw: true,
		A11YCfg: gui.A11YCfg{
			A11YLabel: "Solar system map",
			A11YDescription: "Hover a body to identify it, click to " +
				"focus it. Left and right arrows step between the sun " +
				"and the planets, Escape returns to the full system.",
		},
		OnDraw: func(dc *gui.DrawContext) { drawSystem(a, dc) },

		// OnMouseMove is shape-relative and is the sole cursor source.
		// OnHover reports window-absolute coordinates, which would not
		// agree with the screen positions the hit-test caches.
		OnMouseMove: func(ctx gui.EventCtx) {
			if ctx.Event == nil {
				return
			}
			a.MouseX, a.MouseY = ctx.Event.MouseX, ctx.Event.MouseY
			a.MouseIn = true
			a.Hovered = a.hitTest(a.MouseX, a.MouseY)
		},

		OnMouseLeave: func(gui.EventCtx) {
			a.MouseIn = false
			a.Hovered = -1
		},

		// A click on a body selects it; a click on empty space returns
		// to the full system view.
		OnClick: func(ctx gui.EventCtx) {
			if ctx.Event == nil {
				return
			}
			selectBody(a, ctx.Window,
				a.hitTest(ctx.Event.MouseX, ctx.Event.MouseY))
			ctx.Consume()
		},

		OnMouseScroll: func(ctx gui.EventCtx) {
			e := ctx.Event
			if e == nil {
				return
			}
			// A scroll ends any pinch sequence in progress.
			a.PinchPrev = 0
			per := float32(scrollZoomPerLine)
			if e.ScrollPrecise {
				per = scrollZoomPerPoint
			}
			a.applyUserZoom(1 + e.ScrollY*per)
			ctx.Consume()
		},

		OnGesture: func(ctx gui.EventCtx) {
			e := ctx.Event
			if e == nil {
				return
			}
			if e.GestureType != gui.GesturePinch || e.PinchScale <= 0 {
				// Any other gesture ends the sequence, so the next
				// pinch re-baselines instead of jumping.
				a.PinchPrev = 0
				return
			}
			// Only gesturePhaseBegan/Ended/Cancelled are unexported;
			// GesturePhaseChanged is not, so "not a change" is how a
			// sequence boundary is recognized.
			a.applyPinch(e.PinchScale, e.GesturePhase != gui.GesturePhaseChanged)
			ctx.Consume()
		},
	})
}

// applyPinch folds one pinch event into the manual zoom. boundary marks
// the events that open or close a sequence — began, ended and cancelled
// — as opposed to a mid-gesture change.
//
// PinchScale is cumulative *within* one gesture and restarts near 1 for
// the next, so the previous value is what recovers a per-event ratio —
// but only between events of the same sequence. Carrying it across a
// boundary divides the new pinch's opening scale by the old one's final
// and undoes the whole previous pinch in a single frame.
//
// Baselining on all three boundary phases is what makes that one branch:
// on began it is the correct start value, and on ended or cancelled the
// stale value is overwritten by the next sequence's began before any
// delta is applied.
func (a *App) applyPinch(scale float32, boundary bool) {
	if boundary {
		a.PinchPrev = scale
		return
	}
	if a.PinchPrev > 0 {
		a.applyUserZoom(scale / a.PinchPrev)
	}
	a.PinchPrev = scale
}
