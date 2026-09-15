//go:build linux && !js && !android

package gl

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"

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

	// One event carries the whole string — the contract Event
	// documents and every other backend emits — so the commit is
	// one insert and one undo step.
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	e := got[0]
	if e.Type != gui.EventChar || e.CharCode != uint32('日') ||
		e.IMEText != "日本" {
		t.Errorf("commit mapped to %+v", e)
	}
	if e.Modifiers != 0 {
		t.Errorf("commit carried modifiers %v", e.Modifiers)
	}
}

func TestIMEEventsCommitDropsReplacementChar(t *testing.T) {
	got := imeEvents([]ibus.Event{
		{Kind: ibus.KindCommit, Text: "a�b"},
	}, nil)
	if len(got) != 1 || got[0].CharCode != 'a' ||
		got[0].IMEText != "ab" {
		t.Fatalf("got %+v", got)
	}
}

// An empty commit, or one with nothing left after stripping decoding
// failures, emits nothing: the Win32 backend drops it the same way.
func TestIMEEventsCommitEmptyEmitsNothing(t *testing.T) {
	for _, text := range []string{"", "�"} {
		got := imeEvents([]ibus.Event{
			{Kind: ibus.KindCommit, Text: text},
		}, nil)
		if len(got) != 0 {
			t.Errorf("commit %q emitted %+v, want nothing", text, got)
		}
	}
}

// A hostile engine must not grow the queue without bound through one
// commit: the text is capped like the Win32 composition read.
func TestIMEEventsCommitTruncated(t *testing.T) {
	big := strings.Repeat("あ", maxIMECommitRunes+10)
	got := imeEvents([]ibus.Event{
		{Kind: ibus.KindCommit, Text: big},
	}, nil)
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if n := utf8.RuneCountInString(got[0].IMEText); n != maxIMECommitRunes {
		t.Fatalf("commit runes = %d, want %d", n, maxIMECommitRunes)
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

func TestIMEScaledBounds(t *testing.T) {
	cases := []struct {
		name string
		v    int32
		s    float32
		want int32
	}{
		{"zero", 0, 2, 0},
		{"ordinary", 120, 2, 240},
		{"rounds half up", 3, 0.5, 2},
		{"negative", -40, 2, -80},
		{"past bound", 1 << 29, 8, imePixelMax},
		{"past negative bound", -(1 << 29), 8, -imePixelMax},
		{"nan scale", 100, float32(math.NaN()), 0},
		{"inf scale on zero", 0, float32(math.Inf(1)), 0},
		{"inf scale clamps", 100, float32(math.Inf(1)), imePixelMax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := imeScaled(tc.v, tc.s); got != tc.want {
				t.Errorf("imeScaled(%d, %v) = %d, want %d",
					tc.v, tc.s, got, tc.want)
			}
		})
	}
}

// The root-origin sum must not leave int32 range on a wide virtual
// desktop: the conversion past the check is implementation-defined.
func TestIMEAddOriginSaturates(t *testing.T) {
	if got := imeAddOrigin(imePixelMax, 100); got != imePixelMax {
		t.Errorf("imeAddOrigin = %d, want %d", got, imePixelMax)
	}
	if got := imeAddOrigin(-imePixelMax, -100); got != -imePixelMax {
		t.Errorf("imeAddOrigin = %d, want %d", got, -imePixelMax)
	}
	if got := imeAddOrigin(100, 20); got != 120 {
		t.Errorf("imeAddOrigin = %d, want 120", got)
	}
}

// With no input method present IMEStart/IMEStop must be safe no-ops:
// focusing a text field with no ibus daemon must not panic.
func TestIMENilClientStartStopNoPanic(t *testing.T) {
	b := &Backend{}
	n := &nativePlatform{b: b}
	n.IMEStart()
	n.IMEStop()
	if b.plat.imeHaveRect {
		t.Error("IMEStop with no input method left a rect cached")
	}
}

// A hostile engine must not grow the preedit without bound or smuggle
// decoding failures into the render path: the preedit is sanitized
// like a commit. An empty preedit still emits — it ends the
// composition — so only the text is cleaned, not the event dropped.
func TestIMEEventsPreeditSanitized(t *testing.T) {
	got := imeEvents([]ibus.Event{
		{Kind: ibus.KindPreedit, Text: "a�b", Cursor: 1, SelLen: 1},
	}, nil)
	if len(got) != 1 || got[0].IMEText != "ab" {
		t.Fatalf("preedit with replacement char mapped to %+v", got)
	}

	big := strings.Repeat("あ", maxIMECommitRunes+10)
	got = imeEvents([]ibus.Event{
		{Kind: ibus.KindPreedit, Text: big},
	}, nil)
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if n := utf8.RuneCountInString(got[0].IMEText); n != maxIMECommitRunes {
		t.Fatalf("preedit runes = %d, want %d", n, maxIMECommitRunes)
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
