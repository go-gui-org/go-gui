package gui

import "testing"

func TestSizingConstants(t *testing.T) {
	if FitFit.Width != sizingFit || FitFit.Height != sizingFit {
		t.Error("FitFit")
	}
	if FixedFixed.Width != sizingFixed || FixedFixed.Height != sizingFixed {
		t.Error("FixedFixed")
	}
	if FillFill.Width != sizingFill || FillFill.Height != sizingFill {
		t.Error("FillFill")
	}
	if FitFill.Width != sizingFit || FitFill.Height != sizingFill {
		t.Error("FitFill")
	}
	if FillFit.Width != sizingFill || FillFit.Height != sizingFit {
		t.Error("FillFit")
	}
	if FixedFill.Width != sizingFixed || FixedFill.Height != sizingFill {
		t.Error("FixedFill")
	}
	if FillFixed.Width != sizingFill || FillFixed.Height != sizingFixed {
		t.Error("FillFixed")
	}
	if FitFixed.Width != sizingFit || FitFixed.Height != sizingFixed {
		t.Error("FitFixed")
	}
	if FixedFit.Width != sizingFixed || FixedFit.Height != sizingFit {
		t.Error("FixedFit")
	}
}

func TestApplyFixedSizingConstraintsWidth(t *testing.T) {
	s := &Shape{Sizing: FixedFixed, Width: 50, Height: 30}
	applyFixedSizingConstraints(s)
	if s.MinWidth != 50 || s.MaxWidth != 50 {
		t.Errorf("width: min=%f max=%f", s.MinWidth, s.MaxWidth)
	}
	if s.MinHeight != 30 || s.MaxHeight != 30 {
		t.Errorf("height: min=%f max=%f", s.MinHeight, s.MaxHeight)
	}
}

func TestApplyFixedSizingConstraintsSkipsNonFixed(t *testing.T) {
	s := &Shape{Sizing: FillFill, Width: 50, Height: 30}
	applyFixedSizingConstraints(s)
	if s.MinWidth != 0 || s.MaxWidth != 0 {
		t.Errorf("width should be unchanged: min=%f max=%f",
			s.MinWidth, s.MaxWidth)
	}
	if s.MinHeight != 0 || s.MaxHeight != 0 {
		t.Errorf("height should be unchanged: min=%f max=%f",
			s.MinHeight, s.MaxHeight)
	}
}

func TestApplyFixedSizingConstraintsZeroSize(t *testing.T) {
	s := &Shape{Sizing: FixedFixed, Width: 0, Height: 0}
	applyFixedSizingConstraints(s)
	// Zero size should not set constraints.
	if s.MinWidth != 0 || s.MaxWidth != 0 {
		t.Error("zero width should not set constraints")
	}
}

func TestApplyFixedSizingConstraintsMixed(t *testing.T) {
	s := &Shape{Sizing: FixedFill, Width: 80, Height: 60}
	applyFixedSizingConstraints(s)
	if s.MinWidth != 80 || s.MaxWidth != 80 {
		t.Errorf("fixed width: min=%f max=%f", s.MinWidth, s.MaxWidth)
	}
	if s.MinHeight != 0 || s.MaxHeight != 0 {
		t.Error("fill height should not be constrained")
	}
}
