// Gradient_demo shows the built-in gradient directions and how
// to swap them at runtime.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	Direction gui.GradientDirection
}

func directionName(d gui.GradientDirection) string {
	switch d {
	case gui.GradientToTop:
		return "to_top"
	case gui.GradientToTopRight:
		return "to_top_right"
	case gui.GradientToRight:
		return "to_right"
	case gui.GradientToBottomRight:
		return "to_bottom_right"
	case gui.GradientToBottom:
		return "to_bottom"
	case gui.GradientToBottomLeft:
		return "to_bottom_left"
	case gui.GradientToLeft:
		return "to_left"
	case gui.GradientToTopLeft:
		return "to_top_left"
	default:
		return "to_bottom"
	}
}

func parseDirection(s string) gui.GradientDirection {
	switch s {
	case "to_top":
		return gui.GradientToTop
	case "to_top_right":
		return gui.GradientToTopRight
	case "to_right":
		return gui.GradientToRight
	case "to_bottom_right":
		return gui.GradientToBottomRight
	case "to_bottom":
		return gui.GradientToBottom
	case "to_bottom_left":
		return gui.GradientToBottomLeft
	case "to_left":
		return gui.GradientToLeft
	case "to_top_left":
		return gui.GradientToTopLeft
	default:
		return gui.GradientToBottom
	}
}

func main() {
	gui.SetTheme(gui.ThemeLight.WithPadding(false))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Direction: gui.GradientToBottom},
		Title:  "Gradient Demo",
		Width:  1000,
		Height: 800,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func linearGradient(dir gui.GradientDirection, c1, c2 gui.Color) *gui.GradientDef {
	return &gui.GradientDef{
		Direction: dir,
		Stops: []gui.GradientStop{
			{Color: c1, Pos: 0},
			{Color: c2, Pos: 1},
		},
	}
}

func radialGradient(stops []gui.GradientStop) *gui.GradientDef {
	return &gui.GradientDef{
		Type:  gui.GradientRadial,
		Stops: stops,
	}
}

func gradientBox(w, h, radius float32, grad *gui.GradientDef,
	shadow *gui.BoxShadow, label string, textColor gui.Color) gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:    w,
		Height:   h,
		Radius:   gui.Some(radius),
		Gradient: grad,
		Shadow:   shadow,
		HAlign:   gui.HAlignCenter,
		VAlign:   gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: label,
				TextStyle: gui.TextStyle{
					Color: textColor,
					Align: gui.TextAlignCenter,
				},
			}),
		},
	})
}

var (
	magenta = gui.RGBA(255, 0, 255, 255)
	cyan    = gui.RGBA(0, 255, 255, 255)
)

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)
	dir := app.Direction

	dirOptions := []gui.RadioOption{
		gui.NewRadioOption("to_top", "to_top"),
		gui.NewRadioOption("to_top_right", "to_top_right"),
		gui.NewRadioOption("to_right", "to_right"),
		gui.NewRadioOption("to_bottom_right", "to_bottom_right"),
		gui.NewRadioOption("to_bottom", "to_bottom"),
		gui.NewRadioOption("to_bottom_left", "to_bottom_left"),
		gui.NewRadioOption("to_left", "to_left"),
		gui.NewRadioOption("to_top_left", "to_top_left"),
	}

	return gui.Row(gui.ContainerCfg{
		Width:    float32(ww),
		Height:   float32(wh),
		Sizing:   gui.FixedFixed,
		IDScroll: 1,
		ScrollbarCfgY: &gui.ScrollbarCfg{
			Overflow: gui.ScrollbarAuto,
		},
		Spacing: gui.Some[float32](40),
		Padding: gui.SomeP(40, 40, 40, 40),
		Content: []gui.View{
			// Direction radio group
			gui.Column(gui.ContainerCfg{
				Spacing: gui.Some[float32](10),
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: "Direction",
						TextStyle: gui.TextStyle{
							Size: 20,
						},
					}),
					gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
						Value:       directionName(dir),
						Options:     dirOptions,
						IDFocus:     1,
						SizeBorder:  gui.Some[float32](1),
						ColorBorder: gui.DarkGray,
						OnSelect: func(value string, w *gui.Window) {
							gui.State[App](w).Direction =
								parseDirection(value)
						},
					}),
				},
			}),

			// Linear gradients
			gui.Column(gui.ContainerCfg{
				Spacing: gui.Some[float32](20),
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Linear Gradients",
						TextStyle: gui.TextStyle{Size: 30},
					}),
					gradientBox(200, 150, 15,
						linearGradient(dir, gui.Blue, gui.Purple),
						nil, "Blue -> Purple", gui.White),
					gradientBox(200, 150, 15,
						linearGradient(dir, gui.Red, gui.Orange),
						nil, "Red -> Orange", gui.White),
					gradientBox(200, 150, 15,
						linearGradient(dir, gui.Green, gui.Blue),
						&gui.BoxShadow{
							BlurRadius: 20,
							Color:      gui.RGBA(0, 0, 0, 50),
							OffsetY:    5,
						},
						"Gradient + Shadow", gui.White),
				},
			}),

			// Vertical divider
			gui.Rectangle(gui.RectangleCfg{
				Width:  3,
				Color:  gui.Gray,
				Sizing: gui.FitFill,
			}),

			// Radial gradients
			gui.Column(gui.ContainerCfg{
				Spacing: gui.Some[float32](40),
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Radial Gradients",
						TextStyle: gui.TextStyle{Size: 30},
					}),
					gui.Row(gui.ContainerCfg{
						Spacing: gui.Some[float32](30),
						Content: []gui.View{
							gradientBox(100, 300, 0,
								radialGradient([]gui.GradientStop{
									{Color: magenta, Pos: 0},
									{Color: gui.Black, Pos: 1},
								}),
								nil, "Tall\n100x300", gui.White),
							gradientBox(200, 200, 0,
								radialGradient([]gui.GradientStop{
									{Color: gui.Red, Pos: 0},
									{Color: gui.Green, Pos: 0.5},
									{Color: gui.Blue, Pos: 1},
								}),
								nil, "Square\n200x200", gui.White),
						},
					}),
					gui.Row(gui.ContainerCfg{
						Spacing: gui.Some[float32](40),
						Content: []gui.View{
							gradientBox(300, 100, 0,
								radialGradient([]gui.GradientStop{
									{Color: gui.Yellow, Pos: 0},
									{Color: cyan, Pos: 1},
								}),
								nil, "Wide 300x100", gui.Black),
						},
					}),
				},
			}),
		},
	})
}
