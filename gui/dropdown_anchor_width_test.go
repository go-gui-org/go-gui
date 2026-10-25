package gui

import "testing"

// dropdownWidthCase opens one dropdown widget inside a fixed-width
// column and reports the arranged field and dropdown layers.
type dropdownWidthCase struct {
	name string
	// open puts the widget's dropdown in the open state for the next
	// generation.
	open func(w *Window)
	// view builds the widget with the given options.
	view func(items []string) View
	id   string
}

func dropdownWidthCases() []dropdownWidthCase {
	return []dropdownWidthCase{
		{
			name: "select",
			id:   "sel",
			open: func(w *Window) {
				StateMap[string, bool](w, nsSelect, capModerate).Set("sel", true)
			},
			view: func(items []string) View {
				return Select(SelectCfg{
					ID:       "sel",
					Items:    items,
					Sizing:   FillFit,
					OnSelect: func([]string, EventCtx) {},
				})
			},
		},
		{
			name: "combobox",
			id:   "cb",
			open: func(w *Window) { comboboxOpen("cb", "", w) },
			view: func(items []string) View {
				return Combobox(ComboboxCfg{
					ID:       "cb",
					Items:    items,
					Sizing:   FillFit,
					OnSelect: func(string, EventCtx) {},
				})
			},
		},
	}
}

// arrangeDropdown arranges c inside a column fixed at width and
// returns the field and its open dropdown.
func arrangeDropdown(
	t *testing.T, c dropdownWidthCase, width float32, items []string,
) (field, dropdown *Layout) {
	t.Helper()
	w := &Window{windowWidth: 800, windowHeight: 600}
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	c.open(w)
	root := Column(ContainerCfg{
		Sizing:     FixedFit,
		Width:      width,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    []View{c.view(items)},
	})
	layout := generateViewLayout(root, w)
	layers := layoutArrange(&layout, w)
	dropID := ScopeID(c.id, "dropdown")
	for i := range layers {
		if f := findShapeByID(&layers[i], c.id); f != nil && field == nil {
			field = f
		}
		if layers[i].Shape.idKey() == dropID {
			dropdown = &layers[i]
		}
	}
	if field == nil || dropdown == nil {
		t.Fatalf("field found %v, dropdown found %v", field != nil, dropdown != nil)
	}
	return field, dropdown
}

// TestDropdownMatchesWideField: a Fill field wider than its options
// opens a dropdown at least as wide as the field, and the option rows
// fill that width, so the hover wash spans the list (#1155).
func TestDropdownMatchesWideField(t *testing.T) {
	for _, c := range dropdownWidthCases() {
		t.Run(c.name, func(t *testing.T) {
			field, dropdown := arrangeDropdown(t, c, 640,
				[]string{"BANK", "BPU", "PDAM"})
			if field.Shape.Width < 600 {
				t.Fatalf("field width = %v, want the column's 640", field.Shape.Width)
			}
			if dropdown.Shape.Width != field.Shape.Width {
				t.Errorf("dropdown width = %v, want field width %v",
					dropdown.Shape.Width, field.Shape.Width)
			}
			inner := dropdown.Shape.Width - dropdown.Shape.paddingWidth()
			for i := range dropdown.Children {
				row := dropdown.Children[i].Shape
				if skipLayoutChild(row) {
					continue
				}
				if row.Width != inner {
					t.Errorf("row %d width = %v, want %v", i, row.Width, inner)
				}
			}
		})
	}
}

// TestDropdownKeepsContentWidth: options wider than the field keep
// their own width; the field width is a floor, not a fixed size.
func TestDropdownKeepsContentWidth(t *testing.T) {
	long := "an option label much wider than the field"
	for _, c := range dropdownWidthCases() {
		t.Run(c.name, func(t *testing.T) {
			field, dropdown := arrangeDropdown(t, c, 120,
				[]string{"A", long})
			if dropdown.Shape.Width <= field.Shape.Width {
				t.Errorf("dropdown width = %v, want wider than field %v",
					dropdown.Shape.Width, field.Shape.Width)
			}
			if labelW := float32(len(long)) * 8; dropdown.Shape.Width < labelW {
				t.Errorf("dropdown width = %v, want at least the label's %v",
					dropdown.Shape.Width, labelW)
			}
		})
	}
}

// TestDropdownCappedByMaxWidth: MaxWidth still bounds the list when
// its options are wider than the cap; neither the anchor-width floor
// nor the Combobox widest-label floor pushes past it (#1155).
func TestDropdownCappedByMaxWidth(t *testing.T) {
	const capW = 200
	items := []string{"A", "an option label much wider than the cap"}
	cases := []struct {
		name, id string
		open     func(w *Window)
		view     View
	}{
		{
			name: "select", id: "sel",
			open: func(w *Window) {
				StateMap[string, bool](w, nsSelect, capModerate).Set("sel", true)
			},
			view: Select(SelectCfg{
				ID: "sel", Items: items, Sizing: FillFit, MaxWidth: capW,
				OnSelect: func([]string, EventCtx) {},
			}),
		},
		{
			name: "combobox", id: "cb",
			open: func(w *Window) { comboboxOpen("cb", "", w) },
			view: Combobox(ComboboxCfg{
				ID: "cb", Items: items, Sizing: FillFit, MaxWidth: capW,
				OnSelect: func(string, EventCtx) {},
			}),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := &Window{windowWidth: 800, windowHeight: 600}
			w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
			c.open(w)
			layout := generateViewLayout(Column(ContainerCfg{
				Sizing:     FixedFit,
				Width:      640,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Content:    []View{c.view},
			}), w)
			layers := layoutArrange(&layout, w)
			dropID := ScopeID(c.id, "dropdown")
			for i := range layers {
				if layers[i].Shape.idKey() != dropID {
					continue
				}
				if got := layers[i].Shape.Width; got > capW {
					t.Errorf("dropdown width = %v, want at most MaxWidth %v",
						got, capW)
				}
				return
			}
			t.Fatal("dropdown layer not found")
		})
	}
}

// TestComboboxDropdownWidthStableOnScroll: the Combobox builds only
// the rows in view, so a width taken from them would grow when the
// list scrolls to a long item. The widest label of the whole list sets
// the floor, so the width is the same at the top and at the bottom
// (#1155 review).
func TestComboboxDropdownWidthStableOnScroll(t *testing.T) {
	long := "an option label much wider than the field"
	items := make([]string, 0, 60)
	for i := range 59 {
		items = append(items, "item "+string(rune('a'+i%26)))
	}
	items = append(items, long)
	widthAt := func(scrollY float32) float32 {
		w := &Window{windowWidth: 800, windowHeight: 600}
		w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
		comboboxOpen("cb", "", w)
		w.scrollY().Set(ScopeID("cb", "dropdown"), scrollY)
		layout := generateViewLayout(Column(ContainerCfg{
			Sizing:     FixedFit,
			Width:      120,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Content: []View{Combobox(ComboboxCfg{
				ID:       "cb",
				Items:    items,
				Sizing:   FillFit,
				OnSelect: func(string, EventCtx) {},
			})},
		}), w)
		layers := layoutArrange(&layout, w)
		for i := range layers {
			if layers[i].Shape.idKey() == ScopeID("cb", "dropdown") {
				return layers[i].Shape.Width
			}
		}
		t.Fatal("dropdown layer not found")
		return 0
	}
	top, bottom := widthAt(0), widthAt(-100000)
	if top != bottom {
		t.Errorf("dropdown width at top = %v, at bottom = %v, want equal",
			top, bottom)
	}
	if labelW := float32(len(long)) * 8; top < labelW {
		t.Errorf("dropdown width = %v, want at least the long label's %v",
			top, labelW)
	}
}
