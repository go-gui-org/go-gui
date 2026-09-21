package gui

// Test seams for injecting a style into a Theme.
//
// This file once held one setter per widget style — about thirty of
// them, each a copy of the two lines below. None had a production
// caller: the exported With* API a reader would expect is WithColors,
// WithPadding, WithBorders and AdjustFontSize, all of which rebuild
// through ThemeMaker. The setters here do not, so a style pushed
// through one is dropped by the next rebuild; that is tolerable in a
// test that renders one frame and wrong everywhere else.
//
// The two that a test actually needs are kept and the rest are gone. A
// new one belongs here only if a test needs it; production code sets a
// style by building the Theme with the Cfg that produces it.

// withContainerStyle returns a Theme carrying the given container
// style.
func (t Theme) withContainerStyle(s containerStyle) Theme {
	t.ContainerStyle = s
	t.id = nextThemeID()
	return t
}

// withInputStyle returns a Theme carrying the given input style.
func (t Theme) withInputStyle(s InputStyle) Theme {
	t.InputStyle = s
	t.id = nextThemeID()
	return t
}
