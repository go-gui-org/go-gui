package css

import (
	"strings"
	"testing"
)

// Tests for v0.14.0 selector additions: sibling combinators,
// attribute selectors, :hover / :focus / :not().

func TestMatch_AdjacentSibling(t *testing.T) {
	rules := parseStylesheet(`rect + circle { fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	siblings := []ElementInfo{{Tag: "rect"}}
	got := Match(rules, ElementInfo{Tag: "circle"}, nil, siblings)
	if len(got) != 1 || got[0].Value != "red" {
		t.Fatalf("adjacent match: %+v", got)
	}
	// Non-adjacent: rect, then g, then circle. circle's previous
	// sibling is g, not rect, so + must fail.
	none := Match(rules, ElementInfo{Tag: "circle"}, nil,
		[]ElementInfo{{Tag: "rect"}, {Tag: "g"}})
	if len(none) != 0 {
		t.Fatalf("adjacent matched non-adjacent: %+v", none)
	}
}

func TestMatch_GeneralSibling(t *testing.T) {
	rules := parseStylesheet(`rect ~ circle { fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	got := Match(rules, ElementInfo{Tag: "circle"}, nil,
		[]ElementInfo{{Tag: "rect"}, {Tag: "g"}})
	if len(got) != 1 {
		t.Fatalf("general sibling failed: %+v", got)
	}
	none := Match(rules, ElementInfo{Tag: "circle"}, nil,
		[]ElementInfo{{Tag: "g"}})
	if len(none) != 0 {
		t.Fatalf("general sibling matched without rect: %+v", none)
	}
}

func TestMatch_AttrExists(t *testing.T) {
	rules := parseStylesheet(`[data-x] { fill: red }`, ParseOptions{})
	got := Match(rules, ElementInfo{
		Attrs: map[string]string{"data-x": "anything"},
	}, nil, nil)
	if len(got) != 1 {
		t.Fatalf("[data-x] match: %+v", got)
	}
	none := Match(rules, ElementInfo{Attrs: map[string]string{"other": "1"}},
		nil, nil)
	if len(none) != 0 {
		t.Fatalf("[data-x] matched without attr: %+v", none)
	}
}

func TestMatch_AttrEqual(t *testing.T) {
	rules := parseStylesheet(`[data-state=active] { fill: red }`, ParseOptions{})
	got := Match(rules, ElementInfo{
		Attrs: map[string]string{"data-state": "active"},
	}, nil, nil)
	if len(got) != 1 {
		t.Fatalf("attr equal: %+v", got)
	}
	none := Match(rules, ElementInfo{
		Attrs: map[string]string{"data-state": "inactive"},
	}, nil, nil)
	if len(none) != 0 {
		t.Fatalf("attr equal matched wrong value: %+v", none)
	}
}

func TestMatch_AttrOps(t *testing.T) {
	cases := []struct {
		sel, attr, val string
		want           bool
	}{
		{`[c~=foo]`, "c", "foo bar", true},
		{`[c~=foo]`, "c", "foobar", false},
		{`[c|=en]`, "c", "en", true},
		{`[c|=en]`, "c", "en-US", true},
		{`[c|=en]`, "c", "english", false},
		{`[c^=pre]`, "c", "prefix", true},
		{`[c^=pre]`, "c", "post", false},
		{`[c$=fix]`, "c", "prefix", true},
		{`[c$=fix]`, "c", "prefer", false},
		{`[c*=mid]`, "c", "amidship", true},
		{`[c*=mid]`, "c", "no", false},
	}
	for _, tc := range cases {
		rules := parseStylesheet(tc.sel+` { fill: red }`, ParseOptions{})
		if len(rules) != 1 {
			t.Fatalf("%s: parse failed", tc.sel)
		}
		got := Match(rules, ElementInfo{
			Attrs: map[string]string{tc.attr: tc.val},
		}, nil, nil)
		if (len(got) > 0) != tc.want {
			t.Errorf("%s vs %q: got=%v want=%v",
				tc.sel, tc.val, len(got) > 0, tc.want)
		}
	}
}

func TestMatch_AttrQuotedValue(t *testing.T) {
	rules := parseStylesheet(`[d="hello world"] { fill: red }`, ParseOptions{})
	got := Match(rules, ElementInfo{
		Attrs: map[string]string{"d": "hello world"},
	}, nil, nil)
	if len(got) != 1 {
		t.Fatalf("quoted value: %+v", got)
	}
}

func TestMatch_HoverFocus(t *testing.T) {
	rules := parseStylesheet(
		`circle:hover { fill: red } rect:focus { fill: blue }`,
		ParseOptions{})
	circHover := Match(rules,
		ElementInfo{Tag: "circle", State: matchState{Hover: true}},
		nil, nil)
	if len(circHover) != 1 || circHover[0].Value != "red" {
		t.Errorf(":hover match: %+v", circHover)
	}
	circNoHover := Match(rules, ElementInfo{Tag: "circle"}, nil, nil)
	if len(circNoHover) != 0 {
		t.Errorf(":hover matched without state: %+v", circNoHover)
	}
	rectFocus := Match(rules,
		ElementInfo{Tag: "rect", State: matchState{Focus: true}},
		nil, nil)
	if len(rectFocus) != 1 || rectFocus[0].Value != "blue" {
		t.Errorf(":focus match: %+v", rectFocus)
	}
}

func TestMatch_Not(t *testing.T) {
	rules := parseStylesheet(`circle:not(.hidden) { fill: red }`,
		ParseOptions{})
	got := Match(rules, ElementInfo{Tag: "circle"}, nil, nil)
	if len(got) != 1 {
		t.Errorf(":not basic: %+v", got)
	}
	none := Match(rules, ElementInfo{
		Tag:     "circle",
		Classes: []string{"hidden"},
	}, nil, nil)
	if len(none) != 0 {
		t.Errorf(":not should exclude .hidden: %+v", none)
	}
}

func TestMatch_NotSpecificity(t *testing.T) {
	// :not(.a) contributes a class-tier specificity bump just like
	// .a would.
	rules := parseStylesheet(`:not(.a) { fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	if got := rules[0].selectors[0].spec; got != ([3]uint16{0, 1, 0}) {
		t.Errorf(":not specificity: %v", got)
	}
}

func TestParse_AttrSpecificity(t *testing.T) {
	rules := parseStylesheet(`[a=b] { fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	if got := rules[0].selectors[0].spec; got != ([3]uint16{0, 1, 0}) {
		t.Errorf("attr specificity: %v", got)
	}
}

func TestMatch_AttrSelectorOnNilAttrs(t *testing.T) {
	// Selector requires an attribute; element has nil Attrs map.
	// matchAttr lookup fails on nil → compound rejects.
	rules := parseStylesheet(`[data-x] { fill: red }`, ParseOptions{})
	got := Match(rules, ElementInfo{Tag: "rect"}, nil, nil)
	if len(got) != 0 {
		t.Errorf("nil Attrs should not match attr selector: %+v", got)
	}
}

func TestMatch_MultipleAttrConstraints(t *testing.T) {
	rules := parseStylesheet(
		`[data-a=1][data-b=2] { fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("compound attr parse: %d rules", len(rules))
	}
	// Both attrs match.
	got := Match(rules, ElementInfo{
		Attrs: map[string]string{"data-a": "1", "data-b": "2"},
	}, nil, nil)
	if len(got) != 1 {
		t.Errorf("both attrs should match: %+v", got)
	}
	// One attr missing → reject.
	none := Match(rules, ElementInfo{
		Attrs: map[string]string{"data-a": "1"},
	}, nil, nil)
	if len(none) != 0 {
		t.Errorf("partial match should not pass: %+v", none)
	}
}

func TestParse_AttrSelMalformedDropped(t *testing.T) {
	cases := []string{
		`[] { fill: red }`,
		`[=v] { fill: red }`,
		`[name=] { fill: red }`,
	}
	for _, src := range cases {
		rules := parseStylesheet(src, ParseOptions{})
		if len(rules) != 0 {
			t.Errorf("%q should be dropped: got %d rules", src, len(rules))
		}
	}
}

func TestMatches_EmptyPartsReturnsFalse(t *testing.T) {
	// Defensive guard: a hand-built ComplexSelector with no parts must
	// not match anything.
	cs := complexSelector{}
	if cs.Matches(ElementInfo{Tag: "rect"}, nil, nil) {
		t.Error("empty Parts should not match")
	}
}

func TestMatch_NotCombinedWithAdjacentSibling(t *testing.T) {
	rules := parseStylesheet(
		`rect + circle:not(.skip) { fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("parse: %d", len(rules))
	}
	// Adjacent rect, circle without .skip → match.
	got := Match(rules, ElementInfo{Tag: "circle"}, nil,
		[]ElementInfo{{Tag: "rect"}})
	if len(got) != 1 {
		t.Errorf("adjacent + :not should match: %+v", got)
	}
	// Adjacent rect, circle WITH .skip → :not rejects.
	none := Match(rules, ElementInfo{
		Tag: "circle", Classes: []string{"skip"},
	}, nil, []ElementInfo{{Tag: "rect"}})
	if len(none) != 0 {
		t.Errorf(":not(.skip) should reject: %+v", none)
	}
}

func TestMatch_HoverWithSiblingCombinator(t *testing.T) {
	rules := parseStylesheet(
		`rect + circle:hover { fill: red }`, ParseOptions{})
	got := Match(rules, ElementInfo{
		Tag:   "circle",
		State: matchState{Hover: true},
	}, nil, []ElementInfo{{Tag: "rect"}})
	if len(got) != 1 {
		t.Errorf("rect + circle:hover w/ hover: %+v", got)
	}
	// No hover → rejects despite sibling combinator matching.
	none := Match(rules, ElementInfo{Tag: "circle"}, nil,
		[]ElementInfo{{Tag: "rect"}})
	if len(none) != 0 {
		t.Errorf("missing hover should reject: %+v", none)
	}
}

func TestParse_NotDepthCap(t *testing.T) {
	// Hostile asset: deeply nested :not(). Beyond maxNotDepth the
	// parser must reject the rule rather than recursing without
	// bound. We build maxNotDepth+2 levels of nesting.
	src := strings.Repeat(":not(", maxNotDepth+2) + ".x" +
		strings.Repeat(")", maxNotDepth+2) + " { fill: red }"
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 0 {
		t.Errorf("over-deep :not() should be dropped, got %d rules",
			len(rules))
	}
	// Sanity: a small number of nestings still parses.
	ok := parseStylesheet(":not(:not(.a)) { fill: red }", ParseOptions{})
	if len(ok) != 1 {
		t.Errorf("shallow nested :not() should parse: %d", len(ok))
	}
}

func TestParse_SiblingCombinatorRoundtrip(t *testing.T) {
	rules := parseStylesheet(
		`a + b ~ c { fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	parts := rules[0].selectors[0].parts
	if len(parts) != 3 {
		t.Fatalf("parts: %d", len(parts))
	}
	if parts[1].combinator != combAdjacent {
		t.Errorf("expected adjacent combinator, got %d", parts[1].combinator)
	}
	if parts[2].combinator != combGeneralSibling {
		t.Errorf("expected general sibling, got %d", parts[2].combinator)
	}
}
