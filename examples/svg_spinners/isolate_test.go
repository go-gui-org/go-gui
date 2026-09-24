// Isolation harness — renders one spinner centered; used to rule
// out many-spinner layout cost as the cause of mouse-move animation
// pauses. Kept in a _test.go file so it never ships in the prod
// binary.
package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

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

func TestIsolatedViewNoPanic(t *testing.T) {
	// Not parallel: the tests in this package share process-global
	// theme state via gui.SetTheme, so parallel subtests race.
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		Width:  520,
		Height: 640,
	})
	_ = isolatedView(w).GenerateLayout(w)
}
