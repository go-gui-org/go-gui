//go:build linux

package ibus

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

// ibusText builds the []any shape godbus produces for a marshalled
// IBusText when there is no static target type to decode into.
func ibusText(s string) []any {
	return []any{
		"IBusText",
		map[string]dbus.Variant{},
		s,
		dbus.MakeVariant([]any{"IBusAttrList", map[string]dbus.Variant{}, []dbus.Variant{}}),
	}
}

func TestDecodeText(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
		ok   bool
	}{
		{"variant wrapped", dbus.MakeVariant(ibusText("日本語")), "日本語", true},
		{"bare struct", ibusText("hello"), "hello", true},
		{"empty text", ibusText(""), "", true},
		{"bare string", "plain", "plain", true},
		{"too short", []any{"IBusText", map[string]dbus.Variant{}}, "", false},
		{"wrong name", []any{"IBusAttrList", map[string]dbus.Variant{}, "x"}, "", false},
		{"text not a string", []any{"IBusText", map[string]dbus.Variant{}, 42}, "", false},
		{"name not a string", []any{7, map[string]dbus.Variant{}, "x"}, "", false},
		{"unexpected type", uint32(3), "", false},
		{"nil", nil, "", false},
		{"nested variant", dbus.MakeVariant(dbus.MakeVariant(ibusText("x"))), "x", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := decodeText(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Errorf("decodeText(%v) = %q, %v; want %q, %v",
					tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestDecodePreedit(t *testing.T) {
	text := dbus.MakeVariant(ibusText("にほん"))

	t.Run("three arg form", func(t *testing.T) {
		s, cur, vis, ok := decodePreedit([]any{text, uint32(2), true})
		if !ok || s != "にほん" || cur != 2 || !vis {
			t.Fatalf("got %q, %d, %v, %v", s, cur, vis, ok)
		}
	})

	t.Run("four arg with mode", func(t *testing.T) {
		s, cur, vis, ok := decodePreedit([]any{text, uint32(0), false, uint32(1)})
		if !ok || s != "にほん" || cur != 0 || vis {
			t.Fatalf("got %q, %d, %v, %v", s, cur, vis, ok)
		}
	})

	bad := []struct {
		name string
		body []any
	}{
		{"empty", nil},
		{"too short", []any{text, uint32(0)}},
		{"bad text", []any{dbus.MakeVariant(uint32(1)), uint32(0), true}},
		{"cursor not uint32", []any{text, int32(0), true}},
		{"visible not bool", []any{text, uint32(0), uint32(1)}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, ok := decodePreedit(tc.body); ok {
				t.Errorf("decodePreedit(%v) accepted a malformed body", tc.body)
			}
		})
	}
}

func TestDecodeForwardKey(t *testing.T) {
	kv, kc, st, ok := decodeForwardKey([]any{uint32(0x61), uint32(38), uint32(4)})
	if !ok || kv != 0x61 || kc != 38 || st != 4 {
		t.Fatalf("got %d, %d, %d, %v", kv, kc, st, ok)
	}

	bad := [][]any{
		nil,
		{uint32(1), uint32(2)},
		{int32(1), uint32(2), uint32(3)},
		{uint32(1), "2", uint32(3)},
		{uint32(1), uint32(2), true},
	}
	for _, body := range bad {
		if _, _, _, ok := decodeForwardKey(body); ok {
			t.Errorf("decodeForwardKey(%v) accepted a malformed body", body)
		}
	}
}
