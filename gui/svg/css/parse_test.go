package css

import (
	"reflect"
	"testing"
	"time"
)

func firstCompound(cs complexSelector) compound {
	return cs.parts[len(cs.parts)-1].compound
}

func TestParseStylesheet_Simple(t *testing.T) {
	rules := parseStylesheet(`rect { fill: red; stroke: #00ff00 }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d, want 1", len(rules))
	}
	r := rules[0]
	if len(r.selectors) != 1 || firstCompound(r.selectors[0]).Tag != "rect" {
		t.Fatalf("selector: %+v", r.selectors)
	}
	if len(r.Decls) != 2 {
		t.Fatalf("decls: %d, want 2", len(r.Decls))
	}
	if r.Decls[0].Name != "fill" || r.Decls[0].Value != "red" {
		t.Fatalf("decl[0]: %+v", r.Decls[0])
	}
	if r.Decls[1].Name != "stroke" || r.Decls[1].Value != "#00ff00" {
		t.Fatalf("decl[1]: %+v", r.Decls[1])
	}
}

func TestParseStylesheet_Compound(t *testing.T) {
	rules := parseStylesheet(`circle.dot#a, .b { fill: blue }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	sels := rules[0].selectors
	if len(sels) != 2 {
		t.Fatalf("selectors: %d", len(sels))
	}
	c0 := firstCompound(sels[0])
	if c0.Tag != "circle" || c0.ID != "a" ||
		!reflect.DeepEqual(c0.Classes, []string{"dot"}) {
		t.Errorf("compound: %+v", c0)
	}
	if sels[0].spec != ([3]uint16{1, 1, 1}) {
		t.Errorf("specificity: %v", sels[0].spec)
	}
	c1 := firstCompound(sels[1])
	if c1.Tag != "" || !reflect.DeepEqual(c1.Classes, []string{"b"}) {
		t.Errorf("class-only: %+v", c1)
	}
	if sels[1].spec != ([3]uint16{0, 1, 0}) {
		t.Errorf("class-only spec: %v", sels[1].spec)
	}
}

func TestParseStylesheet_Important(t *testing.T) {
	rules := parseStylesheet(`.x { fill: red !important; stroke: blue }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	d := rules[0].Decls
	if len(d) != 2 {
		t.Fatalf("decls: %d", len(d))
	}
	if !d[0].Important || d[0].Value != "red" {
		t.Errorf("decl 0: %+v", d[0])
	}
	if d[1].Important || d[1].Value != "blue" {
		t.Errorf("decl 1: %+v", d[1])
	}
}

func TestParseStylesheet_Combinators(t *testing.T) {
	rules := parseStylesheet(`g circle { fill: red } a > .b { stroke: blue }`, ParseOptions{})
	if len(rules) != 2 {
		t.Fatalf("rules: %d", len(rules))
	}
	desc := rules[0].selectors[0]
	if len(desc.parts) != 2 ||
		desc.parts[1].combinator != combDescendant ||
		desc.parts[0].compound.Tag != "g" ||
		desc.parts[1].compound.Tag != "circle" {
		t.Errorf("descendant: %+v", desc.parts)
	}
	child := rules[1].selectors[0]
	if len(child.parts) != 2 ||
		child.parts[1].combinator != combChild ||
		child.parts[0].compound.Tag != "a" ||
		child.parts[1].compound.Classes[0] != "b" {
		t.Errorf("child: %+v", child.parts)
	}
}

func TestParseStylesheet_Universal(t *testing.T) {
	rules := parseStylesheet(`* { opacity: 0.5 }`, ParseOptions{})
	if len(rules) != 1 || firstCompound(rules[0].selectors[0]).Tag != "*" {
		t.Fatalf("universal: %+v", rules)
	}
	if rules[0].selectors[0].spec != ([3]uint16{0, 0, 0}) {
		t.Errorf("universal spec: %v", rules[0].selectors[0].spec)
	}
}

func TestParseStylesheet_RootAndNthChild(t *testing.T) {
	rules := parseStylesheet(`:root { fill: red }
		rect:nth-child(2n+1) { fill: blue }`, ParseOptions{})
	if len(rules) != 2 {
		t.Fatalf("rules: %d", len(rules))
	}
	if !firstCompound(rules[0].selectors[0]).Root {
		t.Errorf(":root not parsed: %+v", rules[0].selectors[0])
	}
	c := firstCompound(rules[1].selectors[0])
	if c.nthChild == nil || c.nthChild.A != 2 || c.nthChild.B != 1 {
		t.Errorf("nth-child not parsed: %+v", c)
	}
}

func TestMatch_BasicAndSpecificity(t *testing.T) {
	rules := parseStylesheet(`
		rect { fill: red }
		.cls { fill: blue }
		#id { fill: green }
	`, ParseOptions{})
	got := Match(rules, ElementInfo{
		Tag:     "rect",
		ID:      "id",
		Classes: []string{"cls"},
	}, nil, nil)
	if len(got) != 3 {
		t.Fatalf("matched: %d", len(got))
	}
	SortCascade(got)
	if got[0].Value != "red" || got[1].Value != "blue" ||
		got[2].Value != "green" {
		t.Errorf("cascade order: %+v", got)
	}
}

func TestMatch_ImportantBeatsSpecificity(t *testing.T) {
	rules := parseStylesheet(`
		#id { fill: green }
		.cls { fill: blue !important }
	`, ParseOptions{})
	got := Match(rules, ElementInfo{
		ID:      "id",
		Classes: []string{"cls"},
	}, nil, nil)
	SortCascade(got)
	if got[len(got)-1].Value != "blue" || !got[len(got)-1].Important {
		t.Errorf("!important didn't win: %+v", got)
	}
}

func TestMatch_SourceOrderTiebreak(t *testing.T) {
	rules := parseStylesheet(`.a { fill: red } .a { fill: blue }`, ParseOptions{})
	got := Match(rules, ElementInfo{Classes: []string{"a"}}, nil, nil)
	SortCascade(got)
	if got[len(got)-1].Value != "blue" {
		t.Errorf("source order tiebreak failed: %+v", got)
	}
}

func TestMatch_DescendantCombinator(t *testing.T) {
	rules := parseStylesheet(`g circle { fill: red }`, ParseOptions{})
	ancestors := []ElementInfo{
		{Tag: "svg"},
		{Tag: "g"},
	}
	got := Match(rules, ElementInfo{Tag: "circle"}, ancestors, nil)
	if len(got) != 1 || got[0].Value != "red" {
		t.Errorf("descendant match failed: %+v", got)
	}
	none := Match(rules, ElementInfo{Tag: "circle"},
		[]ElementInfo{{Tag: "svg"}}, nil)
	if len(none) != 0 {
		t.Errorf("descendant matched without g ancestor: %+v", none)
	}
}

func TestMatch_ChildCombinator(t *testing.T) {
	rules := parseStylesheet(`g > circle { fill: red }`, ParseOptions{})
	got := Match(rules, ElementInfo{Tag: "circle"},
		[]ElementInfo{{Tag: "svg"}, {Tag: "g"}}, nil)
	if len(got) != 1 {
		t.Errorf("child match failed: %+v", got)
	}
	// Indirect — fails child combinator.
	none := Match(rules, ElementInfo{Tag: "circle"},
		[]ElementInfo{{Tag: "g"}, {Tag: "a"}}, nil)
	if len(none) != 0 {
		t.Errorf("child shouldn't match indirect: %+v", none)
	}
}

func TestMatch_NthChild(t *testing.T) {
	rules := parseStylesheet(`rect:nth-child(odd) { fill: red }`, ParseOptions{})
	got := Match(rules, ElementInfo{Tag: "rect", Index: 1}, nil, nil)
	if len(got) != 1 {
		t.Errorf("nth-child(odd) idx=1: %+v", got)
	}
	none := Match(rules, ElementInfo{Tag: "rect", Index: 2}, nil, nil)
	if len(none) != 0 {
		t.Errorf("nth-child(odd) idx=2: %+v", none)
	}
}

func TestMatch_Root(t *testing.T) {
	rules := parseStylesheet(`:root { fill: red }`, ParseOptions{})
	got := Match(rules, ElementInfo{Tag: "svg", IsRoot: true}, nil, nil)
	if len(got) != 1 {
		t.Errorf(":root match: %+v", got)
	}
	none := Match(rules, ElementInfo{Tag: "g", IsRoot: false}, nil, nil)
	if len(none) != 0 {
		t.Errorf(":root matched non-root: %+v", none)
	}
}

func TestParseDeclaration_CustomProp(t *testing.T) {
	rules := parseStylesheet(`:root { --brand: blue; fill: var(--brand) }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	d := rules[0].Decls
	if len(d) != 2 {
		t.Fatalf("decls: %d", len(d))
	}
	if !d[0].CustomProp || d[0].Name != "--brand" || d[0].Value != "blue" {
		t.Errorf("custom prop: %+v", d[0])
	}
	if d[1].CustomProp || d[1].Value != "var(--brand)" {
		t.Errorf("var ref: %+v", d[1])
	}
}

func TestParseStylesheet_IsPseudoExpand(t *testing.T) {
	rules := parseStylesheet(
		`:is(#a, #b) { fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	sels := rules[0].selectors
	if len(sels) != 2 {
		t.Fatalf("selectors: %d, want 2", len(sels))
	}
	if firstCompound(sels[0]).ID != "a" {
		t.Errorf("sel[0] id: %+v", firstCompound(sels[0]))
	}
	if firstCompound(sels[1]).ID != "b" {
		t.Errorf("sel[1] id: %+v", firstCompound(sels[1]))
	}
}

func TestParseStylesheet_IsPseudoCompound(t *testing.T) {
	rules := parseStylesheet(
		`circle:is(.front, .back) { stroke: blue }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	sels := rules[0].selectors
	if len(sels) != 2 {
		t.Fatalf("selectors: %d, want 2", len(sels))
	}
	c0 := firstCompound(sels[0])
	if c0.Tag != "circle" || len(c0.Classes) != 1 || c0.Classes[0] != "front" {
		t.Errorf("sel[0]: %+v", c0)
	}
	c1 := firstCompound(sels[1])
	if c1.Tag != "circle" || len(c1.Classes) != 1 || c1.Classes[0] != "back" {
		t.Errorf("sel[1]: %+v", c1)
	}
}

func TestParseStylesheet_VendorPrefixProp(t *testing.T) {
	rules := parseStylesheet(
		`.x { -webkit-animation: spin 1s; -moz-fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	d := rules[0].Decls
	if len(d) != 2 {
		t.Fatalf("decls: %d", len(d))
	}
	if d[0].Name != "animation" {
		t.Errorf("d[0] name: %q", d[0].Name)
	}
	if d[1].Name != "fill" {
		t.Errorf("d[1] name: %q", d[1].Name)
	}
}

func TestParseStylesheet_VendorPrefixKeyframes(t *testing.T) {
	ss := ParseFull(
		`@-webkit-keyframes spin { from { opacity: 0 } to { opacity: 1 } }`,
		ParseOptions{})
	if len(ss.Keyframes) != 1 {
		t.Fatalf("keyframes: %d", len(ss.Keyframes))
	}
	if ss.Keyframes[0].Name != "spin" {
		t.Errorf("name: %q", ss.Keyframes[0].Name)
	}
}

func TestParseStylesheet_LineCommentStripped(t *testing.T) {
	src := `.x { fill: red; // trailing comment
		stroke: blue }`
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	if len(rules[0].Decls) != 2 {
		t.Fatalf("decls: %d, want 2", len(rules[0].Decls))
	}
	if rules[0].Decls[1].Name != "stroke" {
		t.Errorf("d[1]: %+v", rules[0].Decls[1])
	}
}

func TestParseStylesheet_LineCommentPreservesURL(t *testing.T) {
	src := `.x { background: url(http://example.com/x.png) }`
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 1 || len(rules[0].Decls) != 1 {
		t.Fatalf("rules: %+v", rules)
	}
	if !contains(rules[0].Decls[0].Value, "http://example.com") {
		t.Errorf("URL stripped: %q", rules[0].Decls[0].Value)
	}
}

// Recoverable parse errors (e.g. a stray ":" mid-rule, as seen in some
// svg-spinners stylesheets) must not abort parsing — the surrounding
// rule's good declarations should still reach the cascade.
func TestParseStylesheet_RecoversFromGarbageBetweenRules(t *testing.T) {
	src := `.a { fill: red } : { } .b { fill: blue }`
	rules := parseStylesheet(src, ParseOptions{})
	var sawA, sawB bool
	for _, r := range rules {
		for _, sel := range r.selectors {
			tag := firstCompound(sel).Classes
			for _, cls := range tag {
				if cls == "a" {
					sawA = true
				}
				if cls == "b" {
					sawB = true
				}
			}
		}
	}
	if !sawA || !sawB {
		t.Fatalf("recovery dropped good rules: sawA=%v sawB=%v rules=%+v",
			sawA, sawB, rules)
	}
}

// Pathological/stuck input must terminate (no-progress break).
// Smoke test: any binary blob should not hang ParseFull.
func TestParseFull_TerminatesOnGarbage(t *testing.T) {
	inputs := []string{
		"",
		"{",
		"}",
		":::::",
		string([]byte{0, 1, 2, 3, 0xff, 0xfe}),
		"@@@@",
		"a { ; ; ; }",
	}
	done := make(chan struct{})
	go func() {
		for _, in := range inputs {
			_ = ParseFull(in, ParseOptions{})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ParseFull did not terminate on pathological input")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
