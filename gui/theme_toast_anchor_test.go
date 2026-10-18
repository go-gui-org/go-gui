package gui

import "testing"

// Tests for ThemeCfg.ToastAnchor (#1103). Before #735 an app set the
// anchor on the public toast style. #735 made that style private, so
// ThemeCfg is now the only way an app can set the anchor.

// TestThemeMakerToastAnchor checks that each anchor value in ThemeCfg
// gets to the toast style.
func TestThemeMakerToastAnchor(t *testing.T) {
	for _, a := range []ToastAnchor{
		ToastTopLeft, ToastTopRight, ToastBottomLeft, ToastBottomRight,
	} {
		cfg := ThemeDark.Cfg
		cfg.ToastAnchor = Some(a)
		if got := ThemeMaker(cfg).toastStyle.Anchor; got != a {
			t.Errorf("anchor %d: toastStyle.Anchor = %d", a, got)
		}
	}
}

// TestThemeMakerToastAnchorUnset checks the default. Unset must not
// read as the zero value, ToastTopLeft: that is why the field is Opt.
func TestThemeMakerToastAnchorUnset(t *testing.T) {
	if got := ThemeMaker(ThemeDark.Cfg).toastStyle.Anchor; got != ToastBottomRight {
		t.Errorf("toastStyle.Anchor = %d, want ToastBottomRight", got)
	}
}

// TestThemeMakerToastAnchorOutOfRange checks that a value with no
// case in toastContainerView falls back to the default.
// 4 is the first value past ToastBottomRight; 255 is the largest.
func TestThemeMakerToastAnchorOutOfRange(t *testing.T) {
	for _, a := range []ToastAnchor{ToastBottomRight + 1, 255} {
		cfg := ThemeDark.Cfg
		cfg.ToastAnchor = Some(a)
		if got := ThemeMaker(cfg).toastStyle.Anchor; got != ToastBottomRight {
			t.Errorf("anchor %d: toastStyle.Anchor = %d, want ToastBottomRight",
				a, got)
		}
	}
}

// TestThemeToastAnchorSurvivesRebuild checks that the anchor stays
// after each Theme method that builds the theme again from its Cfg.
// An in-place setter (the rejected design C in #1103) fails here.
func TestThemeToastAnchorSurvivesRebuild(t *testing.T) {
	cfg := ThemeDark.Cfg
	cfg.ToastAnchor = Some(ToastTopLeft)
	base := ThemeMaker(cfg)

	resized, err := base.AdjustFontSize(1, 6, 40)
	if err != nil {
		t.Fatalf("AdjustFontSize: %v", err)
	}
	for _, tc := range []struct {
		name  string
		theme Theme
	}{
		{"WithPadding(false)", base.WithPadding(false)},
		{"WithPadding(true)", base.WithPadding(true)},
		{"WithBorders(false)", base.WithBorders(false)},
		{"WithBorders(true)", base.WithBorders(true)},
		{"AdjustFontSize", resized},
	} {
		if got := tc.theme.toastStyle.Anchor; got != ToastTopLeft {
			t.Errorf("%s: toastStyle.Anchor = %d, want ToastTopLeft",
				tc.name, got)
		}
	}
}

// TestGoldenToastTopLeft records a toast with a top anchor, through the
// real frame pipeline, in both themes. The recording pins where the
// toast lands, which is what the anchor changes.
func TestGoldenToastTopLeft(t *testing.T) {
	// goldenFrame installs the theme through SetTheme, so put the
	// default theme back afterwards: later tests read the installed
	// mirrors against ThemeDark.
	restoreTheme(t)
	build := func(w *Window) View {
		// Seed the toast once, fully open. w.Toast would start the
		// enter animation and a dismiss timer, and the recording
		// would then depend on the clock.
		if len(w.toasts) == 0 {
			w.toasts = []toastNotification{{
				id:       1,
				cfg:      ToastCfg{Title: "Saved", Body: "File written"},
				animFrac: 1,
				phase:    toastVisible,
			}}
		}
		return Text(TextCfg{Text: "Body"})
	}
	c := goldenCase{name: "toast_top_left", build: build}
	for _, th := range goldenThemes() {
		cfg := th.theme.Cfg
		cfg.ToastAnchor = Some(ToastTopLeft)
		anchored := ThemeMaker(cfg)
		name := c.name + "." + th.name
		t.Run(name, func(t *testing.T) {
			got := renderGolden(t, anchored, c)
			checkGoldenFile(t, "testdata", name, got)
		})
	}
}
