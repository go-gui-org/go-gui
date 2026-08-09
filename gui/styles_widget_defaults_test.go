package gui

// Pins the Default*Style package vars declared in
// styles_widget_control.go and styles_widget_overlay.go.
//
// Go-Gui has two sources of widget defaults: these package vars (used
// when a widget renders without a theme-supplied style) and ThemeMaker
// (which builds every style from a ThemeCfg). They are maintained by
// hand and drift apart silently. The tests below pin the package-var
// side and call out, in comments, each place the two disagree — so a
// future edit to one has to be a deliberate decision about the other.

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// SetTheme (theme.go:236) overwrites every Default*Style global from
// the active theme, and nothing restores them. Any earlier test that
// switches themes therefore leaves the live vars holding theme values
// rather than the declared literals, which makes naive assertions
// order-dependent (and repeat-run dependent under -count=N).
//
// These snapshots are package-level vars, so Go initializes them from
// the declared literals before any test runs. Assert against the
// snapshot, never the live global.
var (
	pristineProgressBarStyle    = defaultProgressBarStyle
	pristineSliderStyle         = defaultSliderStyle
	pristineTabControlStyle     = defaultTabControlStyle
	pristineBreadcrumbStyle     = defaultBreadcrumbStyle
	pristineSplitterStyle       = defaultSplitterStyle
	pristineTableStyle          = defaultTableStyle
	pristineComboboxStyle       = defaultComboboxStyle
	pristineCommandPaletteStyle = defaultCommandPaletteStyle
	pristineDatePickerStyle     = defaultDatePickerStyle
	pristineColorPickerStyle    = defaultColorPickerStyle
	pristineSkeletonStyle       = defaultSkeletonStyle
	pristineMenubarStyle        = defaultMenubarStyle
	pristineBadgeStyle          = defaultBadgeStyle
	pristineExpandPanelStyle    = defaultExpandPanelStyle
	pristineDialogStyle         = DefaultDialogStyle
	pristineToastStyle          = defaultToastStyle
	pristineTreeStyle           = defaultTreeStyle
	pristineDataGridStyle       = DefaultDataGridStyle
	pristineTextStyle           = DefaultTextStyle
	pristineListBoxStyle        = defaultListBoxStyle
	pristineRadioStyle          = defaultRadioStyle
	pristineSwitchStyle         = defaultSwitchStyle
	pristineToggleStyle         = defaultToggleStyle
	pristineSelectStyle         = defaultSelectStyle
	pristineInputStyle          = defaultInputStyle
	pristineScrollbarStyle      = DefaultScrollbarStyle
	pristineTooltipStyle        = defaultTooltipStyle
)

// --- styles_widget_control.go ----------------------------------------

func TestDefaultProgressBarStyle(t *testing.T) {
	s := pristineProgressBarStyle
	// ThemeMaker drives Size from cfg.SizeProgressBar and never sets
	// SizeBorder at all; the package var hard-codes both.
	if s.Size != 20 {
		t.Errorf("size = %v, want 20", s.Size)
	}
	if s.SizeBorder != 0 {
		t.Errorf("size_border = %v, want 0 (progress bar is borderless)", s.SizeBorder)
	}
	if s.Radius != radiusSmall {
		t.Errorf("radius = %v, want RadiusSmall %v", s.Radius, radiusSmall)
	}
	if !s.TextShow {
		t.Error("text_show should default true")
	}
	if !s.textBackground.eq(ColorTransparent) {
		t.Errorf("text_background = %v, want transparent", s.textBackground)
	}
}

func TestDefaultSliderStyle(t *testing.T) {
	s := pristineSliderStyle
	// Both of these are bare literals, not the SizeBorderDef / Radius*
	// constants — and ThemeMaker computes Radius as cfg.SizeSlider/2.
	if s.SizeBorder != 1 {
		t.Errorf("size_border = %v, want literal 1 (not SizeBorderDef %v)",
			s.SizeBorder, sizeBorderDef)
	}
	if s.Radius != 3 {
		t.Errorf("radius = %v, want literal 3", s.Radius)
	}
	if s.Size != 6 {
		t.Errorf("size = %v, want 6", s.Size)
	}
	if s.ThumbSize != 16 {
		t.Errorf("thumb_size = %v, want 16", s.ThumbSize)
	}
}

func TestDefaultTabControlStyle(t *testing.T) {
	s := pristineTabControlStyle
	if s.SizeBorder != sizeBorderDef || s.sizeTabBorder != sizeBorderDef {
		t.Errorf("borders = %v/%v, want SizeBorderDef %v",
			s.SizeBorder, s.sizeTabBorder, sizeBorderDef)
	}
	if s.Radius != radiusMedium || s.radiusTab != radiusSmall {
		t.Errorf("radius/radius_tab = %v/%v, want %v/%v",
			s.Radius, s.radiusTab, radiusMedium, radiusSmall)
	}
	if s.spacingHeader != 2 {
		t.Errorf("spacing_header = %v, want 2", s.spacingHeader)
	}
	// Disabled text is the normal text color at alpha 130.
	if s.textStyleDisabled.Color.A != 130 {
		t.Errorf("disabled alpha = %d, want 130", s.textStyleDisabled.Color.A)
	}
}

func TestDefaultBreadcrumbStyle(t *testing.T) {
	s := pristineBreadcrumbStyle
	if s.Separator != "/" {
		t.Errorf("separator = %q, want %q", s.Separator, "/")
	}
	// Disabled crumbs sit at alpha 130, separators at the lighter 160.
	if s.textStyleDisabled.Color.A != 130 {
		t.Errorf("disabled alpha = %d, want 130", s.textStyleDisabled.Color.A)
	}
	if s.textStyleSeparator.Color.A != 160 {
		t.Errorf("separator alpha = %d, want 160", s.textStyleSeparator.Color.A)
	}
	if s.sizeContentBorder != sizeBorderDef {
		t.Errorf("content border = %v, want %v", s.sizeContentBorder, sizeBorderDef)
	}
}

func TestDefaultSplitterStyle(t *testing.T) {
	s := pristineSplitterStyle
	if s.HandleSize != 9 {
		t.Errorf("handle_size = %v, want 9", s.HandleSize)
	}
	// Keyboard drag increments: 2% normal, 10% with a modifier.
	if s.dragStep != 0.02 {
		t.Errorf("drag_step = %v, want 0.02", s.dragStep)
	}
	if s.dragStepLarge != 0.10 {
		t.Errorf("drag_step_large = %v, want 0.10", s.dragStepLarge)
	}
}

func TestDefaultTableStyle(t *testing.T) {
	s := pristineTableStyle
	if s.columnWidthDefault != 50 {
		t.Errorf("column_width_default = %v, want 50", s.columnWidthDefault)
	}
	if s.columnWidthMin != 20 {
		t.Errorf("column_width_min = %v, want 20", s.columnWidthMin)
	}
	if s.alignHead != HAlignCenter {
		t.Errorf("align_head = %v, want center", s.alignHead)
	}
	// Header is bold at the normal text size. ThemeMaker instead
	// overwrites TextStyleHead with theme.B3 after the struct literal.
	if s.TextStyleHead.Typeface != glyph.TypefaceBold {
		t.Errorf("head typeface = %v, want bold", s.TextStyleHead.Typeface)
	}
	if s.TextStyleHead.Size != pristineTextStyle.Size {
		t.Errorf("head size = %v, want %v", s.TextStyleHead.Size, pristineTextStyle.Size)
	}
}

func TestDefaultComboboxStyle(t *testing.T) {
	s := pristineComboboxStyle
	// Fixed RadiusMedium here; ThemeMaker uses cfg.Radius, so a
	// square-cornered theme still gets a rounded unthemed combobox.
	if s.Radius != radiusMedium {
		t.Errorf("radius = %v, want RadiusMedium %v", s.Radius, radiusMedium)
	}
	if s.MinWidth != 75 || s.MaxWidth != 200 {
		t.Errorf("width bounds = %v/%v, want 75/200", s.MinWidth, s.MaxWidth)
	}
	if s.maxDropdownHeight != 200 {
		t.Errorf("max_dropdown_height = %v, want 200", s.maxDropdownHeight)
	}
	// Placeholder alpha 100 matches ThemeMaker's placeholderColor.
	if s.PlaceholderStyle.Color.A != 100 {
		t.Errorf("placeholder alpha = %d, want 100", s.PlaceholderStyle.Color.A)
	}
}

func TestDefaultCommandPaletteStyle(t *testing.T) {
	s := pristineCommandPaletteStyle
	if s.Width != 500 || s.MaxHeight != 400 {
		t.Errorf("size = %v x %v, want 500 x 400", s.Width, s.MaxHeight)
	}
	// Fixed grey; ThemeMaker derives the detail color from the theme
	// text color at alpha 140 instead.
	if !s.detailStyle.Color.eq(RGBA(128, 128, 128, 200)) {
		t.Errorf("detail color = %v, want RGBA(128,128,128,200)", s.detailStyle.Color)
	}
	if !s.backdropColor.eq(RGBA(0, 0, 0, 120)) {
		t.Errorf("backdrop = %v, want RGBA(0,0,0,120)", s.backdropColor)
	}
}

func TestDefaultDatePickerStyle(t *testing.T) {
	s := pristineDatePickerStyle
	if s.cellSpacing != 2 {
		t.Errorf("cell_spacing = %v, want 2", s.cellSpacing)
	}
	if s.SizeBorder != sizeBorderDef {
		t.Errorf("size_border = %v, want %v", s.SizeBorder, sizeBorderDef)
	}
	if s.Radius != radiusMedium || s.radiusBorder != radiusMedium {
		t.Errorf("radius/radius_border = %v/%v, want both %v",
			s.Radius, s.radiusBorder, radiusMedium)
	}
}

func TestDefaultColorPickerStyle(t *testing.T) {
	s := pristineColorPickerStyle
	if s.sVSize != 200 {
		t.Errorf("sv_size = %v, want 200", s.sVSize)
	}
	if s.sliderHeight != 24 {
		t.Errorf("slider_height = %v, want 24", s.sliderHeight)
	}
	if s.indicatorSize != 16 {
		t.Errorf("indicator_size = %v, want 16", s.indicatorSize)
	}
}

// ColorHighlight is the one computed value in the control defaults:
// colorInteriorDark.Add(RGBA(20,20,20,0)). Assert the resolved color so
// a change to either the base color or Color.Add saturation shows up.
func TestDefaultSkeletonStyleHighlightResolved(t *testing.T) {
	s := pristineSkeletonStyle
	want := RGBA(94, 94, 94, 255) // 74+20 per channel, alpha unchanged
	if !s.ColorHighlight.eq(want) {
		t.Errorf("color_highlight = %v, want %v", s.ColorHighlight, want)
	}
	// The highlight must be lighter than the base or the shimmer is invisible.
	if s.ColorHighlight.R <= s.Color.R {
		t.Errorf("highlight R %d not lighter than base R %d",
			s.ColorHighlight.R, s.Color.R)
	}
	if s.Radius != radiusSmall {
		t.Errorf("radius = %v, want RadiusSmall %v", s.Radius, radiusSmall)
	}
}

func TestDefaultMenubarStyle(t *testing.T) {
	s := pristineMenubarStyle
	if s.widthSubmenuMin != 50 || s.widthSubmenuMax != 200 {
		t.Errorf("submenu width = %v/%v, want 50/200",
			s.widthSubmenuMin, s.widthSubmenuMax)
	}
	if s.Spacing != SpacingMedium {
		t.Errorf("spacing = %v, want SpacingMedium %v", s.Spacing, SpacingMedium)
	}
	// Submenu items are flush by default; ThemeMaker uses 1 instead.
	if s.spacingSubmenu != 0 {
		t.Errorf("spacing_submenu = %v, want 0", s.spacingSubmenu)
	}
	if s.Radius != radiusSmall || s.radiusBorder != radiusMedium {
		t.Errorf("radius/radius_border = %v/%v, want %v/%v",
			s.Radius, s.radiusBorder, radiusSmall, radiusMedium)
	}
}

// --- styles_widget_overlay.go ----------------------------------------

// DefaultBadgeStyle leaves TextStyle at its zero value, unlike
// ThemeMaker which sets it to B5 with a White color. A badge rendered
// without a theme therefore inherits size 0 and must be given a style
// by the caller.
func TestDefaultBadgeStyle(t *testing.T) {
	s := pristineBadgeStyle
	if s.TextStyle != (TextStyle{}) {
		t.Errorf("text_style = %+v, want zero value", s.TextStyle)
	}
	if s.dotSize != 8 {
		t.Errorf("dot_size = %v, want 8", s.dotSize)
	}
	// Semantic colors are hard-coded rather than theme-derived, and
	// match the toast palette exactly.
	if !s.ColorSuccess.eq(pristineToastStyle.ColorSuccess) {
		t.Errorf("badge success %v != toast success %v",
			s.ColorSuccess, pristineToastStyle.ColorSuccess)
	}
	if !s.ColorWarning.eq(pristineToastStyle.ColorWarning) {
		t.Errorf("badge warning %v != toast warning %v",
			s.ColorWarning, pristineToastStyle.ColorWarning)
	}
	if !s.ColorError.eq(pristineToastStyle.ColorError) {
		t.Errorf("badge error %v != toast error %v",
			s.ColorError, pristineToastStyle.ColorError)
	}
}

func TestDefaultExpandPanelStyle(t *testing.T) {
	s := pristineExpandPanelStyle
	if s.SizeBorder != sizeBorderDef {
		t.Errorf("size_border = %v, want %v", s.SizeBorder, sizeBorderDef)
	}
	if s.Radius != radiusMedium || s.radiusBorder != radiusMedium {
		t.Errorf("radius/radius_border = %v/%v, want both %v",
			s.Radius, s.radiusBorder, radiusMedium)
	}
	if s.Color.eq(Color{}) {
		t.Error("color should not be the zero Color")
	}
	if s.ColorHover.eq(s.colorClick) {
		t.Error("hover and click colors should differ")
	}
}

// ToastAnchor is a bare iota enum with no String and no validation, so
// its ordering is load-bearing for anything persisting the value.
func TestToastAnchorConstants(t *testing.T) {
	anchors := []struct {
		got  toastAnchor
		want toastAnchor
		name string
	}{
		{toastTopLeft, 0, "ToastTopLeft"},
		{toastTopRight, 1, "ToastTopRight"},
		{toastBottomLeft, 2, "ToastBottomLeft"},
		{toastBottomRight, 3, "ToastBottomRight"},
	}
	for _, a := range anchors {
		if a.got != a.want {
			t.Errorf("%s = %d, want %d", a.name, a.got, a.want)
		}
	}
}

// DefaultDialogStyle leaves the width bounds unset while ThemeMaker
// sets 200/300 — an unthemed dialog is therefore unconstrained.
func TestDefaultDialogStyleWidthBoundsUnset(t *testing.T) {
	s := pristineDialogStyle
	if s.MinWidth != 0 || s.MaxWidth != 0 {
		t.Errorf("width bounds = %v/%v, want 0/0 (ThemeMaker sets 200/300)",
			s.MinWidth, s.MaxWidth)
	}
}

func TestDefaultTreeStyleIndentAndSpacing(t *testing.T) {
	s := pristineTreeStyle
	if s.indent != 25 {
		t.Errorf("indent = %v, want 25", s.indent)
	}
	if s.Spacing != 0 {
		t.Errorf("spacing = %v, want 0", s.Spacing)
	}
	// Tree chevrons come from the bundled icon font, not the text font.
	if s.textStyleIcon.Family != IconFontName {
		t.Errorf("icon family = %q, want %q", s.textStyleIcon.Family, IconFontName)
	}
}

// --- styles.go --------------------------------------------------------

// DefaultDataGridStyle is declared with no initializer. That is
// intentional — the datagrid resolves its style from the theme — but it
// means the package var is not a usable fallback like its siblings.
func TestDefaultDataGridStyleIsZeroValue(t *testing.T) {
	if pristineDataGridStyle != (DataGridStyle{}) {
		t.Errorf("DefaultDataGridStyle = %+v, want the zero value", pristineDataGridStyle)
	}
}

// ToGlyphStyle maps only the subset glyph needs. Align, LineSpacing,
// RotationRadians, AffineTransform and Gradient are handled by go-gui's
// own render path — glyph.TextStyle has no counterpart fields at all,
// so the drop is enforced at compile time. What is worth asserting is
// that populating those go-gui-only fields does not perturb the fields
// that do cross.
func TestToGlyphStyleDropsGoGuiOnlyFields(t *testing.T) {
	affine := glyph.AffineRotation(1.0)
	base := TextStyle{
		Family:        "Menlo",
		Color:         RGBA(1, 2, 3, 255),
		Size:          17,
		LetterSpacing: 1.5,
		Underline:     true,
		Typeface:      glyph.TypefaceBold,
	}

	decorated := base
	decorated.Align = TextAlignCenter
	decorated.LineSpacing = 2.5
	decorated.RotationRadians = 1.0
	decorated.AffineTransform = &affine
	decorated.Gradient = &glyph.GradientConfig{}

	if base.toGlyphStyle() != decorated.toGlyphStyle() {
		t.Errorf("go-gui-only fields changed the glyph style:\n plain = %+v\n decorated = %+v",
			base.toGlyphStyle(), decorated.toGlyphStyle())
	}
}
