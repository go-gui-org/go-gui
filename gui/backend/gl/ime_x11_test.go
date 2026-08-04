//go:build linux && !js && !android

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/ibus"
)

func TestIMEEventsPreedit(t *testing.T) {
	got := imeEvents([]ibus.Event{
		{Kind: ibus.KindPreedit, Text: "にほん", Cursor: 2, SelLen: 1},
	}, nil)

	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	e := got[0]
	if e.Type != gui.EventIMEComposition || e.IMEText != "にほん" ||
		e.IMEStart != 2 || e.IMELength != 1 {
		t.Errorf("preedit mapped to %+v", e)
	}
}

// An empty preedit is how a composition ends; it must still produce an
// event, otherwise the widget keeps rendering stale text.
func TestIMEEventsEmptyPreedit(t *testing.T) {
	got := imeEvents([]ibus.Event{{Kind: ibus.KindPreedit}}, nil)
	if len(got) != 1 || got[0].Type != gui.EventIMEComposition ||
		got[0].IMEText != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestIMEEventsCommit(t *testing.T) {
	got := imeEvents([]ibus.Event{
		{Kind: ibus.KindCommit, Text: "日本"},
	}, nil)

	if len(got) != 2 {
		t.Fatalf("got %d events, want one per rune", len(got))
	}
	for i, want := range []rune{'日', '本'} {
		e := got[i]
		if e.Type != gui.EventChar || e.CharCode != uint32(want) ||
			e.IMEText != string(want) {
			t.Errorf("rune %d mapped to %+v", i, e)
		}
		if e.Modifiers != 0 {
			t.Errorf("rune %d carried modifiers %v", i, e.Modifiers)
		}
	}
}

func TestIMEEventsCommitDropsReplacementChar(t *testing.T) {
	got := imeEvents([]ibus.Event{
		{Kind: ibus.KindCommit, Text: "a�b"},
	}, nil)
	if len(got) != 2 || got[0].CharCode != 'a' || got[1].CharCode != 'b' {
		t.Fatalf("got %+v", got)
	}
}

// A forwarded key is one the engine declined after all, so it has to
// come out exactly like an unconsumed key press: key down plus, for a
// printable keysym, a character.
func TestIMEEventsForwardKey(t *testing.T) {
	got := imeEvents([]ibus.Event{
		{Kind: ibus.KindForwardKey, Keyval: 'a', Keycode: 38},
	}, nil)
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if got[0].Type != gui.EventKeyDown {
		t.Errorf("first event is %v, want EventKeyDown", got[0].Type)
	}
	if got[1].Type != gui.EventChar || got[1].CharCode != 'a' {
		t.Errorf("second event is %+v", got[1])
	}
}

// Non-printable keysyms yield a key down only.
func TestIMEEventsForwardKeyNonPrintable(t *testing.T) {
	const xkReturn = 0xff0d
	got := imeEvents([]ibus.Event{
		{Kind: ibus.KindForwardKey, Keyval: xkReturn},
	}, nil)
	if len(got) != 1 || got[0].Type != gui.EventKeyDown {
		t.Fatalf("got %+v", got)
	}
}

// drainIME reuses both buffers, so the conversion must append rather
// than assume an empty destination.
func TestIMEEventsAppends(t *testing.T) {
	dst := []gui.Event{{Type: gui.EventResized}}
	dst = imeEvents([]ibus.Event{{Kind: ibus.KindCommit, Text: "x"}}, dst)
	if len(dst) != 2 || dst[0].Type != gui.EventResized ||
		dst[1].CharCode != 'x' {
		t.Fatalf("got %+v", dst)
	}
}

func TestIMEEventsEmptyInput(t *testing.T) {
	if got := imeEvents(nil, nil); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

// With no input method present the backend must behave exactly as
// before: no client, no queue, and the key path untouched.
func TestNoIMEClientIsInert(t *testing.T) {
	b := &Backend{}
	b.drainIME()
	if b.imeProcessKey('a', 38, 0) {
		t.Error("imeProcessKey consumed a key with no input method")
	}
	if b.plat.imeEvts != nil {
		t.Error("drainIME allocated with no input method")
	}
}
