package svg

import "testing"

// --- unescapeAttrEntities ---

func TestUnescapeAttrEntities_NoEntities(t *testing.T) {
	got := unescapeAttrEntities("hello world")
	if got != "hello world" {
		t.Errorf("got %q want %q", got, "hello world")
	}
}

func TestUnescapeAttrEntities_Amp(t *testing.T) {
	got := unescapeAttrEntities("foo&amp;bar")
	if got != "foo&bar" {
		t.Errorf("got %q want %q", got, "foo&bar")
	}
}

func TestUnescapeAttrEntities_Lt(t *testing.T) {
	got := unescapeAttrEntities("a&lt;b")
	if got != "a<b" {
		t.Errorf("got %q want %q", got, "a<b")
	}
}

func TestUnescapeAttrEntities_Gt(t *testing.T) {
	got := unescapeAttrEntities("a&gt;b")
	if got != "a>b" {
		t.Errorf("got %q want %q", got, "a>b")
	}
}

func TestUnescapeAttrEntities_Quot(t *testing.T) {
	got := unescapeAttrEntities(`&quot;val&quot;`)
	if got != `"val"` {
		t.Errorf("got %q want %q", got, `"val"`)
	}
}

func TestUnescapeAttrEntities_Apos(t *testing.T) {
	got := unescapeAttrEntities("it&#39;s")
	if got != "it's" {
		t.Errorf("got %q want %q", got, "it's")
	}
}

func TestUnescapeAttrEntities_Mixed(t *testing.T) {
	input := "&amp;lt;&lt;&gt;&amp;quot;&quot;&#39;"
	want := "&lt;<>&quot;\"'"
	got := unescapeAttrEntities(input)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestUnescapeAttrEntities_UnknownEntityPassThrough(t *testing.T) {
	// An entity not in our 5-entity set passes through unchanged.
	got := unescapeAttrEntities("&unknown;entity")
	if got != "&unknown;entity" {
		t.Errorf("got %q want %q", got, "&unknown;entity")
	}
}

func TestUnescapeAttrEntities_Empty(t *testing.T) {
	got := unescapeAttrEntities("")
	if got != "" {
		t.Errorf("got %q want empty", got)
	}
}

func TestUnescapeAttrEntities_OnlyAmpersand(t *testing.T) {
	// Lone & without known entity suffix passes through.
	got := unescapeAttrEntities("price & value")
	if got != "price & value" {
		t.Errorf("got %q want %q", got, "price & value")
	}
}

func TestUnescapeAttrEntities_Consecutive(t *testing.T) {
	got := unescapeAttrEntities("&amp;&amp;")
	if got != "&&" {
		t.Errorf("got %q want %q", got, "&&")
	}
}

func TestUnescapeAttrEntities_RoundTrip(t *testing.T) {
	// XML decoder hands back entity-decoded values; the re-encoder
	// (buildOpenTag) escapes them again; findAttr must restore. This
	// test verifies the 5-entity round-trip.
	original := `x & y < z > "q" 'a'`
	// Simulate what buildOpenTag's writeAttrEscaped produces:
	escaped := "x &amp; y &lt; z &gt; &quot;q&quot; &#39;a&#39;"
	got := unescapeAttrEntities(escaped)
	if got != original {
		t.Errorf("round-trip failed:\n  got  %q\n  want %q", got, original)
	}
}
