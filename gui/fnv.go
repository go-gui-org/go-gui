package gui

import (
	"math"

	"github.com/go-gui-org/go-glyph"
)

// FNV-1a 64-bit constants. Exported so the datagrid subpackage,
// which cannot see unexported gui identifiers, hashes with the
// same basis as the rest of the module.
const (
	Fnv64Offset = uint64(14695981039346656037)
	Fnv64Prime  = uint64(1099511628211)
)

// Field separators for FNV-1a cache keys. They mark boundaries
// between hashed fields so concatenating different fields cannot
// produce the same digest as a single longer field. Record
// separates rows, unit separates fields within a row.
const (
	fnvRecordSep = byte(0x1E)
	fnvUnitSep   = byte(0x1F)
)

// Fnv64Str hashes a string into an existing FNV-1a 64-bit hash.
func Fnv64Str(h uint64, s string) uint64 {
	for i := range len(s) {
		h = (h ^ uint64(s[i])) * Fnv64Prime
	}
	return h
}

// Fnv64Byte hashes a byte into an existing FNV-1a 64-bit hash.
func Fnv64Byte(h uint64, b byte) uint64 {
	return (h ^ uint64(b)) * Fnv64Prime
}

// fnvU64 mixes a uint64 word into a hash in little-endian byte
// order, the same digest Fnv64Str would produce for those 8
// bytes. It keeps integer-keyed cache keys on the shared FNV-1a
// basis without formatting the number first.
func fnvU64(h, v uint64) uint64 {
	for range 8 {
		h = (h ^ (v & 0xff)) * Fnv64Prime
		v >>= 8
	}
	return h
}

// normFloat32Bits returns the bit pattern of f for hashing, with
// all NaN forms mapped to 0. NaN has many bit patterns, so
// hashing raw bits would key the same logical value apart and
// churn the cache it guards.
func normFloat32Bits(f float32) uint32 {
	if math.IsNaN(float64(f)) {
		return 0
	}
	return math.Float32bits(f)
}

// fnvFontFeatures mixes OpenType feature and variable-axis
// settings into a hash. A nil set hashes as nothing, matching
// the shaper, which treats nil and empty the same.
func fnvFontFeatures(h uint64, f *glyph.FontFeatures) uint64 {
	if f == nil {
		return h
	}
	for _, feat := range f.OpenTypeFeatures {
		h = Fnv64Str(h, feat.Tag)
		h = fnvU64(h, uint64(feat.Value))
		h = Fnv64Byte(h, fnvUnitSep)
	}
	for _, axis := range f.VariationAxes {
		h = Fnv64Str(h, axis.Tag)
		h = fnvU64(h, uint64(normFloat32Bits(axis.Value)))
		h = Fnv64Byte(h, fnvUnitSep)
	}
	return h
}

// fnvTextStyle mixes the layout-affecting fields of a gui
// TextStyle into a hash: the same subset toGlyphStyle forwards
// to the shaper. Paint-only fields (Color, BgColor, StrokeColor,
// Underline, Strikethrough) and render-time transforms ride
// along untouched because they cannot move glyphs.
func fnvTextStyle(h uint64, s TextStyle) uint64 {
	h = Fnv64Str(h, s.Family)
	h = Fnv64Byte(h, fnvUnitSep)
	h = fnvU64(h, uint64(s.Typeface))
	h = fnvU64(h, uint64(normFloat32Bits(s.Size)))
	h = fnvU64(h, uint64(normFloat32Bits(s.LetterSpacing)))
	h = fnvU64(h, uint64(normFloat32Bits(s.EmojiBoxWidth)))
	h = fnvU64(h, uint64(normFloat32Bits(s.CellWidth)))
	h = fnvU64(h, uint64(normFloat32Bits(s.CellHeight)))
	h = fnvU64(h, uint64(normFloat32Bits(s.StrokeWidth)))
	if s.NoBuiltinBoxGlyphs {
		h = Fnv64Byte(h, 1)
	} else {
		h = Fnv64Byte(h, 0)
	}
	h = fnvFontFeatures(h, s.Features)
	h = Fnv64Byte(h, fnvUnitSep)
	return h
}

// fnvGlyphStyle mixes the layout-affecting fields of a glyph
// TextStyle into a hash. It mirrors fnvTextStyle for styles
// that already crossed the gui/glyph boundary, such as the RTF
// base style.
func fnvGlyphStyle(h uint64, s glyph.TextStyle) uint64 {
	h = Fnv64Str(h, s.FontName)
	h = Fnv64Byte(h, fnvUnitSep)
	h = fnvU64(h, uint64(s.Typeface))
	h = fnvU64(h, uint64(normFloat32Bits(s.Size)))
	h = fnvU64(h, uint64(normFloat32Bits(s.LetterSpacing)))
	h = fnvU64(h, uint64(normFloat32Bits(s.EmojiBoxWidth)))
	h = fnvU64(h, uint64(normFloat32Bits(s.CellWidth)))
	h = fnvU64(h, uint64(normFloat32Bits(s.CellHeight)))
	h = fnvU64(h, uint64(normFloat32Bits(s.StrokeWidth)))
	if s.NoBuiltinBoxGlyphs {
		h = Fnv64Byte(h, 1)
	} else {
		h = Fnv64Byte(h, 0)
	}
	h = fnvFontFeatures(h, s.Features)
	h = Fnv64Byte(h, fnvUnitSep)
	return h
}
