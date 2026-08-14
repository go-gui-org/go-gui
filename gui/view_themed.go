package gui

// themedView scopes a theme to the subtree its builder produces.
type themedView struct {
	build func(*Window) View
	theme Theme
}

// Themed scopes t to one subtree: the views build returns resolve their
// theme defaults from t, and everything outside keeps the window's theme.
// Themed nests — an inner Themed restores the outer theme on the way out.
//
// build runs at layout-generation time, and that is not sugar. Widget
// factories resolve defaults when they are called, so a signature taking
// ready-made child views would receive children already built under the
// enclosing theme. Deferring construction is what makes the scope work.
//
//	gui.Themed(gui.ThemeGet("light"), func(w *gui.Window) gui.View {
//	    return gui.Column(gui.ContainerCfg{Content: []gui.View{
//	        gui.Button(gui.ButtonCfg{ID: "ok", Text: "OK"}),
//	    }})
//	})
//
// The scope covers generation only. The few theme reads that happen after
// generation — scroll metrics on the event path, the theme picker's
// report of the active theme — stay window-scoped.
func Themed(t Theme, build func(w *Window) View) View {
	return themedView{theme: t, build: build}
}

// GenerateLayout installs the scoped theme, builds and generates the
// subtree under it, then restores the theme it displaced.
func (v themedView) GenerateLayout(w *Window) Layout {
	if v.build == nil {
		return Layout{}
	}
	prev := pushTheme(v.theme)
	defer popTheme(prev)

	inner := v.build(w)
	if inner == nil {
		return Layout{}
	}
	return generateViewLayout(inner, w)
}
