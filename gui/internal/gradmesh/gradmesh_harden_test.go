package gradmesh

import (
	"math"
	"testing"
)

func TestStopIsolinesReflectSorted(t *testing.T) {
	offsets := []float32{0, 0.2, 0.5, 0.8, 1}
	got := stopIsolines(offsets, SpreadReflect, -2.5, 2.5, nil)
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Fatalf("reflect isolines not sorted: %v", got)
		}
	}
}

func TestSubdivideNilParamsNoPanic(t *testing.T) {
	tris := []float32{0, 0, 10, 0, 0, 10}
	// Nil params must not panic and must return input unchanged.
	if got := Subdivide(tris, nil, nil, nil, nil); len(got) != len(tris) {
		t.Errorf("nil params: got %d, want %d", len(got), len(tris))
	}
	var split, radial, isolines []float32
	p := &Params{X1: 0, Y1: 0, X2: 10, Y2: 0, StopOffsets: []float32{0, 1}}
	// Nil scratch slices must not panic.
	if got := Subdivide(tris, p, nil, &radial, &isolines); len(got) != len(tris) {
		t.Errorf("nil split: got %d", len(got))
	}
	if got := Subdivide(tris, p, &split, nil, &isolines); len(got) != len(tris) {
		t.Errorf("nil radial: got %d", len(got))
	}
	if got := Subdivide(tris, p, &split, &radial, nil); len(got) != len(tris) {
		t.Errorf("nil isolines: got %d", len(got))
	}
}

func TestSplitTriAtStopsReflectParity(t *testing.T) {
	// Reflect with many periods exercises the binary search vs linear
	// scan equivalence; the split must still eliminate straddling.
	offsets := []float32{0, 0.25, 0.5, 0.75, 1}
	p := &Params{X1: 0, Y1: 0, X2: 10, Y2: 0, Spread: SpreadReflect, StopOffsets: offsets}
	tris := []float32{0, 0, 100, 0, 0, 100}
	var split, radial, iso []float32
	out := Subdivide(tris, p, &split, &radial, &iso)
	// Verify sortedness of isolines used for this subdivision.
	for i := 1; i < len(iso); i++ {
		if iso[i] < iso[i-1] {
			t.Fatalf("iso not sorted %v", iso)
		}
	}
	if len(out) <= len(tris) {
		t.Errorf("reflect split produced %d, want > %d", len(out), len(tris))
	}
	// No triangle should straddle a period boundary by more than 1e-4.
	for i := 0; i+5 < len(out); i += 6 {
		ta := RawT(out[i], out[i+1], p)
		tb := RawT(out[i+2], out[i+3], p)
		tc := RawT(out[i+4], out[i+5], p)
		lo := min(ta, tb, tc)
		hi := max(ta, tb, tc)
		if !finite(lo) || !finite(hi) {
			continue
		}
		k := math.Floor(float64(lo)) + 1
		if float32(k) > lo+1e-4 && float32(k) < hi-1e-4 {
			t.Errorf("triangle %d straddles period %v: [%v %v %v]", i/6, k, ta, tb, tc)
			break
		}
	}
}
