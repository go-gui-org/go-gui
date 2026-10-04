//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"log"
	"sync"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/decor"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// The window's role and frame on Wayland (#919 phase 6).
//
// A Wayland client draws its own window frame unless the compositor offers
// to (xdg-decoration). So a toplevel gets one of three set-ups:
//
//  1. xdg-decoration offered (KDE, wlroots): a plain xdg_toplevel, and the
//     compositor draws the frame. DecorationNone asks for client-side
//     decorations instead, which here means none.
//  2. No xdg-decoration (GNOME, weston) and libdecor installed: libdecor
//     makes the xdg_toplevel and draws a frame around the surface with its
//     plugin (GTK on GNOME). Configure events then come through libdecor.
//     libdecor with no plugin installed falls back to drawing nothing, and
//     says so only on stderr; a warning names the missing package once.
//  3. Neither: a plain xdg_toplevel with no frame. A warning says so once.
//     DecorationNone takes this path on purpose, with no warning.
//
// Move and resize work in all three; with no frame only an app that calls
// StartWindowDrag (a custom title bar) can be moved.

// wlNoFrameOnce warns once that windows have no frame.
var wlNoFrameOnce sync.Once

// makeToplevel gives ww.surface its xdg_toplevel role, a frame by the rules
// above, its title, app id and size limits, and makes the first commit,
// which asks for the first configure.
func (ww *wlWindow) makeToplevel(cfg gui.WindowCfg) error {
	d := ww.d
	frame := cfg.Decorations != gui.DecorationNone
	if frame && !d.decoMgr.Valid() {
		if ctx := d.libdecor(); ctx != nil {
			// Still go through libdecor with no plugin: its fallback
			// handles configure like a plain toplevel, and a wrong guess
			// about the plugin directory then costs only the warning.
			if !decor.HasPlugin() {
				wlNoFrameOnce.Do(func() {
					log.Printf("gl: wayland: the compositor draws no window frames and libdecor " +
						"has no decoration plugin; windows have no frame " +
						"(install libdecor-0-plugin-1-gtk, or the cairo plugin)")
				})
			}
			return ww.makeDecorFrame(ctx, cfg)
		}
		wlNoFrameOnce.Do(func() {
			log.Printf("gl: wayland: the compositor draws no window frames and libdecor " +
				"is not installed; windows have no frame (install libdecor-0-0 and a plugin)")
		})
	}

	ww.xdgSurface = d.wmBase.GetXdgSurface(ww.surface)
	ww.xdgSurface.SetHandlers(wl.XdgSurfaceHandlers{Configure: ww.applyConfigure})
	ww.toplevel = ww.xdgSurface.GetToplevel()
	ww.toplevel.SetHandlers(wl.XdgToplevelHandlers{
		Configure: func(width, height int32, states []byte) {
			ww.pendW, ww.pendH = width, height
			ww.pendActive = statesHave(states, wl.XdgToplevelStateActivated)
		},
		Close: func() { gui.DispatchCloseRequest(ww.b.plat.w) },
	})
	if d.decoMgr.Valid() {
		// Made before the first commit, as the protocol requires.
		ww.deco = d.decoMgr.GetToplevelDecoration(ww.toplevel)
		mode := uint32(wl.ZxdgToplevelDecorationV1ModeServerSide)
		if !frame {
			mode = wl.ZxdgToplevelDecorationV1ModeClientSide
		}
		ww.deco.SetMode(mode)
	}
	ww.setRoleTitle(wlTitle(cfg.Title))
	if cfg.WMClass != "" {
		ww.toplevel.SetAppId(cfg.WMClass)
	}
	// Limits are in logical pixels, the unit xdg_toplevel uses. A zero
	// axis means "no limit" in both requests.
	limits := gui.WindowSizeLimits(cfg)
	if limits.MinW > 0 || limits.MinH > 0 {
		ww.toplevel.SetMinSize(int32(limits.MinW), int32(limits.MinH))
	}
	if limits.MaxW > 0 || limits.MaxH > 0 {
		ww.toplevel.SetMaxSize(int32(limits.MaxW), int32(limits.MaxH))
	}
	ww.surface.Commit()
	return nil
}

// makeDecorFrame is makeToplevel through libdecor.
func (ww *wlWindow) makeDecorFrame(ctx *decor.Context, cfg gui.WindowCfg) error {
	f, err := ctx.Decorate(ww.surface.Ptr(), wlFrameEvents{ww})
	if err != nil {
		return err
	}
	ww.frame = f
	f.SetTitle(wlTitle(cfg.Title))
	if cfg.WMClass != "" {
		f.SetAppID(cfg.WMClass)
	}
	// Content sizes: the frame is added around them.
	limits := gui.WindowSizeLimits(cfg)
	if limits.MinW > 0 || limits.MinH > 0 {
		f.SetMinContentSize(int32(limits.MinW), int32(limits.MinH))
	}
	if limits.MaxW > 0 || limits.MaxH > 0 {
		f.SetMaxContentSize(int32(limits.MaxW), int32(limits.MaxH))
	}
	f.Map()
	return nil
}

// wlTitle is the title a window starts with.
func wlTitle(t string) string {
	if t == "" {
		return "go-gui"
	}
	return t
}

// setRoleTitle sends a title to whichever object carries it.
func (ww *wlWindow) setRoleTitle(t string) {
	if ww.frame != nil {
		ww.frame.SetTitle(t)
		return
	}
	ww.toplevel.SetTitle(t)
}

// destroyRole frees the toplevel role, children before parents.
func (ww *wlWindow) destroyRole() {
	if ww.frame != nil {
		// libdecor owns the xdg objects; this frees them.
		ww.frame.Unref()
		ww.frame = nil
	}
	if ww.deco.Valid() {
		ww.deco.Destroy()
	}
	if ww.toplevel.Valid() {
		ww.toplevel.Destroy()
	}
	if ww.xdgSurface.Valid() {
		ww.xdgSurface.Destroy()
	}
}

// wlFrameEvents takes a libdecor frame's events for ww.
type wlFrameEvents struct{ ww *wlWindow }

// Configure is the libdecor form of xdg_toplevel.configure followed by
// xdg_surface.configure: commit the size (which acknowledges it), then
// apply it as applyConfigure does.
func (e wlFrameEvents) Configure(c decor.Configuration) {
	ww := e.ww
	w, h, _ := c.ContentSize() // 0 when the client picks
	w, h = configureSize(w, h, ww.logW, ww.logH)
	if st, ok := c.WindowState(); ok {
		ww.pendActive = st&decor.StateActive != 0
	}
	ww.frame.Commit(w, h, &c)
	ww.configured = true
	ww.resize(w, h, ww.scale120)
	ww.syncFocus()
	ww.dirty = true
}

func (e wlFrameEvents) Close() { gui.DispatchCloseRequest(e.ww.b.plat.w) }

// Commit: the frame changed, and shows only with a commit of the surface,
// which the next frame makes.
func (e wlFrameEvents) Commit() { e.ww.dirty = true }

// libdecor returns the display's libdecor instance, made on first use, or
// nil when libdecor is missing or failed. It is tried once.
func (d *wlDisplay) libdecor() *decor.Context {
	if d.decorTried {
		return d.decor
	}
	d.decorTried = true
	decor.ErrorFunc = func(code int, msg string) {
		log.Printf("gl: wayland: libdecor error %d: %s", code, msg)
	}
	ctx, err := decor.New(d.conn.Display.Ptr())
	if err != nil {
		return nil
	}
	d.decor = ctx
	return ctx
}

// wlResizeEdges maps gui.WindowEdge, which follows the _NET_WM_MOVERESIZE
// order, to xdg_toplevel resize edges, which are bit flags.
var wlResizeEdges = [...]uint32{
	gui.EdgeTopLeft:     wl.XdgToplevelResizeEdgeTopLeft,
	gui.EdgeTop:         wl.XdgToplevelResizeEdgeTop,
	gui.EdgeTopRight:    wl.XdgToplevelResizeEdgeTopRight,
	gui.EdgeRight:       wl.XdgToplevelResizeEdgeRight,
	gui.EdgeBottomRight: wl.XdgToplevelResizeEdgeBottomRight,
	gui.EdgeBottom:      wl.XdgToplevelResizeEdgeBottom,
	gui.EdgeBottomLeft:  wl.XdgToplevelResizeEdgeBottomLeft,
	gui.EdgeLeft:        wl.XdgToplevelResizeEdgeLeft,
}

// wlDecorEdges is the same map for libdecor, which numbers edges its own
// way.
var wlDecorEdges = [...]uint32{
	gui.EdgeTopLeft:     decor.EdgeTopLeft,
	gui.EdgeTop:         decor.EdgeTop,
	gui.EdgeTopRight:    decor.EdgeTopRight,
	gui.EdgeRight:       decor.EdgeRight,
	gui.EdgeBottomRight: decor.EdgeBottomRight,
	gui.EdgeBottom:      decor.EdgeBottom,
	gui.EdgeBottomLeft:  decor.EdgeBottomLeft,
	gui.EdgeLeft:        decor.EdgeLeft,
}

// moveResize starts a compositor move (direction netMoveResizeMove) or a
// resize from one edge (a gui.WindowEdge). It needs a button held on this
// window: the compositor checks the serial against the press it sent, and
// ignores a request with a stale one.
func (ww *wlWindow) moveResize(direction uint32) {
	s := ww.d.seat
	if s == nil || s.ptrFocus != ww.b || s.buttons == 0 {
		return
	}
	switch {
	case direction == netMoveResizeMove && ww.frame != nil:
		ww.frame.Move(s.seat.Ptr(), s.pressSerial)
	case direction == netMoveResizeMove:
		ww.toplevel.Move(s.seat, s.pressSerial)
	case int(direction) >= len(wlResizeEdges):
	case ww.frame != nil:
		ww.frame.Resize(s.seat.Ptr(), s.pressSerial, wlDecorEdges[direction])
	default:
		ww.toplevel.Resize(s.seat, s.pressSerial, wlResizeEdges[direction])
	}
}
