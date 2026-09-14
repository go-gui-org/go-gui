package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestImageFactory(t *testing.T) {
	v := Image(ImageCfg{ID: "img1", Src: "test.png"})
	if v == nil {
		t.Fatal("Image factory returned nil")
	}
	if _, ok := v.(*imageView); !ok {
		t.Fatal("expected *imageView")
	}
}

func TestImageInvisible(t *testing.T) {
	v := Image(ImageCfg{ID: "img1", Invisible: true})
	if _, ok := v.(*imageView); ok {
		t.Fatal("invisible should not return *imageView")
	}
}

func TestImageGenerateLayoutLocalMissing(t *testing.T) {
	w := &Window{}
	v := Image(ImageCfg{
		ID:  "img1",
		Src: "/nonexistent/photo.png",
	})
	layout := v.GenerateLayout(w)
	// Missing file → error text with magenta color.
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
	if layout.Shape.shapeType != shapeText {
		t.Fatalf("expected shapeText for missing, got %d",
			layout.Shape.shapeType)
	}
}

func TestImageGenerateLayoutLocalExists(t *testing.T) {
	// Create a temp file to simulate an image.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := &Window{}
	v := Image(ImageCfg{
		ID:     "img1",
		Src:    path,
		Width:  200,
		Height: 150,
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.shapeType != shapeImage {
		t.Fatalf("expected shapeImage, got %d",
			layout.Shape.shapeType)
	}
	if layout.Shape.Resource != path {
		t.Fatalf("expected resource %s, got %s",
			path, layout.Shape.Resource)
	}
	if layout.Shape.Width != 200 {
		t.Fatalf("expected width 200, got %f",
			layout.Shape.Width)
	}
}

func TestImageOpacityDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	v := Image(ImageCfg{Src: path, Width: 100, Height: 100})
	layout := v.GenerateLayout(w)
	if layout.Shape.Opacity != 1.0 {
		t.Fatalf("expected Opacity 1.0, got %f",
			layout.Shape.Opacity)
	}
}

func TestImageBgColor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	v := Image(ImageCfg{
		Src: path, Width: 100, Height: 100,
		BgColor: White,
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.Color != White {
		t.Fatalf("expected Color White, got %v",
			layout.Shape.Color)
	}
}

func TestImageDefaultDimensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	v := Image(ImageCfg{ID: "img1", Src: path})
	layout := v.GenerateLayout(w)
	if layout.Shape.Width != 100 || layout.Shape.Height != 100 {
		t.Fatalf("expected default 100x100, got %fx%f",
			layout.Shape.Width, layout.Shape.Height)
	}
}

func TestImageWithEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	clicked := false
	v := Image(ImageCfg{
		ID:    "img1",
		Src:   path,
		Width: 50, Height: 50,
		OnClick: func(ctx EventCtx) {
			clicked = true
		},
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.events == nil {
		t.Fatal("expected events")
	}
	// Simulate left click.
	layout.Shape.events.OnClick(EventCtx{&layout, &Event{
		MouseButton: MouseLeft,
	}, w})
	if !clicked {
		t.Fatal("click handler not called")
	}
	_ = clicked
}

func TestImageA11Y(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	v := Image(ImageCfg{
		ID:     "img1",
		Src:    path,
		Width:  50,
		Height: 50,
		A11YCfg: A11YCfg{
			A11YLabel:       "test image",
			A11YDescription: "a test",
		},
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.A11YRole != AccessRoleImage {
		t.Fatal("expected AccessRoleImage")
	}
	if layout.Shape.a11Y == nil {
		t.Fatal("expected A11Y info")
	}
	if layout.Shape.a11Y.Label != "test image" {
		t.Fatalf("expected label 'test image', got %q",
			layout.Shape.a11Y.Label)
	}
}

func TestDownloadingPlaceholder(t *testing.T) {
	// Neutral rectangle shown while a remote image download is in
	// flight: default 100x100, theme background, carries the ID.
	w := &Window{}
	layout := downloadingPlaceholder(&ImageCfg{ID: "img-remote"}, w)
	if layout.Shape.shapeType != shapeRectangle {
		t.Fatalf("placeholder shapeType = %d, want rectangle",
			layout.Shape.shapeType)
	}
	if layout.Shape.ID != "img-remote" {
		t.Fatalf("placeholder ID = %q, want img-remote", layout.Shape.ID)
	}
	if layout.Shape.Width != 100 || layout.Shape.Height != 100 {
		t.Fatalf("placeholder size = %fx%f, want 100x100",
			layout.Shape.Width, layout.Shape.Height)
	}

	// Explicit dimensions and opacity are honored.
	cfg := ImageCfg{ID: "img2", Width: 200, Height: 50, Opacity: SomeF(0.5)}
	layout2 := downloadingPlaceholder(&cfg, w)
	if layout2.Shape.Width != 200 || layout2.Shape.Height != 50 {
		t.Fatalf("placeholder size = %fx%f, want 200x50",
			layout2.Shape.Width, layout2.Shape.Height)
	}
	if layout2.Shape.Opacity != 0.5 {
		t.Fatalf("placeholder opacity = %v, want 0.5",
			layout2.Shape.Opacity)
	}
}

// A remote image that resolves to an SVG must keep the ImageCfg
// identity, click handler and assistive label instead of dropping
// them at the svgView handoff.
func TestImageRemoteSVGForwardsIdentity(t *testing.T) {
	for _, cacheName := range []string{"abc.svg", "abc.SVG"} {
		url := "https://example.test/" + t.Name() + "/" + cacheName
		// The resolved cache path must exist: svgView stats it.
		path := filepath.Join(t.TempDir(), cacheName)
		if err := os.WriteFile(path, []byte("<svg></svg>"),
			0o644); err != nil {
			t.Fatal(err)
		}
		w := &Window{}
		w.SetSvgParser(&mockSvgParser{width: 64, height: 64})
		resolved := StateMap[string, string](
			w, nsImageResolved, capImageCache)
		resolved.Set(url, path)

		clicked := false
		v := Image(ImageCfg{
			ID:     "img-svg",
			Src:    url,
			Width:  100,
			Height: 100,
			OnClick: func(ctx EventCtx) {
				clicked = true
			},
			A11YCfg: A11YCfg{A11YLabel: "remote svg"},
		})
		layout := v.GenerateLayout(w)
		if layout.Shape.shapeType != shapeSVG {
			t.Fatalf("%s: shapeType = %d, want shapeSVG",
				cacheName, layout.Shape.shapeType)
		}
		if layout.Shape.ID != "img-svg" {
			t.Errorf("%s: ID = %q, want img-svg",
				cacheName, layout.Shape.ID)
		}
		if layout.Shape.a11Y == nil ||
			layout.Shape.a11Y.Label != "remote svg" {
			t.Errorf("%s: A11Y label lost at SVG handoff", cacheName)
		}
		if layout.Shape.events == nil {
			t.Fatalf("%s: OnClick lost at SVG handoff", cacheName)
		}
		layout.Shape.events.OnClick(EventCtx{&layout, &Event{
			MouseButton: MouseLeft,
		}, w})
		if !clicked {
			t.Errorf("%s: click handler not called", cacheName)
		}
	}
}

// A missing file warns once per window, not once per frame.
func TestImageMissingWarnsOnce(t *testing.T) {
	w := &Window{}
	src := "/nonexistent/warn-once-photo.png"
	v := Image(ImageCfg{ID: "img1", Src: src})
	v.GenerateLayout(w)
	v.GenerateLayout(w)
	warned := StateMapRead[string, bool](w, nsImageWarned)
	if warned == nil || !warned.Contains(src) {
		t.Fatal("expected the missing source in the warned set")
	}
}

func TestTruncateSrc(t *testing.T) {
	short := "/a/b.png"
	if got := truncateSrc(short); got != short {
		t.Fatalf("short src rewritten: %q", got)
	}
	long := "/images/" + strings.Repeat("a", 200) + ".png"
	got := truncateSrc(long)
	if len(got) > 80+len("…") {
		t.Fatalf("long src not capped: len %d", len(got))
	}
	// A multibyte character straddling the cut must not split
	// into invalid UTF-8 in the framed UI text.
	multi := "/images/" + strings.Repeat("é", 100) + ".png"
	gotMulti := truncateSrc(multi)
	if !utf8.ValidString(gotMulti) {
		t.Fatalf("multibyte src cut mid-rune: %q", gotMulti)
	}
	if len(gotMulti) > 80+len("…")+len("é") {
		t.Fatalf("multibyte src not capped: len %d", len(gotMulti))
	}
}
