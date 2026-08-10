package gui

import "testing"

func TestNewPadding(t *testing.T) {
	p := NewPadding(1, 2, 3, 4)
	if p.Top != 1 || p.Right != 2 || p.Bottom != 3 || p.Left != 4 {
		t.Errorf("got %+v", p)
	}
}

func TestPadAll(t *testing.T) {
	p := PadAll(5)
	if p.Top != 5 || p.Right != 5 || p.Bottom != 5 || p.Left != 5 {
		t.Errorf("got %+v", p)
	}
}

func TestPadTBLR(t *testing.T) {
	p := padTBLR(3, 7)
	if p.Top != 3 || p.Bottom != 3 || p.Left != 7 || p.Right != 7 {
		t.Errorf("got %+v", p)
	}
}

func TestPaddingWidthHeight(t *testing.T) {
	p := NewPadding(2, 5, 3, 4)
	if !f32AreClose(p.Width(), 9) {
		t.Errorf("width: got %f, want 9", p.Width())
	}
	if !f32AreClose(p.Height(), 5) {
		t.Errorf("height: got %f, want 5", p.Height())
	}
}

func TestPaddingIsNone(t *testing.T) {
	if !PaddingNone.isNone() {
		t.Error("PaddingNone should be none")
	}
	if PadAll(1).isNone() {
		t.Error("PadAll(1) should not be none")
	}
}

// TestPaddingSelfFlag pins the self-flagging contract (#243): the zero
// value is unset, constructors set, and PaddingNone is an explicitly-set
// zero that must not fall through to a theme default.
func TestPaddingSelfFlag(t *testing.T) {
	var z Padding
	if z.IsSet() {
		t.Error("zero Padding should not be set")
	}
	if !NewPadding(0, 0, 0, 0).IsSet() {
		t.Error("NewPadding with all-zero sides should be set")
	}
	if !PadAll(0).IsSet() {
		t.Error("PadAll(0) should be set")
	}
	if !PaddingNone.IsSet() {
		t.Error("PaddingNone should be set")
	}
	if !NoPadding.IsSet() {
		t.Error("NoPadding should be set")
	}
}

func TestPaddingOr(t *testing.T) {
	def := PadAll(7)
	var z Padding
	if got := z.Or(def); got != def {
		t.Errorf("unset Or = %+v, want default", got)
	}
	explicit := NewPadding(1, 2, 3, 4)
	if got := explicit.Or(def); got != explicit {
		t.Errorf("set Or = %+v, want explicit", got)
	}
	if got := PaddingNone.Or(def); got != PaddingNone {
		t.Errorf("PaddingNone Or = %+v, want PaddingNone", got)
	}
}

func TestPredefinedPaddings(t *testing.T) {
	if paddingXSmall != PadAll(PadXSmall) {
		t.Error("PaddingXSmall")
	}
	if PaddingSmall != PadAll(PadSmall) {
		t.Error("PaddingSmall")
	}
	if paddingMedium != PadAll(PadMedium) {
		t.Error("PaddingMedium")
	}
	if PaddingLarge != PadAll(PadLarge) {
		t.Error("PaddingLarge")
	}
}
