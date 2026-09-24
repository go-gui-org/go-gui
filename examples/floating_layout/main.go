// This example demonstrates anchored overlays positioned over their parent content (advanced: floating layout).
// The floating layout example shows how anchored overlays can be
// positioned relative to their parent content.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// Floating Layouts
//
// Many UI designs need to draw content over other content.
// Menus and Dialog Boxes for instance. GUI calls these
// floats. Floats can be nested for z axis stacking often
// required in drop down menus.
//
// Floats can be anchored to their parent container at nine points:
//   - TopLeft, TopCenter, TopRight
//   - MiddleLeft, MiddleCenter, MiddleRight
//   - BottomLeft, BottomCenter, BottomRight
//
// The float itself has similar attachment points called "tie-offs".
//
// A boating analogy can help with picturing how this works.
// A boat can be anchored in a harbor but the anchor line can
// be tied-off to the bow or stern.

type App struct {
	// OverlayDismissed hides the demo overlay once OK is pressed.
	OverlayDismissed bool
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "Floating Layout",
		Width:  500,
		Height: 500,
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
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	content := []gui.View{
		// Menu bar
		gui.Row(gui.ContainerCfg{
			Color:      theme.ColorInterior,
			Sizing:     gui.FillFit,
			SizeBorder: gui.NoBorder,
			VAlign:     gui.VAlignMiddle,
			Content: []gui.View{
				gui.Menubar(w, gui.MenubarCfg{
					ID: "floating_layout_menu",
					// Theme translucent surface: the old faux menu
					// read ColorFocus channels into an RGBA literal.
					Color: theme.ColorFocus.WithOpacity(210.0 / 255),
					Items: []gui.MenuItemCfg{
						gui.MenuItemText("file", "File"),
						gui.MenuSubmenu("edit", "Edit", []gui.MenuItemCfg{
							gui.MenuItemText("cut", "Cut"),
							gui.MenuItemText("copy", "Copy"),
							gui.MenuSubmenu("paste", "Paste", []gui.MenuItemCfg{
								gui.MenuItemText("clean", "Clean"),
								gui.MenuItemText("selection", "Selection"),
							}),
						}),
					},
				}),
				gui.Rectangle(gui.RectangleCfg{Sizing: gui.FillFit}),
				gui.ThemePicker(gui.ThemePickerCfg{
					ID:          "theme-picker",
					Focusable:   true,
					FloatAnchor: gui.FloatBottomRight,
					FloatTieOff: gui.FloatTopRight,
				}),
			},
		}),
		// Two-panel body
		gui.Row(gui.ContainerCfg{
			Padding:    gui.NoPadding,
			Sizing:     gui.FillFill,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Column(gui.ContainerCfg{
					Color:      theme.ColorInterior,
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					MinWidth:   100,
					MaxWidth:   150,
				}),
				gui.Column(gui.ContainerCfg{
					Color:      theme.ColorInterior,
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					MinWidth:   100,
				}),
			},
		}),
	}
	if !app.OverlayDismissed {
		// Centered floating overlay
		content = append(content, gui.Column(gui.ContainerCfg{
			Float:       true,
			FloatAnchor: gui.FloatMiddleCenter,
			FloatTieOff: gui.FloatMiddleCenter,
			HAlign:      gui.HAlignCenter,
			Sizing:      gui.FitFit,
			SizeBorder:  gui.NoBorder,
			Color:       theme.ColorActive,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      "Floating column with content",
					TextStyle: theme.TextStyleTitle,
				}),
				gui.Button(gui.ButtonCfg{
					ID: "floating_layout_ok",
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: "OK"}),
					},
					OnClick: func(ctx gui.EventCtx) {
						gui.State[App](ctx.Window).OverlayDismissed = true
					},
				}),
			},
		}))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Content: content,
	})
}
