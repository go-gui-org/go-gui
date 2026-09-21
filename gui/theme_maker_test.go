package gui

import "testing"

// iconStyleFamilies returns every theme-driven icon style family so the
// tests can assert them as a set.
func iconStyleFamilies(theme Theme) []string {
	return []string{
		theme.TextStyleIconXLarge.Family,
		theme.TextStyleIconLarge.Family,
		theme.TextStyleIconMedium.Family,
		theme.TextStyleIconSmall.Family,
		theme.TextStyleIconXSmall.Family,
		theme.TextStyleIconTiny.Family,
		theme.treeStyle.textStyleIcon.Family,
	}
}

func TestThemeMakerIconFamilyDefault(t *testing.T) {
	theme := ThemeMaker(baseCfg())
	for i, got := range iconStyleFamilies(theme) {
		if got != IconFontName {
			t.Errorf("icon style %d family = %q, want %q",
				i, got, IconFontName)
		}
	}
}

func TestThemeMakerIconFamilyOverride(t *testing.T) {
	cfg := baseCfg()
	cfg.IconFontFamily = "mycustomicons"
	theme := ThemeMaker(cfg)
	for i, got := range iconStyleFamilies(theme) {
		if got != "mycustomicons" {
			t.Errorf("icon style %d family = %q, want %q",
				i, got, "mycustomicons")
		}
	}
}

// A ThemeCfg built from scratch has no IconFontFamily; icons must still
// resolve to the bundled font rather than the default text family.
func TestThemeMakerIconFamilyEmptyFallsBack(t *testing.T) {
	theme := ThemeMaker(ThemeCfg{})
	for i, got := range iconStyleFamilies(theme) {
		if got != IconFontName {
			t.Errorf("icon style %d family = %q, want %q",
				i, got, IconFontName)
		}
	}
}

func TestThemeMakerMonoFamily(t *testing.T) {
	cfg := baseCfg()
	cfg.MonoFontFamily = "mycustommono"
	theme := ThemeMaker(cfg)
	styles := []struct {
		name  string
		style TextStyle
	}{
		{"Code", theme.TextStyleCode},
		{"CodeSmall", theme.TextStyleCodeSmall},
		{"CodeTiny", theme.TextStyleCodeTiny},
		{"Mono donor", theme.Mono(theme.TextStyleBody)},
	}
	for _, s := range styles {
		if s.style.Family != "mycustommono" {
			t.Errorf("%s family = %q, want %q",
				s.name, s.style.Family, "mycustommono")
		}
	}
}

// The icon family must retarget only the icon styles. A leak into the
// plain/italic/bold or mono styles renders body text in an icon font —
// no crash, just unreadable output, so lock it down here.
func TestThemeMakerIconFamilyDoesNotLeak(t *testing.T) {
	cfg := baseCfg()
	cfg.TextStyleDef.Family = "mytextfamily"
	cfg.MonoFontFamily = "mymonofamily"
	cfg.IconFontFamily = "myiconfamily"
	theme := ThemeMaker(cfg)

	text := map[string]TextStyle{
		"Italic":       theme.TextStyleBody.Italic(),
		"Italic small": theme.TextStyleCaptionSmall.Italic(),
		"BoldItalic":   theme.TextStyleBody.Italic().Bold(),
		"BI small":     theme.TextStyleCaptionSmall.Italic().Bold(),
	}
	for name, s := range text {
		if s.Family != "mytextfamily" {
			t.Errorf("%s family = %q, want %q",
				name, s.Family, "mytextfamily")
		}
	}
	if theme.TextStyleCode.Family != "mymonofamily" {
		t.Errorf("Code family = %q, want %q",
			theme.TextStyleCode.Family, "mymonofamily")
	}
}

// Every preset theme must resolve icons to the bundled font. A preset
// added later that builds its ThemeCfg from scratch instead of from
// baseCfg would otherwise silently lose its icon family.
func TestPresetThemesIconFamily(t *testing.T) {
	presets := map[string]Theme{
		"ThemeDark":        ThemeDark,
		"ThemeLight":       ThemeLight,
		"themeMacOS":       themeMacOS,
		"themeMacOSDark":   themeMacOSDark,
		"themeGnome":       themeGnome,
		"themeGnomeDark":   themeGnomeDark,
		"themeWindows":     themeWindows,
		"themeWindowsDark": themeWindowsDark,
	}
	for name, theme := range presets {
		for i, got := range iconStyleFamilies(theme) {
			if got != IconFontName {
				t.Errorf("%s icon style %d family = %q, want %q",
					name, i, got, IconFontName)
			}
		}
	}
}

// --- Derivation branches ---------------------------------------------
//
// ThemeMaker computes four values up front (theme_maker.go:22-35) and
// then fans them out across many widget styles. The tests below pin
// those derivations rather than re-asserting every consumer.

// borderFocus falls back to ColorSelect only when ColorBorderFocus is
// unset — IsSet distinguishes "not specified" from a deliberately
// transparent-but-set color, which does NOT trigger the fallback.
func TestThemeMakerBorderFocusFallsBackToSelect(t *testing.T) {
	cfg := baseCfg()
	cfg.ColorSelect = RGBA(10, 20, 30, 255)
	cfg.ColorBorderFocus = Color{} // unset

	theme := ThemeMaker(cfg)
	consumers := map[string]Color{
		"ButtonStyle": theme.buttonStyle.Colors.BorderFocus,
		"ToggleStyle": theme.toggleStyle.Colors.BorderFocus,
		"SliderStyle": theme.sliderStyle.Colors.BorderFocus,
		"TabControl":  theme.tabControlStyle.ColorsTab.BorderFocus,
	}
	for name, got := range consumers {
		if !got.eq(cfg.ColorSelect) {
			t.Errorf("%s.ColorBorderFocus = %v, want ColorSelect %v",
				name, got, cfg.ColorSelect)
		}
	}
}

func TestThemeMakerBorderFocusExplicitWins(t *testing.T) {
	cfg := baseCfg()
	cfg.ColorSelect = RGBA(10, 20, 30, 255)
	cfg.ColorBorderFocus = RGBA(200, 100, 50, 255)

	theme := ThemeMaker(cfg)
	if !theme.buttonStyle.Colors.BorderFocus.eq(cfg.ColorBorderFocus) {
		t.Errorf("ButtonStyle.ColorBorderFocus = %v, want %v",
			theme.buttonStyle.Colors.BorderFocus, cfg.ColorBorderFocus)
	}
	if !theme.sliderStyle.Colors.BorderFocus.eq(cfg.ColorBorderFocus) {
		t.Errorf("SliderStyle.ColorBorderFocus = %v, want %v",
			theme.sliderStyle.Colors.BorderFocus, cfg.ColorBorderFocus)
	}
}

// An explicit fully-transparent focus border is a stated choice, not
// "unset": it must survive instead of falling back to the select
// color. eq ignores the set flag, so the sentinel has to be IsSet.
func TestThemeMakerBorderFocusTransparentWins(t *testing.T) {
	cfg := baseCfg()
	cfg.ColorSelect = RGBA(10, 20, 30, 255)
	cfg.ColorBorderFocus = ColorTransparent

	theme := ThemeMaker(cfg)
	if theme.buttonStyle.Colors.BorderFocus != ColorTransparent {
		t.Errorf("ButtonStyle.ColorBorderFocus = %v, want transparent %v",
			theme.buttonStyle.Colors.BorderFocus, ColorTransparent)
	}
}

// ThemeMaker isolates elevation pointers: the built theme shares
// nothing with the caller's cfg or the package-level presets, so a
// later mutation on either side stays local.
func TestThemeMakerShadowsIsolated(t *testing.T) {
	cfg := baseDarkCfg()
	theme := ThemeMaker(cfg)
	if theme.Cfg.ShadowPopover == cfg.ShadowPopover {
		t.Error("theme Cfg shares the ShadowPopover pointer with the caller cfg")
	}
	other := ThemeMaker(cfg)
	if theme.selectStyle.Shadow == other.selectStyle.Shadow {
		t.Error("two themes share one ShadowPopover pointer")
	}
	theme.selectStyle.Shadow.BlurRadius = 999
	if other.selectStyle.Shadow.BlurRadius == 999 {
		t.Error("mutating one theme's shadow moved another theme")
	}
}

// An explicit separator color and thickness win over the fallbacks.
func TestThemeMakerSeparatorExplicitWins(t *testing.T) {
	cfg := baseCfg()
	cfg.ColorSeparator = RGBA(10, 20, 30, 255)
	cfg.SizeSeparator = 4

	theme := ThemeMaker(cfg)
	if !theme.separatorStyle.Colors.Base.eq(cfg.ColorSeparator) {
		t.Errorf("separatorStyle.Color = %v, want %v",
			theme.separatorStyle.Colors.Base, cfg.ColorSeparator)
	}
	if theme.separatorStyle.Size != 4 {
		t.Errorf("separatorStyle.Size = %f, want 4", theme.separatorStyle.Size)
	}
}

// The scrollbar radius keys off cfg.Radius, NOT cfg.RadiusSmall — so a
// square-cornered theme squares the scrollbar even when RadiusSmall is
// rounded. This is the surprising half of the branch.
func TestThemeMakerScrollbarRadiusForcedNoneByRadius(t *testing.T) {
	cfg := baseCfg()
	cfg.Radius = radiusNone
	cfg.RadiusSmall = radiusSmall // deliberately rounded

	theme := ThemeMaker(cfg)
	if theme.ScrollbarStyle.Radius != radiusNone {
		t.Errorf("ScrollbarStyle.Radius = %v, want RadiusNone", theme.ScrollbarStyle.Radius)
	}
	if theme.ScrollbarStyle.radiusThumb != radiusNone {
		t.Errorf("ScrollbarStyle.RadiusThumb = %v, want RadiusNone",
			theme.ScrollbarStyle.radiusThumb)
	}
}

func TestThemeMakerScrollbarRadiusUsesRadiusSmall(t *testing.T) {
	cfg := baseCfg()
	cfg.Radius = radiusMedium
	cfg.RadiusSmall = radiusSmall

	theme := ThemeMaker(cfg)
	if theme.ScrollbarStyle.Radius != radiusSmall {
		t.Errorf("ScrollbarStyle.Radius = %v, want RadiusSmall %v",
			theme.ScrollbarStyle.Radius, radiusSmall)
	}
	if theme.ScrollbarStyle.radiusThumb != radiusSmall {
		t.Errorf("ScrollbarStyle.RadiusThumb = %v, want RadiusSmall %v",
			theme.ScrollbarStyle.radiusThumb, radiusSmall)
	}
}

// Placeholder text reuses the default text RGB with a hard-coded alpha
// of 100, shared by Input, Select and Combobox.
func TestThemeMakerPlaceholderColor(t *testing.T) {
	cfg := baseCfg()
	cfg.TextStyleDef = TextStyle{Color: RGBA(220, 210, 200, 255), Size: sizeTextMedium}

	theme := ThemeMaker(cfg)
	want := RGBA(220, 210, 200, 100)
	placeholders := map[string]TextStyle{
		"InputStyle":    theme.inputStyle.PlaceholderStyle,
		"SelectStyle":   theme.selectStyle.PlaceholderStyle,
		"ComboboxStyle": theme.comboboxStyle.PlaceholderStyle,
	}
	for name, got := range placeholders {
		if !got.Color.eq(want) {
			t.Errorf("%s.PlaceholderStyle.Color = %v, want %v", name, got.Color, want)
		}
		if got.Size != cfg.TextStyleDef.Size {
			t.Errorf("%s.PlaceholderStyle.Size = %v, want %v",
				name, got.Size, cfg.TextStyleDef.Size)
		}
	}
}

// TableStyle.TextStyleHead and BadgeStyle.TextStyle are assigned AFTER
// the struct literal (theme_maker.go:538-540), overwriting whatever the
// literal set. Pin the final values so a future edit to the literal
// cannot silently appear to take effect.
func TestThemeMakerPostLiteralOverwrites(t *testing.T) {
	cfg := baseCfg()
	cfg.TextStyleDef = TextStyle{Color: RGBA(200, 200, 200, 255), Size: sizeTextMedium}
	theme := ThemeMaker(cfg)

	if theme.tableStyle.TextStyleHead != theme.TextStyleTitleSmall {
		t.Errorf("TableStyle.TextStyleHead = %+v, want TitleSmall %+v",
			theme.tableStyle.TextStyleHead, theme.TextStyleTitleSmall)
	}

	wantBadge := theme.TextStyleCaption.Bold()
	wantBadge.Color = White
	if theme.badgeStyle.TextStyle != wantBadge {
		t.Errorf("BadgeStyle.TextStyle = %+v, want Caption.Bold with White color %+v",
			theme.badgeStyle.TextStyle, wantBadge)
	}
	if !theme.badgeStyle.TextStyle.Color.eq(White) {
		t.Errorf("BadgeStyle.TextStyle.Color = %v, want White",
			theme.badgeStyle.TextStyle.Color)
	}
}

// Heading roles take the title roles (visual-refresh §2.2): dialog,
// toast and selected-tab labels are bold while body and value text stays
// Body. The command golden serializer records no typeface, so this is
// the pin — same pattern as TestThemeMakerPostLiteralOverwrites above.
func TestThemeMakerHeadingWeights(t *testing.T) {
	theme := ThemeMaker(baseDarkCfg())

	if theme.dialogStyle.titleTextStyle != theme.TextStyleTitle {
		t.Errorf("dialog title = %+v, want Title %+v",
			theme.dialogStyle.titleTextStyle, theme.TextStyleTitle)
	}
	if theme.toastStyle.TitleStyle != theme.TextStyleTitleSmall {
		t.Errorf("toast title = %+v, want TitleSmall %+v",
			theme.toastStyle.TitleStyle, theme.TextStyleTitleSmall)
	}
	if theme.tabControlStyle.textStyleSelected.Typeface != theme.TextStyleTitleSmall.Typeface {
		t.Errorf("selected tab weight = %+v, want TitleSmall's %+v",
			theme.tabControlStyle.textStyleSelected.Typeface, theme.TextStyleTitleSmall.Typeface)
	}
	if theme.tabControlStyle.textStyleSelected.Color != White {
		t.Errorf("selected tab color = %+v, want the paired foreground",
			theme.tabControlStyle.textStyleSelected.Color)
	}
	// The strip's resting label stays body weight.
	if theme.tabControlStyle.TextStyle.Typeface != 0 {
		t.Errorf("resting tab label = %+v, want regular", theme.tabControlStyle.TextStyle)
	}
}

// The ladder derives from TextStyleDef.Size, and only a stated rung
// overrides its rung of it (visual-refresh §2.1). A zero body size is
// "unset", not "0px text": it falls back to the built-in body so a
// partial theme never derives a negative ladder.
func TestThemeMakerLadderFallback(t *testing.T) {
	empty := ThemeMaker(ThemeCfg{})
	want := textSizeLadder{10, 11, 12, 14, 17, 22}
	got := textSizeLadder{
		tiny:   empty.SizeTextTiny,
		xSmall: empty.SizeTextXSmall,
		small:  empty.SizeTextSmall,
		medium: empty.SizeTextMedium,
		large:  empty.SizeTextLarge,
		xLarge: empty.SizeTextXLarge,
	}
	if got != want {
		t.Errorf("ThemeMaker(ThemeCfg{}).SizeText* = %v, want built-in %v", got, want)
	}

	// A theme that states a body and one rung keeps the rung and
	// derives the rest from the body.
	partial := ThemeMaker(ThemeCfg{
		TextStyleDef:  TextStyle{Size: 16},
		SizeTextSmall: 13,
	})
	if partial.SizeTextMedium != 16 {
		t.Errorf("medium = %v, want 16 derived from TextStyleDef.Size", partial.SizeTextMedium)
	}
	if partial.SizeTextSmall != 13 {
		t.Errorf("small = %v, want the stated 13", partial.SizeTextSmall)
	}
	if partial.SizeTextTiny != 12 {
		t.Errorf("tiny = %v, want 12 derived from TextStyleDef.Size", partial.SizeTextTiny)
	}
}

// The accent ramp (visual-refresh §4.3): hover = L+0.10, pressed =
// L-0.10 in OKLCH, subtle = accent at the polarity-fixed alpha,
// text = white under the 0.45 luminance threshold. These are the
// values the rules actually produce — the spec table was corrected
// to match when the two disagreed (see the spec's decisions).
func TestThemeMakerAccentRamp(t *testing.T) {
	dark := ThemeMaker(baseDarkCfg())
	if dark.ColorAccent != colorAccentDark {
		t.Errorf("accent = %v, want %v", dark.ColorAccent, colorAccentDark)
	}
	if dark.ColorAccentHover != RGB(137, 167, 222) {
		t.Errorf("dark hover = %v, want #89A7DE", dark.ColorAccentHover)
	}
	if dark.ColorAccentPressed != RGB(49, 99, 206) {
		t.Errorf("dark pressed = %v, want #3163CE", dark.ColorAccentPressed)
	}
	if dark.ColorAccentSubtle != RGBA(77, 130, 240, 40) {
		t.Errorf("dark subtle = %v, want accent at alpha 40", dark.ColorAccentSubtle)
	}
	if dark.ColorTextOnAccent != White {
		t.Errorf("dark text on accent = %v, want white", dark.ColorTextOnAccent)
	}

	light := ThemeMaker(themeLightCfg)
	if light.ColorAccent != colorAccentLight {
		t.Errorf("accent = %v, want %v", light.ColorAccent, colorAccentLight)
	}
	if light.ColorAccentHover != RGB(115, 148, 204) {
		t.Errorf("light hover = %v, want #7394CC", light.ColorAccentHover)
	}
	if light.ColorAccentPressed != RGB(13, 79, 190) {
		t.Errorf("light pressed = %v, want #0D4FBE", light.ColorAccentPressed)
	}
	if light.ColorAccentSubtle != RGBA(47, 111, 224, 30) {
		t.Errorf("light subtle = %v, want accent at alpha 30", light.ColorAccentSubtle)
	}
	if light.ColorTextOnAccent != White {
		t.Errorf("light text on accent = %v, want white", light.ColorTextOnAccent)
	}
}

// The accent fallback chain: ColorAccent, else ColorSelect (a theme
// with a native or taste select keeps it as its accent), else the
// legacy select value. ColorSelect resolves to the accent when unset.
func TestThemeMakerAccentFallback(t *testing.T) {
	// A stated select becomes the accent.
	selectTheme := ThemeMaker(ThemeCfg{ColorSelect: Blue})
	if selectTheme.ColorAccent != Blue {
		t.Errorf("accent = %v, want select %v", selectTheme.ColorAccent, Blue)
	}
	if selectTheme.ColorSelect != Blue {
		t.Errorf("select = %v, want %v", selectTheme.ColorSelect, Blue)
	}
	if selectTheme.ColorAccentSubtle != subtleFor(Blue, selectTheme.TextStyleDef.Color, Color{}) {
		t.Errorf("subtle = %v, want derived from select", selectTheme.ColorAccentSubtle)
	}

	// Neither stated: the legacy select keeps old themes' appearance.
	empty := ThemeMaker(ThemeCfg{})
	if empty.ColorAccent != colorSelectDark {
		t.Errorf("accent = %v, want legacy select %v",
			empty.ColorAccent, colorSelectDark)
	}
	if empty.ColorSelect != colorSelectDark {
		t.Errorf("select = %v, want legacy select %v",
			empty.ColorSelect, colorSelectDark)
	}

	// A stated accent resolves ColorSelect to it.
	accentTheme := ThemeMaker(ThemeCfg{ColorAccent: Red})
	if accentTheme.ColorSelect != Red {
		t.Errorf("select = %v, want accent %v", accentTheme.ColorSelect, Red)
	}
	// An explicit slot always wins over the derivation.
	explicit := ThemeMaker(ThemeCfg{
		ColorAccent:      Blue,
		ColorAccentHover: Red,
	})
	if explicit.ColorAccentHover != Red {
		t.Errorf("hover = %v, want the stated %v", explicit.ColorAccentHover, Red)
	}
}

// The derivation's extremes: a black accent clamps hover at 0.10 L
// and keeps pressed at black, and a light accent flips the text on it
// to black under the 0.45 luminance threshold.
func TestThemeMakerAccentExtremes(t *testing.T) {
	black := ThemeMaker(ThemeCfg{ColorAccent: RGB(0, 0, 0)})
	if black.ColorAccentHover != RGB(3, 3, 3) {
		t.Errorf("black hover = %v, want L=0.10", black.ColorAccentHover)
	}
	if black.ColorAccentPressed != RGB(0, 0, 0) {
		t.Errorf("black pressed = %v, want clamped at 0",
			black.ColorAccentPressed)
	}
	if black.ColorTextOnAccent != White {
		t.Errorf("black text on accent = %v, want white",
			black.ColorTextOnAccent)
	}

	yellow := ThemeMaker(ThemeCfg{ColorAccent: RGB(255, 255, 0)})
	if yellow.ColorTextOnAccent != RGB(0, 0, 0) {
		t.Errorf("yellow text on accent = %v, want black",
			yellow.ColorTextOnAccent)
	}
	if yellow.ColorAccentPressed != RGB(216, 218, 135) {
		t.Errorf("yellow pressed = %v, want L-0.10", yellow.ColorAccentPressed)
	}
}

// The widget-level override: a caller-set ColorSelect is an explicit
// choice and wins over the theme's wash, so the subtle slot follows
// it (visual-refresh §4.3). One test covers the pattern the five
// list-like widgets share.
func TestWidgetColorOverrideWinsOverWash(t *testing.T) {
	var lb ListBoxCfg
	lb.ColorSelect = Blue
	applyListBoxDefaults(&lb)
	if lb.ColorSelectSubtle != Blue {
		t.Errorf("listbox subtle = %v, want caller select %v",
			lb.ColorSelectSubtle, Blue)
	}

	var cb ComboboxCfg
	cb.ColorHighlight = Red
	applyComboboxDefaults(&cb)
	if cb.ColorHighlightSubtle != Red {
		t.Errorf("combobox subtle = %v, want caller highlight %v",
			cb.ColorHighlightSubtle, Red)
	}
}

// The unset path: with no caller color the subtle slot takes the
// theme's wash, never the full accent slab the plain slot resolves
// to. This pins the resolution order inside apply*Defaults — the wash
// once became dead code when the plain color was resolved from the
// theme first, making every row paint the full accent again.
func TestWidgetSubtleWashWhenUnset(t *testing.T) {
	var lb ListBoxCfg
	applyListBoxDefaults(&lb)
	if !lb.ColorSelectSubtle.IsSet() || lb.ColorSelectSubtle == lb.ColorSelect {
		t.Errorf("listbox unset subtle = %v, want the theme wash, not the full select %v",
			lb.ColorSelectSubtle, lb.ColorSelect)
	}

	var cb ComboboxCfg
	applyComboboxDefaults(&cb)
	if !cb.ColorHighlightSubtle.IsSet() ||
		cb.ColorHighlightSubtle == cb.ColorHighlight {
		t.Errorf("combobox unset subtle = %v, want the theme wash, not the full highlight %v",
			cb.ColorHighlightSubtle, cb.ColorHighlight)
	}
}

// Semantic colors flow from the Cfg into the styles that paint them,
// and unset falls back to the values those styles historically used
// (visual-refresh §4.4). The subtle companions derive on the accent
// rule.
func TestThemeMakerSemanticColors(t *testing.T) {
	dark := ThemeMaker(baseDarkCfg())
	if dark.toastStyle.ColorSuccess != colorSuccessDark ||
		dark.badgeStyle.ColorSuccess != colorSuccessDark {
		t.Error("dark success should reach toast and badge styles")
	}
	if dark.toastStyle.ColorWarning != colorWarningDark ||
		dark.badgeStyle.ColorWarning != colorWarningDark {
		t.Error("dark warning should reach toast and badge styles")
	}
	if dark.toastStyle.ColorError != colorErrorDark ||
		dark.badgeStyle.ColorError != colorErrorDark {
		t.Error("dark error should reach toast and badge styles")
	}
	if dark.ColorSuccessSubtle != subtleFor(colorSuccessDark, dark.TextStyleDef.Color,
		colorBackgroundDark) {
		t.Errorf("dark success subtle = %v, want derived", dark.ColorSuccessSubtle)
	}

	// Unset: the legacy literal values, so old themes are unchanged.
	legacy := ThemeMaker(ThemeCfg{})
	if legacy.toastStyle.ColorSuccess != RGBA(46, 160, 67, 255) {
		t.Errorf("legacy success = %v, want the historic literal",
			legacy.toastStyle.ColorSuccess)
	}
	if legacy.toastStyle.ColorWarning != RGBA(210, 153, 34, 255) {
		t.Errorf("legacy warning = %v, want the historic literal",
			legacy.toastStyle.ColorWarning)
	}
	if legacy.toastStyle.ColorError != RGBA(218, 54, 51, 255) {
		t.Errorf("legacy error = %v, want the historic literal",
			legacy.toastStyle.ColorError)
	}

	// A stated semantic color wins everywhere it is painted.
	cfg := baseDarkCfg()
	cfg.ColorSuccess = Green
	cfg.ColorError = Red
	theme := ThemeMaker(cfg)
	if theme.toastStyle.ColorSuccess != Green || theme.badgeStyle.ColorSuccess != Green {
		t.Error("stated success should reach both styles")
	}
	if theme.inputStyle.colorSpellError != Red {
		t.Error("stated error should reach the spell-error path")
	}
}
func TestThemeMakerColorTextOnSelect(t *testing.T) {
	tests := []struct {
		name string
		set  bool
		want Color
	}{
		// The default is the accent-paired foreground — the legacy
		// select (colorSelectDark) is darker than the 0.45 luminance
		// threshold, so the paired color is white (visual-refresh
		// §4.3); the old body-color default drew near-black text on
		// a light theme's accent.
		{"unset falls back to the accent-paired foreground", false, White},
		{"explicit wins", true, Green},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseCfg()
			if tt.set {
				cfg.ColorTextOnSelect = Green
			}

			theme := ThemeMaker(cfg)
			if !theme.ColorTextOnSelect.eq(tt.want) {
				t.Errorf("Theme.ColorTextOnSelect = %v, want %v",
					theme.ColorTextOnSelect, tt.want)
			}
			for name, got := range colorTextOnSelectByStyle(theme) {
				if !got.eq(tt.want) {
					t.Errorf("%s.ColorTextOnSelect = %v, want %v",
						name, got, tt.want)
				}
			}
		})
	}
}

// Sizes derived from cfg rather than copied straight through.
func TestThemeMakerDerivedSizes(t *testing.T) {
	cfg := baseCfg()
	cfg.TextStyleDef = TextStyle{Size: 18}
	cfg.SizeSlider = 10

	theme := ThemeMaker(cfg)
	if theme.toggleStyle.Size != 22 {
		t.Errorf("ToggleStyle.Size = %v, want TextStyleDef.Size+4 = 22",
			theme.toggleStyle.Size)
	}
	if theme.sliderStyle.Radius != 5 {
		t.Errorf("SliderStyle.Radius = %v, want SizeSlider/2 = 5",
			theme.sliderStyle.Radius)
	}
}

// SizeSlider is not clamped: a zero slider size yields a zero radius
// rather than a default. Documented, not a bug to "fix" here.
func TestThemeMakerSliderRadiusZeroSizeNotClamped(t *testing.T) {
	cfg := baseCfg()
	cfg.SizeSlider = 0

	theme := ThemeMaker(cfg)
	if theme.sliderStyle.Radius != 0 {
		t.Errorf("SliderStyle.Radius = %v, want 0 for SizeSlider 0",
			theme.sliderStyle.Radius)
	}
}

// A wholly empty ThemeCfg must still build a Theme without panicking.
// Every size lands at zero and every color transparent — only the icon
// family falls back.
func TestThemeMakerZeroCfg(t *testing.T) {
	theme := ThemeMaker(ThemeCfg{})

	if theme.Name != "" {
		t.Errorf("Name = %q, want empty", theme.Name)
	}
	for i, got := range iconStyleFamilies(theme) {
		if got != IconFontName {
			t.Errorf("icon style %d family = %q, want fallback %q",
				i, got, IconFontName)
		}
	}
	if theme.buttonStyle.Radius != 0 || theme.buttonStyle.SizeBorder != 0 {
		t.Errorf("ButtonStyle radius/border = %v/%v, want 0/0",
			theme.buttonStyle.Radius, theme.buttonStyle.SizeBorder)
	}
	// The accent fallback chain bottoms out at the legacy select, so
	// even a zero Cfg gets a working accent ramp and borderFocus.
	if !theme.buttonStyle.Colors.BorderFocus.eq(colorSelectDark) {
		t.Errorf("ButtonStyle.ColorBorderFocus = %v, want legacy select %v",
			theme.buttonStyle.Colors.BorderFocus, colorSelectDark)
	}
	// Placeholder alpha is unconditional, even over a zero text color.
	if theme.inputStyle.PlaceholderStyle.Color.A != 100 {
		t.Errorf("placeholder alpha = %d, want 100",
			theme.inputStyle.PlaceholderStyle.Color.A)
	}
	// Separator falls back to the border color and 1px thickness.
	if theme.separatorStyle.Colors.Base != theme.ColorBorder {
		t.Errorf("separatorStyle.Color = %v, want ColorBorder %v",
			theme.separatorStyle.Colors.Base, theme.ColorBorder)
	}
	if theme.separatorStyle.Size != 1 {
		t.Errorf("separatorStyle.Size = %f, want 1", theme.separatorStyle.Size)
	}
}

// TestThemeMakerRoleLadder locks the role ladder (issues #343, #734):
// every role is populated, sizes descend within each family, the icon
// roles share the roman scale, and the code roles sit +1 above it. A
// zero role or a drifted mono offset would otherwise render silently
// and surface nowhere.
func TestThemeMakerRoleLadder(t *testing.T) {
	roman := [6]TextStyle{
		ThemeDark.TextStyleDisplay.Roman(),
		ThemeDark.TextStyleBodyLarge,
		ThemeDark.TextStyleBody,
		ThemeDark.TextStyleBodySmall,
		ThemeDark.TextStyleCaption,
		ThemeDark.TextStyleCaptionSmall,
	}
	titles := [3]TextStyle{
		ThemeDark.TextStyleDisplay,
		ThemeDark.TextStyleTitle,
		ThemeDark.TextStyleTitleSmall,
	}
	icons := [6]TextStyle{
		ThemeDark.TextStyleIconXLarge,
		ThemeDark.TextStyleIconLarge,
		ThemeDark.TextStyleIconMedium,
		ThemeDark.TextStyleIconSmall,
		ThemeDark.TextStyleIconXSmall,
		ThemeDark.TextStyleIconTiny,
	}
	code := [3]TextStyle{
		ThemeDark.TextStyleCode,
		ThemeDark.TextStyleCodeSmall,
		ThemeDark.TextStyleCodeTiny,
	}
	codeRoman := [3]TextStyle{
		ThemeDark.TextStyleBody,
		ThemeDark.TextStyleCaption,
		ThemeDark.TextStyleCaptionSmall,
	}

	families := []struct {
		name  string
		roles []TextStyle
	}{
		{"roman", roman[:]},
		{"title", titles[:]},
		{"icon", icons[:]},
	}
	for _, f := range families {
		for i, s := range f.roles {
			if s.Size <= 0 {
				t.Errorf("%s role %d Size = %v, want > 0",
					f.name, i, s.Size)
			}
		}
		// Sizes descend within a family: the first role is largest.
		for i := 0; i+1 < len(f.roles); i++ {
			if f.roles[i].Size <= f.roles[i+1].Size {
				t.Errorf("%s role %d Size = %v, want > role %d (%v)",
					f.name, i, f.roles[i].Size,
					i+1, f.roles[i+1].Size)
			}
		}
	}

	// The icon roles share the roman scale, rung by rung.
	for i := range 6 {
		if icons[i].Size != roman[i].Size {
			t.Errorf("icon role %d Size = %v, want roman %v",
				i, icons[i].Size, roman[i].Size)
		}
	}

	// The code roles sit +1 above the roman scale at their rungs.
	for i := range 3 {
		if code[i].Size != codeRoman[i].Size+1 {
			t.Errorf("code role %d Size = %v, want roman+1 %v",
				i, code[i].Size, codeRoman[i].Size+1)
		}
	}
}

// Button variants derive from the accent and error ramps, and their
// geometry matches the base button style so variants of one theme
// align in a row (visual-refresh §6).
func TestThemeMakerButtonVariants(t *testing.T) {
	dark := ThemeMaker(baseDarkCfg())
	if dark.buttonStylePrimary.Colors.Base != dark.ColorAccent {
		t.Errorf("primary fill = %v, want the accent %v",
			dark.buttonStylePrimary.Colors.Base, dark.ColorAccent)
	}
	if dark.buttonStylePrimary.Colors.Hover != dark.ColorAccentHover {
		t.Errorf("primary hover = %v, want %v",
			dark.buttonStylePrimary.Colors.Hover, dark.ColorAccentHover)
	}
	if dark.buttonStylePrimary.Colors.Click != dark.ColorAccentPressed {
		t.Errorf("primary click = %v, want %v",
			dark.buttonStylePrimary.Colors.Click, dark.ColorAccentPressed)
	}
	if dark.buttonStyleDanger.Colors.Base != dark.Cfg.ColorError {
		t.Errorf("danger fill = %v, want the error color %v",
			dark.buttonStyleDanger.Colors.Base, dark.Cfg.ColorError)
	}
	if dark.buttonStyleDanger.Colors.Click == dark.buttonStyleDanger.Colors.Base {
		t.Errorf("danger click = %v, want a pressed variant of the error color",
			dark.buttonStyleDanger.Colors.Click)
	}
	if dark.buttonStyleGhost.Colors.Base != ColorTransparent {
		t.Errorf("ghost fill = %v, want transparent", dark.buttonStyleGhost.Colors.Base)
	}
	if dark.buttonStyleGhost.Colors.Border != ColorTransparent {
		t.Errorf("ghost border = %v, want transparent", dark.buttonStyleGhost.Colors.Border)
	}
	// Geometry comes from the base style, never its own literals.
	for _, v := range []buttonStyle{
		dark.buttonStylePrimary, dark.buttonStyleGhost, dark.buttonStyleDanger,
	} {
		if v.Padding != dark.buttonStyle.Padding || v.Radius != dark.buttonStyle.Radius ||
			v.SizeBorder != dark.buttonStyle.SizeBorder {
			t.Errorf("variant geometry diverged from the base: %+v vs %+v",
				v, dark.buttonStyle)
		}
	}
}

// The error hover/pressed steps use the same absolute-L ramp as the
// accent (theme_maker's accent derivation), clamped to [0,1].
func TestThemeMakerDangerRamp(t *testing.T) {
	dark := ThemeMaker(baseDarkCfg())
	hover := ColorToHSLA(dark.buttonStyleDanger.Colors.Hover)
	base := ColorToHSLA(dark.buttonStyleDanger.Colors.Base)
	pressed := ColorToHSLA(dark.buttonStyleDanger.Colors.Click)
	if hover.L < base.L {
		t.Errorf("danger hover L = %.2f, want above base %.2f", hover.L, base.L)
	}
	if pressed.L > base.L {
		t.Errorf("danger pressed L = %.2f, want below base %.2f", pressed.L, base.L)
	}
	if base.L < 0.05 {
		t.Errorf("danger base L = %.2f, want a non-black error color", base.L)
	}
}

// The progress readout trails the bar in the secondary role
// (visual-refresh §8) — it never sits on the fill again, so it takes
// the de-emphasis outright instead of balancing body text against
// both backgrounds.
func TestThemeMakerProgressReadoutIsSecondary(t *testing.T) {
	dark := ThemeMaker(baseDarkCfg())
	if dark.progressBarStyle.TextStyle != dark.TextStyleSecondary {
		t.Errorf("dark readout style = %v, want TextStyleSecondary %v",
			dark.progressBarStyle.TextStyle, dark.TextStyleSecondary)
	}
	if ThemeLight.progressBarStyle.TextStyle != ThemeLight.TextStyleSecondary {
		t.Errorf("light readout style = %v, want TextStyleSecondary %v",
			ThemeLight.progressBarStyle.TextStyle, ThemeLight.TextStyleSecondary)
	}
}

// TestThemeMakerScrollbarGapsDefault pins the fallback: a ThemeCfg
// that says nothing about the gaps keeps the values ThemeMaker used
// before they were themable.
func TestThemeMakerScrollbarGapsDefault(t *testing.T) {
	th := ThemeMaker(ThemeDark.Cfg)
	if got, want := th.ScrollbarStyle.GapEdge, float32(3); got != want {
		t.Errorf("GapEdge = %v, want %v", got, want)
	}
	if got, want := th.ScrollbarStyle.GapEnd, float32(2); got != want {
		t.Errorf("GapEnd = %v, want %v", got, want)
	}
}

// TestThemeMakerScrollbarGapsZero is why the fields are Opt: an
// explicit zero must reach the style, not fall back to the default.
func TestThemeMakerScrollbarGapsZero(t *testing.T) {
	cfg := ThemeDark.Cfg
	cfg.SizeScrollbarGap = SomeF(0)
	cfg.SizeScrollbarGapEnd = SomeF(0)
	th := ThemeMaker(cfg)
	if got := th.ScrollbarStyle.GapEdge; got != 0 {
		t.Errorf("GapEdge = %v, want 0", got)
	}
	if got := th.ScrollbarStyle.GapEnd; got != 0 {
		t.Errorf("GapEnd = %v, want 0", got)
	}
}
