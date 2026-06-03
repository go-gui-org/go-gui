// The rotated_box example demonstrates the RotatedBox widget
// with quarter-turn rotations, interactive content, and nesting.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

func main() {
	w := gui.NewWindow(gui.WindowCfg{
		Title:  "RotatedBox Demo",
		Width:  600,
		Height: 500,
		State:  &app{},
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})
	backend.Run(w)
}

type app struct {
	clicks int
}

func mainView(w *gui.Window) gui.View {
	state := gui.State[app](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SomeF(30),
		Padding: gui.SomeP(30, 30, 30, 30),
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "RotatedBox Demo",
				TextStyle: gui.TextStyle{Size: 24},
			}),

			// All four rotations side by side.
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				Spacing:    gui.SomeF(20),
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignMiddle,
				Content: []gui.View{
					rotatedLabel(0, "0°", gui.RGBA(100, 180, 255, 255)),
					rotatedLabel(1, "90°", gui.RGBA(100, 255, 100, 255)),
					rotatedLabel(2, "180°", gui.RGBA(255, 180, 100, 255)),
					rotatedLabel(3, "270°", gui.RGBA(255, 100, 180, 255)),
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
							Sizing:  gui.FitFit,
							Padding: gui.SomeP(8, 16, 8, 16),
							Color:   gui.RGBA(80, 120, 200, 255),
							Radius:  gui.SomeF(6),
							OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
								s := gui.State[app](w)
								s.clicks++
								w.UpdateView(mainView)
							},
							Content: []gui.View{
								gui.Text(gui.TextCfg{
									Text: "Click Me",
									TextStyle: gui.TextStyle{
										Color: gui.White,
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
								Padding:    gui.SomeP(6, 12, 6, 12),
								Color:      gui.RGBA(200, 100, 200, 255),
								SizeBorder: gui.NoBorder,
								Content: []gui.View{
									gui.Text(gui.TextCfg{
										Text: "Nested",
										TextStyle: gui.TextStyle{
											Color: gui.White,
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

func rotatedLabel(turns int, label string, bg gui.Color) gui.View {
	return gui.RotatedBox(gui.RotatedBoxCfg{
		QuarterTurns: turns,
		Content: gui.Row(gui.ContainerCfg{
			Sizing:     gui.FitFit,
			Padding:    gui.SomeP(8, 16, 8, 16),
			Color:      bg,
			Radius:     gui.SomeF(4),
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      label,
					TextStyle: gui.TextStyle{Color: gui.White},
				}),
			},
		}),
	})
}
