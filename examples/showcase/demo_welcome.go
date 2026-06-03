package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/highlight"
)

func demoWelcome(w *gui.Window) gui.View {
	return showcaseMarkdownPanel(w, "showcase-welcome", docPageSource("welcome"))
}

func showcaseMarkdownPanel(w *gui.Window, id, source string) gui.View {
	style := gui.DefaultMarkdownStyle()
	style.CodeHighlighter = highlight.Default()
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Padding:    gui.Some(gui.PaddingSmall),
		SizeBorder: gui.NoBorder,
		Color:      gui.CurrentTheme().ColorPanel,
		Content: []gui.View{
			w.Markdown(gui.MarkdownCfg{
				ID:      id,
				Style:   style,
				Source:  source,
				Mode:    gui.Some(gui.TextModeWrap),
				Padding: gui.NoPadding,
			}),
		},
	})
}
