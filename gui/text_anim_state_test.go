package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// glyphStubMeasurer is stubTextMeasurer with a layout that carries
// glyphs: one per byte, each at its own byte index, in one item. The
// typewriter's paint mask is applied per glyph, so a test of it needs
// glyphs to mask.
type glyphStubMeasurer struct {
	stubTextMeasurer
	layouts int
}

func (m *glyphStubMeasurer) LayoutText(
	text string, style TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	m.layouts++
	l, err := m.stubTextMeasurer.LayoutText(text, style, wrapWidth)
	if err != nil {
		return l, err
	}
	l.Text = text
	l.Glyphs = make([]glyph.Glyph, len(text))
	for i := range l.Glyphs {
		l.Glyphs[i] = glyph.Glyph{
			Index: uint32(i), Codepoint: 1,
			XAdvance: float64(m.charWidth),
		}
	}
	l.Items = []glyph.Item{{
		GlyphCount: len(text), Length: len(text),
		Color: colorToGlyph(style.Color),
	}}
	return l, nil
}

// renderAnimFrame drives one frame of view through the real pipeline
// with a glyph-producing measurer, and returns the window.
func renderAnimFrame(
	t *testing.T, width, height int, m TextMeasurer,
	view func(*Window) View,
) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: width, Height: height})
	w.textMeasurer = m
	w.viewGenerator = view
	w.refreshLayout = true
	w.FrameFn()
	return w
}

// More animated IDs than a capped map holds must not evict the done
// flags of texts that are still on screen. An evicted flag read as a
// new text, and its entrance replayed — then evicted the next one, for
// as long as the list was shown.
func TestTextAnimDoneSurvivesManyIDs(t *testing.T) {
	w := newTestWindow()
	pm := textAnimStates(w)
	fade := TextAnimCfg{Kind: TextAnimFadeIn}
	const n = textAnimPruneAt + 50
	for i := range n {
		pm.Set(ScopeIDN("list", "row", i), textAnimState{
			sig: fade.sig(), progress: 1, done: true, started: true,
			seen: w.viewPass,
		})
	}

	Text(TextCfg{
		ID:   ScopeIDN("list", "row", 0),
		Text: "hi",
		Anim: fade,
	}).GenerateLayout(w)

	if w.HasAnimation(ScopeID("textanim", ScopeIDN("list", "row", 0))) {
		t.Error("the oldest finished entrance played again")
	}
	if pm.Len() != n {
		t.Errorf("entries = %d, want all %d kept", pm.Len(), n)
	}
}

// The map is unbounded, so it must drop the entries of texts that have
// left the tree. It does that when a new ID arrives at a full map.
func TestTextAnimPruneDropsGoneTexts(t *testing.T) {
	w := newTestWindow()
	w.viewPass = 10
	pm := textAnimStates(w)
	for i := range textAnimPruneAt {
		pm.Set(ScopeIDN("list", "gone", i), textAnimState{done: true, seen: 2})
	}
	pm.Set("live", textAnimState{done: true, seen: 9})

	Text(TextCfg{
		ID:   "new",
		Text: "hi",
		Anim: TextAnimCfg{Kind: TextAnimFadeIn},
	}).GenerateLayout(w)

	if pm.Contains(ScopeIDN("list", "gone", 0)) {
		t.Error("an entry for a text that left the tree was kept")
	}
	if !pm.Contains("live") || !pm.Contains("new") {
		t.Errorf("keys = %v, want live and new kept", pm.Keys())
	}
}

// A wrapped typewriter keeps the height of its full text. Sizing the
// box from the revealed prefix grew it a line at a time and pushed
// everything below it down.
func TestTextAnimTypewriterWrapHeightFixed(t *testing.T) {
	heightAt := func(p float32) float32 {
		m := &glyphStubMeasurer{
			stubTextMeasurer: stubTextMeasurer{charWidth: 10, fontHeight: 20},
		}
		w := renderAnimFrame(t, 60, 400, m, func(win *Window) View {
			seedTextAnim(win, "tw", p)
			return Column(ContainerCfg{
				Sizing: FillFill,
				Content: []View{Text(TextCfg{
					ID:   "tw",
					Text: "aaa bbb ccc ddd",
					Mode: TextModeWrap,
					Anim: TextAnimCfg{Kind: TextAnimTypewriter},
				})},
			})
		})
		return mustShape(t, w, "tw").Height
	}
	if early, full := heightAt(0.1), heightAt(1); early != full {
		t.Errorf("height = %v at 10%%, %v at 100%%; want equal", early, full)
	}
}

// The painted layout is the full text's, with the unrevealed glyphs
// marked unknown. glyph skips an unknown glyph but advances past it, so
// the revealed part sits where it will when all of it is shown — the
// alignment and wrapping do not follow the prefix.
func TestTextAnimTypewriterMasksGlyphs(t *testing.T) {
	m := &glyphStubMeasurer{
		stubTextMeasurer: stubTextMeasurer{charWidth: 8, fontHeight: 16},
	}
	w := renderAnimFrame(t, 200, 60, m, func(win *Window) View {
		seedTextAnim(win, "tw", 0.5)
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:        "tw",
				Text:      "abcdefgh",
				TextStyle: TextStyle{Align: TextAlignCenter, Color: White},
				Anim:      TextAnimCfg{Kind: TextAnimTypewriter},
			})},
		})
	})
	cmd := findTextCmd(t, w.renderers)
	if cmd.LayoutPtr == nil {
		t.Fatalf("kind = %s, want a layout draw", renderKindName(cmd.Kind))
	}
	l := cmd.LayoutPtr
	if l.Text != "abcdefgh" {
		t.Errorf("layout text = %q, want the full string", l.Text)
	}
	for i, g := range l.Glyphs {
		hidden := g.Index&glyph.PangoGlyphUnknownFlag != 0
		if want := i >= 4; hidden != want {
			t.Errorf("glyph %d hidden = %v, want %v", i, hidden, want)
		}
	}
	// The cached layout must keep every glyph: the mask is a copy.
	sh := mustShape(t, w, "tw")
	for i, g := range sh.TC.textLayout.Glyphs {
		if g.Index&glyph.PangoGlyphUnknownFlag != 0 {
			t.Fatalf("cached glyph %d was masked", i)
		}
	}
}

// Changing the animation on the same ID starts the new one. A finished
// entrance used to block every later animation, and a running loop
// kept running after the Cfg asked for something else.
func TestTextAnimCfgChangeRestarts(t *testing.T) {
	w := newTestWindow()
	fade := TextAnimCfg{Kind: TextAnimFadeIn}
	textAnimStates(w).Set("s", textAnimState{
		sig: fade.sig(), started: true, done: true, progress: 1,
	})
	animID := ScopeID("textanim", "s")

	pulse := TextAnimCfg{Kind: TextAnimPulse, Repeat: true}
	Text(TextCfg{ID: "s", Text: "hi", Anim: pulse}).GenerateLayout(w)
	if !w.HasAnimation(animID) {
		t.Fatal("a pulse after a finished fade never registered")
	}
	st, _ := textAnimStates(w).Get("s")
	if st.done || st.gen != 1 {
		t.Errorf("state = %+v, want a fresh run at gen 1", st)
	}

	Text(TextCfg{ID: "s", Text: "hi",
		Anim: TextAnimCfg{Kind: TextAnimFadeOut}}).GenerateLayout(w)
	w.animMu.Lock()
	a, ok := w.animations[animID].(*KeyframeAnimation)
	w.animMu.Unlock()
	if !ok || a.Repeat {
		t.Errorf("driver = %+v, want the one-shot fade out", a)
	}
}

// A callback from a replaced driver must not write into the new run.
func TestTextAnimStaleDriverIgnored(t *testing.T) {
	w := newTestWindow()
	textAnimStates(w).Set("k", textAnimState{gen: 2, progress: 0.1})

	old := newTextAnimDriver("id", "k", 1, 100, 0, false)
	old.OnValue(0.9, w)
	old.OnDone(w)

	st, _ := textAnimStates(w).Get("k")
	if st.progress != 0.1 || st.done {
		t.Errorf("state = %+v, want the gen-2 run untouched", st)
	}
}

// Text that grows by appending keeps typing from where it had got to.
// Restarting from zero retyped the whole reply on every chunk of a
// stream, and a finished typewriter showed new text at once.
func TestTextAnimTypewriterContinuesOnAppend(t *testing.T) {
	w := newTestWindow()
	cfg := TextAnimCfg{Kind: TextAnimTypewriter}
	textAnimStates(w).Set("tw", textAnimState{
		sig: cfg.sig(), text: "hello", started: true, done: true,
		progress: 1,
	})

	sh := Text(TextCfg{ID: "tw", Text: "hello world", Anim: cfg}).
		GenerateLayout(w).Shape

	st, _ := textAnimStates(w).Get("tw")
	if st.from != 5 || st.done {
		t.Fatalf("state = %+v, want a run from 5 characters", st)
	}
	if !w.HasAnimation(ScopeID("textanim", "tw")) {
		t.Error("appended text registered no driver")
	}
	if a := sh.TC.anim; a == nil || a.revealEnd != 5 {
		t.Errorf("anim = %+v, want the old text still shown", a)
	}

	// Text that is not an extension starts over.
	Text(TextCfg{ID: "tw", Text: "bye", Anim: cfg}).GenerateLayout(w)
	if st, _ = textAnimStates(w).Get("tw"); st.from != 0 {
		t.Errorf("from = %d after a replacement, want 0", st.from)
	}
}

// The loop deletes a finished one-shot at once, but its OnDone lands
// only at the next flush. A view pass in between must read the missing
// driver as finished, not register it again.
func TestTextAnimFinishedBeforeOnDoneFlush(t *testing.T) {
	w := newTestWindow()
	cfg := TextAnimCfg{Kind: TextAnimFadeIn}
	textAnimStates(w).Set("f", textAnimState{
		sig: cfg.sig(), started: true, progress: 0.97,
	})

	sh := Text(TextCfg{ID: "f", Text: "hi", Anim: cfg}).
		GenerateLayout(w).Shape

	if w.HasAnimation(ScopeID("textanim", "f")) {
		t.Error("a finished entrance registered again")
	}
	if sh.Opacity != 1 {
		t.Errorf("Opacity = %v, want the settled 1", sh.Opacity)
	}
}

// Scale and rotation turn about the arranged box, not about the size
// the text measured before sizing.
func TestTextAnimPivotIsArrangedCenter(t *testing.T) {
	a := &textAnimRender{scale: 0.5, xformOn: true}
	sh := &Shape{Width: 270, Height: 60}
	tr, ok := a.transform(sh)
	if !ok {
		t.Fatal("no transform")
	}
	if x, y := tr.Apply(135, 30); !f32AreClose(x, 135) || !f32AreClose(y, 30) {
		t.Errorf("center moved to (%v,%v), want (135,30)", x, y)
	}
}

// A wrapped paragraph that pops must keep its center through the whole
// frame pipeline: the transform is built after arrange.
func TestTextAnimPopPivotAfterArrange(t *testing.T) {
	m := &glyphStubMeasurer{
		stubTextMeasurer: stubTextMeasurer{charWidth: 10, fontHeight: 20},
	}
	// A 60px window wraps the text to four lines: the arranged box is
	// far narrower and taller than the one line it measured first.
	w := renderAnimFrame(t, 60, 400, m, func(win *Window) View {
		seedTextAnim(win, "pop", 0.5)
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:   "pop",
				Text: "aaa bbb ccc ddd eee",
				Mode: TextModeWrap,
				Anim: TextAnimCfg{Kind: TextAnimPop},
			})},
		})
	})
	cmd := findTextCmd(t, w.renderers)
	if cmd.LayoutTransform == nil {
		t.Fatal("pop emitted no transform")
	}
	sh := mustShape(t, w, "pop")
	cx, cy := sh.Width/2, sh.Height/2
	if x, y := cmd.LayoutTransform.Apply(cx, cy); !f32AreClose(x, cx) ||
		!f32AreClose(y, cy) {
		t.Errorf("center (%v,%v) moved to (%v,%v)", cx, cy, x, y)
	}
}

// A one-shot shimmer is a flash: once it finishes, the text goes back
// to its own colour. It used to stay dimmed for good.
func TestTextAnimOneShotShimmerClears(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "sh", 1)

	sh := Text(TextCfg{
		ID:   "sh",
		Text: "saved",
		Anim: TextAnimCfg{Kind: TextAnimShimmer},
	}).GenerateLayout(w).Shape

	if sh.TC.anim.gradient(sh.TC.TextStyle.Color, w) != nil {
		t.Error("a finished one-shot shimmer still paints a gradient")
	}
}

// The shimmer's band follows TextAnimCfg.Easing, as documented.
func TestTextAnimShimmerUsesEasing(t *testing.T) {
	w := newTestWindow()
	seedTextAnim(w, "sh", 0.8)

	sh := Text(TextCfg{
		ID:   "sh",
		Text: "loading",
		Anim: TextAnimCfg{
			Kind: TextAnimShimmer, Repeat: true,
			Easing: func(float32) float32 { return 0 },
		},
	}).GenerateLayout(w).Shape

	got := sh.TC.anim.gradient(sh.TC.TextStyle.Color, w).Stops[2].Position
	var want textAnimShimmer
	if w := textAnimShimmerGradient(&want, Color{}, 0).Stops[2].Position; got != w {
		t.Errorf("band at %v, want the eased 0 position %v", got, w)
	}
}

// A shimmer inside a filled button takes the label colour the button
// stamps after arrange. Baked in the view pass, it kept the theme's
// text colour: dark text on the accent fill.
func TestTextAnimShimmerFollowsButtonLabelColor(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Button(ButtonCfg{
				ID: "pri", Variant: ButtonPrimary, OnClick: noop,
				Content: []View{Text(TextCfg{
					ID:   "lbl",
					Text: "Saving",
					Anim: TextAnimCfg{Kind: TextAnimShimmer, Repeat: true},
				})},
			})},
		})
	})
	sh := mustShape(t, w, ScopeID("pri", "lbl"))
	cmd := findTextCmd(t, w.renderers)
	if cmd.TextGradient == nil {
		t.Fatal("no shimmer gradient")
	}
	got := cmd.TextGradient.Stops[2].Color
	if want := colorToGlyph(sh.TC.TextStyle.Color); got != want {
		t.Errorf("highlight = %v, want the stamped label colour %v", got, want)
	}
}

// Motion composes with the style's own rotation. Replacing it drew the
// label unrotated on every frame the animation moved it.
func TestTextAnimKeepsCallerRotation(t *testing.T) {
	m := &glyphStubMeasurer{
		stubTextMeasurer: stubTextMeasurer{charWidth: 8, fontHeight: 16},
	}
	w := renderAnimFrame(t, 200, 200, m, func(win *Window) View {
		seedTextAnim(win, "rot", 0.1)
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:        "rot",
				Text:      "turned",
				TextStyle: TextStyle{RotationRadians: f32Pi / 2, Color: White},
				Anim:      TextAnimCfg{Kind: TextAnimShake, Repeat: true},
			})},
		})
	})
	cmd := findTextCmd(t, w.renderers)
	tr := cmd.LayoutTransform
	if tr == nil {
		t.Fatal("no transform")
	}
	// A quarter turn maps the x axis onto y: XX near 0, YX near 1.
	if !f32AreClose(tr.XX, 0) || !f32AreClose(tr.YX, 1) {
		t.Errorf("transform = %+v, want the caller's quarter turn kept", *tr)
	}
}

// A finished entrance allocates nothing more per frame than the same
// text with no animation: no animation ID, no lock, no driver lookup.
func TestTextAnimFinishedEntranceAllocs(t *testing.T) {
	w := newTestWindow()
	// Inside a view pass, as in a real frame: EffID outside one builds
	// a debug report, which would be counted against the animation.
	w.viewState.genDepth = 1
	cfg := TextAnimCfg{Kind: TextAnimFadeIn}
	textAnimStates(w).Set("done", textAnimState{
		sig: cfg.sig(), started: true, done: true, progress: 1,
	})
	plain := TextCfg{ID: "done", Text: "hi"}
	animated := TextCfg{ID: "done", Text: "hi", Anim: cfg}

	base := testing.AllocsPerRun(50, func() {
		_ = Text(plain).GenerateLayout(w)
	})
	got := testing.AllocsPerRun(50, func() {
		_ = Text(animated).GenerateLayout(w)
	})
	if got > base {
		t.Errorf("allocs = %v, want no more than the plain text's %v",
			got, base)
	}
}

// An animated text with no ID allocates nothing for its debug report
// while the check is off.
func TestTextAnimNoIDNoAllocWhenDebugOff(t *testing.T) {
	prev := debugMask.Load()
	debugMask.Store(0)
	defer debugMask.Store(prev)

	w := newTestWindow()
	plain := TextCfg{Text: "hi"}
	animated := TextCfg{Text: "hi",
		Anim: TextAnimCfg{Kind: TextAnimFadeIn}}
	base := testing.AllocsPerRun(50, func() {
		_ = Text(plain).GenerateLayout(w)
	})
	got := testing.AllocsPerRun(50, func() {
		_ = Text(animated).GenerateLayout(w)
	})
	if got > base {
		t.Errorf("allocs = %v, want no more than the plain text's %v",
			got, base)
	}
}
