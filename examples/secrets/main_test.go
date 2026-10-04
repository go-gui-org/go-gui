package main

import (
	"errors"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// NewTestWindow gives the window an in-memory secret store, so these
// tests never touch the real credential store.

// syncSpawn runs store calls on the test goroutine. run still posts the
// result with QueueCommand; the next Test* action settles the window,
// which runs the queued command, as a frame would.
func syncSpawn(t *testing.T) {
	t.Helper()
	prev := spawn
	spawn = func(f func()) { f() }
	t.Cleanup(func() { spawn = prev })
}

func TestFirstRunReportsNoToken(t *testing.T) {
	syncSpawn(t)
	w := gui.NewTestWindow(t, windowCfg())
	w.TestRender(nil)
	if err := w.TestType("token", "a"); err != nil {
		t.Fatal(err)
	}
	app := gui.State[App](w)
	if app.Busy || app.Status != "No token is saved." {
		t.Fatalf("Busy %v, Status %q", app.Busy, app.Status)
	}
}

func TestSaveThenDelete(t *testing.T) {
	syncSpawn(t)
	w := gui.NewTestWindow(t, windowCfg())
	w.TestRender(nil)
	if err := w.TestType("token", "abc123"); err != nil {
		t.Fatal(err)
	}
	if err := w.TestClick("save"); err != nil {
		t.Fatal(err)
	}
	app := gui.State[App](w)
	if app.Busy || app.Draft != "" || app.Status != "A token is saved (6 bytes)." {
		t.Fatalf("after save: Busy %v, Draft %q, Status %q", app.Busy, app.Draft, app.Status)
	}
	got, err := gui.LoadSecret(w, tokenKey)
	if err != nil || string(got) != "abc123" {
		t.Fatalf("LoadSecret = %q, %v", got, err)
	}
	if err = w.TestClick("delete"); err != nil {
		t.Fatal(err)
	}
	if status := gui.State[App](w).Status; status != "No token is saved." {
		t.Fatalf("after delete: Status = %q", status)
	}
	if _, err = gui.LoadSecret(w, tokenKey); !errors.Is(err, gui.ErrSecretNotFound) {
		t.Fatalf("after delete: got %v, want ErrSecretNotFound", err)
	}
}

// settle runs a Test* action, which settles the window and so runs the
// commands that store calls have queued. TestRender does not run them.
func settle(t *testing.T, w *gui.Window) {
	t.Helper()
	if err := w.TestType("token", "x"); err != nil {
		t.Fatal(err)
	}
}

// TestStoreCallLeavesMainThread checks the real spawn: run returns
// while the store call is still blocked (as during an OS unlock
// prompt), the window stays busy until the call ends, and the result
// arrives through QueueCommand.
func TestStoreCallLeavesMainThread(t *testing.T) {
	realSpawn := spawn
	syncSpawn(t) // OnInit's status load finishes before the test starts
	w := gui.NewTestWindow(t, windowCfg())
	w.TestRender(nil)
	settle(t, w)
	spawn = realSpawn

	release := make(chan struct{})
	returned := make(chan struct{})
	go func() {
		run(w, func() string {
			<-release
			return "released"
		})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("run blocked until the store call finished")
	}
	settle(t, w)
	if app := gui.State[App](w); !app.Busy || app.Status == "released" {
		t.Fatalf("while blocked: Busy %v, Status %q", app.Busy, app.Status)
	}

	// Settle until the result lands. Once Status changes, the goroutine
	// has queued its command, so nothing touches the window after the
	// test returns.
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for gui.State[App](w).Status != "released" {
		if time.Now().After(deadline) {
			t.Fatal("the store call result never arrived")
		}
		time.Sleep(time.Millisecond)
		settle(t, w)
	}
	if gui.State[App](w).Busy {
		t.Fatal("still busy after the result arrived")
	}
}
