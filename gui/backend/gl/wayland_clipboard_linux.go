//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"errors"
	"io"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// The clipboard and the primary selection on Wayland (#919 phase 6).
//
// Both work the same way, through two protocols: wl_data_device for the
// clipboard (core) and primary-selection-v1 for the primary selection
// (filled by selecting text, pasted with the middle button).
//
//   - Copy: the client makes a source, lists the MIME types it offers, and
//     sets it as the selection, naming the serial of a recent input event.
//     When another client pastes, the compositor sends the source a pipe to
//     write the text into. The source stays ours until it is cancelled
//     (another client copied).
//   - Paste: for each new selection, the compositor sends an offer and its
//     MIME types. Reading asks the offer to write into a pipe and reads
//     the other end, bounded by clipReadTimeout and maxClipboardChars.
//
// Text we copied is read back from memory, without a pipe: the source is
// served by this thread, which would be blocked in the read.
//
// The wl package is single-threaded, so a copy or paste from another
// goroutine is handed to the run loop (wlDisplay.post).

// wlSelKind names one of the two selections.
type wlSelKind int

const (
	wlSelClipboard wlSelKind = iota
	wlSelPrimary
)

// wlTextMimes are the text MIME types offered on copy, preferred first.
// UTF8_STRING, TEXT and STRING are the X11 names, for XWayland clients.
var wlTextMimes = [...]string{
	"text/plain;charset=utf-8", "UTF8_STRING", "text/plain", "TEXT", "STRING",
}

// wlMimeRank ranks a MIME type for reading: higher is better, 0 is not
// text. Plain text/plain is taken as UTF-8, as every toolkit sends it.
func wlMimeRank(mime string) int {
	switch mime {
	case "text/plain;charset=utf-8":
		return 3
	case "UTF8_STRING":
		return 2
	case "text/plain":
		return 1
	}
	return 0
}

// wlOfferProxy is what a clipboard and a primary-selection offer have in
// common.
type wlOfferProxy interface {
	Receive(mimeType string, fd int)
	Destroy()
	Ptr() uintptr
}

// wlOffer is an offer from another client and the best text type it has.
type wlOffer struct {
	px   wlOfferProxy
	mime string // "" when it has no text
	rank int
}

func (o *wlOffer) addMime(mime string) {
	if r := wlMimeRank(mime); r > o.rank {
		o.mime, o.rank = strings.Clone(mime), r
	}
}

// wlSelection is the state of one selection on the seat.
type wlSelection struct {
	// offer is the current selection from another client, nil when the
	// selection is empty or ours.
	offer *wlOffer
	// source is our source while we own the selection; text is what it
	// serves.
	source interface{ Destroy() }
	text   string
}

// owned reports whether the selection is ours.
func (sel *wlSelection) owned() bool { return sel.source != nil }

// wlClip is the selection state of the seat.
type wlClip struct {
	dataDev    wl.DataDevice
	primaryDev wl.ZwpPrimarySelectionDeviceV1
	sel        [2]wlSelection // by wlSelKind
	// offers holds every live offer by proxy, from its data_offer event
	// until it is replaced or dropped. dnd is the offer of a drag over
	// one of our windows; drops are not supported, so it is only freed.
	offers map[uintptr]*wlOffer
	dnd    uintptr
}

// attachSelection gets the seat's selection devices once the managers
// are bound. The first seat is bound during the first round trip, before
// the managers, so openWaylandDisplay calls this again afterwards.
func (s *wlSeat) attachSelection() {
	c := &s.clip
	if c.offers == nil {
		c.offers = map[uintptr]*wlOffer{}
	}
	if !c.dataDev.Valid() && s.d.dataMgr.Valid() {
		c.dataDev = s.d.dataMgr.GetDataDevice(s.seat)
		c.dataDev.SetHandlers(wl.DataDeviceHandlers{
			DataOffer: func(o wl.DataOffer) {
				off := s.trackOffer(o)
				o.SetHandlers(wl.DataOfferHandlers{Offer: off.addMime})
			},
			Selection: func(o wl.DataOffer) { s.setOffer(wlSelClipboard, o.Ptr()) },
			Enter: func(_ uint32, _ wl.Surface, _, _ wl.Fixed, o wl.DataOffer) {
				s.dropOffer(c.dnd)
				c.dnd = o.Ptr()
			},
			Leave: func() { s.dropOffer(c.dnd); c.dnd = 0 },
			Drop:  func() { s.dropOffer(c.dnd); c.dnd = 0 },
		})
	}
	if !c.primaryDev.Valid() && s.d.primaryMgr.Valid() {
		c.primaryDev = s.d.primaryMgr.GetDevice(s.seat)
		c.primaryDev.SetHandlers(wl.ZwpPrimarySelectionDeviceV1Handlers{
			DataOffer: func(o wl.ZwpPrimarySelectionOfferV1) {
				off := s.trackOffer(o)
				o.SetHandlers(wl.ZwpPrimarySelectionOfferV1Handlers{Offer: off.addMime})
			},
			Selection: func(o wl.ZwpPrimarySelectionOfferV1) { s.setOffer(wlSelPrimary, o.Ptr()) },
		})
	}
}

// hasSelection reports whether the seat can reach the system selection of
// kind. Without it, the display keeps the text in the process.
func (s *wlSeat) hasSelection(kind wlSelKind) bool {
	if kind == wlSelPrimary {
		return s.clip.primaryDev.Valid()
	}
	return s.clip.dataDev.Valid()
}

func (s *wlSeat) trackOffer(px wlOfferProxy) *wlOffer {
	o := &wlOffer{px: px}
	s.clip.offers[px.Ptr()] = o
	return o
}

// dropOffer destroys an offer. ptr 0 is no offer.
func (s *wlSeat) dropOffer(ptr uintptr) {
	if o, ok := s.clip.offers[ptr]; ok {
		o.px.Destroy()
		delete(s.clip.offers, ptr)
	}
}

// setOffer makes ptr (0 for an empty selection) the selection's offer and
// frees the one it replaces.
func (s *wlSeat) setOffer(kind wlSelKind, ptr uintptr) {
	sel := &s.clip.sel[kind]
	if sel.offer != nil {
		if p := sel.offer.px.Ptr(); p != ptr {
			s.dropOffer(p)
		}
	}
	sel.offer = s.clip.offers[ptr]
}

// writeSelection makes text the selection, served by a new source.
func (s *wlSeat) writeSelection(kind wlSelKind, text string) {
	sel := &s.clip.sel[kind]
	old := sel.source
	var src interface{ Destroy() }
	send := func(_ string, fd int) { go wlServeText(fd, text) }
	if kind == wlSelPrimary {
		p := s.d.primaryMgr.CreateSource()
		for _, m := range wlTextMimes {
			p.Offer(m)
		}
		p.SetHandlers(wl.ZwpPrimarySelectionSourceV1Handlers{
			Send:      send,
			Cancelled: func() { s.cancelSource(kind, p) },
		})
		s.clip.primaryDev.SetSelection(p, s.serial)
		src = p
	} else {
		p := s.d.dataMgr.CreateDataSource()
		for _, m := range wlTextMimes {
			p.Offer(m)
		}
		p.SetHandlers(wl.DataSourceHandlers{
			Send:      send,
			Cancelled: func() { s.cancelSource(kind, p) },
		})
		s.clip.dataDev.SetSelection(p, s.serial)
		src = p
	}
	sel.source, sel.text = src, text
	if old != nil {
		// Replaced: the compositor would cancel it next.
		old.Destroy()
	}
}

// cancelSource handles a source's cancelled event: another client took the
// selection (or ours was refused, a stale serial), so it is no longer ours.
func (s *wlSeat) cancelSource(kind wlSelKind, src interface{ Destroy() }) {
	sel := &s.clip.sel[kind]
	if sel.source == src {
		sel.source, sel.text = nil, ""
	}
	src.Destroy()
}

// readSelection returns the selection's text: from memory when it is
// ours, else read from the offer through a pipe.
func (s *wlSeat) readSelection(kind wlSelKind) string {
	sel := &s.clip.sel[kind]
	if sel.owned() {
		return sel.text
	}
	o := sel.offer
	if o == nil || o.mime == "" {
		return ""
	}
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC); err != nil {
		return ""
	}
	o.px.Receive(o.mime, p[1])
	// The write end now belongs to the other client; ours must close,
	// or the read never sees EOF.
	_ = unix.Close(p[1])
	if err := s.d.conn.Flush(); err != nil {
		_ = unix.Close(p[0])
		return ""
	}
	text, err := wlReadText(p[0], clipReadTimeout, maxClipboardChars)
	if err != nil {
		log.Printf("gl: wayland: read selection: %v", err)
	}
	return text
}

// destroyClip frees the selection devices, sources and offers.
func (s *wlSeat) destroyClip() {
	c := &s.clip
	for i := range c.sel {
		if c.sel[i].source != nil {
			c.sel[i].source.Destroy()
		}
		c.sel[i] = wlSelection{}
	}
	for ptr := range c.offers {
		s.dropOffer(ptr)
	}
	c.dnd = 0
	if c.dataDev.Valid() {
		if c.dataDev.Version() >= 2 {
			c.dataDev.Release()
		} else {
			c.dataDev.DestroyProxy()
		}
	}
	if c.primaryDev.Valid() {
		c.primaryDev.Destroy()
	}
	c.dataDev, c.primaryDev = wl.DataDevice{}, wl.ZwpPrimarySelectionDeviceV1{}
}

// wlServeTimeout bounds how long a paste may take to read what we copied.
// The write runs on its own goroutine, so a reader that never reads only
// costs that goroutine until then.
const wlServeTimeout = 5 * time.Second

// wlServeText writes text into fd, a pipe from another client's paste, and
// closes it. It runs on its own goroutine: a large text must not block the
// loop.
func wlServeText(fd int, text string) {
	// Only this end of the pipe turns non-blocking, so a deadline works;
	// the reader's end is its own open file description.
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return
	}
	f := os.NewFile(uintptr(fd), "wayland-selection")
	defer func() { _ = f.Close() }()
	_ = f.SetWriteDeadline(time.Now().Add(wlServeTimeout))
	// EPIPE (the reader went away) is not worth a log line.
	_, _ = io.WriteString(f, text)
}

// wlReadText reads fd to EOF, at most limit bytes and for at most
// timeout, then closes it. A hostile or stuck source must not grow the
// buffer without bound or hang the loop.
func wlReadText(fd int, timeout time.Duration, limit int) (string, error) {
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return "", err
	}
	f := os.NewFile(uintptr(fd), "wayland-selection")
	defer func() { _ = f.Close() }()
	if err := f.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return "", err
	}
	var out []byte
	page := make([]byte, 64<<10)
	for {
		n, err := f.Read(page)
		var more bool
		out, more = appendClipChunk(out, page[:n], limit)
		switch {
		case !more:
			return string(out), nil
		case errors.Is(err, io.EOF):
			return string(out), nil
		case errors.Is(err, os.ErrDeadlineExceeded):
			return string(out), errors.New("source did not finish in time")
		case err != nil:
			return string(out), err
		}
	}
}

// --- gui hooks ---

// wlOnMainThread reports whether the caller runs on the thread b's loop
// runs on. Every Wayland window shares that thread.
func wlOnMainThread(b *Backend) bool {
	return b.plat.lockedTid != 0 && syscall.Gettid() == b.plat.lockedTid
}

// setSelection is the copy hook. Any goroutine.
func (d *wlDisplay) setSelection(b *Backend, kind wlSelKind, text string) {
	if !wlOnMainThread(b) {
		d.post(func() { d.setSelection(b, kind, text) })
		return
	}
	if s := d.seat; s != nil && s.hasSelection(kind) {
		s.writeSelection(kind, text)
		return
	}
	d.localSel[kind] = text
}

// getSelection is the paste hook. Any goroutine; another one waits for
// the loop to read it, bounded so a loop that is not running cannot hang
// the caller.
func (d *wlDisplay) getSelection(b *Backend, kind wlSelKind) string {
	if !wlOnMainThread(b) {
		out := make(chan string, 1)
		d.post(func() { out <- d.getSelection(b, kind) })
		select {
		case t := <-out:
			return t
		case <-time.After(2 * clipReadTimeout):
			return ""
		}
	}
	if s := d.seat; s != nil && s.hasSelection(kind) {
		return s.readSelection(kind)
	}
	return d.localSel[kind]
}
