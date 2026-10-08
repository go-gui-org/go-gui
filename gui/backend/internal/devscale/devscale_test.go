package devscale

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want float32
		ok   bool
	}{
		{"", 0, false},
		{"2", 2, true},
		{" 1.5 ", 1.5, true},
		{"8", 8, true},
		{"0.25", 0.25, true},
		// Outside the range the backends accept, or not a number: no
		// override, so the monitor's scale is used.
		{"0", 0, false},
		{"-1", 0, false},
		{"0.001", 0, false},
		{"0.24", 0, false},
		{"9", 0, false},
		{"NaN", 0, false},
		{"Inf", 0, false},
		{"2x", 0, false},
	}
	for _, tt := range tests {
		got, ok := parse(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("parse(%q) = (%v, %v), want (%v, %v)",
				tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestApply(t *testing.T) {
	if got := apply(1, 2, true); got != 2 {
		t.Errorf("apply with override = %v, want 2", got)
	}
	if got := apply(1.25, 0, false); got != 1.25 {
		t.Errorf("apply without override = %v, want 1.25", got)
	}
}

func TestApply120(t *testing.T) {
	tests := []struct {
		name     string
		platform int32
		s        float32
		ok       bool
		whole    bool
		want     int32
	}{
		{"no override", 180, 0, false, false, 180},
		{"no override, whole", 180, 0, false, true, 180},
		{"fractional", 120, 1.25, true, false, 150},
		{"two", 120, 2, true, false, 240},
		{"below one keeps a viewport scale", 120, 0.5, true, false, 60},
		// A tiny value must not round to 0: that sizes a 0x0 buffer.
		{"tiny", 120, 0.001, true, false, 1},
		// Without a viewport the scale goes to wl_surface.set_buffer_scale,
		// which takes a whole number of at least 1. 0 is a protocol error,
		// and a fraction would size the buffer apart from what the
		// compositor divides it by (#971).
		{"whole rounds down", 120, 1.4, true, true, 120},
		{"whole rounds up", 120, 1.5, true, true, 240},
		{"whole 2.5", 120, 2.5, true, true, 360},
		{"whole below one", 120, 0.5, true, true, 120},
		{"whole tiny", 120, 0.001, true, true, 120},
	}
	for _, tt := range tests {
		if got := apply120(tt.platform, tt.s, tt.ok, tt.whole); got != tt.want {
			t.Errorf("%s: apply120 = %v, want %v", tt.name, got, tt.want)
		}
	}
}
