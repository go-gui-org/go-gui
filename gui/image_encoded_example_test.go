package gui_test

import (
	_ "embed"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// This file proves an app outside the gui package can draw a go:embed
// PNG without a temporary file (#1228). It also pins the image guide
// section that quotes it (examples/showcase/sound_snippet_test.go).

// doc:snippet-begin encoded-image
//
// loadImages registers the embedded logo once, at startup. Its Src stays
// valid until DropImage: the in-memory image budget does not evict it.

//go:embed default_icon.png
var logoPNG []byte

var logoSrc string

func loadImages() error {
	var err error
	logoSrc, err = gui.RegisterEncodedImage("myapp/logo-v1", logoPNG)
	return err
}

func logoView() gui.View {
	return gui.Image(gui.ImageCfg{
		Src:    logoSrc,
		Width:  128,
		Height: 128,
	})
}

// doc:snippet-end encoded-image

func TestEncodedImageExample(t *testing.T) {
	t.Cleanup(func() { gui.DropImage("myapp/logo-v1") })
	if err := loadImages(); err != nil {
		t.Fatal(err)
	}
	if w, h, _, ok := gui.LookupImage(logoSrc); !ok || w != 128 || h != 128 {
		t.Fatalf("registered logo = %dx%d ok=%t, want 128x128", w, h, ok)
	}
	w := gui.NewWindow(gui.WindowCfg{Width: 300, Height: 200})
	if got := logoView().GenerateLayout(w).Shape.Resource; got != logoSrc {
		t.Errorf("Image Resource = %q, want %q", got, logoSrc)
	}
}
