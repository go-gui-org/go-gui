package svg

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMixOptsHashFieldOrderMatters(t *testing.T) {
	base := uint64(0xcbf29ce484222325)

	hAOnly := mixOptsHash(base,
		gui.SvgParseOpts{HoveredElementID: "a"})
	fAOnly := mixOptsHash(base,
		gui.SvgParseOpts{FocusedElementID: "a"})
	if hAOnly == fAOnly {
		t.Errorf("hovered=a and focused=a must hash differently")
	}

	flat := mixOptsHash(base,
		gui.SvgParseOpts{FlatnessTolerance: 0.5})
	if flat == base {
		t.Errorf("FlatnessTolerance must perturb the hash")
	}

	rm := mixOptsHash(base,
		gui.SvgParseOpts{PrefersReducedMotion: true})
	none := mixOptsHash(base, gui.SvgParseOpts{})
	if rm == none {
		t.Errorf("PrefersReducedMotion must perturb the hash")
	}
}

func TestMixOptsHashEmptyIsStable(t *testing.T) {
	base := uint64(42)
	a := mixOptsHash(base, gui.SvgParseOpts{})
	b := mixOptsHash(base, gui.SvgParseOpts{})
	if a != b {
		t.Errorf("empty opts must hash deterministically: %x vs %x", a, b)
	}
}

// inlineSourceKey must produce a fixed-size key regardless of input
// length so retained sourceKeys cannot be inflated by hostile inline
// data (DoS surface for the cache).
func TestInlineSourceKey_FixedSize(t *testing.T) {
	short := inlineSourceKey("<svg/>")
	long := inlineSourceKey(strings.Repeat("<g/>", 100000))
	const want = 7 + 64 // "inline:" + 32-byte sha256 hex
	if len(short) != want || len(long) != want {
		t.Fatalf("len(short)=%d len(long)=%d; want %d both",
			len(short), len(long), want)
	}
	if !strings.HasPrefix(short, "inline:") {
		t.Fatalf("missing prefix: %q", short)
	}
}

// Same input → same key (cache-lookup invariant; InvalidateSvgSource
// relies on it).
func TestInlineSourceKey_Deterministic(t *testing.T) {
	data := `<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	a := inlineSourceKey(data)
	b := inlineSourceKey(data)
	if a != b {
		t.Fatalf("non-deterministic: %q vs %q", a, b)
	}
}

// Different inputs → different keys (cache-correctness invariant;
// collision would serve cached output for the wrong SVG).
func TestInlineSourceKey_DistinctForDifferentInputs(t *testing.T) {
	a := inlineSourceKey("<svg/>")
	b := inlineSourceKey("<svg ></svg>")
	if a == b {
		t.Fatalf("collision on different inputs: %q", a)
	}
}

// Empty input is valid (cache key for empty inline SVG) — must not
// panic, must still be fixed-size.
func TestInlineSourceKey_EmptyInput(t *testing.T) {
	k := inlineSourceKey("")
	if len(k) != 7+64 {
		t.Fatalf("empty key len=%d; want 71", len(k))
	}
}
