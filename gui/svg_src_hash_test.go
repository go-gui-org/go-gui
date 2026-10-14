package gui

import (
	"strings"
	"testing"
)

// svgSrcHash must equal hashString for every source: the memo changes
// only how often the hash is computed, never its value.
func TestSvgSrcHashMatchesHashString(t *testing.T) {
	t.Parallel()
	w := &Window{}
	long := "<svg>" + strings.Repeat("x", svgSrcHashMemoMin) + "</svg>"
	// Same content, separate allocation: a different memo key that must
	// still produce the same hash.
	twin := string([]byte(long))
	for name, src := range map[string]string{
		"short": "icons/close.svg",
		"long":  long,
		"twin":  twin,
	} {
		for pass := range 2 { // the second pass is a memo hit for long
			if got, want := w.svgSrcHash(src), hashString(src); got != want {
				t.Errorf("%s pass %d: svgSrcHash = %x, want %x", name, pass, got, want)
			}
		}
	}
	memo := StateMapRead[svgSrcKey, uint64](w, nsSvgSrcHash)
	if memo == nil {
		t.Fatal("memo not created for a long source")
	}
	if n := memo.Len(); n != 2 {
		t.Fatalf("memo holds %d entries, want 2 (long, twin; short skips it)", n)
	}

	// The threshold is exact: 255 bytes bypass, 256 memoize.
	wb := &Window{}
	below := strings.Repeat("y", svgSrcHashMemoMin-1)
	if got, want := wb.svgSrcHash(below), hashString(below); got != want {
		t.Errorf("below threshold: svgSrcHash = %x, want %x", got, want)
	}
	if m := StateMapRead[svgSrcKey, uint64](wb, nsSvgSrcHash); m != nil && m.Len() != 0 {
		t.Errorf("below threshold created %d memo entries, want 0", m.Len())
	}
	atMin := strings.Repeat("y", svgSrcHashMemoMin)
	for pass := range 2 { // the second pass is a memo hit
		if got, want := wb.svgSrcHash(atMin), hashString(atMin); got != want {
			t.Errorf("at threshold pass %d: svgSrcHash = %x, want %x", pass, got, want)
		}
	}
	mb := StateMapRead[svgSrcKey, uint64](wb, nsSvgSrcHash)
	if mb == nil {
		t.Fatal("memo not created at threshold")
	}
	if n := mb.Len(); n != 1 {
		t.Fatalf("threshold memo holds %d entries, want 1", n)
	}
}
