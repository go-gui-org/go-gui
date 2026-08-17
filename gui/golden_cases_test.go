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

import (
	"testing"
	"time"
)

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
			// A descender-bearing placeholder, which is where Select
			// parts company with Badge and Button: those measure the
			// run and leave a descender alone, this one centres the
			// cap band regardless, so both spellings sit level
			// (issue #346). If this ever records the same Y as an
			// uncorrected shape, the widget has reverted to the
			// measured form.
			name: "select_placeholder_descender",
			build: func(_ *Window) View {
				return Select(SelectCfg{
					ID:          "sel",
					Options:     []string{"alpha", "beta"},
					Placeholder: "pick a language",
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
		// --- Disabled-text sweep (issue #341) ---
		//
		// One case per text-bearing widget that can be disabled.
		// Before the fix, every disabled text command here renders
		// at alpha 127 (#7f) on both themes — dimAlpha halving the
		// color instead of the theme's TextStyleDisabled (128 dark,
		// 149 light). The fix moves the dark recording to #80 and
		// the light recording to #95; a case whose text does not
		// move was never reaching dimAlpha and is not part of the
		// sweep.
		{
			name: "select_disabled",
			build: func(_ *Window) View {
				return Select(SelectCfg{
					ID:       "sel",
					Options:  []string{"alpha", "beta"},
					Selected: []string{"beta"},
					Disabled: true,
				})
			},
		},
		{
			name: "combobox_disabled",
			build: func(_ *Window) View {
				return Combobox(ComboboxCfg{
					ID:       "cb",
					Options:  []string{"alpha", "beta"},
					Value:    "alpha",
					Disabled: true,
				})
			},
		},
		{
			name: "listbox_disabled",
			build: func(_ *Window) View {
				return ListBox(ListBoxCfg{
					ID:          "lb",
					Items:       []string{"one", "two", "three"},
					SelectedIDs: []string{"two"},
					Disabled:    true,
				})
			},
		},
		{
			name: "numericinput_disabled",
			build: func(_ *Window) View {
				return NumericInput(NumericInputCfg{
					ID:       "num",
					Text:     "42.5",
					Disabled: true,
				})
			},
		},
		{
			name: "inputdate_disabled",
			build: func(_ *Window) View {
				return InputDate(InputDateCfg{
					ID:       "id",
					Date:     time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
					Disabled: true,
				})
			},
		},
		{
			name: "datepicker_disabled",
			build: func(_ *Window) View {
				return DatePicker(DatePickerCfg{
					ID:       "dp",
					Dates:    []time.Time{time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)},
					Disabled: true,
				})
			},
		},
		{
			name: "button_disabled",
			build: func(_ *Window) View {
				return Button(ButtonCfg{
					ID:       "btn",
					Disabled: true,
					Content:  []View{Text(TextCfg{Text: "Save"})},
				})
			},
		},
		{
			name: "boolean_labels_disabled",
			build: func(_ *Window) View {
				return Column(ContainerCfg{
					Content: []View{
						Switch(SwitchCfg{ID: "sw", Label: "Enabled", Disabled: true}),
						Toggle(ToggleCfg{ID: "tg", Label: "Enabled", Disabled: true}),
						Radio(RadioCfg{ID: "rd", Label: "Enabled", Disabled: true}),
					},
				})
			},
		},
		{
			name: "tree_disabled",
			build: func(_ *Window) View {
				return Tree(TreeCfg{
					ID:       "tr",
					Disabled: true,
					Nodes: []TreeNodeCfg{
						{ID: "a", Text: "Alpha"},
						{ID: "b", Text: "Beta"},
					},
				})
			},
		},
		{
			name: "menubar_disabled",
			build: func(w *Window) View {
				return Menubar(w, MenubarCfg{
					ID: "mb",
					Items: []MenuItemCfg{
						MenuItemText("f", "File"),
						MenuSubtitle("Grouped"),
					},
				})
			},
		},
		{
			// An open menu, which is where the choice of band shows:
			// items stack at a regular pitch, so a descending label and
			// a cap-only one must record the same offset from their own
			// row. The cap band gives that; measuring each run would
			// move "Paste" down and leave the descending "Copy" where
			// it was, skewing the pitch between them (issue #346).
			name:    "menu_open_descender",
			focusID: "mb",
			build: func(w *Window) View {
				// Menu selection lives in nsMenu keyed by the menubar's
				// ID; setting it is what opens the menu for one frame.
				StateMap[string, string](w, nsMenu, capModerate).
					Set("mb", "edit")
				return Menubar(w, MenubarCfg{
					ID: "mb",
					Items: []MenuItemCfg{
						MenuSubmenu("edit", "Edit", []MenuItemCfg{
							MenuItemText("copy", "Copy"),
							MenuItemText("paste", "Paste"),
						}),
					},
				})
			},
		},
		{
			// The group-box title is the one text path dimmed twice:
			// addGroupBoxTitle halves at generation and renderText
			// halves the stamp again. Before the fix the title
			// records at ~63, a full stop under every other widget.
			name: "container_title_disabled",
			build: func(_ *Window) View {
				return Column(ContainerCfg{
					Title:       "Group",
					ColorBorder: Hex(0x4a6b8a),
					Disabled:    true,
				})
			},
		},
		{
			name: "text_disabled",
			build: func(_ *Window) View {
				return Text(TextCfg{
					Text:     "quiet",
					Disabled: true,
				})
			},
		},
		// The optical-centring cases (issue #346). Both widgets render
		// digits the widget owns, so both take the correction, and both
		// exist to make the ~0.09em downward shift of the text command a
		// reviewable diff rather than something only a screenshot would
		// catch.
		//
		// TextMeasurer is nil here, so what these pin is the fallback
		// ratio path, not the per-face measurement: that the correction
		// happens and by how much, not what a given font measures.
		{
			name: "badge",
			build: func(_ *Window) View {
				return Badge(BadgeCfg{Label: "128"})
			},
		},
		{
			// Max forces the "+" form, the widest label a badge
			// produces and still descender-free.
			name: "badge_capped",
			build: func(_ *Window) View {
				return Badge(BadgeCfg{Label: "250", Max: 99})
			},
		},
		{
			// The counter-case: a label that paints below the baseline
			// keeps metric centring, because it already sits low. If
			// this one ever moves, the correction has stopped asking
			// what the run actually does.
			name: "badge_descender",
			build: func(_ *Window) View {
				return Badge(BadgeCfg{Label: "gypsy"})
			},
		},
		{
			// Enabled and disabled buttons must take the same
			// correction: buttonAmendLayout returns early for a
			// disabled button, so the label would otherwise sit a
			// pixel above its enabled neighbour. button_disabled is
			// the other half of this pair.
			name: "button",
			build: func(_ *Window) View {
				return Button(ButtonCfg{
					ID:      "btn",
					Content: []View{Text(TextCfg{Text: "Save"})},
					OnClick: func(EventCtx) {},
				})
			},
		},
		{
			// A descending label takes the same offset as a cap-only
			// one: a button is a control whose text is a label, and a
			// row of them must agree. This records level with `button`
			// (issue #346); measuring the run would leave this one
			// behind.
			name: "button_descender",
			build: func(_ *Window) View {
				return Button(ButtonCfg{
					ID:      "btn",
					Content: []View{Text(TextCfg{Text: "Apply gypsy"})},
					OnClick: func(EventCtx) {},
				})
			},
		},
		{
			// The two editable fields whose mask constrains the
			// alphabet, so they take the content-free correction while
			// the plain `input` case above must stay put. That pairing
			// is the rule: `input` moving is the regression to watch
			// for, not these two.
			name: "numeric_input",
			build: func(_ *Window) View {
				return NumericInput(NumericInputCfg{ID: "num", Text: "42.5"})
			},
		},
		{
			name: "input_date",
			build: func(_ *Window) View {
				return InputDate(InputDateCfg{
					ID:   "id",
					Date: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
				})
			},
		},
		{
			name: "progress_bar",
			build: func(_ *Window) View {
				return ProgressBar(ProgressBarCfg{
					ID:       "pb",
					Percent:  0.42,
					TextShow: true,
				})
			},
		},
		{
			// The widget itself is enabled; a disabled ancestor does
			// the stamping via layoutDisables. This is the case the
			// per-widget sweep could not reach — a widget themes its
			// own disabled state, but cannot know about an ancestor's
			// at generation time. The renderer's role read covers it.
			name: "disabled_ancestor",
			build: func(_ *Window) View {
				return Column(ContainerCfg{
					Disabled: true,
					Content: []View{
						Input(InputCfg{ID: "in", Text: "typed value"}),
					},
				})
			},
		},
		// --- ColorSet migration (issue #342) ---
		//
		// The eleven flat-Color* widgets gain a Colors ColorSet that
		// folds through applyTo into their flat fields. The migration
		// must not move a pixel: these four widgets had no recording
		// before, so the no-diff criterion could not see them. Each
		// case is recorded pre-migration behavior and must hold
		// across the refactor.
		{
			name: "slider",
			build: func(_ *Window) View {
				return Slider(SliderCfg{ID: "sl", Value: 40})
			},
		},
		{
			name: "expand_panel",
			build: func(_ *Window) View {
				return ExpandPanel(ExpandPanelCfg{
					ID:      "ep",
					Head:    Text(TextCfg{Text: "Details"}),
					Content: Text(TextCfg{Text: "content"}),
					Open:    true,
				})
			},
		},
		{
			name: "context_menu",
			build: func(w *Window) View {
				return ContextMenu(w, ContextMenuCfg{
					ID: "cm",
					Content: []View{
						Button(ButtonCfg{
							ID:      "cm_btn",
							Content: []View{Text(TextCfg{Text: "Target"})},
						}),
					},
					Items: []MenuItemCfg{
						MenuItemText("open", "Open"),
						MenuItemText("save", "Save"),
					},
				})
			},
		},
		{
			// The resting menubar: only menubar_disabled exists today,
			// which cannot see a change to the resting colors.
			name: "menubar",
			build: func(w *Window) View {
				return Menubar(w, MenubarCfg{
					ID: "mb",
					Items: []MenuItemCfg{
						MenuItemText("f", "File"),
						MenuItemText("e", "Edit"),
					},
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
