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
