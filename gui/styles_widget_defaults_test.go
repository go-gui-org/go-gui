package gui

// Guards the single-source rule for the default*Style package vars
// (issue #300).
//
// Go-Gui used to have two sources of widget defaults: hand-written
// literals in styles*.go and ThemeMaker. They drifted silently — an app
// that never called SetTheme ran on a mixture of the two. The literals
// are gone; the vars are now mirrors that applyTheme fills from the
// installed theme, and the test below fails the moment someone gives one
// of them an initializer again.

import (
	"reflect"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// Snapshots taken by a test-file init(), which the go tool runs after
// the init() in theme_defaults.go — package-var initializers would be
// too early, since they are evaluated before any init() runs.
var (
	pkgInitButtonStyle      buttonStyle
	pkgInitDataGridStyle    DataGridStyle
	pkgInitInstalledThemeID uint64
)

func init() {
	pkgInitButtonStyle = defaultButtonStyle
	pkgInitDataGridStyle = DefaultDataGridStyle
	pkgInitInstalledThemeID = installedThemeID
}

// TestDefaultStylesMirrorThemeDark pins every mirror to its ThemeDark
// field. Re-adding a literal to styles*.go breaks this immediately,
// because a package-var initializer is evaluated before init() calls
// applyTheme — but applyTheme overwrites it, so the literal would be
// dead weight that only diverges.
//
// reflect.DeepEqual rather than ==: several style structs carry pointer
// fields (*BoxShadow, *GradientDef, *glyph.FontFeatures) where value
// equality is what the test means.
func TestDefaultStylesMirrorThemeDark(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)

	// One entry per assignment in applyTheme (gui/theme.go). Add a row
	// whenever a mirror is added there.
	mirrors := []struct {
		got  any
		want any
		name string
	}{
		{DefaultTextStyle, ThemeDark.TextStyleDef, "DefaultTextStyle"},
		{defaultButtonStyle, ThemeDark.ButtonStyle, "defaultButtonStyle"},
		{defaultContainerStyle, ThemeDark.ContainerStyle, "defaultContainerStyle"},
		{defaultInputStyle, ThemeDark.InputStyle, "defaultInputStyle"},
		{DefaultScrollbarStyle, ThemeDark.ScrollbarStyle, "DefaultScrollbarStyle"},
		{defaultRadioStyle, ThemeDark.radioStyle, "defaultRadioStyle"},
		{defaultSwitchStyle, ThemeDark.switchStyle, "defaultSwitchStyle"},
		{defaultToggleStyle, ThemeDark.toggleStyle, "defaultToggleStyle"},
		{defaultSelectStyle, ThemeDark.selectStyle, "defaultSelectStyle"},
		{defaultListBoxStyle, ThemeDark.listBoxStyle, "defaultListBoxStyle"},
		{defaultTreeStyle, ThemeDark.treeStyle, "defaultTreeStyle"},
		{DefaultDialogStyle, ThemeDark.dialogStyle, "DefaultDialogStyle"},
		{defaultToastStyle, ThemeDark.toastStyle, "defaultToastStyle"},
		{defaultTooltipStyle, ThemeDark.tooltipStyle, "defaultTooltipStyle"},
		{defaultBadgeStyle, ThemeDark.badgeStyle, "defaultBadgeStyle"},
		{defaultExpandPanelStyle, ThemeDark.expandPanelStyle, "defaultExpandPanelStyle"},
		{defaultProgressBarStyle, ThemeDark.progressBarStyle, "defaultProgressBarStyle"},
		{defaultSliderStyle, ThemeDark.sliderStyle, "defaultSliderStyle"},
		{defaultTabControlStyle, ThemeDark.tabControlStyle, "defaultTabControlStyle"},
		{defaultBreadcrumbStyle, ThemeDark.breadcrumbStyle, "defaultBreadcrumbStyle"},
		{defaultSplitterStyle, ThemeDark.splitterStyle, "defaultSplitterStyle"},
		{defaultTableStyle, ThemeDark.tableStyle, "defaultTableStyle"},
		{defaultComboboxStyle, ThemeDark.comboboxStyle, "defaultComboboxStyle"},
		{defaultCommandPaletteStyle, ThemeDark.commandPaletteStyle, "defaultCommandPaletteStyle"},
		{defaultMenubarStyle, ThemeDark.MenubarStyle, "defaultMenubarStyle"},
		{defaultDatePickerStyle, ThemeDark.datePickerStyle, "defaultDatePickerStyle"},
		{defaultColorPickerStyle, ThemeDark.colorPickerStyle, "defaultColorPickerStyle"},
		{DefaultDataGridStyle, ThemeDark.dataGridStyle, "DefaultDataGridStyle"},
		{defaultSkeletonStyle, ThemeDark.skeletonStyle, "defaultSkeletonStyle"},
		{defaultSeparatorStyle, ThemeDark.separatorStyle, "defaultSeparatorStyle"},
		{defaultInspectorStyle, ThemeDark.inspectorStyle, "defaultInspectorStyle"},
	}

	for _, m := range mirrors {
		if !reflect.DeepEqual(m.got, m.want) {
			t.Errorf("%s does not match ThemeDark:\n got  = %+v\n want = %+v",
				m.name, m.got, m.want)
		}
	}
}

// TestDefaultStylesFilledAtInit is the half the table above cannot cover:
// it asserts the mirrors are populated by package init alone, with no
// SetTheme call anywhere. That is precisely the case issue #300 broke —
// a fresh app used to run on the literals until it switched themes.
func TestDefaultStylesFilledAtInit(t *testing.T) {
	if pkgInitDataGridStyle == (DataGridStyle{}) {
		t.Error("DefaultDataGridStyle was still the zero value after init; " +
			"init must call applyTheme(ThemeDark)")
	}
	if pkgInitButtonStyle != ThemeDark.ButtonStyle {
		t.Errorf("defaultButtonStyle after init = %+v, want ThemeDark.ButtonStyle %+v",
			pkgInitButtonStyle, ThemeDark.ButtonStyle)
	}
	if pkgInitInstalledThemeID != ThemeDark.id {
		t.Errorf("installedThemeID after init = %d, want ThemeDark.id %d",
			pkgInitInstalledThemeID, ThemeDark.id)
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
