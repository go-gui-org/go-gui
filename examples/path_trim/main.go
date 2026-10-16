// The path_trim example draws a check mark and a progress ring that
// draw themselves in, with CanvasPath.TrimTo (#1023).
package main

import (
	"flag"
	"log"
	"math"
	"os"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const (
	canvasW = 320
	canvasH = 160
	// trimAnim is the ID of the tween that moves progress.
	trimAnim = "trim"
)

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Path Trim",
		Width:  400,
		Height: 300,
		// The screenshot shows the drawing part of the way in.
		State: &app{progress: 0.7},
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

type app struct {
	// progress is how much of each path is drawn, from 0 to 1.
	progress float32
	// version changes each time progress does. The canvas repaints
	// when it changes, also on a render-only frame.
	version uint64
	// mark and ring are built once. trimmed takes the trimmed copy of
	// each one in turn: reused every frame, it stops allocating after
	// the first one.
	mark, ring, trimmed gui.CanvasPath
}

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	s := gui.State[app](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingLarge,
		Padding: gui.PaddingLarge,
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Path Trim",
				TextStyle: theme.TextStyleTitle,
			}),
			gui.DrawCanvas(gui.DrawCanvasCfg{
				ID:     "paths",
				Width:  canvasW,
				Height: canvasH,
				Sizing: gui.FixedFixed,
				// The tween changes progress with no new layout, so
				// the canvas reads its version live (#1016).
				VersionFn: func() uint64 { return s.version },
				OnDraw: func(dc *gui.DrawContext) {
					drawPaths(dc, s, theme)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "replay",
				Label:   "Replay",
				OnClick: replay,
			}),
		},
	})
}

// replay draws both paths in again from nothing.
func replay(ctx gui.EventCtx) {
	tw := gui.NewTweenAnimation(trimAnim, 0, 1,
		func(v float32, w *gui.Window) {
			s := gui.State[app](w)
			s.progress = v
			s.version++
		})
	tw.Duration = 1200 * time.Millisecond
	// Only the drawing changes, not the widget tree: skip the view
	// function and the layout on each tick.
	tw.Refresh = gui.AnimationRefreshRenderOnly
	ctx.Window.AnimationAdd(tw)
	ctx.Consume()
}

// doc:snippet-begin path-trim-draw

// drawPaths strokes the part of each path that progress gives, over a
// faint track that shows the whole ring.
func drawPaths(dc *gui.DrawContext, s *app, theme gui.Theme) {
	if s.mark.IsEmpty() {
		buildPaths(s)
	}
	round := gui.StrokeStyle{Cap: gui.StrokeRoundCap, Join: gui.StrokeRoundJoin}

	s.mark.TrimTo(&s.trimmed, 0, s.progress)
	dc.StrokePath(&s.trimmed, theme.ColorSelect, 8, round)

	dc.StrokePath(&s.ring, theme.ColorBorder, 6, round)
	s.ring.TrimTo(&s.trimmed, 0, s.progress)
	dc.StrokePath(&s.trimmed, theme.ColorAccent, 6, round)
}

// doc:snippet-end path-trim-draw

// buildPaths builds the check mark on the left half of the canvas and
// a rounded-square ring, a custom path with lines and arcs, on the
// right half.
func buildPaths(s *app) {
	s.mark.MoveTo(40, 85)
	s.mark.LineTo(75, 120)
	s.mark.LineTo(135, 45)

	// The ring starts at the top middle and runs clockwise.
	const x, y, size, r = 190, 30, 100, 24
	const half = math.Pi / 2
	s.ring.MoveTo(x+size/2, y)
	s.ring.ArcTo(x+size-r, y+r, r, r, -half, half)
	s.ring.ArcTo(x+size-r, y+size-r, r, r, 0, half)
	s.ring.ArcTo(x+r, y+size-r, r, r, half, half)
	s.ring.ArcTo(x+r, y+r, r, r, math.Pi, half)
	s.ring.LineTo(x+size/2, y)
}
