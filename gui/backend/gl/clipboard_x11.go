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

// selectionState returns pointers to the cached text and owner flag backing
// sel, so the set/get/serve paths can treat CLIPBOARD and PRIMARY uniformly.
// Reports false for any other selection (e.g. SECONDARY), which we neither
// own nor serve.
func (p *platformState) selectionState(sel xproto.Atom) (text *string, owns *bool, ok bool) {
	// atomClipboard is 0 until the atoms are interned; without this an
	// uninitialized platformState would match a zero selection atom as
	// CLIPBOARD and claim ownership of nothing.
	if sel == 0 {
		return nil, nil, false
	}
	switch sel {
	case p.atomClipboard:
		return &p.clipboardText, &p.ownsClipboard, true
	case xproto.AtomPrimary:
		return &p.primaryText, &p.ownsPrimary, true
	}
	return nil, nil, false
}

// setSelection takes ownership of sel and caches the text so incoming
// SelectionRequest events can be served. Selections we do not model are
// ignored rather than claimed.
func setSelection(p *platformState, sel xproto.Atom, s string) {
	text, owns, ok := p.selectionState(sel)
	if !ok {
		return
	}
	*text = s
	*owns = true
	xproto.SetSelectionOwner(p.conn, p.window, sel, xproto.TimeCurrentTime)
	p.conn.Sync() // flush so the server records ownership immediately
}

// getSelection returns the current text of sel. When this process owns the
// selection the cached text is returned directly; otherwise the value is
// requested from the current owner over a dedicated connection so the
// round-trip does not reenter the main event loop.
func getSelection(p *platformState, sel xproto.Atom) string {
	text, owns, ok := p.selectionState(sel)
	if !ok {
		return ""
	}
	if *owns {
		return *text
	}
	if p.atomUTF8 == 0 || p.atomClipProp == 0 {
		return ""
	}
	if !p.ensureClipReadConn() {
		return ""
	}
	conn, win := p.clipReadConn, p.clipReadWin

	xproto.ConvertSelection(conn, win, sel, p.atomUTF8,
		p.atomClipProp, xproto.TimeCurrentTime)
	conn.Sync()

	if !waitSelectionNotify(conn, clipReadTimeout) {
		return ""
	}
	return readClipProperty(conn, win, p.atomClipProp)
}

// setClipboard publishes s as the CLIPBOARD selection (explicit copy).
func setClipboard(p *platformState, s string) { setSelection(p, p.atomClipboard, s) }

// getClipboard returns the current CLIPBOARD text.
func getClipboard(p *platformState) string { return getSelection(p, p.atomClipboard) }

// setPrimary publishes s as the PRIMARY selection — the select-to-copy buffer
// pasted with the middle mouse button, independent of CLIPBOARD. PRIMARY is a
// predefined atom, so unlike CLIPBOARD it needs no interning and works even
// before setupAtoms has run.
func setPrimary(p *platformState, s string) { setSelection(p, xproto.AtomPrimary, s) }

// getPrimary returns the current PRIMARY selection text.
func getPrimary(p *platformState) string { return getSelection(p, xproto.AtomPrimary) }

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

// serveSelectionRequest answers a SelectionRequest for a selection we own,
// honoring the TARGETS and UTF8_STRING/STRING targets per ICCCM. The value
// served depends on which selection was asked for — CLIPBOARD and PRIMARY
// hold different text — so a requestor pasting one never receives the other.
func (p *platformState) serveSelectionRequest(e xproto.SelectionRequestEvent) {
	text, _, ok := p.selectionState(e.Selection)
	if !ok {
		// A selection we never owned. Refuse rather than answering with
		// whatever happens to be in the clipboard cache.
		p.refuseSelectionRequest(e)
		return
	}
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
		b := []byte(*text)
		xproto.ChangeProperty(p.conn, xproto.PropModeReplace, e.Requestor,
			prop, e.Target, 8, uint32(len(b)), b)
	default:
		prop = 0 // refuse unsupported targets
	}
	p.sendSelectionNotify(e, prop)
}

// refuseSelectionRequest answers a request we cannot satisfy. ICCCM requires
// a SelectionNotify either way; Property 0 signals refusal, and omitting it
// would leave the requestor blocked until its own timeout expires.
func (p *platformState) refuseSelectionRequest(e xproto.SelectionRequestEvent) {
	p.sendSelectionNotify(e, 0)
}

// sendSelectionNotify replies to a SelectionRequest, naming the property the
// value was written to (or 0 for refusal).
func (p *platformState) sendSelectionNotify(e xproto.SelectionRequestEvent, prop xproto.Atom) {
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
