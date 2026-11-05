//go:build linux && !js && !android

package gl

import (
	"encoding/binary"

	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

// Window state and placement on X11 (#1171, #1220,
// docs/specs/window-state.md). A Wayland window goes to its wlWindow.
//
// Everything but the size is a request to the window manager, which
// answers through the event queue, so no call here delivers an event
// before it returns. The state comes back as a _NET_WM_STATE
// PropertyNotify, whoever changed it.

const (
	// _NET_WM_STATE actions.
	netWMStateRemove = 0
	netWMStateAdd    = 1
	// Normal-application source indication, the last _NET_WM_STATE word.
	netWMStateSourceApp = 1

	// wmIconicState is WM_CHANGE_STATE's IconicState (ICCCM 4.1.4).
	wmIconicState = 3

	// _NET_MOVERESIZE_WINDOW flags. Gravity 0 keeps the window's own,
	// NorthWest here, so x, y is the frame's corner. The source
	// indication sits at bit 12.
	netMoveResizeHasX       = 1 << 8
	netMoveResizeHasY       = 1 << 9
	netMoveResizeSourceBits = netWMStateSourceApp << 12
)

// x11StateAtoms are the atoms syncWindowState reads. Interned once, in
// New: every PropertyNotify compares against them.
type x11StateAtoms struct {
	state, maxVert, maxHorz, fullscreen, hidden xproto.Atom
}

// internStateAtoms fills p.stateAtoms.
func (p *platformState) internStateAtoms() {
	p.stateAtoms = x11StateAtoms{
		state:      internAtom(p.conn, "_NET_WM_STATE"),
		maxVert:    internAtom(p.conn, "_NET_WM_STATE_MAXIMIZED_VERT"),
		maxHorz:    internAtom(p.conn, "_NET_WM_STATE_MAXIMIZED_HORZ"),
		fullscreen: internAtom(p.conn, "_NET_WM_STATE_FULLSCREEN"),
		hidden:     internAtom(p.conn, "_NET_WM_STATE_HIDDEN"),
	}
}

// sendWMMessage sends a client message for this window to the window
// manager, by way of the root window.
func (p *platformState) sendWMMessage(atom xproto.Atom, words []uint32) {
	ev := xproto.ClientMessageEvent{
		Format: 32,
		Window: p.window,
		Type:   atom,
		Data:   xproto.ClientMessageDataUnionData32New(words),
	}
	xproto.SendEvent(p.conn, false, p.root,
		xproto.EventMaskSubstructureRedirect|xproto.EventMaskSubstructureNotify,
		string(ev.Bytes()))
}

// setNetWMState adds or removes up to two _NET_WM_STATE values.
func (p *platformState) setNetWMState(action uint32, first, second xproto.Atom) {
	if p.conn == nil || p.stateAtoms.state == 0 {
		return
	}
	p.sendWMMessage(p.stateAtoms.state, []uint32{
		action, uint32(first), uint32(second), netWMStateSourceApp, 0,
	})
}

// netWMStateHas reads a _NET_WM_STATE property value. The value is in
// the client's byte order, as for every local-connection property here.
func netWMStateHas(value []byte, atom xproto.Atom) bool {
	for i := 0; i+4 <= len(value); i += 4 {
		if xproto.Atom(binary.LittleEndian.Uint32(value[i:])) == atom {
			return true
		}
	}
	return false
}

// syncWindowState reads _NET_WM_STATE and tells gui. Leaving fullscreen
// puts back the max-size hint that entering lifted. Window managers
// differ on a maximized window in fullscreen: openbox drops the state
// until it leaves, mutter keeps it. Either way the window returns to
// maximized, so the last answer from outside fullscreen stands.
func (b *Backend) syncWindowState() {
	p := &b.plat
	r, err := xproto.GetProperty(p.conn, false, p.window, p.stateAtoms.state,
		xproto.AtomAtom, 0, 64).Reply()
	if err != nil || r == nil {
		return
	}
	a := p.stateAtoms
	maximized := netWMStateHas(r.Value, a.maxVert) && netWMStateHas(r.Value, a.maxHorz)
	fullscreen := netWMStateHas(r.Value, a.fullscreen)
	p.stateHidden = netWMStateHas(r.Value, a.hidden)
	if fullscreen != p.fullscreen {
		p.fullscreen = fullscreen
		p.writeSizeHints()
	}
	if !fullscreen || maximized {
		p.maximized = maximized
	}
	gui.DispatchWindowState(p.w, p.maximized, fullscreen)
}

// iconified reports whether the window manager minimized the window.
// Some unmap it; mutter keeps it mapped and says so in _NET_WM_STATE.
// gui refuses a hidden window, so an unmapped window here is a minimized
// one.
func (p *platformState) iconified() bool {
	return p.unmapped || p.stateHidden
}

// writeSizeHints writes WM_NORMAL_HINTS at the current scale. A
// fullscreen window drops the ceiling: window managers refuse to
// fullscreen a window whose hints forbid the size, and MaxWidth bounds
// the user's drag, not a window filling its monitor.
func (p *platformState) writeSizeHints() {
	writeSizeHints(p.conn, p.window, p.hintLimits().Scaled(p.scale))
}

// hintLimits is the limits writeSizeHints writes, in logical pixels.
func (p *platformState) hintLimits() gui.SizeLimits {
	limits := p.limits
	if p.fullscreen {
		limits.MaxW, limits.MaxH = 0, 0
	}
	return limits
}

// plat is the window's platform state. With no backend it is an empty
// one: no connection and no Wayland window, so every method returns.
func (n *nativePlatform) plat() *platformState {
	if n.b == nil {
		return &platformState{}
	}
	return &n.b.plat
}

// MinimizeWindow asks for iconic state (ICCCM 4.1.4).
func (n *nativePlatform) MinimizeWindow() {
	p := n.plat()
	if p.wl != nil {
		p.wl.minimize()
		return
	}
	if p.conn == nil {
		return
	}
	if atom := internAtom(p.conn, "WM_CHANGE_STATE"); atom != 0 {
		p.sendWMMessage(atom, []uint32{wmIconicState, 0, 0, 0, 0})
	}
}

// MaximizeWindow adds both maximized states.
func (n *nativePlatform) MaximizeWindow() {
	p := n.plat()
	if p.wl != nil {
		p.wl.maximize()
		return
	}
	p.setNetWMState(netWMStateAdd, p.stateAtoms.maxVert, p.stateAtoms.maxHorz)
}

// RestoreWindow brings back a minimized window in the state it left:
// mapping does that for a window manager that unmapped it, activating
// for one that did not. Otherwise it removes both maximized states,
// unless the window is fullscreen, which Restore leaves alone.
func (n *nativePlatform) RestoreWindow() {
	p := n.plat()
	if p.wl != nil {
		p.wl.restore()
		return
	}
	if p.conn == nil {
		return
	}
	if p.iconified() {
		xproto.MapWindow(p.conn, p.window)
		if atom := internAtom(p.conn, "_NET_ACTIVE_WINDOW"); atom != 0 {
			p.sendWMMessage(atom, []uint32{netWMStateSourceApp, 0, 0, 0, 0})
		}
		return
	}
	if !p.fullscreen {
		p.setNetWMState(netWMStateRemove, p.stateAtoms.maxVert, p.stateAtoms.maxHorz)
	}
}

// SetWindowFullscreen asks for _NET_WM_STATE_FULLSCREEN. The hints go
// first on the way in and last on the way out, so the ceiling never
// applies to a fullscreen window.
func (n *nativePlatform) SetWindowFullscreen(fullscreen bool) {
	p := n.plat()
	if p.wl != nil {
		p.wl.setFullscreen(fullscreen)
		return
	}
	if p.conn == nil {
		return
	}
	if fullscreen {
		p.fullscreen = true
		p.writeSizeHints()
		p.setNetWMState(netWMStateAdd, p.stateAtoms.fullscreen, 0)
		return
	}
	p.setNetWMState(netWMStateRemove, p.stateAtoms.fullscreen, 0)
	p.fullscreen = false
	p.writeSizeHints()
}

// ResizeWindow sets the window's size. The result arrives as a
// ConfigureNotify.
func (n *nativePlatform) ResizeWindow(width, height int) {
	p := n.plat()
	if p.wl != nil {
		p.wl.resizeTo(int32(width), int32(height))
		return
	}
	if p.conn == nil {
		return
	}
	xproto.ConfigureWindow(p.conn, p.window,
		xproto.ConfigWindowWidth|xproto.ConfigWindowHeight,
		[]uint32{
			uint32(float32(width) * p.scale),
			uint32(float32(height) * p.scale),
		})
}

// MoveWindow puts the client window's corner at x, y. A window manager
// places the frame, so the frame extents come off. StaticGravity would
// name the client's corner directly, but openbox then stores client
// coordinates as the frame's, and the first restore lands the window
// off by the frame. No-op on Wayland, where a client cannot place its
// window.
func (n *nativePlatform) MoveWindow(x, y int) {
	p := n.plat()
	if p.wl != nil || p.conn == nil {
		return
	}
	atom := internAtom(p.conn, "_NET_MOVERESIZE_WINDOW")
	if atom == 0 {
		return
	}
	left, top := p.frameExtents()
	p.sendWMMessage(atom, []uint32{
		netMoveResizeHasX | netMoveResizeHasY | netMoveResizeSourceBits,
		uint32(int32(x) - left), uint32(int32(y) - top), 0, 0,
	})
}

// frameExtents is the frame's left and top width, zero for a window
// with no frame or a window manager that does not say.
func (p *platformState) frameExtents() (left, top int32) {
	atom := internAtom(p.conn, "_NET_FRAME_EXTENTS")
	if atom == 0 {
		return 0, 0
	}
	r, err := xproto.GetProperty(p.conn, false, p.window, atom,
		xproto.AtomCardinal, 0, 4).Reply()
	if err != nil || r == nil {
		return 0, 0
	}
	return frameExtentsOf(r.Value)
}

// frameExtentsOf reads a _NET_FRAME_EXTENTS value: left, right, top,
// bottom.
func frameExtentsOf(value []byte) (left, top int32) {
	if len(value) < 12 {
		return 0, 0
	}
	return int32(binary.LittleEndian.Uint32(value)),
		int32(binary.LittleEndian.Uint32(value[8:]))
}

// WindowPosition returns the client window's corner in root pixels. An
// iconified window has no place on screen, and Wayland does not say.
func (n *nativePlatform) WindowPosition() (x, y int, ok bool) {
	p := n.plat()
	if p.wl != nil || p.conn == nil || p.iconified() {
		return 0, 0, false
	}
	t, err := xproto.TranslateCoordinates(p.conn, p.window, p.root, 0, 0).Reply()
	if err != nil || t == nil {
		return 0, 0, false
	}
	return int(t.DstX), int(t.DstY), true
}
