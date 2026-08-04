//go:build linux

package ibus

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	ifaceBus     = "org.freedesktop.IBus"
	ifaceContext = "org.freedesktop.IBus.InputContext"

	nameDirect = "org.freedesktop.IBus"
	namePortal = "org.freedesktop.portal.IBus"
	pathBus    = "/org/freedesktop/IBus"

	// IBus capability bits. Only preedit and focus are claimed: go-gui
	// renders the preedit itself but has no candidate-list UI, so the
	// lookup-table, auxiliary-text and property bits are deliberately
	// left unset and the IME panel keeps drawing its own candidate
	// window. Surrounding text is not implemented either.
	capPreeditText = 1 << 0
	capFocus       = 1 << 3

	// IBusModifierType release bit, set on the state word so the engine
	// can tell key release from key press.
	releaseMask = 1 << 30

	// A key press must not stall the frame. The daemon is on a local
	// socket, so this is generous; exceeding it means something is
	// wrong and the client latches off rather than hanging the UI.
	callTimeout = 100 * time.Millisecond
)

// Kind classifies an event produced by the input method.
type Kind int

const (
	// KindPreedit carries an updated (possibly empty) preedit string.
	KindPreedit Kind = iota
	// KindCommit carries text the input method has committed.
	KindCommit
	// KindForwardKey carries a key the engine declined after all.
	KindForwardKey
)

// Event is one input-method result, queued for the main thread.
type Event struct {
	Text                   string
	Cursor, SelLen         int32
	Keyval, Keycode, State uint32
	Kind                   Kind
}

// Client is an IBus input context. A nil *Client is inert: every method
// is safe to call on it, so callers need only the one nil check that
// distinguishes "no input method on this system" from "have one".
type Client struct {
	conn *dbus.Conn
	ctx  dbus.BusObject
	wake func()

	signals chan *dbus.Signal

	mu      sync.Mutex
	queue   []Event
	preedit string // last preedit, replayed by ShowPreeditText

	// disabled latches on the first transport failure. Recovering a
	// half-dead input method is not worth the complexity: dropping back
	// to the raw keysym path keeps plain typing working, which is the
	// behaviour that must never regress.
	disabled atomic.Bool
}

// New connects to an input method and creates an input context for it.
// It returns nil when no input method is reachable, which is the normal
// case on a system without one; the caller then keeps decoding keysyms
// directly.
//
// wake is called from a D-Bus goroutine whenever events are queued. It
// must be safe off the main thread and should make the event loop come
// round to Drain.
func New(clientName string, wake func()) *Client {
	conn, dest := connect()
	if conn == nil {
		return nil
	}

	var ctxPath dbus.ObjectPath
	err := conn.Object(dest, pathBus).
		Call(ifaceBus+".CreateInputContext", 0, clientName).
		Store(&ctxPath)
	if err != nil {
		_ = conn.Close()
		return nil
	}

	c := &Client{
		conn:    conn,
		ctx:     conn.Object(dest, ctxPath),
		wake:    wake,
		signals: make(chan *dbus.Signal, 32),
	}

	// A failed match rule is not fatal on a direct peer connection,
	// where signals arrive regardless.
	_ = conn.AddMatchSignal(
		dbus.WithMatchObjectPath(ctxPath),
		dbus.WithMatchInterface(ifaceContext),
	)
	conn.Signal(c.signals)
	go c.readSignals()

	c.call("SetCapabilities", uint32(capPreeditText|capFocus))
	return c
}

// connect finds an input method and returns a private connection to it
// along with the bus name to address. The connection is always private
// so Close cannot disturb the shared session bus used by atspi and sni.
func connect() (*dbus.Conn, string) {
	if addr := os.Getenv("IBUS_ADDRESS"); addr != "" {
		if conn := dialPrivate(addr); conn != nil {
			return conn, nameDirect
		}
	}
	if addr := socketAddress(); addr != "" {
		if conn := dialPrivate(addr); conn != nil {
			return conn, nameDirect
		}
	}
	// fcitx5 owns org.freedesktop.IBus on the session bus; the portal
	// name is what a sandboxed session sees.
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return nil, ""
	}
	if err = conn.Auth(nil); err != nil {
		_ = conn.Close()
		return nil, ""
	}
	if err = conn.Hello(); err != nil {
		_ = conn.Close()
		return nil, ""
	}
	for _, name := range []string{nameDirect, namePortal} {
		var has bool
		if e := conn.BusObject().Call(
			"org.freedesktop.DBus.NameHasOwner", 0, name,
		).Store(&has); e == nil && has {
			return conn, name
		}
	}
	_ = conn.Close()
	return nil, ""
}

func dialPrivate(addr string) *dbus.Conn {
	conn, err := dbus.Dial(addr)
	if err != nil {
		return nil
	}
	if err = conn.Auth(nil); err != nil {
		_ = conn.Close()
		return nil
	}
	if err = conn.Hello(); err != nil {
		_ = conn.Close()
		return nil
	}
	return conn
}

// socketAddress reads the address ibus-daemon publishes under the user
// config directory. The file outlives the daemon, so the recorded pid
// is checked before its address is trusted.
func socketAddress() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	dir = filepath.Join(dir, "ibus", "bus")

	id := machineID()
	if id == "" {
		return ""
	}
	// The exact name encodes the display, which is reconstructed the
	// same way ibus does it. Globbing covers the cases where that
	// reconstruction disagrees (a renamed host, IBUS_DISPLAY set for
	// the daemon but not for us).
	names := []string{id + "-" + displaySuffix()}
	if matches, err := filepath.Glob(filepath.Join(dir, id+"-*")); err == nil {
		for _, m := range matches {
			names = append(names, filepath.Base(m))
		}
	}
	for _, name := range names {
		if addr := readSocketFile(filepath.Join(dir, name)); addr != "" {
			return addr
		}
	}
	return ""
}

func readSocketFile(path string) string {
	// #nosec G304 — path is the caller's own ibus config dir joined with
	// a name that came from globbing that same dir; no external input.
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var addr string
	alive := false
	for line := range strings.SplitSeq(string(data), "\n") {
		switch key, val, _ := strings.Cut(strings.TrimSpace(line), "="); key {
		case "IBUS_ADDRESS":
			addr = val
		case "IBUS_DAEMON_PID":
			pid, err2 := strconv.Atoi(val)
			alive = err2 == nil && pid > 0 && syscall.Kill(pid, 0) == nil
		}
	}
	if !alive {
		return ""
	}
	return addr
}

func machineID() string {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		// #nosec G304 — p iterates two compile-time constants.
		if data, err := os.ReadFile(p); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id
			}
		}
	}
	return ""
}

// displaySuffix rebuilds the "<host>-<number>" part of the socket file
// name from DISPLAY, matching ibus_get_socket_path.
func displaySuffix() string {
	disp := os.Getenv("IBUS_DISPLAY")
	if disp == "" {
		disp = os.Getenv("DISPLAY")
	}
	host, rest, found := strings.Cut(disp, ":")
	if !found {
		return "unix-0"
	}
	if host == "" {
		host = "unix"
	}
	num, _, _ := strings.Cut(rest, ".")
	if num == "" {
		num = "0"
	}
	return host + "-" + num
}

// --- input context ---

// ProcessKey offers a key to the input method and reports whether it was
// consumed. A consumed key must not also be handled by the caller: the
// engine owns navigation and editing keys while a composition is live.
func (c *Client) ProcessKey(keyval, keycode, state uint32) bool {
	if c == nil || c.disabled.Load() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()

	var consumed bool
	err := c.ctx.CallWithContext(
		ctx, ifaceContext+".ProcessKeyEvent", 0, keyval, keycode, state,
	).Store(&consumed)
	if err != nil {
		c.disabled.Store(true)
		return false
	}
	return consumed
}

// ProcessKeyRelease offers a key release to the input method. The result
// is ignored: releases never suppress the caller's own handling.
func (c *Client) ProcessKeyRelease(keyval, keycode, state uint32) {
	if c == nil {
		return
	}
	c.ProcessKey(keyval, keycode, state|releaseMask)
}

// FocusIn tells the input method this context is now active.
func (c *Client) FocusIn() { c.call("FocusIn") }

// FocusOut abandons any live composition and tells the input method
// this context is no longer active. The reset comes first so a
// half-composed string cannot survive the focus change.
func (c *Client) FocusOut() {
	c.call("Reset")
	c.call("FocusOut")
}

// SetCursorLocation reports the caret rect in root window pixels so the
// candidate window can anchor to it.
func (c *Client) SetCursorLocation(x, y, w, h int32) {
	c.call("SetCursorLocation", x, y, w, h)
}

// Close destroys the input context and drops the connection.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.disabled.Store(true)
	c.call("Destroy")
	_ = c.conn.Close()
}

// call invokes a fire-and-forget input-context method, latching the
// client off if the transport fails.
func (c *Client) call(method string, args ...any) {
	if c == nil || c.disabled.Load() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	if err := c.ctx.CallWithContext(
		ctx, ifaceContext+"."+method, 0, args...,
	).Err; err != nil {
		c.disabled.Store(true)
	}
}

// --- signals ---

// readSignals translates input-method signals into queued events. It
// exits when the connection closes, which godbus signals by closing the
// channel; that also latches the client off.
func (c *Client) readSignals() {
	for sig := range c.signals {
		switch sig.Name {
		case ifaceContext + ".UpdatePreeditText",
			ifaceContext + ".UpdatePreeditTextWithMode":
			text, cursor, visible, ok := decodePreedit(sig.Body)
			if !ok {
				continue
			}
			if !visible {
				text = ""
			}
			c.setPreedit(text, int32(cursor)) //nolint:gosec // clamped in gui.imeUpdate

		case ifaceContext + ".ShowPreeditText":
			c.mu.Lock()
			text := c.preedit
			c.mu.Unlock()
			c.push(Event{Kind: KindPreedit, Text: text})

		case ifaceContext + ".HidePreeditText":
			c.setPreedit("", 0)

		case ifaceContext + ".CommitText":
			if len(sig.Body) == 0 {
				continue
			}
			text, ok := decodeText(sig.Body[0])
			if !ok || text == "" {
				continue
			}
			// The preedit is consumed by the commit; clear it first so
			// the caller never renders both.
			c.setPreedit("", 0)
			c.push(Event{Kind: KindCommit, Text: text})

		case ifaceContext + ".ForwardKeyEvent":
			keyval, keycode, state, ok := decodeForwardKey(sig.Body)
			if !ok || state&releaseMask != 0 {
				continue
			}
			c.push(Event{
				Kind:   KindForwardKey,
				Keyval: keyval, Keycode: keycode, State: state,
			})
		}
	}
	c.disabled.Store(true)
}

// setPreedit caches the preedit for ShowPreeditText to replay and queues
// it for the caller.
func (c *Client) setPreedit(text string, cursor int32) {
	c.mu.Lock()
	c.preedit = text
	c.mu.Unlock()
	c.push(Event{Kind: KindPreedit, Text: text, Cursor: cursor})
}

// push queues an event and wakes the event loop.
func (c *Client) push(e Event) {
	c.mu.Lock()
	c.queue = append(c.queue, e)
	c.mu.Unlock()
	if c.wake != nil {
		c.wake()
	}
}

// Drain appends the queued events to dst and returns it. The queue's
// backing array is reused, so callers should pass a scratch slice back
// each time to keep this allocation-free. Main thread only.
func (c *Client) Drain(dst []Event) []Event {
	if c == nil {
		return dst
	}
	c.mu.Lock()
	dst = append(dst, c.queue...)
	c.queue = c.queue[:0]
	c.mu.Unlock()
	return dst
}
