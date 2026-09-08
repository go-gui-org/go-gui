package gui

import (
	"testing"
	"time"
)

// seedTextAnim pins an animated text's progress so a test sees one
// deterministic frame. done is set too, which keeps applyTextAnim from
// registering a live animation — a test asserts on a frame, not on a
// goroutine.
func seedTextAnim(w *Window, key string, progress float32) {
	StateMap[string, textAnimState](w, nsTextAnim, capMany).
		Set(key, textAnimState{progress: progress, done: true})
}

func TestTextAnimCfgIsSet(t *testing.T) {
	cases := []struct {
		name string
		cfg  TextAnimCfg
		want bool
	}{
		{"zero", TextAnimCfg{}, false},
		{"kind", TextAnimCfg{Kind: TextAnimFadeIn}, true},
		{"custom", TextAnimCfg{Custom: func(float32) TextAnimFrame {
			return TextAnimFrame{}
		}}, true},
		{"duration only", TextAnimCfg{Duration: time.Second}, false},
		{"out of range kind", TextAnimCfg{
			Kind: textAnimKindCount,
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cfg.isSet(); got != c.want {
				t.Errorf("isSet() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestTextAnimZeroCfgInert is the guarantee every existing Text
// depends on: a Text that never mentions Anim must render exactly as
// it did before the field existed.
func TestTextAnimZeroCfgInert(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{ID: "plain", Text: "hello"})
	sh := v.GenerateLayout(w).Shape

	if sh.Opacity != 1 {
		t.Errorf("Opacity = %v, want 1", sh.Opacity)
	}
	if sh.TC.Text != "hello" {
		t.Errorf("Text = %q, want %q", sh.TC.Text, "hello")
	}
	if sh.TC.TextStyle.AffineTransform != nil {
		t.Error("zero Anim installed a transform")
	}
	if sh.TC.TextStyle.Gradient != nil {
		t.Error("zero Anim installed a gradient")
	}
}

// TestTextAnimSamplerEndpoints pins the shape of every canned kind at
// the start, middle and end of its cycle. These are the numbers a
// caller sees, so a change here is a change to the look of the widget.
func TestTextAnimSamplerEndpoints(t *testing.T) {
	const em = float32(16)

	opacityAt := func(k TextAnimKind, p float32) float32 {
		f := sampleTextAnim(k, p, em)
		v, ok := f.Opacity.Value()
		if !ok {
			t.Helper()
			t.Fatalf("kind %d at p=%v set no opacity", k, p)
		}
		return v
	}

	t.Run("fade in rises", func(t *testing.T) {
		if got := opacityAt(TextAnimFadeIn, 0); got != 0 {
			t.Errorf("p=0 opacity = %v, want 0", got)
		}
		if got := opacityAt(TextAnimFadeIn, 1); got != 1 {
			t.Errorf("p=1 opacity = %v, want 1", got)
		}
	})

	t.Run("fade out falls", func(t *testing.T) {
		if got := opacityAt(TextAnimFadeOut, 0); got != 1 {
			t.Errorf("p=0 opacity = %v, want 1", got)
		}
		if got := opacityAt(TextAnimFadeOut, 1); got != 0 {
			t.Errorf("p=1 opacity = %v, want 0", got)
		}
	})

	// A loop that does not start and end at the same value shows a
	// jump once per cycle, so the seam is the property worth pinning.
	t.Run("pulse cycle joins up", func(t *testing.T) {
		start := opacityAt(TextAnimPulse, 0)
		end := opacityAt(TextAnimPulse, 1)
		if !f32AreClose(start, end) {
			t.Errorf("start %v != end %v", start, end)
		}
		if !f32AreClose(start, 1) {
			t.Errorf("cycle starts at %v, want full brightness", start)
		}
		if mid := opacityAt(TextAnimPulse, 0.5); !f32AreClose(
			mid, textAnimPulseFloor) {
			t.Errorf("mid opacity = %v, want floor %v",
				mid, textAnimPulseFloor)
		}
	})

	t.Run("shake cycle joins up", func(t *testing.T) {
		start := sampleTextAnim(TextAnimShake, 0, em).OffsetX
		end := sampleTextAnim(TextAnimShake, 1, em).OffsetX
		if !f32AreClose(start, 0) || !f32AreClose(end, 0) {
			t.Errorf("shake ends: start %v end %v, want 0", start, end)
		}
	})

	// Each slide starts displaced along its own axis, in the direction
	// it travels from, and lands on zero.
	slides := []struct {
		kind     TextAnimKind
		wantX    float32
		wantY    float32
		axisName string
	}{
		{TextAnimSlideUp, 0, textAnimSlideEm * em, "up"},
		{TextAnimSlideDown, 0, -textAnimSlideEm * em, "down"},
		{TextAnimSlideLeft, textAnimSlideEm * em, 0, "left"},
		{TextAnimSlideRight, -textAnimSlideEm * em, 0, "right"},
	}
	for _, s := range slides {
		t.Run("slide "+s.axisName, func(t *testing.T) {
			at0 := sampleTextAnim(s.kind, 0, em)
			if at0.OffsetX != s.wantX || at0.OffsetY != s.wantY {
				t.Errorf("p=0 offset = (%v,%v), want (%v,%v)",
					at0.OffsetX, at0.OffsetY, s.wantX, s.wantY)
			}
			at1 := sampleTextAnim(s.kind, 1, em)
			if at1.OffsetX != 0 || at1.OffsetY != 0 {
				t.Errorf("p=1 offset = (%v,%v), want (0,0)",
					at1.OffsetX, at1.OffsetY)
			}
		})
	}

	t.Run("pop grows to full size", func(t *testing.T) {
		if got := sampleTextAnim(TextAnimPop, 0, em).Scale; got !=
			textAnimPopFrom {
			t.Errorf("p=0 scale = %v, want %v", got, textAnimPopFrom)
		}
		if got := sampleTextAnim(TextAnimPop, 1, em).Scale; got != 1 {
			t.Errorf("p=1 scale = %v, want 1", got)
		}
	})

	// easeOutBack carries progress past 1, and an alpha above 1 is not
	// a color. The sampler clamps rather than leaving that to render.
	t.Run("pop opacity clamps past one", func(t *testing.T) {
		if got := opacityAt(TextAnimPop, 1.12); got != 1 {
			t.Errorf("overshoot opacity = %v, want 1", got)
		}
	})

	t.Run("typewriter reveals", func(t *testing.T) {
		f := sampleTextAnim(TextAnimTypewriter, 0.5, em)
		if got := f.Reveal.Get(-1); got != 0.5 {
			t.Errorf("reveal = %v, want 0.5", got)
		}
	})

	// Shimmer paints through a gradient, not through frame fields.
	t.Run("shimmer frame is inert", func(t *testing.T) {
		if f := sampleTextAnim(TextAnimShimmer, 0.5, em); f !=
			(TextAnimFrame{}) {
			t.Errorf("frame = %+v, want zero", f)
		}
	})
}

func TestTextAnimReveal(t *testing.T) {
	cases := []struct {
		name string
		in   string
		frac float32
		want string
	}{
		{"none", "hello", 0, ""},
		{"all", "hello", 1, "hello"},
		{"past end", "hello", 2, "hello"},
		{"negative", "hello", -0.5, ""},
		{"half", "hello", 0.6, "hel"},
		{"empty", "", 0.5, ""},
		// Rounds down: a character only appears once it is fully due.
		{"rounds down", "abcd", 0.7, "ab"},
		// Multi-byte runes must not be cut mid-character, or the
		// painted string carries a replacement glyph.
		{"multibyte", "héllo", 0.4, "hé"},
		{"emoji", "a👍b", 0.7, "a👍"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := textAnimReveal(c.in, c.frac); got != c.want {
				t.Errorf("textAnimReveal(%q, %v) = %q, want %q",
					c.in, c.frac, got, c.want)
			}
		})
	}
}

// TestTextAnimTypewriterKeepsFullWidth is the reason the reveal is
// applied after the measure string is chosen: a typewriter that
// measured what it paints would grow its box rune by rune and reflow
// everything beside it.
func TestTextAnimTypewriterKeepsFullWidth(t *testing.T) {
	w := newTestWindow()

	full := Text(TextCfg{ID: "t", Text: "hello world"}).
		GenerateLayout(w).Shape.Width

	seedTextAnim(w, "t", 0.25)
	v := Text(TextCfg{
		ID:   "t",
		Text: "hello world",
		Anim: TextAnimCfg{Kind: TextAnimTypewriter},
	})
	sh := v.GenerateLayout(w).Shape

	if sh.Width != full {
		t.Errorf("Width = %v, want the full string's %v", sh.Width, full)
	}
	if sh.TC.Text == "hello world" || sh.TC.Text == "" {
		t.Errorf("painted text = %q, want a partial reveal", sh.TC.Text)
	}
}

func TestTextAnimAppliesOpacity(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "fade", 0)

	sh := Text(TextCfg{
		ID:   "fade",
		Text: "hi",
		Anim: TextAnimCfg{Kind: TextAnimFadeIn},
	}).GenerateLayout(w).Shape

	if sh.Opacity != 0 {
		t.Errorf("Opacity = %v, want 0 at the start of a fade in",
			sh.Opacity)
	}
}

// TestTextAnimOpacityCompounds: TextCfg.Opacity is the author's
// setting and the animation is a modifier on it, so a half-transparent
// label that fades in must never become more opaque than it was told
// to be.
func TestTextAnimOpacityCompounds(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "fade", 1)

	sh := Text(TextCfg{
		ID:      "fade",
		Text:    "hi",
		Opacity: SomeF(0.5),
		Anim:    TextAnimCfg{Kind: TextAnimFadeIn},
	}).GenerateLayout(w).Shape

	if !f32AreClose(sh.Opacity, 0.5) {
		t.Errorf("Opacity = %v, want the cfg's 0.5", sh.Opacity)
	}
}

// TestTextAnimFastPathPreserved guards the performance property that
// decided the design: an effect with no motion must not install a
// transform, because any transform pushes the text off the fast
// RenderText path onto the glyph-layout path.
func TestTextAnimFastPathPreserved(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "fade", 0.5)

	sh := Text(TextCfg{
		ID:   "fade",
		Text: "hi",
		Anim: TextAnimCfg{Kind: TextAnimFadeIn},
	}).GenerateLayout(w).Shape

	if sh.TC.TextStyle.AffineTransform != nil {
		t.Error("a fade installed a transform; it must stay on the " +
			"fast path")
	}
}

func TestTextAnimInstallsTransformForMotion(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "slide", 0)

	sh := Text(TextCfg{
		ID:   "slide",
		Text: "hi",
		Anim: TextAnimCfg{Kind: TextAnimSlideUp},
	}).GenerateLayout(w).Shape

	tr := sh.TC.TextStyle.AffineTransform
	if tr == nil {
		t.Fatal("a slide installed no transform")
	}
	if tr.Y0 <= 0 {
		t.Errorf("Y0 = %v, want a downward start offset", tr.Y0)
	}
}

// A non-finite value anywhere in the transform makes the whole render
// command invalid, so the text would vanish for that frame. A Custom
// callback is the realistic source of one.
func TestTextAnimRejectsNonFiniteTransform(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "bad", 0.5)

	inf := float32(1)
	for range 40 {
		inf *= 1e10
	}

	sh := Text(TextCfg{
		ID:   "bad",
		Text: "hi",
		Anim: TextAnimCfg{
			Custom: func(float32) TextAnimFrame {
				return TextAnimFrame{OffsetX: inf}
			},
		},
	}).GenerateLayout(w).Shape

	if sh.TC.TextStyle.AffineTransform != nil {
		t.Error("a non-finite offset installed a transform")
	}
}

func TestTextAnimCustomOverridesKind(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "c", 1)

	sh := Text(TextCfg{
		ID:   "c",
		Text: "hi",
		Anim: TextAnimCfg{
			// Kind says fade to full; Custom says fade to nothing.
			Kind: TextAnimFadeIn,
			Custom: func(float32) TextAnimFrame {
				return TextAnimFrame{Opacity: SomeF(0)}
			},
		},
	}).GenerateLayout(w).Shape

	if sh.Opacity != 0 {
		t.Errorf("Opacity = %v, want Custom's 0", sh.Opacity)
	}
	if sh.TC.TextStyle.Gradient != nil {
		t.Error("Custom must not pick up a Kind's gradient")
	}
}

func TestTextAnimShimmerBuildsGradient(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "sh", 0.5)

	sh := Text(TextCfg{
		ID:   "sh",
		Text: "loading",
		Anim: TextAnimCfg{Kind: TextAnimShimmer, Repeat: true},
	}).GenerateLayout(w).Shape

	g := sh.TC.TextStyle.Gradient
	if g == nil {
		t.Fatal("shimmer installed no gradient")
	}
	if len(g.Stops) != 5 {
		t.Fatalf("stops = %d, want 5", len(g.Stops))
	}
	// Stops must be sorted ascending; glyph documents that as a
	// precondition and does not sort them itself.
	for i := 1; i < len(g.Stops); i++ {
		if g.Stops[i].Position < g.Stops[i-1].Position {
			t.Errorf("stop %d at %v is before stop %d at %v",
				i, g.Stops[i].Position,
				i-1, g.Stops[i-1].Position)
		}
	}
}

// The highlight has to travel: a shimmer whose band sits still is just
// a gradient.
func TestTextAnimShimmerBandMoves(t *testing.T) {
	w := newTestWindow()

	bandAt := func(p float32) float32 {
		seedTextAnim(w, "sh", p)
		sh := Text(TextCfg{
			ID:   "sh",
			Text: "loading",
			Anim: TextAnimCfg{Kind: TextAnimShimmer, Repeat: true},
		}).GenerateLayout(w).Shape
		return sh.TC.TextStyle.Gradient.Stops[2].Position
	}

	if early, late := bandAt(0.2), bandAt(0.8); early >= late {
		t.Errorf("band did not travel: %v at p=0.2, %v at p=0.8",
			early, late)
	}
}

// A non-finite progress would bake NaN stop positions into the
// shimmer gradient — f32Clamp passes NaN through — so the frame
// renders unanimated instead.
func TestTextAnimShimmerNaNProgressNoGradient(t *testing.T) {
	w := newTestWindow()
	zero := float32(0)
	seedTextAnim(w, "sh", zero/zero)

	sh := Text(TextCfg{
		ID:   "sh",
		Text: "loading",
		Anim: TextAnimCfg{Kind: TextAnimShimmer, Repeat: true},
	}).GenerateLayout(w).Shape

	if sh.TC.TextStyle.Gradient != nil {
		t.Error("non-finite progress installed a gradient")
	}
	if sh.Opacity != 1 {
		t.Errorf("Opacity = %v, want 1", sh.Opacity)
	}
}

// A loop eased like an entrance would stall at both ends, and
// because the end wraps to the start the seam shows as a stutter
// once per cycle — so loops stay linear and entrances ease out.
func TestTextAnimDefaultEasing(t *testing.T) {
	linear := []TextAnimKind{
		TextAnimPulse, TextAnimShake, TextAnimShimmer,
		TextAnimTypewriter,
	}
	for _, k := range linear {
		if got := textAnimDefaultEasing(k)(0.37); got != 0.37 {
			t.Errorf("kind %d eases 0.37 to %v, want linear",
				k, got)
		}
	}

	if got := textAnimDefaultEasing(TextAnimFadeIn)(0.5); !f32AreClose(
		got, 0.875) {
		t.Errorf("entrance eases 0.5 to %v, want 0.875", got)
	}

	if got := textAnimDefaultEasing(TextAnimPop)(0.7); got <= 1 {
		t.Errorf("pop eases 0.7 to %v, want an overshoot past 1",
			got)
	}
}

func TestTextAnimDefaultDuration(t *testing.T) {
	cases := []struct {
		name  string
		kind  TextAnimKind
		runes int
		want  time.Duration
	}{
		{"entrance", TextAnimFadeIn, 5, textAnimDurationEntrance},
		{"pulse", TextAnimPulse, 5, textAnimDurationPulse},
		{"shake", TextAnimShake, 5, textAnimDurationShake},
		{"shimmer", TextAnimShimmer, 5, textAnimDurationShimmer},
		// Short strings take the floor, long ones scale per rune.
		{"short typewriter", TextAnimTypewriter, 2, textAnimTypeMin},
		{"long typewriter", TextAnimTypewriter, 40,
			40 * textAnimTypeRate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := textAnimDefaultDuration(c.kind, c.runes)
			if got != c.want {
				t.Errorf("duration = %v, want %v", got, c.want)
			}
		})
	}
}

// A delay must not shorten the animation: the driver runs for
// delay+duration and holds its first value until the delay is spent.
func TestTextAnimDriverDelay(t *testing.T) {
	a := newTextAnimDriver("id", "key",
		300*time.Millisecond, 100*time.Millisecond, false)

	if want := 400 * time.Millisecond; a.Duration != want {
		t.Errorf("Duration = %v, want %v", a.Duration, want)
	}
	if len(a.Keyframes) != 3 {
		t.Fatalf("keyframes = %d, want 3 (start, delay end, finish)",
			len(a.Keyframes))
	}
	if a.Keyframes[1].Value != 0 {
		t.Errorf("value at the end of the delay = %v, want 0",
			a.Keyframes[1].Value)
	}
	if got := a.Keyframes[1].At; !f32AreClose(got, 0.25) {
		t.Errorf("delay ends at %v, want 0.25 of the run", got)
	}

	noDelay := newTextAnimDriver("id", "key",
		300*time.Millisecond, 0, false)
	if len(noDelay.Keyframes) != 2 {
		t.Errorf("keyframes with no delay = %d, want 2",
			len(noDelay.Keyframes))
	}
}

// A finished one-shot must not register again. The loop deletes a
// stopped animation, so without the done flag the next frame would
// find it missing and start the entrance over, for ever.
func TestTextAnimOneShotDoesNotReregister(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "once", 1)

	Text(TextCfg{
		ID:   "once",
		Text: "hi",
		Anim: TextAnimCfg{Kind: TextAnimFadeIn},
	}).GenerateLayout(w)

	if w.HasAnimation(ScopeID("textanim", "once")) {
		t.Error("a finished entrance registered itself again")
	}
}

func TestTextAnimRegistersOnFirstFrame(t *testing.T) {
	w := newTestWindow()

	Text(TextCfg{
		ID:   "first",
		Text: "hi",
		Anim: TextAnimCfg{Kind: TextAnimFadeIn},
	}).GenerateLayout(w)

	if !w.HasAnimation(ScopeID("textanim", "first")) {
		t.Error("no animation registered on the first frame")
	}
}

// An animated text with no ID cannot be keyed, so nothing animates.
// The debug gate is what tells the author, rather than silence.
func TestTextAnimNoIDReported(t *testing.T) {
	w := newTestWindow()
	var found []string
	w.debug.collect = &found
	prev := debugMask.Load()
	debugMask.Store(uint32(DebugMissingIDs))
	defer debugMask.Store(prev)

	sh := Text(TextCfg{
		Text: "hi",
		Anim: TextAnimCfg{Kind: TextAnimFadeIn},
	}).GenerateLayout(w).Shape

	if sh.Opacity != 1 {
		t.Errorf("Opacity = %v; an ID-less animation must be inert",
			sh.Opacity)
	}
	if len(found) != 1 {
		t.Fatalf("findings = %v, want one missing-ID report", found)
	}
}

func TestTextAnimCheckCategory(t *testing.T) {
	if got := checkCategory(debugCheckTextAnimNoID); got !=
		DebugMissingIDs {
		t.Errorf("category = %v, want DebugMissingIDs", got)
	}
}

// The transform turns about the text's center, so a pure scale must
// leave the center where it was. A transform anchored at the origin
// would grow the text down and to the right instead.
func TestTextAnimTransformScalesAboutCenter(t *testing.T) {
	const cx, cy = float32(50), float32(10)
	tr := textAnimTransform(2, 0, 0, 0, cx, cy)

	x, y := tr.Apply(cx, cy)
	if !f32AreClose(x, cx) || !f32AreClose(y, cy) {
		t.Errorf("center moved to (%v,%v), want (%v,%v)", x, y, cx, cy)
	}

	// The left edge should move out by half the box, not stay put.
	lx, _ := tr.Apply(0, cy)
	if !f32AreClose(lx, -cx) {
		t.Errorf("left edge at %v, want %v", lx, -cx)
	}
}

func TestTextAnimTransformTranslates(t *testing.T) {
	tr := textAnimTransform(1, 0, 3, -4, 10, 5)
	x, y := tr.Apply(10, 5)
	if !f32AreClose(x, 13) || !f32AreClose(y, 1) {
		t.Errorf("point = (%v,%v), want (13,1)", x, y)
	}
}

// Rotation is clockwise in screen coordinates (y down): a point to
// the right of the center must land below it after a quarter turn.
func TestTextAnimTransformRotatesClockwise(t *testing.T) {
	const cx, cy = float32(50), float32(10)
	tr := textAnimTransform(1, f32Pi/2, 0, 0, cx, cy)

	x, y := tr.Apply(cx+10, cy)
	if !f32AreClose(x, cx) || !f32AreClose(y, cy+10) {
		t.Errorf("point = (%v,%v), want (%v,%v)", x, y, cx, cy+10)
	}
}

// BenchmarkTextAnimFrame measures the per-frame cost of an animated
// text's layout generation. The number that matters is allocations:
// the frame runs at 60Hz for as long as the animation lives, so an
// allocation here is an allocation sixty times a second.
func BenchmarkTextAnimFrame(b *testing.B) {
	kinds := []struct {
		name string
		kind TextAnimKind
	}{
		// The baseline: the same Text with no animation at all. Every
		// other row is only meaningful against this one.
		{"none", TextAnimNone},
		{"fade", TextAnimFadeIn},
		{"slide", TextAnimSlideUp},
		{"typewriter", TextAnimTypewriter},
		{"shimmer", TextAnimShimmer},
	}
	for _, k := range kinds {
		b.Run(k.name, func(b *testing.B) {
			w := newTestWindow()
			seedTextAnim(w, "bench", 0.5)
			cfg := TextCfg{
				ID:   "bench",
				Text: "the quick brown fox",
				Anim: TextAnimCfg{Kind: k.kind, Repeat: true},
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				v := Text(cfg)
				_ = v.GenerateLayout(w)
			}
		})
	}
}

// renderTextAnimCmds drives one animated text through the real frame
// pipeline and returns the commands it emitted.
//
// It installs a stub TextMeasurer, which the golden harness
// deliberately does without. Without a measurer renderText cannot
// build a glyph layout, so it falls back to the plain RenderText path
// and no transform or gradient ever reaches a command — which is
// exactly what these tests are here to check.
func renderTextAnimCmds(
	t *testing.T, id string, progress float32, cfg TextAnimCfg,
) []RenderCmd {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 200, Height: 60})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	w.viewGenerator = func(win *Window) View {
		seedTextAnim(win, id, progress)
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:   id,
				Text: "Animated",
				Anim: cfg,
			})},
		})
	}
	w.refreshLayout = true
	w.FrameFn()
	return w.renderers
}

func findTextCmd(t *testing.T, cmds []RenderCmd) RenderCmd {
	t.Helper()
	for _, c := range cmds {
		switch c.Kind {
		case RenderText, RenderLayout, RenderLayoutTransformed:
			return c
		}
	}
	t.Fatal("no text command emitted")
	return RenderCmd{}
}

// A moving text must reach the backend as a transformed layout draw. A
// plain RenderText here means the offset was computed and then
// silently dropped, which looks like an animation that does not move.
func TestTextAnimEmitsTransformedCommand(t *testing.T) {
	// Mid-slide, not the very start: at progress 0 a slide is fully
	// transparent, and renderText drops a command with no alpha
	// before it ever looks at the transform.
	cmds := renderTextAnimCmds(t, "slide", 0.3,
		TextAnimCfg{Kind: TextAnimSlideUp})
	cmd := findTextCmd(t, cmds)

	if cmd.Kind != RenderLayoutTransformed {
		t.Fatalf("kind = %s, want RenderLayoutTransformed",
			renderKindName(cmd.Kind))
	}
	if cmd.LayoutTransform == nil {
		t.Fatal("transformed command carries no transform")
	}
	if cmd.LayoutTransform.Y0 <= 0 {
		t.Errorf("Y0 = %v, want a downward start offset",
			cmd.LayoutTransform.Y0)
	}
}

// The counterpart: a fade must stay on the cheap path. This is the
// performance property the design turns on, so it is worth asserting
// against the emitted command and not just against the style.
func TestTextAnimFadeStaysOnPlainTextCommand(t *testing.T) {
	cmds := renderTextAnimCmds(t, "fade", 0.5,
		TextAnimCfg{Kind: TextAnimFadeIn})
	cmd := findTextCmd(t, cmds)

	if cmd.Kind != RenderText {
		t.Errorf("kind = %s, want RenderText", renderKindName(cmd.Kind))
	}
	if cmd.Color.A == 0 || cmd.Color.A == 255 {
		t.Errorf("alpha = %d, want a partial fade", cmd.Color.A)
	}
}

func TestTextAnimShimmerReachesCommand(t *testing.T) {
	cmds := renderTextAnimCmds(t, "shimmer", 0.5,
		TextAnimCfg{Kind: TextAnimShimmer, Repeat: true})
	cmd := findTextCmd(t, cmds)

	if cmd.TextGradient == nil {
		t.Fatal("shimmer emitted no gradient")
	}
	if len(cmd.TextGradient.Stops) != 5 {
		t.Errorf("stops = %d, want 5", len(cmd.TextGradient.Stops))
	}
}

// TestTextAnimRejectsNonFiniteFrameValues covers the other half of the
// non-finite guard: a Custom hook that divides by a zero progress hands
// back a NaN opacity or reveal, and f32Clamp passes a NaN through. A
// NaN alpha renders as garbage and a NaN reveal fraction cuts the
// string at an arbitrary point, so both are dropped.
func TestTextAnimRejectsNonFiniteFrameValues(t *testing.T) {
	zero := float32(0)
	nan := zero / zero

	cases := []struct {
		name  string
		frame TextAnimFrame
	}{
		{"opacity", TextAnimFrame{Opacity: SomeF(nan)}},
		{"reveal", TextAnimFrame{Reveal: SomeF(nan)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWindow()
			seedTextAnim(w, "nan", 0.5)

			sh := Text(TextCfg{
				ID:   "nan",
				Text: "hello",
				Anim: TextAnimCfg{
					Custom: func(float32) TextAnimFrame {
						return tc.frame
					},
				},
			}).GenerateLayout(w).Shape

			if !f32IsFinite(sh.Opacity) {
				t.Errorf("Opacity = %v, want a finite alpha", sh.Opacity)
			}
			if sh.TC.Text != "hello" {
				t.Errorf("Text = %q, want the full string", sh.TC.Text)
			}
		})
	}
}
