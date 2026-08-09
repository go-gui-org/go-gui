package svg

import (
	"strconv"
	"strings"
	"testing"
)

// --- hasUseElement short-circuit ---

func TestHasUseElementAbsent(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 10 10"><rect width="5" height="5"/></svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if hasUseElement(root) {
		t.Fatalf("expected hasUseElement=false")
	}
}

func TestHasUseElementPresent(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 10 10"><defs><rect id="r" width="5" height="5"/></defs><use href="#r"/></svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !hasUseElement(root) {
		t.Fatalf("expected hasUseElement=true")
	}
}

// --- expandUseElements basic ---

func TestExpandUseSymbol(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<defs>
			<symbol id="s"><rect width="10" height="10" fill="red"/></symbol>
		</defs>
		<use href="#s" x="5" y="6"/>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) == 0 {
		t.Fatalf("expected at least one path from <use>")
	}
}

func TestExpandUseTranslateComposesTransform(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s"><rect width="1" height="1"/></symbol></defs>` +
			`<use href="#s" x="3" y="4" transform="rotate(90)"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	useG := findFirstByName(root, "g")
	if useG == nil {
		t.Fatalf("expected synthesized <g> wrapper")
	}
	tr := useG.AttrMap["transform"]
	if !strings.Contains(tr, "rotate(90)") || !strings.Contains(tr, "translate(3,4)") {
		t.Fatalf("expected composed transform, got %q", tr)
	}
	if strings.Index(tr, "rotate(90)") > strings.Index(tr, "translate(3,4)") {
		t.Fatalf("expected rotate before translate, got %q", tr)
	}
}

// --- malformed href ---

func TestExpandUseMalformedHref(t *testing.T) {
	cases := []string{
		`<svg><defs><rect id="a" width="1" height="1"/></defs><use href=""/></svg>`,
		`<svg><defs><rect id="a" width="1" height="1"/></defs><use href="#"/></svg>`,
		`<svg><defs><rect id="a" width="1" height="1"/></defs><use href="a"/></svg>`,
		`<svg><defs><rect id="a" width="1" height="1"/></defs><use href="#missing"/></svg>`,
	}
	for i, svg := range cases {
		root, err := decodeSvgTree(svg)
		if err != nil {
			t.Fatalf("case %d decode: %v", i, err)
		}
		expandUseElements(root)
		// <use> remains as-is when unresolvable; parse must not loop or crash.
		if findFirstByName(root, "use") == nil {
			t.Fatalf("case %d: <use> should remain when href is unresolvable", i)
		}
	}
}

// --- cycle guard ---

func TestExpandUseCycleGuard(t *testing.T) {
	svg := `<svg>
		<defs>
			<g id="a"><use href="#b"/></g>
			<g id="b"><use href="#a"/></g>
		</defs>
		<use href="#a"/>
	</svg>`
	root, err := decodeSvgTree(svg)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	// no panic, no infinite expansion
}

// --- depth cap ---

func TestExpandUseDepthCap(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<svg><defs>`)
	for i := range 20 {
		// chain: id_i contains <use href="#id_{i+1}">
		// last id has no use
		if i < 19 {
			b.WriteString(`<g id="g`)
			b.WriteString(strconv.Itoa(i))
			b.WriteString(`"><use href="#g`)
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(`"/></g>`)
		} else {
			b.WriteString(`<g id="g`)
			b.WriteString(strconv.Itoa(i))
			b.WriteString(`"><rect width="1" height="1"/></g>`)
		}
	}
	b.WriteString(`</defs><use href="#g0"/></svg>`)

	root, err := decodeSvgTree(b.String())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	// passes if no stack blow-up; depth cap stops chain.
}

// --- fanout budget ---

func TestExpandUseFanoutBudget(t *testing.T) {
	// Build symbol with a single rect, then a parent containing
	// many <use href="#s"> siblings. Each surfaces 1 child clone, so
	// total clones <= budget.
	var b strings.Builder
	b.WriteString(`<svg><defs><symbol id="s"><rect width="1" height="1"/></symbol></defs>`)
	const fan = 5
	for range fan {
		b.WriteString(`<use href="#s"/>`)
	}
	b.WriteString(`</svg>`)
	root, err := decodeSvgTree(b.String())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	// Each <use> becomes a <g> wrapping the symbol's rect child.
	gs := countByName(root, "g")
	if gs < fan {
		t.Fatalf("expected at least %d <g> wrappers, got %d", fan, gs)
	}
}

func TestExpandUseFanoutBudgetTruncates(t *testing.T) {
	// Force budget exhaustion: clone deeply nested target many times.
	// Use a target with multiple children so each <use> consumes
	// several budget units.
	var sym strings.Builder
	sym.WriteString(`<symbol id="s">`)
	for range 100 {
		sym.WriteString(`<rect width="1" height="1"/>`)
	}
	sym.WriteString(`</symbol>`)

	var b strings.Builder
	b.WriteString(`<svg><defs>`)
	b.WriteString(sym.String())
	b.WriteString(`</defs>`)
	for range 2000 {
		b.WriteString(`<use href="#s"/>`)
	}
	b.WriteString(`</svg>`)
	root, err := decodeSvgTree(b.String())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Must complete without OOM/panic and budget must clamp output.
	expandUseElements(root)
	// Without the budget, expansion would produce 2000 × 100 = 200000
	// cloned rects on top of the 100 in the source <symbol>. The
	// budget caps total clones at maxUseExpandClones; the surviving
	// rect count must be well below the unbounded result.
	rectCount := countByName(root, "rect")
	if rectCount > maxUseExpandClones+200 {
		t.Fatalf("rectCount=%d exceeds budget %d (+slack)",
			rectCount, maxUseExpandClones)
	}
	if rectCount >= 200000 {
		t.Fatalf("budget did not truncate fanout: rectCount=%d", rectCount)
	}
}

// --- ID stripping after clone ---

func TestExpandUseStripsClonedID(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg><defs><symbol id="s"><rect id="inner" width="1" height="1"/></symbol></defs>` +
			`<use href="#s"/></svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	// At most one rect retains id="inner" (the original inside
	// <symbol>); cloned rects under the synthesized <g> must not.
	withID := 0
	for _, r := range collectByName(root, "rect") {
		if r.AttrMap["id"] == "inner" {
			withID++
		}
	}
	if withID > 1 {
		t.Fatalf("expected at most 1 rect with id=inner, got %d", withID)
	}
}

// Nested ids inside a cloned <use> subtree must also be stripped.
// stripID was previously non-recursive, so deep ids leaked into every
// clone and corrupted url(#id) / CSS / animation targeting.
func TestExpandUseStripsNestedClonedID(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg><defs><symbol id="s"><g id="outer"><rect id="inner" width="1" height="1"/></g></symbol></defs>` +
			`<use href="#s"/><use href="#s"/></svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	innerHits := 0
	outerHits := 0
	for _, r := range collectByName(root, "rect") {
		if r.AttrMap["id"] == "inner" {
			innerHits++
		}
	}
	for _, g := range collectByName(root, "g") {
		if g.AttrMap["id"] == "outer" {
			outerHits++
		}
	}
	// Source <symbol> retains its descendants; clones must not.
	if innerHits > 1 {
		t.Fatalf("expected at most 1 rect#inner, got %d", innerHits)
	}
	if outerHits > 1 {
		t.Fatalf("expected at most 1 g#outer, got %d", outerHits)
	}
}

// <use width=W height=H> referencing a <symbol viewBox=...> must scale
// the symbol's viewport to fit the requested box. Default
// preserveAspectRatio is xMidYMid meet (uniform scale + center) per
// SVG 1.1 — earlier impl always stretched independently, distorting
// 10x10 symbols when used at non-square boxes. 20x40 box of 10x10
// symbol → uniform 2x; 20x20 of content centered in 20x40 → ay=10,
// so y-translate = 6+10 = 16.
func TestExpandUseSymbolHonorsWidthHeight(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="5" y="6" width="20" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if !strings.Contains(tr, "translate(5,16)") {
		t.Fatalf("missing translate(5,16) (xMidYMid meet centers Y): %q", tr)
	}
	if !strings.Contains(tr, "scale(2)") {
		t.Fatalf("missing uniform scale(2): %q", tr)
	}
}

// preserveAspectRatio="none" on the symbol opts back into independent
// per-axis scaling — the legacy stretch behavior.
func TestExpandUseSymbolPreserveNoneStretches(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10" preserveAspectRatio="none"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="5" y="6" width="20" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if !strings.Contains(tr, "translate(5,6)") {
		t.Fatalf("missing translate(5,6): %q", tr)
	}
	if !strings.Contains(tr, "scale(2,4)") {
		t.Fatalf("missing scale(2,4) for none stretch: %q", tr)
	}
}

// preserveAspectRatio="xMinYMin meet" pins to the box origin: no
// align offset, uniform scale = min.
func TestExpandUseSymbolPreserveXMinYMin(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10" preserveAspectRatio="xMinYMin meet"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="5" y="6" width="20" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if !strings.Contains(tr, "translate(5,6)") {
		t.Fatalf("xMinYMin must zero align offsets: %q", tr)
	}
	if !strings.Contains(tr, "scale(2)") {
		t.Fatalf("missing uniform scale(2): %q", tr)
	}
}

// xMidYMid slice produces uniform max-scale + center alignment, plus
// a synth clipPath constraining content to the use box (spec
// requirement; without the clip, slice content would overflow).
func TestExpandUseSymbolPreserveSlice(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10" preserveAspectRatio="xMidYMid slice"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" width="20" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	// Slice → s = max(2,4) = 4. ax = 0.5*(20 - 10*4) = -10. ay = 0.
	if !strings.Contains(tr, "translate(-10,0)") {
		t.Fatalf("slice align offset wrong: %q", tr)
	}
	if !strings.Contains(tr, "scale(4)") {
		t.Fatalf("missing uniform scale(4) for slice: %q", tr)
	}
	cp := g.AttrMap["clip-path"]
	if !strings.HasPrefix(cp, "url(#__use_clip_") {
		t.Fatalf("expected synth use-box clip-path on <g>; got %q", cp)
	}
}

// Slice scaling end-to-end: the synth use-box clipPath must reach
// vg.ClipPaths and the inlined shape must reference it.
func TestParseSvg_UseSymbolPreserveSliceClipsToBox(t *testing.T) {
	vg, err := parseSvg(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10" preserveAspectRatio="xMidYMid slice">` +
			`<rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" width="20" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var clipID string
	for id := range vg.clipPaths {
		if strings.HasPrefix(id, "__use_clip_") {
			clipID = id
			break
		}
	}
	if clipID == "" {
		t.Fatal("expected a __use_clip_* entry in vg.ClipPaths")
	}
	if len(vg.Paths) == 0 {
		t.Fatal("no paths from inlined symbol")
	}
	if vg.Paths[0].clipPathID != clipID {
		t.Fatalf("inlined shape ClipPathID=%q; want %q",
			vg.Paths[0].clipPathID, clipID)
	}
}

// preserveAspectRatio="meet" (default) does NOT need the use-box clip
// because uniform min-scale leaves content inside the box. Confirm
// no synth clip is emitted in that case.
func TestExpandUseSymbolPreserveMeet_NoUseBoxClip(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" width="20" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatal("expected synthesized <g>")
	}
	if cp := g.AttrMap["clip-path"]; cp != "" {
		t.Fatalf("meet mode must not synth clip; got clip-path=%q", cp)
	}
}

// Author clip-path on the <use> itself wins; synth use-box clip must
// not overwrite it (cascade origin precedence).
func TestExpandUseSymbolPreserveSlice_AuthorClipOnUseSuppressesSynth(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs>` +
			`<clipPath id="author"><rect width="50" height="50"/></clipPath>` +
			`<symbol id="s" viewBox="0 0 10 10" preserveAspectRatio="xMidYMid slice">` +
			`<rect width="10" height="10"/></symbol>` +
			`</defs>` +
			`<use href="#s" x="0" y="0" width="20" height="40" ` +
			`clip-path="url(#author)"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatal("expected synthesized <g>")
	}
	if cp := g.AttrMap["clip-path"]; cp != "url(#author)" {
		t.Fatalf("author clip-path should win; got %q", cp)
	}
}

// Two slice <use> elements sharing one symbol must each mint a
// distinct __use_clip_N id — collision would make one shape clip to
// the other's box.
func TestParseSvg_MultipleSliceUsesGetDistinctClipIDs(t *testing.T) {
	vg, err := parseSvg(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10" ` +
			`preserveAspectRatio="xMidYMid slice">` +
			`<rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" width="20" height="40"/>` +
			`<use href="#s" x="50" y="0" width="40" height="20"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var ids []string
	for id := range vg.clipPaths {
		if strings.HasPrefix(id, "__use_clip_") {
			ids = append(ids, id)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("got %d __use_clip_* entries; want 2 (one per <use>)", len(ids))
	}
	if ids[0] == ids[1] {
		t.Fatalf("clip ids collide: %q", ids[0])
	}
	if len(vg.Paths) != 2 {
		t.Fatalf("got %d paths; want 2", len(vg.Paths))
	}
	if vg.Paths[0].clipPathID == vg.Paths[1].clipPathID {
		t.Fatalf("paths reference same clip id %q; each <use> should have own",
			vg.Paths[0].clipPathID)
	}
}

// Authored id matching the synth-clip stem (`__use_clip_1`) must
// not collide with a freshly minted slice clip id. The minter must
// skip ids already present in the document index, otherwise either
// the authored clipPath silently shadows the synth rect or the synth
// rect overwrites references to the authored id.
func TestExpandUseSymbolPreserveSlice_AuthorIDCollisionAvoided(t *testing.T) {
	vg, err := parseSvg(
		`<svg viewBox="0 0 100 100">` +
			`<defs>` +
			`<clipPath id="__use_clip_1">` +
			`<rect x="0" y="0" width="5" height="5"/>` +
			`</clipPath>` +
			`<symbol id="s" viewBox="0 0 10 10" ` +
			`preserveAspectRatio="xMidYMid slice">` +
			`<rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" width="40" height="20"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	authored, ok := vg.clipPaths["__use_clip_1"]
	if !ok {
		t.Fatal("authored __use_clip_1 missing from ClipPaths")
	}
	// Authored clip is a 5x5 rect; synth would be 40x20. Anything
	// other than 5x5 means the synth shadowed the authored entry.
	if len(authored) != 1 || len(authored[0].Segments) == 0 {
		t.Fatal("authored __use_clip_1 has no segments")
	}
	var synthID string
	for id := range vg.clipPaths {
		if strings.HasPrefix(id, "__use_clip_") && id != "__use_clip_1" {
			synthID = id
			break
		}
	}
	if synthID == "" {
		t.Fatal("synth slice clip id missing; minter must skip authored id")
	}
}

// Two consecutive authored synth-prefix ids force the minter to walk
// past both before landing on a free id. Single-collision coverage
// alone wouldn't catch a minter that ignored idIndex on retries.
func TestExpandUseSymbolPreserveSlice_AuthorIDCollisionAvoided_ConsecutiveAuthored(t *testing.T) {
	vg, err := parseSvg(
		`<svg viewBox="0 0 100 100">` +
			`<defs>` +
			`<clipPath id="__use_clip_1">` +
			`<rect x="0" y="0" width="5" height="5"/>` +
			`</clipPath>` +
			`<clipPath id="__use_clip_2">` +
			`<rect x="0" y="0" width="6" height="6"/>` +
			`</clipPath>` +
			`<symbol id="s" viewBox="0 0 10 10" ` +
			`preserveAspectRatio="xMidYMid slice">` +
			`<rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" width="40" height="20"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := vg.clipPaths["__use_clip_1"]; !ok {
		t.Fatal("authored __use_clip_1 missing")
	}
	if _, ok := vg.clipPaths["__use_clip_2"]; !ok {
		t.Fatal("authored __use_clip_2 missing")
	}
	if _, taken := vg.clipPaths["__use_clip_3"]; !taken {
		t.Fatal("synth slice clip should land on __use_clip_3 " +
			"after skipping authored 1 and 2")
	}
	if len(vg.Paths) != 1 {
		t.Fatalf("got %d paths; want 1", len(vg.Paths))
	}
	if vg.Paths[0].clipPathID != "__use_clip_3" {
		t.Fatalf("Paths[0].ClipPathID=%q; want __use_clip_3",
			vg.Paths[0].clipPathID)
	}
}

// width-only <use>: missing height falls back to symbol viewBox height
// (mirrors symbolViewportScale). Synth clip box must use that height.
func TestExpandUseSymbolPreserveSlice_WidthOnlyUsesViewBoxHeight(t *testing.T) {
	vg, err := parseSvg(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 7" ` +
			`preserveAspectRatio="xMidYMid slice">` +
			`<rect width="10" height="7"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" width="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var clipID string
	for id := range vg.clipPaths {
		if strings.HasPrefix(id, "__use_clip_") {
			clipID = id
		}
	}
	if clipID == "" {
		t.Fatal("expected synth __use_clip_* with width-only <use>")
	}
	rect := vg.clipPaths[clipID][0]
	w := rect.Segments[1].Points[0] - rect.Segments[0].Points[0]
	h := rect.Segments[2].Points[1] - rect.Segments[1].Points[1]
	if w != 40 {
		t.Errorf("clip width=%v; want 40 (use width)", w)
	}
	if h != 7 {
		t.Errorf("clip height=%v; want 7 (viewBox fallback)", h)
	}
}

// height-only <use>: missing width falls back to symbol viewBox width.
func TestExpandUseSymbolPreserveSlice_HeightOnlyUsesViewBoxWidth(t *testing.T) {
	vg, err := parseSvg(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 5 10" ` +
			`preserveAspectRatio="xMidYMid slice">` +
			`<rect width="5" height="10"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var clipID string
	for id := range vg.clipPaths {
		if strings.HasPrefix(id, "__use_clip_") {
			clipID = id
		}
	}
	if clipID == "" {
		t.Fatal("expected synth __use_clip_* with height-only <use>")
	}
	rect := vg.clipPaths[clipID][0]
	w := rect.Segments[1].Points[0] - rect.Segments[0].Points[0]
	h := rect.Segments[2].Points[1] - rect.Segments[1].Points[1]
	if w != 5 {
		t.Errorf("clip width=%v; want 5 (viewBox fallback)", w)
	}
	if h != 40 {
		t.Errorf("clip height=%v; want 40 (use height)", h)
	}
}

// NaN viewBox dimensions must be rejected by mintUseSliceClipID — no
// synth clip emitted, expansion still produces the <g>.
func TestExpandUseSymbolPreserveSlice_NaNViewBoxRejected(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 NaN NaN" ` +
			`preserveAspectRatio="xMidYMid slice">` +
			`<rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" width="20" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatal("expected synthesized <g>")
	}
	if cp := g.AttrMap["clip-path"]; cp != "" {
		t.Fatalf("NaN viewBox must not produce clip; got %q", cp)
	}
}

// Inf viewBox dimensions likewise rejected — boundedScale + finiteF32
// gate fails fast in the harden path.
func TestExpandUseSymbolPreserveSlice_InfViewBoxRejected(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 Inf 10" ` +
			`preserveAspectRatio="xMidYMid slice">` +
			`<rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="0" y="0" width="20" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatal("expected synthesized <g>")
	}
	if cp := g.AttrMap["clip-path"]; cp != "" {
		t.Fatalf("Inf viewBox must not produce clip; got %q", cp)
	}
}

// viewBox origin offsets must be undone by a translate so the symbol's
// (vbX,vbY) maps to (0,0) in the use's local frame before scaling.
func TestExpandUseSymbolViewBoxOriginCompensated(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="3 4 10 10"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" width="20" height="20"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if !strings.Contains(tr, "translate(-3,-4)") {
		t.Fatalf("expected translate(-3,-4) viewBox compensation: %q", tr)
	}
}

// Author-supplied <use x="..."> must not splice raw into the
// transform attribute: a value like `0)scale(99)` would otherwise
// inject extra transforms. positioningTransform parses x/y as
// numbers, so injection material gets dropped.
func TestExpandUseRejectsXYInjection(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg><defs><symbol id="s"><rect width="1" height="1"/></symbol></defs>` +
			`<use href="#s" x="0)scale(99)" y="0"/></svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if strings.Contains(tr, "scale(99") {
		t.Fatalf("injected scale leaked into transform: %q", tr)
	}
	if strings.Contains(tr, ")scale") || strings.Contains(tr, "))") {
		t.Fatalf("malformed transform from injection attempt: %q", tr)
	}
}

// Pathological viewBox dimensions (tiny but positive) against author
// width/height must not emit absurd scale factors that blow up
// downstream geometry.
func TestExpandUseSymbolClampsAbsurdScale(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 0.0000001 0.0000001"><rect/></symbol></defs>` +
			`<use href="#s" width="100" height="100"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if strings.Contains(tr, "scale(") {
		t.Fatalf("absurd scale must be clamped (no scale emitted): %q", tr)
	}
}

// Percentage on <use x|y> must not be silently treated as raw number.
func TestExpandUseRejectsPercentageXY(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg><defs><symbol id="s"><rect width="1" height="1"/></symbol></defs>` +
			`<use href="#s" x="50%" y="25%"/></svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if strings.Contains(tr, "translate(50") || strings.Contains(tr, "translate(25") {
		t.Fatalf("percentage x/y treated as raw number: %q", tr)
	}
}

// Percentage width/height on <use> resolve against the parent
// viewport, which positioningTransform cannot reach. Treating "50%"
// as raw 50 would silently mis-scale, so the synthesizer must skip
// scaling instead.
func TestExpandUseSymbolSkipsPercentageWidthHeight(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="2" y="3" width="50%" height="50%"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if strings.Contains(tr, "scale(") {
		t.Fatalf("percentage width/height must not produce numeric scale: %q", tr)
	}
	if !strings.Contains(tr, "translate(2,3)") {
		t.Fatalf("expected translate(2,3) preserved: %q", tr)
	}
}

// viewBox with zero or negative dimensions must fall back without
// dividing by zero or emitting an Inf scale.
func TestExpandUseSymbolZeroViewBoxDimsFallsBack(t *testing.T) {
	cases := []string{
		`<svg><defs><symbol id="s" viewBox="0 0 0 10"><rect/></symbol></defs><use href="#s" x="1" y="2" width="20" height="20"/></svg>`,
		`<svg><defs><symbol id="s" viewBox="0 0 10 0"><rect/></symbol></defs><use href="#s" x="1" y="2" width="20" height="20"/></svg>`,
		`<svg><defs><symbol id="s" viewBox="0 0 -5 10"><rect/></symbol></defs><use href="#s" x="1" y="2" width="20" height="20"/></svg>`,
	}
	for i, svg := range cases {
		root, err := decodeSvgTree(svg)
		if err != nil {
			t.Fatalf("case %d decode: %v", i, err)
		}
		expandUseElements(root)
		g := findFirstByName(root, "g")
		if g == nil {
			t.Fatalf("case %d: expected synthesized <g>", i)
		}
		tr := g.AttrMap["transform"]
		if strings.Contains(tr, "scale(") {
			t.Fatalf("case %d: zero/neg viewBox dim must skip scale: %q", i, tr)
		}
		if !strings.Contains(tr, "translate(1,2)") {
			t.Fatalf("case %d: translate must still be emitted: %q", i, tr)
		}
	}
}

// width="" alone must default to viewBox width and still produce a
// valid scale based on height.
func TestExpandUseSymbolHonorsHeightOnly(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" height="40"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	// height=40 vs vb=10. width defaults to vb=10 → raw sx=1. Default
	// preserveAspectRatio is xMidYMid meet → uniform min = 1; the
	// extra height becomes a center-Y offset of (40-10)/2 = 15. With
	// no x/y the leading translate carries (0,15). scale(1) collapses
	// to identity in the emitter, so only the translate appears.
	if !strings.Contains(tr, "translate(0,15)") {
		t.Fatalf("expected translate(0,15) with height-only meet: %q", tr)
	}
}

// Lowercase `viewbox` (HTML-authored SVG) must drive scaling too —
// the parser falls back to lowercase at the dimension lookup.
func TestExpandUseSymbolLowercaseViewBoxScales(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg>` +
			`<defs><symbol id="s" viewbox="0 0 10 10"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" width="30" height="30"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	// 30x30 use of 10x10 symbol → uniform 3x; emitter collapses
	// scale(s,s) to scale(s).
	if !strings.Contains(tr, "scale(3)") {
		t.Fatalf("expected scale(3) from lowercase viewbox: %q", tr)
	}
}

// Garbage viewBox tokens yield <4 numbers; must fall back without
// emitting scale.
func TestExpandUseSymbolGarbageViewBoxFallsBack(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg>` +
			`<defs><symbol id="s" viewBox="abc def"><rect/></symbol></defs>` +
			`<use href="#s" x="1" y="2" width="20" height="20"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if strings.Contains(tr, "scale(") {
		t.Fatalf("garbage viewBox must not produce scale: %q", tr)
	}
}

// <use> without width/height (and non-symbol targets) keeps the
// pre-existing translate-only behavior.
func TestExpandUseNoWidthHeightUsesTranslateOnly(t *testing.T) {
	root, err := decodeSvgTree(
		`<svg viewBox="0 0 100 100">` +
			`<defs><symbol id="s" viewBox="0 0 10 10"><rect width="10" height="10"/></symbol></defs>` +
			`<use href="#s" x="2" y="3"/>` +
			`</svg>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	expandUseElements(root)
	g := findFirstByName(root, "g")
	if g == nil {
		t.Fatalf("expected synthesized <g>")
	}
	tr := g.AttrMap["transform"]
	if strings.Contains(tr, "scale(") {
		t.Fatalf("expected no scale without use width/height: %q", tr)
	}
}

// --- helpers ---

func findFirstByName(n *xmlNode, name string) *xmlNode {
	for i := range n.Children {
		c := &n.Children[i]
		if c.Name == name {
			return c
		}
		if r := findFirstByName(c, name); r != nil {
			return r
		}
	}
	return nil
}

func collectByName(n *xmlNode, name string) []*xmlNode {
	var out []*xmlNode
	walk(n, func(c *xmlNode) {
		if c.Name == name {
			out = append(out, c)
		}
	})
	return out
}

func countByName(n *xmlNode, name string) int {
	count := 0
	walk(n, func(c *xmlNode) {
		if c.Name == name {
			count++
		}
	})
	return count
}

// <use> expansion must preserve mixed-content Tail data on cloned
// children so a referenced <text> with `<text>A <tspan>B</tspan> C`
// keeps the trailing "C" run after expansion.
func TestUseExpandPreservesMixedContentTail(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">` +
		`<defs><symbol id="s">` +
		`<text x="0" y="10">A <tspan>B</tspan> C</text>` +
		`</symbol></defs>` +
		`<use href="#s"/></svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"A": false, "B": false, "C": false}
	for _, txt := range vg.Texts {
		if _, ok := want[txt.Text]; ok {
			want[txt.Text] = true
		}
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("missing run %q after <use> expansion", k)
		}
	}
}

func walk(n *xmlNode, fn func(*xmlNode)) {
	for i := range n.Children {
		c := &n.Children[i]
		fn(c)
		walk(c, fn)
	}
}
