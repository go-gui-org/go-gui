package gui

import "unsafe"

// svgSrcHashMemoMin is the source length from which svgSrcHash
// memoizes. Below it hashing costs about what the memo lookup does.
const svgSrcHashMemoMin = 256

// svgSrcKey identifies a source string by its bytes' address and
// length. Go strings are immutable, so one key always names one
// content, and the key's pointer keeps those bytes alive: the address
// cannot be reused by another string while the entry exists.
type svgSrcKey struct {
	data *byte
	n    int
}

// svgSrcHash returns hashString(svgSrc). Every Svg view looks its
// source up each frame, in generation and again in render, and an
// inline document is hashed in full each time: on an SVG-heavy page
// that was most of the frame (a 68 KB document × every Svg view in
// the tree). A long source is hashed once per string and then found
// by identity, which is what an embedded or held document gives.
func (w *Window) svgSrcHash(svgSrc string) uint64 {
	if len(svgSrc) < svgSrcHashMemoMin {
		return hashString(svgSrc)
	}
	key := svgSrcKey{unsafe.StringData(svgSrc), len(svgSrc)}
	memo := StateMap[svgSrcKey, uint64](w, nsSvgSrcHash, capModerate)
	if h, ok := memo.Get(key); ok {
		return h
	}
	h := hashString(svgSrc)
	memo.Set(key, h)
	return h
}
