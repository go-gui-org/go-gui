package gui

// The recorded widget set for TestGolden.
//
// These are the widgets issue #335 touches. Each case is deliberately
// small — one widget, states pinned by config rather than by driving
// events — so a golden diff points at a styling change and nothing
// else.
//
// Cases cover the states that carry the visual roles under audit:
// placeholder text, disabled text, secondary text, and focus/hover
// borders. A widget recorded only in its resting state would not
// catch a change to the role it uses.

import "testing"

// goldenHSLA is one fixed color for every color-widget case, so a
// diff never reflects a different input color.
var goldenHSLA = HSLA{H: 210, S: 0.6, L: 0.5, A: 1}

func goldenCases() []goldenCase {
	return []goldenCase{
		{
			name: "input",
			build: func(_ *Window) View {
				return Input(InputCfg{ID: "in", Text: "typed value"})
			},
		},
		{
			// Placeholder is the only path that reaches the
			// placeholder text role.
			name: "input_placeholder",
			build: func(_ *Window) View {
				return Input(InputCfg{
					ID:          "in",
					Placeholder: "enter a value",
				})
			},
		},
		{
			name: "input_disabled",
			build: func(_ *Window) View {
				return Input(InputCfg{
					ID:       "in",
					Text:     "typed value",
					Disabled: true,
				})
			},
		},
		{
			name: "select",
			build: func(_ *Window) View {
				return Select(SelectCfg{
					ID:       "sel",
					Options:  []string{"alpha", "beta", "gamma"},
					Selected: []string{"beta"},
				})
			},
		},
		{
			// Records Select's placeholder beside Input's, which is
			// how the two insets in audit §7 become visible.
			name: "select_placeholder",
			build: func(_ *Window) View {
				return Select(SelectCfg{
					ID:          "sel",
					Options:     []string{"alpha", "beta"},
					Placeholder: "choose one",
				})
			},
		},
		{
			name: "combobox",
			build: func(_ *Window) View {
				return Combobox(ComboboxCfg{
					ID:      "cb",
					Options: []string{"alpha", "beta"},
					Value:   "alpha",
				})
			},
		},
		{
			name: "listbox",
			build: func(_ *Window) View {
				return ListBox(ListBoxCfg{
					ID:          "lb",
					Items:       []string{"one", "two", "three"},
					SelectedIDs: []string{"two"},
				})
			},
		},
		{
			// Focused, so the ring ListBox previously lacked is
			// actually in the recording (issue #335, audit s6).
			name:    "listbox_focused",
			focusID: "lb",
			build: func(_ *Window) View {
				return ListBox(ListBoxCfg{
					ID:          "lb",
					Items:       []string{"one", "two", "three"},
					SelectedIDs: []string{"two"},
				})
			},
		},
		{
			name:    "select_focused",
			focusID: "sel",
			build: func(_ *Window) View {
				return Select(SelectCfg{
					ID:       "sel",
					Options:  []string{"alpha", "beta"},
					Selected: []string{"beta"},
				})
			},
		},
		{
			name: "table",
			build: func(_ *Window) View {
				return Table(TableCfg{
					ID: "tbl",
					Data: []TableRowCfg{
						{Cells: []TableCellCfg{
							{Value: "Name", HeadCell: true},
							{Value: "Size", HeadCell: true},
						}},
						{Cells: []TableCellCfg{
							{Value: "alpha"},
							{Value: "12"},
						}},
					},
				})
			},
		},
		{
			// Carries a disabled tab, which is one of the two
			// themed textStyleDisabled sites (audit §1).
			name: "tab_control",
			build: func(_ *Window) View {
				return TabControl(TabControlCfg{
					ID:       "tabs",
					Selected: "a",
					Items: []TabItemCfg{
						{ID: "a", Label: "First"},
						{ID: "b", Label: "Second"},
						{ID: "c", Label: "Third", Disabled: true},
					},
				})
			},
		},
		{
			// Carries both the disabled crumb and the separator,
			// the other two themed alphas.
			name: "breadcrumb",
			build: func(_ *Window) View {
				return Breadcrumb(BreadcrumbCfg{
					ID:       "bc",
					Selected: "c",
					Items: []BreadcrumbItemCfg{
						{ID: "a", Label: "Home"},
						{ID: "b", Label: "Docs", Disabled: true},
						{ID: "c", Label: "Page"},
					},
				})
			},
		},
		{
			// The new Label field. A labelled Input must differ from
			// the unlabelled one by exactly a label Text plus the
			// stack it sits in -- the field itself must not move
			// relative to its own box (issue #335, audit section 3).
			name: "input_labelled",
			build: func(_ *Window) View {
				return Input(InputCfg{
					ID:    "in",
					Text:  "typed value",
					Label: "Full name",
				})
			},
		},
		{
			// Same label convention on a different control: the two
			// recordings are what prove there is one convention and
			// not two implementations.
			name: "select_labelled",
			build: func(_ *Window) View {
				return Select(SelectCfg{
					ID:       "sel",
					Options:  []string{"alpha", "beta"},
					Selected: []string{"beta"},
					Label:    "Variant",
				})
			},
		},
		{
			// The three boolean controls each spelled their trailing
			// label differently; only Radio left a gap. Recording all
			// three is what makes "one convention" checkable.
			name: "boolean_labels",
			build: func(_ *Window) View {
				return Column(ContainerCfg{
					Content: []View{
						Switch(SwitchCfg{ID: "sw", Label: "Enabled"}),
						Toggle(ToggleCfg{ID: "tg", Label: "Enabled"}),
						Radio(RadioCfg{ID: "rd", Label: "Enabled"}),
					},
				})
			},
		},
		{
			name: "color_fields",
			build: func(_ *Window) View {
				return ColorFields(ColorFieldsCfg{
					ID:    "cf",
					Value: goldenHSLA,
				})
			},
		},
		{
			// The HSL variant is where the shrunk, dimmed channel
			// labels appear — the case that prompted #335.
			name: "color_fields_hsl",
			build: func(_ *Window) View {
				return ColorFields(ColorFieldsCfg{
					ID:      "cf",
					Value:   goldenHSLA,
					ShowHSL: true,
				})
			},
		},
		{
			name: "color_plane",
			build: func(_ *Window) View {
				return ColorPlane(ColorPlaneCfg{
					ID:    "cp",
					Value: goldenHSLA,
				})
			},
		},
		{
			// The plane had no focus indication at all. The ring is
			// painted from AmendLayout rather than reserved as a
			// border, so this golden must show a stroke appearing
			// while the gradient image stays at the same xy as the
			// resting case — a shifted image means the ring went
			// back to insetting content (issue #335).
			name:    "color_plane_focused",
			focusID: "cp",
			build: func(_ *Window) View {
				return ColorPlane(ColorPlaneCfg{
					ID:    "cp",
					Value: goldenHSLA,
				})
			},
		},
	}
}

// TestGoldenSerializerStable guards the harness itself: a golden is
// only evidence if serializing the same commands twice is identical,
// and if unset colors stay distinguishable from transparent black.
func TestGoldenSerializerStable(t *testing.T) {
	cmds := []RenderCmd{
		{Kind: RenderRect, X: 1, Y: 2, W: 3, H: 4,
			Color: RGBA(1, 2, 3, 4), Fill: true},
		{Kind: RenderText, Text: "x", FontSize: 16},
		{Kind: RenderRect}, // unset color
	}
	first := serializeCmds(cmds)
	if second := serializeCmds(cmds); first != second {
		t.Errorf("serializer not stable:\n%s\n%s", first, second)
	}
	if got := colorStr(Color{}); got != "unset" {
		t.Errorf("zero Color: got %q, want %q", got, "unset")
	}
	if colorStr(ColorTransparent) == "unset" {
		t.Error("ColorTransparent must not serialize as unset")
	}
}

// TestGoldenNegativeZero pins the normalization that keeps a -0.00
// from diffing against 0.00 on one platform and not another.
func TestGoldenNegativeZero(t *testing.T) {
	var neg float32 = -0
	if got := f2(neg); got != "0.00" {
		t.Errorf("f2(-0): got %q, want %q", got, "0.00")
	}
}
