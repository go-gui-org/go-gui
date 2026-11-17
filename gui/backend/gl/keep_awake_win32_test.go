//go:build windows && !js

package gl

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

// EXECUTION_STATE bits that a display and system power request set.
const (
	esSystemRequired  = 0x1
	esDisplayRequired = 0x2
	esKeepAwake       = esSystemRequired | esDisplayRequired
)

var pCallNtPowerInformation = windows.NewLazySystemDLL("powrprof.dll").
	NewProc("CallNtPowerInformation")

// systemExecutionState reads the power state of the whole system.
// Unlike `powercfg /requests`, it needs no administrator rights.
func systemExecutionState(t *testing.T) uint32 {
	t.Helper()
	const levelSystemExecutionState = 16 // POWER_INFORMATION_LEVEL
	var state uint32
	status, _, _ := pCallNtPowerInformation.Call(levelSystemExecutionState,
		0, 0, uintptr(unsafe.Pointer(&state)), unsafe.Sizeof(state))
	if status != 0 {
		t.Fatalf("CallNtPowerInformation: NTSTATUS %#x", status)
	}
	return state
}

// endTestPowerRequest ends a request a failed test left behind.
func endTestPowerRequest(t *testing.T) {
	t.Cleanup(func() { _ = (&nativePlatform{}).SetKeepAwake(false, "", "") })
}

// Issue #1172: the request reaches Windows. While it is held, the
// system's execution state asks for the display and the system. After
// the release it does not, unless another program holds a request of
// its own.
func TestKeepAwakeSetsExecutionState(t *testing.T) {
	endTestPowerRequest(t)
	before := systemExecutionState(t)
	n := &nativePlatform{}
	if err := n.SetKeepAwake(true, "go-gui", "go-gui keep-awake test"); err != nil {
		t.Fatalf("SetKeepAwake(true): %v", err)
	}
	if got := systemExecutionState(t); got&esKeepAwake != esKeepAwake {
		t.Errorf("held: execution state %#x, want display and system bits", got)
	}
	if err := n.SetKeepAwake(false, "", ""); err != nil {
		t.Fatalf("SetKeepAwake(false): %v", err)
	}
	if before&esKeepAwake != 0 {
		t.Logf("another program keeps the system awake (%#x); "+
			"the release is not checked", before)
		return
	}
	if got := systemExecutionState(t); got&esKeepAwake != 0 {
		t.Errorf("released: execution state %#x, want no display or system bit", got)
	}
}

// gui calls only on the edges, but a repeated call must not stack a
// second request or fail.
func TestKeepAwakeRepeatedEdges(t *testing.T) {
	endTestPowerRequest(t)
	n := &nativePlatform{}
	if err := n.SetKeepAwake(true, "go-gui", "first"); err != nil {
		t.Fatalf("SetKeepAwake(true): %v", err)
	}
	h := powerRequest.h
	if err := n.SetKeepAwake(true, "go-gui", "second"); err != nil {
		t.Fatalf("second SetKeepAwake(true): %v", err)
	}
	if powerRequest.h != h {
		t.Error("a second on made a second request")
	}
	for range 2 {
		if err := n.SetKeepAwake(false, "", ""); err != nil {
			t.Fatalf("SetKeepAwake(false): %v", err)
		}
	}
	if powerRequest.h != 0 {
		t.Error("the request is still held after off")
	}
}

// The whole path on a real window: a request held through gui, ended by
// a minimize, taken again on the restore, ended by the release.
func TestKeepAwakeFollowsMinimize(t *testing.T) {
	endTestPowerRequest(t)
	b := newPlacedWindow(t)
	w := b.plat.w
	w.SetView(func(_ *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{})
	})
	w.SetNativePlatform(&nativePlatform{b: b})

	release := w.KeepAwake("go-gui keep-awake test")
	w.FrameFn() // runs the queued request
	if powerRequest.h == 0 {
		t.Fatal("KeepAwake made no power request")
	}
	if _, handled := b.handleMessage(wmSize, sizeMinimized, 0); !handled {
		t.Fatal("WM_SIZE/SIZE_MINIMIZED fell through to DefWindowProc")
	}
	if powerRequest.h != 0 {
		t.Error("the request is still held while minimized")
	}
	gui.DispatchWindowOccluded(w, false)
	if powerRequest.h == 0 {
		t.Error("the restore did not take the request again")
	}
	release()
	w.FrameFn()
	if powerRequest.h != 0 {
		t.Error("the request is still held after release")
	}
}
