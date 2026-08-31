//go:build js || android || ios

package main

import "github.com/go-gui-org/go-gui/gui"

func demoAudio(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Audio not available on this platform.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}

// widgetSoundControls has no audio backend on these targets, so the
// Sound Feedback page shows why rather than a dead switch.
func widgetSoundControls(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Text(gui.TextCfg{
		Text:      "Widget sound is not available on this platform.",
		TextStyle: t.N4,
		Mode:      gui.TextModeWrap,
	})
}
