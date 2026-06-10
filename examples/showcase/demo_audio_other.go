//go:build !audio || js || android || ios

package main

import "github.com/go-gui-org/go-gui/gui"

func demoAudio(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Audio disabled (build with -tags audio). Requires SDL2_mixer.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}
