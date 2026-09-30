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
//		"github.com/go-gui-org/go-gui/gui/backend"
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
//			OnInit: func(w *gui.Window) { w.SetView(view) },
//		})
//		backend.Run(w)
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
//					ID: "clicks",
//					Content: []gui.View{gui.Text(gui.TextCfg{
//						Text: fmt.Sprintf("%d clicks", app.Clicks),
//					})},
//					OnClick: func(ctx gui.EventCtx) {
//						gui.State[App](ctx.Window).Clicks++
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
//     from an explicit zero, for primitives only. Owned types
//     (Color, Padding, Sizing, Spacing, Radius, Border) self-flag
//     instead: build them with their constructors or roles and read
//     them with Or.
//   - required tags: fields tagged `gui:"required"` (e.g.
//     InputGroupCfg.ID) must be non-empty. Enforced by the requiredid
//     vet analyzer.
//   - auto tags: an ID tagged `gui:"auto"` may be left empty. The
//     widget then takes a generated ID from its kind and position, and
//     still joins focus, scroll and state. Set an explicit ID to name
//     it from code (SetFocus, FindByID, tests), or when a widget of
//     the same kind can appear before it at run time.
//   - Focusable: true opts the widget into keyboard focus (click or
//     Tab). An opt-in Focusable without an ID is a silent no-op; a
//     focusable-by-default control with an empty auto ID still joins
//     the tab order. Tab order follows layout-tree (depth-first) order.
//   - Common fields: Sizing, Spacing, Float, FloatAnchor, FloatTieOff,
//     Disabled, Invisible, Padding, Radius, SizeBorder appear on
//     most Cfg types with identical semantics.
//
// # Layout tree
//
// The [Layout] tree is the central data structure. Each frame:
//
//  1. The active view function returns a [View]; [GenerateViewLayout]
//     generates its [Layout] tree
//  2. The layout engine sizes and positions nodes ([layoutArrange])
//  3. The renderer walks the tree to produce []RenderCmd ([renderLayout])
//
// A View generates its children itself, through appendChildViews, so a
// View tree cannot be walked without generating it — and generating it
// discards View identity: Layout.Children carries [Shape], not View.
// Record the Views you need to find by type at construction time.
//
// Layouts use pointer parents and value children — no reference
// cycles. [Shape] holds visual state (position, size, color, events).
// Layout provides structure (parent, children) and animation offsets.
package gui
