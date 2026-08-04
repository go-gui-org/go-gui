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

// The offsets arrive from a platform input method and index the
// preedit for slice arithmetic in the render path, so they must land
// inside the composition whatever the IME reports. Counted in runes:
// "かん" is two.
func TestIMEUpdateClampsOffsets(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		start, length          int32
		wantCursor, wantSelLen int
	}{
		{"in range", 1, 1, 1, 1},
		{"negative start", -5, 2, 0, 2},
		{"start past end", 9, 0, 2, 0},
		{"negative length", 0, -3, 0, 0},
		{"length past end", 1, 9, 1, 1},
		{"NSNotFound truncated to -1", -1, -1, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWindow()
			w.imeUpdate(&Event{
				Type:      EventIMEComposition,
				IMEText:   "かん",
				IMEStart:  tc.start,
				IMELength: tc.length,
			})
			if got := w.IMECompCursor(); got != tc.wantCursor {
				t.Errorf("cursor: got %d, want %d", got, tc.wantCursor)
			}
			if got := w.IMECompSelLen(); got != tc.wantSelLen {
				t.Errorf("sel len: got %d, want %d", got, tc.wantSelLen)
			}
		})
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

// renderIMEPreedit composes "かん" into a focused text field and
// returns the string the renderer emitted. cursor is the field's caret
// position; placeholder marks the field text as a hint rather than
// content.
func renderIMEPreedit(t *testing.T, text string, cursor int,
	placeholder bool,
) string {
	t.Helper()

	const id = "ime-field"
	w := makeWindowWithScratch()
	w.viewState.focusID = id
	StateMap[string, InputState](w, nsInput, capMany).Set(
		id, InputState{CursorPos: cursor})
	w.imeUpdate(&Event{Type: EventIMEComposition, IMEText: "かん"})

	style := DefaultTextStyle
	renderText(&Shape{
		shapeType: shapeText,
		Focusable: true, ID: id,
		Width: 200, Height: 20,
		Opacity: 1.0,
		TC: &ShapeTextConfig{
			Text:              text,
			TextStyle:         &style,
			TextIsPlaceholder: placeholder,
		},
	}, makeClip(0, 0, 400, 400), w)

	for _, r := range w.renderers {
		if r.Kind == RenderText {
			return r.Text
		}
	}
	t.Fatal("no text command emitted")
	return ""
}

// A composition replaces the placeholder rather than being inserted
// into it. Inserting pushed the placeholder out to the right of the
// preedit, leaving the hint visible while the user typed.
func TestRenderTextIMEReplacesPlaceholder(t *testing.T) {
	if got := renderIMEPreedit(t, "Enter a name", 0, true); got != "かん" {
		t.Fatalf("composing over a placeholder rendered %q, want %q",
			got, "かん")
	}
}

// The same composition in a field with real text is still inserted at
// the cursor, not substituted for the content.
func TestRenderTextIMEInsertsIntoRealText(t *testing.T) {
	if got := renderIMEPreedit(t, "abcd", 2, false); got != "abかんcd" {
		t.Fatalf("preedit at cursor rendered %q, want %q",
			got, "abかんcd")
	}
}

// A cursor outside the text must not panic the render path: the
// preedit is drawn every frame, so a bad offset would take the whole
// app down rather than mis-draw one string.
func TestRenderTextIMEOutOfRangeCursorNoPanic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cursor int
		want   string
	}{
		{"negative", -5, "かんabcd"},
		{"past end", 99, "abcdかん"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := renderIMEPreedit(t, "abcd", tc.cursor, false)
			if got != tc.want {
				t.Fatalf("rendered %q, want %q", got, tc.want)
			}
		})
	}
}
