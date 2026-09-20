package gui

import (
	"reflect"
	"testing"
	"unsafe"
)

// WithColors restates ThemeMaker's cfg-to-style mapping by hand, so the
// two drift silently: a slot added to ThemeMaker and not to WithColors
// simply keeps the old theme's color, and nothing fails. Fourteen slots
// had drifted this way on a same-polarity recolor and twenty-eight
// across a polarity flip, including a white selected-tab label left on
// a light accent fill.
//
// This is the gate. Recolor a preset with an override set that states
// every field, and ThemeMaker fed the same colors must produce the same
// styles: with nothing left unstated, WithColors' track/fork rule has
// no room to disagree, so any difference is a mapping gap.
//
// What is deliberately excluded: Cfg and restoreCfg (the configuration,
// not the styles), id (fresh per derivation by contract) and Name.
func TestWithColorsMatchesThemeMaker(t *testing.T) {
	// Two polarities and two accent polarities, because the text roles
	// and the fill pairings are the slots that only move when the
	// theme flips.
	cases := []struct {
		name string
		base Theme
		over ColorOverrides
	}{
		{"dark to dark", ThemeDark, darkOverrides()},
		{"dark to light", ThemeDark, lightOverrides()},
		{"light to dark", ThemeLight, darkOverrides()},
		{"light to light", ThemeLight, lightOverrides()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.base.WithColors(tc.over)

			cfg := tc.base.Cfg
			applyOverridesToCfg(&cfg, tc.over)
			want := ThemeMaker(cfg)

			diffThemeStyles(t, got, want)
			diffCfgColors(t, got.Cfg, cfg)
		})
	}
}

// TestWithColorsLoneSelectMovesAccent pins the coupling in
// theme_colors.go: select and accent are one decision while they
// agree, so a lone select override moves both. Stating both sets
// them apart. The full-set cases above never exercise this because
// they state both together.
func TestWithColorsLoneSelectMovesAccent(t *testing.T) {
	sel := RGB(10, 200, 120)
	got := ThemeDark.WithColors(ColorOverrides{ColorSelect: sel})
	if got.ColorSelect != sel {
		t.Errorf("ColorSelect = %v, want %v", got.ColorSelect, sel)
	}
	if got.ColorAccent != sel {
		t.Errorf("ColorAccent = %v, want lone select %v",
			got.ColorAccent, sel)
	}
	both := ThemeDark.WithColors(ColorOverrides{
		ColorSelect: sel,
		ColorAccent: RGB(90, 120, 220),
	})
	if both.ColorAccent == both.ColorSelect {
		t.Errorf("stating both left accent %v equal to select %v, want apart",
			both.ColorAccent, both.ColorSelect)
	}
}

// darkOverrides is a complete recolor onto a dark ground.
func darkOverrides() ColorOverrides {
	return ColorOverrides{
		ColorBackground:    RGB(18, 18, 24),
		ColorPanel:         RGB(28, 28, 36),
		ColorInterior:      RGB(38, 38, 48),
		ColorHover:         RGB(52, 52, 64),
		ColorFocus:         RGB(60, 60, 74),
		ColorActive:        RGB(70, 70, 86),
		ColorBorder:        RGB(80, 80, 96),
		ColorBorderFocus:   RGB(110, 140, 230),
		ColorSeparator:     RGB(64, 64, 78),
		ColorSelect:        RGB(90, 120, 220),
		ColorTextOnSelect:  RGB(240, 244, 255),
		ColorAccent:        RGB(90, 120, 220),
		ColorAccentHover:   RGB(110, 140, 230),
		ColorAccentPressed: RGB(70, 100, 200),
		ColorAccentSubtle:  RGBA(90, 120, 220, 40),
		ColorTextOnAccent:  White,
		ColorSuccess:       RGB(46, 160, 67),
		ColorWarning:       RGB(210, 153, 34),
		ColorError:         RGB(218, 54, 51),
		ColorSuccessSubtle: RGBA(46, 160, 67, 40),
		ColorWarningSubtle: RGBA(210, 153, 34, 40),
		ColorErrorSubtle:   RGBA(218, 54, 51, 40),
	}
}

// lightOverrides is a complete recolor onto a light ground, with a
// light accent: the case that exposed the polarity-blind slots.
func lightOverrides() ColorOverrides {
	return ColorOverrides{
		ColorBackground:    RGB(250, 250, 252),
		ColorPanel:         RGB(240, 240, 244),
		ColorInterior:      RGB(232, 232, 238),
		ColorHover:         RGB(220, 220, 228),
		ColorFocus:         RGB(210, 210, 220),
		ColorActive:        RGB(200, 200, 212),
		ColorBorder:        RGB(190, 190, 202),
		ColorBorderFocus:   RGB(120, 140, 200),
		ColorSeparator:     RGB(198, 198, 210),
		ColorSelect:        RGB(240, 210, 90),
		ColorTextOnSelect:  RGB(40, 32, 8),
		ColorAccent:        RGB(240, 210, 90),
		ColorAccentHover:   RGB(244, 218, 120),
		ColorAccentPressed: RGB(210, 180, 60),
		ColorAccentSubtle:  RGBA(240, 210, 90, 30),
		ColorTextOnAccent:  RGB(0, 0, 0),
		ColorSuccess:       RGB(120, 220, 140),
		ColorWarning:       RGB(250, 220, 120),
		ColorError:         RGB(250, 140, 140),
		ColorSuccessSubtle: RGBA(120, 220, 140, 30),
		ColorWarningSubtle: RGBA(250, 220, 120, 30),
		ColorErrorSubtle:   RGBA(250, 140, 140, 30),
	}
}

// applyOverridesToCfg is the ThemeMaker-side spelling of the same
// recolor: WithColors syncs exactly these fields into Cfg, so feeding
// the maker the synced cfg is what "the same colors" means.
func applyOverridesToCfg(cfg *ThemeCfg, o ColorOverrides) {
	set := func(dst *Color, src Color) {
		if src.IsSet() {
			*dst = src
		}
	}
	set(&cfg.ColorBackground, o.ColorBackground)
	set(&cfg.ColorPanel, o.ColorPanel)
	set(&cfg.ColorInterior, o.ColorInterior)
	set(&cfg.ColorHover, o.ColorHover)
	set(&cfg.ColorFocus, o.ColorFocus)
	set(&cfg.ColorActive, o.ColorActive)
	set(&cfg.ColorBorder, o.ColorBorder)
	set(&cfg.ColorBorderFocus, o.ColorBorderFocus)
	set(&cfg.ColorSeparator, o.ColorSeparator)
	set(&cfg.ColorSelect, o.ColorSelect)
	set(&cfg.ColorTextOnSelect, o.ColorTextOnSelect)
	set(&cfg.ColorAccent, o.ColorAccent)
	set(&cfg.ColorAccentHover, o.ColorAccentHover)
	set(&cfg.ColorAccentPressed, o.ColorAccentPressed)
	set(&cfg.ColorAccentSubtle, o.ColorAccentSubtle)
	set(&cfg.ColorTextOnAccent, o.ColorTextOnAccent)
	set(&cfg.ColorSuccess, o.ColorSuccess)
	set(&cfg.ColorWarning, o.ColorWarning)
	set(&cfg.ColorError, o.ColorError)
	set(&cfg.ColorSuccessSubtle, o.ColorSuccessSubtle)
	set(&cfg.ColorWarningSubtle, o.ColorWarningSubtle)
	set(&cfg.ColorErrorSubtle, o.ColorErrorSubtle)
}

// diffCfgColors compares the synced color fields of Cfg. WithColors
// syncs every set override into Cfg so a later rebuild keeps it; the
// style diff above cannot see a dropped sync because want is built
// from the synced cfg. One t.Errorf per field, as with the styles.
func diffCfgColors(t *testing.T, got, want ThemeCfg) {
	t.Helper()
	rg := reflect.ValueOf(&got).Elem()
	rw := reflect.ValueOf(&want).Elem()
	typ := rg.Type()
	colorType := reflect.TypeFor[Color]()
	for i := range typ.NumField() {
		if typ.Field(i).Type != colorType {
			continue
		}
		name := typ.Field(i).Name
		a := readable(rg.Field(i))
		b := readable(rw.Field(i))
		if reflect.DeepEqual(a.Interface(), b.Interface()) {
			continue
		}
		t.Errorf("Cfg.%s: WithColors=%v want=%v",
			name, a.Interface(), b.Interface())
	}
}

// themeDiffSkip names the fields that are allowed to differ.
var themeDiffSkip = map[string]bool{
	"Cfg": true, "restoreCfg": true, "id": true, "Name": true,
}

// diffThemeStyles reports every style field where got and want differ,
// one t.Errorf per field so a drift report names all of them at once.
// Reflection reaches unexported fields through reflect.NewAt, which is
// what lets one gate cover forty style structs instead of forty hand
// written comparisons that would themselves drift.
func diffThemeStyles(t *testing.T, got, want Theme) {
	t.Helper()
	rg := reflect.ValueOf(&got).Elem()
	rw := reflect.ValueOf(&want).Elem()
	typ := rg.Type()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if themeDiffSkip[name] {
			continue
		}
		a := readable(rg.Field(i))
		b := readable(rw.Field(i))
		if a.Kind() == reflect.Struct {
			diffStruct(t, name, a, b)
			continue
		}
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			t.Errorf("%s: WithColors=%v ThemeMaker=%v",
				name, a.Interface(), b.Interface())
		}
	}
}

// diffStruct compares one style struct field by field, so a failure
// names the slot rather than dumping two forty-field structs.
func diffStruct(t *testing.T, prefix string, a, b reflect.Value) {
	t.Helper()
	typ := a.Type()
	for i := range typ.NumField() {
		fa := readable(a.Field(i))
		fb := readable(b.Field(i))
		if reflect.DeepEqual(fa.Interface(), fb.Interface()) {
			continue
		}
		t.Errorf("%s.%s: WithColors=%v ThemeMaker=%v",
			prefix, typ.Field(i).Name, fa.Interface(), fb.Interface())
	}
}

// readable re-derives an addressable value so an unexported field can
// be read. Safe here: the values are local copies the test owns.
func readable(v reflect.Value) reflect.Value {
	if v.CanInterface() {
		return v
	}
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}
