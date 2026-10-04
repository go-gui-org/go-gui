//go:build linux && !js && !android && (amd64 || arm64)

package gl

import "testing"

// newOutputTestWindow is a window with no compositor objects, registered
// with d, that takes its scale from its outputs. Its size is 100×80
// logical pixels at scale 1.
func newOutputTestWindow(d *wlDisplay, key uintptr) *wlWindow {
	b := &Backend{}
	ww := &wlWindow{d: d, b: b, scale120: 120, logW: 100, logH: 80, outputScale: true}
	b.plat.wl = ww
	d.wins[key] = b
	return ww
}

func TestWlOutputsMaxScale(t *testing.T) {
	a, b := &wlOutput{scale: 1}, &wlOutput{scale: 2}
	for _, c := range []struct {
		outs []*wlOutput
		want int32
	}{
		{nil, 0}, // on no output: keep the current scale
		{[]*wlOutput{a}, 1},
		{[]*wlOutput{a, b}, 2},
		{[]*wlOutput{b, a}, 2},
		{[]*wlOutput{{scale: 0}}, 1},   // a broken scale counts as 1
		{[]*wlOutput{{scale: 100}}, 8}, // clamped to wlMaxScale120
	} {
		if got := wlOutputsScale(c.outs); got != c.want {
			t.Errorf("wlOutputsScale(%v) = %d, want %d", c.outs, got, c.want)
		}
	}
}

func TestWlWindowFollowsOutputs(t *testing.T) {
	d := &wlDisplay{wins: map[uintptr]*Backend{}}
	ww := newOutputTestWindow(d, 1)
	lo, hi := &wlOutput{scale: 1}, &wlOutput{scale: 2}

	ww.enterOutput(lo)
	if ww.scale120 != 120 {
		t.Fatalf("on a 1× output: scale120 = %d, want 120", ww.scale120)
	}
	ww.enterOutput(hi)
	if ww.scale120 != 240 || ww.b.plat.physW != 200 || ww.b.plat.physH != 160 {
		t.Fatalf("spanning 1× and 2×: scale120 %d, buffer %d×%d, want 240, 200×160",
			ww.scale120, ww.b.plat.physW, ww.b.plat.physH)
	}
	ww.enterOutput(hi) // a repeated enter is not a second output
	if len(ww.outputs) != 2 {
		t.Fatalf("outputs = %d after a repeated enter, want 2", len(ww.outputs))
	}
	ww.leaveOutput(hi)
	if ww.scale120 != 120 {
		t.Fatalf("back on the 1× output: scale120 = %d, want 120", ww.scale120)
	}
	ww.leaveOutput(lo)
	if ww.scale120 != 120 || len(ww.outputs) != 0 {
		t.Fatalf("on no output: scale120 %d, outputs %d, want 120 kept, 0",
			ww.scale120, len(ww.outputs))
	}
}

// A compositor that sends its own scale (wl_surface v6 or fractional
// scale) is not second-guessed by output enters.
func TestWlWindowOutputScaleOff(t *testing.T) {
	d := &wlDisplay{wins: map[uintptr]*Backend{}}
	ww := newOutputTestWindow(d, 1)
	ww.outputScale = false
	ww.enterOutput(&wlOutput{scale: 2})
	if ww.scale120 != 120 {
		t.Fatalf("scale120 = %d, want 120: the compositor's own scale rules", ww.scale120)
	}
}

func TestWlOutputScaleChange(t *testing.T) {
	d := &wlDisplay{wins: map[uintptr]*Backend{}}
	on := newOutputTestWindow(d, 1)
	off := newOutputTestWindow(d, 2)
	o, other := &wlOutput{scale: 1}, &wlOutput{scale: 1}
	on.enterOutput(o)
	off.enterOutput(other)

	// wl_output v2+: scale is pending until done.
	o.version = 2
	d.outputScaleEvent(o, 3)
	if on.scale120 != 120 {
		t.Fatalf("before done: scale120 = %d, want 120", on.scale120)
	}
	d.outputDone(o)
	if on.scale120 != 360 {
		t.Fatalf("after done: scale120 = %d, want 360", on.scale120)
	}
	if off.scale120 != 120 {
		t.Fatalf("a window on another output rescaled to %d", off.scale120)
	}

	// wl_output v1 has no done: the scale applies at once.
	other.version = 1
	d.outputScaleEvent(other, 2)
	if off.scale120 != 240 {
		t.Fatalf("v1 output: scale120 = %d, want 240", off.scale120)
	}
}

func TestWlOutputRemoved(t *testing.T) {
	d := &wlDisplay{wins: map[uintptr]*Backend{}, outputs: map[uint32]*wlOutput{}}
	ww := newOutputTestWindow(d, 1)
	lo, hi := &wlOutput{name: 7, scale: 1}, &wlOutput{name: 8, scale: 2}
	d.outputs[lo.name], d.outputs[hi.name] = lo, hi
	ww.enterOutput(lo)
	ww.enterOutput(hi)

	// The compositor sends no leave for an output it unplugs.
	d.removeGlobal(hi.name)
	if _, ok := d.outputs[hi.name]; ok {
		t.Fatal("removed output still tracked")
	}
	if len(ww.outputs) != 1 || ww.scale120 != 120 {
		t.Fatalf("after unplug: outputs %d, scale120 %d, want 1, 120",
			len(ww.outputs), ww.scale120)
	}
}
