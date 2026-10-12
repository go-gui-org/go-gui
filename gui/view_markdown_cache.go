package gui

// view_markdown_cache.go keys the markdown styled-block cache.
//
// The cached blocks hold resolved run styles, so the key names the
// style as well as the source: two documents with the same source and
// different styles share nothing, and restyling a document with an
// unchanged source rebuilds its blocks (issue #1000). The key hashes
// every MarkdownStyle field, including geometry-only ones such as
// blockSpacing: hashing a field that cannot move a run only misses
// the cache after a real style edit (safe, just a re-parse), while
// omitting a field that can moves the bug. A new field that the
// fingerprint ignores fails TestMarkdownStyleFingerprintCoversLeaf.

import (
	"reflect"
	"unsafe"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui/highlight"
	"github.com/go-gui-org/go-gui/gui/markdown"
)

// markdownCacheKey names one styled-block cache entry: the source
// hash, the fingerprint of the style that styled it, and the
// highlighter's dynamic type. Comparable, so it keys the window's
// BoundedMap directly. The type keeps two views that differ only in
// highlighter kind (chroma beside a custom one) in separate entries;
// without it, each view's Set would evict the other's entry and both
// would re-parse every frame. reflect.Type is comparable for any
// highlighter, where the interface value itself is not: a key holding
// a slice-bearing highlighter would panic on map lookup.
type markdownCacheKey struct {
	source      uint64
	style       uint64
	highlighter reflect.Type
}

// newMarkdownCacheKey builds the key for one source under one style.
func newMarkdownCacheKey(
	source string, s *MarkdownStyle,
) markdownCacheKey {
	return markdownCacheKey{
		source:      uint64(markdown.MathHash(source)),
		style:       markdownStyleFingerprint(s),
		highlighter: reflect.TypeOf(s.CodeHighlighter),
	}
}

// markdownCacheEntry is one cached parse: the styled blocks plus the
// highlighter that tokenized them. The key carries only the
// highlighter's type; two highlighters of one type with different
// token tables compare apart here, so they never share blocks.
type markdownCacheEntry struct {
	highlighter highlight.Highlighter
	blocks      []markdownBlock
}

// sameMarkdownHighlighter reports whether two highlighters tokenize
// alike. Nil matches nil only. The cheap checks run first, so the
// common cases cost no heap:
//
//  1. Different dynamic types never match.
//  2. The same interface data word is the same value: an interface
//     copy shares its data word, and the value behind it never
//     changes. This covers a pointer highlighter and an app that
//     reuses one style value across frames.
//  3. A comparable value compares with ==. Value.Comparable checks
//     the dynamic contents, so an interface field holding a slice
//     falls through instead of panicking.
//  4. Anything else compares deep. reflect.DeepEqual allocates for a
//     slice-bearing value (two allocs a frame for a stub rebuilt each
//     frame) and reports false for a set func, so a func-bearing
//     value rebuilt each frame re-parses each frame. Neither case
//     shares wrong blocks.
func sameMarkdownHighlighter(
	a, b highlight.Highlighter,
) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false
	}
	if ifaceData(a) == ifaceData(b) {
		return true
	}
	if reflect.ValueOf(a).Comparable() {
		return a == b
	}
	return reflect.DeepEqual(a, b)
}

// ifaceData returns the data word of a non-empty interface. Two
// values of one dynamic type with the same data word are the same
// value. The caller checks the types first.
func ifaceData(h highlight.Highlighter) unsafe.Pointer {
	return (*[2]unsafe.Pointer)(unsafe.Pointer(&h))[1]
}

// markdownStyleFingerprint hashes every MarkdownStyle field into an
// FNV-1a digest. Over-approximation is deliberate (see the file
// header): when a field is added, it is hashed here and covered by
// the leaf test in the same pass.
func markdownStyleFingerprint(s *MarkdownStyle) uint64 {
	h := Fnv64Offset
	h = fnvMarkdownTextStyle(h, s.Text)
	h = fnvMarkdownTextStyle(h, s.H1)
	h = fnvMarkdownTextStyle(h, s.H2)
	h = fnvMarkdownTextStyle(h, s.H3)
	h = fnvMarkdownTextStyle(h, s.H4)
	h = fnvMarkdownTextStyle(h, s.H5)
	h = fnvMarkdownTextStyle(h, s.H6)
	h = fnvMarkdownTextStyle(h, s.Bold)
	h = fnvMarkdownTextStyle(h, s.Italic)
	h = fnvMarkdownTextStyle(h, s.BoldItalic)
	h = fnvMarkdownTextStyle(h, s.Code)
	h = fnvMarkdownTextStyle(h, s.codeBlockText)
	h = fnvMarkdownTextStyle(h, s.tableHeadStyle)
	h = fnvMarkdownTextStyle(h, s.tableCellStyle)
	h = fnvMarkdownBool(h, s.CodeHighlighter != nil)
	h = fnvMarkdownColorPtr(h, s.tableRowAlt)
	h = fnvU64(h, uint64(s.mathDPIDisplay))
	h = fnvU64(h, uint64(s.mathDPIInline))
	h = fnvMarkdownPadding(h, s.codeBlockPadding)
	h = fnvMarkdownPadding(h, s.tableCellPadding)
	h = fnvU64(h, uint64(normFloat32Bits(s.blockSpacing)))
	h = fnvU64(h, uint64(normFloat32Bits(s.nestIndent)))
	h = fnvU64(h, uint64(normFloat32Bits(s.prefixCharWidth)))
	h = fnvU64(h, uint64(normFloat32Bits(s.codeBlockRadius)))
	h = fnvU64(h, uint64(normFloat32Bits(s.tableBorderSize)))
	h = fnvMarkdownColor(h, s.CodeBlockBG)
	h = fnvMarkdownColor(h, s.codeKeywordColor)
	h = fnvMarkdownColor(h, s.codeStringColor)
	h = fnvMarkdownColor(h, s.codeNumberColor)
	h = fnvMarkdownColor(h, s.codeCommentColor)
	h = fnvMarkdownColor(h, s.codeOperatorColor)
	h = fnvMarkdownColor(h, s.codeTypeColor)
	h = fnvMarkdownColor(h, s.codeFunctionColor)
	h = fnvMarkdownColor(h, s.codeBuiltinColor)
	h = fnvMarkdownColor(h, s.hRColor)
	h = fnvMarkdownColor(h, s.linkColor)
	h = fnvMarkdownColor(h, s.blockquoteBorder)
	h = fnvMarkdownColor(h, s.blockquoteBG)
	h = fnvMarkdownColor(h, s.tableBorderColor)
	h = fnvMarkdownColor(h, s.highlightBG)
	h = fnvMarkdownColor(h, s.mermaidBG)
	h = fnvMarkdownBool(h, s.h1Separator)
	h = fnvMarkdownBool(h, s.h2Separator)
	h = Fnv64Byte(h, byte(s.TableBorderStyle))
	h = fnvMarkdownBool(h, s.hardLineBreaks)
	return h
}

// fnvMarkdownBool mixes one flag. The separators around variable
// length fields live at their call sites; a bare flag is one byte,
// self-delimiting.
func fnvMarkdownBool(h uint64, b bool) uint64 {
	if b {
		return Fnv64Byte(h, 1)
	}
	return Fnv64Byte(h, 0)
}

// fnvMarkdownColor mixes a Color: every channel plus the set flag,
// so an explicit transparent and an unset color hash apart.
func fnvMarkdownColor(h uint64, c Color) uint64 {
	h = Fnv64Byte(h, c.R)
	h = Fnv64Byte(h, c.G)
	h = Fnv64Byte(h, c.B)
	h = Fnv64Byte(h, c.A)
	return fnvMarkdownBool(h, c.IsSet())
}

// fnvMarkdownColorPtr mixes a *Color: nil hashes as its own slot,
// so a table with no alt row and one with an explicit alt never
// share blocks.
func fnvMarkdownColorPtr(h uint64, c *Color) uint64 {
	if c == nil {
		return Fnv64Byte(h, 0)
	}
	return fnvMarkdownColor(Fnv64Byte(h, 1), *c)
}

// fnvMarkdownPadding mixes a Padding: four edges plus the set flag,
// so an explicit zero inset and an unset one hash apart.
func fnvMarkdownPadding(h uint64, p Padding) uint64 {
	h = fnvU64(h, uint64(normFloat32Bits(p.Top)))
	h = fnvU64(h, uint64(normFloat32Bits(p.Right)))
	h = fnvU64(h, uint64(normFloat32Bits(p.Bottom)))
	h = fnvU64(h, uint64(normFloat32Bits(p.Left)))
	return fnvMarkdownBool(h, p.IsSet())
}

// fnvMarkdownAffine mixes an AffineTransform by value. A nil
// transform is its own slot; sharing the shaper's reading, a nil
// and an identity transform hash apart and only miss.
func fnvMarkdownAffine(
	h uint64, a *glyph.AffineTransform,
) uint64 {
	if a == nil {
		return Fnv64Byte(h, 0)
	}
	h = Fnv64Byte(h, 1)
	h = fnvU64(h, uint64(normFloat32Bits(a.XX)))
	h = fnvU64(h, uint64(normFloat32Bits(a.XY)))
	h = fnvU64(h, uint64(normFloat32Bits(a.YX)))
	h = fnvU64(h, uint64(normFloat32Bits(a.YY)))
	h = fnvU64(h, uint64(normFloat32Bits(a.X0)))
	h = fnvU64(h, uint64(normFloat32Bits(a.Y0)))
	return h
}

// fnvMarkdownGradient mixes a GradientConfig: direction plus every
// stop position and channel. Glyph colors carry no set flag, so four
// channel bytes name the color.
func fnvMarkdownGradient(
	h uint64, g *glyph.GradientConfig,
) uint64 {
	if g == nil {
		return Fnv64Byte(h, 0)
	}
	h = Fnv64Byte(h, 1)
	h = fnvU64(h, uint64(g.Direction))
	for _, st := range g.Stops {
		h = fnvU64(h, uint64(normFloat32Bits(st.Position)))
		h = Fnv64Byte(h, st.Color.R)
		h = Fnv64Byte(h, st.Color.G)
		h = Fnv64Byte(h, st.Color.B)
		h = Fnv64Byte(h, st.Color.A)
		h = Fnv64Byte(h, fnvUnitSep)
	}
	return Fnv64Byte(h, fnvUnitSep)
}

// fnvMarkdownTextStyle mixes a whole TextStyle: the layout fields
// fnvTextStyle covers plus the paint fields it skips (Color,
// BgColor, StrokeColor, Underline, Strikethrough), the remaining
// geometry (LineSpacing, RotationRadians, Align), the role flags
// that ride into cached runs, and the pointed-to transform,
// gradient and feature sets. Paint rides along because the cached
// blocks hold resolved run styles: a recolor with identical metrics
// must still miss.
func fnvMarkdownTextStyle(h uint64, s TextStyle) uint64 {
	// Layout subset first, from the shared helper, so a field later
	// added there reaches this key too.
	h = fnvTextStyle(h, s)
	h = fnvU64(h, uint64(normFloat32Bits(s.LineSpacing)))
	h = fnvU64(h, uint64(normFloat32Bits(s.RotationRadians)))
	h = fnvMarkdownColor(h, s.Color)
	h = fnvMarkdownColor(h, s.BgColor)
	h = fnvMarkdownColor(h, s.StrokeColor)
	h = fnvU64(h, uint64(s.Align))
	h = fnvMarkdownBool(h, s.Underline)
	h = fnvMarkdownBool(h, s.Strikethrough)
	h = fnvMarkdownBool(h, s.disabledRole)
	h = fnvMarkdownBool(h, s.glyphRole)
	h = fnvMarkdownBool(h, s.defaultedColor)
	h = fnvMarkdownAffine(h, s.AffineTransform)
	h = fnvMarkdownGradient(h, s.Gradient)
	return Fnv64Byte(h, fnvUnitSep)
}
