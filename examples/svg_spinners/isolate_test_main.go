// Isolation harness — flip useIsolation to true and rebuild to
// render one spinner centered; used to rule out 110-spinner
// layout cost as the cause of mouse-move animation pauses.
package main

import "github.com/go-gui-org/go-gui/gui"

const useIsolation = false

func isolatedView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Padding: gui.Some(gui.PaddingSmall),
		Content: []gui.View{
			gui.SvgSpinner(gui.SvgSpinnerCfg{
				Kind:   gui.SvgSpinner90Ring,
				Width:  128,
				Height: 128,
			}),
		},
	})
}
