//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"
	"testing"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
)

func TestUTF16RuneIndex(t *testing.T) {
	cases := []struct {
		name string
		s    string
		n    int
		want int32
	}{
		{"empty", "", 5, 0},
		{"ascii", "hello", 2, 2},
		{"clamped past end", "hi", 99, 2},
		{"negative", "ab", -3, 0},
		{"bmp multibyte", "あい", 1, 1},
		{"after astral pair", "a𝄞b", 3, 2},
		// An offset inside a surrogate pair lands after the
		// astral character, matching the Win32 imeRuneIndex rule.
		{"inside astral pair", "a𝄞b", 2, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := utf16RuneIndex(tc.s, tc.n); got != tc.want {
				t.Errorf("utf16RuneIndex(%q, %d) = %d, want %d",
					tc.s, tc.n, got, tc.want)
			}
		})
	}
}

func TestIMESanitizeText(t *testing.T) {
	if got := imeSanitizeText("a�b"); got != "ab" {
		t.Errorf("imeSanitizeText stripped to %q, want %q", got, "ab")
	}
	if got := imeSanitizeText(""); got != "" {
		t.Errorf("imeSanitizeText(\"\") = %q, want empty", got)
	}
	if got := imeSanitizeText("�"); got != "" {
		t.Errorf("imeSanitizeText lone failure = %q, want empty", got)
	}
	big := strings.Repeat("あ", maxIMETextRunes+10)
	if got := imeSanitizeText(big); utf8.RuneCountInString(got) != maxIMETextRunes {
		t.Errorf("sanitized runes = %d, want %d",
			utf8.RuneCountInString(got), maxIMETextRunes)
	}
}

// A missing composition data value must not coerce to a literal:
// Value.String on null yields "null", which would arrive as a bogus
// one-word preedit.
func TestJSStringNoCoercion(t *testing.T) {
	cases := []struct {
		name string
		v    js.Value
		want string
	}{
		{"null", js.Null(), ""},
		{"undefined", js.Undefined(), ""},
		{"number", js.ValueOf(42), ""},
		{"string", js.ValueOf("かん"), "かん"},
		{"empty string", js.ValueOf(""), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsString(tc.v); got != tc.want {
				t.Errorf("jsString = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyBeforeInput(t *testing.T) {
	cases := []struct {
		inputType string
		want      beforeInputKind
	}{
		{"insertText", beforeInputText},
		{"insertReplacementText", beforeInputText},
		{"deleteContentBackward", beforeInputBackspace},
		{"deleteContentForward", beforeInputDelete},
		{"insertLineBreak", beforeInputEnter},
		{"insertParagraph", beforeInputEnter},
		// Composition has its own events.
		{"insertCompositionText", beforeInputIgnore},
		{"", beforeInputIgnore},
	}
	for _, tc := range cases {
		if got := classifyBeforeInput(tc.inputType); got != tc.want {
			t.Errorf("classifyBeforeInput(%q) = %d, want %d",
				tc.inputType, got, tc.want)
		}
	}
}

func TestBeforeInputData(t *testing.T) {
	obj := js.Global().Get("Object")
	e := obj.New()
	e.Set("data", "a")
	if got := beforeInputData(e); got != "a" {
		t.Errorf("data: got %q, want %q", got, "a")
	}
	// A replacement carries its text in dataTransfer, data is null.
	e = obj.New()
	e.Set("data", js.Null())
	dt := obj.New()
	dt.Set("getData", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		return "Diana"
	}))
	e.Set("dataTransfer", dt)
	if got := beforeInputData(e); got != "Diana" {
		t.Errorf("dataTransfer: got %q, want %q", got, "Diana")
	}
	if got := beforeInputData(obj.New()); got != "" {
		t.Errorf("empty event: got %q, want empty", got)
	}
}

// A soft keyboard's edit keys arrive with an empty code (Android Chrome
// through KEYCODE_DEL, iOS Safari). They must map from key instead, or the
// key is lost: the hidden input is empty, so no beforeinput follows.
func TestDOMKeyNameFallsBackToKey(t *testing.T) {
	cases := []struct {
		code, key string
		keyCode   int
		want      gui.KeyCode
	}{
		{"Backspace", "Backspace", 8, gui.KeyBackspace},
		{"", "Backspace", 8, gui.KeyBackspace},
		{"", "Enter", 13, gui.KeyEnter},
		{"", "Delete", 46, gui.KeyDelete},
		{"", "ArrowLeft", 37, gui.KeyLeft},
		// A printable key with no code is typed by the char path.
		{"", "a", 65, gui.KeyInvalid},
		{"", "Unidentified", 229, gui.KeyInvalid},
		// An input method's keydown maps to nothing: Android Chrome
		// ignores its preventDefault, so the beforeinput that follows
		// (insertLineBreak, deleteContentBackward) makes the edit.
		// Mapping it too would send Enter twice.
		{"", "Enter", 229, gui.KeyInvalid},
		{"", "Backspace", 229, gui.KeyInvalid},
		// code wins when present: an AZERTY "KeyQ" sends key "a".
		{"KeyQ", "a", 81, gui.KeyQ},
	}
	for _, tc := range cases {
		got := mapKeyCode(domKeyName(tc.code, tc.key, tc.keyCode))
		if got != tc.want {
			t.Errorf("code %q key %q keyCode %d: got %d, want %d",
				tc.code, tc.key, tc.keyCode, got, tc.want)
		}
	}
}

// A keyCode 229 keydown belongs to the input method: Android Chrome
// ignores preventDefault on it, so typing its key would type it twice,
// once here and once from the beforeinput that follows.
func TestIsIMEProcessKey(t *testing.T) {
	cases := []struct {
		keyCode int
		key     string
		want    bool
	}{
		{229, "a", true},
		{229, "Unidentified", true},
		{0, "Process", true},
		{0, "Unidentified", true},
		{65, "a", false},
		{8, "Backspace", false},
	}
	for _, tc := range cases {
		if got := isIMEProcessKey(tc.keyCode, tc.key); got != tc.want {
			t.Errorf("isIMEProcessKey(%d, %q) = %v, want %v",
				tc.keyCode, tc.key, got, tc.want)
		}
	}
}

// Delete on the hidden input must cancel its default, as Backspace does:
// otherwise the beforeinput deleteContentForward that follows deletes a
// second character.
func TestPreventKeyDefault(t *testing.T) {
	cases := []struct {
		name      string
		imeTarget bool
		want      bool
	}{
		{"Backspace", false, true},
		{"Tab", false, true},
		{"Delete", true, true},
		{"Enter", true, true},
		{"NumpadEnter", true, true},
		// A host page's own inputs keep Delete and Enter.
		{"Delete", false, false},
		{"Enter", false, false},
		{"KeyA", true, false},
	}
	for _, tc := range cases {
		got := preventKeyDefault(tc.name, mapKeyCode(tc.name), tc.imeTarget)
		if got != tc.want {
			t.Errorf("preventKeyDefault(%q, ime=%v) = %v, want %v",
				tc.name, tc.imeTarget, got, tc.want)
		}
	}
}

// keyEvent builds a fake KeyboardEvent on the hidden input. Every field a
// handler reads with Bool is set, since Bool panics on undefined.
func keyEvent(code, key string, keyCode int) (js.Value, *bool) {
	obj := js.Global().Get("Object")
	target := obj.New()
	target.Set("hasAttribute", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return args[0].String() == imeInputAttr
	}))
	e := obj.New()
	e.Set("target", target)
	e.Set("code", code)
	e.Set("key", key)
	e.Set("keyCode", keyCode)
	for _, f := range []string{"isComposing", "shiftKey", "ctrlKey",
		"altKey", "metaKey", "repeat"} {
		e.Set(f, false)
	}
	prevented := new(bool)
	e.Set("preventDefault", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		*prevented = true
		return nil
	}))
	return e, prevented
}

// recordEvents returns an emit that copies each event into a slice.
func recordEvents() (func(*gui.Event), *[]gui.Event) {
	var got []gui.Event
	return func(e *gui.Event) { got = append(got, *e) }, &got
}

// countType counts the events of one type.
func countType(evts []gui.Event, typ gui.EventType) int {
	n := 0
	for _, e := range evts {
		if e.Type == typ {
			n++
		}
	}
	return n
}

func TestHandleKeyDownTypesPrintableKey(t *testing.T) {
	emit, got := recordEvents()
	e, prevented := keyEvent("KeyA", "a", 65)
	handleKeyDown(emit, new(gui.Event), e)
	if n := countType(*got, gui.EventChar); n != 1 {
		t.Fatalf("got %d EventChar, want 1: %+v", n, *got)
	}
	// Typed here, so the browser must not type it into the hidden input.
	if !*prevented {
		t.Error("printable keydown on the hidden input not prevented")
	}
}

// An input method's keydown types nothing: the beforeinput that follows
// types the key, and Android Chrome ignores this keydown's preventDefault.
func TestHandleKeyDownIMEProcessKeyTypesNothing(t *testing.T) {
	for _, key := range []string{"a", "Enter", "Backspace"} {
		emit, got := recordEvents()
		e, _ := keyEvent("", key, 229)
		handleKeyDown(emit, new(gui.Event), e)
		if n := countType(*got, gui.EventChar); n != 0 {
			t.Errorf("key %q: got %d EventChar, want 0", key, n)
		}
		for _, ev := range *got {
			if ev.Type == gui.EventKeyDown && ev.KeyCode != gui.KeyInvalid {
				t.Errorf("key %q: key-down %d, want KeyInvalid", key, ev.KeyCode)
			}
		}
	}
}

// A soft keyboard's Backspace with no code still reaches the field.
func TestHandleKeyDownEmptyCodeMapsKey(t *testing.T) {
	emit, got := recordEvents()
	e, prevented := keyEvent("", "Backspace", 8)
	handleKeyDown(emit, new(gui.Event), e)
	if len(*got) != 1 || (*got)[0].KeyCode != gui.KeyBackspace {
		t.Fatalf("got %+v, want one KeyBackspace key-down", *got)
	}
	if !*prevented {
		t.Error("Backspace not prevented")
	}
}

func TestHandleKeyDownDeleteOnHiddenInputPrevented(t *testing.T) {
	emit, _ := recordEvents()
	e, prevented := keyEvent("Delete", "Delete", 46)
	handleKeyDown(emit, new(gui.Event), e)
	if !*prevented {
		t.Error("Delete on the hidden input not prevented")
	}
}

func TestHandleKeyUpMapsKeyLikeKeyDown(t *testing.T) {
	cases := []struct {
		code, key string
		keyCode   int
		want      gui.KeyCode
	}{
		{"", "Backspace", 8, gui.KeyBackspace},
		{"", "Enter", 229, gui.KeyInvalid},
		{"KeyA", "a", 65, gui.KeyA},
	}
	for _, tc := range cases {
		emit, got := recordEvents()
		e, _ := keyEvent(tc.code, tc.key, tc.keyCode)
		handleKeyUp(emit, new(gui.Event), e)
		if len(*got) != 1 || (*got)[0].Type != gui.EventKeyUp ||
			(*got)[0].KeyCode != tc.want {
			t.Errorf("code %q key %q keyCode %d: got %+v, want key-up %d",
				tc.code, tc.key, tc.keyCode, *got, tc.want)
		}
	}
}

// inputEvent builds a fake beforeinput on the hidden input.
func inputEvent(inputType, data string) (js.Value, *bool) {
	e, prevented := keyEvent("", "", 0)
	e.Set("inputType", inputType)
	e.Set("data", data)
	return e, prevented
}

func TestHandleBeforeInputDispatch(t *testing.T) {
	cases := []struct {
		name, inputType, data string
		want                  gui.Event
	}{
		{"text", "insertText", "é", gui.Event{
			Type: gui.EventChar, CharCode: 'é', IMEText: "é"}},
		{"replacement", "insertReplacementText", "the", gui.Event{
			Type: gui.EventChar, CharCode: 't', IMEText: "the"}},
		{"backspace", "deleteContentBackward", "", gui.Event{
			Type: gui.EventKeyDown, KeyCode: gui.KeyBackspace}},
		{"delete", "deleteContentForward", "", gui.Event{
			Type: gui.EventKeyDown, KeyCode: gui.KeyDelete}},
		{"enter", "insertLineBreak", "", gui.Event{
			Type: gui.EventKeyDown, KeyCode: gui.KeyEnter}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			emit, got := recordEvents()
			e, prevented := inputEvent(tc.inputType, tc.data)
			handleBeforeInput(emit, new(gui.Event), e)
			if !*prevented {
				t.Error("beforeinput not prevented")
			}
			if len(*got) != 1 || (*got)[0] != tc.want {
				t.Errorf("got %+v, want %+v", *got, tc.want)
			}
		})
	}
}

// Empty text is still cancelled, so the hidden input stays empty, but
// types nothing.
func TestHandleBeforeInputEmptyTextSendsNothing(t *testing.T) {
	emit, got := recordEvents()
	e, prevented := inputEvent("insertText", "")
	handleBeforeInput(emit, new(gui.Event), e)
	if !*prevented || len(*got) != 0 {
		t.Errorf("prevented %v, events %+v; want prevented, none", *prevented, *got)
	}
}

// A beforeinput outside the hidden input, or during a composition, is
// left to the browser and the composition events.
func TestHandleBeforeInputIgnored(t *testing.T) {
	foreign, fPrevented := inputEvent("insertText", "a")
	foreign.Set("target", js.Global().Get("Object").New())
	composing, cPrevented := inputEvent("insertText", "a")
	composing.Set("isComposing", true)
	for name, c := range map[string]struct {
		e         js.Value
		prevented *bool
	}{"foreign": {foreign, fPrevented}, "composing": {composing, cPrevented}} {
		emit, got := recordEvents()
		handleBeforeInput(emit, new(gui.Event), c.e)
		if *c.prevented || len(*got) != 0 {
			t.Errorf("%s: prevented %v, events %+v; want neither", name,
				*c.prevented, *got)
		}
	}
}

func TestJSInt(t *testing.T) {
	cases := []struct {
		name string
		v    js.Value
		want int
	}{
		{"number", js.ValueOf(229), 229},
		{"undefined", js.Undefined(), 0},
		{"null", js.Null(), 0},
		{"string", js.ValueOf("229"), 0},
	}
	for _, tc := range cases {
		if got := jsInt(tc.v); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
