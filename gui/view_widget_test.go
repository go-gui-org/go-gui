package gui

import "testing"

func noop(ctx EventCtx) {}

// --- Radio ---

func TestRadioGeneratesLayout(t *testing.T) {
	w := newTestWindow()
	v := Radio(RadioCfg{
		Label:   "Option A",
		OnClick: noop, ID: "f1",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleRadioButton {
		t.Fatalf("got role %d, want RadioButton", layout.Shape.A11YRole)
	}
	// Children: circle + label row
	if len(layout.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(layout.Children))
	}
}

func TestRadioSelectedState(t *testing.T) {
	w := newTestWindow()
	v := Radio(RadioCfg{
		ID:       "widget_test_test_radio_selected_state",
		OnClick:  noop,
		Selected: true,
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YState != AccessStateSelected {
		t.Fatalf("got state %d, want Selected", layout.Shape.A11YState)
	}
}

func TestRadioNoLabel(t *testing.T) {
	w := newTestWindow()
	v := Radio(RadioCfg{ID: "widget_test_test_radio_no_label", OnClick: noop})
	layout := generateViewLayout(v, w)
	// Only circle child.
	if len(layout.Children) != 1 {
		t.Fatalf("got %d children, want 1", len(layout.Children))
	}
}

func TestRadioDisabledCircle(t *testing.T) {
	w := newTestWindow()
	v := Radio(RadioCfg{
		OnClick: noop, ID: "f1",
		Disabled: true,
	})
	layout := generateViewLayout(v, w)
	if !layout.Children[0].Shape.Disabled {
		t.Error("circle child should be disabled")
	}
}

func TestRadioDisabledSuppressesHover(t *testing.T) {
	w := newTestWindow()
	v := Radio(RadioCfg{ID: "widget_test_test_radio_disabled_suppresses_hover", OnClick: noop, Disabled: true})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled {
		t.Fatal("outer row should be disabled")
	}
	origBorder := layout.Children[0].Shape.ColorBorder
	e := &Event{MouseButton: MouseInvalid}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	if layout.Children[0].Shape.ColorBorder != origBorder {
		t.Error("hover should not change border when disabled")
	}
}

func TestRadioHoverChangesBorder(t *testing.T) {
	w := newTestWindow()
	v := Radio(RadioCfg{ID: "widget_test_test_radio_hover_changes_border", OnClick: noop, Label: "X"})
	layout := generateViewLayout(v, w)
	origBorder := layout.Children[0].Shape.ColorBorder
	// MouseInvalid = no button pressed (MouseLeft = 0 is zero value).
	e := &Event{MouseButton: MouseInvalid}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	if layout.Children[0].Shape.ColorBorder == origBorder {
		t.Error("hover should change circle border color")
	}
}

func TestRadioClickHoverChangesBorder(t *testing.T) {
	w := newTestWindow()
	v := Radio(RadioCfg{ID: "widget_test_test_radio_click_hover_changes_border", OnClick: noop, Label: "X"})
	layout := generateViewLayout(v, w)
	clickColor := defaultRadioStyle.colorClick
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	got := layout.Children[0].Shape.ColorBorder
	if got != clickColor {
		t.Errorf("got %v, want click color %v", got, clickColor)
	}
}

func TestRadioFocusBorder(t *testing.T) {
	w := newTestWindow()
	w.viewState.focusID = "f5"
	v := Radio(RadioCfg{OnClick: noop, ID: "f5"})
	layout := generateViewLayout(v, w)
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if layout.Children[0].Shape.ColorBorder != defaultRadioStyle.ColorBorderFocus {
		t.Errorf("focus border = %v, want %v",
			layout.Children[0].Shape.ColorBorder,
			defaultRadioStyle.ColorBorderFocus)
	}
}

func TestRadioUsesRadioStyleDefaults(t *testing.T) {
	w := newTestWindow()
	v := Radio(RadioCfg{ID: "widget_test_test_radio_uses_radio_style_defaults", OnClick: noop, Label: "Y"})
	layout := generateViewLayout(v, w)
	// Padding should come from DefaultRadioStyle, not NoPadding.
	got := layout.Shape.Padding
	want := defaultRadioStyle.Padding
	if got != want {
		t.Errorf("padding = %v, want %v", got, want)
	}
}

func TestRadioCustomTextStyleMerged(t *testing.T) {
	w := newTestWindow()
	custom := TextStyle{Color: RGBA(255, 0, 0, 255)}
	v := Radio(RadioCfg{
		ID:        "widget_test_test_radio_custom_text_style_merged",
		OnClick:   noop,
		Label:     "Z",
		TextStyle: custom,
	})
	layout := generateViewLayout(v, w)
	// Label is second child (row with text).
	labelRow := layout.Children[1]
	textLayout := labelRow.Children[0]
	ts := textLayout.Shape.TC.TextStyle
	if ts.Color != custom.Color {
		t.Errorf("color = %v, want custom red", ts.Color)
	}
	// Size should be merged from default, not zero.
	if ts.Size == 0 {
		t.Error("Size should be merged from default, got 0")
	}
}

// --- Toggle / Checkbox ---

func TestToggleGeneratesLayout(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{
		Label:   "Accept",
		OnClick: noop,
		ID:      "f2",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleCheckbox {
		t.Fatalf("got role %d, want Checkbox", layout.Shape.A11YRole)
	}
	// Children: toggle box + label text
	if len(layout.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(layout.Children))
	}
}

func TestCheckboxIsToggleAlias(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_checkbox_is_toggle_alias", OnClick: noop})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleCheckbox {
		t.Fatalf("got role %d, want Checkbox", layout.Shape.A11YRole)
	}
}

func TestToggleCheckedState(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_toggle_checked_state", OnClick: noop, Selected: true})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YState != AccessStateChecked {
		t.Fatalf("got state %d, want Checked", layout.Shape.A11YState)
	}
}

func TestToggleDisabledSuppressesHover(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_toggle_disabled_suppresses_hover", OnClick: noop, Disabled: true})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled {
		t.Fatal("outer row should be disabled")
	}
	origColor := layout.Children[0].Shape.Color
	e := &Event{MouseButton: MouseInvalid}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	if layout.Children[0].Shape.Color != origColor {
		t.Error("hover should not change color when disabled")
	}
}

func TestToggleNilOnClickSuppressesHover(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_toggle_nil_on_click_suppresses_hover"})
	layout := generateViewLayout(v, w)
	origColor := layout.Children[0].Shape.Color
	e := &Event{MouseButton: MouseInvalid}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	if layout.Children[0].Shape.Color != origColor {
		t.Error("hover should not change color without OnClick")
	}
}

func TestToggleHoverChangesColor(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_toggle_hover_changes_color", OnClick: noop})
	layout := generateViewLayout(v, w)
	origColor := layout.Children[0].Shape.Color
	e := &Event{MouseButton: MouseInvalid}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	if layout.Children[0].Shape.Color == origColor {
		t.Error("hover should change box color")
	}
}

func TestToggleClickHoverChangesColor(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_toggle_click_hover_changes_color", OnClick: noop})
	layout := generateViewLayout(v, w)
	clickColor := defaultToggleStyle.colorClick
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	got := layout.Children[0].Shape.Color
	if got != clickColor {
		t.Errorf("got %v, want click color %v", got, clickColor)
	}
}

func TestToggleFocusBorder(t *testing.T) {
	w := newTestWindow()
	w.viewState.focusID = "f5"
	v := Toggle(ToggleCfg{OnClick: noop, ID: "f5"})
	layout := generateViewLayout(v, w)
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if layout.Children[0].Shape.ColorBorder != defaultToggleStyle.ColorBorderFocus {
		t.Errorf("focus border = %v, want %v",
			layout.Children[0].Shape.ColorBorder,
			defaultToggleStyle.ColorBorderFocus)
	}
}

func TestToggleDefaultStyles(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_toggle_default_styles", OnClick: noop})
	layout := generateViewLayout(v, w)
	d := &defaultToggleStyle
	box := layout.Children[0].Shape
	if box.Color != d.Color {
		t.Errorf("box color: got %v, want %v", box.Color, d.Color)
	}
	if box.ColorBorder != d.ColorBorder {
		t.Errorf("border color: got %v, want %v", box.ColorBorder, d.ColorBorder)
	}
}

func TestToggleOuterRowNoBorder(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_toggle_outer_row_no_border", OnClick: noop})
	layout := generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("outer row SizeBorder: got %v, want 0", layout.Shape.SizeBorder)
	}
}

func TestToggleInvisibleHidesWidget(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_toggle_invisible_hides_widget", OnClick: noop, Invisible: true})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled || !layout.Shape.OverDraw {
		t.Error("invisible toggle should be disabled+overdraw")
	}
}

func TestToggleUnselectedText(t *testing.T) {
	w := newTestWindow()
	v := Toggle(ToggleCfg{ID: "widget_test_test_toggle_unselected_text", OnClick: noop, Selected: false})
	layout := generateViewLayout(v, w)
	// Unselected with default TextUnselect=" " → transparent text color.
	box := layout.Children[0]
	txt := box.Children[0]
	if txt.Shape.TC.TextStyle.Color != ColorTransparent {
		t.Errorf("unselected text color: got %v, want transparent",
			txt.Shape.TC.TextStyle.Color)
	}
}

// --- Switch ---

func TestSwitchGeneratesLayout(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{
		Label:   "Dark mode",
		OnClick: noop,
		ID:      "f3",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleSwitchToggle {
		t.Fatalf("got role %d, want SwitchToggle", layout.Shape.A11YRole)
	}
	// Children: switch body + label text
	if len(layout.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(layout.Children))
	}
}

func TestSwitchSelectedState(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{ID: "widget_test_test_switch_selected_state", OnClick: noop, Selected: true})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YState != AccessStateChecked {
		t.Fatalf("got state %d, want Checked", layout.Shape.A11YState)
	}
}

func TestSwitchNoLabel(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{ID: "widget_test_test_switch_no_label", OnClick: noop})
	layout := generateViewLayout(v, w)
	// Only switch body child.
	if len(layout.Children) != 1 {
		t.Fatalf("got %d children, want 1", len(layout.Children))
	}
}

func TestSwitchDisabledSuppressesHover(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{ID: "widget_test_test_switch_disabled_suppresses_hover", OnClick: noop, Disabled: true})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled {
		t.Fatal("outer row should be disabled")
	}
	origColor := layout.Children[0].Shape.Color
	e := &Event{MouseButton: MouseInvalid}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	if layout.Children[0].Shape.Color != origColor {
		t.Error("hover should not change pill color when disabled")
	}
}

func TestSwitchNilOnClickSuppressesHover(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{ID: "widget_test_test_switch_nil_on_click_suppresses_hover"})
	layout := generateViewLayout(v, w)
	origColor := layout.Children[0].Shape.Color
	e := &Event{MouseButton: MouseInvalid}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	if layout.Children[0].Shape.Color != origColor {
		t.Error("hover should not change pill color without OnClick")
	}
}

func TestSwitchHoverChangesColor(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{ID: "widget_test_test_switch_hover_changes_color", OnClick: noop})
	layout := generateViewLayout(v, w)
	origColor := layout.Children[0].Shape.Color
	e := &Event{MouseButton: MouseInvalid}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	if layout.Children[0].Shape.Color == origColor {
		t.Error("hover should change pill color")
	}
}

func TestSwitchClickHoverChangesColor(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{ID: "widget_test_test_switch_click_hover_changes_color", OnClick: noop})
	layout := generateViewLayout(v, w)
	clickColor := defaultSwitchStyle.colorClick
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	got := layout.Children[0].Shape.Color
	if got != clickColor {
		t.Errorf("got %v, want click color %v", got, clickColor)
	}
}

func TestSwitchFocusBorder(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{OnClick: noop, ID: "f5", Label: "on"})
	layout := generateViewLayout(v, w)
	w.SetFocus("f5")
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	// Focus paints the pill only; the outer row (which spans the
	// label) stays untouched.
	if layout.Children[0].Shape.ColorBorder !=
		defaultSwitchStyle.ColorBorderFocus {
		t.Error("focused pill should have focus border color")
	}
	if layout.Shape.ColorBorder == defaultSwitchStyle.ColorBorderFocus {
		t.Error("outer row should not be highlighted")
	}
}

func TestSwitchInvisibleHidesWidget(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{ID: "widget_test_test_switch_invisible_hides_widget", OnClick: noop, Invisible: true})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled || !layout.Shape.OverDraw {
		t.Error("invisible switch should be disabled+overdraw")
	}
}

func TestSwitchDefaultStyles(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{ID: "widget_test_test_switch_default_styles", OnClick: noop})
	layout := generateViewLayout(v, w)
	d := &defaultSwitchStyle
	pill := layout.Children[0].Shape
	if pill.Color != d.Color {
		t.Errorf("pill color: got %v, want %v", pill.Color, d.Color)
	}
	if pill.ColorBorder != d.ColorBorder {
		t.Errorf("border color: got %v, want %v", pill.ColorBorder, d.ColorBorder)
	}
}

func TestSwitchThumbColor(t *testing.T) {
	w := newTestWindow()
	d := &defaultSwitchStyle

	off := Switch(SwitchCfg{ID: "widget_test_test_switch_thumb_color", OnClick: noop})
	lo := generateViewLayout(off, w)
	thumb := lo.Children[0].Children[0].Shape
	if thumb.Color != d.colorUnselect {
		t.Errorf("unselected thumb: got %v, want %v", thumb.Color, d.colorUnselect)
	}

	on := Switch(SwitchCfg{ID: "widget_test_test_switch_thumb_color_2", OnClick: noop, Selected: true})
	lo = generateViewLayout(on, w)
	thumb = lo.Children[0].Children[0].Shape
	if thumb.Color != d.ColorSelect {
		t.Errorf("selected thumb: got %v, want %v", thumb.Color, d.ColorSelect)
	}
}

func TestSwitchOuterRowNoBorder(t *testing.T) {
	w := newTestWindow()
	v := Switch(SwitchCfg{ID: "widget_test_test_switch_outer_row_no_border", OnClick: noop})
	layout := generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("outer row SizeBorder: got %v, want 0", layout.Shape.SizeBorder)
	}
}

// --- Select ---

func TestSelectGeneratesLayout(t *testing.T) {
	w := newTestWindow()
	v := Select(SelectCfg{
		ID:       "country",
		Options:  []string{"US", "UK", "DE"},
		OnSelect: func(_ []string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleComboBox {
		t.Fatalf("got role %d, want ComboBox", layout.Shape.A11YRole)
	}
	// Content: label wrapper + arrow
	if len(layout.Children) < 2 {
		t.Fatalf("got %d children, want >= 2", len(layout.Children))
	}
}

func TestSelectPlaceholder(t *testing.T) {
	w := newTestWindow()
	v := Select(SelectCfg{
		ID:          "sel",
		Placeholder: "Choose...",
		Options:     []string{"A", "B"},
		OnSelect:    func(_ []string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) == 0 {
		t.Fatal("no children")
	}
	// The label lives inside the field's clipping wrapper row.
	if txt := firstTextShape(&layout); txt == nil || txt.TC.Text != "Choose..." {
		t.Fatalf("placeholder not rendered")
	}
}

func TestSelectShowsSelected(t *testing.T) {
	w := newTestWindow()
	v := Select(SelectCfg{
		ID:       "sel",
		Selected: []string{"B"},
		Options:  []string{"A", "B", "C"},
		OnSelect: func(_ []string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) == 0 {
		t.Fatal("no children")
	}
	txt := firstTextShape(&layout)
	if txt == nil || txt.TC.Text != "B" {
		t.Fatalf("got %+v, want B", txt)
	}
}

// firstTextShape returns the first text shape in the tree, depth first.
// Widgets wrap their label in scaffolding rows, so the label is not
// always a direct child.
func firstTextShape(layout *Layout) *Shape {
	if layout.Shape != nil && layout.Shape.shapeType == shapeText &&
		layout.Shape.TC != nil {
		return layout.Shape
	}
	for i := range layout.Children {
		if s := firstTextShape(&layout.Children[i]); s != nil {
			return s
		}
	}
	return nil
}

// --- NumericInput ---

func TestNumericInputGeneratesLayout(t *testing.T) {
	w := newTestWindow()
	v := NumericInput(NumericInputCfg{
		ID:   "qty",
		Text: "42",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleTextField {
		t.Fatalf("got role %d, want TextField", layout.Shape.A11YRole)
	}
}

func TestNumericInputNoButtons(t *testing.T) {
	w := newTestWindow()
	v := NumericInput(NumericInputCfg{
		ID:      "qty",
		Text:    "10",
		StepCfg: NumericStepCfg{ShowButtons: false},
	})
	layout := generateViewLayout(v, w)
	// Should be a Column (Input view), not Row with step buttons.
	if layout.Shape.Axis != axisTopToBottom {
		t.Fatalf("expected column axis, got %d", layout.Shape.Axis)
	}
}

func TestNumericInputWithButtons(t *testing.T) {
	w := newTestWindow()
	v := NumericInput(NumericInputCfg{
		ID:      "qty",
		Text:    "10",
		StepCfg: NumericStepCfg{ShowButtons: true, Step: 1},
	})
	layout := generateViewLayout(v, w)
	// Row with field + step button column.
	if len(layout.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(layout.Children))
	}
}

// --- ListBox ---

func TestListBoxGeneratesLayout(t *testing.T) {
	w := newTestWindow()
	v := ListBox(ListBoxCfg{
		ID: "fruits",
		Data: []ListBoxOption{
			{ID: "a", Name: "Apple"},
			{ID: "b", Name: "Banana"},
			{ID: "c", Name: "Cherry"},
		},
		OnSelect: func(_ []string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleList {
		t.Fatalf("got role %d, want List", layout.Shape.A11YRole)
	}
	if got := len(listBoxRows(&layout)); got != 3 {
		t.Fatalf("got %d rows, want 3", got)
	}
}

func TestListBoxItemRole(t *testing.T) {
	w := newTestWindow()
	v := ListBox(ListBoxCfg{
		ID: "lb",
		Data: []ListBoxOption{
			{ID: "x", Name: "Item X"},
		},
		OnSelect: func(_ []string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) == 0 {
		t.Fatal("no children")
	}
	item := layout.Children[0]
	if item.Shape.A11YRole != AccessRoleListItem {
		t.Fatalf("got role %d, want ListItem", item.Shape.A11YRole)
	}
}

func TestListBoxSelectedState(t *testing.T) {
	w := newTestWindow()
	v := ListBox(ListBoxCfg{
		ID: "lb",
		Data: []ListBoxOption{
			{ID: "x", Name: "Item X"},
			{ID: "y", Name: "Item Y"},
		},
		SelectedIDs: []string{"y"},
		OnSelect:    func(_ []string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) < 2 {
		t.Fatal("too few children")
	}
	if layout.Children[1].Shape.A11YState != AccessStateSelected {
		t.Fatalf("item y not selected")
	}
	if layout.Children[0].Shape.A11YState != AccessStateNone {
		t.Fatalf("item x should be unselected")
	}
}

func TestListBoxSubheading(t *testing.T) {
	w := newTestWindow()
	v := ListBox(ListBoxCfg{
		ID: "lb",
		Data: []ListBoxOption{
			NewListBoxSubheading("h1", "Heading"),
			NewListBoxOption("a", "Alpha", "val"),
		},
		OnSelect: func(_ []string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	if got := len(listBoxRows(&layout)); got != 2 {
		t.Fatalf("got %d rows, want 2", got)
	}
}

// --- ListCore ---

func TestListCoreNavigate(t *testing.T) {
	if listCoreNavigate(KeyUp, 5) != listCoreMoveUp {
		t.Fatal("expected MoveUp")
	}
	if listCoreNavigate(KeyDown, 5) != listCoreMoveDown {
		t.Fatal("expected MoveDown")
	}
	if listCoreNavigate(KeyEnter, 5) != listCoreSelectItem {
		t.Fatal("expected SelectItem")
	}
	if listCoreNavigate(KeyEscape, 5) != listCoreDismiss {
		t.Fatal("expected Dismiss")
	}
	if listCoreNavigate(KeyUp, 0) != listCoreNone {
		t.Fatal("expected None for empty list")
	}
}

func TestListCoreApplyNav(t *testing.T) {
	next, changed := listCoreApplyNav(listCoreMoveDown, 0, 5)
	if !changed || next != 1 {
		t.Fatalf("got %d/%v, want 1/true", next, changed)
	}
	next, changed = listCoreApplyNav(listCoreMoveUp, 0, 5)
	if changed || next != 0 {
		t.Fatalf("got %d/%v, want 0/false", next, changed)
	}
	next, changed = listCoreApplyNav(listCoreLast, 0, 5)
	if !changed || next != 4 {
		t.Fatalf("got %d/%v, want 4/true", next, changed)
	}
	next, changed = listCoreApplyNav(listCoreFirst, 3, 5)
	if !changed || next != 0 {
		t.Fatalf("got %d/%v, want 0/true", next, changed)
	}
}

func TestListCoreFuzzyScore(t *testing.T) {
	if listCoreFuzzyScore("Hello World", "hw") != 5 {
		t.Fatalf("got %d, want 5", listCoreFuzzyScore("Hello World", "hw"))
	}
	if listCoreFuzzyScore("abc", "xyz") != -1 {
		t.Fatal("expected no match")
	}
	if listCoreFuzzyScore("test", "") != 0 {
		t.Fatal("empty query should score 0")
	}
}

func TestListCoreVisibleRange(t *testing.T) {
	first, last := listCoreVisibleRange(100, 20, 200, 0)
	if first != 0 {
		t.Fatalf("first: got %d, want 0", first)
	}
	if last > 14 { // ~10 visible + 2 buffer + 1
		t.Fatalf("last: got %d, want <= 14", last)
	}
}

func TestListBoxNextSelectedIDs(t *testing.T) {
	// Single select.
	got := listBoxNextSelectedIDs([]string{"a"}, "b", false)
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("single: got %v", got)
	}
	// Multi add.
	got = listBoxNextSelectedIDs([]string{"a"}, "b", true)
	if len(got) != 2 {
		t.Fatalf("multi add: got %v", got)
	}
	// Multi remove.
	got = listBoxNextSelectedIDs([]string{"a", "b"}, "a", true)
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("multi remove: got %v", got)
	}
}

func TestListBoxOnKeyDownHandled(t *testing.T) {
	w := newTestWindow()
	itemIDs := []string{"a", "b"}
	e := &Event{KeyCode: KeyDown}
	listBoxOnKeyDown("lb", itemIDs, false,
		func(_ []string, ctx EventCtx) {}, nil,
		"", 0, 0, nil, soundCues{}, e, w)
	if !e.IsHandled {
		t.Fatal("expected key navigation event to be handled")
	}

	e = &Event{KeyCode: KeyEnter}
	called := false
	listBoxOnKeyDown("lb", itemIDs, false,
		func(_ []string, ctx EventCtx) { called = true },
		nil, "", 0, 0, nil, soundCues{}, e, w)
	if !e.IsHandled {
		t.Fatal("expected key select event to be handled")
	}
	if !called {
		t.Fatal("expected select callback to run")
	}
}

func TestListBoxDataIndex(t *testing.T) {
	// Data: [sub, a, b, sub, c] → itemIDs=[a,b,c], indices=[1,2,4]
	indices := []int{1, 2, 4}
	if got := listBoxDataIndex(indices, 0); got != 1 {
		t.Errorf("idx 0 → %d, want 1", got)
	}
	if got := listBoxDataIndex(indices, 2); got != 4 {
		t.Errorf("idx 2 → %d, want 4", got)
	}
	// Out of range falls through.
	if got := listBoxDataIndex(indices, 5); got != 5 {
		t.Errorf("idx 5 → %d, want 5", got)
	}
	// Nil mapping returns idx unchanged.
	if got := listBoxDataIndex(nil, 3); got != 3 {
		t.Errorf("nil idx 3 → %d, want 3", got)
	}
}

func TestListBoxScrollWithSubheadings(t *testing.T) {
	w := &Window{}
	idScroll := "90"
	rowH := float32(26)
	listH := float32(187)

	// Data: [sub, a, b, c, d, e, f, g, sub, h, i, j]
	// itemIDs index 7 = "h" → data index 9 (after 2 subheadings).
	itemDataIndices := []int{1, 2, 3, 4, 5, 6, 7, 9, 10, 11}

	// Scroll to item at itemIDs index 7 using data index.
	dataIdx := listBoxDataIndex(itemDataIndices, 7)
	scrollEnsureVisible(idScroll, dataIdx, rowH, listH, w)

	sy, _ := w.scrollY().Get(idScroll)
	// Data index 9: bottom = 10*26 = 260 > 187 → scroll = -(260-187) = -73
	want := -(float32(10)*rowH - listH)
	if sy != want {
		t.Fatalf("scrollY = %f, want %f", sy, want)
	}
}

// --- Focusable-by-default (Phase 2 flip) ---

// Each in-scope control with an ID and no FocusDisabled exposes exactly
// one focus candidate, and FocusDisabled yields zero. Pins against
// duplicate tab stops.
//
// There is no longer a no-ID column: every control here requires an ID
// and panics without one. TestFocusWidgetsRequireID covers that.
func TestPhase2WidgetsFocusableByDefault(t *testing.T) {
	w := newTestWindow()
	cases := []struct {
		name     string
		withID   View
		disabled View
	}{
		{
			name:     "Toggle",
			withID:   Toggle(ToggleCfg{ID: "fc-t", OnClick: noop}),
			disabled: Toggle(ToggleCfg{ID: "fc-t", OnClick: noop, FocusDisabled: true}),
		},
		{
			name:     "Switch",
			withID:   Switch(SwitchCfg{ID: "fc-s", OnClick: noop}),
			disabled: Switch(SwitchCfg{ID: "fc-s", OnClick: noop, FocusDisabled: true}),
		},
		{
			name:     "Select",
			withID:   Select(SelectCfg{ID: "fc-sel", Options: []string{"a"}}),
			disabled: Select(SelectCfg{ID: "fc-sel", Options: []string{"a"}, FocusDisabled: true}),
		},
		{
			name:     "Slider",
			withID:   Slider(SliderCfg{ID: "fc-sl", Max: 10}),
			disabled: Slider(SliderCfg{ID: "fc-sl", Max: 10, FocusDisabled: true}),
		},
	}
	for _, tc := range cases {
		layout := generateViewLayout(tc.withID, w)
		if got := countFocusCandidates(&layout); got != 1 {
			t.Errorf("%s with ID: got %d focus candidates, want 1", tc.name, got)
		}
		layout = generateViewLayout(tc.disabled, w)
		if got := countFocusCandidates(&layout); got != 0 {
			t.Errorf("%s FocusDisabled: got %d focus candidates, want 0", tc.name, got)
		}
	}
}

// Every focusable-by-default factory rejects an empty ID, and accepts
// one when the caller opts the control out of focus entirely. The two
// halves are the runtime counterpart of the `gui:"required,focus"`
// tag: required, except for a decorative control.
//
// The build funcs omit the ID on purpose, so each carries the
// requiredid:ignore directive that suppresses the analyzer for one
// literal — without it the pass would flag the very configs this test
// exists to reject.
func TestFocusWidgetsRequireID(t *testing.T) {
	cases := []struct {
		name     string
		build    func()
		optedOut func()
	}{
		{"Button",
			func() { _ = Button(ButtonCfg{}) }, // requiredid:ignore
			func() { _ = Button(ButtonCfg{FocusDisabled: true}) }},
		{"Input",
			func() { _ = Input(InputCfg{}) }, // requiredid:ignore
			func() { _ = Input(InputCfg{FocusDisabled: true}) }},
		{"InputDate",
			func() { _ = InputDate(InputDateCfg{}) }, // requiredid:ignore
			func() { _ = InputDate(InputDateCfg{FocusDisabled: true}) }},
		{"NumericInput",
			func() { _ = NumericInput(NumericInputCfg{}) }, // requiredid:ignore
			func() { _ = NumericInput(NumericInputCfg{FocusDisabled: true}) }},
		{"RadioButtonGroup",
			func() { _ = RadioButtonGroupColumn(RadioButtonGroupCfg{}) },
			func() {
				_ = RadioButtonGroupColumn(RadioButtonGroupCfg{FocusDisabled: true})
			}},
		{"Radio",
			func() { _ = Radio(RadioCfg{}) }, // requiredid:ignore
			func() { _ = Radio(RadioCfg{FocusDisabled: true}) }},
		{"Select",
			func() { _ = Select(SelectCfg{Options: []string{"a"}}) }, // requiredid:ignore
			func() {
				_ = Select(SelectCfg{Options: []string{"a"}, FocusDisabled: true})
			}},
		{"Switch",
			func() { _ = Switch(SwitchCfg{}) }, // requiredid:ignore
			func() { _ = Switch(SwitchCfg{FocusDisabled: true}) }},
		{"Toggle",
			func() { _ = Toggle(ToggleCfg{}) }, // requiredid:ignore
			func() { _ = Toggle(ToggleCfg{FocusDisabled: true}) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertPanicsRequiringID(t, tc.name, tc.build)
			// FocusDisabled: the control never joins the tab order, so
			// it has no identity to name and must be accepted as is.
			tc.optedOut()
		})
	}
}

// assertPanicsRequiringID runs build and fails unless it panics with
// the RequireID message naming widget.
func assertPanicsRequiringID(t *testing.T, widget string, build func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s without an ID must panic", widget)
		}
		want := "gui: " + widget + " requires a non-empty Cfg.ID"
		if msg, ok := r.(string); !ok || msg != want {
			t.Fatalf("panic = %v, want %q", r, want)
		}
	}()
	build()
}
