package gui

// Golden-file harness for the render pipeline.
//
// Every visual claim in this package was previously verified by one
// person looking at a screen once. These tests make a visual change
// reviewable as a diff: build a widget, run the real pipeline to
// w.renderers, serialize the emitted []RenderCmd to a stable text
// form, and compare against a recorded file in testdata/.
//
// The harness itself is public (gui/golden.go): TestGolden is what
// apps and sibling repos call to pin their own appearance. This file
// holds only the in-repo case list driver and the -update flag shim.
//
// Re-record after reading the diff:
//
//	go test ./gui/ -run TestGolden -update
//
// Consumers outside this module re-record with GOGUI_UPDATE_GOLDEN=1
// instead: a flag.Bool in a library package collides with the
// consumer's own flags, so the public helper reads the env var and
// this flag only bridges it for the in-repo workflow.
//
// Each case is recorded in both ThemeDark and ThemeLight, which is
// what makes a per-theme styling decision checkable.

import (
	"flag"
	"testing"
)

var updateGolden = flag.Bool("update", false,
	"re-record golden render-command files")

// goldenWindowSize is fixed so a golden never encodes the machine it
// was recorded on.
const (
	goldenWidth  = 320
	goldenHeight = 240
)

// renderGolden drives one frame of one case under one theme and
// returns the header-prefixed serialized command list. Single-theme
// callers (theme_patch_test.go) use this; TestGolden below covers
// the dark/light pair through the public helper.
func renderGolden(t *testing.T, theme Theme, c goldenCase) string {
	t.Helper()

	w := NewTestWindow(t, WindowCfg{
		State:  new(int),
		Width:  goldenWidth,
		Height: goldenHeight,
	})
	// Wrap in a filling root. A widget generated as the bare root
	// sizes Fit, and with a nil TextMeasurer several collapse to zero
	// and emit nothing. Real apps put widgets inside a layout, so the
	// wrapper is both what makes the case record and what the widget
	// actually sees.
	w.SetView(wrapGoldenRoot(c.build))
	// Pin the clock (goldenInstant). The window is per-case garbage,
	// so no restore is needed.
	goldenNow := goldenInstant()
	w.setVirtualNow(&goldenNow)
	return goldenFrame(t, w, theme, GoldenCfg{
		FocusID:      c.focusID,
		KeyPressID:   c.keyPressID,
		HoverX:       c.hoverX,
		HoverY:       c.hoverY,
		MousePressed: c.mousePressed,
		HoverInert:   c.hoverInert,
	}, c.name)
}

// goldenCase is one recorded widget. build receives the window so a
// case can reach window-scoped state; most ignore it.
type goldenCase struct {
	build func(*Window) View
	name  string
	// focusID, when set, is focused before the frame is rendered.
	// Focus resolves after layout, so a focus ring only appears in a
	// recording that actually holds focus — a case without this pins
	// the resting appearance and would not notice a ring regressing.
	focusID string
	// keyPressID, when set, is held pressed by a Space key down before
	// the frame is rendered (#658). Set after focusID, because a focus
	// change cancels a key press.
	keyPressID string

	// hoverX, hoverY place the pointer before the frame is rendered, so
	// layoutHover fires and the recording pins the hovered appearance.
	// Without them no golden hovers anything: renderGolden leaves the
	// pointer at the origin, layoutHover finds nothing under it, and a
	// hover-color regression records clean. Take the coordinates from
	// the case's own resting recording, not by guessing — hoverAsserts
	// fails a case whose point misses.
	hoverX, hoverY float32

	// mousePressed holds the left button down for the hovered frame,
	// pinning the pressed-by-mouse appearance the way keyPressID pins
	// the pressed-by-Space one (#658).
	//
	// A bool rather than a MouseButton because MouseLeft is 0
	// (gui/event.go:64): a MouseButton field would mark every case that
	// did not mention it as pressed. NewWindow dodges the same trap by
	// seeding mouseButtonHeld to MouseInvalid (gui/window_cfg.go:169).
	mousePressed bool

	// hoverInert says the pointer is expected to land on nothing, which
	// is what a disabled widget does: layoutHoverDepth returns before
	// the callback and recordHoverTarget skips disabled shapes, so the
	// target stays empty. Without this the hit assert below would fail
	// every disabled-hover case, which are the ones worth recording.
	hoverInert bool
}

// wrapGoldenRoot wraps a case build in a filling root. See
// renderGolden for why the wrapper is load bearing.
func wrapGoldenRoot(build func(*Window) View) func(*Window) View {
	return func(win *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{build(win)},
		})
	}
}

func TestGolden(t *testing.T) {
	if *updateGolden {
		t.Setenv("GOGUI_UPDATE_GOLDEN", "1")
	}
	for _, c := range goldenCases() {
		t.Run(c.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{
				State:  new(int),
				Width:  goldenWidth,
				Height: goldenHeight,
			})
			w.SetView(wrapGoldenRoot(c.build))
			w.TestGolden(t, GoldenCfg{
				Dir:          "testdata",
				Name:         c.name,
				FocusID:      c.focusID,
				KeyPressID:   c.keyPressID,
				HoverX:       c.hoverX,
				HoverY:       c.hoverY,
				MousePressed: c.mousePressed,
				HoverInert:   c.hoverInert,
			})
		})
	}
}
