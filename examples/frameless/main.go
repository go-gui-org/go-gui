// Frameless shows a window with no native title bar or border, built
// with WindowCfg.Decorations. Because the OS draws nothing to grab, the
// app supplies its own: the header strip calls Window.StartWindowDrag to
// move the window, and the bottom-right grip calls
// Window.StartWindowResize. Both hand the gesture to the OS, so window
// snapping and edge tiling keep working.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// App holds which decoration the window was created with, so the view
// can say so on screen.
type App struct {
	Decoration gui.WindowDecoration
}

const (
	headerHeight = 40.0
	gripSize     = 18.0
	// Width the macOS traffic lights occupy. A hidden-titlebar window
	// keeps them floating over the content, so the app's own header has
	// to start to their right. No platform helper provides this
	// (checked gui/backend): the value is the measured cluster width
	// plus its leading gap on macOS, used only for
	// DecorationHiddenTitlebar.
	trafficLightInset = 76.0
)

// framelessScope owns every ID in this window, so the header, body and
// grip helpers stay reusable when dropped into another owner.
const framelessScope = "frameless"

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	hidden := flag.Bool("hidden-titlebar", false,
		"use DecorationHiddenTitlebar instead of DecorationNone (macOS)")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	decoration := gui.DecorationNone
	if *hidden {
		decoration = gui.DecorationHiddenTitlebar
	}

	w := gui.NewWindow(gui.WindowCfg{
		State:       &App{Decoration: decoration},
		Title:       "frameless",
		Width:       440,
		Height:      280,
		Decorations: decoration,
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

func mainView(w *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		ID:         gui.ScopeID(framelessScope, "root"),
		Sizing:     gui.FillFill,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			header(framelessScope, gui.State[App](w).Decoration),
			body(framelessScope, w),
			grip(framelessScope),
		},
	})
}

// header is the drag strip that stands in for the missing title bar.
// The press starts a native move; the app never sees the matching
// release, so there is no drag state of its own to keep.
func header(owner string, decoration gui.WindowDecoration) gui.View {
	theme := gui.CurrentTheme()
	left := theme.SpacingMedium
	if decoration == gui.DecorationHiddenTitlebar {
		left = trafficLightInset
	}
	return gui.Row(gui.ContainerCfg{
		ID:         gui.ScopeID(owner, "header"),
		Sizing:     gui.FillFixed,
		Height:     headerHeight,
		SizeBorder: gui.NoBorder,
		Padding:    gui.NewPadding(0, theme.SpacingMedium, 0, left),
		Spacing:    gui.SomeF(theme.SpacingSmall),
		VAlign:     gui.VAlignMiddle,
		Color:      theme.ColorPanel,
		OnMouseDown: func(ctx gui.EventCtx) {
			ctx.Window.StartWindowDrag()
			ctx.Consume()
		},
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Drag me",
				TextStyle: theme.TextStyleDisplay,
			}),
			// Fill container: pushes the close button to the right.
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				SizeBorder: gui.NoBorder,
			}),
			gui.Button(gui.ButtonCfg{
				ID:      gui.ScopeID(owner, "close"),
				A11YCfg: gui.A11YCfg{A11YLabel: "Close window"},
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Close"})},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Window.Close()
					ctx.Consume()
				},
			}),
		},
	})
}

func body(owner string, w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	label := "Decorations: DecorationNone"
	if gui.State[App](w).Decoration == gui.DecorationHiddenTitlebar {
		label = "Decorations: DecorationHiddenTitlebar"
	}
	return gui.Column(gui.ContainerCfg{
		ID:         gui.ScopeID(owner, "body"),
		Sizing:     gui.FillFill,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Spacing:    gui.SomeF(theme.SpacingSmall),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: theme.TextStyleDisplay}),
			gui.Text(gui.TextCfg{
				Text:      "Corner grip resizes on Windows and X11.",
				TextStyle: theme.TextStyleSecondary,
			}),
			gui.Text(gui.TextCfg{
				Text:      "macOS resizes from any edge, so the grip is inert there.",
				TextStyle: theme.TextStyleSecondary,
			}),
		},
	})
}

// grip is the bottom-right resize handle. macOS ignores the request
// because AppKit already resizes a borderless window from its edges.
func grip(owner string) gui.View {
	theme := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		ID:         gui.ScopeID(owner, "grip_row"),
		Sizing:     gui.FillFixed,
		Height:     gripSize,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignRight,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				ID:         gui.ScopeID(owner, "grip"),
				Sizing:     gui.FixedFixed,
				Width:      gripSize,
				Height:     gripSize,
				SizeBorder: gui.NoBorder,
				Color:      theme.ColorPanel,
				OnMouseDown: func(ctx gui.EventCtx) {
					ctx.Window.StartWindowResize(gui.EdgeBottomRight)
					ctx.Consume()
				},
			}),
		},
	})
}
