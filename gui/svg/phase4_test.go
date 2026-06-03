package svg

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestPhase4BeginSpecLiteralOnly — begin="0.5s" has no syncbase
// refs, so parseBeginSpecs returns nil (no post-pass needed).
func TestPhase4BeginSpecLiteralOnly(t *testing.T) {
	elem := `<animate begin="0.5s"/>`
	if got := parseBeginSpecs(elem); got != nil {
		t.Fatalf("want nil for pure literal, got %+v", got)
	}
}

// TestPhase4BeginSpecSyncbaseBegin — "a.begin+0.1s" parses to a
// single syncbase spec.
func TestPhase4BeginSpecSyncbaseBegin(t *testing.T) {
	elem := `<animate begin="a.begin+0.1s"/>`
	got := parseBeginSpecs(elem)
	if len(got) != 1 {
		t.Fatalf("want 1 spec, got %d", len(got))
	}
	if got[0].targetID != "a" || got[0].isEnd || got[0].offset != 0.1 {
		t.Fatalf("unexpected spec: %+v", got[0])
	}
}

// TestPhase4BeginSpecSyncbaseEnd — "a.end+0.25s" parses as an
// end-reference with +0.25 offset.
func TestPhase4BeginSpecSyncbaseEnd(t *testing.T) {
	elem := `<animate begin="a.end+0.25s"/>`
	got := parseBeginSpecs(elem)
	if len(got) != 1 {
		t.Fatalf("want 1 spec, got %d", len(got))
	}
	if got[0].targetID != "a" || !got[0].isEnd ||
		got[0].offset != 0.25 {
		t.Fatalf("unexpected spec: %+v", got[0])
	}
}

// TestPhase4BeginSpecNegativeOffset — "a.end-0.1s" yields negative
// offset.
func TestPhase4BeginSpecNegativeOffset(t *testing.T) {
	elem := `<animate begin="a.end-0.1s"/>`
	got := parseBeginSpecs(elem)
	if len(got) != 1 || got[0].offset != -0.1 {
		t.Fatalf("want offset=-0.1, got %+v", got)
	}
}

// TestPhase4BeginSpecList — "0;a.end+0.25s" yields a literal then
// a syncbase spec, in document order.
func TestPhase4BeginSpecList(t *testing.T) {
	elem := `<animate begin="0;a.end+0.25s"/>`
	got := parseBeginSpecs(elem)
	if len(got) != 2 {
		t.Fatalf("want 2 specs, got %d", len(got))
	}
	if got[0].targetID != "" || got[0].offset != 0 {
		t.Fatalf("spec[0] expected literal 0, got %+v", got[0])
	}
	if got[1].targetID != "a" || !got[1].isEnd ||
		got[1].offset != 0.25 {
		t.Fatalf("spec[1] unexpected: %+v", got[1])
	}
}

// TestPhase4BeginSpecBareSyncbase — "a.begin" with no offset.
func TestPhase4BeginSpecBareSyncbase(t *testing.T) {
	elem := `<animate begin="a.begin"/>`
	got := parseBeginSpecs(elem)
	if len(got) != 1 || got[0].targetID != "a" || got[0].offset != 0 {
		t.Fatalf("unexpected: %+v", got)
	}
}

// TestPhase4ResolveSimpleChain — A is literal 0, B refs A.begin+0.1.
// Resolution yields A.BeginSec=0, B.BeginSec=0.1.
func TestPhase4ResolveSimpleChain(t *testing.T) {
	asset := `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3"><animate id="a"
			attributeName="cy" dur="0.6s" values="12;6;12" begin="0"/>
		</circle>
		<circle cx="12" cy="12" r="3"><animate
			attributeName="cy" dur="0.6s" values="12;6;12"
			begin="a.begin+0.1s"/>
		</circle>
	</svg>`
	parsed, err := New().ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Animations) != 2 {
		t.Fatalf("want 2 anims, got %d", len(parsed.Animations))
	}
	if f32Abs(parsed.Animations[0].BeginSec) > 1e-5 {
		t.Fatalf("A.BeginSec want 0, got %g",
			parsed.Animations[0].BeginSec)
	}
	if f32Abs(parsed.Animations[1].BeginSec-0.1) > 1e-5 {
		t.Fatalf("B.BeginSec want 0.1, got %g",
			parsed.Animations[1].BeginSec)
	}
}

// TestPhase4ResolveSyncbaseEnd — A dur=0.6 begin=0, B begin=A.end+0.2.
// Expect B.BeginSec = 0.6+0.2 = 0.8.
func TestPhase4ResolveSyncbaseEnd(t *testing.T) {
	asset := `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3"><animate id="a"
			attributeName="cy" dur="0.6s" values="12;6;12" begin="0"/>
		</circle>
		<circle cx="12" cy="12" r="3"><animate
			attributeName="cy" dur="0.6s" values="12;6;12"
			begin="a.end+0.2s"/>
		</circle>
	</svg>`
	parsed, err := New().ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f32Abs(parsed.Animations[1].BeginSec-0.8) > 1e-5 {
		t.Fatalf("B.BeginSec want 0.8, got %g",
			parsed.Animations[1].BeginSec)
	}
}

// TestPhase4ResolveUnknownTargetFallsThrough — begin lists a
// non-existent id followed by a literal; literal wins.
func TestPhase4ResolveUnknownTargetFallsThrough(t *testing.T) {
	asset := `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3"><animate
			attributeName="cy" dur="0.6s" values="12;6;12"
			begin="nosuch.begin+1s;0.3s"/>
		</circle>
	</svg>`
	parsed, err := New().ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f32Abs(parsed.Animations[0].BeginSec-0.3) > 1e-5 {
		t.Fatalf("want 0.3 literal fallback, got %g",
			parsed.Animations[0].BeginSec)
	}
}

// TestPhase4ResolveCycleIgnored — A → B → A. Cycle must not hang;
// both animations keep their parse-time BeginSec (0 here).
func TestPhase4ResolveCycleIgnored(t *testing.T) {
	asset := `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3"><animate id="a"
			attributeName="cy" dur="0.6s" values="12;6;12"
			begin="b.begin+0.1s"/>
		</circle>
		<circle cx="12" cy="12" r="3"><animate id="b"
			attributeName="cy" dur="0.6s" values="12;6;12"
			begin="a.begin+0.1s"/>
		</circle>
	</svg>`
	parsed, err := New().ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Cycle → first-match fails for both, BeginSec stays 0.
	for i, a := range parsed.Animations {
		if a.BeginSec != 0 {
			t.Fatalf("anim[%d].BeginSec want 0 on cycle, got %g",
				i, a.BeginSec)
		}
	}
}

// TestPhase4ResolveFirstMatchLiteralWins — "0;a.end+1s" picks the
// literal 0 even when id "a" exists.
func TestPhase4ResolveFirstMatchLiteralWins(t *testing.T) {
	asset := `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3"><animate id="a"
			attributeName="cy" dur="0.6s" values="12;6;12" begin="0"/>
		</circle>
		<circle cx="12" cy="12" r="3"><animate
			attributeName="cy" dur="0.6s" values="12;6;12"
			begin="0;a.end+1s"/>
		</circle>
	</svg>`
	parsed, err := New().ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Animations[1].BeginSec != 0 {
		t.Fatalf("first-match literal must win; got %g",
			parsed.Animations[1].BeginSec)
	}
}

// TestPhase4ThreeDotsBounceStagger parses the real 3-dots-bounce
// asset inline and verifies the three animations end up with
// begins of 0, 0.1, 0.2 — the staggered phase that makes the
// dots bounce in sequence rather than in lock-step.
func TestPhase4ThreeDotsBounceStagger(t *testing.T) {
	asset := `<svg fill="currentColor" viewBox="0 0 24 24" ` +
		`xmlns="http://www.w3.org/2000/svg">` +
		`<circle cx="4" cy="12" r="3"><animate id="spinner_qFRN" ` +
		`begin="0;spinner_OcgL.end+0.25s" attributeName="cy" ` +
		`calcMode="spline" dur="0.6s" values="12;6;12" ` +
		`keySplines=".33,.66,.66,1;.33,0,.66,.33"/></circle>` +
		`<circle cx="12" cy="12" r="3"><animate ` +
		`begin="spinner_qFRN.begin+0.1s" attributeName="cy" ` +
		`calcMode="spline" dur="0.6s" values="12;6;12" ` +
		`keySplines=".33,.66,.66,1;.33,0,.66,.33"/></circle>` +
		`<circle cx="20" cy="12" r="3"><animate id="spinner_OcgL" ` +
		`begin="spinner_qFRN.begin+0.2s" attributeName="cy" ` +
		`calcMode="spline" dur="0.6s" values="12;6;12" ` +
		`keySplines=".33,.66,.66,1;.33,0,.66,.33"/></circle></svg>`
	parsed, err := New().ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Animations) != 3 {
		t.Fatalf("want 3 anims, got %d", len(parsed.Animations))
	}
	// Verify all three anims are SvgAnimAttr on "cy".
	for i, a := range parsed.Animations {
		if a.Kind != gui.SvgAnimAttr || a.AttrName != gui.SvgAttrCY {
			t.Fatalf("anim[%d] unexpected kind/attr: %+v", i, a)
		}
	}
	// Collect begins as a set — document order is stable but
	// index-by-group keeps the assertion clearer.
	begins := map[float32]bool{}
	for _, a := range parsed.Animations {
		// Round to 1e-3 to tolerate float32 drift.
		begins[f32Round3(a.BeginSec)] = true
	}
	for _, want := range []float32{0, 0.1, 0.2} {
		if !begins[want] {
			t.Fatalf("missing begin=%g in set %+v", want, begins)
		}
	}
}

// TestParseBeginSpecsCapsAtMaxKeyframes — oversized begin list
// is truncated so resolveBegins work stays bounded.
func TestParseBeginSpecsCapsAtMaxKeyframes(t *testing.T) {
	body := strings.Repeat("a.begin+0.1s;", maxKeyframes+50)
	elem := `<animate begin="` + body + `"/>`
	got := parseBeginSpecs(elem)
	if len(got) > maxKeyframes {
		t.Fatalf("want len<=%d, got %d", maxKeyframes, len(got))
	}
}

func f32Round3(f float32) float32 {
	return float32(int32(f*1000+0.5)) / 1000
}
