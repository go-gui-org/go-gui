// Isolation harness — flip useIsolation to true and rebuild to
// render one spinner centered; used to rule out 110-spinner
// layout cost as the cause of mouse-move animation pauses.
package main

import "github.com/go-gui-org/go-gui/gui"

const useIsolation = false

func isolatedView(w *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Padding: gui.PaddingSmall,

		Content: []gui.View{
			gui.SvgSpinner(gui.SvgSpinnerCfg{
				Kind:   gui.SvgSpinner90Ring,
				Width:  128,
				Height: 128,
			}),
		},
	})
}
