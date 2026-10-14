package main

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The playground sanitizes corrupt state before it reaches the
// transform or the readout: non-finite values fall back, and a zero
// scale is floored so the preview cannot collapse to its origin.
func TestTransformPlaygroundSanitizesBadFloats(t *testing.T) {
	nan := float32(math.NaN())
	for _, tc := range []struct {
		name                                     string
		rotate, scale, tx, ty, mx, rx, ry, persp float32
		want                                     string
	}{
		{"non-finite falls back", nan, nan, float32(math.Inf(1)), float32(math.Inf(-1)), nan, nan, nan, nan, "Translate(0, 0).Rotate(0).Scale(1.00).MirrorX(1.00).RotateX(0).RotateY(0).Perspective(500)"},
		{"zero scale floors", 0, 0, 0, 0, 1, 0, 0, 500, "Translate(0, 0).Rotate(0).Scale(0.01).MirrorX(1.00).RotateX(0).RotateY(0).Perspective(500)"},
		{"finite passes through", 30, 1.5, 24, -12, -1, 20, -25, 800, "Translate(24, -12).Rotate(30).Scale(1.50).MirrorX(-1.00).RotateX(20).RotateY(-25).Perspective(800)"},
		{"bad perspective falls back", 0, 1, 0, 0, 1, 0, 0, -100, "Translate(0, 0).Rotate(0).Scale(1.00).MirrorX(1.00).RotateX(0).RotateY(0).Perspective(500)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newShowcaseApp()
			app.TransformRotate = tc.rotate
			app.TransformScale = tc.scale
			app.TransformTX = tc.tx
			app.TransformTY = tc.ty
			app.TransformMirrorX = tc.mx
			app.TransformRotX = tc.rx
			app.TransformRotY = tc.ry
			app.TransformPersp = tc.persp
			w := gui.NewTestWindow(t, gui.WindowCfg{State: app})
			layout := gui.GenerateViewLayout(demoTransform(w), w)
			l, ok := layout.FindByID("transform-readout")
			if !ok {
				t.Fatal("transform-readout not found")
			}
			if l.Shape.TC == nil {
				t.Fatal("transform-readout has no text config")
			}
			if got := l.Shape.TC.Text; got != tc.want {
				t.Fatalf("readout = %q, want %q", got, tc.want)
			}
		})
	}
}

// The transform seeds carry the playground defaults: scale 1 (the
// zero value would collapse the preview), mirror 1 (no flip) and a
// mid perspective distance.
func TestNewShowcaseAppSeedsTransformScale(t *testing.T) {
	app := newShowcaseApp()
	if got := app.TransformScale; got != 1 {
		t.Fatalf("TransformScale = %v, want 1", got)
	}
	if got := app.TransformMirrorX; got != 1 {
		t.Fatalf("TransformMirrorX = %v, want 1", got)
	}
	if got := app.TransformPersp; got != 500 {
		t.Fatalf("TransformPersp = %v, want 500", got)
	}
}

// transformTestWindow renders the showcase at the transform page in
// a window tall enough that the flip card and reset button are laid
// out in view: TestClick aims at the visible clip center and misses
// a scrolled-out target.
func transformTestWindow(t *testing.T, app *ShowcaseApp) *gui.Window {
	t.Helper()
	app.SelectedGroup = groupDrawing
	app.SelectedComponent = "transform"
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 2200})
	w.TestRender(mainView)
	w.TestRender(nil)
	return w
}

// Typing in the playground dialog's text field lands in state, and
// either dialog button clears it.
func TestTransformPlaygroundInputAcceptsText(t *testing.T) {
	app := newShowcaseApp()
	w := transformTestWindow(t, app)

	if err := w.TestType("detail:transform-playground-input", "hello"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if app.TransformPlaygroundText != "hello" {
		t.Fatalf("TransformPlaygroundText = %q, want %q", app.TransformPlaygroundText, "hello")
	}
}

// Clicking OK or Cancel in the playground dialog clears the text
// field, which is the visible proof a click lands through the
// playground transform (hit testing).
func TestTransformPlaygroundButtonsClearText(t *testing.T) {
	for _, tc := range []struct{ name, id string }{
		{"OK", "detail:transform-playground-ok"},
		{"Cancel", "detail:transform-playground-cancel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newShowcaseApp()
			app.TransformPlaygroundText = "hello"
			w := transformTestWindow(t, app)

			if err := w.TestClick(tc.id); err != nil {
				t.Fatalf("TestClick: %v", err)
			}
			if app.TransformPlaygroundText != "" {
				t.Fatalf("TransformPlaygroundText = %q after %s, want empty", app.TransformPlaygroundText, tc.name)
			}
		})
	}
}

// Clicking the flip card toggles the face and starts the flip tween
// toward the other side.
func TestTransformFlipClickStartsAnimation(t *testing.T) {
	app := newShowcaseApp()
	w := transformTestWindow(t, app)

	if err := w.TestClick("detail:transform-flip"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if !app.TransformFlipped {
		t.Fatal("TransformFlipped = false, want true")
	}
	if !w.HasAnimation("transform-flip") {
		t.Fatal("no transform-flip animation after click")
	}

	if err := w.TestClick("detail:transform-flip"); err != nil {
		t.Fatalf("second TestClick: %v", err)
	}
	if app.TransformFlipped {
		t.Fatal("TransformFlipped = true after second click, want false")
	}
}

// The reset button restores the playground sliders to identity.
func TestTransformResetClick(t *testing.T) {
	app := newShowcaseApp()
	app.TransformRotate = 45
	app.TransformScale = 1.5
	app.TransformTX = 10
	app.TransformTY = -10
	app.TransformMirrorX = -1
	app.TransformRotX = 20
	app.TransformRotY = -20
	app.TransformPersp = 800
	w := transformTestWindow(t, app)

	if err := w.TestClick("detail:transform-reset"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if app.TransformRotate != 0 || app.TransformTX != 0 || app.TransformTY != 0 {
		t.Fatalf("rotate/tx/ty = %v/%v/%v, want 0/0/0",
			app.TransformRotate, app.TransformTX, app.TransformTY)
	}
	if app.TransformScale != 1 {
		t.Fatalf("scale = %v, want 1", app.TransformScale)
	}
	if app.TransformMirrorX != 1 || app.TransformRotX != 0 || app.TransformRotY != 0 {
		t.Fatalf("mirror/rotx/roty = %v/%v/%v, want 1/0/0",
			app.TransformMirrorX, app.TransformRotX, app.TransformRotY)
	}
	if app.TransformPersp != 500 {
		t.Fatalf("persp = %v, want 500", app.TransformPersp)
	}
}
