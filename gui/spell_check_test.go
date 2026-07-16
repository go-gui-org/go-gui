package gui

import (
	"testing"
	"time"
)

// mockSpellPlatform is a minimal NativePlatform stub for spell check
// tests. SpellCheck returns a range for the whole input text.
type mockSpellPlatform struct{ NoopNativePlatform }

func (m *mockSpellPlatform) SpellCheck(text string) []SpellRange {
	if len(text) == 0 {
		return nil
	}
	return []SpellRange{{StartByte: 0, LenBytes: len(text)}}
}

func newSpellCheckWindow(spellChk bool, text string) *Window {
	type appState struct{ text string }
	w := NewWindow(WindowCfg{
		State:  &appState{text: text},
		Width:  400,
		Height: 200,
	})
	w.SetNativePlatform(&mockSpellPlatform{})
	w.viewGenerator = func(w *Window) View {
		app := State[appState](w)
		return Input(InputCfg{
			ID:         "f1",
			Sizing:     FillFit,
			Text:       app.text,
			SpellCheck: spellChk,
		})
	}
	w.SetFocus("f1")
	return w
}

func TestSpellCheckTriggerOnEnable(t *testing.T) {
	// Simulate: user types "helo" with spell check OFF, then
	// enables it. Verify results are stored after delay.
	w := newSpellCheckWindow(false, "helo")
	w.Update()

	// No spell state should exist yet.
	sm := StateMapRead[string, spellCheckState](w, nsSpellCheck)
	if sm != nil {
		if _, ok := sm.Get("f1"); ok {
			t.Fatal("spell state should not exist before enable")
		}
	}

	// Enable spell check by switching the view generator.
	w.viewGenerator = func(_ *Window) View {
		return Input(InputCfg{
			ID:         "f1",
			Sizing:     FillFit,
			Text:       "helo",
			SpellCheck: true,
		})
	}
	w.refreshLayout = true
	w.Update()

	// Pending state should exist (text set, ranges nil).
	sm = StateMapRead[string, spellCheckState](w, nsSpellCheck)
	if sm == nil {
		t.Fatal("spell state map should exist after trigger")
	}
	state, ok := sm.Get("f1")
	if !ok {
		t.Fatal("pending spell state should exist for IDFocus 1")
	}
	if state.Text != "helo" {
		t.Fatalf("pending text = %q, want %q", state.Text, "helo")
	}
	if len(state.Ranges) != 0 {
		t.Fatal("pending state should have nil ranges")
	}

	// Simulate animation firing: directly invoke the callback.
	animID := spellCheckAnimID("f1")
	anim, ok := w.animations[animID]
	if !ok {
		t.Fatal("spell check animation should be registered")
	}
	a, ok := anim.(*Animate)
	if !ok {
		t.Fatal("animation should be *Animate")
	}
	a.Callback(a, w)

	// Results should now be stored.
	state, ok = sm.Get("f1")
	if !ok {
		t.Fatal("spell state should exist after callback")
	}
	if state.Text != "helo" {
		t.Fatalf("text = %q, want %q", state.Text, "helo")
	}
	if len(state.Ranges) == 0 {
		t.Fatal("spell ranges should not be empty after callback")
	}
}

func TestSpellCheckPendingPreventsTimerReset(t *testing.T) {
	w := newSpellCheckWindow(true, "helo")
	w.Update()

	// Trigger happened during Update. Get the animation.
	animID := spellCheckAnimID("f1")
	anim1, ok := w.animations[animID]
	if !ok {
		t.Fatal("animation should exist after first Update")
	}
	start1 := anim1.(*Animate).start

	// Second Update should NOT reset the animation (pending text
	// matches).
	time.Sleep(time.Millisecond)
	w.refreshLayout = true
	w.Update()

	anim2, ok := w.animations[animID]
	if !ok {
		t.Fatal("animation should still exist after second Update")
	}
	start2 := anim2.(*Animate).start
	if start2 != start1 {
		t.Fatal("animation start time should not change — timer was reset")
	}
}

func TestSpellCheckClearOnDisable(t *testing.T) {
	w := newSpellCheckWindow(true, "helo")
	w.Update()

	// State should exist (pending).
	sm := StateMapRead[string, spellCheckState](w, nsSpellCheck)
	if sm == nil {
		t.Fatal("state map should exist")
	}
	if _, ok := sm.Get("f1"); !ok {
		t.Fatal("pending state should exist")
	}

	// Disable spell check.
	w.viewGenerator = func(_ *Window) View {
		return Input(InputCfg{
			ID:         "f1",
			Sizing:     FillFit,
			Text:       "helo",
			SpellCheck: false,
		})
	}
	w.refreshLayout = true
	w.Update()

	// State should be cleared.
	state, ok := sm.Get("f1")
	if ok && len(state.Ranges) > 0 {
		t.Fatal("spell state should be cleared after disable")
	}
	if _, ok := w.animations[spellCheckAnimID("f1")]; ok {
		t.Fatal("animation should be cancelled after disable")
	}
}
