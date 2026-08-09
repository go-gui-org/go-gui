package css

import "testing"

func TestParseStylesheet_IsPseudoNested(t *testing.T) {
	rules := parseStylesheet(
		`:is(:is(#a, #b), #c) { fill: red }`, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	sels := rules[0].selectors
	if len(sels) != 3 {
		t.Fatalf("selectors: %d, want 3", len(sels))
	}
	got := map[string]bool{}
	for _, s := range sels {
		got[firstCompound(s).ID] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !got[want] {
			t.Errorf("missing #%s; got %v", want, got)
		}
	}
}

func TestParseStylesheet_IsExpansionUnderCap(t *testing.T) {
	src := `:is(:is(:is(#a, #b), #c), #d) { fill: red }`
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	if len(rules[0].selectors) != 4 {
		t.Fatalf("selectors: %d, want 4", len(rules[0].selectors))
	}
}

func TestParseStylesheet_CommaInsideFunction(t *testing.T) {
	src := `:is(.a, .b), #c { fill: red }`
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	if len(rules[0].selectors) != 3 {
		t.Fatalf("selectors: %d, want 3", len(rules[0].selectors))
	}
}

func TestStripLineComments_SingleQuotedString(t *testing.T) {
	src := `.x { content: '//keep me' }`
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 1 || len(rules[0].Decls) != 1 {
		t.Fatalf("rules: %+v", rules)
	}
	if !contains(rules[0].Decls[0].Value, "//keep me") {
		t.Errorf("string body lost: %q", rules[0].Decls[0].Value)
	}
}

// Escaped char inside a string must advance two bytes — otherwise a
// `'` after `\` closes the string and `//` would start a comment.
func TestStripLineComments_EscapedQuote(t *testing.T) {
	src := `.x { content: '\'//still inside' }`
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 1 || len(rules[0].Decls) != 1 {
		t.Fatalf("rules: %+v", rules)
	}
	if !contains(rules[0].Decls[0].Value, "//still inside") {
		t.Errorf("escape mishandled: %q", rules[0].Decls[0].Value)
	}
}

func TestStripLineComments_EOF(t *testing.T) {
	src := ".x { fill: red } // trailing"
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	if rules[0].Decls[0].Name != "fill" {
		t.Errorf("decl: %+v", rules[0].Decls[0])
	}
}

// CRLF line endings: stripper terminates on \r as well as \n,
// otherwise the rest of the file becomes one big comment.
func TestStripLineComments_CRLF(t *testing.T) {
	src := ".x { fill: red; // bye\r\n stroke: blue }"
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	if len(rules[0].Decls) != 2 {
		t.Fatalf("decls: %d, want 2", len(rules[0].Decls))
	}
	if rules[0].Decls[1].Name != "stroke" {
		t.Errorf("decl[1]: %+v", rules[0].Decls[1])
	}
}

func TestStripLineComments_HTTPSPreserved(t *testing.T) {
	src := `.x { background: url(https://example.com/y.png) }`
	rules := parseStylesheet(src, ParseOptions{})
	if len(rules) != 1 || len(rules[0].Decls) != 1 {
		t.Fatalf("rules: %+v", rules)
	}
	if !contains(rules[0].Decls[0].Value, "https://example.com") {
		t.Errorf("URL stripped: %q", rules[0].Decls[0].Value)
	}
}

func TestStripVendorPrefix_AllPrefixes(t *testing.T) {
	cases := []struct{ in, want string }{
		{"-webkit-animation", "animation"},
		{"-moz-fill", "fill"},
		{"-ms-stroke", "stroke"},
		{"-o-opacity", "opacity"},
		{"animation", "animation"},
		{"", ""},
	}
	for _, c := range cases {
		if got := StripVendorPrefix(c.in); got != c.want {
			t.Errorf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}
