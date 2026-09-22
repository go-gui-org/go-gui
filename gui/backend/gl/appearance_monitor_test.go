//go:build !js && !android

package gl

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// feedMonitor runs monitorAppearanceOutput over input with a dummy
// command value (never started; used only for identity). The exit
// path clears a matching monitor without sleeping when nobody is
// left subscribed.
func feedMonitor(t *testing.T, cmd *exec.Cmd, input string) {
	t.Helper()
	monitorAppearanceOutput(cmd, strings.NewReader(input))
}

func resetMonitorState(t *testing.T) {
	t.Helper()
	sysAppearanceMu.Lock()
	sysAppearanceCBs = make(map[*nativePlatform]func(gui.Appearance))
	sysAppearanceMon = nil
	sysAppearanceMu.Unlock()
}

func TestMonitorAppearanceOutputFansOut(t *testing.T) {
	resetMonitorState(t)
	oldDelay := monitorRestartDelay
	monitorRestartDelay = 0
	defer func() { monitorRestartDelay = oldDelay }()
	n := &nativePlatform{}
	var got []gui.Appearance
	setAppearanceCallback(n, func(a gui.Appearance) { got = append(got, a) })
	// Kill the real monitor the subscribe above may have spawned
	// (gsettings exists on Linux CI): the test feeds its own input.
	sysAppearanceMu.Lock()
	if sysAppearanceMon != nil {
		mon := sysAppearanceMon
		sysAppearanceMon = nil
		sysAppearanceMu.Unlock()
		if mon.Process != nil {
			_ = mon.Process.Kill()
		}
		_ = mon.Wait()
	} else {
		sysAppearanceMu.Unlock()
	}

	cmd := exec.Command("true")
	sysAppearanceMu.Lock()
	sysAppearanceMon = cmd
	sysAppearanceMu.Unlock()
	feedMonitor(t, cmd, "color-scheme: 'prefer-dark'\nnoise without quotes\ncolor-scheme: 'prefer-light'\n")

	if len(got) != 2 || got[0] != gui.AppearanceDark || got[1] != gui.AppearanceLight {
		t.Errorf("callbacks: got %v, want [dark light]", got)
	}
	// Abnormal exit with subscribers restarts the monitor; the
	// final unsubscribe must stop it again.
	setAppearanceCallback(n, nil)
	sysAppearanceMu.Lock()
	stillMon := sysAppearanceMon
	sysAppearanceMu.Unlock()
	if stillMon != nil {
		t.Error("unsubscribe must stop the restarted monitor")
	}
}

func TestMonitorDeliberateStopSkipsRestart(t *testing.T) {
	resetMonitorState(t)
	n := &nativePlatform{}
	setAppearanceCallback(n, func(gui.Appearance) {})
	setAppearanceCallback(n, nil) // deliberate stop before any output
	cmd := exec.Command("true")
	// A stale child exiting after an unsubscribe must not resurrect
	// the monitor (and must not sleep): mon no longer matches.
	feedMonitor(t, cmd, "")
	setAppearanceCallback(n, nil)
}
