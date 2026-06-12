package gui

import "testing"

func TestPreserveAlignFractions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a            SvgAlign
		wantX, wantY float32
	}{
		{SvgAlignXMidYMid, 0.5, 0.5},
		{SvgAlignXMinYMin, 0, 0},
		{SvgAlignXMidYMin, 0.5, 0},
		{SvgAlignXMaxYMin, 1, 0},
		{SvgAlignXMinYMid, 0, 0.5},
		{SvgAlignXMaxYMid, 1, 0.5},
		{SvgAlignXMinYMax, 0, 1},
		{SvgAlignXMidYMax, 0.5, 1},
		{SvgAlignXMaxYMax, 1, 1},
		// SvgAlignNone: non-uniform stretch — slack is zero,
		// fractions are irrelevant. Returns 0,0 for determinism.
		{SvgAlignNone, 0, 0},
	}
	for _, c := range cases {
		x, y := PreserveAlignFractions(c.a)
		if x != c.wantX || y != c.wantY {
			t.Errorf("align %v: got (%v,%v), want (%v,%v)",
				c.a, x, y, c.wantX, c.wantY)
		}
	}
}

func TestPreserveAlignFractionsDefaultZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value SvgAlign must equal xMidYMid so that callers
	// constructing SvgParsed without setting PreserveAlign keep
	// historic centered behavior.
	x, y := PreserveAlignFractions(0)
	if x != 0.5 || y != 0.5 {
		t.Errorf("zero-value align: got (%v,%v), want (0.5,0.5)", x, y)
	}
}

func TestPreserveAlignFractions_UnknownValueDefaultsToMid(t *testing.T) {
	t.Parallel()
	// Values outside the named SvgAlign enum must default to
	// xMidYMid so a future enum addition or memory corruption
	// doesn't produce a nonsense offset.
	x, y := PreserveAlignFractions(SvgAlign(99))
	if x != 0.5 || y != 0.5 {
		t.Errorf("unknown align: got (%v,%v), want (0.5,0.5)", x, y)
	}
}
