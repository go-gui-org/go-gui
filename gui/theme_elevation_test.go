package gui

import "testing"

// The elevation and focus-ring tokens are additive: a theme that does
// not set them must produce exactly the styles it produced before they
// existed. After visual-refresh §5.3 the dark/light presets and their
// derived twins carry elevation, so this guards the remaining
// elevation-free presets (the blue taste preset) — and every golden
// recorded against one — from moving.
func TestPresetThemesCarryNoElevation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		theme Theme
	}{
		{"blue", themeBlue},
		{"blue-bordered", themeBlueBordered},
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

// The dark and light presets carry the spec's elevation values (§5.3
// table), fanned out to the floating surfaces, and their § 5.4 focus
// rings. The derived presets (no-padding, bordered) copy their
// polarity's cfg, so this also pins the init order — an elevation or
// ring assignment moved after those copies would silently deflate the
// twins. Pinning the resolved style shadows keeps the table and the
// code from drifting the way TestThemeMakerAccentRamp pins the accent
// ramp.
func TestThemePresetElevationValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		theme       Theme
		wantPopover *BoxShadow
		wantDialog  *BoxShadow
		wantRing    *BoxShadow
	}{
		{"dark", ThemeDark, darkShadowPopover, darkShadowDialog, darkFocusRing},
		{"dark-no-padding", themeDarkNoPadding, darkShadowPopover, darkShadowDialog, darkFocusRing},
		{"dark-bordered", themeDarkBordered, darkShadowPopover, darkShadowDialog, darkFocusRing},
		{"light", ThemeLight, lightShadowPopover, lightShadowDialog, lightFocusRing},
		{"light-no-padding", themeLightNoPadding, lightShadowPopover, lightShadowDialog, lightFocusRing},
		{"light-bordered", themeLightBordered, lightShadowPopover, lightShadowDialog, lightFocusRing},
	} {
		for name, got := range popoverShadows(tc.theme) {
			if got != tc.wantPopover {
				t.Errorf("%s: %s = %v, want the popover tier %v",
					tc.name, name, got, tc.wantPopover)
			}
		}
		for name, got := range modalShadows(tc.theme) {
			if got != tc.wantDialog {
				t.Errorf("%s: %s = %v, want the dialog tier %v",
					tc.name, name, got, tc.wantDialog)
			}
		}
		if tc.theme.focusRing != tc.wantRing {
			t.Errorf("%s: focus ring = %v, want %v (visual-refresh § 5.4)",
				tc.name, tc.theme.focusRing, tc.wantRing)
		}
	}
}

// The spec table's numbers, asserted once so the consts and the
// documented values cannot drift.
func TestThemePresetElevationConsts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		got        *BoxShadow
		color      Color
		offsetY    float32
		blurRadius float32
	}{
		{darkShadowPopover, RGBA(0, 0, 0, 140), 4, 12},
		{darkShadowDialog, RGBA(0, 0, 0, 170), 12, 32},
		{lightShadowPopover, RGBA(16, 24, 40, 40), 4, 12},
		{lightShadowDialog, RGBA(16, 24, 40, 60), 12, 32},
	} {
		if tc.got.Color != tc.color ||
			tc.got.OffsetY != tc.offsetY ||
			tc.got.BlurRadius != tc.blurRadius {
			t.Errorf("shadow = %v, want %v/%v/%v",
				tc.got, tc.color, tc.offsetY, tc.blurRadius)
		}
	}
}

// Phase 4 dropped the macOS and GNOME radius overrides, so their
// ladders are the base's by inheritance — and Windows keeps its native
// large at 8. No golden renders a large-radius surface under a platform
// theme, so this is the only pin that catches a re-added override or a
// base-ladder move (visual-refresh §10: an unrecorded platform theme is
// where a base-ladder change silently breaks a hand-tuned override).
func TestPlatformRadiusLadder(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		theme Theme
		small float32
		med   float32
		large float32
	}{
		{"dark", ThemeDark, 4, 6, 12},
		{"macos", themeMacOS, 4, 6, 12},
		{"macos-dark", themeMacOSDark, 4, 6, 12},
		{"gnome", themeGnome, 4, 6, 12},
		{"gnome-dark", themeGnomeDark, 4, 6, 12},
		{"windows", themeWindows, 2, 4, 8},
		{"windows-dark", themeWindowsDark, 2, 4, 8},
	} {
		th := tc.theme
		if th.RadiusSmall != tc.small || th.RadiusMedium != tc.med ||
			th.RadiusLarge != tc.large {
			t.Errorf("%s: radius ladder = %v/%v/%v, want %v/%v/%v",
				tc.name, th.RadiusSmall, th.RadiusMedium, th.RadiusLarge,
				tc.small, tc.med, tc.large)
		}
	}
}

// popoverShadows and modalShadows name the styles each elevation tier
// fans out to. One shared list, so a style added to the fan-out can
// never miss its pin in either test.
func popoverShadows(th Theme) map[string]*BoxShadow {
	return map[string]*BoxShadow{
		"select":     th.selectStyle.Shadow,
		"combobox":   th.comboboxStyle.Shadow,
		"tooltip":    th.tooltipStyle.Shadow,
		"toast":      th.toastStyle.Shadow,
		"submenu":    th.MenubarStyle.Shadow,
		"datepicker": th.datePickerStyle.Shadow,
	}
}

func modalShadows(th Theme) map[string]*BoxShadow {
	return map[string]*BoxShadow{
		"dialog":  th.dialogStyle.Shadow,
		"palette": th.commandPaletteStyle.Shadow,
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

	for name, got := range popoverShadows(th) {
		if got != popover {
			t.Errorf("%s: got %p, want the popover token %p",
				name, got, popover)
		}
	}

	for name, got := range modalShadows(th) {
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
