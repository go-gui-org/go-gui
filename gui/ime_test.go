package gui

import "testing"

func TestIMEUpdateSetsComposing(t *testing.T) {
	w := newTestWindow()
	e := &Event{
		Type:      EventIMEComposition,
		IMEText:   "かん",
		IMEStart:  0,
		IMELength: 2,
	}
	w.imeUpdate(e)
	if !w.IMEComposing() {
		t.Fatal("expected composing=true")
	}
	if w.IMECompText() != "かん" {
		t.Fatalf("got %q, want かん", w.IMECompText())
	}
}

func TestIMEClearResetsState(t *testing.T) {
	w := newTestWindow()
	w.imeUpdate(&Event{
		Type:    EventIMEComposition,
		IMEText: "かん",
	})
	w.imeClear()
	if w.IMEComposing() {
		t.Fatal("expected composing=false after clear")
	}
	if w.IMECompText() != "" {
		t.Fatalf("got %q, want empty", w.IMECompText())
	}
}

func TestIMEUpdateEmptyTextClears(t *testing.T) {
	w := newTestWindow()
	w.imeUpdate(&Event{
		Type:    EventIMEComposition,
		IMEText: "かん",
	})
	w.imeUpdate(&Event{
		Type:    EventIMEComposition,
		IMEText: "",
	})
	if w.IMEComposing() {
		t.Fatal("expected composing=false after empty text")
	}
}

func TestIMEClearedOnCharEvent(t *testing.T) {
	w := newTestWindow()
	w.imeUpdate(&Event{
		Type:    EventIMEComposition,
		IMEText: "漢",
	})
	// Simulate EventChar dispatch clearing IME.
	w.imeClear()
	if w.IMEComposing() {
		t.Fatal("expected composing=false after char commit")
	}
}

func TestIMEClearedOnFocusChange(t *testing.T) {
	w := newTestWindow()
	w.imeUpdate(&Event{
		Type:    EventIMEComposition,
		IMEText: "字",
	})
	w.SetFocus("f42")
	if w.IMEComposing() {
		t.Fatal("expected composing=false after focus change")
	}
}

func TestIMESetRectNoopWithoutPlatform(_ *testing.T) {
	w := newTestWindow()
	// Should not panic with nil nativePlatform.
	w.IMESetRect(10, 20, 30, 40)
}
