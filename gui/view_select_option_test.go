package gui

import "testing"

// Label/value options for Select (issue #809). The field shows the
// Label; Selected and OnSelect carry the Value.

func selectLangOptions() []SelectOption {
	return []SelectOption{
		NewSelectOption("Go", "go"),
		NewSelectOption("Rust", "rust"),
		NewSelectOption("TypeScript (Node.js)", "typescript_node"),
	}
}

// selectRenderedFieldText renders a Select and returns the text of its closed
// field label.
func selectRenderedFieldText(t *testing.T, cfg SelectCfg) string {
	t.Helper()
	w := &Window{}
	layout := Select(cfg).GenerateLayout(w)
	txt := firstTextShape(&layout)
	if txt == nil {
		t.Fatal("expected a text shape in the field")
	}
	return txt.TC.Text
}

func TestSelectClosedFieldShowsOptionLabel(t *testing.T) {
	got := selectRenderedFieldText(t, SelectCfg{
		ID:       "lang",
		Selected: []string{"typescript_node"},
		Options:  selectLangOptions(),
	})
	if got != "TypeScript (Node.js)" {
		t.Errorf("field text = %q, want the option label", got)
	}
}

func TestSelectMultipleJoinsOptionLabels(t *testing.T) {
	got := selectRenderedFieldText(t, SelectCfg{
		ID:             "lang",
		Selected:       []string{"go", "rust"},
		Options:        selectLangOptions(),
		SelectMultiple: true,
	})
	if got != "Go, Rust" {
		t.Errorf("field text = %q, want %q", got, "Go, Rust")
	}
}

// A selected value with no matching option shows as written. The data
// grid relies on this: its cell value need not be in the option list.
func TestSelectUnknownSelectedValueShowsRaw(t *testing.T) {
	got := selectRenderedFieldText(t, SelectCfg{
		ID:       "lang",
		Selected: []string{"zig"},
		Options:  selectLangOptions(),
	})
	if got != "zig" {
		t.Errorf("field text = %q, want the raw value %q", got, "zig")
	}
}

func TestSelectOptionRowShowsLabelAndEmitsValue(t *testing.T) {
	var selected []string
	cfg := &SelectCfg{
		ID:      "lang",
		Options: selectLangOptions(),
		OnSelect: func(s []string, _ EventCtx) {
			selected = s
		},
	}
	applySelectDefaults(cfg)
	w := &Window{}
	layout := generateViewLayout(
		selectOptionView(cfg, cfg.ID, cfg.Options[2], 2, false), w)
	label := layout.Children[0].Children[1].Shape.TC
	if label == nil || label.Text != "TypeScript (Node.js)" {
		t.Fatalf("option row text = %v, want the label", label)
	}
	layout.Shape.events.OnClick(
		EventCtx{nil, &Event{MouseButton: MouseLeft}, w})
	if len(selected) != 1 || selected[0] != "typescript_node" {
		t.Errorf("OnSelect got %v, want [typescript_node]", selected)
	}
}

// The check mark follows the value. An option whose Label equals a
// selected value, but whose Value does not, stays unchecked.
func TestSelectCheckMarkKeysOnValue(t *testing.T) {
	cfg := &SelectCfg{
		ID:       "lang",
		Selected: []string{"go"},
	}
	applySelectDefaults(cfg)
	w := &Window{}
	checkColor := func(opt SelectOption) Color {
		layout := generateViewLayout(
			selectOptionView(cfg, cfg.ID, opt, 0, false), w)
		return layout.Children[0].Children[0].Shape.TC.TextStyle.Color
	}
	if c := checkColor(NewSelectOption("Go", "go")); c == ColorTransparent {
		t.Error("option with the selected value is not checked")
	}
	if c := checkColor(NewSelectOption("go", "golang")); c != ColorTransparent {
		t.Error("option whose label matches the value is checked")
	}
}

func TestSelectMultipleClickTogglesValue(t *testing.T) {
	var selected []string
	cfg := &SelectCfg{
		ID:             "lang",
		Selected:       []string{"go"},
		Options:        selectLangOptions(),
		SelectMultiple: true,
		OnSelect: func(s []string, _ EventCtx) {
			selected = s
		},
	}
	applySelectDefaults(cfg)
	w := &Window{}
	click := func(i int) {
		layout := generateViewLayout(
			selectOptionView(cfg, cfg.ID, cfg.Options[i], i, false), w)
		layout.Shape.events.OnClick(
			EventCtx{nil, &Event{MouseButton: MouseLeft}, w})
	}
	click(1)
	if len(selected) != 2 || selected[0] != "go" || selected[1] != "rust" {
		t.Errorf("after adding Rust got %v, want [go rust]", selected)
	}
	click(0)
	if len(selected) != 0 {
		t.Errorf("after removing Go got %v, want []", selected)
	}
}

func TestSelectKeyboardEnterEmitsValue(t *testing.T) {
	w := &Window{}
	var selected []string
	cfg := SelectCfg{
		ID:      "lang",
		Options: selectLangOptions(),
		OnSelect: func(s []string, _ EventCtx) {
			selected = s
		},
	}
	applySelectDefaults(&cfg)
	idScroll := ScopeID(cfg.ID, "dropdown")
	selectOnKeyDown(&cfg, cfg.ID, idScroll, &Event{KeyCode: KeySpace}, w)
	selectOnKeyDown(&cfg, cfg.ID, idScroll, &Event{KeyCode: KeyDown}, w)
	selectOnKeyDown(&cfg, cfg.ID, idScroll, &Event{KeyCode: KeyEnter}, w)
	if len(selected) != 1 || selected[0] != "rust" {
		t.Errorf("Enter emitted %v, want [rust]", selected)
	}
}

// Opening the dropdown highlights the row of the selected value.
func TestSelectInitialHighlightMatchesValue(t *testing.T) {
	got := selectInitialHighlight(&SelectCfg{
		Selected: []string{"typescript_node"},
		Options:  selectLangOptions(),
	})
	if got != 2 {
		t.Errorf("initial highlight = %d, want 2", got)
	}
}

// Items is the string shorthand: each string is both label and value,
// and the "---" prefix still marks a subheading.
func TestSelectItemsReadAsOptions(t *testing.T) {
	cfg := SelectCfg{
		ID:    "s",
		Items: []string{"A", "---Section", "B"},
	}
	if n := cfg.optionCount(); n != 3 {
		t.Fatalf("options = %d, want 3", n)
	}
	if o := cfg.optionAt(0); o.Label != "A" || o.Value != "A" {
		t.Errorf("option 0 = %+v, want Label=Value=A", o)
	}
	if o := cfg.optionAt(1); !o.isSubheading || o.Label != "Section" {
		t.Errorf("option 1 = %+v, want subheading Section", o)
	}
	if cfg.optionAt(2).isSubheading {
		t.Error("option 2 marked as subheading")
	}
}

// Items wins over Options, the same rule RadioButtonGroup follows.
func TestSelectItemsWinOverOptions(t *testing.T) {
	cfg := SelectCfg{
		ID:      "s",
		Items:   []string{"X"},
		Options: selectLangOptions(),
	}
	if cfg.optionCount() != 1 || cfg.optionAt(0).Value != "X" {
		t.Errorf("got %d options, first %+v; want the Items list",
			cfg.optionCount(), cfg.optionAt(0))
	}
}

// Items has no length cap: the old Options []string path had none, so
// a migrated long list must keep every row (Combobox agrees).
func TestSelectItemsNotCapped(t *testing.T) {
	items := make([]string, maxDataConvLen+1)
	cfg := SelectCfg{ID: "s", Items: items}
	if n := cfg.optionCount(); n != len(items) {
		t.Errorf("options = %d, want %d", n, len(items))
	}
}

// Items is read in place. A Select built from Items must allocate no
// more per frame than the same list given as Options; converting the
// strings to a []SelectOption each frame was one extra allocation.
func TestSelectItemsAllocsMatchOptions(t *testing.T) {
	const n = 200
	items := make([]string, n)
	opts := make([]SelectOption, n)
	for i := range n {
		items[i] = string(rune('a' + i%26))
		opts[i] = NewSelectOption(items[i], items[i])
	}
	w := &Window{}
	measure := func(cfg SelectCfg) float64 {
		return testing.AllocsPerRun(50, func() {
			_ = Select(cfg).GenerateLayout(w)
		})
	}
	fromItems := measure(SelectCfg{ID: "s", Items: items, Selected: []string{"c"}})
	fromOpts := measure(SelectCfg{ID: "s", Options: opts, Selected: []string{"c"}})
	if fromItems > fromOpts {
		t.Errorf("Items path allocates %v per frame, Options path %v",
			fromItems, fromOpts)
	}
}

// An option may have an empty value ("None"). Selecting it shows its
// label, not the placeholder, the same as Combobox.
func TestSelectEmptyValueOptionShowsLabel(t *testing.T) {
	got := selectRenderedFieldText(t, SelectCfg{
		ID:          "s",
		Placeholder: "Pick",
		Selected:    []string{""},
		Options: []SelectOption{
			NewSelectOption("None", ""),
			NewSelectOption("Go", "go"),
		},
	})
	if got != "None" {
		t.Errorf("field text = %q, want None", got)
	}
}

// An empty value no option holds is no selection: the placeholder shows.
func TestSelectUnmatchedEmptyValueShowsPlaceholder(t *testing.T) {
	got := selectRenderedFieldText(t, SelectCfg{
		ID:          "s",
		Placeholder: "Pick",
		Selected:    []string{""},
		Items:       []string{"A"},
	})
	if got != "Pick" {
		t.Errorf("field text = %q, want the placeholder", got)
	}
}

// In multi-select, an unmatched empty value first in the list must not
// hide the other labels behind the placeholder.
func TestSelectMultipleSkipsUnmatchedEmptyValue(t *testing.T) {
	got := selectRenderedFieldText(t, SelectCfg{
		ID:             "lang",
		Placeholder:    "Pick",
		Selected:       []string{"", "go"},
		Options:        selectLangOptions(),
		SelectMultiple: true,
	})
	if got != "Go" {
		t.Errorf("field text = %q, want Go", got)
	}
}

// A typed option whose label starts with "---" is a normal option. Only
// NewSelectSubheading makes a subheading on the typed path.
func TestSelectTypedDashLabelIsNotSubheading(t *testing.T) {
	cfg := SelectCfg{
		ID:      "s",
		Options: []SelectOption{NewSelectOption("---", "none")},
	}
	if cfg.optionAt(0).isSubheading {
		t.Error("typed option with a dash label became a subheading")
	}
}

func TestSelectTypedSubheadingSkippedByKeyboard(t *testing.T) {
	w := &Window{}
	cfg := SelectCfg{
		ID: "s",
		Options: []SelectOption{
			NewSelectOption("A", "a"),
			NewSelectSubheading("Section"),
			NewSelectOption("B", "b"),
		},
		OnSelect: func([]string, EventCtx) {},
	}
	applySelectDefaults(&cfg)
	idScroll := ScopeID(cfg.ID, "dropdown")
	selectOnKeyDown(&cfg, cfg.ID, idScroll, &Event{KeyCode: KeySpace}, w)
	selectOnKeyDown(&cfg, cfg.ID, idScroll, &Event{KeyCode: KeyDown}, w)
	sh := StateMap[string, int](w, nsSelectHL, capModerate)
	if idx, _ := sh.Get("s"); idx != 2 {
		t.Errorf("highlight = %d, want 2 (subheading skipped)", idx)
	}
}

// A bare "---" Items entry is a subheading that shows "---", as before
// #809: removing the prefix would leave an empty header.
func TestSelectItemsBareDashesSubheadingKeepsText(t *testing.T) {
	cfg := SelectCfg{ID: "s", Items: []string{"---"}}
	if o := cfg.optionAt(0); !o.isSubheading || o.Label != "---" {
		t.Errorf("option 0 = %+v, want subheading labelled ---", o)
	}
}
