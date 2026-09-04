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
		ID:       "bench-cb",
		Options:  options,
		OnSelect: func(_ string, ctx EventCtx) {},
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
	commandPaletteShow(id, w)
	StateMap[string, string](w, nsCmdPaletteQuery, capModerate).
		Set(id, "49")

	v := CommandPalette(CommandPaletteCfg{
		ID:         id,
		Items:      items,
		OnAction:   func(_ string, ctx EventCtx) {},
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
			OnSelect:    func(_ []string, ctx EventCtx) {},
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
			OnSelect:    func(_ []string, ctx EventCtx) {},
		})

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(v, w)
		}
	})

	b.Run("fill_10k_virtualized", func(b *testing.B) {
		// Fill sizing with no configured height: virtualization runs
		// against the height the amend hook captured from the last
		// arranged frame, so the 10k-row build stays bounded.
		big := listBoxTestData(10_000)
		w := newTestWindow()
		cfg := ListBoxCfg{
			ID:         "bench-lb-fill",
			Scrollable: true,
			Sizing:     FillFill,
			Data:       big,
			OnSelect:   func(_ []string, ctx EventCtx) {},
		}
		cache := listBoxEnsureCache(&cfg, w)
		cache.resolvedH = 300
		cache.hSeen = true
		v := ListBox(cfg)

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(v, w)
		}
	})
}

// BenchmarkVirtualListGenerateLayout pins the property that matters
// for a virtualized list: the per-frame cost is a function of the
// *visible* rows, not of the corpus. The assertion in the alloc gate
// is invariance across ItemCount, not an absolute number — a build
// that starts scaling with the data is the regression, whatever the
// constant happens to be on the day.
func BenchmarkVirtualListGenerateLayout(b *testing.B) {
	for _, n := range []int{1_000, 100_000, 1_000_000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			w := newTestWindow()
			cfg := virtualListCfg("bench-vl", n, 300)
			cfg.ItemHeight = func(i int, _ float32) float32 {
				return virtualListRowH(i)
			}
			v := VirtualList(cfg)
			// Land mid-corpus so both spacers exist and the range walk
			// is a real Fenwick descent rather than a top-of-list hit.
			_ = generateViewLayout(v, w)
			m, _ := listHeightLookup(w, "bench-vl")
			w.scrollY().Set("bench-vl", -m.Prefix(n/2))

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				w.scratch.resetViewPools()
				_ = generateViewLayout(v, w)
			}
		})
	}
}

// TestVirtualListAllocsAreFlat is the gate form of the benchmark
// above: the per-frame allocation count must not grow with the item
// count. A build that starts touching every item shows up here as a
// thousand-fold difference, long before it shows up as a slow frame.
func TestVirtualListAllocsAreFlat(t *testing.T) {
	measure := func(n int) float64 {
		w := newTestWindow()
		cfg := virtualListCfg("flat-vl", n, 300)
		cfg.ItemHeight = func(i int, _ float32) float32 {
			return virtualListRowH(i)
		}
		v := VirtualList(cfg)
		_ = generateViewLayout(v, w)
		m, _ := listHeightLookup(w, "flat-vl")
		w.scrollY().Set("flat-vl", -m.Prefix(n/2))
		res := testing.Benchmark(func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				w.scratch.resetViewPools()
				_ = generateViewLayout(v, w)
			}
		})
		return float64(res.AllocsPerOp())
	}
	small, big := measure(1_000), measure(1_000_000)
	if small == 0 {
		t.Fatal("benchmark reported no allocations")
	}
	if ratio := big / small; ratio > 1.05 {
		t.Fatalf("allocs/op grew with ItemCount: %v at 1k, %v at 1M",
			small, big)
	}
}
