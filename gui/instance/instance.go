// Package instance keeps one running copy of an app per user.
//
// The first process to call Acquire with an app ID becomes the
// primary. A later process with the same ID becomes a secondary: it
// sends its command-line arguments to the primary and gets
// primary=false back, so it can exit. The primary turns those
// arguments into gui.OpenEvent values for App.OnOpen, so "open a file
// while the app runs" works on every desktop platform.
//
//	lock, primary, err := instance.Acquire(info.ID)
//	if err != nil {
//		log.Print(err) // primary is still true when the app should run
//	}
//	if !primary {
//		return // the running copy has our arguments
//	}
//	defer lock.Close()
//	app := gui.NewApp()
//	app.OnOpen = onOpen
//	lock.Attach(app)
//	backend.RunApp(app, w)
//
// Election is atomic and the kernel releases it when the primary dies,
// also on kill -9, so a crash never leaves the app locked out:
//
//   - Unix: an advisory flock on a lock file in a per-user directory
//     ($XDG_RUNTIME_DIR, else a 0700 directory in os.TempDir). The
//     arguments travel over a Unix domain socket beside it.
//   - Windows: a named mutex in the session namespace. The arguments
//     travel over a named pipe that only the current user can open.
//   - Web and mobile: every process is the primary.
package instance

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// Limits on one forwarded message. A real launch sends a few paths;
// the caps stop a bad client from making the primary allocate without
// bound.
const (
	maxArgs      = 1024
	maxArgBytes  = 64 << 10
	maxMsgBytes  = 1 << 20
	maxPending   = 16
	ioTimeout    = 2 * time.Second
	dialTimeout  = 2 * time.Second
	dialInterval = 50 * time.Millisecond
	ackByte      = 0x06
)

// errNoPrimary means a secondary found the election taken but could
// not reach the primary before dialTimeout.
var errNoPrimary = errors.New("instance: another instance holds the lock but does not answer")

// Lock is the primary's hold on the election. Keep it until the app
// exits; Close releases it. The methods are safe on a nil *Lock, so
// the error path of Acquire needs no extra branch.
//
// exportaudit:keep — app-facing: apps hold the *Lock that Acquire returns
// and may store it in a field by type name
type Lock struct {
	plat platformLock

	mu sync.Mutex
	// deliverMu serializes every call of deliver, so the replay in
	// attachFunc and a concurrent receive cannot interleave messages.
	deliverMu  sync.Mutex
	deliver    func([]string) // set by Attach; nil until then
	pending    [][]string     // messages that arrived before Attach
	dropLogged bool           // the pending-overflow warning was logged
	closed     bool
	done       chan struct{} // closed when the accept loop exits
}

// Acquire runs the election for id, normally appinfo.Info.ID.
//
//   - Primary: (lock, true, nil). Keep lock until exit.
//   - Secondary: (nil, false, nil). os.Args[1:] reached the primary;
//     exit.
//   - The election failed (for example, no writable lock directory):
//     (nil, true, err). Run as usual, without single-instance mode.
//   - Another instance holds the election but did not accept the
//     arguments: (nil, false, err). The caller decides whether to exit.
//
// Call it before RunApp and before any window opens.
func Acquire(id string) (*Lock, bool, error) {
	return acquire(id, os.Args[1:])
}

func acquire(id string, args []string) (*Lock, bool, error) {
	if id == "" {
		return nil, true, errors.New("instance: empty app ID")
	}
	l := &Lock{done: make(chan struct{})}
	primary, err := l.plat.acquire(lockName(id), args)
	if err != nil {
		return nil, primary, err
	}
	if !primary {
		return nil, false, nil
	}
	go l.acceptLoop()
	return l, true, nil
}

// Attach routes the arguments of this launch and of every forwarded
// launch to app.DispatchOpen, through gui.ParseOpenArgs. Call it once,
// before RunApp; a later call does nothing. On a nil Lock it still
// dispatches this launch's own arguments.
func (l *Lock) Attach(app *gui.App) {
	dispatch := func(args []string) {
		app.DispatchOpen(gui.ParseOpenArgs(args))
	}
	if l == nil {
		dispatch(os.Args[1:])
		return
	}
	l.attachFunc(dispatch, os.Args[1:])
}

// attachFunc sets the delivery function. It hands fn the own
// arguments first (when not nil), then the messages that arrived
// before it, in arrival order: this launch started before any launch
// it received. Reports false when a function was already set. fn runs
// on the accept goroutine afterwards; it must not block for long.
func (l *Lock) attachFunc(fn func([]string), own []string) bool {
	// Held across the replay: a receive that sees deliver set waits
	// here, so a newer message cannot land between older ones.
	l.deliverMu.Lock()
	defer l.deliverMu.Unlock()
	l.mu.Lock()
	if l.deliver != nil || l.closed {
		l.mu.Unlock()
		return false
	}
	l.deliver = fn
	pending := l.pending
	l.pending = nil
	l.mu.Unlock()
	if own != nil {
		fn(own)
	}
	for _, args := range pending {
		fn(args)
	}
	return true
}

// Close stops listening for forwarded launches and releases the
// election. Safe to call more than once.
func (l *Lock) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()
	if err := l.plat.closeListener(l.done); err != nil {
		// The accept loop may still be blocked, so its handles cannot
		// be freed under it. The OS releases the election when the
		// process exits.
		return err
	}
	<-l.done
	return l.plat.release()
}

// acceptLoop serves forwarded launches one at a time until Close.
// Sequential on purpose: launches are rare, and one at a time bounds
// the memory a burst can take to one message.
func (l *Lock) acceptLoop() {
	defer close(l.done)
	for {
		conn, err := l.plat.accept()
		if err != nil {
			l.mu.Lock()
			closed := l.closed
			l.mu.Unlock()
			if closed {
				return
			}
			// A failed accept is not fatal (EMFILE, a client that
			// went away); back off so it cannot spin.
			time.Sleep(dialInterval)
			continue
		}
		args, err := readMsg(conn)
		if err == nil {
			_, _ = conn.Write([]byte{ackByte})
		}
		_ = conn.Close()
		if err == nil {
			l.receive(args)
		}
	}
}

// receive delivers one forwarded message, or keeps it until Attach.
func (l *Lock) receive(args []string) {
	l.mu.Lock()
	fn := l.deliver
	if fn == nil {
		full := len(l.pending) >= maxPending
		if !full {
			l.pending = append(l.pending, args)
		}
		// Logged once: the sender already got its ack and exited, so
		// the log is the only trace. Once bounds a flood of launches.
		logDrop := full && !l.dropLogged
		if logDrop {
			l.dropLogged = true
		}
		l.mu.Unlock()
		if logDrop {
			log.Printf("instance: %d launches arrived before Attach; "+
				"later ones are dropped", maxPending)
		}
		return
	}
	l.mu.Unlock()
	l.deliverMu.Lock()
	fn(args)
	l.deliverMu.Unlock()
}

// lockName derives the file, mutex and pipe name from the app ID. A
// hash keeps it short (Unix socket paths are limited to about 100
// bytes) and free of characters a path cannot hold.
func lockName(id string) string {
	sum := sha256.Sum256([]byte(id))
	return "gogui-" + hex.EncodeToString(sum[:8])
}

// writeMsg encodes args as a big-endian uint32 count followed by a
// uint32 length and the bytes of each argument.
func writeMsg(w io.Writer, args []string) error {
	if len(args) > maxArgs {
		return fmt.Errorf("%d arguments, limit %d", len(args), maxArgs)
	}
	size := 4
	for _, a := range args {
		if len(a) > maxArgBytes {
			return fmt.Errorf("argument of %d bytes, limit %d", len(a), maxArgBytes)
		}
		size += 4 + len(a)
	}
	if size > maxMsgBytes {
		return fmt.Errorf("message of %d bytes, limit %d", size, maxMsgBytes)
	}
	buf := make([]byte, 0, size)
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(args)))
	for _, a := range args {
		buf = binary.BigEndian.AppendUint32(buf, uint32(len(a)))
		buf = append(buf, a...)
	}
	_, err := w.Write(buf)
	return err
}

// readMsg decodes one writeMsg message. It checks every length before
// it allocates, so a hostile length cannot make it reserve memory.
func readMsg(r io.Reader) ([]string, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > maxArgs {
		return nil, fmt.Errorf("instance: %d arguments, limit %d", n, maxArgs)
	}
	args := make([]string, 0, n)
	total := 4
	for range n {
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return nil, err
		}
		size := binary.BigEndian.Uint32(hdr[:])
		// size is checked before the sum, so the sum cannot
		// overflow an int on 32-bit platforms.
		if size > maxArgBytes {
			return nil, errors.New("instance: argument over size limit")
		}
		total += 4 + int(size)
		if total > maxMsgBytes {
			return nil, errors.New("instance: message over size limit")
		}
		b := make([]byte, size)
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}
		args = append(args, string(b))
	}
	return args, nil
}
