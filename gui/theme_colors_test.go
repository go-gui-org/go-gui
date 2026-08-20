package gui

import "testing"

func TestWithColorsNilPreservesDefaults(t *testing.T) {
	theme := ThemeDark
	updated := theme.withColors(ColorOverrides{})
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
	updated := theme.withColors(ColorOverrides{
		ColorBackground: &newBg,
	})
	if updated.ColorBackground != Red {
		t.Error("background should be overridden")
	}
}

func TestWithColorsOverridesPropagates(t *testing.T) {
	theme := ThemeDark
	newInterior := Blue
	updated := theme.withColors(ColorOverrides{
		ColorInterior: &newInterior,
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
	updated := theme.withColors(ColorOverrides{
		ColorBorder: &newBorder,
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
	updated := theme.withColors(ColorOverrides{
		ColorSelect: &newSel,
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
	updated := theme.withColors(ColorOverrides{
		ColorTextOnSelect: &newTxt,
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
