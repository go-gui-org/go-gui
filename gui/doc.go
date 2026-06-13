// Package gui is a cross-platform GUI framework for Go.
//
// It implements a stateless View → Layout → RenderCmd pipeline: each frame,
// a View function returns a tree of [Layout] nodes; the layout engine sizes
// and positions them; and the backend converts the result into draw commands.
// No virtual DOM diffing — the whole tree is rebuilt every frame.
//
// # Minimal example
//
//	package main
//
//	import (
//		"fmt"
//
//		"github.com/go-gui-org/go-gui/gui"
//		sdl2 "github.com/go-gui-org/go-gui/gui/backend/sdl2"
//	)
//
//	type App struct{ Clicks int }
//
//	func main() {
//		gui.SetTheme(gui.ThemeDark.WithBorders(true))
//		w := gui.NewWindow(gui.WindowCfg{
//			State:  &App{},
//			Title:  "hello",
//			Width:  400,
//			Height: 300,
//			OnInit: func(w *gui.Window) { w.UpdateView(view) },
//		})
//		sdl2.Run(w)
//	}
//
//	func view(w *gui.Window) gui.View {
//		app := gui.State[App](w)
//		ww, wh := w.WindowSize()
//		return gui.Column(gui.ContainerCfg{
//			Width: float32(ww), Height: float32(wh),
//			Sizing: gui.FixedFixed,
//			HAlign: gui.HAlignCenter, VAlign: gui.VAlignMiddle,
//			Content: []gui.View{
//				gui.Button(gui.ButtonCfg{
//					IDFocus: 1,
//					Content: []gui.View{gui.Text(gui.TextCfg{
//						Text: fmt.Sprintf("%d clicks", app.Clicks),
//					})},
//					OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
//						gui.State[App](w).Clicks++
//					},
//				}),
//			},
//		})
//	}
//
// See [examples/get_started] for a complete runnable program.
//
// # Widget configuration
//
// Every widget factory accepts a *Cfg struct (e.g. [ButtonCfg],
// [ContainerCfg], [TextCfg]). Cfg types follow these conventions:
//
//   - Zero-initializable: all fields have usable zero values. Create
//     with ButtonCfg{Text: "Click"} — omit fields you don't need.
//   - Opt[T] fields: optional overrides that distinguish "not set"
//     from an explicit zero. Use cfg.Radius.Get(default) or
//     cfg.Radius.Set(5).
//   - required tags: fields tagged `gui:"required"` (e.g. FormCfg.ID)
//     must be non-empty. Enforced by the requiredid vet analyzer.
//   - IDFocus > 0: opts the widget into tab-order focus. The numeric
//     value determines tab sequence (lower = earlier).
//   - Common fields: Sizing, Float, FloatAnchor, FloatTieOff,
//     Disabled, Invisible, Padding, Radius, SizeBorder appear on
//     most Cfg types with identical semantics.
//
// # Layout tree
//
// The [Layout] tree is the central data structure. Each frame:
//
//  1. [View] functions return a Layout tree via [GenerateViewLayout]
//  2. The layout engine sizes and positions nodes ([layoutArrange])
//  3. The renderer walks the tree to produce []RenderCmd ([renderLayout])
//
// Layouts use pointer parents and value children — no reference
// cycles. [Shape] holds visual state (position, size, color, events).
// Layout provides structure (parent, children) and animation offsets.
package gui
