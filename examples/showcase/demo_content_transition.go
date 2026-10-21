package main

import (
	"strconv"

	"github.com/go-gui-org/go-gui/gui"
)

// contentTransitionStatuses are the words the Fade row steps through.
var contentTransitionStatuses = []string{"Saved", "Saving", "Offline"}

// contentTransitionLabelWidth lines up the demos: every row's label
// takes this width.
const contentTransitionLabelWidth = 90

// demoContentTransition shows TextCfg.Transition: one row for each
// kind. TextCfg.Anim plays on its own schedule; a transition plays
// when the text changes, so each row has a control that changes it.
func demoContentTransition(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)

	likeIcon := gui.IconHeartO
	if app.ContentTransitionLiked {
		likeIcon = gui.IconHeart
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				ID: "ctrans-intro",
				Text: "TextCfg.Transition animates a change of the text. " +
					"The kind says how the old text gives way to the new " +
					"one. The text needs an ID: the last text shown is " +
					"keyed by identity.",
				TextStyle: t.TextStyleCaption,
				Mode:      gui.TextModeWrap,
			}),
			textDemoCard("", "One row for each kind", 0, []gui.View{
				contentTransitionRow(t, "Numeric",
					gui.Button(gui.ButtonCfg{
						ID:    "ctrans-dec",
						Label: "−",
						OnClick: func(ctx gui.EventCtx) {
							appState(ctx.Window).ContentTransitionCount--
							ctx.Consume()
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:    "ctrans-inc",
						Label: "+",
						OnClick: func(ctx gui.EventCtx) {
							appState(ctx.Window).ContentTransitionCount++
							ctx.Consume()
						},
					}),
					gui.Text(gui.TextCfg{
						ID:        "ctrans-count",
						Text:      strconv.Itoa(app.ContentTransitionCount),
						TextStyle: t.TextStyleDisplay,
						Transition: gui.ContentTransition{
							Kind: gui.ContentTransitionNumeric,
						},
					}),
				),
				contentTransitionRow(t, "Fade",
					gui.Button(gui.ButtonCfg{
						ID:    "ctrans-next",
						Label: "Next",
						OnClick: func(ctx gui.EventCtx) {
							a := appState(ctx.Window)
							a.ContentTransitionStatus = (a.ContentTransitionStatus + 1) %
								len(contentTransitionStatuses)
							ctx.Consume()
						},
					}),
					gui.Text(gui.TextCfg{
						ID:   "ctrans-status",
						Text: contentTransitionStatuses[app.ContentTransitionStatus],
						Transition: gui.ContentTransition{
							Kind: gui.ContentTransitionFade,
						},
					}),
				),
				contentTransitionRow(t, "Replace",
					gui.Button(gui.ButtonCfg{
						ID:      "ctrans-like",
						A11YCfg: gui.A11YCfg{A11YLabel: "Like"},
						Content: []gui.View{gui.Text(gui.TextCfg{
							ID:        "ctrans-like-icon",
							Text:      likeIcon,
							TextStyle: t.TextStyleIconMedium,
							Transition: gui.ContentTransition{
								Kind: gui.ContentTransitionReplace,
							},
						})},
						OnClick: func(ctx gui.EventCtx) {
							a := appState(ctx.Window)
							a.ContentTransitionLiked = !a.ContentTransitionLiked
							ctx.Consume()
						},
					}),
				),
			}),
		},
	})
}

// contentTransitionRow is one row of the demo: the name of a transition
// kind, then the widgets that show it.
func contentTransitionRow(t gui.Theme, kind string, demo ...gui.View) gui.View {
	content := []gui.View{
		gui.Column(gui.ContainerCfg{
			Sizing:     gui.FixedFit,
			Width:      contentTransitionLabelWidth,
			Padding:    gui.NoPadding,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{gui.Text(gui.TextCfg{
				Text:      kind,
				TextStyle: t.TextStyleLabel,
			})},
		}),
	}
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SpacingMedium,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		VAlign:     gui.VAlignMiddle,
		Content:    append(content, demo...),
	})
}
