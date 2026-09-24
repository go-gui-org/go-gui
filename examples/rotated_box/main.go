// This example demonstrates the RotatedBox widget with quarter-turn rotations, interactive content, and nesting.
// The rotated_box example demonstrates the RotatedBox widget
// with quarter-turn rotations, interactive content, and nesting.
package main

import (
	"fmt"

	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "RotatedBox Demo",
		Width:  600,
		Height: 500,
		State:  &app{},
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
	clicks int
}

func mainView(w *gui.Window) gui.View {
	state := gui.State[app](w)
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SomeF(30),
		Padding: gui.NewPadding(30, 30, 30, 30),
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "RotatedBox Demo",
				TextStyle: theme.TextStyleTitle,
			}),

			// All four rotations side by side.
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				Spacing:    gui.SomeF(20),
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignMiddle,
				Content: []gui.View{
					rotatedLabel(0, "0°", gui.RGB(100, 180, 255), theme),
					rotatedLabel(1, "90°", gui.RGB(100, 255, 100), theme),
					rotatedLabel(2, "180°", gui.RGB(255, 180, 100), theme),
					rotatedLabel(3, "270°", gui.RGB(255, 100, 180), theme),
				},
			}),

			// Interactive: rotated button.
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				Spacing:    gui.SomeF(20),
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: "Interactive:",
					}),
					gui.RotatedBox(gui.RotatedBoxCfg{
						QuarterTurns: 1,
						Content: gui.Row(gui.ContainerCfg{
							Sizing:     gui.FitFit,
							Padding:    gui.NewPadding(8, 16, 8, 16),
							Color:      gui.RGB(80, 120, 200),
							Radius:     gui.SomeF(6),
							SizeBorder: gui.NoBorder,
							OnClick: func(ctx gui.EventCtx) {
								s := gui.State[app](ctx.Window)
								s.clicks++
							},
							Content: []gui.View{
								gui.Text(gui.TextCfg{
									Text: "Click Me",
									TextStyle: gui.TextStyle{
										Color: theme.ColorTextOnAccent,
									},
								}),
							},
						}),
					}),
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("Clicks: %d", state.clicks),
					}),
				},
			}),

			// Nested rotation: 90° + 90° = 180° visual.
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				Spacing:    gui.SomeF(20),
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: "Nested (90°+90°=180°):",
					}),
					gui.RotatedBox(gui.RotatedBoxCfg{
						QuarterTurns: 1,
						Content: gui.RotatedBox(gui.RotatedBoxCfg{
							QuarterTurns: 1,
							Content: gui.Row(gui.ContainerCfg{
								Sizing:     gui.FitFit,
								Padding:    gui.NewPadding(6, 12, 6, 12),
								Color:      gui.RGB(200, 100, 200),
								SizeBorder: gui.NoBorder,
								Content: []gui.View{
									gui.Text(gui.TextCfg{
										Text: "Nested",
										TextStyle: gui.TextStyle{
											Color: theme.ColorTextOnAccent,
										},
									}),
								},
							}),
						}),
					}),
				},
			}),
		},
	})
}

func rotatedLabel(turns int, label string, bg gui.Color, theme gui.Theme) gui.View {
	return gui.RotatedBox(gui.RotatedBoxCfg{
		QuarterTurns: turns,
		Content: gui.Row(gui.ContainerCfg{
			Sizing:     gui.FitFit,
			Padding:    gui.NewPadding(8, 16, 8, 16),
			Color:      bg,
			Radius:     gui.SomeF(4),
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      label,
					TextStyle: gui.TextStyle{Color: theme.ColorTextOnAccent},
				}),
			},
		}),
	})
}
