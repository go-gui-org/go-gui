package gui

// view_markdown_cache_test.go guards the markdown styled-block cache
// key (issue #1000): the blocks hold resolved run styles, so the same
// source under two styles must never share entries.

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui/highlight"
)

// mdCacheRunSizes walks a generated layout and returns every RTF run
// size in tree order, so a cache test asserts the style the shapes
// actually carry rather than the cache's internals.
func mdCacheRunSizes(l *Layout, out *[]float32) {
	if l.Shape != nil && l.Shape.TC != nil && l.Shape.TC.rtf != nil &&
		l.Shape.TC.rtf.rTFRuns != nil {
		for _, r := range l.Shape.TC.rtf.rTFRuns.Runs {
			*out = append(*out, r.Style.Size)
		}
	}
	for i := range l.Children {
		mdCacheRunSizes(&l.Children[i], out)
	}
}

// mdCacheRunColors walks a generated layout and returns every RTF
// run's text and color, for the highlighter-separation test below.
func mdCacheRunColors(l *Layout, out *[][2]string) {
	if l.Shape != nil && l.Shape.TC != nil && l.Shape.TC.rtf != nil &&
		l.Shape.TC.rtf.rTFRuns != nil {
		for _, r := range l.Shape.TC.rtf.rTFRuns.Runs {
			*out = append(*out, [2]string{
				r.Text, r.Style.Color.String(),
			})
		}
	}
	for i := range l.Children {
		mdCacheRunColors(&l.Children[i], out)
	}
}

// TestMarkdownCacheSeparatesStyles is the issue #1000 repro: two
// documents in one window with the same source and different styles.
// The second used to draw with the first view's faces and sizes.
func TestMarkdownCacheSeparatesStyles(t *testing.T) {
	w := &Window{}
	src := "x"

	first := DefaultMarkdownStyle()
	firstLayout := generateViewLayout(w.Markdown(MarkdownCfg{
		ID: "md-first", Source: src, Style: first,
	}), w)
	var firstSizes []float32
	mdCacheRunSizes(&firstLayout, &firstSizes)
	if len(firstSizes) == 0 {
		t.Fatal("first document rendered no runs")
	}
	for _, s := range firstSizes {
		if s != first.Text.Size {
			t.Fatalf("first run size = %g, want %g",
				s, first.Text.Size)
		}
	}

	second := DefaultMarkdownStyle()
	second.Text.Size = 20
	secondLayout := generateViewLayout(w.Markdown(MarkdownCfg{
		ID: "md-second", Source: src, Style: second,
	}), w)
	var secondSizes []float32
	mdCacheRunSizes(&secondLayout, &secondSizes)
	if len(secondSizes) == 0 {
		t.Fatal("second document rendered no runs")
	}
	for _, s := range secondSizes {
		if s != 20 {
			t.Errorf("second run size = %g, want 20", s)
		}
	}
}

// TestMarkdownCacheRebuildsOnStyleChange covers the same-view half
// of issue #1000: restyling a document with an unchanged source must
// rebuild its blocks instead of serving the old ones.
func TestMarkdownCacheRebuildsOnStyleChange(t *testing.T) {
	w := &Window{}
	src := "restyle me"

	generateViewLayout(w.Markdown(MarkdownCfg{
		ID: "md", Source: src, Style: DefaultMarkdownStyle(),
	}), w)

	restyled := DefaultMarkdownStyle()
	restyled.Text.Size = 22
	restyled.linkColor = RGB(1, 2, 3)
	again := generateViewLayout(w.Markdown(MarkdownCfg{
		ID: "md", Source: src, Style: restyled,
	}), w)
	var sizes []float32
	mdCacheRunSizes(&again, &sizes)
	if len(sizes) == 0 {
		t.Fatal("restyled document rendered no runs")
	}
	for _, s := range sizes {
		if s != 22 {
			t.Errorf("restyled run size = %g, want 22", s)
		}
	}
}

// TestMarkdownCacheSeparatesHighlighters covers the fingerprint's
// blind spot by behavior: the key carries only the highlighter's
// nil bit, and two set highlighters with different token tables must
// still never share blocks.
func TestMarkdownCacheSeparatesHighlighters(t *testing.T) {
	w := &Window{}
	src := "```go\npackage main\n```\n"

	kw := DefaultMarkdownStyle()
	kw.CodeHighlighter = stubHighlighter{toks: []highlight.Token{
		{Text: "package main", Kind: highlight.KindKeyword},
	}}
	kwLayout := generateViewLayout(w.Markdown(MarkdownCfg{
		ID: "md-hl-kw", Source: src, Style: kw,
	}), w)

	str := DefaultMarkdownStyle()
	str.CodeHighlighter = stubHighlighter{toks: []highlight.Token{
		{Text: "package main", Kind: highlight.KindString},
	}}
	strLayout := generateViewLayout(w.Markdown(MarkdownCfg{
		ID: "md-hl-str", Source: src, Style: str,
	}), w)

	var kwRuns [][2]string
	mdCacheRunColors(&kwLayout, &kwRuns)
	var strRuns [][2]string
	mdCacheRunColors(&strLayout, &strRuns)
	if len(kwRuns) == 0 || len(strRuns) == 0 {
		t.Fatal("highlighted documents rendered no runs")
	}
	wantKW := kw.codeKeywordColor.String()
	foundKW := false
	for _, r := range kwRuns {
		if r[1] == wantKW {
			foundKW = true
		}
	}
	if !foundKW {
		t.Errorf("keyword document has no %s run: %v", wantKW, kwRuns)
	}
	wantStr := str.codeStringColor.String()
	foundStr := false
	for _, r := range strRuns {
		if r[1] == wantStr {
			foundStr = true
		}
	}
	if !foundStr {
		t.Errorf("string document has no %s run: %v", wantStr, strRuns)
	}
}

// mdRichMarkdownStyle returns a style with every pointer, slice and
// interface leaf populated, so the leaf walk below perturbs set
// values as well as nil ones.
func mdRichMarkdownStyle() MarkdownStyle {
	s := DefaultMarkdownStyle()
	s.CodeHighlighter = stubHighlighter{toks: []highlight.Token{
		{Text: "x", Kind: highlight.KindKeyword},
	}}
	s.Text.Features = &glyph.FontFeatures{
		OpenTypeFeatures: []glyph.FontFeature{{Tag: "smcp", Value: 1}},
		VariationAxes:    []glyph.FontAxis{{Tag: "wght", Value: 700}},
	}
	s.Text.AffineTransform = &glyph.AffineTransform{XX: 1, YY: 1}
	s.Text.Gradient = &glyph.GradientConfig{
		Direction: glyph.GradientHorizontal,
		Stops: []glyph.GradientStop{
			{Color: glyph.Color{R: 255, A: 255}, Position: 0},
			{Color: glyph.Color{B: 255, A: 255}, Position: 1},
		},
	}
	alt := RGB(1, 2, 3)
	s.tableRowAlt = &alt
	return s
}

// TestMarkdownStyleFingerprintCoversEveryLeaf walks every leaf of
// MarkdownStyle from three bases (zero, theme default, fully
// populated) and perturbs one leaf at a time. A field the
// fingerprint ignores keeps the digest and fails naming the path,
// so a later field addition cannot silently reopen issue #1000.
func TestMarkdownStyleFingerprintCoversEveryLeaf(t *testing.T) {
	bases := []MarkdownStyle{
		{},
		DefaultMarkdownStyle(),
		mdRichMarkdownStyle(),
	}
	for bi, base := range bases {
		for _, path := range mdLeafPaths(reflect.TypeOf(base)) {
			mod := base
			desc := mdPerturbAtPath(reflect.ValueOf(&mod), path)
			if desc == "" {
				t.Errorf("base %d: cannot perturb path %v", bi, path)
				continue
			}
			if markdownStyleFingerprint(&mod) ==
				markdownStyleFingerprint(&base) {
				t.Errorf("base %d: fingerprint ignores %s", bi, desc)
			}
		}
	}
}

// mdLeafPaths returns one index path per settable leaf under typ.
// Structs, pointers and slices are descended through; scalars and
// interfaces are leaves. A depth cap stops a pathological cycle
// instead of recursing forever.
func mdLeafPaths(typ reflect.Type) [][]int {
	var out [][]int
	var walk func(t reflect.Type, path []int, depth int)
	walk = func(t reflect.Type, path []int, depth int) {
		if depth > 8 {
			return
		}
		switch t.Kind() {
		case reflect.Struct:
			for i := range t.NumField() {
				next := append(append([]int(nil), path...), i)
				walk(t.Field(i).Type, next, depth+1)
			}
		case reflect.Pointer:
			next := append(append([]int(nil), path...), 0)
			walk(t.Elem(), next, depth+1)
		case reflect.Slice, reflect.Array:
			next := append(append([]int(nil), path...), 0)
			walk(t.Elem(), next, depth+1)
		default:
			out = append(out, append([]int(nil), path...))
		}
	}
	walk(typ, nil, 0)
	return out
}

// mdPerturbAtPath navigates path from the root struct pointer and
// changes the leaf it lands on, allocating through nil pointers and
// growing empty slices. It returns a dotted field description for
// failure messages, or "" when the leaf cannot be changed.
func mdPerturbAtPath(root reflect.Value, path []int) string {
	v := root.Elem()
	var names []string
	for depth, idx := range path {
		switch v.Kind() {
		case reflect.Struct:
			if idx >= v.NumField() {
				return ""
			}
			names = append(names, v.Type().Field(idx).Name)
			v = v.Field(idx)
			// Reach through unexported fields, the same way the
			// readable helper in theme_colors_equivalence_test.go
			// does: the values are test-owned copies.
			v = reflect.NewAt(v.Type(),
				unsafe.Pointer(v.UnsafeAddr())).Elem()
		case reflect.Pointer:
			// Clone, not just descend: mod started as a shallow
			// copy, so its pointers still name base's structs and
			// an in-place perturb would move both digests
			// together. The clone is shallow; deeper levels on
			// the path are cloned when the walk reaches them.
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			} else {
				cp := reflect.New(v.Type().Elem())
				cp.Elem().Set(v.Elem())
				v.Set(cp)
			}
			v = v.Elem()
		case reflect.Slice:
			// Same aliasing rule as pointers: a perturbed
			// element must live in mod's own backing array.
			if v.Len() == 0 {
				v.Set(reflect.MakeSlice(v.Type(), 1, 1))
			} else {
				cp := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
				reflect.Copy(cp, v)
				v.Set(cp)
			}
			names = append(names, "[0]")
			v = v.Index(0)
		case reflect.Array:
			names = append(names, "[0]")
			v = v.Index(0)
		default:
			_ = depth
			return ""
		}
	}
	if mdPerturbLeaf(v) {
		return strings.Join(names, ".")
	}
	return ""
}

// mdPerturbLeaf changes one scalar or interface leaf. Every branch
// preserves the leaf's kind and only moves its value, so the result
// stays a valid style whose digest must differ.
func mdPerturbLeaf(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		v.SetString(v.String() + "\x00")
		return true
	case reflect.Bool:
		v.SetBool(!v.Bool())
		return true
	case reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64:
		// XOR, not arithmetic: 255*2+1 wraps to 255 on a channel,
		// and SetInt may panic on overflow. A low-bit flip always
		// lands a different value of the same kind.
		v.SetInt(v.Int() ^ 1)
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		v.SetUint(v.Uint() ^ 1)
		return true
	case reflect.Float32, reflect.Float64:
		v.SetFloat(v.Float()*2 + 1)
		return true
	case reflect.Interface:
		// The fingerprint carries only the highlighter's nil
		// bit; either direction must move it.
		if v.IsNil() {
			v.Set(reflect.ValueOf(stubHighlighter{}))
		} else {
			v.Set(reflect.Zero(v.Type()))
		}
		return true
	default:
		return false
	}
}

// TestMarkdownCacheSharesIdenticalPairs pins the other half of the
// key: two documents with the same source, style and highlighter
// reuse one entry rather than parsing twice.
func TestMarkdownCacheSharesIdenticalPairs(t *testing.T) {
	w := &Window{}
	src := "```go\npackage main\n```\n"
	style := DefaultMarkdownStyle()
	style.CodeHighlighter = highlight.New()
	key := newMarkdownCacheKey(src, &style)

	generateViewLayout(w.Markdown(MarkdownCfg{
		ID: "md-a", Source: src, Style: style,
	}), w)
	first, ok := w.viewState.markdownCache.Get(key)
	if !ok || len(first.blocks) == 0 {
		t.Fatal("first document left no cache entry")
	}
	generateViewLayout(w.Markdown(MarkdownCfg{
		ID: "md-b", Source: src, Style: style,
	}), w)
	second, ok := w.viewState.markdownCache.Get(key)
	if !ok || len(second.blocks) == 0 {
		t.Fatal("second document left no cache entry")
	}
	if n := w.viewState.markdownCache.Len(); n != 1 {
		t.Errorf("cache entries = %d, want 1", n)
	}
	if &second.blocks[0] != &first.blocks[0] {
		t.Error("second document re-parsed instead of sharing blocks")
	}
}

// TestSameMarkdownHighlighter covers the entry check: nil matches
// nil only, equal values and one pointer match, different token
// tables miss.
func TestSameMarkdownHighlighter(t *testing.T) {
	kw := stubHighlighter{toks: []highlight.Token{
		{Text: "x", Kind: highlight.KindKeyword},
	}}
	str := stubHighlighter{toks: []highlight.Token{
		{Text: "x", Kind: highlight.KindString},
	}}
	ptr := &stubHighlighter{}
	cases := []struct {
		name string
		a, b highlight.Highlighter
		want bool
	}{
		{"nil nil", nil, nil, true},
		{"nil set", nil, kw, false},
		{"set nil", kw, nil, false},
		{"equal values", kw, stubHighlighter{toks: kw.toks}, true},
		{"different tables", kw, str, false},
		{"same pointer", ptr, ptr, true},
		{"chroma", highlight.New(), highlight.New(), true},
	}
	for _, c := range cases {
		if got := sameMarkdownHighlighter(c.a, c.b); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// TestMarkdownCacheKeyNoAllocs guards the per-frame cost: the key
// and the highlighter check run on every markdown generation, so
// neither may touch the heap for chroma, a pointer highlighter, or a
// slice-bearing value reused across frames.
func TestMarkdownCacheKeyNoAllocs(t *testing.T) {
	style := mdRichMarkdownStyle()
	stub := stubHighlighter{toks: []highlight.Token{
		{Text: "x", Kind: highlight.KindKeyword},
	}}
	for _, hl := range []highlight.Highlighter{
		highlight.New(), &stubHighlighter{}, stub,
	} {
		style.CodeHighlighter = hl
		allocs := testing.AllocsPerRun(100, func() {
			_ = newMarkdownCacheKey("x", &style)
			_ = sameMarkdownHighlighter(hl, hl)
		})
		if allocs != 0 {
			t.Errorf("%T: allocs per lookup = %g, want 0", hl, allocs)
		}
	}
}

// TestMarkdownCacheSeparatesHighlighterTypes is the eviction-loop
// repro: two views with the same source and style but highlighters
// of different types. With one shared key, each frame's Set evicted
// the other view's entry and both re-parsed every frame. The key
// carries the type, so both entries survive and a second frame
// reuses them.
func TestMarkdownCacheSeparatesHighlighterTypes(t *testing.T) {
	w := &Window{}
	src := "```go\npackage main\n```\n"
	chroma := DefaultMarkdownStyle()
	chroma.CodeHighlighter = highlight.New()
	stub := DefaultMarkdownStyle()
	stub.CodeHighlighter = stubHighlighter{toks: []highlight.Token{
		{Text: "package main", Kind: highlight.KindKeyword},
	}}
	frame := func() {
		generateViewLayout(w.Markdown(MarkdownCfg{
			ID: "md-chroma", Source: src, Style: chroma,
		}), w)
		generateViewLayout(w.Markdown(MarkdownCfg{
			ID: "md-stub", Source: src, Style: stub,
		}), w)
	}
	frame()
	if n := w.viewState.markdownCache.Len(); n != 2 {
		t.Fatalf("cache entries = %d, want 2", n)
	}
	before, _ := w.viewState.markdownCache.Get(
		newMarkdownCacheKey(src, &chroma))
	frame()
	after, ok := w.viewState.markdownCache.Get(
		newMarkdownCacheKey(src, &chroma))
	if !ok || len(before.blocks) == 0 || len(after.blocks) == 0 {
		t.Fatal("chroma document left no cache entry")
	}
	if &after.blocks[0] != &before.blocks[0] {
		t.Error("chroma document re-parsed on the second frame")
	}
}
