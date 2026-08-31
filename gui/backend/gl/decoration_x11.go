//go:build linux && !js && !android

package gl

import (
	"unsafe"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

// X11 has no decoration property of its own. Every mainstream window
// manager still reads the Motif hints CDE left behind, so that is how a
// client asks for an undecorated frame.
const (
	// mwmHintsDecorations marks the decorations field as meaningful.
	mwmHintsDecorations = 1 << 1
	// mwmDecorNone asks for no decorations at all.
	mwmDecorNone = 0
	// netMoveResizeMove is the _NET_WM_MOVERESIZE direction for a move.
	// Directions 0..7 are the edges and corners and match WindowEdge.
	netMoveResizeMove = 8
	// netMoveResizeSourceApp marks the request as coming from a normal
	// application rather than a pager.
	netMoveResizeSourceApp = 1
)

// motifHintsUndecorated builds the _MOTIF_WM_HINTS payload that strips
// the frame. The property is five CARDINALs: flags, functions,
// decorations, input mode, status.
func motifHintsUndecorated() []uint32 {
	return []uint32{mwmHintsDecorations, 0, mwmDecorNone, 0, 0}
}

// setWindowDecorations applies cfg.Decorations to a freshly created,
// not-yet-mapped window. Only DecorationNone does anything on X11:
// DecorationHiddenTitlebar has no portable equivalent, so it degrades
// to the standard frame the way vibrancy degrades off macOS.
//
// The hints have to be in place before MapWindow — a window manager
// reads them when it reparents, and several ignore a later change.
func setWindowDecorations(conn *xgb.Conn, win xproto.Window, d gui.WindowDecoration) {
	if d != gui.DecorationNone {
		return
	}
	atom := internAtom(conn, "_MOTIF_WM_HINTS")
	if atom == 0 {
		return
	}
	hints := motifHintsUndecorated()
	// The property's type is the atom itself, and its values are in the
	// client's byte order — the same local-connection assumption
	// setWindowIcon makes for _NET_WM_ICON.
	xproto.ChangeProperty(conn, xproto.PropModeReplace, win,
		atom, atom, 32, uint32(len(hints)),
		unsafe.Slice((*byte)(unsafe.Pointer(&hints[0])), len(hints)*4))
}

// StartWindowDrag asks the window manager to run its own move loop,
// which keeps the WM's snapping and edge tiling working instead of
// fighting them with client-side ConfigureWindow calls.
func (n *nativePlatform) StartWindowDrag() {
	n.b.plat.startMoveResize(netMoveResizeMove)
}

// StartWindowResize asks the window manager to run its own resize loop
// for one edge or corner. WindowEdge values are the protocol's own
// direction codes, so no mapping table is needed.
func (n *nativePlatform) StartWindowResize(edge gui.WindowEdge) {
	if edge > gui.EdgeLeft {
		return
	}
	n.b.plat.startMoveResize(uint32(edge))
}

// startMoveResize sends _NET_WM_MOVERESIZE for the last button press.
// The press left an implicit pointer grab on the app, so the grab is
// released first or the window manager never sees the drag.
func (p *platformState) startMoveResize(direction uint32) {
	if p.conn == nil || !p.havePress {
		return
	}
	atom := internAtom(p.conn, "_NET_WM_MOVERESIZE")
	if atom == 0 {
		return
	}
	xproto.UngrabPointer(p.conn, xproto.TimeCurrentTime)
	data := xproto.ClientMessageDataUnionData32New([]uint32{
		uint32(int32(p.pressRootX)),
		uint32(int32(p.pressRootY)),
		direction,
		uint32(p.pressButton),
		netMoveResizeSourceApp,
	})
	ev := xproto.ClientMessageEvent{
		Format: 32,
		Window: p.window,
		Type:   atom,
		Data:   data,
	}
	// Routed through the root window: the request is for the window
	// manager, not for us.
	xproto.SendEvent(p.conn, false, p.root,
		xproto.EventMaskSubstructureRedirect|xproto.EventMaskSubstructureNotify,
		string(ev.Bytes()))
	// The gesture is now the WM's, so the next press has to re-arm it.
	p.havePress = false
}
