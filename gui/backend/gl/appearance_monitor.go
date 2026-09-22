//go:build !js && !android

package gl

import (
	"bufio"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// Shared OS-appearance monitor machinery (issue #752). Used by the
// Linux backend; dormant elsewhere. Kept platform-neutral so its
// test runs on every host.
//
// The one consumer is the gsettings monitor, so its command and
// line parser live here directly; a second consumer would
// parameterize them.

const (
	gsettingsSchema = "org.gnome.desktop.interface"
	gsettingsKey    = "color-scheme"
)

// monitorCommand spawns the change monitor whose stdout lines feed
// parseGsettingsColorScheme (which accepts both `get` output and
// `monitor` lines).
var monitorCommand = []string{"gsettings", "monitor", gsettingsSchema, gsettingsKey}

var (
	sysAppearanceMu  sync.Mutex
	sysAppearanceCBs = make(map[*nativePlatform]func(gui.Appearance))
	// sysAppearanceMon is the running monitor child, nil when no
	// window is subscribed. One process for all windows.
	sysAppearanceMon *exec.Cmd
)

// monitorRestartDelay caps a crashing monitor's respawn rate: the
// child dying on its own reads as EOF, indistinguishable from a
// kill except by the sysAppearanceMon check below, so without a
// pause a broken monitor would hot-loop spawns. A var so tests can
// shrink it; production stays at 2 seconds.
var monitorRestartDelay = 2 * time.Second

// startAppearanceMonitorLocked spawns the monitor child and scans
// its stdout. Caller holds sysAppearanceMu. A spawn failure leaves
// the monitor nil: queries already report no setting, so callbacks
// simply never fire.
func startAppearanceMonitorLocked() {
	cmd := exec.Command(monitorCommand[0], monitorCommand[1:]...)
	cmd.SysProcAttr = monitorSysProcAttr()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	sysAppearanceMon = cmd
	go monitorAppearanceOutput(cmd, stdout)
}

// stopAppearanceMonitorLocked kills the monitor and reaps it.
// Caller holds sysAppearanceMu.
func stopAppearanceMonitorLocked() {
	mon := sysAppearanceMon
	sysAppearanceMon = nil
	if mon.Process != nil {
		_ = mon.Process.Kill()
	}
	_ = mon.Wait()
}

// setAppearanceCallback registers cb, or unregisters on nil, and
// starts or stops the shared monitor to match. The decision is one
// critical section: the spawn/kill runs under the mutex so a
// concurrent unsubscribe cannot kill a monitor a new subscriber
// needs. Neither operation re-enters this package (SIGKILL delivers
// nothing; the scanner only retakes the lock after stop released
// it), so this cannot deadlock.
func setAppearanceCallback(n *nativePlatform, cb func(gui.Appearance)) {
	sysAppearanceMu.Lock()
	defer sysAppearanceMu.Unlock()
	if cb == nil {
		delete(sysAppearanceCBs, n)
	} else {
		sysAppearanceCBs[n] = cb
	}
	want, running := len(sysAppearanceCBs) > 0, sysAppearanceMon != nil
	if want && !running {
		startAppearanceMonitorLocked()
	} else if !want && running {
		stopAppearanceMonitorLocked()
	}
}

// snapshotAppearanceCallbacks returns the current subscribers.
// Caller must not hold sysAppearanceMu (invoking a callback must
// never run under it: callbacks marshal to the frame thread).
func snapshotAppearanceCallbacks() []func(gui.Appearance) {
	sysAppearanceMu.Lock()
	defer sysAppearanceMu.Unlock()
	cbs := make([]func(gui.Appearance), 0, len(sysAppearanceCBs))
	for _, cb := range sysAppearanceCBs {
		cbs = append(cbs, cb)
	}
	return cbs
}

// monitorAppearanceOutput scans the monitor child's stdout and fans
// out readings. Lines are the monitor's own short status lines, so
// bufio.Scanner's 64 KB line cap cannot trip on real output; an
// oversize line ends the scan instead of growing a buffer without
// bound.
func monitorAppearanceOutput(cmd *exec.Cmd, stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		a, ok := parseGsettingsColorScheme(sc.Text())
		if !ok {
			continue
		}
		for _, cb := range snapshotAppearanceCallbacks() {
			cb(a)
		}
	}
	// Clearing sysAppearanceMon marks a deliberate stop (which
	// nils it first): only a monitor that died on its own still
	// matches cmd here. With subscribers left, restart after a
	// pause rather than leaving them unwatched.
	sysAppearanceMu.Lock()
	abnormal := sysAppearanceMon == cmd
	if abnormal {
		sysAppearanceMon = nil
	}
	subscribed := len(sysAppearanceCBs) > 0
	sysAppearanceMu.Unlock()
	if !abnormal || !subscribed {
		return
	}
	time.Sleep(monitorRestartDelay)
	sysAppearanceMu.Lock()
	if len(sysAppearanceCBs) > 0 && sysAppearanceMon == nil {
		startAppearanceMonitorLocked()
	}
	sysAppearanceMu.Unlock()
}
