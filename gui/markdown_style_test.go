package gui

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui/highlight"
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

// Inline code keeps its own size (two points under body) in body text,
// so an identifier in a sentence does not stand out from the words
// around it, and follows the heading size inside a heading.
func TestCodeRunSizeFollowsContext(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	style := DefaultMarkdownStyle()

	body := styleMdRun(markdown.Run{Text: "c", Format: markdown.FormatCode},
		style.Text, style)
	if want := style.Text.Size - 2; body.Style.Size != want {
		t.Errorf("code-in-body size = %v, want body-2 %v",
			body.Style.Size, want)
	}

	headerBase := style.Text
	headerBase.Size = 22
	head := styleMdRun(markdown.Run{Text: "c", Format: markdown.FormatCode},
		headerBase, style)
	if head.Style.Size != 22 {
		t.Errorf("code-in-header size = %v, want 22", head.Style.Size)
	}
}

// H4 and a definition term are bold at body size. A size-only check
// took them for body text, so their code drew two points smaller and
// regular next to bold body-size words. Code there must match the
// size and weight of the text around it.
func TestCodeRunInBoldBodySizeBlockKeepsSizeAndWeight(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	style := DefaultMarkdownStyle()

	for _, tc := range []struct {
		name string
		base TextStyle
	}{
		{"H4", mdHeaderStyle(4, style)},
		{"def term", style.Bold},
	} {
		got := styleMdRun(markdown.Run{Text: "c", Format: markdown.FormatCode},
			tc.base, style)
		if got.Style.Size != tc.base.Size {
			t.Errorf("%s: code size = %v, want %v",
				tc.name, got.Style.Size, tc.base.Size)
		}
		if got.Style.Typeface != tc.base.Typeface {
			t.Errorf("%s: code typeface = %v, want %v",
				tc.name, got.Style.Typeface, tc.base.Typeface)
		}
	}
}

// Inline code sits on the same fill as a fenced block, so both read as
// code. A fenced block's runs carry no fill of their own: the block
// already paints CodeBlockBG behind them, and a per-run fill would draw
// darker strips over it.
func TestInlineCodeTakesCodeBlockBG(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	style := DefaultMarkdownStyle()
	style.CodeBlockBG = RGB(1, 2, 3)

	got := styleMdRun(markdown.Run{Text: "c", Format: markdown.FormatCode},
		style.Text, style)
	if got.Style.BgColor != style.CodeBlockBG {
		t.Errorf("inline code bg = %v, want CodeBlockBG %v",
			got.Style.BgColor, style.CodeBlockBG)
	}

	for _, hl := range []highlight.Highlighter{nil, highlight.Default()} {
		style.CodeHighlighter = hl
		mb := styleMdBlock(markdown.Block{
			IsCode:       true,
			CodeLanguage: "go",
			Runs: []markdown.Run{{
				Text:      "x := 1",
				Format:    markdown.FormatCode,
				CodeToken: markdown.TokenPlain,
			}},
		}, style)
		for _, r := range mb.Content.Runs {
			if r.Style.BgColor.IsSet() {
				t.Errorf("highlighter %v: fenced run %q has bg %v",
					hl != nil, r.Text, r.Style.BgColor)
			}
		}
	}
}

// The parser folds **`x`** into plain FormatCode (mergeFormat), so
// bold-wrapped code in body text draws regular like any inline code.
// This pins that on purpose: the CHANGELOG records it.
func TestBoldWrappedCodeIsRegular(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	style := DefaultMarkdownStyle()

	blocks := markdown.Parse("**`x`**", false)
	if len(blocks) == 0 || len(blocks[0].Runs) == 0 {
		t.Fatal("no runs")
	}
	mb := styleMdBlock(blocks[0], style)
	for _, r := range mb.Content.Runs {
		if r.Text == "x" && r.Style.Typeface == glyph.TypefaceBold {
			t.Error("bold-wrapped code run is bold")
		}
	}
}

// Inline code is set apart by its mono face alone. A bold mono run in a
// paragraph full of identifiers (the DX cheat sheet) reads louder than
// the prose it sits in. Fenced blocks share the run style, so they stay
// regular too.
func TestCodeRunIsRegularWeight(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	style := DefaultMarkdownStyle()

	got := styleMdRun(markdown.Run{Text: "c", Format: markdown.FormatCode},
		style.Text, style)
	if got.Style.Typeface != style.Code.Typeface {
		t.Errorf("code run typeface = %v, want the Code style's %v",
			got.Style.Typeface, style.Code.Typeface)
	}
	if got.Style.Typeface == glyph.TypefaceBold {
		t.Error("code run is bold")
	}
}

// A fenced block's plain runs go through the same code run style, and
// with a highlighter the whole block takes its base from the first run.
// Both paths draw regular weight, not bold.
func TestFencedCodeBlockIsRegularWeight(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)

	for _, tc := range []struct {
		name string
		hl   highlight.Highlighter
	}{
		{"no highlighter", nil},
		{"highlighter", highlight.Default()},
	} {
		style := DefaultMarkdownStyle()
		style.CodeHighlighter = tc.hl
		mb := styleMdBlock(markdown.Block{
			IsCode:       true,
			CodeLanguage: "go",
			Runs: []markdown.Run{{
				Text:      "x := 1",
				Format:    markdown.FormatCode,
				CodeToken: markdown.TokenPlain,
			}},
		}, style)
		if len(mb.Content.Runs) == 0 {
			t.Fatalf("%s: no runs", tc.name)
		}
		for _, r := range mb.Content.Runs {
			if r.Style.Typeface == glyph.TypefaceBold {
				t.Errorf("%s: run %q is bold", tc.name, r.Text)
			}
		}
	}
}

// A fenced-block comment drew in the border color: contrast 1.24 on
// light themes (near-white on near-white) and 1.37 on dark ones, not
// merely low but invisible. Comments are de-emphasized text, so the
// color is the theme's secondary role, which is contrast-matched per
// polarity. Fails on the old ColorBorder for every preset.
func TestMarkdownCommentColorUsesSecondaryRole(t *testing.T) {
	restore := CurrentTheme()
	defer applyTheme(&restore)

	for _, name := range ThemeRegisteredNames() {
		theme, ok := ThemeGet(name)
		if !ok {
			t.Fatalf("theme %q registered but not retrievable", name)
		}
		applyTheme(&theme)
		style := DefaultMarkdownStyle()
		if !style.codeCommentColor.eq(theme.TextStyleSecondary.Color) {
			t.Errorf("%s: comment %v, want secondary %v",
				name, style.codeCommentColor, theme.TextStyleSecondary.Color)
		}
		if style.codeCommentColor.eq(theme.ColorBorder) {
			t.Errorf("%s: comment equals the border color %v",
				name, theme.ColorBorder)
		}
	}
}

// TestMarkdownCommentReadable states the consequence as a number: the
// comment clears AA on the code-block fill it actually sits on, in
// every preset. The fill is the translucent CodeBlockBG wash over the
// theme panel, so both layers are composited before the ratio.
func TestMarkdownCommentReadable(t *testing.T) {
	const wantAA = 4.5
	restore := CurrentTheme()
	defer applyTheme(&restore)

	for _, name := range ThemeRegisteredNames() {
		theme, ok := ThemeGet(name)
		if !ok {
			t.Fatalf("theme %q registered but not retrievable", name)
		}
		applyTheme(&theme)
		style := DefaultMarkdownStyle()
		bg := style.CodeBlockBG.Over(theme.ColorPanel)
		fg := style.codeCommentColor.Over(bg)
		if got := contrastRatio(fg, bg); got < wantAA {
			t.Errorf("%s: comment %v on code fill %v is contrast "+
				"%.2f, want >= %.2f", name, fg, bg, got, wantAA)
		}
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

// Markdown is document prose, so paragraph text reads at the theme's
// body role, not one rung below it (#998). The headings shift with it:
// H4 takes body bold, H5 and H6 the two rungs under body. Inline code
// keeps the mono +1 step over the text it sits in; fenced blocks stay
// on the small code role so long lines wrap less.
func TestDefaultMarkdownStyleUsesBodyRoles(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	th := CurrentTheme()
	style := DefaultMarkdownStyle()

	cases := []struct {
		name      string
		got, want float32
	}{
		{"Text", style.Text.Size, th.TextStyleBody.Size},
		{"Bold", style.Bold.Size, th.TextStyleBody.Size},
		{"Italic", style.Italic.Size, th.TextStyleBody.Size},
		{"BoldItalic", style.BoldItalic.Size, th.TextStyleBody.Size},
		{"H3", style.H3.Size, th.TextStyleTitleSmall.Size},
		{"H4", style.H4.Size, th.TextStyleBody.Size},
		{"H5", style.H5.Size, th.TextStyleBodySmall.Size},
		{"H6", style.H6.Size, th.TextStyleCaption.Size},
		{"Code", style.Code.Size, th.TextStyleBody.Size - 2},
		{"codeBlockText", style.codeBlockText.Size, th.TextStyleCodeSmall.Size},
		{"tableCellStyle", style.tableCellStyle.Size, th.TextStyleBody.Size},
		{"tableHeadStyle", style.tableHeadStyle.Size, th.TextStyleBody.Size},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s size = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// A MarkdownCfg with no Style must render like one given
// DefaultMarkdownStyle(): every Cfg is zero-initializable. Before #998
// the zero style reached the renderer as-is, so every run had size 0
// (go-glyph then guessed a size) while list bullets, plain Text with
// a zero style, drew at the 14 fallback.
func TestMarkdownUnsetStyleUsesDefault(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)
	want := DefaultMarkdownStyle().Text.Size

	w := &Window{}
	layout := generateViewLayout(w.Markdown(MarkdownCfg{
		Source: "Plain **bold** text.\n\n- item\n",
	}), w)

	var runs, bullets int
	var walk func(l *Layout)
	walk = func(l *Layout) {
		if tc := l.Shape.TC; tc != nil {
			if tc.rtf != nil && tc.rtf.rTFRuns != nil {
				for _, r := range tc.rtf.rTFRuns.Runs {
					runs++
					if r.Style.Size != want {
						t.Errorf("run %q size = %v, want %v",
							r.Text, r.Style.Size, want)
					}
				}
			}
			if tc.Text == "• " && tc.TextStyle != nil {
				bullets++
				if tc.TextStyle.Size != want {
					t.Errorf("bullet size = %v, want %v",
						tc.TextStyle.Size, want)
				}
			}
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	walk(&layout)
	if runs == 0 || bullets == 0 {
		t.Fatalf("walk found %d runs, %d bullets; want both > 0",
			runs, bullets)
	}
}

// The unset-style fallback resolves per generation, never into the
// view's own cfg. Written back, the second generation of one view would
// see a set style, skip the fallback and keep the first theme's faces
// after a theme flip. The second theme states a different body size so
// a stale style shows as a wrong run size.
func TestMarkdownUnsetStyleFollowsThemeFlip(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)

	bigCfg := themeDarkCfg
	bigCfg.TextStyleDef.Size = 18
	bigCfg.SizeTextMedium = 18
	big := ThemeMaker(bigCfg)

	w := &Window{}
	v := w.Markdown(MarkdownCfg{Source: "Plain **bold** text."})

	sizes := func() []float32 {
		layout := generateViewLayout(v, w)
		var out []float32
		var walk func(l *Layout)
		walk = func(l *Layout) {
			if tc := l.Shape.TC; tc != nil && tc.rtf != nil &&
				tc.rtf.rTFRuns != nil {
				for _, r := range tc.rtf.rTFRuns.Runs {
					out = append(out, r.Style.Size)
				}
			}
			for i := range l.Children {
				walk(&l.Children[i])
			}
		}
		walk(&layout)
		return out
	}

	for _, s := range sizes() {
		if s != ThemeDark.TextStyleBody.Size {
			t.Fatalf("dark run size = %v, want %v",
				s, ThemeDark.TextStyleBody.Size)
		}
	}
	SetTheme(big)
	got := sizes()
	if len(got) == 0 {
		t.Fatal("no runs after theme flip")
	}
	for _, s := range got {
		if s != big.TextStyleBody.Size {
			t.Errorf("run size after flip = %v, want %v",
				s, big.TextStyleBody.Size)
		}
	}
}

// countingHighlighter records Tokenize calls so a test can see whether
// the style that reached block building still carried it.
type countingHighlighter struct{ calls int }

func (c *countingHighlighter) Tokenize(_, _ string) []highlight.Token {
	c.calls++
	return nil
}

// The unset-style fallback swaps in DefaultMarkdownStyle(), which has no
// highlighter. A caller who set only CodeHighlighter on an otherwise
// empty style must keep it, or fenced code silently loses its colors.
func TestMarkdownUnsetStyleKeepsCodeHighlighter(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)

	hl := &countingHighlighter{}
	w := &Window{}
	generateViewLayout(w.Markdown(MarkdownCfg{
		Source: "```go\npackage main\n```\n",
		Style:  MarkdownStyle{CodeHighlighter: hl},
	}), w)
	if hl.calls == 0 {
		t.Error("CodeHighlighter on an unset style was dropped")
	}
}
