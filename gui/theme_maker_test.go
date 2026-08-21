package gui

import "testing"

// iconStyleFamilies returns every theme-driven icon style family so the
// tests can assert them as a set.
func iconStyleFamilies(theme Theme) []string {
	return []string{
		theme.Icon1.Family,
		theme.Icon2.Family,
		theme.Icon3.Family,
		theme.Icon4.Family,
		theme.Icon5.Family,
		theme.Icon6.Family,
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
	styles := []TextStyle{
		theme.M1, theme.M2, theme.M3, theme.M4, theme.M5, theme.M6,
	}
	for i, s := range styles {
		if s.Family != "mycustommono" {
			t.Errorf("M%d family = %q, want %q",
				i+1, s.Family, "mycustommono")
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
		"I3": theme.I3, "I6": theme.I6,
		"BI3": theme.BI3, "BI6": theme.BI6,
	}
	for name, s := range text {
		if s.Family != "mytextfamily" {
			t.Errorf("%s family = %q, want %q",
				name, s.Family, "mytextfamily")
		}
	}
	if theme.M3.Family != "mymonofamily" {
		t.Errorf("M3 family = %q, want %q",
			theme.M3.Family, "mymonofamily")
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
// the exact zero Color — Color.Eq compares the IsSet flag too, so a
// deliberately transparent-but-set color does NOT trigger the fallback.
func TestThemeMakerBorderFocusFallsBackToSelect(t *testing.T) {
	cfg := baseCfg()
	cfg.ColorSelect = RGBA(10, 20, 30, 255)
	cfg.ColorBorderFocus = Color{} // unset

	theme := ThemeMaker(cfg)
	consumers := map[string]Color{
		"ButtonStyle": theme.ButtonStyle.ColorBorderFocus,
		"ToggleStyle": theme.toggleStyle.ColorBorderFocus,
		"SliderStyle": theme.sliderStyle.ColorBorderFocus,
		"TabControl":  theme.tabControlStyle.colorTabBorderFocus,
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
	if !theme.ButtonStyle.ColorBorderFocus.eq(cfg.ColorBorderFocus) {
		t.Errorf("ButtonStyle.ColorBorderFocus = %v, want %v",
			theme.ButtonStyle.ColorBorderFocus, cfg.ColorBorderFocus)
	}
	if !theme.sliderStyle.ColorBorderFocus.eq(cfg.ColorBorderFocus) {
		t.Errorf("SliderStyle.ColorBorderFocus = %v, want %v",
			theme.sliderStyle.ColorBorderFocus, cfg.ColorBorderFocus)
	}
}

// An explicit separator color and thickness win over the fallbacks.
func TestThemeMakerSeparatorExplicitWins(t *testing.T) {
	cfg := baseCfg()
	cfg.ColorSeparator = RGBA(10, 20, 30, 255)
	cfg.SizeSeparator = 4

	theme := ThemeMaker(cfg)
	if !theme.separatorStyle.Color.eq(cfg.ColorSeparator) {
		t.Errorf("separatorStyle.Color = %v, want %v",
			theme.separatorStyle.Color, cfg.ColorSeparator)
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
		"InputStyle":    theme.InputStyle.PlaceholderStyle,
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

	if theme.tableStyle.TextStyleHead != theme.B3 {
		t.Errorf("TableStyle.TextStyleHead = %+v, want B3 %+v",
			theme.tableStyle.TextStyleHead, theme.B3)
	}

	wantBadge := theme.B5
	wantBadge.Color = White
	if theme.badgeStyle.TextStyle != wantBadge {
		t.Errorf("BadgeStyle.TextStyle = %+v, want B5 with White color %+v",
			theme.badgeStyle.TextStyle, wantBadge)
	}
	if !theme.badgeStyle.TextStyle.Color.eq(White) {
		t.Errorf("BadgeStyle.TextStyle.Color = %v, want White",
			theme.badgeStyle.TextStyle.Color)
	}
}

// Heading roles take B rungs (visual-refresh §2.2): dialog, toast and
// selected-tab labels are bold while body and value text stays N. The
// command golden serializer records no typeface, so this is the pin —
// same pattern as TestThemeMakerPostLiteralOverwrites above.
func TestThemeMakerHeadingWeights(t *testing.T) {
	theme := ThemeMaker(baseDarkCfg())

	if theme.dialogStyle.titleTextStyle != theme.B2 {
		t.Errorf("dialog title = %+v, want B2 %+v",
			theme.dialogStyle.titleTextStyle, theme.B2)
	}
	if theme.toastStyle.TitleStyle != theme.B3 {
		t.Errorf("toast title = %+v, want B3 %+v",
			theme.toastStyle.TitleStyle, theme.B3)
	}
	if theme.tabControlStyle.textStyleSelected.Typeface != theme.B3.Typeface {
		t.Errorf("selected tab weight = %+v, want B3's %+v",
			theme.tabControlStyle.textStyleSelected.Typeface, theme.B3.Typeface)
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

// The accent ramp (visual-refresh §4.3): hover = L+0.12, pressed =
// L-0.12 in sRGB HSL, subtle = accent at the polarity-fixed alpha,
// text = white under the 0.45 luminance threshold. These are the
// values the rules actually produce — the spec table was corrected
// to match when the two disagreed (see the spec's decisions).
func TestThemeMakerAccentRamp(t *testing.T) {
	dark := ThemeMaker(baseDarkCfg())
	if dark.ColorAccent != colorAccentDark {
		t.Errorf("accent = %v, want %v", dark.ColorAccent, colorAccentDark)
	}
	if dark.ColorAccentHover != RGB(133, 170, 245) {
		t.Errorf("dark hover = %v, want #85AAF5", dark.ColorAccentHover)
	}
	if dark.ColorAccentPressed != RGB(21, 90, 235) {
		t.Errorf("dark pressed = %v, want #155AEB", dark.ColorAccentPressed)
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
	if light.ColorAccentHover != RGB(100, 148, 232) {
		t.Errorf("light hover = %v, want #6494E8", light.ColorAccentHover)
	}
	if light.ColorAccentPressed != RGB(27, 83, 183) {
		t.Errorf("light pressed = %v, want #1B53B7", light.ColorAccentPressed)
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
	if selectTheme.ColorAccentSubtle != subtleFor(Blue, Color{}) {
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

// The derivation's extremes: a black accent clamps hover at 0.12 L
// and keeps pressed at black, and a light accent flips the text on it
// to black under the 0.45 luminance threshold.
func TestThemeMakerAccentExtremes(t *testing.T) {
	black := ThemeMaker(ThemeCfg{ColorAccent: RGB(0, 0, 0)})
	if black.ColorAccentHover != RGB(31, 31, 31) {
		t.Errorf("black hover = %v, want L=0.12", black.ColorAccentHover)
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
	if yellow.ColorAccentPressed != RGB(194, 194, 0) {
		t.Errorf("yellow pressed = %v, want L-0.12", yellow.ColorAccentPressed)
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
	if dark.ColorSuccessSubtle != subtleFor(colorSuccessDark, colorBackgroundDark) {
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
	if theme.InputStyle.colorSpellError != Red {
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
	if theme.ButtonStyle.Radius != 0 || theme.ButtonStyle.SizeBorder != 0 {
		t.Errorf("ButtonStyle radius/border = %v/%v, want 0/0",
			theme.ButtonStyle.Radius, theme.ButtonStyle.SizeBorder)
	}
	// The accent fallback chain bottoms out at the legacy select, so
	// even a zero Cfg gets a working accent ramp and borderFocus.
	if !theme.ButtonStyle.ColorBorderFocus.eq(colorSelectDark) {
		t.Errorf("ButtonStyle.ColorBorderFocus = %v, want legacy select %v",
			theme.ButtonStyle.ColorBorderFocus, colorSelectDark)
	}
	// Placeholder alpha is unconditional, even over a zero text color.
	if theme.InputStyle.PlaceholderStyle.Color.A != 100 {
		t.Errorf("placeholder alpha = %d, want 100",
			theme.InputStyle.PlaceholderStyle.Color.A)
	}
	// Separator falls back to the border color and 1px thickness.
	if theme.separatorStyle.Color != theme.ColorBorder {
		t.Errorf("separatorStyle.Color = %v, want ColorBorder %v",
			theme.separatorStyle.Color, theme.ColorBorder)
	}
	if theme.separatorStyle.Size != 1 {
		t.Errorf("separatorStyle.Size = %f, want 1", theme.separatorStyle.Size)
	}
}

// TestThemeMakerLadderGrid locks the closed 6x6 ladder (issue #343):
// every rung of every face is populated, rungs ascend N1..N6 within a
// face, the roman faces share one size scale, and the mono face sits
// +1 above it. A zero rung or a drifted mono offset would otherwise
// render silently and surface nowhere.
func TestThemeMakerLadderGrid(t *testing.T) {
	type face struct {
		name string
		runs [6]TextStyle
	}
	faces := []face{
		{"N", [6]TextStyle{ThemeDark.N1, ThemeDark.N2, ThemeDark.N3, ThemeDark.N4, ThemeDark.N5, ThemeDark.N6}},
		{"B", [6]TextStyle{ThemeDark.B1, ThemeDark.B2, ThemeDark.B3, ThemeDark.B4, ThemeDark.B5, ThemeDark.B6}},
		{"I", [6]TextStyle{ThemeDark.I1, ThemeDark.I2, ThemeDark.I3, ThemeDark.I4, ThemeDark.I5, ThemeDark.I6}},
		{"BI", [6]TextStyle{ThemeDark.BI1, ThemeDark.BI2, ThemeDark.BI3, ThemeDark.BI4, ThemeDark.BI5, ThemeDark.BI6}},
		{"Icon", [6]TextStyle{ThemeDark.Icon1, ThemeDark.Icon2, ThemeDark.Icon3, ThemeDark.Icon4, ThemeDark.Icon5, ThemeDark.Icon6}},
		{"M", [6]TextStyle{ThemeDark.M1, ThemeDark.M2, ThemeDark.M3, ThemeDark.M4, ThemeDark.M5, ThemeDark.M6}},
	}

	// The canonical roman scale the other faces must match, rung by rung.
	nSizes := [6]float32{
		ThemeDark.N1.Size, ThemeDark.N2.Size, ThemeDark.N3.Size,
		ThemeDark.N4.Size, ThemeDark.N5.Size, ThemeDark.N6.Size,
	}

	for _, f := range faces {
		for i, s := range f.runs {
			if s.Size <= 0 {
				t.Errorf("%s%d.Size = %v, want > 0", f.name, i+1, s.Size)
			}
		}
		// Rungs ascend within a face: N1 (xlarge) is the largest.
		for i := 0; i+1 < len(f.runs); i++ {
			if f.runs[i].Size <= f.runs[i+1].Size {
				t.Errorf("%s%d.Size = %v, want > %s%d (%v)",
					f.name, i+1, f.runs[i].Size,
					f.name, i+2, f.runs[i+1].Size)
			}
		}
	}

	// The roman faces share the N scale at every rung.
	for _, f := range faces {
		if f.name == "M" {
			continue
		}
		for rung := range 6 {
			if f.runs[rung].Size != nSizes[rung] {
				t.Errorf("%s%d.Size = %v, want N%d %v",
					f.name, rung+1, f.runs[rung].Size, rung+1, nSizes[rung])
			}
		}
	}

	// The mono face sits +1 above the roman scale at every rung.
	mSizes := [6]float32{
		ThemeDark.M1.Size, ThemeDark.M2.Size, ThemeDark.M3.Size,
		ThemeDark.M4.Size, ThemeDark.M5.Size, ThemeDark.M6.Size,
	}
	for rung := range 6 {
		if mSizes[rung] != nSizes[rung]+1 {
			t.Errorf("M%d.Size = %v, want N%d+1 %v",
				rung+1, mSizes[rung], rung+1, nSizes[rung]+1)
		}
	}
}

// Button variants derive from the accent and error ramps, and their
// geometry matches the base button style so variants of one theme
// align in a row (visual-refresh §6).
func TestThemeMakerButtonVariants(t *testing.T) {
	dark := ThemeMaker(baseDarkCfg())
	if dark.ButtonStylePrimary.Color != dark.ColorAccent {
		t.Errorf("primary fill = %v, want the accent %v",
			dark.ButtonStylePrimary.Color, dark.ColorAccent)
	}
	if dark.ButtonStylePrimary.ColorHover != dark.ColorAccentHover {
		t.Errorf("primary hover = %v, want %v",
			dark.ButtonStylePrimary.ColorHover, dark.ColorAccentHover)
	}
	if dark.ButtonStylePrimary.colorClick != dark.ColorAccentPressed {
		t.Errorf("primary click = %v, want %v",
			dark.ButtonStylePrimary.colorClick, dark.ColorAccentPressed)
	}
	if dark.ButtonStyleDanger.Color != dark.Cfg.ColorError {
		t.Errorf("danger fill = %v, want the error color %v",
			dark.ButtonStyleDanger.Color, dark.Cfg.ColorError)
	}
	if dark.ButtonStyleDanger.colorClick == dark.ButtonStyleDanger.Color {
		t.Errorf("danger click = %v, want a pressed variant of the error color",
			dark.ButtonStyleDanger.colorClick)
	}
	if dark.ButtonStyleGhost.Color != ColorTransparent {
		t.Errorf("ghost fill = %v, want transparent", dark.ButtonStyleGhost.Color)
	}
	if dark.ButtonStyleGhost.ColorBorder != ColorTransparent {
		t.Errorf("ghost border = %v, want transparent", dark.ButtonStyleGhost.ColorBorder)
	}
	// Geometry comes from the base style, never its own literals.
	for _, v := range []buttonStyle{
		dark.ButtonStylePrimary, dark.ButtonStyleGhost, dark.ButtonStyleDanger,
	} {
		if v.Padding != dark.ButtonStyle.Padding || v.Radius != dark.ButtonStyle.Radius ||
			v.SizeBorder != dark.ButtonStyle.SizeBorder {
			t.Errorf("variant geometry diverged from the base: %+v vs %+v",
				v, dark.ButtonStyle)
		}
	}
}

// The error hover/pressed steps use the same absolute-L ramp as the
// accent (theme_maker's accent derivation), clamped to [0,1].
func TestThemeMakerDangerRamp(t *testing.T) {
	dark := ThemeMaker(baseDarkCfg())
	hover := ColorToHSLA(dark.ButtonStyleDanger.ColorHover)
	base := ColorToHSLA(dark.ButtonStyleDanger.Color)
	pressed := ColorToHSLA(dark.ButtonStyleDanger.colorClick)
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
