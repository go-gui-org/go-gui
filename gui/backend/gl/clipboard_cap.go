package gl

// appendClipChunk appends one transfer page to a clipboard buffer
// while enforcing a total cap: any process can stuff the selection,
// so a hostile owner must not grow our buffer without bound. Reports
// false when paging stops — either the buffer reached the cap or the
// caller decides so from BytesAfter. Kept free of platform calls so
// it unit-tests on every GOOS, unlike the transfer loops that use it.
func appendClipChunk(out, page []byte, limit int) ([]byte, bool) {
	if len(out) >= limit {
		return out, false
	}
	out = append(out, page...)
	if len(out) > limit {
		// Truncation can split a trailing multi-byte rune; the
		// result decodes with a RuneError tail. Acceptable for a
		// hostile payload that only a cap stops.
		out = out[:limit]
		return out, false
	}
	return out, true
}
