package svg

import "testing"

// Two non-contiguous elements that share the same filter must produce
// two distinct svgFilteredGroups in document order — not be merged into
// one offscreen buffer (which would composite them at the wrong z) and
// not be ordered by map iteration (nondeterministic across runs).
func TestParseSvg_SameFilterOnTwoElementsYieldsTwoGroups(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<filter id="b"><feGaussianBlur stdDeviation="1"/></filter>
		</defs>
		<rect id="a" x="0" y="0" width="10" height="10" filter="url(#b)"/>
		<rect id="mid" x="20" y="0" width="10" height="10"/>
		<rect id="c" x="40" y="0" width="10" height="10" filter="url(#b)"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(vg.FilteredGroups); got != 2 {
		t.Fatalf("FilteredGroups=%d; want 2 (one per filter occurrence)", got)
	}
	for i, g := range vg.FilteredGroups {
		if g.FilterID != "b" {
			t.Errorf("group %d FilterID=%q; want %q", i, g.FilterID, "b")
		}
		if len(g.Paths) != 1 {
			t.Errorf("group %d paths=%d; want 1", i, len(g.Paths))
		}
	}
	if vg.FilteredGroups[0].GroupKey == vg.FilteredGroups[1].GroupKey {
		t.Fatalf("group keys collide: %d", vg.FilteredGroups[0].GroupKey)
	}
	if vg.FilteredGroups[0].GroupKey >= vg.FilteredGroups[1].GroupKey {
		t.Fatalf("groups out of document order: keys %d,%d",
			vg.FilteredGroups[0].GroupKey, vg.FilteredGroups[1].GroupKey)
	}
}

// filter declared via inline style="" must reach FilteredGroups, not
// be silently dropped. Same applies to clip-path via style.
func TestParseSvg_FilterFromInlineStyle(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<filter id="b"><feGaussianBlur stdDeviation="1"/></filter>
		</defs>
		<rect x="0" y="0" width="10" height="10" style="filter:url(#b)"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(vg.FilteredGroups); got != 1 {
		t.Fatalf("FilteredGroups=%d; want 1", got)
	}
	if vg.FilteredGroups[0].FilterID != "b" {
		t.Fatalf("FilterID=%q; want b", vg.FilteredGroups[0].FilterID)
	}
}

func TestParseSvg_FilterFromCSSRule(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<filter id="b"><feGaussianBlur stdDeviation="1"/></filter>
		</defs>
		<style>.blurred { filter: url(#b); }</style>
		<rect class="blurred" x="0" y="0" width="10" height="10"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(vg.FilteredGroups); got != 1 {
		t.Fatalf("FilteredGroups=%d; want 1", got)
	}
	if vg.FilteredGroups[0].FilterID != "b" {
		t.Fatalf("FilterID=%q; want b", vg.FilteredGroups[0].FilterID)
	}
}

// clip-path via inline style and CSS rule must populate
// path.ClipPathID after the cascade.
func TestParseSvg_ClipPathFromInlineStyle(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<clipPath id="cp"><rect x="0" y="0" width="50" height="50"/></clipPath>
		</defs>
		<rect x="0" y="0" width="100" height="100" style="clip-path:url(#cp)"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) == 0 {
		t.Fatal("no paths parsed")
	}
	if vg.Paths[0].clipPathID != "cp" {
		t.Fatalf("ClipPathID=%q; want cp", vg.Paths[0].clipPathID)
	}
}

func TestParseSvg_ClipPathFromCSSRule(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<clipPath id="cp"><rect x="0" y="0" width="50" height="50"/></clipPath>
		</defs>
		<style>.clipped { clip-path: url(#cp); }</style>
		<rect class="clipped" x="0" y="0" width="100" height="100"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) == 0 {
		t.Fatal("no paths parsed")
	}
	if vg.Paths[0].clipPathID != "cp" {
		t.Fatalf("ClipPathID=%q; want cp", vg.Paths[0].clipPathID)
	}
}

// inline style="clip-path:url(#a)" must override the bare
// clip-path attribute per cascade origin precedence.
func TestParseSvg_InlineStyleClipPathOverridesAttr(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<clipPath id="a"><rect x="0" y="0" width="50" height="50"/></clipPath>
			<clipPath id="b"><rect x="0" y="0" width="80" height="80"/></clipPath>
		</defs>
		<rect x="0" y="0" width="100" height="100"
			clip-path="url(#a)" style="clip-path:url(#b)"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) == 0 {
		t.Fatal("no paths parsed")
	}
	if got := vg.Paths[0].clipPathID; got != "b" {
		t.Fatalf("ClipPathID=%q; inline style should override attr → want b", got)
	}
}

// Child redeclaring the SAME filter id its parent already inherits
// must still allocate its own FilterGroupKey — every authored decl
// is a distinct occurrence and renders to its own offscreen buffer.
// Pure-inheritance children share the parent's group.
func TestParseSvg_ChildRedeclaresParentFilterGetsOwnGroup(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<filter id="b"><feGaussianBlur stdDeviation="1"/></filter>
		</defs>
		<g filter="url(#b)">
			<rect id="inh" x="0" y="0" width="10" height="10"/>
			<rect id="redecl" x="20" y="0" width="10" height="10" filter="url(#b)"/>
		</g>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(vg.FilteredGroups); got != 2 {
		t.Fatalf("FilteredGroups=%d; want 2 "+
			"(inheritor shares parent group; redeclarer gets own)", got)
	}
	if vg.FilteredGroups[0].GroupKey == vg.FilteredGroups[1].GroupKey {
		t.Fatalf("redeclared filter shares parent's group key %d",
			vg.FilteredGroups[0].GroupKey)
	}
}

// clip-path: none via CSS rule must wipe the inherited clip — the
// cascade can clear, not just set.
func TestParseSvg_ClipPathNoneViaCSSWipesInherited(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<clipPath id="cp"><rect x="0" y="0" width="50" height="50"/></clipPath>
		</defs>
		<style>.bare { clip-path: none; }</style>
		<g clip-path="url(#cp)">
			<rect class="bare" x="0" y="0" width="100" height="100"/>
		</g>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) == 0 {
		t.Fatal("no paths parsed")
	}
	if got := vg.Paths[0].clipPathID; got != "" {
		t.Fatalf("ClipPathID=%q; clip-path:none should clear → want \"\"", got)
	}
}

// filter: none via CSS rule must wipe the inherited filter — symmetric
// with clip-path:none. Without the wipe the child renders into the
// parent's offscreen buffer and gets blurred twice.
func TestParseSvg_FilterNoneViaCSSWipesInherited(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<filter id="b"><feGaussianBlur stdDeviation="1"/></filter>
		</defs>
		<style>.unblur { filter: none; }</style>
		<g filter="url(#b)">
			<rect class="unblur" x="0" y="0" width="10" height="10"/>
		</g>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(vg.FilteredGroups); got != 0 {
		t.Fatalf("FilteredGroups=%d; child cleared filter → want 0", got)
	}
	if len(vg.Paths) == 0 {
		t.Fatal("no paths parsed")
	}
	if vg.Paths[0].FilterID != "" {
		t.Fatalf("FilterID=%q; want \"\"", vg.Paths[0].FilterID)
	}
	if vg.Paths[0].FilterGroupKey != 0 {
		t.Fatalf("FilterGroupKey=%d; filter:none must reset → want 0",
			vg.Paths[0].FilterGroupKey)
	}
}

// Filter set via inline style on a child of a filter-inheriting parent
// must trigger fresh group key (authored decl, not inheritance).
func TestParseSvg_FilterFromInlineStyleOnRedeclaringChildGetsOwnGroup(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<filter id="b"><feGaussianBlur stdDeviation="1"/></filter>
		</defs>
		<g filter="url(#b)">
			<rect id="inh" x="0" y="0" width="10" height="10"/>
			<rect id="redecl" x="20" y="0" width="10" height="10"
				style="filter:url(#b)"/>
		</g>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(vg.FilteredGroups); got != 2 {
		t.Fatalf("FilteredGroups=%d; want 2 (inline-style redeclare gets own)",
			got)
	}
	if vg.FilteredGroups[0].GroupKey == vg.FilteredGroups[1].GroupKey {
		t.Fatalf("inline-style redeclare shares parent group key %d",
			vg.FilteredGroups[0].GroupKey)
	}
}

// Invalid filter declaration (e.g. `filter: bogus`) must be ignored
// per CSS rather than allocating its own per-occurrence group buffer.
// Regression: the cascade used to mark the child as authored on
// property name alone, which then minted a fresh FilterGroupKey for
// a declaration that contributed no actual filter.
func TestParseSvg_InvalidFilterDeclarationIgnored(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<filter id="b"><feGaussianBlur stdDeviation="1"/></filter>
		</defs>
		<g filter="url(#b)">
			<rect id="inh"    x="0"  y="0" width="10" height="10"/>
			<rect id="bogus" x="20" y="0" width="10" height="10"
				style="filter: bogus"/>
		</g>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(vg.FilteredGroups); got != 1 {
		t.Fatalf("FilteredGroups=%d; invalid filter must be ignored, "+
			"both rects share inherited parent group → want 1", got)
	}
}

// `clip-path: url()` (empty id, parseFillURL fails) must be ignored
// the same as `clip-path: bogus`. Without the value-validation gate,
// the cascade would still mark the declaration authored even though
// no usable id was supplied.
func TestParseSvg_EmptyURLClipPathTreatedAsInvalid(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<clipPath id="cp"><rect x="0" y="0" width="50" height="50"/></clipPath>
		</defs>
		<g clip-path="url(#cp)">
			<svg x="0" y="0" width="40" height="40"
				style="clip-path: url()">
				<rect x="0" y="0" width="100" height="100"/>
			</svg>
		</g>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) == 0 {
		t.Fatal("no paths parsed")
	}
	got := vg.Paths[0].clipPathID
	if got == "" || got == "cp" {
		t.Fatalf("ClipPathID=%q; nested svg with empty url() clip-path "+
			"must receive synth viewport clip", got)
	}
}

// Invalid clip-path declaration on a nested <svg> must not suppress
// the synthesized viewport clip. Regression: AuthoredClipPath was
// flipped on property name alone, so an invalid value left the flag
// set while ClipPathID remained inherited from the parent — the
// nested-svg synth-clip gate read that as "author already clipped"
// and skipped the spec-required viewport clip.
func TestParseSvg_InvalidClipPathOnNestedSvgKeepsViewportClip(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 100 100">
		<defs>
			<clipPath id="cp"><rect x="0" y="0" width="50" height="50"/></clipPath>
		</defs>
		<g clip-path="url(#cp)">
			<svg x="0" y="0" width="40" height="40"
				style="clip-path: bogus">
				<rect x="0" y="0" width="100" height="100"/>
			</svg>
		</g>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) == 0 {
		t.Fatal("no paths parsed")
	}
	got := vg.Paths[0].clipPathID
	if got == "" || got == "cp" {
		t.Fatalf("ClipPathID=%q; nested svg with invalid clip-path "+
			"must receive synth viewport clip (not inherited cp, not empty)",
			got)
	}
}
