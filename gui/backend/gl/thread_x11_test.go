//go:build linux && !js && !android && !cgo

package gl

import (
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// These tests pin #827. New locks its goroutine to the OS thread. If the
// goroutine then exits while still locked, the Go runtime terminates that
// thread. On linux/arm64 under CGO_ENABLED=0, thread exit goes through purego's
// fakecgo threadentry_trampoline, whose frame-pointer epilogue corrupts R29,
// and the process segfaults in glibc. So New must hand the lock back on every
// error return, and Destroy must hand it back after a successful New.
//
// The observable effect is the same on every architecture: an unlocked
// goroutine's thread returns to the runtime's idle pool and stays alive, while
// a locked one is gone from /proc/self/task shortly after the goroutine ends.
// Go never reaps idle threads, so "still alive" is deterministic.

// threadSurvives runs fn on a fresh goroutine, lets that goroutine exit, and
// reports whether the OS thread it ran on still exists.
func threadSurvives(t *testing.T, fn func()) bool {
	t.Helper()
	tidCh := make(chan int, 1)
	go func() {
		fn()
		// Read the tid last, after fn has had its chance to lock or unlock:
		// this is the thread the goroutine exits from.
		tidCh <- syscall.Gettid()
	}()
	tid := <-tidCh
	// A locked thread is torn down right after the goroutine returns; give
	// the runtime ample time so a survivor is not a race we won.
	time.Sleep(200 * time.Millisecond)
	_, err := os.Stat("/proc/self/task/" + strconv.Itoa(tid))
	return err == nil
}

func TestNewErrorReleasesThread(t *testing.T) {
	// An unreachable display makes New fail at its first step, the X
	// connect, with no GL involved, so this runs on any Linux runner.
	t.Setenv("DISPLAY", ":65000")
	var newErr error
	alive := threadSurvives(t, func() {
		w := gui.NewWindow(gui.WindowCfg{State: new(int)})
		_, newErr = New(w)
	})
	if newErr == nil {
		t.Skip("New unexpectedly connected to display :65000")
	}
	if !alive {
		t.Fatal("New returned an error but left its goroutine locked: " +
			"the OS thread exited with the goroutine (#827)")
	}
}

func TestDestroyReleasesThread(t *testing.T) {
	var newErr error
	alive := threadSurvives(t, func() {
		w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 64, Height: 64})
		w.SetView(func(_ *gui.Window) gui.View { return gui.Column(gui.ContainerCfg{}) })
		b, err := New(w)
		if err != nil {
			newErr = err
			return
		}
		b.Destroy()
	})
	if newErr != nil {
		if os.Getenv("GOGUI_REQUIRE_GL") != "" {
			t.Fatalf("GOGUI_REQUIRE_GL is set but the backend failed: %v", newErr)
		}
		t.Skipf("backend init failed (no display?): %v", newErr)
	}
	if !alive {
		t.Fatal("Destroy left New's goroutine locked: " +
			"the OS thread exited with the goroutine (#827)")
	}
}
