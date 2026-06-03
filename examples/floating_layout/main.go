// The floating layout example shows how anchored overlays can be
// positioned relative to their parent content.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
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

type App struct{}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "Floating Layout",
		Width:  500,
		Height: 500,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		Content: []gui.View{
			// Faux menu bar
			gui.Row(gui.ContainerCfg{
				Color:  theme.ColorInterior,
				Sizing: gui.FillFit,
				VAlign: gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "File"}),
					fauxEditMenu(theme),
					gui.Rectangle(gui.RectangleCfg{Sizing: gui.FillFit}),
					gui.ThemePicker(gui.ThemePickerCfg{
						ID:          "theme-picker",
						IDFocus:     1,
						FloatAnchor: gui.FloatBottomRight,
						FloatTieOff: gui.FloatTopRight,
					}),
				},
			}),
			// Two-panel body
			gui.Row(gui.ContainerCfg{
				Padding: gui.NoPadding,
				Sizing:  gui.FillFill,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Color:    theme.ColorInterior,
						Sizing:   gui.FillFill,
						MinWidth: 100,
						MaxWidth: 150,
					}),
					gui.Column(gui.ContainerCfg{
						Color:    theme.ColorInterior,
						Sizing:   gui.FillFill,
						MinWidth: 100,
					}),
				},
			}),
			// Centered floating overlay
			gui.Column(gui.ContainerCfg{
				Float:       true,
				FloatAnchor: gui.FloatMiddleCenter,
				FloatTieOff: gui.FloatMiddleCenter,
				HAlign:      gui.HAlignCenter,
				Color:       theme.ColorActive,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Floating column with content",
						TextStyle: theme.B2,
					}),
					gui.Button(gui.ButtonCfg{
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "OK"}),
						},
					}),
				},
			}),
		},
	})
}

func fauxEditMenu(theme gui.Theme) gui.View {
	return gui.Column(gui.ContainerCfg{
		Spacing: gui.Some[float32](0),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Edit"}),
			gui.Column(gui.ContainerCfg{
				Float:       true,
				FloatAnchor: gui.FloatBottomLeft,
				MinWidth:    75,
				MaxWidth:    100,
				Color:       gui.RGBA(theme.ColorFocus.R, theme.ColorFocus.G, theme.ColorFocus.B, 210),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Cut"}),
					gui.Text(gui.TextCfg{Text: "Copy"}),
					gui.Row(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Padding: gui.NoPadding,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Paste >"}),
							gui.Column(gui.ContainerCfg{
								Float:        true,
								FloatAnchor:  gui.FloatMiddleRight,
								FloatOffsetX: 5,
								MinWidth:     75,
								MaxWidth:     100,
								Color:        gui.RGBA(theme.ColorFocus.R, theme.ColorFocus.G, theme.ColorFocus.B, 210),
								Content: []gui.View{
									gui.Text(gui.TextCfg{Text: "Clean"}),
									gui.Text(gui.TextCfg{Text: "Selection"}),
								},
							}),
						},
					}),
				},
			}),
		},
	})
}
