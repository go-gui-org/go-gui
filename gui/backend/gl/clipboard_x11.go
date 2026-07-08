//go:build linux && !js && !android

package gl

import (
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// clipReadTimeout bounds a blocking clipboard read so a missing or slow
// selection owner cannot hang the UI thread.
const clipReadTimeout = time.Second

// setClipboard takes ownership of the X11 CLIPBOARD selection and caches
// the text so incoming SelectionRequest events can be served.
func setClipboard(p *platformState, s string) {
	p.clipboardText = s
	if p.atomClipboard == 0 {
		return
	}
	p.ownsClipboard = true
	xproto.SetSelectionOwner(p.conn, p.window, p.atomClipboard,
		xproto.TimeCurrentTime)
	p.conn.Sync() // flush so the server records ownership immediately
}

// getClipboard returns the current CLIPBOARD text. When this process owns
// the selection the cached text is returned directly; otherwise the value
// is requested from the current owner over a dedicated connection so the
// round-trip does not reenter the main event loop.
func getClipboard(p *platformState) string {
	if p.ownsClipboard {
		return p.clipboardText
	}
	if p.atomClipboard == 0 || p.atomUTF8 == 0 || p.atomClipProp == 0 {
		return ""
	}
	if !p.ensureClipReadConn() {
		return ""
	}
	conn, win := p.clipReadConn, p.clipReadWin

	xproto.ConvertSelection(conn, win, p.atomClipboard, p.atomUTF8,
		p.atomClipProp, xproto.TimeCurrentTime)
	conn.Sync()

	if !waitSelectionNotify(conn, clipReadTimeout) {
		return ""
	}
	return readClipProperty(conn, win, p.atomClipProp)
}

// ensureClipReadConn lazily opens the dedicated read connection and its
// requestor window. Returns false if either could not be created.
func (p *platformState) ensureClipReadConn() bool {
	if p.clipReadConn != nil {
		return true
	}
	conn, err := xgb.NewConn()
	if err != nil {
		return false
	}
	root := xproto.Setup(conn).DefaultScreen(conn).Root
	wid, err := conn.NewId()
	if err != nil {
		conn.Close()
		return false
	}
	win := xproto.Window(wid)
	xproto.CreateWindow(conn, 0, win, root, 0, 0, 1, 1, 0,
		xproto.WindowClassInputOnly, 0, 0, nil)
	p.clipReadConn = conn
	p.clipReadWin = win
	return true
}

// waitSelectionNotify polls conn until a SelectionNotify arrives or the
// deadline elapses. Reports whether the notify was seen.
func waitSelectionNotify(conn *xgb.Conn, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ev, err := conn.PollForEvent()
		if err != nil {
			return false
		}
		if ev == nil {
			time.Sleep(2 * time.Millisecond)
			continue
		}
		if _, ok := ev.(xproto.SelectionNotifyEvent); ok {
			return true
		}
	}
	return false
}

// readClipProperty pages the transfer property off win and returns its
// UTF-8 contents, deleting the property when done. INCR (very large
// payloads streamed by the owner) is not supported and yields "".
func readClipProperty(conn *xgb.Conn, win xproto.Window, prop xproto.Atom) string {
	const chunkWords = 1 << 18 // 256K 32-bit words = 1 MiB per request
	var out []byte
	var offset uint32
	for {
		reply, err := xproto.GetProperty(conn, false, win, prop,
			xproto.GetPropertyTypeAny, offset, chunkWords).Reply()
		if err != nil || reply == nil || reply.Format == 0 {
			break
		}
		out = append(out, reply.Value...)
		if reply.BytesAfter == 0 {
			break
		}
		// long-offset is in 32-bit units; ValueLen counts Format/8 (=1)
		// byte units here, so advance by the words just read.
		offset += reply.ValueLen / 4
	}
	xproto.DeleteProperty(conn, win, prop)
	return string(out)
}

// serveSelectionRequest answers a SelectionRequest for the CLIPBOARD we
// own, honoring the TARGETS and UTF8_STRING/STRING targets per ICCCM.
func (p *platformState) serveSelectionRequest(e xproto.SelectionRequestEvent) {
	prop := e.Property
	if prop == 0 { // obsolete client: fall back to the target atom
		prop = e.Target
	}
	switch e.Target {
	case p.atomTargets:
		data := make([]byte, 8)
		xgb.Put32(data[0:], uint32(p.atomTargets))
		xgb.Put32(data[4:], uint32(p.atomUTF8))
		xproto.ChangeProperty(p.conn, xproto.PropModeReplace, e.Requestor,
			prop, xproto.AtomAtom, 32, 2, data)
	case p.atomUTF8, xproto.AtomString:
		b := []byte(p.clipboardText)
		xproto.ChangeProperty(p.conn, xproto.PropModeReplace, e.Requestor,
			prop, e.Target, 8, uint32(len(b)), b)
	default:
		prop = 0 // refuse unsupported targets
	}
	notify := xproto.SelectionNotifyEvent{
		Time:      e.Time,
		Requestor: e.Requestor,
		Selection: e.Selection,
		Target:    e.Target,
		Property:  prop,
	}
	xproto.SendEvent(p.conn, false, e.Requestor, 0, string(notify.Bytes()))
	p.conn.Sync()
}
