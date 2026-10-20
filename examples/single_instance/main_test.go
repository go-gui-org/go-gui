package main

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// An open request reaches OnOpen through the App and lands in the
// window state the view lists.
func TestOnOpenRecordsFilesAndURLs(t *testing.T) {
	app := gui.NewApp()
	app.OnOpen = onOpen
	w := newWindow()
	app.Register(1, w)
	app.DispatchOpen(gui.OpenEvent{
		Files: []string{"/tmp/a.txt"},
		URLs:  []string{"single-instance://x"},
	})
	w.FrameFn()
	got := gui.State[DemoApp](w).Opened
	if len(got) != 2 ||
		!strings.HasSuffix(got[0], "file /tmp/a.txt") ||
		!strings.HasSuffix(got[1], "url  single-instance://x") {
		t.Fatalf("Opened = %q", got)
	}
}

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	w := newWindow()
	_ = mainView(w).GenerateLayout(w)
	gui.State[DemoApp](w).Opened = []string{"x"}
	_ = mainView(w).GenerateLayout(w)
}
