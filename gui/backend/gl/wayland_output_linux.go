//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"slices"

	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// Scale on older compositors.
//
// A compositor tells a window its scale in one of two ways go-gui reads
// first: wp_fractional_scale_v1 (sway, mutter, KWin) or
// wl_surface.preferred_buffer_scale (wl_surface v6). Older compositors,
// among them Muffin 6.6 (Cinnamon) and weston 13, offer neither. There the
// client is expected to work the scale out itself: each wl_output sends
// its integer scale, and wl_surface.enter and .leave say which outputs the
// window is on. Without this, a window on a 2× output renders at 1× and
// the compositor stretches it, so the content is blurred inside a libdecor
// frame that is sharp (libdecor does this itself).
//
// The window takes the highest scale of the outputs it is on, as GTK and
// SDL do: a window spanning a 1× and a 2× output is sharp on the 2× one
// and downscaled on the other. A window on no output keeps its scale.

// wlOutputVersion is the highest wl_output version bound: v2 adds done
// (scale changes apply as one), v3 the release destructor. v4 only adds
// names, which go-gui does not read.
const wlOutputVersion = 3

// wlOutput is one bound wl_output and the integer scale it last reported.
type wlOutput struct {
	proxy   wl.Output
	name    uint32 // registry global name, to notice its removal
	version uint32
	// scale is the applied scale; pending holds one sent before done
	// (v2+), 0 when none is pending.
	scale, pending int32
}

// addOutput binds a wl_output global. Every output is bound, also one
// plugged in later, so a window that moves onto it can be told its scale.
func (d *wlDisplay) addOutput(name, version uint32) {
	o := &wlOutput{name: name, version: min(version, wlOutputVersion), scale: 1}
	o.proxy = wl.Output{Proxy: d.registry.Bind(name, &wl.OutputInterface, o.version)}
	o.proxy.SetHandlers(wl.OutputHandlers{
		Scale: func(factor int32) { d.outputScaleEvent(o, factor) },
		Done:  func() { d.outputDone(o) },
	})
	d.outputs[name] = o
}

// outputFor returns the tracked output behind a proxy an event named, or
// nil. An output bound by someone else on the connection (libdecor's
// plugin binds its own) is not tracked; the compositor sends an enter for
// each binding, so go-gui's own one arrives too.
func (d *wlDisplay) outputFor(px wl.Output) *wlOutput {
	for _, o := range d.outputs {
		if o.proxy.Ptr() == px.Ptr() {
			return o
		}
	}
	return nil
}

// outputScaleEvent handles wl_output.scale. From v2 the change waits for
// done, which ends a batch of output events; v1 has no done.
func (d *wlDisplay) outputScaleEvent(o *wlOutput, factor int32) {
	o.pending = factor
	if o.version < 2 {
		d.outputDone(o)
	}
}

// outputDone handles wl_output.done: applies a pending scale and rescales
// the windows on the output.
func (d *wlDisplay) outputDone(o *wlOutput) {
	if o.pending == 0 {
		return
	}
	o.scale, o.pending = o.pending, 0
	for _, b := range d.wins {
		if ww := b.plat.wl; slices.Contains(ww.outputs, o) {
			ww.applyOutputScale()
		}
	}
}

// removeOutput forgets an unplugged output. The compositor sends no leave
// for it, so it is dropped from every window here.
func (d *wlDisplay) removeOutput(o *wlOutput) {
	delete(d.outputs, o.name)
	for _, b := range d.wins {
		b.plat.wl.leaveOutput(o)
	}
	o.destroy()
}

// destroy frees the output proxy, with release where the version has it
// so the compositor frees its side too.
func (o *wlOutput) destroy() {
	if !o.proxy.Valid() {
		return
	}
	if o.version >= 3 {
		o.proxy.Release()
	} else {
		o.proxy.DestroyProxy()
	}
	o.proxy = wl.Output{}
}

// enterOutput handles wl_surface.enter for a tracked output.
func (ww *wlWindow) enterOutput(o *wlOutput) {
	if slices.Contains(ww.outputs, o) {
		return
	}
	ww.outputs = append(ww.outputs, o)
	ww.applyOutputScale()
}

// leaveOutput handles wl_surface.leave, and an output's removal.
func (ww *wlWindow) leaveOutput(o *wlOutput) {
	i := slices.Index(ww.outputs, o)
	if i < 0 {
		return
	}
	ww.outputs = slices.Delete(ww.outputs, i, i+1)
	ww.applyOutputScale()
}

// applyOutputScale sets the window's scale from its outputs, on a
// compositor that sends no scale of its own (outputScale).
func (ww *wlWindow) applyOutputScale() {
	if !ww.outputScale {
		return
	}
	s := wlOutputsScale(ww.outputs)
	if s == 0 {
		return // on no output: keep the scale
	}
	ww.resize(ww.logW, ww.logH, s*120)
	ww.dirty = true
}

// wlOutputsScale is the highest scale of outs, each at least 1 and at most
// wlMaxScale120/120, or 0 when outs is empty.
func wlOutputsScale(outs []*wlOutput) int32 {
	var s int32
	for _, o := range outs {
		s = max(s, min(max(o.scale, 1), wlMaxScale120/120))
	}
	return s
}
