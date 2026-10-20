package instance

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// TestAttachDeliversForwardedArgsToOnOpen runs the whole path: a
// secondary's argv → the primary's accept loop → gui.ParseOpenArgs →
// App.DispatchOpen → OnOpen on the main window's frame.
func TestAttachDeliversForwardedArgsToOnOpen(t *testing.T) {
	id := testID(t)
	l := mustPrimary(t, id)
	app := gui.NewApp()
	var got []gui.OpenEvent
	app.OnOpen = func(_ *gui.App, ev gui.OpenEvent) { got = append(got, ev) }
	// os.Args[1:] of a test binary is -test.* flags only, which
	// ParseOpenArgs skips, so this launch dispatches nothing itself.
	l.Attach(app)
	w := gui.NewWindow(gui.WindowCfg{})
	app.Register(1, w)

	file := filepath.Join(t.TempDir(), "doc.txt")
	if _, primary, err := acquire(id, []string{"-v", file, "my://open"}); err != nil || primary {
		t.Fatalf("acquire = (%v, %v), want forwarded", primary, err)
	}
	// The accept goroutine queues the event; poll the frame for it.
	deadline := time.Now().Add(5 * time.Second)
	for len(got) == 0 && time.Now().Before(deadline) {
		w.FrameFn()
		time.Sleep(10 * time.Millisecond)
	}
	if len(got) != 1 {
		t.Fatalf("OnOpen ran %d times, want 1", len(got))
	}
	ev := got[0]
	if len(ev.Files) != 1 || ev.Files[0] != file {
		t.Errorf("Files = %q, want [%q]", ev.Files, file)
	}
	if len(ev.URLs) != 1 || ev.URLs[0] != "my://open" {
		t.Errorf("URLs = %q, want [my://open]", ev.URLs)
	}
}

func TestAttachNilLockDispatchesOwnArgs(t *testing.T) {
	// No panic, and with flag-only os.Args nothing reaches OnOpen.
	var l *Lock
	app := gui.NewApp()
	calls := 0
	app.OnOpen = func(*gui.App, gui.OpenEvent) { calls++ }
	l.Attach(app)
	w := gui.NewWindow(gui.WindowCfg{})
	app.Register(1, w)
	w.FrameFn()
	if calls != 0 {
		t.Fatalf("OnOpen ran %d times for flag-only args", calls)
	}
}
