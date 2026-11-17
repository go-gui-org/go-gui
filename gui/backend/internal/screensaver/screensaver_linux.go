//go:build linux && !android

// Package screensaver keeps the display on through the freedesktop
// ScreenSaver D-Bus service (issue #1172). GNOME offers it through
// gsd-screensaver-proxy, KDE and Xfce offer it themselves. sway and
// Hyprland do not. On Wayland the GL backend uses the idle-inhibit
// protocol where the compositor offers it (issue #1287); this package is
// the X11 path and the fallback.
//
// The request lives on a private session-bus connection that stays open
// while the request is held. The service ends a request whose caller
// left the bus, so a crash ends it too.
package screensaver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// ScreenSaver D-Bus names.
const (
	dest  = "org.freedesktop.ScreenSaver"
	path  = dbus.ObjectPath("/org/freedesktop/ScreenSaver")
	iface = "org.freedesktop.ScreenSaver"
)

// callTimeout bounds connecting and each call. They run on the UI
// thread, so a bus or service that does not answer must not hang the
// app. A variable so a test can shorten it.
var callTimeout = 2 * time.Second

// held is the request of the process, nil while none is held.
var held struct {
	mu   sync.Mutex
	conn *dbus.Conn
	// stop cancels the context the connection is bound to.
	stop context.CancelFunc
	// cookie is what Inhibit returned, for UnInhibit.
	cookie uint32
}

// Set begins the request when on is true and ends it when on is false.
// A call that asks for the state already held does nothing. app and
// reason name the request to the desktop; they must not be empty, or
// gnome-session refuses it.
func Set(on bool, app, reason string) error {
	held.mu.Lock()
	defer held.mu.Unlock()
	if !on {
		if held.conn == nil {
			return nil
		}
		end(held.conn, held.cookie)
		held.stop()
		held.conn, held.stop = nil, nil
		return nil
	}
	if held.conn != nil {
		return nil
	}
	conn, stop, cookie, err := begin(app, reason)
	if err != nil {
		return err
	}
	held.conn, held.stop, held.cookie = conn, stop, cookie
	return nil
}

// begin opens a private connection and asks the service to inhibit
// idle. Nothing stays open when it fails.
func begin(app, reason string) (*dbus.Conn, context.CancelFunc, uint32, error) {
	conn, stop, err := connect()
	if err != nil {
		return nil, nil, 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	var cookie uint32
	err = conn.Object(dest, path).
		CallWithContext(ctx, iface+".Inhibit", 0, app, reason).Store(&cookie)
	if err != nil {
		stop()
		return nil, nil, 0, fmt.Errorf("%s.Inhibit: %w", iface, err)
	}
	return conn, stop, cookie, nil
}

// connect opens a private session-bus connection within callTimeout.
// Auth and Hello take no context, so they run in a goroutine: a bus
// that accepts the connection and then says nothing must not hang the
// UI thread. The connection is bound to a context that the timeout
// cancels; godbus then closes it, which ends the blocked Auth, so
// nothing is left open. stop cancels that context: call it once the
// connection is no longer needed.
func connect() (conn *dbus.Conn, stop context.CancelFunc, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		conn *dbus.Conn
		err  error
	}
	done := make(chan result, 1)
	go func() {
		c, dialErr := dial(ctx)
		done <- result{c, dialErr}
	}()
	timer := time.NewTimer(callTimeout)
	defer timer.Stop()
	select {
	case r := <-done:
		if r.err != nil {
			cancel()
			return nil, nil, r.err
		}
		return r.conn, cancel, nil
	case <-timer.C:
		cancel()
		return nil, nil, fmt.Errorf(
			"the D-Bus session bus did not answer within %v", callTimeout)
	}
}

// dial connects to the session bus, authenticates and says Hello. The
// connection closes when ctx is cancelled.
func dial(ctx context.Context) (*dbus.Conn, error) {
	conn, err := dbus.SessionBusPrivate(dbus.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("no D-Bus session bus: %w", err)
	}
	if err = conn.Auth(nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("D-Bus auth: %w", err)
	}
	if err = conn.Hello(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("D-Bus hello: %w", err)
	}
	return conn, nil
}

// end asks the service to end the request, then closes the connection.
// Closing alone would end it too, since the service watches its caller
// leave the bus, so a failed UnInhibit is not reported.
func end(conn *dbus.Conn, cookie uint32) {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	_ = conn.Object(dest, path).
		CallWithContext(ctx, iface+".UnInhibit", 0, cookie).Err
	_ = conn.Close()
}
