package gui

import "testing"

func TestWithColorsNilPreservesDefaults(t *testing.T) {
	theme := ThemeDark
	updated := theme.WithColors(ColorOverrides{})
	if updated.ColorBackground != theme.ColorBackground {
		t.Error("nil override should preserve background")
	}
	if updated.ButtonStyle.Color != theme.ButtonStyle.Color {
		t.Error("nil override should preserve button color")
	}
}

func TestWithColorsOverridesBackground(t *testing.T) {
	theme := ThemeDark
	newBg := Red
	updated := theme.WithColors(ColorOverrides{
		ColorBackground: newBg,
	})
	if updated.ColorBackground != Red {
		t.Error("background should be overridden")
	}
}

func TestWithColorsOverridesPropagates(t *testing.T) {
	theme := ThemeDark
	newInterior := Blue
	updated := theme.WithColors(ColorOverrides{
		ColorInterior: newInterior,
	})
	if updated.ButtonStyle.Color != Blue {
		t.Error("interior override should propagate to button color")
	}
	if updated.InputStyle.Color != Blue {
		t.Error("interior override should propagate to input color")
	}
	if updated.dataGridStyle.ColorBackground != Blue {
		t.Error("interior override should propagate to data grid background")
	}
}

func TestWithColorsOverridesBorder(t *testing.T) {
	theme := ThemeDark
	newBorder := Green
	updated := theme.WithColors(ColorOverrides{
		ColorBorder: newBorder,
	})
	if updated.ColorBorder != Green {
		t.Error("border should be overridden on theme")
	}
	if updated.ButtonStyle.ColorBorder != Green {
		t.Error("border should propagate to button")
	}
	if updated.rectangleStyle.ColorBorder != Green {
		t.Error("border should propagate to rectangle")
	}
}

func TestWithColorsSelectPropagates(t *testing.T) {
	theme := ThemeDark
	newSel := RGBA(255, 0, 255, 255)
	updated := theme.WithColors(ColorOverrides{
		ColorSelect: newSel,
	})
	if updated.ColorSelect != newSel {
		t.Error("select should be overridden on theme")
	}
	if updated.radioStyle.ColorFocus != newSel {
		t.Error("select should propagate to radio focus")
	}
}

// colorTextOnSelectByStyle collects the token across every style
// that draws text over a full select fill. One place to update when a
// style adopts the token. The list-like widgets paint the subtle wash
// now and need no paired text (visual-refresh §4.3); only the menu
// surface and the date picker's selected day still fill with the
// accent.
func colorTextOnSelectByStyle(t Theme) map[string]Color {
	return map[string]Color{
		"MenubarStyle":    t.MenubarStyle.ColorTextOnSelect,
		"DatePickerStyle": t.datePickerStyle.ColorTextOnSelect,
	}
}

func TestWithColorsTextOnSelectPropagates(t *testing.T) {
	theme := ThemeDark
	newTxt := RGBA(10, 20, 30, 255)
	updated := theme.WithColors(ColorOverrides{
		ColorTextOnSelect: newTxt,
	})
	if updated.ColorTextOnSelect != newTxt {
		t.Error("text-on-select should be overridden on theme")
	}
	for name, got := range colorTextOnSelectByStyle(updated) {
		if got != newTxt {
			t.Errorf("%s.ColorTextOnSelect = %v, want %v",
				name, got, newTxt)
		}
	}
}

func TestAdjustFontSizeBasic(t *testing.T) {
	theme := ThemeDark
	origSize := theme.Cfg.TextStyleDef.Size
	adjusted, err := theme.AdjustFontSize(2, 1, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adjusted.Cfg.TextStyleDef.Size != origSize+2 {
		t.Errorf("size: got %f, want %f", adjusted.Cfg.TextStyleDef.Size, origSize+2)
	}
}

func TestAdjustFontSizeClampsHigh(t *testing.T) {
	theme := ThemeDark
	_, err := theme.AdjustFontSize(1000, 1, 30)
	if err == nil {
		t.Error("should error when new size exceeds max")
	}
}

func TestAdjustFontSizeClampsLow(t *testing.T) {
	theme := ThemeDark
	_, err := theme.AdjustFontSize(-1000, 8, 100)
	if err == nil {
		t.Error("should error when new size below min")
	}
}

func TestAdjustFontSizeMinSizeZero(t *testing.T) {
	theme := ThemeDark
	_, err := theme.AdjustFontSize(0, 0, 100)
	if err == nil {
		t.Error("should error when minSize < 1")
	}
}

func TestAdjustFontSizePreservesOther(t *testing.T) {
	theme := ThemeDark
	adjusted, err := theme.AdjustFontSize(2, 1, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Color should be unchanged.
	if adjusted.ColorBackground != theme.ColorBackground {
		t.Error("font size adjust should not change colors")
	}
}

// An unset border-focus override keeps the theme's current value. Before
// the fix the fallback was ColorSelect, so any unrelated WithColors
// tweak clobbered a theme with an explicit focus border.
func TestWithColorsPreservesBorderFocusWithoutOverride(t *testing.T) {
	cfg := baseDarkCfg()
	cfg.ColorSelect = RGBA(10, 20, 30, 255)
	cfg.ColorBorderFocus = RGBA(200, 100, 50, 255)
	theme := ThemeMaker(cfg)
	newBg := Red
	updated := theme.WithColors(ColorOverrides{ColorBackground: newBg})
	if updated.ButtonStyle.ColorBorderFocus != cfg.ColorBorderFocus {
		t.Errorf("ButtonStyle.ColorBorderFocus = %v, want %v",
			updated.ButtonStyle.ColorBorderFocus, cfg.ColorBorderFocus)
	}
	if updated.InputStyle.ColorBorderFocus != cfg.ColorBorderFocus {
		t.Errorf("InputStyle.ColorBorderFocus = %v, want %v",
			updated.InputStyle.ColorBorderFocus, cfg.ColorBorderFocus)
	}
}

// WithColors stamps a fresh id like every with*Style helper, so the
// install fast path reinstalls instead of keeping stale mirrors.
func TestWithColorsStampsFreshID(t *testing.T) {
	theme := ThemeMaker(baseDarkCfg())
	updated := theme.WithColors(ColorOverrides{})
	if updated.id == 0 {
		t.Error("WithColors id = 0, want a fresh stamped id")
	}
	if updated.id == theme.id {
		t.Error("WithColors reused the parent id, want a fresh one")
	}
}

// WithColors syncs touched fields into Cfg so a later rebuild from
// the configuration (WithPadding, WithBorders, AdjustFontSize) does
// not drop the overrides.
func TestWithColorsSyncsCfg(t *testing.T) {
	theme := ThemeMaker(baseDarkCfg())
	newBg := Red
	updated := theme.WithColors(ColorOverrides{ColorBackground: newBg})
	if updated.Cfg.ColorBackground != Red {
		t.Errorf("Cfg.ColorBackground = %v, want %v",
			updated.Cfg.ColorBackground, Red)
	}
	if updated.Cfg.ColorPanel.IsSet() &&
		updated.Cfg.ColorPanel != theme.Cfg.ColorPanel {
		t.Errorf("untouched Cfg.ColorPanel moved to %v",
			updated.Cfg.ColorPanel)
	}
}

// Overriding select alone moves the accent while the two agree — the
// state every preset ships in — including the derived ramp, the
// border focus that defaults to select, and the paired text.
func TestWithColorsSelectAloneMovesAccentWhenAgree(t *testing.T) {
	theme := ThemeMaker(baseDarkCfg())
	if theme.ColorAccent != theme.ColorSelect {
		t.Skip("preset does not start in the one-decision state")
	}
	newSel := RGB(100, 40, 180)
	updated := theme.WithColors(ColorOverrides{ColorSelect: newSel})
	if updated.ColorSelect != newSel {
		t.Errorf("select = %v, want %v", updated.ColorSelect, newSel)
	}
	if updated.ColorAccent != newSel {
		t.Errorf("accent = %v, want %v (follow select while they agree)",
			updated.ColorAccent, newSel)
	}
	if updated.ColorAccentHover == theme.ColorAccentHover {
		t.Error("accent hover did not re-derive from the new accent")
	}
	if updated.ButtonStylePrimary.Color != newSel {
		t.Errorf("primary button = %v, want %v",
			updated.ButtonStylePrimary.Color, newSel)
	}
	if updated.ButtonStyle.ColorBorderFocus != newSel {
		t.Errorf("border focus = %v, want %v (follow select while derived)",
			updated.ButtonStyle.ColorBorderFocus, newSel)
	}
}

// Stating both select and accent sets them apart: no follow either way.
func TestWithColorsSelectAndAccentFork(t *testing.T) {
	theme := ThemeMaker(baseDarkCfg())
	newSel := RGB(100, 40, 180)
	newAccent := RGB(10, 200, 120)
	updated := theme.WithColors(ColorOverrides{
		ColorSelect: newSel,
		ColorAccent: newAccent,
	})
	if updated.ColorSelect != newSel {
		t.Errorf("select = %v, want %v", updated.ColorSelect, newSel)
	}
	if updated.ColorAccent != newAccent {
		t.Errorf("accent = %v, want %v", updated.ColorAccent, newAccent)
	}
}

// A select-only patch leaves a forked accent alone.
func TestWithColorsSelectAloneKeepsForkedAccent(t *testing.T) {
	cfg := baseDarkCfg()
	cfg.ColorSelect = RGBA(10, 20, 30, 255)
	cfg.ColorAccent = RGBA(200, 100, 50, 255)
	theme := ThemeMaker(cfg)
	newSel := RGB(100, 40, 180)
	updated := theme.WithColors(ColorOverrides{ColorSelect: newSel})
	if updated.ColorSelect != newSel {
		t.Errorf("select = %v, want %v", updated.ColorSelect, newSel)
	}
	if updated.ColorAccent != cfg.ColorAccent {
		t.Errorf("accent = %v, want %v (forked accents stay put)",
			updated.ColorAccent, cfg.ColorAccent)
	}
}

// An explicit fully-transparent override stays honorable instead of
// reading as "unset".
func TestWithColorsTransparentOverrideWins(t *testing.T) {
	theme := ThemeMaker(baseDarkCfg())
	updated := theme.WithColors(ColorOverrides{
		ColorBorderFocus: ColorTransparent,
	})
	if updated.ButtonStyle.ColorBorderFocus != ColorTransparent {
		t.Errorf("border focus = %v, want transparent %v",
			updated.ButtonStyle.ColorBorderFocus, ColorTransparent)
	}
}

// Semantic overrides fan out to the toast, badge and danger button;
// an explicit subtle wash wins over the re-derived one.
func TestWithColorsSemanticFanOut(t *testing.T) {
	theme := ThemeMaker(baseDarkCfg())
	newErr := RGB(200, 30, 30)
	updated := theme.WithColors(ColorOverrides{ColorError: newErr})
	if updated.toastStyle.ColorError != newErr {
		t.Errorf("toast error = %v, want %v",
			updated.toastStyle.ColorError, newErr)
	}
	if updated.badgeStyle.ColorError != newErr {
		t.Errorf("badge error = %v, want %v",
			updated.badgeStyle.ColorError, newErr)
	}
	if updated.ButtonStyleDanger.Color != newErr {
		t.Errorf("danger button = %v, want %v",
			updated.ButtonStyleDanger.Color, newErr)
	}
	if updated.ColorErrorSubtle == theme.ColorErrorSubtle {
		t.Error("error subtle did not re-derive from the new error color")
	}

	explicit := RGBA(1, 2, 3, 40)
	updated = theme.WithColors(ColorOverrides{
		ColorError:       newErr,
		ColorErrorSubtle: explicit,
	})
	if updated.ColorErrorSubtle != explicit {
		t.Errorf("error subtle = %v, want explicit %v",
			updated.ColorErrorSubtle, explicit)
	}
}
