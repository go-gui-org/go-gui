package gui

import "testing"

// The elevation and focus-ring tokens are additive: a theme that does
// not set them must produce exactly the styles it produced before they
// existed. This is the guard that keeps every pre-existing theme — and
// every golden recorded against one — from moving.
func TestPresetThemesCarryNoElevation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		theme Theme
	}{
		{"dark", ThemeDark},
		{"light", ThemeLight},
		{"blue", themeBlue},
		{"dark-no-padding", themeDarkNoPadding},
		{"light-bordered", themeLightBordered},
	} {
		for name, got := range map[string]*BoxShadow{
			"dialog":     tc.theme.dialogStyle.Shadow,
			"select":     tc.theme.selectStyle.Shadow,
			"combobox":   tc.theme.comboboxStyle.Shadow,
			"datepicker": tc.theme.datePickerStyle.Shadow,
			"tooltip":    tc.theme.tooltipStyle.Shadow,
			"toast":      tc.theme.toastStyle.Shadow,
			"palette":    tc.theme.commandPaletteStyle.Shadow,
			"submenu":    tc.theme.MenubarStyle.Shadow,
			"focus ring": tc.theme.focusRing,
		} {
			if got != nil {
				t.Errorf("%s: %s shadow set, want nil", tc.name, name)
			}
		}
	}
}

// ThemeMaker fans one popover token out to every popover style and one
// dialog token to the modal styles. Asserting the fan-out rather than
// each widget separately is what stops a newly added popover from
// quietly missing its elevation.
func TestThemeMakerFansOutElevation(t *testing.T) {
	t.Parallel()
	popover := &BoxShadow{Color: RGBA(0, 0, 0, 80), BlurRadius: 10}
	dialog := &BoxShadow{Color: RGBA(0, 0, 0, 120), BlurRadius: 30}
	ring := &BoxShadow{Color: RGBA(0, 122, 255, 90), BlurRadius: 3}

	cfg := baseDarkCfg()
	cfg.Name = "elevation-test"
	cfg.ShadowPopover = popover
	cfg.ShadowDialog = dialog
	cfg.FocusRing = ring
	th := ThemeMaker(cfg)

	popovers := map[string]*BoxShadow{
		"select":     th.selectStyle.Shadow,
		"combobox":   th.comboboxStyle.Shadow,
		"tooltip":    th.tooltipStyle.Shadow,
		"toast":      th.toastStyle.Shadow,
		"submenu":    th.MenubarStyle.Shadow,
		"datepicker": th.datePickerStyle.Shadow,
	}
	for name, got := range popovers {
		if got != popover {
			t.Errorf("%s: got %p, want the popover token %p",
				name, got, popover)
		}
	}

	modals := map[string]*BoxShadow{
		"dialog":  th.dialogStyle.Shadow,
		"palette": th.commandPaletteStyle.Shadow,
	}
	for name, got := range modals {
		if got != dialog {
			t.Errorf("%s: got %p, want the dialog token %p",
				name, got, dialog)
		}
	}

	if th.focusRing != ring {
		t.Errorf("focus ring: got %p, want %p", th.focusRing, ring)
	}
}

// The platform themes are the reason the tokens exist, so they must
// actually carry them, and must be reachable by name like every other
// preset. GNOME and Windows keep the hard accent outline for focus
// (ColorBorderFocus) rather than the macOS glow, so they must NOT
// carry a ring.
func TestPlatformThemesRegisteredAndElevated(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		wantRing bool
	}{
		{"macos", true},
		{"macos-dark", true},
		{"gnome", false},
		{"gnome-dark", false},
		{"windows", false},
		{"windows-dark", false},
	} {
		th, ok := ThemeGet(tc.name)
		if !ok {
			t.Fatalf("%q not registered", tc.name)
		}
		if th.Name != tc.name {
			t.Errorf("%q: Name is %q", tc.name, th.Name)
		}
		if th.dialogStyle.Shadow == nil {
			t.Errorf("%q: dialog has no elevation", tc.name)
		}
		if th.selectStyle.Shadow == nil {
			t.Errorf("%q: dropdown has no elevation", tc.name)
		}
		if got := th.focusRing != nil; got != tc.wantRing {
			t.Errorf("%q: focus ring present=%v, want %v",
				tc.name, got, tc.wantRing)
		}
	}

	// The macOS ring is tinted with the theme's own accent, so the two
	// polarities must not share one value.
	if themeMacOS.focusRing == themeMacOSDark.focusRing {
		t.Error("light and dark share a focus ring; accent differs")
	}
}

// A popover shadow has to survive as far as a RenderShadow command,
// not merely be present on the style struct.
func TestPopoverShadowReachesRenderer(t *testing.T) {
	t.Parallel()
	w := makeWindow()
	shadow := &BoxShadow{Color: RGBA(0, 0, 0, 80), OffsetY: 5, BlurRadius: 20}
	s := &Shape{
		shapeType: shapeRectangle,
		X:         10,
		Y:         20,
		Width:     120,
		Height:    90,
		Color:     RGB(255, 255, 255),
		Radius:    6,
		fx:        &shapeEffects{Shadow: shadow},
	}
	renderContainer(s, ColorTransparent, makeClip(0, 0, 500, 500), w)

	if len(w.renderers) == 0 {
		t.Fatal("no render commands emitted")
	}
	// Shadow first, so it lands behind the popover's own fill rather
	// than over it.
	if got := w.renderers[0].Kind; got != RenderShadow {
		t.Fatalf("first command is %v, want RenderShadow", got)
	}
	if w.renderers[0].BlurRadius != shadow.BlurRadius {
		t.Errorf("blur: got %v, want %v",
			w.renderers[0].BlurRadius, shadow.BlurRadius)
	}
}
