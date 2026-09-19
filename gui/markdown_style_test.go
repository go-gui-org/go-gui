package gui

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui/markdown"
)

// Sup/sub runs keep the base size and gain their feature tag: the
// shaper synthesizes size and shift from the run size, so scaling here
// would double-shrink. Each run gets a fresh Features so mutating one
// run cannot corrupt the others.
func TestScriptRunsKeepSizeAndMergeFeatures(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	style := DefaultMarkdownStyle()
	base := style.Text
	base.Features = &glyph.FontFeatures{
		OpenTypeFeatures: []glyph.FontFeature{{Tag: "smcp", Value: 1}},
	}

	sup := styleMdRun(markdown.Run{Text: "2", Superscript: true},
		base, style)
	if sup.Style.Size != base.Size {
		t.Errorf("sup size = %v, want base %v (shaper scales)",
			sup.Style.Size, base.Size)
	}
	assertFeatureTags(t, sup.Style.Features, "smcp", "sups")

	sub := styleMdRun(markdown.Run{Text: "2", Subscript: true},
		base, style)
	if sub.Style.Size != base.Size {
		t.Errorf("sub size = %v, want base %v (shaper scales)",
			sub.Style.Size, base.Size)
	}
	assertFeatureTags(t, sub.Style.Features, "smcp", "subs")

	if sup.Style.Features == sub.Style.Features {
		t.Error("sup and sub runs share one Features pointer")
	}
	if len(base.Features.OpenTypeFeatures) != 1 {
		t.Errorf("base Features mutated: %+v", base.Features)
	}
}

func assertFeatureTags(
	t *testing.T, feats *glyph.FontFeatures, want ...string,
) {
	t.Helper()
	if feats == nil {
		t.Fatalf("Features is nil, want %v", want)
	}
	got := make(map[string]bool, len(feats.OpenTypeFeatures))
	for _, f := range feats.OpenTypeFeatures {
		got[f.Tag] = true
	}
	for _, tag := range want {
		if !got[tag] {
			t.Errorf("Features %+v misses tag %q", feats, tag)
		}
	}
}

// A bold run inside a heading keeps the heading size and color; the
// style face supplies only the weight.
func TestBoldRunInHeaderKeepsHeaderGeometry(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	style := DefaultMarkdownStyle()
	headerColor := RGB(10, 20, 30)
	base := TextStyle{Size: 22, Color: headerColor}

	got := styleMdRun(markdown.Run{Text: "b", Format: markdown.FormatBold},
		base, style)
	if got.Style.Size != 22 {
		t.Errorf("bold-in-header size = %v, want 22", got.Style.Size)
	}
	if got.Style.Color != headerColor {
		t.Errorf("bold-in-header color = %v, want %v",
			got.Style.Color, headerColor)
	}
}

// A custom Bold color still applies to body runs: the base color wins
// only outside body text.
func TestBoldRunInBodyKeepsStyleColor(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	style := DefaultMarkdownStyle()
	style.Bold.Color = RGB(200, 10, 10)

	got := styleMdRun(markdown.Run{Text: "b", Format: markdown.FormatBold},
		style.Text, style)
	if got.Style.Color != RGB(200, 10, 10) {
		t.Errorf("bold-in-body color = %v, want custom red",
			got.Style.Color)
	}
}

// Inline code keeps its mono size in body text but follows the heading
// size inside a heading.
func TestCodeRunSizeFollowsContext(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	style := DefaultMarkdownStyle()

	body := styleMdRun(markdown.Run{Text: "c", Format: markdown.FormatCode},
		style.Text, style)
	if body.Style.Size != style.Code.Size {
		t.Errorf("code-in-body size = %v, want %v",
			body.Style.Size, style.Code.Size)
	}

	headerBase := style.Text
	headerBase.Size = 22
	head := styleMdRun(markdown.Run{Text: "c", Format: markdown.FormatCode},
		headerBase, style)
	if head.Style.Size != 22 {
		t.Errorf("code-in-header size = %v, want 22", head.Style.Size)
	}
}

// A hostile fenced block keeps the parser's runs: the highlighter never
// sees input over the cap.
func TestHighlightCodeBlockSizeCap(t *testing.T) {
	big := strings.Repeat("x", maxHighlightBytes+1)
	existing := []RichTextRun{{Text: big, Style: TextStyle{Size: 14}}}
	style := MarkdownStyle{}
	if got := highlightCodeBlock(existing, "go", &style); got != nil {
		t.Errorf("over-cap block retokenized into %d runs", len(got))
	}
}

// A nil base yields just the tag; a base that already carries the tag
// is not duplicated, and its disabled value is switched on.
func TestMdWithScriptFeatureNilAndExistingTag(t *testing.T) {
	got := mdWithScriptFeature(nil, "sups")
	if len(got.OpenTypeFeatures) != 1 ||
		got.OpenTypeFeatures[0] != (glyph.FontFeature{Tag: "sups", Value: 1}) {
		t.Errorf("nil base = %+v, want only sups=1", got)
	}

	base := &glyph.FontFeatures{
		OpenTypeFeatures: []glyph.FontFeature{{Tag: "sups", Value: 0}},
	}
	got = mdWithScriptFeature(base, "sups")
	if len(got.OpenTypeFeatures) != 1 || got.OpenTypeFeatures[0].Value != 1 {
		t.Errorf("existing tag = %+v, want one sups=1", got)
	}
	if base.OpenTypeFeatures[0].Value != 0 {
		t.Error("base Features mutated")
	}
}
