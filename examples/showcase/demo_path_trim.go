package main

import (
	"math"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// The path trim panel on the Draw Canvas page: a check mark and a
// rounded-square ring that draw themselves in with CanvasPath.TrimTo
// (#1023). examples/path_trim is the same demo on its own.

const (
	pathTrimW float32 = 480
	pathTrimH float32 = 200
	// pathTrimAnim is the ID of the tween that moves the progress.
	pathTrimAnim = "showcase-path-trim"
)

func demoDrawCanvasPathTrim(app *ShowcaseApp) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FitFit,
		Spacing:    gui.SpacingSmall,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.DrawCanvas(gui.DrawCanvasCfg{
				ID:      "showcase-draw-canvas-path-trim",
				Width:   pathTrimW,
				Height:  pathTrimH,
				Color:   gui.RGBA(30, 30, 40, 255),
				Radius:  8,
				Padding: gui.NewPadding(24, 24, 24, 24),
				// The tween changes progress with no new layout, so the
				// canvas reads its version live.
				VersionFn: func() uint64 { return app.PathTrimVersion },
				OnDraw: func(dc *gui.DrawContext) {
					drawPathTrim(dc, app)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "path-trim-replay",
				Label:   "Replay",
				OnClick: pathTrimReplay,
			}),
		},
	})
}

// pathTrimReplay draws both paths in again from nothing.
func pathTrimReplay(ctx gui.EventCtx) {
	tw := gui.NewTweenAnimation(pathTrimAnim, 0, 1,
		func(v float32, w *gui.Window) {
			app := appState(w)
			app.PathTrimProgress = v
			app.PathTrimVersion++
		})
	tw.Duration = 1200 * time.Millisecond
	// Only the drawing changes, not the widget tree: skip the view
	// function and the layout on each tick.
	tw.Refresh = gui.AnimationRefreshRenderOnly
	ctx.Window.AnimationAdd(tw)
	ctx.Consume()
}

// drawPathTrim strokes the part of each path the progress gives, over
// a faint track that shows the whole ring. One trimmed path is reused
// for both: each stroke is tessellated before the next trim.
//
// The colors are fixed, like the other canvases on this page. The
// canvas repaints only when PathTrimVersion changes, and a theme switch
// does not change it: a theme color would stay stale until Replay.
func drawPathTrim(dc *gui.DrawContext, app *ShowcaseApp) {
	if app.PathTrimMark.IsEmpty() {
		buildPathTrimPaths(app)
	}
	round := gui.StrokeStyle{Cap: gui.StrokeRoundCap, Join: gui.StrokeRoundJoin}

	app.PathTrimMark.TrimTo(&app.PathTrimOut, 0, app.PathTrimProgress)
	dc.StrokePath(&app.PathTrimOut, gui.RGBA(90, 180, 130, 255), 10, round)

	dc.StrokePath(&app.PathTrimRing, gui.RGBA(80, 80, 100, 255), 8, round)
	app.PathTrimRing.TrimTo(&app.PathTrimOut, 0, app.PathTrimProgress)
	dc.StrokePath(&app.PathTrimOut, gui.RGBA(70, 130, 220, 255), 8, round)
}

// buildPathTrimPaths builds the check mark on the left of the canvas
// and the ring, lines joined by quarter arcs, on the right. Both fit
// the 432×152 area inside the canvas padding.
func buildPathTrimPaths(app *ShowcaseApp) {
	app.PathTrimMark.MoveTo(40, 80)
	app.PathTrimMark.LineTo(85, 125)
	app.PathTrimMark.LineTo(170, 25)

	// The ring starts at the top middle and runs clockwise.
	const x, y, size, r = 250, 10, 132, 32
	const half = math.Pi / 2
	ring := &app.PathTrimRing
	ring.MoveTo(x+size/2, y)
	ring.ArcTo(x+size-r, y+r, r, r, -half, half)
	ring.ArcTo(x+size-r, y+size-r, r, r, 0, half)
	ring.ArcTo(x+r, y+size-r, r, r, half, half)
	ring.ArcTo(x+r, y+r, r, r, math.Pi, half)
	ring.LineTo(x+size/2, y)
}
