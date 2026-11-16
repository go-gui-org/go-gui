//go:build linux && !js && !android

package gl

// The X11 soft upload (#1265 step 5): MIT-SHM when the server can map our
// memory, core PutImage from a reused request buffer when it cannot.
//
// Neither path allocates in proportion to the frame. Before step 5, xgb's
// xproto.PutImage built a new request buffer per band and copied the pixels
// into it: one frame of garbage per frame (4 MB at 1280x800). What remains
// is xgb's own per-request cost (a cookie, a channel and a request record,
// about 3 allocations per request), which no caller of xgb can avoid; xgb
// owns the sequence numbers, so a second writer on the connection is not an
// option.
//
// MIT-SHM: one SysV segment per window, the frame size, attached by the
// server read-only. A frame is swizzled straight into it and shown with one
// small ShmPutImage request, so the pixels never cross the socket. The
// segment is reused frame after frame without a completion event: present
// paces with a round trip after every put (pace), and the server has
// finished every request before it answers, the ShmPutImage's read of the
// segment included. repaint, which does not pace, makes the same round trip
// itself (sync).
//
// Fallback, decided per window, once: no MIT-SHM extension, no SysV IPC
// (a sandbox), or a server that cannot attach the segment (remote X, or a
// container with its own IPC namespace) all leave the window on core
// PutImage for good. A resize that cannot get a new segment does the same.

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/shm"
	"github.com/jezek/xgb/xproto"
	"golang.org/x/sys/unix"
)

// x11ShmSeg is one MIT-SHM segment, attached by this process (mem) and by
// the server (seg). It holds one w x h frame of 4-byte pixels.
type x11ShmSeg struct {
	seg  shm.Seg
	mem  []byte
	w, h int
}

// newShmSegFunc is the seam a test replaces to make the server's attach
// fail, which is what a remote X server does.
var newShmSegFunc = newShmSeg

// newShmSeg creates a segment for a w x h frame and has the server attach
// it. Any failure undoes what was done and returns the error; the caller
// falls back to core PutImage.
func newShmSeg(conn *xgb.Conn, w, h int) (*x11ShmSeg, error) {
	size := w * h * 4
	if w <= 0 || h <= 0 || size/4/h != w {
		return nil, fmt.Errorf("gl: soft: shm: bad frame size %dx%d", w, h)
	}
	id, err := unix.SysvShmGet(unix.IPC_PRIVATE, size, unix.IPC_CREAT|0o600)
	if err != nil {
		return nil, fmt.Errorf("gl: soft: shmget: %w", err)
	}
	// The segment is marked for removal on every path out of here: on
	// success once the server has attached it, so it goes away with the
	// last detach even if this process is killed.
	removeSeg := func() { _, _ = unix.SysvShmCtl(id, unix.IPC_RMID, nil) }
	mem, err := unix.SysvShmAttach(id, 0, 0)
	if err != nil {
		removeSeg()
		return nil, fmt.Errorf("gl: soft: shmat: %w", err)
	}
	segID, err := conn.NewId()
	if err != nil {
		_ = unix.SysvShmDetach(mem)
		removeSeg()
		return nil, fmt.Errorf("gl: soft: shm seg id: %w", err)
	}
	seg := shm.Seg(segID)
	// Checked, so a server that cannot see the segment says so here, as
	// an error, not later as an asynchronous one on every frame.
	if err = shm.AttachChecked(conn, seg, uint32(id), true).Check(); err != nil {
		_ = unix.SysvShmDetach(mem)
		removeSeg()
		return nil, fmt.Errorf("gl: soft: server cannot attach the shm segment: %v", err)
	}
	removeSeg()
	return &x11ShmSeg{seg: seg, mem: mem, w: w, h: h}, nil
}

// free detaches the segment on both sides. The server's detach is a request,
// so it runs after every put already sent. Nil-safe.
func (s *x11ShmSeg) free(conn *xgb.Conn) {
	if s == nil {
		return
	}
	shm.Detach(conn, s.seg)
	_ = unix.SysvShmDetach(s.mem)
	s.mem = nil
}

// put uploads sp.last: through the shm segment when the window has one or
// can get one, with core PutImage otherwise.
func (sp *softPresent) put() {
	if sp.shmOK {
		err := sp.putShm()
		if err == nil {
			return
		}
		// For good: a segment that could not be made for this size will
		// not be made for the next one either.
		log.Printf("%v; using core PutImage", err)
		sp.shmOK = false
	}
	sp.putCore()
}

// putShm swizzles sp.last into the segment and shows it with one
// ShmPutImage. A new frame size gets a new segment first.
func (sp *softPresent) putShm() error {
	if s := sp.shm; s == nil || s.w != sp.lastW || s.h != sp.lastH {
		sp.shm.free(sp.conn)
		sp.shm = nil
		seg, err := newShmSegFunc(sp.conn, sp.lastW, sp.lastH)
		if err != nil {
			return err
		}
		sp.shm = seg
	}
	w, h := uint16(sp.lastW), uint16(sp.lastH)
	swizzleBGRA(sp.shm.mem, sp.last[:sp.lastW*sp.lastH*4])
	// SendEvent 0: no completion event. The round trip in pace or sync
	// is what says the server is done with the segment.
	shm.PutImage(sp.conn, xproto.Drawable(sp.win), sp.gc, w, h, 0, 0, w, h, 0, 0,
		sp.depth, xproto.ImageFormatZPixmap, 0, sp.shm.seg, 0)
	return nil
}

// putCore uploads sp.last with core PutImage in row bands that each fit one
// request. Each request is built in sp.req, reused across bands and frames:
// the header is written in place and the band swizzled straight after it.
// xgb's NewRequest returns once the bytes are on the wire (or the
// connection is gone), so the buffer is free again for the next band.
func (sp *softPresent) putCore() {
	rows := bandRows(sp.lastW, sp.maxReq)
	rowBytes := sp.lastW * 4
	if need := putImageHeader + rows*rowBytes; cap(sp.req) < need {
		sp.req = make([]byte, need)
	}
	for y := 0; y < sp.lastH; y += rows {
		n := min(rows, sp.lastH-y)
		req := sp.req[:putImageHeader+n*rowBytes]
		putImageRequestHeader(req, uint32(sp.win), uint32(sp.gc),
			uint16(sp.lastW), uint16(n), int16(y), sp.depth)
		swizzleBGRA(req[putImageHeader:], sp.last[y*rowBytes:(y+n)*rowBytes])
		sp.conn.NewRequest(req, sp.conn.NewCookie(false, false))
	}
}

// putImageRequestHeader writes the 24-byte header of a core PutImage
// request (X11 protocol, request 72) for a ZPixmap of width x height at
// (0, dstY), into req, whose length is the whole request: the header plus
// the pixel data, which is already a multiple of 4 bytes at 4 bytes per
// pixel. Byte order is little-endian, as xgb sends it.
func putImageRequestHeader(req []byte, drawable, gc uint32, width, height uint16,
	dstY int16, depth byte) {

	_ = req[putImageHeader-1] // one bounds check for the header
	req[0] = 72               // PutImage
	req[1] = xproto.ImageFormatZPixmap
	binary.LittleEndian.PutUint16(req[2:], uint16(len(req)/4)) // length in 4-byte units
	binary.LittleEndian.PutUint32(req[4:], drawable)
	binary.LittleEndian.PutUint32(req[8:], gc)
	binary.LittleEndian.PutUint16(req[12:], width)
	binary.LittleEndian.PutUint16(req[14:], height)
	binary.LittleEndian.PutUint16(req[16:], 0)            // dst-x
	binary.LittleEndian.PutUint16(req[18:], uint16(dstY)) // dst-y
	req[20] = 0                                           // left-pad
	req[21] = depth
	req[22], req[23] = 0, 0 // unused
}

// sync waits until the server has processed every request sent so far.
func (sp *softPresent) sync() {
	_, _ = xproto.GetInputFocus(sp.conn).Reply()
}
