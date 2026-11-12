package gui

import "testing"

// A wheel notch eases over several frames, and each frame can measure
// chunks above the view. The anchor moves the offset by what they added;
// the notch must still move the text by one notch, in the frame the ease
// settles. An ease that kept its old target travelled the correction on
// screen as well.
func TestTextAnchorWheelNotchMovesOneNotch(t *testing.T) {
	withChunkBytes(t, 256)
	withEagerBytes(t, 0)
	text := anchorTestText(200)
	w := newBlockTestWindow(t)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "sc", Scrollable: true,
			Sizing: FixedFixed, Width: 300, Height: 200, SizeBorder: NoBorder,
			Content: []View{
				Text(TextCfg{ID: "big", Text: text, Mode: TextModeWrap}),
			}})
	})
	w.ScrollVerticalToPct("sc", 1)
	w.TestRender(nil)

	const notches = 2
	want := notches * w.themeRef().ScrollMultiplier
	corrected := 0
	for range 300 {
		before := chunkScreenYs(w)
		off0 := w.ScrollVerticalOffset("sc")
		if off0 >= -want {
			break
		}
		sc, ok := findLayoutByScrollID(&w.layout, "sc")
		if !ok {
			t.Fatal("scroll view not found")
		}
		if !scrollSmoothBy(w, sc, scrollAxisY, notches) {
			t.Fatal("the notch did not arm an ease")
		}
		for range 200 {
			if !wheelEaseTick(w) {
				break
			}
			w.TestRender(nil)
		}
		// The window's own animation loop may have taken the last tick,
		// whose value waits in the command queue: apply it before reading.
		commandApplyScrollSmooth(w)
		w.TestRender(nil)
		if off1 := w.ScrollVerticalOffset("sc"); f32Abs(off1-off0-want) > 0.01 {
			corrected++
		}
		after := chunkScreenYs(w)
		kept := 0
		for txt, y0 := range before {
			y1, ok := after[txt]
			if !ok {
				continue
			}
			kept++
			if f32Abs(y1-y0-want) > 0.01 {
				t.Fatalf("text in view moved %v px for a %v px notch (offset %v)",
					y1-y0, want, off0)
			}
		}
		if kept == 0 && len(before) > 0 {
			t.Fatalf("nothing drawn before a %v px notch is drawn after it", want)
		}
	}
	if corrected == 0 {
		t.Fatal("no notch was corrected; the estimates were right")
	}
	t.Logf("%d notches corrected", corrected)
}

// wheelEaseTick advances the ease by one frame and reports whether it is
// still running. It holds animMu, as the window's animation loop does: that
// loop runs too, once an ease has been armed.
func wheelEaseTick(w *Window) bool {
	w.animMu.Lock()
	deferred := make([]queuedCommand, 0, 2)
	ac := newAnimationCommands(&deferred)
	w.scrollSmooth.Update(w, 0.016, &ac)
	e := w.scrollSmooth.findEntry("sc", scrollAxisY)
	running := e != nil && e.active
	w.animMu.Unlock()
	commandApplyScrollSmooth(w)
	return running
}

// A correction moves a wheel ease with the content, because a notch means
// "this far from here". An ease to an offset keeps it: that is a place.
func TestScrollSmoothShiftMovesRelativeTarget(t *testing.T) {
	layout, w := makeScrollLayout("1", 100, 100, 100, 3000)
	w.scrollY().Set("1", -1000)

	if !scrollSmoothBy(w, layout, scrollAxisY, 1) {
		t.Fatal("wheel ease did not arm")
	}
	e := w.scrollSmooth.findEntry("1", scrollAxisY)
	armed := e.target
	scrollSmoothShiftY(w, "1", -300)
	if e.target != armed-300 {
		t.Errorf("wheel target = %v, want %v", e.target, armed-300)
	}
	if e.current != -1300 {
		t.Errorf("wheel current = %v, want -1300", e.current)
	}

	// The top of the content is still the top.
	scrollSmoothShiftY(w, "1", 5000)
	if e.target != 0 {
		t.Errorf("wheel target past the top = %v, want 0", e.target)
	}

	scrollSmoothCancel(w, "1", scrollAxisY)
	w.scrollY().Set("1", -1000)
	if !scrollSmoothTo(w, layout, scrollAxisY, -500) {
		t.Fatal("offset ease did not arm")
	}
	scrollSmoothShiftY(w, "1", -300)
	if e = w.scrollSmooth.findEntry("1", scrollAxisY); e.target != -500 {
		t.Errorf("offset target = %v, want -500", e.target)
	}
}
