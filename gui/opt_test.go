package gui

import "testing"

func TestOptZeroValueNotSet(t *testing.T) {
	var o Opt[int]
	if o.IsSet() {
		t.Error("zero Opt should not be set")
	}
	if got := o.Get(99); got != 99 {
		t.Errorf("Get = %d, want 99", got)
	}
}

func TestOptSomeIsSet(t *testing.T) {
	o := Some(42)
	if !o.IsSet() {
		t.Error("Some(42) should be set")
	}
	if got := o.Get(0); got != 42 {
		t.Errorf("Get = %d, want 42", got)
	}
}

func TestOptSomeZeroIsSet(t *testing.T) {
	o := Some(0)
	if !o.IsSet() {
		t.Error("Some(0) should be set")
	}
	if got := o.Get(99); got != 0 {
		t.Errorf("Get = %d, want 0", got)
	}
}

func TestOptValue(t *testing.T) {
	o := Some(42)
	val, ok := o.Value()
	if !ok {
		t.Error("Value should return true")
	}
	if val != 42 {
		t.Errorf("val = %d, want 42", val)
	}
}

func TestOptValueUnset(t *testing.T) {
	var o Opt[int]
	val, ok := o.Value()
	if ok {
		t.Error("Value should return false")
	}
	if val != 0 {
		t.Errorf("val = %d, want 0", val)
	}
}

func TestOptGetDefault(t *testing.T) {
	var o Opt[int]
	if got := o.Get(99); got != 99 {
		t.Errorf("Get = %d, want 99", got)
	}
}

func TestOptSomeF(t *testing.T) {
	o := SomeF(1.5)
	if !o.IsSet() {
		t.Error("SomeF should be set")
	}
	if got := o.Get(0); got != 1.5 {
		t.Errorf("Get = %f, want 1.5", got)
	}
}

func TestOptNoBorder(t *testing.T) {
	if !NoBorder.IsSet() {
		t.Error("NoBorder should be set")
	}
	if got := NoBorder.Get(5); got != 0 {
		t.Errorf("Get = %f, want 0", got)
	}
}

func TestOptNoSpacing(t *testing.T) {
	if !NoSpacing.IsSet() {
		t.Error("NoSpacing should be set")
	}
	if got := NoSpacing.Get(5); got != 0 {
		t.Errorf("Get = %f, want 0", got)
	}
}

func TestOptNoRadius(t *testing.T) {
	if !NoRadius.IsSet() {
		t.Error("NoRadius should be set")
	}
	if got := NoRadius.Get(5); got != 0 {
		t.Errorf("Get = %f, want 0", got)
	}
}

func TestOptNoPadding(t *testing.T) {
	if !NoPadding.IsSet() {
		t.Error("NoPadding should be set")
	}
	if got := NoPadding.Or(PadAll(10)); got != PaddingNone {
		t.Errorf("Or = %+v, want PaddingNone", got)
	}
}
