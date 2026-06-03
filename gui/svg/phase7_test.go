package svg

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestPhase7ParseDashArrayBasic — ring-resize-style values parse
// into a flat Values slice with DashKeyframeLen=2 and four frames.
func TestPhase7ParseDashArrayBasic(t *testing.T) {
	elem := `<animate attributeName="stroke-dasharray" dur="1.5s" ` +
		`values="0 150;42 150;42 150;42 150"/>`
	a, ok := parseAnimateDashArrayElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if a.Kind != gui.SvgAnimDashArray {
		t.Fatalf("kind=%v", a.Kind)
	}
	if a.DashKeyframeLen != 2 {
		t.Fatalf("stride=%d want 2", a.DashKeyframeLen)
	}
	if len(a.Values) != 8 {
		t.Fatalf("values len=%d want 8", len(a.Values))
	}
	want := []float32{0, 150, 42, 150, 42, 150, 42, 150}
	for i, v := range want {
		if a.Values[i] != v {
			t.Fatalf("values[%d]=%v want %v", i, a.Values[i], v)
		}
	}
}

// TestPhase7ParseDashArrayRejectsUnequalFrames — frames must all
// have the same float count.
func TestPhase7ParseDashArrayRejectsUnequalFrames(t *testing.T) {
	elem := `<animate attributeName="stroke-dasharray" dur="1s" ` +
		`values="0 150;42"/>`
	if _, ok := parseAnimateDashArrayElement(elem,
		ComputedStyle{GroupID: "g"}); ok {
		t.Fatal("expected reject")
	}
}

// TestPhase7ParseDashArrayRejectsOverCap — a frame with >cap floats
// rejects to guard the fixed-size override slot.
func TestPhase7ParseDashArrayRejectsOverCap(t *testing.T) {
	many := strings.TrimRight(strings.Repeat("1 ", 9), " ")
	elem := `<animate attributeName="stroke-dasharray" dur="1s" ` +
		`values="` + many + `;` + many + `"/>`
	if _, ok := parseAnimateDashArrayElement(elem,
		ComputedStyle{GroupID: "g"}); ok {
		t.Fatal("expected reject for over-cap frame")
	}
}

// TestPhase7ParseDashOffsetScalar — scalar dashoffset animation
// parses with one float per keyframe.
func TestPhase7ParseDashOffsetScalar(t *testing.T) {
	elem := `<animate attributeName="stroke-dashoffset" dur="1.5s" ` +
		`values="0;-16;-59;-59"/>`
	a, ok := parseAnimateDashOffsetElement(elem,
		ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if a.Kind != gui.SvgAnimDashOffset {
		t.Fatalf("kind=%v", a.Kind)
	}
	if len(a.Values) != 4 || a.Values[2] != -59 {
		t.Fatalf("values=%v", a.Values)
	}
}

// TestPhase7ApplyDasharrayOffsetPhase — offset=3 on [3,2] cycle
// over a horizontal poly 0..10 starts 3 units into the pattern, so
// stroke x=0 lands at the start of the gap. First visible dash
// begins at stroke x=2.
func TestPhase7ApplyDasharrayOffsetPhase(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	result := applyDasharray([][]float32{poly}, []float32{3, 2}, 3)
	if len(result) == 0 {
		t.Fatal("no dashes")
	}
	first := result[0]
	if len(first) < 4 {
		t.Fatalf("first dash too short: %v", first)
	}
	if first[0] < 1.99 || first[0] > 2.01 ||
		first[2] < 4.99 || first[2] > 5.01 {
		t.Fatalf("first dash x0=%v x1=%v; want (2,5)",
			first[0], first[2])
	}
}

// TestPhase7ApplyDasharrayNegativeOffsetWraps — offset=-4 on [3,2]
// cycle length=5: -4 mod 5 = 1. Skip 1 unit of first dash, so
// dashes start at 0..2, 4..7, 9..10.
func TestPhase7ApplyDasharrayNegativeOffsetWraps(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	result := applyDasharray([][]float32{poly}, []float32{3, 2}, -4)
	if len(result) == 0 {
		t.Fatal("no dashes")
	}
	// First dash should be shorter (0..2 = 2 units, not 0..3).
	first := result[0]
	if first[2] < 1.99 || first[2] > 2.01 {
		t.Fatalf("first dash x1=%v; want 2 (phase shifted 1 unit)",
			first[2])
	}
}

// TestPhase7RingResizeParsesAnimated — ring-resize.svg parses, the
// stroked circle is flagged Animated, and two dash animations plus
// one rotate animation are recorded.
func TestPhase7RingResizeParsesAnimated(t *testing.T) {
	asset := `<svg stroke="currentColor" viewBox="0 0 24 24" ` +
		`xmlns="http://www.w3.org/2000/svg"><g>` +
		`<circle cx="12" cy="12" r="9.5" fill="none" ` +
		`stroke-width="3" stroke-linecap="round">` +
		`<animate attributeName="stroke-dasharray" dur="1.5s" ` +
		`values="0 150;42 150;42 150;42 150" ` +
		`repeatCount="indefinite"/>` +
		`<animate attributeName="stroke-dashoffset" dur="1.5s" ` +
		`values="0;-16;-59;-59" ` +
		`repeatCount="indefinite"/></circle>` +
		`<animateTransform attributeName="transform" type="rotate" ` +
		`dur="2s" values="0 12 12;360 12 12" ` +
		`repeatCount="indefinite"/></g></svg>`
	p := New()
	parsed, err := p.ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var kinds []gui.SvgAnimKind
	for _, a := range parsed.Animations {
		kinds = append(kinds, a.Kind)
	}
	hasArr := false
	hasOff := false
	for _, k := range kinds {
		if k == gui.SvgAnimDashArray {
			hasArr = true
		}
		if k == gui.SvgAnimDashOffset {
			hasOff = true
		}
	}
	if !hasArr || !hasOff {
		t.Fatalf("kinds=%v; missing dash anims", kinds)
	}

	tris := p.Tessellate(parsed, 1)
	if len(tris) == 0 {
		t.Fatal("no tessellated paths")
	}
	animatedCount := 0
	for _, tp := range tris {
		if tp.Animated {
			animatedCount++
		}
	}
	if animatedCount == 0 {
		t.Fatal("expected stroked circle flagged Animated")
	}
}

// TestPhase7TessellateAnimatedSubstitutesDashes — supplying live
// dash overrides via TessellateAnimated yields triangles whose
// count differs from the undashed cached stroke (dashing splits
// the ring into segments + round caps per segment).
func TestPhase7TessellateAnimatedSubstitutesDashes(t *testing.T) {
	asset := `<svg stroke="currentColor" viewBox="0 0 24 24" ` +
		`xmlns="http://www.w3.org/2000/svg"><g>` +
		`<circle cx="12" cy="12" r="9.5" fill="none" ` +
		`stroke-width="3" stroke-linecap="round">` +
		`<animate attributeName="stroke-dasharray" dur="1.5s" ` +
		`values="0 150;42 150;42 150;42 150" ` +
		`repeatCount="indefinite"/>` +
		`<animate attributeName="stroke-dashoffset" dur="1.5s" ` +
		`values="0;-16;-59;-59" ` +
		`repeatCount="indefinite"/></circle></g></svg>`
	p := New()
	parsed, err := p.ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	base := p.Tessellate(parsed, 1)
	baseTris := 0
	for _, tp := range base {
		if tp.IsStroke {
			baseTris = len(tp.Triangles)
		}
	}
	if baseTris == 0 {
		t.Fatal("no base stroke triangles")
	}

	// Mid-cycle override: 42-unit dash, 150-unit gap, offset -30.
	ov := gui.SvgAnimAttrOverride{
		Mask: gui.SvgAnimMaskStrokeDashArray |
			gui.SvgAnimMaskStrokeDashOffset,
		StrokeDashArrayLen: 2,
		StrokeDashOffset:   -30,
	}
	ov.StrokeDashArray[0] = 42
	ov.StrokeDashArray[1] = 150
	got := p.TessellateAnimated(parsed, 1,
		map[uint32]gui.SvgAnimAttrOverride{
			firstAnimatedPathID(parsed): ov,
		}, nil)
	if len(got) == 0 {
		t.Fatal("TessellateAnimated returned empty")
	}
	animTris := 0
	for _, tp := range got {
		if tp.IsStroke {
			animTris = len(tp.Triangles)
		}
	}
	if animTris == 0 {
		t.Fatal("no animated stroke triangles")
	}
	if animTris == baseTris {
		t.Fatalf("dashed stroke should differ from solid; both %d",
			animTris)
	}
}
