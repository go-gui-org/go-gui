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
			// The dock had no golden coverage at all (issue #389). One
			// two-group split pins the tab strip, the separator, and the
			// panel background against a styling change.
			name: "dock_layout",
			build: func(_ *Window) View {
				return DockLayout(DockLayoutCfg{
					ID: "dock",
					Root: DockSplit("s1", DockSplitHorizontal, 0.5,
						DockPanelGroup("g1", []string{"a", "b"}, "a"),
						DockPanelGroup("g2", []string{"c"}, "c")),
					Panels: []DockPanelDef{
						{ID: "a", Label: "Alpha"},
						{ID: "b", Label: "Beta"},
						{ID: "c", Label: "Gamma"},
					},
				})
			},
		},
		{
			name: "input",
			build: func(_ *Window) View {
				return Input(InputCfg{ID: "in", Text: "typed value"})
			},
		},
		{
			// Focused input: the ring on the field, the focus border
			// color, and the caret. The ring is the shadow the theme's
			// FocusRing paints outside the field (visual-refresh § 5.4).
			name:    "input_focused",
			focusID: "in",
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
			// The reorderable path builds its rows through
			// listBoxReorderItemView, which had no focus
			// indication at all until the ring landed.
			name:    "listbox_reorder_focused",
			focusID: "lb",
			build: func(_ *Window) View {
				return ListBox(ListBoxCfg{
					ID:          "lb",
					Items:       []string{"one", "two", "three"},
					SelectedIDs: []string{"two"},
					Reorderable: true,
					OnReorder: func(string, string, EventCtx) {
					},
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
			// The regression pin for the labelledField sizing bug:
			// the wrapper Column hardcoded FitFit, so a FillFit
			// field inside a label stack shrink-wrapped its text
			// and no labelled field in any app could fill its row.
			// Both fields must span half the row here.
			//
			// MinWidth is stated on purpose, so this case pins the
			// width pass-through and not the field floor -- the
			// plain `input` and `select` cases pin that.
			name: "form_row_labelled",
			build: func(_ *Window) View {
				return Row(ContainerCfg{
					Sizing:  FillFit,
					Spacing: SomeF(SpacingSmall),
					Content: []View{
						Input(InputCfg{
							ID:       "fn",
							Label:    "First",
							Text:     "Mike",
							Sizing:   FillFit,
							MinWidth: 40,
						}),
						Select(SelectCfg{
							ID:       "role",
							Label:    "Role",
							Options:  []string{"admin", "user"},
							Selected: []string{"user"},
							Sizing:   FillFit,
							MinWidth: 40,
						}),
					},
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
			// A titled container with no explicit colors resolves the
			// group-box ink and the hairline border, so the box reads
			// on both polarities where the hairline wash alone does
			// not (visual-refresh §4.1 keeps the wash for filled
			// controls). The disabled case below pins the
			// explicit-color path.
			name: "container_title",
			build: func(_ *Window) View {
				return Column(ContainerCfg{
					Title: "Group",
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
			// Focused header: the ring that lands on the header row
			// when it joins the tab order (issue #345). The header is
			// scoped under the panel, hence the ep:head focus ID.
			name:    "expand_panel_focused",
			focusID: "ep:head",
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
			// Resting swatch: hairline outline, no ring. The focused
			// case is the contrast that proves the ring replaces the
			// outline on the color layer rather than adding a frame
			// around the box.
			name: "color_swatch",
			build: func(_ *Window) View {
				return ColorSwatch(ColorSwatchCfg{
					ID:    "sw",
					Color: RGBA(80, 120, 200, 128),
				})
			},
		},
		{
			name:    "color_swatch_focused",
			focusID: "sw",
			build: func(_ *Window) View {
				return ColorSwatch(ColorSwatchCfg{
					ID:        "sw",
					Color:     RGBA(80, 120, 200, 128),
					Focusable: true,
				})
			},
		},
		{
			// Focused table: the outer ring plus the active row's
			// hover tint (row 0 by default) are what keyboard
			// navigation looks like before any arrow key is pressed.
			name:    "table_focused",
			focusID: "tbl",
			build: func(_ *Window) View {
				return Table(TableCfg{
					ID:        "tbl",
					Focusable: true,
					Data: []TableRowCfg{
						{Cells: []TableCellCfg{
							{Value: "Name", HeadCell: true},
							{Value: "Size", HeadCell: true},
						}},
						{Cells: []TableCellCfg{
							{Value: "alpha"},
							{Value: "12"},
						}},
						{Cells: []TableCellCfg{
							{Value: "beta"},
							{Value: "9"},
						}},
					},
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
			// A variable-height virtualized list, scrolled into the
			// middle so both spacers and the row window are recorded.
			// This is the pixel-level guard on the spacer arithmetic:
			// a leading spacer sized from the wrong prefix moves every
			// row, and reading GenerateLayout would not show it —
			// spacer heights only become geometry after arrange.
			name: "virtual_list",
			build: func(w *Window) View {
				const id = "vl"
				rowH := func(i int, _ float32) float32 {
					if i%3 == 0 {
						return 44
					}
					return 22
				}
				// Fixed offset rather than a driven scroll: a golden
				// pins appearance, and an offset set here is the same
				// every run.
				w.scrollY().Set(id, -300)
				return VirtualList(VirtualListCfg{
					ID:         id,
					ItemCount:  400,
					Height:     160,
					Sizing:     FillFixed,
					OverscanPx: 20,
					ItemHeight: rowH,
					ItemView: func(i int, _ float32) View {
						return Column(ContainerCfg{
							ID:         ScopeIDN(id, "row", i),
							Height:     rowH(i, 0),
							Sizing:     FillFixed,
							SizeBorder: NoBorder,
							Content: []View{Text(TextCfg{
								Text: "row " + itoa(i),
							})},
						})
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
		{
			// A Fit-width wrap resolves against its nearest definite
			// ancestor (issue #379): this records wrapped rows at the
			// window width, not one unwrapped row at the content sum.
			// 3 chips of 80 + 2 gaps of 4 fit in the 320 window; a
			// regression to the unconstrained sum would record a single
			// row of 12 chips at 1004px.
			name: "wrap_fit",
			build: func(_ *Window) View {
				chips := make([]View, 12)
				for i := range chips {
					chips[i] = Column(ContainerCfg{
						ID:         "chip-" + itoa(i),
						Sizing:     FixedFixed,
						Width:      80,
						Height:     40,
						SizeBorder: NoBorder,
						Color:      RGBA(180, 190, 200, 255),
					})
				}
				return Wrap(ContainerCfg{
					ID:      "chips",
					Sizing:  FitFit,
					Spacing: SomeF(4),
					Content: chips,
				})
			},
		},
		{
			// The four button variants side by side (visual-refresh
			// §10): one Row carrying the hierarchy, so a diff on any
			// variant's fill, label color or geometry lands on this
			// case and nothing else. The row has no ID, so the buttons
			// stay at window scope — the harness focuses by effective
			// ID, and a scoped row would silently miss it. Labels are
			// short: the golden window is 320 wide, and the renderer
			// culls a text that starts past the right edge.
			name:  "button_variants",
			build: buildButtonVariants,
		},
		{
			// The same row focused on the primary: the ring over the
			// accent fill is a state no other case records.
			name:    "button_variants_focused",
			focusID: "bv_primary",
			build:   buildButtonVariants,
		},
		{
			// Canvas gradient fills (issue #398). Records both the
			// linear and the radial path, because the two tessellate
			// differently: the linear splits at stop isolines and the
			// radial by edge length. The golden pins the vertex count
			// each pass settles on and both ends of each ramp, so a
			// change to the subdivision heuristics shows as a diff
			// rather than as a screenshot nobody re-takes.
			name:  "canvas_gradient",
			build: buildCanvasGradient,
		},
		{
			// The concentric radial fast path: a ramp centered on the
			// circle it fills, which is emitted as a ring mesh rather
			// than a subdivided fan. Multi-stop and off-[0,1] on
			// purpose — the ring list is where the pad regions inside
			// the first stop and outside the last are decided, and a
			// hard stop (two stops at one position) is the case that
			// must stay a color jump and not a zero-area band.
			name:  "canvas_gradient_rings",
			build: buildCanvasGradientRings,
		},
		{
			// Caller-supplied per-vertex color (issue #400). The
			// counterpart to canvas_gradient: nothing here is
			// evaluated, so what the golden pins is that the caller's
			// geometry and colors reach the render command untouched
			// and in order, as one batch that never merges with the
			// flat fill drawn after it.
			name:  "canvas_vertex_colors",
			build: buildCanvasVertexColors,
		},
	}
}

// buildCanvasVertexColors draws a hand-colored two-triangle mesh, then
// a flat rect in the mesh's own mean color — the color a naive batch
// merge would fold the two together on. Colors are fixed literals for
// the same reason buildCanvasGradient's are.
func buildCanvasVertexColors(_ *Window) View {
	return DrawCanvas(DrawCanvasCfg{
		ID:      "canvas_vertex_colors",
		Sizing:  FixedFixed,
		Width:   120,
		Height:  60,
		Version: 1,
		OnDraw: func(dc *DrawContext) {
			red, green := RGB(255, 0, 0), RGB(0, 255, 0)
			blue, white := RGB(0, 0, 255), RGB(255, 255, 255)
			dc.FillTrianglesColors([]float32{
				0, 0, 60, 0, 60, 60,
				0, 0, 60, 60, 0, 60,
			}, []Color{
				red, green, blue,
				red, blue, white,
			})
			dc.FilledRect(70, 10, 40, 40, RGB(127, 85, 127))
		},
	})
}

// buildCanvasGradient draws one linear and one radial gradient fill.
// Colors are fixed literals rather than theme colors: this case exists
// to pin the gradient math, and a theme-derived color would make the
// dark and light recordings differ for a reason unrelated to it.
func buildCanvasGradient(_ *Window) View {
	return DrawCanvas(DrawCanvasCfg{
		ID:      "canvas_gradient",
		Sizing:  FixedFixed,
		Width:   120,
		Height:  60,
		Version: 1,
		OnDraw: func(dc *DrawContext) {
			dc.FilledRectGradient(0, 0, 60, 60, &CanvasGradient{
				Stops: []GradientStop{
					{Color: RGB(255, 0, 0), Pos: 0},
					{Color: RGB(0, 255, 0), Pos: 0.5},
					{Color: RGB(0, 0, 255), Pos: 1},
				},
			})
			dc.FilledCircleGradient(90, 30, 25, &CanvasGradient{
				Radial: true,
				Stops: []GradientStop{
					{Color: RGBA(255, 255, 255, 255), Pos: 0},
					{Color: RGBA(255, 255, 255, 0), Pos: 1},
				},
			})
		},
	})
}

// buildCanvasGradientRings exercises the concentric radial fill with a
// ramp the two-stop case cannot reach: flat regions at both ends, a
// hard stop in the middle, and stops that are neither sorted nor in
// range on the way in.
func buildCanvasGradientRings(_ *Window) View {
	return DrawCanvas(DrawCanvasCfg{
		ID:      "canvas_gradient_rings",
		Sizing:  FixedFixed,
		Width:   80,
		Height:  80,
		Version: 1,
		OnDraw: func(dc *DrawContext) {
			dc.FilledCircleGradient(40, 40, 35, &CanvasGradient{
				Radial: true,
				Stops: []GradientStop{
					{Color: RGB(0, 0, 255), Pos: 1.4},
					{Color: RGB(255, 0, 0), Pos: 0.2},
					{Color: RGB(0, 255, 0), Pos: 0.6},
					{Color: RGB(255, 255, 0), Pos: 0.6},
				},
			})
		},
	})
}

// buildButtonVariants is the shared build for the two variant golden
// cases; the focused variant only differs in the harness's focusID.
func buildButtonVariants(_ *Window) View {
	return Row(ContainerCfg{
		Sizing:  FitFit,
		Spacing: SomeF(8),
		Content: []View{
			TextButtonVariant("bv_sec", "Sec", ButtonSecondary,
				func(EventCtx) {}),
			TextButtonVariant("bv_primary", "Pri", ButtonPrimary,
				func(EventCtx) {}),
			TextButtonVariant("bv_ghost", "Ghost", ButtonGhost,
				func(EventCtx) {}),
			TextButtonVariant("bv_danger", "Danger", ButtonDanger,
				func(EventCtx) {}),
		},
	})
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
