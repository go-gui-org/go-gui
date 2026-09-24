// This example demonstrates the built-in gradient directions and how to swap them at runtime.
// Gradient_demo shows the built-in gradient directions and how
// to swap them at runtime.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type App struct {
	Direction gui.GradientDirection
}

func directionName(d gui.GradientDirection) (string, bool) {
	switch d {
	case gui.GradientToTop:
		return "to_top", true
	case gui.GradientToTopRight:
		return "to_top_right", true
	case gui.GradientToRight:
		return "to_right", true
	case gui.GradientToBottomRight:
		return "to_bottom_right", true
	case gui.GradientToBottom:
		return "to_bottom", true
	case gui.GradientToBottomLeft:
		return "to_bottom_left", true
	case gui.GradientToLeft:
		return "to_left", true
	case gui.GradientToTopLeft:
		return "to_top_left", true
	default:
		return "", false
	}
}

func parseDirection(s string) (gui.GradientDirection, bool) {
	switch s {
	case "to_top":
		return gui.GradientToTop, true
	case "to_top_right":
		return gui.GradientToTopRight, true
	case "to_right":
		return gui.GradientToRight, true
	case "to_bottom_right":
		return gui.GradientToBottomRight, true
	case "to_bottom":
		return gui.GradientToBottom, true
	case "to_bottom_left":
		return gui.GradientToBottomLeft, true
	case "to_left":
		return gui.GradientToLeft, true
	case "to_top_left":
		return gui.GradientToTopLeft, true
	default:
		return gui.GradientDirection(0), false
	}
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeLight.WithPadding(false))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Direction: gui.GradientToBottom},
		Title:  "Gradient Demo",
		Width:  1000,
		Height: 800,
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

	// Static radial gradients, hoisted so one shared def serves every
	// frame. The linear ones stay built per frame: they take the live
	// direction, so they are not static.
	radialTall = radialGradient([]gui.GradientStop{
		{Color: magenta, Pos: 0},
		{Color: gui.Black, Pos: 1},
	})
	radialSquare = radialGradient([]gui.GradientStop{
		{Color: gui.Red, Pos: 0},
		{Color: gui.Green, Pos: 0.5},
		{Color: gui.Blue, Pos: 1},
	})
	radialWide = radialGradient([]gui.GradientStop{
		{Color: gui.Yellow, Pos: 0},
		{Color: cyan, Pos: 1},
	})
)

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	dir := app.Direction

	dirName, dirOK := directionName(dir)
	if !dirOK {
		// Unreachable: dir only ever holds a parsed direction, but
		// the radio group needs a string, so fall back explicitly.
		dirName = "to_bottom"
	}

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
		ID:         "gradient-scroll",
		Sizing:     gui.FillFill,
		Scrollable: true,
		ScrollbarCfgX: &gui.ScrollbarCfg{
			Overflow: gui.ScrollbarAuto,
		},
		ScrollbarCfgY: &gui.ScrollbarCfg{
			Overflow: gui.ScrollbarAuto,
		},
		Spacing: gui.Some[float32](40),
		Padding: gui.NewPadding(40, 40, 40, 40),
		Content: []gui.View{
			// Direction radio group
			gui.Column(gui.ContainerCfg{
				Spacing:    gui.Some[float32](10),
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Direction",
						TextStyle: theme.TextStyleTitleSmall,
					}),
					gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
						ID:          "gradient_demo_main_view",
						Value:       dirName,
						Options:     dirOptions,
						SizeBorder:  gui.Some[float32](1),
						ColorBorder: theme.ColorBorder,
						OnSelect: func(value string, ctx gui.EventCtx) {
							parsed, ok := parseDirection(value)
							if !ok {
								return
							}
							gui.State[App](ctx.Window).Direction = parsed
						},
					}),
				},
			}),

			// Linear gradients
			gui.Column(gui.ContainerCfg{
				Spacing:    gui.Some[float32](20),
				HAlign:     gui.HAlignCenter,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Linear Gradients",
						TextStyle: theme.TextStyleTitle,
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
				Color:  theme.ColorBorder,
				Sizing: gui.FitFill,
			}),

			// Radial gradients
			gui.Column(gui.ContainerCfg{
				Spacing:    gui.Some[float32](40),
				HAlign:     gui.HAlignCenter,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Radial Gradients",
						TextStyle: theme.TextStyleTitle,
					}),
					gui.Row(gui.ContainerCfg{
						Spacing: gui.Some[float32](30),
						Content: []gui.View{
							gradientBox(100, 300, 0,
								radialTall,
								nil, "Tall\n100x300", gui.White),
							gradientBox(200, 200, 0,
								radialSquare,
								nil, "Square\n200x200", gui.White),
						},
					}),
					gui.Row(gui.ContainerCfg{
						Spacing: gui.Some[float32](40),
						Content: []gui.View{
							gradientBox(300, 100, 0,
								radialWide,
								nil, "Wide 300x100", gui.Black),
						},
					}),
				},
			}),
		},
	})
}
