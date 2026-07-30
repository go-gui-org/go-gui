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
		theme.TreeStyle.TextStyleIcon.Family,
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
		"ThemeDark":  ThemeDark,
		"ThemeLight": ThemeLight,
		"ThemeBlue":  ThemeBlue,
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
		"ToggleStyle": theme.ToggleStyle.ColorBorderFocus,
		"SliderStyle": theme.SliderStyle.ColorBorderFocus,
		"TabControl":  theme.TabControlStyle.ColorTabBorderFocus,
	}
	for name, got := range consumers {
		if !got.Eq(cfg.ColorSelect) {
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
	if !theme.ButtonStyle.ColorBorderFocus.Eq(cfg.ColorBorderFocus) {
		t.Errorf("ButtonStyle.ColorBorderFocus = %v, want %v",
			theme.ButtonStyle.ColorBorderFocus, cfg.ColorBorderFocus)
	}
	if !theme.SliderStyle.ColorBorderFocus.Eq(cfg.ColorBorderFocus) {
		t.Errorf("SliderStyle.ColorBorderFocus = %v, want %v",
			theme.SliderStyle.ColorBorderFocus, cfg.ColorBorderFocus)
	}
}

// The scrollbar radius keys off cfg.Radius, NOT cfg.RadiusSmall — so a
// square-cornered theme squares the scrollbar even when RadiusSmall is
// rounded. This is the surprising half of the branch.
func TestThemeMakerScrollbarRadiusForcedNoneByRadius(t *testing.T) {
	cfg := baseCfg()
	cfg.Radius = RadiusNone
	cfg.RadiusSmall = RadiusSmall // deliberately rounded

	theme := ThemeMaker(cfg)
	if theme.ScrollbarStyle.Radius != RadiusNone {
		t.Errorf("ScrollbarStyle.Radius = %v, want RadiusNone", theme.ScrollbarStyle.Radius)
	}
	if theme.ScrollbarStyle.RadiusThumb != RadiusNone {
		t.Errorf("ScrollbarStyle.RadiusThumb = %v, want RadiusNone",
			theme.ScrollbarStyle.RadiusThumb)
	}
}

func TestThemeMakerScrollbarRadiusUsesRadiusSmall(t *testing.T) {
	cfg := baseCfg()
	cfg.Radius = RadiusMedium
	cfg.RadiusSmall = RadiusSmall

	theme := ThemeMaker(cfg)
	if theme.ScrollbarStyle.Radius != RadiusSmall {
		t.Errorf("ScrollbarStyle.Radius = %v, want RadiusSmall %v",
			theme.ScrollbarStyle.Radius, RadiusSmall)
	}
	if theme.ScrollbarStyle.RadiusThumb != RadiusSmall {
		t.Errorf("ScrollbarStyle.RadiusThumb = %v, want RadiusSmall %v",
			theme.ScrollbarStyle.RadiusThumb, RadiusSmall)
	}
}

// Placeholder text reuses the default text RGB with a hard-coded alpha
// of 100, shared by Input, Select and Combobox.
func TestThemeMakerPlaceholderColor(t *testing.T) {
	cfg := baseCfg()
	cfg.TextStyleDef = TextStyle{Color: RGBA(220, 210, 200, 255), Size: SizeTextMedium}

	theme := ThemeMaker(cfg)
	want := RGBA(220, 210, 200, 100)
	placeholders := map[string]TextStyle{
		"InputStyle":    theme.InputStyle.PlaceholderStyle,
		"SelectStyle":   theme.SelectStyle.PlaceholderStyle,
		"ComboboxStyle": theme.ComboboxStyle.PlaceholderStyle,
	}
	for name, got := range placeholders {
		if !got.Color.Eq(want) {
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
	cfg.TextStyleDef = TextStyle{Color: RGBA(200, 200, 200, 255), Size: SizeTextMedium}
	theme := ThemeMaker(cfg)

	if theme.TableStyle.TextStyleHead != theme.B3 {
		t.Errorf("TableStyle.TextStyleHead = %+v, want B3 %+v",
			theme.TableStyle.TextStyleHead, theme.B3)
	}

	wantBadge := theme.B5
	wantBadge.Color = White
	if theme.BadgeStyle.TextStyle != wantBadge {
		t.Errorf("BadgeStyle.TextStyle = %+v, want B5 with White color %+v",
			theme.BadgeStyle.TextStyle, wantBadge)
	}
	if !theme.BadgeStyle.TextStyle.Color.Eq(White) {
		t.Errorf("BadgeStyle.TextStyle.Color = %v, want White",
			theme.BadgeStyle.TextStyle.Color)
	}
}

// Sizes derived from cfg rather than copied straight through.
func TestThemeMakerDerivedSizes(t *testing.T) {
	cfg := baseCfg()
	cfg.TextStyleDef = TextStyle{Size: 18}
	cfg.SizeSlider = 10

	theme := ThemeMaker(cfg)
	if theme.ToggleStyle.Size != 22 {
		t.Errorf("ToggleStyle.Size = %v, want TextStyleDef.Size+4 = 22",
			theme.ToggleStyle.Size)
	}
	if theme.SliderStyle.Radius != 5 {
		t.Errorf("SliderStyle.Radius = %v, want SizeSlider/2 = 5",
			theme.SliderStyle.Radius)
	}
}

// SizeSlider is not clamped: a zero slider size yields a zero radius
// rather than a default. Documented, not a bug to "fix" here.
func TestThemeMakerSliderRadiusZeroSizeNotClamped(t *testing.T) {
	cfg := baseCfg()
	cfg.SizeSlider = 0

	theme := ThemeMaker(cfg)
	if theme.SliderStyle.Radius != 0 {
		t.Errorf("SliderStyle.Radius = %v, want 0 for SizeSlider 0",
			theme.SliderStyle.Radius)
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
	// borderFocus fallback resolves to the (also zero) ColorSelect.
	if !theme.ButtonStyle.ColorBorderFocus.Eq(Color{}) {
		t.Errorf("ButtonStyle.ColorBorderFocus = %v, want zero Color",
			theme.ButtonStyle.ColorBorderFocus)
	}
	// Placeholder alpha is unconditional, even over a zero text color.
	if theme.InputStyle.PlaceholderStyle.Color.A != 100 {
		t.Errorf("placeholder alpha = %d, want 100",
			theme.InputStyle.PlaceholderStyle.Color.A)
	}
}
