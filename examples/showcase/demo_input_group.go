package main

import "github.com/go-gui-org/go-gui/gui"

// demoInputGroup shows InputGroup (issue #820): controls joined into
// one shape with shared borders and text addons.
func demoInputGroup(w *gui.Window) gui.View {
	app := appState(w)
	titleStyle := gui.CurrentTheme().TextStyleTitleSmall

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Prefix and suffix addons", TextStyle: titleStyle}),
			gui.InputGroup(gui.InputGroupCfg{
				ID:      "ig_user",
				A11YCfg: gui.A11YCfg{A11YLabel: "Email address"},
				Segments: []gui.InputGroupSegment{
					gui.InputGroupText(gui.InputGroupTextCfg{Text: "@"}),
					gui.InputGroupInput(gui.InputCfg{
						ID:          "user",
						Text:        app.InputGroupUser,
						Placeholder: "user",
						OnTextChanged: func(s string, ctx gui.EventCtx) {
							appState(ctx.Window).InputGroupUser = s
						},
					}),
					gui.InputGroupSelect(gui.SelectCfg{
						ID:       "domain",
						Selected: []string{app.InputGroupDomain},
						Options: []gui.SelectOption{
							gui.NewSelectOption("example.com", "example.com"),
							gui.NewSelectOption("example.org", "example.org"),
						},
						OnSelect: func(sel []string, ctx gui.EventCtx) {
							appState(ctx.Window).InputGroupDomain = sel[0]
							ctx.Consume()
						},
					}),
				},
			}),

			gui.Text(gui.TextCfg{Text: "Currency", TextStyle: titleStyle}),
			gui.InputGroup(gui.InputGroupCfg{
				ID:      "ig_amount",
				A11YCfg: gui.A11YCfg{A11YLabel: "Amount"},
				Segments: []gui.InputGroupSegment{
					gui.InputGroupText(gui.InputGroupTextCfg{Text: "$"}),
					gui.InputGroupInput(gui.InputCfg{
						ID:          "amount",
						Text:        app.InputGroupAmount,
						Placeholder: "0",
						OnTextChanged: func(s string, ctx gui.EventCtx) {
							appState(ctx.Window).InputGroupAmount = s
						},
					}),
					gui.InputGroupText(gui.InputGroupTextCfg{Text: ".00"}),
				},
			}),

			gui.Text(gui.TextCfg{Text: "Fill width with a button", TextStyle: titleStyle}),
			gui.InputGroup(gui.InputGroupCfg{
				ID:     "ig_search",
				Sizing: gui.FillFit,
				Segments: []gui.InputGroupSegment{
					gui.InputGroupInput(gui.InputCfg{
						ID:          "query",
						Text:        app.InputGroupSearch,
						Placeholder: "Search",
						Sizing:      gui.FillFit,
						OnTextChanged: func(s string, ctx gui.EventCtx) {
							appState(ctx.Window).InputGroupSearch = s
						},
					}),
					gui.InputGroupButton(gui.ButtonCfg{
						ID:      "search_go",
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Search"})},
						OnClick: func(ctx gui.EventCtx) {
							appState(ctx.Window).InputGroupSearch = ""
							ctx.Consume()
						},
					}),
				},
			}),

			gui.Text(gui.TextCfg{Text: "Disabled", TextStyle: titleStyle}),
			gui.InputGroup(gui.InputGroupCfg{
				ID:       "ig_disabled",
				Disabled: true,
				Segments: []gui.InputGroupSegment{
					gui.InputGroupText(gui.InputGroupTextCfg{Text: "https://"}),
					gui.InputGroupInput(gui.InputCfg{ID: "url", Text: "go-gui.org"}),
				},
			}),
		},
	})
}
