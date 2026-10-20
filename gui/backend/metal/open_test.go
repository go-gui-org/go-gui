//go:build darwin && cgo && !ios

package metal

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestDeliverOpenSplitsFilesAndURLs(t *testing.T) {
	app := gui.NewApp()
	var got []gui.OpenEvent
	app.OnOpen = func(_ *gui.App, ev gui.OpenEvent) { got = append(got, ev) }
	w := gui.NewWindow(gui.WindowCfg{})
	app.Register(1, w)

	deliverOpen(app,
		[]string{"/tmp/a.txt", "myapp://x?y=1", "/tmp/b c.txt"},
		[]bool{true, false, true})
	w.FrameFn()

	if len(got) != 1 {
		t.Fatalf("OnOpen ran %d times, want 1", len(got))
	}
	ev := got[0]
	if len(ev.Files) != 2 || ev.Files[0] != "/tmp/a.txt" || ev.Files[1] != "/tmp/b c.txt" {
		t.Errorf("Files = %q", ev.Files)
	}
	if len(ev.URLs) != 1 || ev.URLs[0] != "myapp://x?y=1" {
		t.Errorf("URLs = %q", ev.URLs)
	}
}

// Single-window Run sets no App; the request is dropped, not a panic.
func TestDeliverOpenNoAppDrops(t *testing.T) {
	deliverOpen(nil, []string{"/tmp/a.txt"}, []bool{true})
}
