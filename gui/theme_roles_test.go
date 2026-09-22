package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// Tests for the semantic text roles (issue #734). With the rung grid
// removed, these pin each role to the size ladder and face it derives
// from, so a derivation edit that moves pixels fails here first.

func themeRolesTestTheme() Theme {
	return ThemeMaker(themeDarkCfg)
}

// TestTextRolesDeriveFromLadder pins every role's size and face.
func TestTextRolesDeriveFromLadder(t *testing.T) {
	theme := themeRolesTestTheme()

	roles := []struct {
		name     string
		role     TextStyle
		size     float32
		typeface glyph.Typeface
	}{
		{"Display", theme.TextStyleDisplay,
			theme.SizeTextXLarge, glyph.TypefaceBold},
		{"Title", theme.TextStyleTitle,
			theme.SizeTextLarge, glyph.TypefaceBold},
		{"TitleSmall", theme.TextStyleTitleSmall,
			theme.SizeTextMedium, glyph.TypefaceBold},
		{"BodyLarge", theme.TextStyleBodyLarge,
			theme.SizeTextLarge, glyph.TypefaceRegular},
		{"Body", theme.TextStyleBody,
			theme.SizeTextMedium, glyph.TypefaceRegular},
		{"BodySmall", theme.TextStyleBodySmall,
			theme.SizeTextSmall, glyph.TypefaceRegular},
		{"Caption", theme.TextStyleCaption,
			theme.SizeTextXSmall, glyph.TypefaceRegular},
		{"CaptionSmall", theme.TextStyleCaptionSmall,
			theme.SizeTextTiny, glyph.TypefaceRegular},
		{"Code", theme.TextStyleCode,
			theme.SizeTextMedium + 1, glyph.TypefaceRegular},
		{"CodeSmall", theme.TextStyleCodeSmall,
			theme.SizeTextXSmall + 1, glyph.TypefaceRegular},
		{"CodeTiny", theme.TextStyleCodeTiny,
			theme.SizeTextTiny + 1, glyph.TypefaceRegular},
		{"IconXLarge", theme.TextStyleIconXLarge,
			theme.SizeTextXLarge, glyph.TypefaceRegular},
		{"IconLarge", theme.TextStyleIconLarge,
			theme.SizeTextLarge, glyph.TypefaceRegular},
		{"IconMedium", theme.TextStyleIconMedium,
			theme.SizeTextMedium, glyph.TypefaceRegular},
		{"IconSmall", theme.TextStyleIconSmall,
			theme.SizeTextSmall, glyph.TypefaceRegular},
		{"IconXSmall", theme.TextStyleIconXSmall,
			theme.SizeTextXSmall, glyph.TypefaceRegular},
		{"IconTiny", theme.TextStyleIconTiny,
			theme.SizeTextTiny, glyph.TypefaceRegular},
	}

	for _, r := range roles {
		if r.role.Size != r.size {
			t.Errorf("%s Size = %v, want ladder %v",
				r.name, r.role.Size, r.size)
		}
		if r.role.Typeface != r.typeface {
			t.Errorf("%s Typeface = %v, want %v",
				r.name, r.role.Typeface, r.typeface)
		}
	}
}

// TestTextRolesBodyOffLadder pins the preserved rung-3 rule: Body
// comes off the size ladder, not off the body style directly, so a
// Cfg that states both and disagrees still sets Body to the ladder.
func TestTextRolesBodyOffLadder(t *testing.T) {
	cfg := themeDarkCfg
	cfg.SizeTextMedium = themeDarkCfg.SizeTextMedium + 6
	theme := ThemeMaker(cfg)

	if theme.TextStyleBody.Size != cfg.SizeTextMedium {
		t.Errorf("Body Size = %v, want ladder %v",
			theme.TextStyleBody.Size, cfg.SizeTextMedium)
	}
	if theme.TextStyleTitleSmall.Size != cfg.SizeTextMedium {
		t.Errorf("TitleSmall Size = %v, want ladder %v",
			theme.TextStyleTitleSmall.Size, cfg.SizeTextMedium)
	}
}

// TestTextRolesFamilies pins the mono and icon families, and the
// glyph mark that tells optical centring a run is icons, not text.
func TestTextRolesFamilies(t *testing.T) {
	theme := themeRolesTestTheme()
	mono := theme.Cfg.MonoFontFamily

	for _, role := range []TextStyle{
		theme.TextStyleCode,
		theme.TextStyleCodeSmall,
		theme.TextStyleCodeTiny,
	} {
		if role.Family != mono {
			t.Errorf("Code family = %q, want mono %q", role.Family, mono)
		}
	}

	icons := []TextStyle{
		theme.TextStyleIconXLarge,
		theme.TextStyleIconLarge,
		theme.TextStyleIconMedium,
		theme.TextStyleIconSmall,
		theme.TextStyleIconXSmall,
		theme.TextStyleIconTiny,
	}
	for i, role := range icons {
		if !role.glyphRole {
			t.Errorf("Icon role %d missing glyphRole", i)
		}
	}
	for _, role := range []TextStyle{
		theme.TextStyleBody,
		theme.TextStyleDisplay,
		theme.TextStyleCode,
	} {
		if role.glyphRole {
			t.Errorf("text role %+v carries glyphRole", role)
		}
	}
}

// TestIconRolesFamily pins the icon family on every icon role. The
// derivation sets it on one line; dropping that line renders tofu in
// the body family with no size change for the ladder test to catch.
func TestIconRolesFamily(t *testing.T) {
	theme := themeRolesTestTheme()

	icons := []TextStyle{
		theme.TextStyleIconXLarge,
		theme.TextStyleIconLarge,
		theme.TextStyleIconMedium,
		theme.TextStyleIconSmall,
		theme.TextStyleIconXSmall,
		theme.TextStyleIconTiny,
	}
	for i, role := range icons {
		if role.Family != IconFontName {
			t.Errorf("Icon role %d Family = %q, want %q",
				i, role.Family, IconFontName)
		}
	}
}

// TestTextModifierFaces pins every input-face transition, so the
// slant-preserving branches fail here rather than as a wrong pixel
// downstream.
func TestTextModifierFaces(t *testing.T) {
	theme := themeRolesTestTheme()
	base := theme.TextStyleBody

	cases := []struct {
		in      glyph.Typeface
		bold    glyph.Typeface
		italic  glyph.Typeface
		regular glyph.Typeface
	}{
		{glyph.TypefaceRegular, glyph.TypefaceBold, glyph.TypefaceItalic, glyph.TypefaceRegular},
		{glyph.TypefaceBold, glyph.TypefaceBold, glyph.TypefaceBoldItalic, glyph.TypefaceRegular},
		{glyph.TypefaceItalic, glyph.TypefaceBoldItalic, glyph.TypefaceItalic, glyph.TypefaceRegular},
		{glyph.TypefaceBoldItalic, glyph.TypefaceBoldItalic, glyph.TypefaceBoldItalic, glyph.TypefaceRegular},
	}

	for _, c := range cases {
		base.Typeface = c.in
		if got := base.Bold().Typeface; got != c.bold {
			t.Errorf("Bold(%v) = %v, want %v", c.in, got, c.bold)
		}
		if got := base.Italic().Typeface; got != c.italic {
			t.Errorf("Italic(%v) = %v, want %v", c.in, got, c.italic)
		}
		if got := base.Regular().Typeface; got != c.regular {
			t.Errorf("Regular(%v) = %v, want %v", c.in, got, c.regular)
		}
	}
}

// TestTextModifiersKeepFields pins the donor promise: modifiers
// retarget face (Mono the family) and never touch size or color, so a
// donor with an overridden size spells exactly the old rung.
func TestTextModifiersKeepFields(t *testing.T) {
	theme := themeRolesTestTheme()
	donor := theme.TextStyleDisplay
	donor.Size = 120
	donor.Color = White

	mods := []struct {
		name string
		mod  func(TextStyle) TextStyle
	}{
		{"Bold", TextStyle.Bold},
		{"Italic", TextStyle.Italic},
		{"Regular", TextStyle.Regular},
	}

	for _, m := range mods {
		got := m.mod(donor)
		if got.Size != 120 || got.Color != donor.Color {
			t.Errorf("%s changed Size/Color: %+v", m.name, got)
		}
		if got.Family != donor.Family {
			t.Errorf("%s changed Family: %+v", m.name, got)
		}
	}

	if got := theme.Mono(donor); got.Typeface != donor.Typeface {
		t.Errorf("Mono changed Typeface: %+v", got)
	}
	if got := theme.Mono(donor); got.Color != donor.Color {
		t.Errorf("Mono changed Color: %+v", got)
	}
}

// TestTextModifierLaws pins order-independence and idempotence, so a
// donor chain means the same value however it is written.
func TestTextModifierLaws(t *testing.T) {
	theme := themeRolesTestTheme()
	body := theme.TextStyleBody

	if body.Bold().Italic() != body.Italic().Bold() {
		t.Errorf("Bold().Italic() != Italic().Bold(): %+v vs %+v",
			body.Bold().Italic(), body.Italic().Bold())
	}
	if body.Bold().Bold() != body.Bold() {
		t.Errorf("Bold() not idempotent: %+v vs %+v",
			body.Bold().Bold(), body.Bold())
	}
	if body.Italic().Italic() != body.Italic() {
		t.Errorf("Italic() not idempotent: %+v vs %+v",
			body.Italic().Italic(), body.Italic())
	}
	if body.Regular().Regular() != body.Regular() {
		t.Errorf("Regular() not idempotent: %+v vs %+v",
			body.Regular().Regular(), body.Regular())
	}
	if body.Bold().Typeface != glyph.TypefaceBold {
		t.Errorf("Bold Typeface = %v, want Bold",
			body.Bold().Typeface)
	}
	if theme.TextStyleDisplay.Regular().Typeface != glyph.TypefaceRegular {
		t.Errorf("Display.Regular() Typeface = %v, want Regular",
			theme.TextStyleDisplay.Regular().Typeface)
	}
	if want := theme.Cfg.MonoFontFamily; theme.Mono(body).Family != want {
		t.Errorf("Mono family = %q, want theme mono %q",
			theme.Mono(body).Family, want)
	}
	if sized := theme.Mono(body).Size; sized != body.Size+1 {
		t.Errorf("Mono Size = %v, want compensation %v",
			sized, body.Size+1)
	}
}
