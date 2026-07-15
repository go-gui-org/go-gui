package gui

import (
	"strconv"
	"testing"
)

func benchmarklistCoreItems(n int) []listCoreItem {
	items := make([]listCoreItem, n)
	for i := range n {
		s := "Item " + strconv.Itoa(i)
		items[i] = listCoreItem{ID: s, Label: s}
	}
	return items
}

func benchmarkOptions(n int) []string {
	out := make([]string, n)
	for i := range n {
		out[i] = "Option " + strconv.Itoa(i)
	}
	return out
}

func BenchmarkListCorePrepare(b *testing.B) {
	items := benchmarklistCoreItems(2000)
	b.ReportAllocs()

	b.Run("empty_query", func(b *testing.B) {
		b.ResetTimer()
		for b.Loop() {
			_ = listCorePrepare(items, "", 25)
		}
	})

	b.Run("fuzzy_query", func(b *testing.B) {
		b.ResetTimer()
		for b.Loop() {
			_ = listCorePrepare(items, "199", 25)
		}
	})
}

func BenchmarkComboboxGenerateLayout(b *testing.B) {
	options := benchmarkOptions(500)
	w := newTestWindow()
	cfg := ComboboxCfg{
		ID:         "bench-cb",
		Options:    options,
		OnSelect:   func(_ string, _ *Event, _ *Window) {},
		Scrollable: true,
	}

	b.Run("closed", func(b *testing.B) {
		ss := StateMap[string, bool](w, nsCombobox, capModerate)
		ss.Set(cfg.ID, false)
		v := Combobox(cfg)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(v, w)
		}
	})

	b.Run("open_query", func(b *testing.B) {
		ss := StateMap[string, bool](w, nsCombobox, capModerate)
		ss.Set(cfg.ID, true)
		sq := StateMap[string, string](w, nsComboboxQuery, capModerate)
		sq.Set(cfg.ID, "49")
		v := Combobox(cfg)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(v, w)
		}
	})
}

func BenchmarkCommandPaletteGenerateLayout(b *testing.B) {
	items := make([]CommandPaletteItem, 500)
	for i := range len(items) {
		s := strconv.Itoa(i)
		items[i] = CommandPaletteItem{
			ID:     "cmd-" + s,
			Label:  "Command " + s,
			Detail: "Detail " + s,
		}
	}

	w := newTestWindow()
	id := "bench-cp"
	CommandPaletteShow(id, w)
	StateMap[string, string](w, nsCmdPaletteQuery, capModerate).
		Set(id, "49")

	v := CommandPalette(CommandPaletteCfg{
		ID:         id,
		Items:      items,
		OnAction:   func(_ string, _ *Event, _ *Window) {},
		Scrollable: true,
	})

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w.scratch.resetViewPools()
		_ = generateViewLayout(v, w)
	}
}

func BenchmarkListBoxGenerateLayout(b *testing.B) {
	data := make([]ListBoxOption, 500)
	for i := range len(data) {
		s := strconv.Itoa(i)
		data[i] = NewListBoxOption("id-"+s, "Name "+s, "")
	}
	selected := make([]string, 0, 100)
	for i := range 100 {
		selected = append(selected, "id-"+strconv.Itoa(i))
	}

	b.Run("unbounded", func(b *testing.B) {
		w := newTestWindow()
		v := ListBox(ListBoxCfg{
			ID:          "bench-lb",
			Data:        data,
			SelectedIDs: selected,
			OnSelect:    func(_ []string, _ *Event, _ *Window) {},
		})

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(v, w)
		}
	})

	b.Run("bounded_virtualized", func(b *testing.B) {
		w := newTestWindow()
		scrollID := "9903"
		w.scrollY().Set(scrollID, 1000)
		v := ListBox(ListBoxCfg{
			ID:          "bench-lb-v",
			Scrollable:  true,
			MaxHeight:   220,
			Data:        data,
			SelectedIDs: selected,
			OnSelect:    func(_ []string, _ *Event, _ *Window) {},
		})

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(v, w)
		}
	})
}
