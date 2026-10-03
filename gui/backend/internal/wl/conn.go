//go:build linux && !android && (amd64 || arm64)

package wl

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// Conn is a connection to a Wayland compositor.
type Conn struct {
	// Display is the wl_display object: Sync and GetRegistry are on it.
	Display Display
	fd      int
	// pollFds is reused by Dispatch so polling does not allocate. Entry 0
	// is the compositor socket, entry 1 the wake eventfd.
	pollFds [2]unix.PollFd
	// drain receives the eventfd counter when Dispatch empties it.
	drain [8]byte

	// wakeMu guards wakeFd. Wake runs on any goroutine, and must not
	// write to the descriptor number after Close has freed it: the
	// kernel can hand that number to an unrelated file.
	wakeMu sync.Mutex
	wakeFd int
}

// wakeOne is the 8-byte counter increment written to the eventfd. A
// package value, so Wake passes a slice of it without allocating.
var wakeOne = [8]byte{1}

// Connect opens the compositor socket. An empty name uses $WAYLAND_DISPLAY,
// then "wayland-0", as libwayland does.
func Connect(name string) (*Conn, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	nameC := cString(name, true)
	r, _, _ := purego.SyscallN(fnDisplayConnect, cStringPtr(nameC))
	runtime.KeepAlive(nameC)
	if r == 0 {
		if name == "" {
			name = "$WAYLAND_DISPLAY"
		}
		return nil, fmt.Errorf("wl_display_connect(%s): no Wayland compositor", name)
	}
	wakeFd, err := unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		_, _, _ = purego.SyscallN(fnDisplayDisconnect, r)
		return nil, fmt.Errorf("eventfd: %w", err)
	}
	fd, _, _ := purego.SyscallN(fnDisplayGetFd, r)
	c := &Conn{Display: Display{Proxy{ptr: r}}, fd: int(int32(fd)), wakeFd: wakeFd}
	c.pollFds[0] = unix.PollFd{Fd: int32(c.fd), Events: unix.POLLIN}
	c.pollFds[1] = unix.PollFd{Fd: int32(wakeFd), Events: unix.POLLIN}
	return c, nil
}

// Wake makes a Dispatch that is waiting return at once, or the next one
// return without waiting. Unlike every other method it is safe to call
// from any goroutine, and after Close, where it does nothing.
func (c *Conn) Wake() {
	c.wakeMu.Lock()
	defer c.wakeMu.Unlock()
	if c.wakeFd < 0 {
		return
	}
	// EAGAIN means the counter is at its maximum: a wake is pending
	// anyway, so the error is ignored.
	_, _ = unix.Write(c.wakeFd, wakeOne[:])
}

// Close disconnects. Proxies of this connection must not be used afterwards.
func (c *Conn) Close() {
	if c.Display.ptr == 0 {
		return
	}
	_, _, _ = purego.SyscallN(fnDisplayDisconnect, c.Display.ptr)
	c.Display.ptr = 0
	c.wakeMu.Lock()
	_ = unix.Close(c.wakeFd)
	c.wakeFd = -1
	c.wakeMu.Unlock()
	// One Conn at a time (doc.go): every handler belonged to this one.
	clear(handlers)
}

// Fd returns the connection's file descriptor, for an outer poll loop.
func (c *Conn) Fd() int { return c.fd }

// Roundtrip sends all queued requests and blocks until the compositor has
// handled them, dispatching every event that arrives meanwhile.
func (c *Conn) Roundtrip() error {
	r, _, _ := purego.SyscallN(fnDisplayRoundtrip, c.Display.ptr)
	rethrow()
	if int32(r) < 0 {
		return c.Err()
	}
	return nil
}

// DispatchPending runs the handlers for events already read from the
// socket, without reading more.
func (c *Conn) DispatchPending() error {
	r, _, _ := purego.SyscallN(fnDisplayDispatchPending, c.Display.ptr)
	rethrow()
	if int32(r) < 0 {
		return c.Err()
	}
	return nil
}

// Flush sends queued requests. A full socket buffer is not an error: the
// rest goes out on a later Flush.
func (c *Conn) Flush() error {
	r, _, _ := purego.SyscallN(fnDisplayFlush, c.Display.ptr)
	if int32(r) < 0 {
		// -1 with no connection error is EAGAIN (socket buffer full).
		return c.Err()
	}
	return nil
}

// Dispatch flushes requests, waits up to timeout for events, reads them and
// runs their handlers. A negative timeout waits until an event or a Wake
// arrives; zero only takes what is already there. It is the wl_display_prepare_read
// sequence libwayland documents for a client with its own wait, so another
// library reading the same connection (EGL) never loses events.
func (c *Conn) Dispatch(timeout time.Duration) error {
	for {
		r, _, _ := purego.SyscallN(fnDisplayPrepareRead, c.Display.ptr)
		if int32(r) == 0 {
			break
		}
		// Events are already queued: they must be dispatched before a read.
		if err := c.DispatchPending(); err != nil {
			return err
		}
	}
	if err := c.Flush(); err != nil {
		_, _, _ = purego.SyscallN(fnDisplayCancelRead, c.Display.ptr)
		return err
	}

	ms := -1
	if timeout >= 0 {
		// Round up so a sub-millisecond timeout still waits.
		ms = int((timeout + time.Millisecond - 1) / time.Millisecond)
	}
	n, err := unix.Poll(c.pollFds[:], ms)
	if err != nil && !errors.Is(err, unix.EINTR) {
		_, _, _ = purego.SyscallN(fnDisplayCancelRead, c.Display.ptr)
		return fmt.Errorf("poll wayland fd: %w", err)
	}
	if n > 0 && c.pollFds[1].Revents != 0 {
		// Reset the counter so the next wait blocks again. Nonblocking:
		// a second Dispatch woken by the same write finds it empty.
		_, _ = unix.Read(int(c.pollFds[1].Fd), c.drain[:])
	}
	if n > 0 && c.pollFds[0].Revents != 0 {
		r, _, _ := purego.SyscallN(fnDisplayReadEvents, c.Display.ptr)
		if int32(r) < 0 {
			return c.Err()
		}
	} else {
		// Woken, or timed out: nothing to read from the compositor.
		_, _, _ = purego.SyscallN(fnDisplayCancelRead, c.Display.ptr)
	}
	return c.DispatchPending()
}

// ProtocolError is a fatal error the compositor sent: the client broke the
// protocol. The connection is unusable afterwards.
type ProtocolError struct {
	Interface string // interface of the object that raised it
	ID        uint32 // object id
	Code      uint32 // error code, from that interface's error enum
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("wayland protocol error %d on %s@%d", e.Code, e.Interface, e.ID)
}

// Err returns the connection's fatal error, or nil if it is healthy.
func (c *Conn) Err() error {
	code, _, _ := purego.SyscallN(fnDisplayGetError, c.Display.ptr)
	if code == 0 {
		return nil
	}
	if syscall.Errno(code) == syscall.EPROTO {
		var iface uintptr
		var id uint32
		pcode, _, _ := purego.SyscallN(fnDisplayGetProtocolError, c.Display.ptr,
			uintptr(unsafe.Pointer(&iface)), uintptr(unsafe.Pointer(&id)))
		pe := &ProtocolError{ID: id, Code: uint32(pcode)}
		if iface != 0 {
			// struct wl_interface starts with its name.
			pe.Interface = goString(*(*uintptr)(cptr(iface)))
		}
		return pe
	}
	return fmt.Errorf("wayland connection: %w", syscall.Errno(code))
}
