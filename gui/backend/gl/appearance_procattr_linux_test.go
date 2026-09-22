//go:build linux && !js && !android

package gl

import (
	"syscall"
	"testing"
)

// Regression: the monitor child had no parent-death signal, so an
// app that ended without WindowCleanup left gsettings running.
func TestMonitorSysProcAttrKillsOnParentDeath(t *testing.T) {
	attr := monitorSysProcAttr()
	if attr == nil || attr.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("monitor SysProcAttr: got %+v, want Pdeathsig SIGKILL", attr)
	}
}
