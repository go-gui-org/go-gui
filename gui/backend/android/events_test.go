//go:build android

package android

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
)

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

func TestIMECommitKey(t *testing.T) {
	cases := []struct {
		text string
		want gui.KeyCode
		ok   bool
	}{
		{"\b", gui.KeyBackspace, true},
		{"\x7f", gui.KeyDelete, true},
		{"\n", gui.KeyEnter, true},
		{"\r", gui.KeyEnter, true},
		{"a", gui.KeyInvalid, false},
		// A pasted line keeps its text; only a lone key maps.
		{"a\n", gui.KeyInvalid, false},
		{"\b\b", gui.KeyInvalid, false},
	}
	for _, tc := range cases {
		got, ok := imeCommitKey(tc.text)
		if got != tc.want || ok != tc.ok {
			t.Errorf("imeCommitKey(%q) = %v, %v; want %v, %v",
				tc.text, got, ok, tc.want, tc.ok)
		}
	}
}

func TestIMEDeleteEvents(t *testing.T) {
	cases := []struct {
		name          string
		before, after int64
		inCodePoints  bool
		want          []gui.Event
	}{
		{"both", 2, 1, true, []gui.Event{
			{Type: gui.EventKeyDown, KeyCode: gui.KeyBackspace,
				IMEDeleteCount: 2, IMEDeleteInCodePoints: true},
			{Type: gui.EventKeyDown, KeyCode: gui.KeyDelete,
				IMEDeleteCount: 1, IMEDeleteInCodePoints: true},
		}},
		{"after only", 0, 3, false, []gui.Event{
			{Type: gui.EventKeyDown, KeyCode: gui.KeyDelete, IMEDeleteCount: 3},
		}},
		{"empty", 0, 0, false, nil},
		// A Java int is never negative here, but the bridge widens it
		// to int64 and must not dispatch a negative count.
		{"negative", -1, -5, false, nil},
		{"capped", math.MaxInt64, 0, false, []gui.Event{
			{Type: gui.EventKeyDown, KeyCode: gui.KeyBackspace,
				IMEDeleteCount: math.MaxInt32},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evts, n := imeDeleteEvents(tc.before, tc.after, tc.inCodePoints)
			if n != len(tc.want) {
				t.Fatalf("got %d events, want %d", n, len(tc.want))
			}
			for i, want := range tc.want {
				if evts[i] != want {
					t.Errorf("event %d = %+v, want %+v", i, evts[i], want)
				}
			}
		})
	}
}
