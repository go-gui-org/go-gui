package gui

import "testing"

// Label/value options for Combobox (issue #809). The field and the
// dropdown show the Label; Value and OnSelect carry the Value.

// comboboxCache renders a Combobox and returns its items cache.
func comboboxCache(t *testing.T, w *Window, cfg ComboboxCfg) *comboboxItemsCache {
	t.Helper()
	_ = generateViewLayout(Combobox(cfg), w)
	cm := StateMapRead[string, *comboboxItemsCache](w, nsComboboxItems)
	if cm == nil {
		t.Fatal("expected combobox items cache map")
	}
	cache, ok := cm.Get(cfg.ID)
	if !ok || cache == nil {
		t.Fatal("expected combobox cache entry")
	}
	return cache
}

func TestComboboxClosedFieldShowsOptionLabel(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Combobox(ComboboxCfg{
		ID:      "lang",
		Value:   "typescript_node",
		Options: selectLangOptions(),
	}), w)
	txt := firstTextShape(&layout)
	if txt == nil || txt.TC.Text != "TypeScript (Node.js)" {
		t.Errorf("field text = %v, want the option label", txt)
	}
}

// A value with no matching option shows as written.
func TestComboboxUnknownValueShowsRaw(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Combobox(ComboboxCfg{
		ID:      "lang",
		Value:   "zig",
		Options: selectLangOptions(),
	}), w)
	txt := firstTextShape(&layout)
	if txt == nil || txt.TC.Text != "zig" {
		t.Errorf("field text = %v, want the raw value", txt)
	}
}

// The dropdown rows are keyed by Value (what click and Enter emit) and
// labelled by Label (what is shown and what the query matches).
func TestComboboxRowsKeyOnValueAndMatchLabel(t *testing.T) {
	w := &Window{}
	StateMap[string, bool](w, nsCombobox, capModerate).Set("lang", true)
	StateMap[string, string](w, nsComboboxQuery, capModerate).
		Set("lang", "Node")
	cache := comboboxCache(t, w, ComboboxCfg{
		ID:      "lang",
		Options: selectLangOptions(),
	})
	if len(cache.ids) != 1 || cache.ids[0] != "typescript_node" {
		t.Errorf("filtered ids = %v, want [typescript_node]", cache.ids)
	}
	if len(cache.filtered) != 1 ||
		cache.filtered[0].Label != "TypeScript (Node.js)" {
		t.Errorf("filtered = %+v, want the Node.js row", cache.filtered)
	}
}

// Changing only a label must rebuild the cache, or the dropdown keeps
// showing the old text.
func TestComboboxCacheInvalidatesOnLabelChange(t *testing.T) {
	w := &Window{}
	cfg := ComboboxCfg{
		ID:      "lang",
		Options: []SelectOption{NewSelectOption("Go", "go")},
	}
	_ = comboboxCache(t, w, cfg)
	cfg.Options = []SelectOption{NewSelectOption("Golang", "go")}
	cache := comboboxCache(t, w, cfg)
	if cache.items[0].Label != "Golang" {
		t.Errorf("cached label = %q, want Golang", cache.items[0].Label)
	}
}

// Items is the string shorthand: each string is both label and value.
// A "---" prefix has no meaning in a Combobox, as before.
func TestComboboxItemsConvertToRows(t *testing.T) {
	w := &Window{}
	cache := comboboxCache(t, w, ComboboxCfg{
		ID:      "s",
		Items:   []string{"A", "---B"},
		Options: selectLangOptions(),
	})
	if len(cache.items) != 2 {
		t.Fatalf("items = %d, want 2 (Items wins over Options)",
			len(cache.items))
	}
	for i, want := range []string{"A", "---B"} {
		it := cache.items[i]
		if it.ID != want || it.Label != want || it.isSubheading {
			t.Errorf("item %d = %+v, want ID=Label=%q", i, it, want)
		}
	}
}

// A Combobox has no section rows: its keyboard walks the filtered
// list without skipping. A subheading option is left out.
func TestComboboxDropsSubheadings(t *testing.T) {
	w := &Window{}
	cache := comboboxCache(t, w, ComboboxCfg{
		ID: "s",
		Options: []SelectOption{
			NewSelectSubheading("Section"),
			NewSelectOption("A", "a"),
		},
	})
	if len(cache.items) != 1 || cache.items[0].ID != "a" {
		t.Errorf("items = %+v, want only [a]", cache.items)
	}
}

// The Items and Options paths hash in different domains. Without a
// domain byte on both, Items {"\x01a", "b"} and Options {("a", "b")}
// hash to the same bytes, and switching one for the other kept the
// old rows.
func TestComboboxCacheInvalidatesOnItemsOptionsSwitch(t *testing.T) {
	w := &Window{}
	cfg := ComboboxCfg{ID: "s", Items: []string{"\x01a", "b"}}
	_ = comboboxCache(t, w, cfg)
	cfg = ComboboxCfg{
		ID:      "s",
		Options: []SelectOption{NewSelectOption("a", "b")},
	}
	cache := comboboxCache(t, w, cfg)
	if len(cache.items) != 1 || cache.items[0].ID != "b" {
		t.Errorf("items = %+v, want only [b]", cache.items)
	}
}

// An option with an empty value ("None") is a real choice: the closed
// field shows its label, not the placeholder.
func TestComboboxEmptyValueOptionShowsLabel(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Combobox(ComboboxCfg{
		ID:          "s",
		Placeholder: "Pick",
		Options: []SelectOption{
			NewSelectOption("None", ""),
			NewSelectOption("Go", "go"),
		},
	}), w)
	txt := firstTextShape(&layout)
	if txt == nil || txt.TC.Text != "None" {
		t.Errorf("field text = %v, want None", txt)
	}
}
