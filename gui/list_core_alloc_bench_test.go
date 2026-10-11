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
		Items:    options,
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

// BenchmarkSelectGenerateLayout runs the factory inside the loop, as a
// frame does: that is where an Items→Options copy would allocate. The
// items and options cases should report the same allocs/op (#809).
func BenchmarkSelectGenerateLayout(b *testing.B) {
	items := benchmarkOptions(500)
	opts := make([]SelectOption, len(items))
	for i, it := range items {
		opts[i] = NewSelectOption(it, it)
	}
	w := newTestWindow()
	cases := []struct {
		name string
		cfg  SelectCfg
	}{
		{"items", SelectCfg{ID: "bench-sel", Items: items,
			Selected: []string{"Option 250"}}},
		{"options", SelectCfg{ID: "bench-sel", Options: opts,
			Selected: []string{"Option 250"}}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				w.scratch.resetViewPools()
				_ = generateViewLayout(Select(tc.cfg), w)
			}
		})
	}
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
		ID:       id,
		Items:    items,
		OnAction: func(_ string, ctx EventCtx) {},
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
			ID:       "bench-lb-fill",
			Sizing:   FillFill,
			Data:     big,
			OnSelect: func(_ []string, ctx EventCtx) {},
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

// rowAllocsPerRow is the per-row allocation cost of a list-like
// widget: the allocations of one frame with 16 rows minus those with
// 8, over 8. List-level costs cancel out, so the number is what each
// visible row adds. build makes the test data for n rows and returns
// the factory call, which runs inside the measured func as it does in
// a real frame; rows counts the rows it built.
func rowAllocsPerRow(
	t *testing.T, build func(n int) func() View, rows func(*Layout) int,
) float64 {
	t.Helper()
	const small, large = 8, 16
	measure := func(n int) float64 {
		w := newTestWindow()
		view := build(n)
		if got := rows(new(generateViewLayout(view(), w))); got != n {
			t.Fatalf("rows built = %d, want %d", got, n)
		}
		return testing.AllocsPerRun(50, func() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(view(), w)
		})
	}
	perRow := (measure(large) - measure(small)) / (large - small)
	t.Logf("allocs per row = %.2f", perRow)
	return perRow
}

// rowAllocsListBoxData is n plain ListBox options.
func rowAllocsListBoxData(n int) []ListBoxOption {
	data := make([]ListBoxOption, n)
	for i := range n {
		s := strconv.Itoa(i)
		data[i] = NewListBoxOption("id-"+s, "Name "+s, "")
	}
	return data
}

// rowAllocsTreeNodes is n leaf nodes at the root.
func rowAllocsTreeNodes(n int) []TreeNodeCfg {
	nodes := make([]TreeNodeCfg, n)
	for i := range n {
		s := strconv.Itoa(i)
		nodes[i] = TreeNodeCfg{ID: "n-" + s, Text: "Node " + s}
	}
	return nodes
}

// The per-row gates below hold each list-like row to what its own
// shapes need. The hover look is a HoverStyle (#977), the click is the
// shared rowOnClick that reads its row through the shape, and the
// Content slice comes from the frame arena. A closure or a heap slice
// per row raises the count.

func TestListBoxRowAllocsPerRow(t *testing.T) {
	perRow := rowAllocsPerRow(t, func(n int) func() View {
		cfg := ListBoxCfg{
			ID:       "lb-allocs",
			Data:     rowAllocsListBoxData(n),
			OnSelect: func([]string, EventCtx) {},
		}
		return func() View { return ListBox(cfg) }
	}, func(l *Layout) int { return len(listBoxRows(l)) })
	// Measured 3: the row's a11y payload, its text view and the text
	// shape's payload. With a closure per row for click and hover and
	// a heap Content slice it was 6.
	if perRow > 3 {
		t.Errorf("allocs per row = %.2f, want <= 3", perRow)
	}
}

func TestListBoxReorderRowAllocsPerRow(t *testing.T) {
	perRow := rowAllocsPerRow(t, func(n int) func() View {
		cfg := ListBoxCfg{
			ID:          "lb-reorder-allocs",
			Data:        rowAllocsListBoxData(n),
			Reorderable: true,
			OnSelect:    func([]string, EventCtx) {},
			OnReorder:   func(string, string, EventCtx) {},
		}
		return func() View { return ListBox(cfg) }
	}, func(l *Layout) int { return len(listBoxRows(l)) })
	// Measured 4: the plain row's 3 and the row's layout ID, which
	// drag tracking needs. With a click closure and a heap Content
	// slice, a hover closure and the layout ID composed twice, it was 8.
	if perRow > 4 {
		t.Errorf("allocs per row = %.2f, want <= 4", perRow)
	}
}

func TestTreeRowAllocsPerRow(t *testing.T) {
	perRow := rowAllocsPerRow(t, func(n int) func() View {
		cfg := TreeCfg{
			ID:       "tree-allocs",
			Nodes:    rowAllocsTreeNodes(n),
			OnSelect: func(string, EventCtx) {},
		}
		return func() View { return Tree(cfg) }
	}, func(l *Layout) int { return len(treeRows(l)) })
	// Measured 5.25: the row's container, its a11y payload and its 3
	// text views, plus map growth. With a click closure that copied
	// the row, a hover closure, a heap Content slice and a map holding
	// row values it was 11.25.
	if perRow > 5.5 {
		t.Errorf("allocs per row = %.2f, want <= 5.5", perRow)
	}
}

func TestTreeReorderRowAllocsPerRow(t *testing.T) {
	perRow := rowAllocsPerRow(t, func(n int) func() View {
		cfg := TreeCfg{
			ID:          "tree-reorder-allocs",
			Nodes:       rowAllocsTreeNodes(n),
			Reorderable: true,
			OnSelect:    func(string, EventCtx) {},
			OnReorder:   func(string, string, EventCtx) {},
		}
		return func() View { return Tree(cfg) }
	}, func(l *Layout) int { return len(treeRows(l)) })
	// Measured 7.38: the plain row's cost, the row's layout ID, and
	// the growth of the per-parent drag maps. With a click closure
	// that copied the row, a hover closure and the layout ID composed
	// twice it was 14.75.
	if perRow > 7.5 {
		t.Errorf("allocs per row = %.2f, want <= 7.5", perRow)
	}
}

// listCoreViews builds the rows of Combobox and CommandPalette.
func TestListCoreRowAllocsPerRow(t *testing.T) {
	cfg := listCoreCfg{
		Colors:      defaultComboboxStyle.Colors,
		OnItemClick: func(string, int, EventCtx) {},
	}
	perRow := rowAllocsPerRow(t, func(n int) func() View {
		items := benchmarklistCoreItems(n)
		return func() View {
			return Column(ContainerCfg{
				Content: listCoreViews(items, cfg, 0, n-1, -1, nil, 0),
			})
		}
	}, func(l *Layout) int { return len(l.Children) })
	// Measured 3. With a click closure and a hover closure per row it
	// was 5. The Content slice stays on the heap: Combobox and
	// CommandPalette cache these views across frames.
	if perRow > 3 {
		t.Errorf("allocs per row = %.2f, want <= 3", perRow)
	}
}

// BenchmarkTreeGenerateLayout builds a 200-node tree, every row
// visible, as a frame does: the factory runs inside the loop.
func BenchmarkTreeGenerateLayout(b *testing.B) {
	nodes := rowAllocsTreeNodes(200)
	for _, reorder := range []bool{false, true} {
		name := "plain"
		if reorder {
			name = "reorderable"
		}
		b.Run(name, func(b *testing.B) {
			w := newTestWindow()
			cfg := TreeCfg{
				ID:          "bench-tree",
				Nodes:       nodes,
				Reorderable: reorder,
				OnSelect:    func(string, EventCtx) {},
				OnReorder:   func(string, string, EventCtx) {},
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				w.scratch.resetViewPools()
				_ = generateViewLayout(Tree(cfg), w)
			}
		})
	}
}
