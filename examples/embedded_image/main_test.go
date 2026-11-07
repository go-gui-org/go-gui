package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoFindings(t *testing.T) {
	pictures, err := loadPictures()
	if err != nil {
		t.Fatal(err)
	}
	w := gui.NewTestWindow(t, gui.WindowCfg{
		State:  &App{Pictures: pictures},
		Width:  720,
		Height: 320,
	})
	w.TestRender(mainView)
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}

// Every embedded encoding decodes to the full scene, and a second load
// returns the same sources: the keys are already registered.
func TestPicturesDecode(t *testing.T) {
	first, err := loadPictures()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 {
		t.Fatalf("got %d pictures, want 3", len(first))
	}
	for _, p := range first {
		w, h, _, ok := gui.LookupImage(p.src)
		if !ok || w != sceneW || h != sceneH {
			t.Errorf("%s: %dx%d ok=%t, want %dx%d", p.caption, w, h, ok, sceneW, sceneH)
		}
	}
	again, err := loadPictures()
	if err != nil {
		t.Fatal(err)
	}
	for i := range first {
		if again[i].src != first[i].src {
			t.Errorf("second load: src %q, want %q", again[i].src, first[i].src)
		}
	}
}
