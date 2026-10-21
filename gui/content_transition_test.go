package gui

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// newContentTransitionWindow lays out a Text showing from, then the
// same Text showing to, which starts the transition. The animation
// goroutine is stopped after that, so the test sets progress by hand:
// a live tick would write wall-clock values between frames.
//
// The stub measurer gives every byte 8px and every line 16px, so a
// column sits at 8px per grapheme and a roll moves 16px.
func newContentTransitionWindow(
	t *testing.T, tr ContentTransition, from, to string,
	np NativePlatform,
) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{State: new(int), Width: 200, Height: 60})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	if np != nil {
		w.nativePlatform = np
	}
	text := from
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:         "n",
				Text:       text,
				Transition: tr,
			})},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	text = to
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.stopAnimationLoop()
	return w
}

// contentTransitionDriver returns the window's transition driver, or
// nil when none was started.
func contentTransitionDriver(w *Window) *KeyframeAnimation {
	for _, kf := range renderOnlyAnimKeyframes(w) {
		if strings.HasPrefix(kf.AnimID, "ctrans") {
			return kf
		}
	}
	return nil
}

// renderContentTransitionAt delivers progress v the way a tick does and
// draws a render-only frame.
func renderContentTransitionAt(t *testing.T, w *Window, v float32) []RenderCmd {
	t.Helper()
	kf := contentTransitionDriver(w)
	if kf == nil {
		t.Fatal("the text change started no transition")
	}
	kf.OnValue(v, w)
	w.renderOnlyLocked()
	return w.renderers
}

// textCmdsByText indexes the plain text draws of a frame by their text.
// Each text appears once in these cases.
func textCmdsByText(cmds []RenderCmd) map[string]RenderCmd {
	out := map[string]RenderCmd{}
	for _, c := range cmds {
		if c.Kind == RenderText {
			out[c.Text] = c
		}
	}
	return out
}

func TestContentTransitionFirstFrameStartsNothing(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "41", "41", nil)
	if kf := contentTransitionDriver(w); kf != nil {
		t.Fatalf("unchanged text started driver %s", kf.AnimID)
	}
}

func TestContentTransitionTickIsRenderOnly(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "41", "42", nil)
	kf := contentTransitionDriver(w)
	if kf == nil {
		t.Fatal("no driver")
	}
	if got := kf.RefreshKind(); got != AnimationRefreshRenderOnly {
		t.Errorf("RefreshKind = %d, want render-only", got)
	}
}

// "41" → "42": the "4" stays put in every frame, the "1" rolls up and
// out, the "2" rolls up and in.
func TestContentTransitionNumericRollsChangedDigitOnly(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "41", "42", nil)

	var baseX, baseY float32
	for i, v := range []float32{0, 0.5, 1} {
		got := textCmdsByText(renderContentTransitionAt(t, w, v))
		four, ok := got["4"]
		if !ok {
			t.Fatalf("p=%v: unchanged \"4\" not drawn on its own", v)
		}
		if i == 0 {
			baseX, baseY = four.X, four.Y
		} else if four.X != baseX || four.Y != baseY {
			t.Errorf("p=%v: \"4\" moved to (%v,%v), want (%v,%v)",
				v, four.X, four.Y, baseX, baseY)
		}
		p := EaseOutCubic(v)
		if old, ok := got["1"]; ok {
			if want := baseY - p*16; !f32AreClose(old.Y, want) {
				t.Errorf("p=%v: old \"1\" Y = %v, want %v", v, old.Y, want)
			}
			if old.X != baseX+8 {
				t.Errorf("p=%v: old \"1\" X = %v, want %v", v, old.X, baseX+8)
			}
		} else if v < 1 {
			t.Errorf("p=%v: old \"1\" not drawn", v)
		}
		if nw, ok := got["2"]; ok {
			if want := baseY + (1-p)*16; !f32AreClose(nw.Y, want) {
				t.Errorf("p=%v: new \"2\" Y = %v, want %v", v, nw.Y, want)
			}
		} else if v > 0 {
			t.Errorf("p=%v: new \"2\" not drawn", v)
		}
	}
}

// The roll is clipped to the line box and the parent clip comes back
// after it, so a digit is never seen outside its line.
func TestContentTransitionNumericClipsToLineBox(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "41", "42", nil)
	cmds := renderContentTransitionAt(t, w, 0.5)
	first, last := -1, -1
	for i, c := range cmds {
		if c.Kind == RenderText {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 1 || cmds[first-1].Kind != RenderClip {
		t.Fatal("roll is not preceded by a clip")
	}
	box := cmds[first-1]
	if box.H != 16 || box.W != 16 {
		t.Errorf("clip = %vx%v, want the 16x16 line box", box.W, box.H)
	}
	if last+1 >= len(cmds) || cmds[last+1].Kind != RenderClip {
		t.Fatal("parent clip not restored after the roll")
	}
	if cmds[last+1].H <= box.H {
		t.Errorf("restored clip height %v, want the parent's", cmds[last+1].H)
	}
}

// Down forces the roll down even when the value grows.
func TestContentTransitionNumericDown(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric, Down: true},
		"41", "42", nil)
	got := textCmdsByText(renderContentTransitionAt(t, w, 0.5))
	base := got["4"].Y
	if got["1"].Y <= base {
		t.Errorf("old digit Y = %v, want below %v (leaving down)", got["1"].Y, base)
	}
	if got["2"].Y >= base {
		t.Errorf("new digit Y = %v, want above %v (arriving from above)",
			got["2"].Y, base)
	}
}

// A smaller value rolls down without Down.
func TestContentTransitionNumericSmallerRollsDown(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "42", "41", nil)
	got := textCmdsByText(renderContentTransitionAt(t, w, 0.5))
	if base := got["4"].Y; got["1"].Y >= base {
		t.Errorf("new digit Y = %v, want above %v", got["1"].Y, base)
	}
}

// "99" → "100": three columns roll. The new "1" has no old digit; each
// "9" rolls into a "0" in the same column, aligned from the right.
func TestContentTransitionNumericAlignsFromRight(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "99", "100", nil)
	cmds := renderContentTransitionAt(t, w, 0.5)
	var nines, zeros []RenderCmd
	var one *RenderCmd
	for i, c := range cmds {
		switch {
		case c.Kind != RenderText:
		case c.Text == "9":
			nines = append(nines, c)
		case c.Text == "0":
			zeros = append(zeros, c)
		case c.Text == "1":
			one = &cmds[i]
		}
	}
	if len(nines) != 2 || len(zeros) != 2 || one == nil {
		t.Fatalf("got %d nines, %d zeros, one=%v; want 2, 2, true",
			len(nines), len(zeros), one != nil)
	}
	for i := range 2 {
		if nines[i].X != zeros[i].X {
			t.Errorf("column %d: old X %v, new X %v; want one column",
				i, nines[i].X, zeros[i].X)
		}
		if nines[i].Y == zeros[i].Y {
			t.Errorf("column %d does not roll", i)
		}
	}
	if one.X != zeros[0].X-8 {
		t.Errorf("new \"1\" X = %v, want %v", one.X, zeros[0].X-8)
	}
	if one.Y == nines[0].Y {
		t.Error("new \"1\" does not roll in")
	}
}

// Reduced motion fades: the whole old and new texts, at one Y. A
// numeric text asks for tabular figures, so like its plain draw, its
// fade draws through glyph layouts.
func TestContentTransitionReducedMotionFades(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "41", "42",
		reducedMotionPlatform{pref: true})
	got := map[string]RenderCmd{}
	for _, c := range renderContentTransitionAt(t, w, 0.5) {
		if c.Kind == RenderLayout {
			got[c.Text] = c
		}
	}
	old, okOld := got["41"]
	nw, okNew := got["42"]
	if !okOld || !okNew {
		t.Fatalf("want whole old and new texts, got %v", got)
	}
	if old.Y != nw.Y || old.X != nw.X {
		t.Errorf("old at (%v,%v), new at (%v,%v); want no movement",
			old.X, old.Y, nw.X, nw.Y)
	}
	oldA, newA := old.TextStylePtr.Color.A, nw.TextStylePtr.Color.A
	if oldA == 0 || oldA == 255 || newA == 0 || newA == 255 {
		t.Errorf("alphas %d, %d; want both partway", oldA, newA)
	}
}

func TestContentTransitionFade(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionFade}, "Saved", "Saving", nil)
	got := textCmdsByText(renderContentTransitionAt(t, w, 0.5))
	old, nw := got["Saved"], got["Saving"]
	if old.Color.A == 0 || nw.Color.A == 0 {
		t.Fatalf("want both texts drawn, got %v", got)
	}
	if old.Color.A >= nw.Color.A {
		t.Errorf("past halfway, old alpha %d should be below new %d",
			old.Color.A, nw.Color.A)
	}
}

// Replace scales both texts about their centers: the old one shrinks,
// the new one grows.
func TestContentTransitionReplaceScales(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionReplace}, "A", "B", nil)
	cmds := renderContentTransitionAt(t, w, 0.5)
	var scales []float32
	for _, c := range cmds {
		if c.Kind == RenderLayoutTransformed && c.LayoutTransform != nil {
			scales = append(scales, c.LayoutTransform.XX)
		}
	}
	if len(scales) != 2 {
		t.Fatalf("got %d transformed draws, want 2", len(scales))
	}
	p := EaseOutCubic(0.5)
	if !f32AreClose(scales[0], 1-p) || !f32AreClose(scales[1], p) {
		t.Errorf("scales = %v, want [%v %v]", scales, 1-p, p)
	}
}

// A render-only frame draws what a full frame draws at the same
// progress. Otherwise the picture is stale until something else lays
// the window out.
func TestContentTransitionRenderOnlyMatchesFullFrame(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "41", "42", nil)
	before := serializeCmds(renderContentTransitionAt(t, w, 0.2))
	got := serializeCmds(renderContentTransitionAt(t, w, 0.6))
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	want := serializeCmds(w.renderers)
	if got == before {
		t.Fatal("progress does not change the picture; the case checks nothing")
	}
	if got != want {
		t.Errorf("render-only frame differs from the full frame:\n%s",
			goldenDiff(want, got))
	}
}

// Once the driver finishes, the text draws as one plain command again,
// before any view pass drops the run.
func TestContentTransitionDoneDrawsPlainText(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "41", "42", nil)
	kf := contentTransitionDriver(w)
	kf.OnValue(1, w)
	kf.OnDone(w)
	w.renderOnlyLocked()
	// Tabular figures set Features, which sends the plain text down
	// the glyph-layout path, so any text command kind counts.
	var texts []string
	for _, c := range w.renderers {
		switch c.Kind {
		case RenderText, RenderLayout, RenderLayoutTransformed:
			texts = append(texts, c.Text)
		}
	}
	if len(texts) != 1 || texts[0] != "42" {
		t.Errorf("want only \"42\", got %q", texts)
	}
}

func TestContentTransitionNumericUsesTabularFigures(t *testing.T) {
	v := Text(TextCfg{Text: "1", Transition: ContentTransition{
		Kind: ContentTransitionNumeric,
	}})
	if f := v.(*textView).cfg.TextStyle.Features; f != contentTransitionTnum {
		t.Errorf("Features = %v, want tabular figures", f)
	}
	own := TextStyle{Size: 12, Features: &glyph.FontFeatures{}}
	v = Text(TextCfg{Text: "1", TextStyle: own, Transition: ContentTransition{
		Kind: ContentTransitionNumeric,
	}})
	if f := v.(*textView).cfg.TextStyle.Features; f != own.Features {
		t.Error("the caller's own Features were replaced")
	}
}

func TestContentTransitionNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"42", 42, true},
		{"$1,204.50", 1204.5, true},
		{"-3", -3, true},
		{"12:05", 1205, true},
		{"abc", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := contentTransitionNumber(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%q: got %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
	if !contentTransitionLess("9", "10") || contentTransitionLess("10", "9") {
		t.Error("contentTransitionLess compares as text, not as numbers")
	}
	if contentTransitionLess("b", "a") {
		t.Error("non-numbers must not read as smaller")
	}
}

func TestContentTransitionNoIDReported(t *testing.T) {
	w := newTestWindow()
	var found []string
	w.debug.collect = &found
	prev := debugMask.Load()
	debugMask.Store(uint32(DebugMissingIDs))
	defer debugMask.Store(prev)

	_ = Text(TextCfg{Text: "42", Transition: ContentTransition{
		Kind: ContentTransitionNumeric,
	}}).GenerateLayout(w)

	if len(found) != 1 || !strings.Contains(found[0], "content transition") {
		t.Fatalf("findings = %v, want one missing-ID report", found)
	}
}

// seedContentTransition returns a Text whose transition from from to
// cfg.Text is pinned at progress v, for a golden recording. The run is
// seeded as if the text had changed on the last view pass, and no
// driver is started, so the frame is deterministic.
func seedContentTransition(w *Window, from string, cfg TextCfg, v float32) View {
	view := Text(cfg)
	tv := view.(*textView)
	run := newContentTransitionRun(from, &tv.cfg, w)
	run.progress = v
	contentTransitionStates(w).Set(cfg.ID, contentTransitionState{
		text: cfg.Text, slot: &contentTransitionSlot{run: run},
		seen: w.viewPass,
	})
	return view
}

// A wrapped text fades, and both of its draws wrap at the box width,
// as the plain path does. Without W the backend draws one long line.
// No text measurer: with one, both paths draw glyph layouts, which
// carry their own wrap.
func TestContentTransitionWrappedFadeKeepsWrapWidth(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{State: new(int), Width: 200, Height: 60})
	text := "one two"
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:   "wrap",
				Text: text,
				Mode: TextModeWrap,
				Transition: ContentTransition{
					Kind: ContentTransitionNumeric,
				},
			})},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	text = "three four"
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.stopAnimationLoop()
	got := textCmdsByText(renderContentTransitionAt(t, w, 0.5))
	old, nw := got["one two"], got["three four"]
	if old.W <= 0 || nw.W <= 0 || old.W != nw.W {
		t.Errorf("wrap widths %v, %v; want the box width on both", old.W, nw.W)
	}
}

// "100" → "99": the old text is longer, so its leading "1" has no new
// column. It rolls out to the left of the new text, past the clip.
func TestContentTransitionNumericShorterNewText(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "100", "99", nil)
	cmds := renderContentTransitionAt(t, w, 0.5)
	var one *RenderCmd
	var nines []RenderCmd
	for i, c := range cmds {
		switch {
		case c.Kind != RenderText:
		case c.Text == "1":
			one = &cmds[i]
		case c.Text == "9":
			nines = append(nines, c)
		}
	}
	if one == nil || len(nines) != 2 {
		t.Fatalf("one=%v nines=%d, want the old 1 and two new 9s",
			one != nil, len(nines))
	}
	if one.X != nines[0].X-8 {
		t.Errorf("old \"1\" X = %v, want %v", one.X, nines[0].X-8)
	}
	if one.Y <= nines[0].Y {
		t.Errorf("smaller value: old \"1\" Y %v should leave downward", one.Y)
	}
}

// A change to "" starts no transition. renderText draws nothing for an
// empty text, so a run would animate nothing for its whole length.
func TestContentTransitionToEmptyStartsNothing(t *testing.T) {
	for _, k := range []ContentTransitionKind{
		ContentTransitionNumeric, ContentTransitionFade,
		ContentTransitionReplace,
	} {
		w := newContentTransitionWindow(t,
			ContentTransition{Kind: k}, "41", "", nil)
		if kf := contentTransitionDriver(w); kf != nil {
			t.Errorf("kind %d: change to \"\" started %s", k, kf.AnimID)
		}
	}
}

// From "" the new text rolls in with no old column to leave. The plan
// must not index an old grapheme that is not there.
func TestContentTransitionNumericFromEmpty(t *testing.T) {
	w := newContentTransitionWindow(t,
		ContentTransition{Kind: ContentTransitionNumeric}, "", "42", nil)
	got := textCmdsByText(renderContentTransitionAt(t, w, 0.5))
	if _, ok := got["4"]; !ok {
		t.Fatalf("new digits not drawn: %v", got)
	}
}

// The column plan with an empty new text, called directly: the view
// pass no longer starts a run for "", but the planner must not panic
// on it either.
func TestContentTransitionColumnsEmptyNewText(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{State: new(int)})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	cols := contentTransitionColumns("41", "", TextStyle{Size: 14}, w)
	if len(cols) != 2 || cols[0].x != -16 || cols[1].x != -8 {
		t.Errorf("cols = %+v, want two old columns at -16 and -8", cols)
	}
}

// While a transition runs it draws the text: Anim's shimmer waits.
// This pins the documented behaviour, so a change to it is a choice.
func TestContentTransitionPausesShimmer(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{State: new(int), Width: 200, Height: 60})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	text := "41"
	w.viewGenerator = func(win *Window) View {
		seedTextAnim(win, "n", 0.5)
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:   "n",
				Text: text,
				Anim: TextAnimCfg{Kind: TextAnimShimmer, Repeat: true},
				Transition: ContentTransition{
					Kind: ContentTransitionNumeric,
				},
			})},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	text = "42"
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.stopAnimationLoop()
	for _, c := range renderContentTransitionAt(t, w, 0.5) {
		if c.Kind == RenderText && c.TextGradient != nil {
			t.Fatalf("%q carries a shimmer gradient mid-transition", c.Text)
		}
	}
}

// A password never paints in clear. The run draws its texts as given,
// so a password must start no run at all.
func TestContentTransitionPasswordNeverDrawsClearText(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{State: new(int), Width: 200, Height: 60})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	text := "hunter1"
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:         "pw",
				Text:       text,
				IsPassword: true,
				Transition: ContentTransition{Kind: ContentTransitionFade},
			})},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	text = "hunter2"
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.stopAnimationLoop()
	if kf := contentTransitionDriver(w); kf != nil {
		t.Fatalf("password change started %s", kf.AnimID)
	}
	for _, c := range w.renderers {
		if strings.Contains(c.Text, "hunter") {
			t.Fatalf("clear password painted: %q", c.Text)
		}
	}
}

// kernTextMeasurer shapes like stubTextMeasurer, but returns real
// glyphs for a single line, and kerns every "1" that follows a "1" by
// kern px, the way a display face kerns the "11" pair.
type kernTextMeasurer struct {
	stubTextMeasurer
	kern float64
}

func (m *kernTextMeasurer) LayoutText(
	text string, _ TextStyle, _ float32,
) (glyph.Layout, error) {
	gs := make([]glyph.Glyph, len(text))
	for i := range text {
		gs[i] = glyph.Glyph{
			Index: uint32(i), Codepoint: 1, XAdvance: float64(m.charWidth),
		}
		if i > 0 && text[i] == '1' && text[i-1] == '1' {
			gs[i].XOffset = m.kern
		}
	}
	return glyph.Layout{
		Text:   text,
		Glyphs: gs,
		Items:  []glyph.Item{{GlyphStart: 0, GlyphCount: len(gs)}},
		Width:  float32(len(text)) * m.charWidth,
		Height: m.fontHeight,
	}, nil
}

// "110" → "111": the columns are placed where the shaped text puts its
// glyphs, kerning included. Placed by measured prefix width instead,
// the middle "1" sat 1px right during the roll and jumped left when
// the shaped text took over at the end.
func TestContentTransitionNumericColumnsFollowKerning(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{State: new(int)})
	w.textMeasurer = &kernTextMeasurer{
		stubTextMeasurer: stubTextMeasurer{charWidth: 8, fontHeight: 16},
		kern:             -1,
	}
	cols := contentTransitionColumns("110", "111", TextStyle{Size: 14}, w)
	want := []float32{0, 7, 15}
	for i, c := range cols {
		if c.x != want[i] {
			t.Errorf("column %d x = %v, want %v", i, c.x, want[i])
		}
	}
}

// A text whose transition was off for a pass has no old value when it
// comes back on: the last text the state saw is out of date, and a run
// from it would draw a value the text has not shown for a while. The
// same holds for a password field made visible: no run may start from
// a text recorded before it.
func TestContentTransitionStaleEntryStartsNothing(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{State: new(int), Width: 200, Height: 60})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	text, kind, pw := "41", ContentTransitionNumeric, false
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:         "n",
				Text:       text,
				IsPassword: pw,
				Transition: ContentTransition{Kind: kind},
			})},
		})
	}
	frame := func() {
		w.markLayoutRefresh(refreshTest)
		w.FrameFn()
	}
	frame()
	text, kind = "50", ContentTransitionNone
	frame()
	text, kind = "51", ContentTransitionNumeric
	frame()
	if kf := contentTransitionDriver(w); kf != nil {
		t.Fatalf("re-enabled transition started %s from a stale text", kf.AnimID)
	}
	text, pw = "secret", true
	frame()
	text, pw = "visible", false
	frame()
	w.stopAnimationLoop()
	if kf := contentTransitionDriver(w); kf != nil {
		t.Fatalf("unmasked password started %s from a stale text", kf.AnimID)
	}
}

// An ID-less text with both Anim and Transition reports both: each is
// its own missing-ID finding, not one swallowed by the other's
// warn-once key.
func TestContentTransitionNoIDReportedBesideAnim(t *testing.T) {
	w := newTestWindow()
	var found []string
	w.debug.collect = &found
	prev := debugMask.Load()
	debugMask.Store(uint32(DebugMissingIDs))
	defer debugMask.Store(prev)

	_ = Text(TextCfg{
		Text: "42",
		Anim: TextAnimCfg{Kind: TextAnimFadeIn},
		Transition: ContentTransition{
			Kind: ContentTransitionNumeric,
		},
	}).GenerateLayout(w)

	if len(found) != 2 {
		t.Fatalf("findings = %v, want the animation and the transition", found)
	}
}

// A focused text draws its caret and its selection, which the
// transition path does not. So a focused text draws plain: the new text
// at once, with no old text going out.
func TestContentTransitionFocusedDrawsPlain(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{State: new(int), Width: 200, Height: 60})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	text := "41"
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Text(TextCfg{
				ID:         "n",
				Text:       text,
				Focusable:  true,
				Transition: ContentTransition{Kind: ContentTransitionFade},
			})},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.SetFocus("n")
	text = "42"
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.stopAnimationLoop()
	for _, c := range renderContentTransitionAt(t, w, 0.5) {
		if c.Text == "41" {
			t.Fatalf("focused text drew the old text: %+v", c)
		}
	}
}

// The shape is laid out for the new text, and a centered parent moves
// a shape whose width changed. The old text still fades out where it
// was drawn, not where the new box would align it.
func TestContentTransitionFadeKeepsOldTextInPlace(t *testing.T) {
	for _, kind := range []ContentTransitionKind{
		ContentTransitionFade, ContentTransitionReplace,
	} {
		w := NewTestWindow(t, WindowCfg{State: new(int), Width: 200, Height: 60})
		w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
		text := "Three digits"
		w.viewGenerator = func(*Window) View {
			return Column(ContainerCfg{
				Sizing: FillFill,
				HAlign: HAlignCenter,
				Content: []View{Text(TextCfg{
					ID:         "n",
					Text:       text,
					Transition: ContentTransition{Kind: kind},
				})},
			})
		}
		w.markLayoutRefresh(refreshTest)
		w.FrameFn()
		var oldX float32 = -1
		for _, c := range w.renderers {
			if c.Text == "Three digits" {
				oldX = c.X
			}
		}
		if oldX < 0 {
			t.Fatal("first frame drew no text")
		}
		text = "Counting"
		w.markLayoutRefresh(refreshTest)
		w.FrameFn()
		w.stopAnimationLoop()
		var got float32 = -1
		for _, c := range renderContentTransitionAt(t, w, 0.5) {
			if c.Text == "Three digits" {
				got = c.X
			}
		}
		if got < 0 {
			t.Fatalf("kind %d: old text not drawn", kind)
		}
		// Replace scales about the text's center, so its X is the
		// unscaled left edge, the same as the fade's.
		if !f32AreClose(got, oldX) {
			t.Errorf("kind %d: old text at x=%v, was drawn at %v", kind, got, oldX)
		}
	}
}

// runContentTransitionCfg lays out the Text that mk builds for from,
// then for to, with the parent aligning its child to halign. It
// returns the window with the animation goroutine stopped, and the
// commands of the first frame.
func runContentTransitionCfg(
	t *testing.T, halign HorizontalAlign, from, to string,
	mk func(text string) TextCfg,
) (*Window, []RenderCmd) {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{State: new(int), Width: 200, Height: 80})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	text := from
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			HAlign:  halign,
			Content: []View{Text(mk(text))},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	first := append([]RenderCmd(nil), w.renderers...)
	text = to
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.stopAnimationLoop()
	return w, first
}

// A multiline text with a non-left alignment draws through a glyph
// layout, which aligns each line. Its fade must draw the same way, or
// its lines sit in other places for the fade and then jump.
func TestContentTransitionMultilineFadeDrawsLayouts(t *testing.T) {
	centered := DefaultTextStyle
	centered.Align = TextAlignCenter
	mk := func(text string) TextCfg {
		return TextCfg{
			ID:         "n",
			Text:       text,
			Mode:       TextModeMultiline,
			TextStyle:  centered,
			Transition: ContentTransition{Kind: ContentTransitionFade},
		}
	}
	w, _ := runContentTransitionCfg(t, HAlignLeft, "a\nbbb", "cc\nd", mk)
	var layouts int
	for _, c := range renderContentTransitionAt(t, w, 0.5) {
		if c.Kind == RenderText && (c.Text == "a\nbbb" || c.Text == "cc\nd") {
			t.Errorf("multiline fade drew %q as plain text", c.Text)
		}
		if c.Kind == RenderLayout {
			layouts++
		}
	}
	if layouts != 2 {
		t.Errorf("got %d layout draws, want 2", layouts)
	}
}

// A text with a gradient fill keeps the gradient while it moves, in
// every kind.
func TestContentTransitionKeepsGradient(t *testing.T) {
	grad := &glyph.GradientConfig{Stops: []glyph.GradientStop{
		{Color: glyph.Color{R: 255, A: 255}},
		{Position: 1, Color: glyph.Color{B: 255, A: 255}},
	}}
	style := DefaultTextStyle
	style.Gradient = grad
	for _, kind := range []ContentTransitionKind{
		ContentTransitionFade, ContentTransitionNumeric,
		ContentTransitionReplace,
	} {
		mk := func(text string) TextCfg {
			return TextCfg{
				ID:         "n",
				Text:       text,
				TextStyle:  style,
				Transition: ContentTransition{Kind: kind},
			}
		}
		w, _ := runContentTransitionCfg(t, HAlignLeft, "41", "42", mk)
		var draws int
		for _, c := range renderContentTransitionAt(t, w, 0.5) {
			switch c.Kind {
			case RenderText, RenderLayout, RenderLayoutTransformed:
				draws++
				if c.TextGradient == nil {
					t.Errorf("kind %d: %q drawn with no gradient", kind, c.Text)
				}
			}
		}
		if draws == 0 {
			t.Errorf("kind %d: nothing drawn", kind)
		}
	}
}

// A box recorded before a frame that skipped the text — culled or
// fully transparent — is out of date. The old text is then aligned in
// the current box, not drawn where that stale box was.
func TestContentTransitionStaleBoxFallsBack(t *testing.T) {
	mk := func(text string) TextCfg {
		return TextCfg{
			ID:         "n",
			Text:       text,
			Transition: ContentTransition{Kind: ContentTransitionFade},
		}
	}
	w := NewTestWindow(t, WindowCfg{State: new(int), Width: 200, Height: 80})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	text := "Three digits"
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			HAlign:  HAlignCenter,
			Content: []View{Text(mk(text))},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	// One render pass in which the text was not drawn.
	w.renderPass++
	text = "Counting"
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.stopAnimationLoop()
	var oldX, newX float32 = -1, -1
	for _, c := range renderContentTransitionAt(t, w, 0.5) {
		switch c.Text {
		case "Three digits":
			oldX = c.X
		case "Counting":
			newX = c.X
		}
	}
	if oldX < 0 || newX < 0 {
		t.Fatalf("want both texts drawn, got old %v new %v", oldX, newX)
	}
	if oldX != newX {
		t.Errorf("old text at x=%v, want the current box's %v", oldX, newX)
	}
}

// A frame partway through a transition allocates nothing, a gradient
// text included: the gradient dimmed for the fade comes from a render
// arena, not the heap.
func TestContentTransitionFrameAllocatesNothing(t *testing.T) {
	grad := &glyph.GradientConfig{Stops: []glyph.GradientStop{
		{Color: glyph.Color{R: 255, A: 255}},
		{Position: 1, Color: glyph.Color{B: 255, A: 255}},
	}}
	style := DefaultTextStyle
	style.Gradient = grad
	for _, kind := range []ContentTransitionKind{
		ContentTransitionFade, ContentTransitionNumeric,
		ContentTransitionReplace,
	} {
		mk := func(text string) TextCfg {
			return TextCfg{
				ID:         "n",
				Text:       text,
				TextStyle:  style,
				Transition: ContentTransition{Kind: kind},
			}
		}
		w, _ := runContentTransitionCfg(t, HAlignLeft, "41", "42", mk)
		kf := contentTransitionDriver(w)
		if kf == nil {
			t.Fatalf("kind %d: no driver", kind)
		}
		kf.OnValue(0.5, w)
		frame := func() { w.renderOnlyLocked() }
		for range 5 {
			frame()
		}
		if got := testing.AllocsPerRun(50, frame); got > 0 {
			t.Errorf("kind %d: frame allocs = %v, want 0", kind, got)
		}
	}
}
