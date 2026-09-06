package gui

import (
	"math"
	"strings"
	"testing"
)

// A window that was never faded is fully opaque. The field is seeded in
// NewWindow because the zero value would read as invisible.
func TestWindowOpacityDefault(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	if got := w.WindowOpacity(); got != 1 {
		t.Fatalf("default opacity = %v, want 1", got)
	}
}

// Out-of-range values are held inside [0, 1] rather than rejected: the
// setter is best-effort, and a clamp is what every platform does anyway.
func TestSetWindowOpacityClamps(t *testing.T) {
	tests := []struct {
		name string
		set  float32
		want float32
	}{
		{"in range", 0.5, 0.5},
		{"zero", 0, 0},
		{"one", 1, 1},
		{"above one", 4, 1},
		{"below zero", -2, 0},
		{"negative infinity", float32(math.Inf(-1)), 0},
		{"positive infinity", float32(math.Inf(1)), 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWindow(WindowCfg{})
			defer w.Close()
			w.SetWindowOpacity(tc.set)
			if got := w.WindowOpacity(); got != tc.want {
				t.Fatalf("SetWindowOpacity(%v) then WindowOpacity() = %v, want %v",
					tc.set, got, tc.want)
			}
		})
	}
}

// NaN names no fade, so it leaves the current value alone rather than
// clamping to an arbitrary end of the range.
func TestSetWindowOpacityIgnoresNaN(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	w.SetWindowOpacity(0.25)
	w.SetWindowOpacity(float32(math.NaN()))
	if got := w.WindowOpacity(); got != 0.25 {
		t.Fatalf("NaN must not disturb the value, got %v", got)
	}
}

// Tests run with no native platform injected, the same as every other
// window setter.
func TestSetWindowOpacityNilPlatform(_ *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	w.SetWindowOpacity(0.5)
	w.SetWindowOpacity(1)
}

// Each distinct cause is reported once. A refusal and a missing
// compositor can both hold, so they must not collapse into one finding.
func TestDebugWindowOpacityWarnOnce(t *testing.T) {
	buf := captureDebugMask(t, DebugWindowDegraded)
	w := &Window{}

	w.DebugWindowOpacity("the window is Transparent")
	w.DebugWindowOpacity("the window is Transparent")
	got := buf.String()
	if !strings.Contains(got, "opacity requested but the window is Transparent") {
		t.Fatalf("want the degrade finding, got %q", got)
	}
	if n := strings.Count(got, "opacity requested"); n != 1 {
		t.Fatalf("warn-once: want 1 finding, got %d", n)
	}

	w.DebugWindowOpacity("no compositing manager")
	if n := strings.Count(buf.String(), "opacity requested"); n != 2 {
		t.Fatalf("a second cause must report separately, got %d", n)
	}
}

// The check runs from the backend, outside the per-frame audit, so it
// has to consult the mask itself.
func TestDebugWindowOpacityCategoryGate(t *testing.T) {
	buf := captureDebug(t)
	DebugCategories(DebugMissingIDs)
	w := &Window{}

	w.DebugWindowOpacity("the window is Transparent")
	if got := buf.String(); got != "" {
		t.Fatalf("category off must be silent, got %q", got)
	}
}

// opacityRecorder captures what the setter hands the platform. Backends
// trust that value without re-clamping, so the clamp has to happen on
// this side of the call and not only in the cached field.
type opacityRecorder struct {
	noopNativePlatform
	got  []float32
	sawN bool
}

func (p *opacityRecorder) SetWindowOpacity(opacity float32) {
	if math.IsNaN(float64(opacity)) {
		p.sawN = true
	}
	p.got = append(p.got, opacity)
}

// The platform sees the clamped value, never the caller's raw one, and
// never a NaN: opacityCardinal and opacityAlphaByte are written against
// that promise.
func TestSetWindowOpacityForwardsClamped(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	rec := &opacityRecorder{}
	w.SetNativePlatform(rec)

	w.SetWindowOpacity(4)
	w.SetWindowOpacity(-2)
	w.SetWindowOpacity(0.5)
	w.SetWindowOpacity(float32(math.NaN()))

	want := []float32{1, 0, 0.5}
	if len(rec.got) != len(want) {
		t.Fatalf("platform saw %v, want %v", rec.got, want)
	}
	for i, v := range want {
		if rec.got[i] != v {
			t.Fatalf("platform call %d = %v, want %v", i, rec.got[i], v)
		}
	}
	if rec.sawN {
		t.Fatal("NaN reached the platform; it must be filtered by the setter")
	}
}
