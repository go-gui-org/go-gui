package main

import "github.com/go-gui-org/go-gui/gui"

var thinkingOrbDesigns = []struct {
	label  string
	design gui.ThinkingOrbDesign
	size   gui.ThinkingOrbSize
	status string
}{
	{"Working", gui.ThinkingOrbWorking, gui.ThinkingOrbRegular, "Working…"},
	{"Searching", gui.ThinkingOrbSearching, gui.ThinkingOrbRegular, "Searching the web…"},
	{"Solving", gui.ThinkingOrbSolving, gui.ThinkingOrbRegular, "Solving…"},
	{"Listening", gui.ThinkingOrbListening, gui.ThinkingOrbRegular, "Listening…"},
	{"Connecting", gui.ThinkingOrbConnecting, gui.ThinkingOrbRegular, "Syncing…"},
	{"Weaving", gui.ThinkingOrbWeaving, gui.ThinkingOrbRegular, "Planning…"},
	{"Composing", gui.ThinkingOrbComposing, gui.ThinkingOrbRegular, "Writing a reply…"},
	{"Breathing", gui.ThinkingOrbBreathing, gui.ThinkingOrbRegular, "Thinking…"},
	{"Shaping", gui.ThinkingOrbShaping, gui.ThinkingOrbRegular, "Shaping…"},
}

func demoThinkingOrb(w *gui.Window) gui.View {
	t := gui.CurrentTheme()

	cells := make([]gui.View, len(thinkingOrbDesigns))
	for i, e := range thinkingOrbDesigns {
		cells[i] = gui.Column(gui.ContainerCfg{
			Sizing:  gui.FitFit,
			Padding: t.PaddingMedium,

			SizeBorder: gui.NoBorder,
			HAlign:     gui.HAlignCenter,
			Spacing:    gui.SomeF(6),
			Content: []gui.View{
				gui.ThinkingOrb(gui.ThinkingOrbCfg{
					ID:     gui.ScopeIDN("thinking-orb", "hero", i),
					Design: e.design,
					Size:   e.size,
				}),
				gui.ThinkingOrb(gui.ThinkingOrbCfg{
					ID:     gui.ScopeIDN("thinking-orb", "inline", i),
					Design: e.design,
					Size:   gui.ThinkingOrbSmall,
				}),
				gui.Text(gui.TextCfg{
					Text:      e.label,
					TextStyle: t.TextStyleBodySmall,
				}),
			},
		})
	}

	labels := make([]gui.View, len(thinkingOrbDesigns))
	for i, e := range thinkingOrbDesigns {
		labels[i] = gui.ThinkingOrbLabel(gui.ThinkingOrbLabelCfg{
			ID:     gui.ScopeIDN("thinking-orb", "label", i),
			Text:   e.status,
			Design: e.design,
			Size:   gui.ThinkingOrbSmall,
		})
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(12),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Nine semantic states, regular and small",
				TextStyle: t.TextStyleBody,
			}),
			gui.Wrap(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(8),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content:    cells,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Status labels",
				TextStyle: t.TextStyleBody,
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(8),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content:    labels,
			}),
		},
	})
}
