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
		"ThemeDark":  ThemeDark,
		"ThemeLight": ThemeLight,
		"ThemeBlue":  themeBlue,
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

// ColorTextOnSelect resolves to the body text color when unset, so
// a theme that never states it keeps its current appearance — and
// every preset stays byte-identical (issue #373). An explicit value
// wins over the body fallback.
func TestThemeMakerColorTextOnSelect(t *testing.T) {
	body := RGBA(12, 34, 56, 255)
	tests := []struct {
		name string
		set  bool
		want Color
	}{
		{"unset falls back to body", false, body},
		{"explicit wins", true, White},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseCfg()
			cfg.TextStyleDef.Color = body
			if tt.set {
				cfg.ColorTextOnSelect = White
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
	// borderFocus fallback resolves to the (also zero) ColorSelect.
	if !theme.ButtonStyle.ColorBorderFocus.eq(Color{}) {
		t.Errorf("ButtonStyle.ColorBorderFocus = %v, want zero Color",
			theme.ButtonStyle.ColorBorderFocus)
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
