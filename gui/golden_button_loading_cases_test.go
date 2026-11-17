package gui

// goldenButtonLoadingCases records ButtonCfg.Loading (#1264): the
// label is laid out at opacity 0, so it keeps the button's width and
// draws nothing, and the spinner is drawn where it was. The golden window has no SVG parser; build
// installs the mock, or the spinner would record as a "[missing: …]"
// text.
func goldenButtonLoadingCases() []goldenCase {
	return []goldenCase{
		{
			name: "button_loading",
			build: func(w *Window) View {
				w.SetSvgParser(&mockSvgParser{width: 24, height: 24})
				return Button(ButtonCfg{
					ID:      "btn",
					Loading: true,
					Label:   "Save",
					OnClick: func(EventCtx) {},
				})
			},
		},
		{
			// Loading keeps focus and the focus ring, and a filled
			// variant's spinner takes the variant's label color.
			name:    "button_loading_focus",
			focusID: "btn",
			build: func(w *Window) View {
				w.SetSvgParser(&mockSvgParser{width: 24, height: 24})
				return Button(ButtonCfg{
					ID:      "btn",
					Loading: true,
					Variant: ButtonPrimary,
					Label:   "Save",
					OnClick: func(EventCtx) {},
				})
			},
		},
	}
}
