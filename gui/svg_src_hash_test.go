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
}
