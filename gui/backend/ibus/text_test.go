//go:build linux

package ibus

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

// ibusText builds the []any shape godbus produces for a marshalled
// IBusText when there is no static target type to decode into.
func ibusText(s string, attrs ...dbus.Variant) []any {
	return []any{
		"IBusText",
		map[string]dbus.Variant{},
		s,
		dbus.MakeVariant([]any{"IBusAttrList", map[string]dbus.Variant{}, attrs}),
	}
}

// ibusAttr builds one IBusAttribute, (sa{sv}uuuu).
func ibusAttr(typ, val, start, end uint32) dbus.Variant {
	return dbus.MakeVariant([]any{
		"IBusAttribute", map[string]dbus.Variant{},
		typ, val, start, end,
	})
}

// mozcClause1 and mozcClause2 are the attribute lists ibus-mozc emits
// for "今日はいい天気ですね" (今日は / いい天気ですね) as the focus moves
// between the two clauses, captured from live UpdatePreeditText signals.
// The focused clause carries a background, a double underline and a
// foreground over the same range; the rest carries a single underline.
// The indices are rune offsets — 今日は is 3 runes but 9 bytes.
func mozcClause1() []dbus.Variant {
	return []dbus.Variant{
		ibusAttr(attrTypeUnderline, attrUnderlineDouble, 0, 3),
		ibusAttr(attrTypeBackground, 0xd1eaff, 0, 3),
		ibusAttr(2 /* foreground */, 0x000000, 0, 3),
		ibusAttr(attrTypeUnderline, 1 /* single */, 3, 10),
	}
}

func mozcClause2() []dbus.Variant {
	return []dbus.Variant{
		ibusAttr(attrTypeUnderline, 1 /* single */, 0, 3),
		ibusAttr(attrTypeUnderline, attrUnderlineDouble, 3, 10),
		ibusAttr(attrTypeBackground, 0xd1eaff, 3, 10),
		ibusAttr(2 /* foreground */, 0x000000, 3, 10),
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
		p, ok := decodePreedit([]any{text, uint32(2), true})
		if !ok || p.text != "にほん" || p.cursor != 2 || !p.visible {
			t.Fatalf("got %+v, %v", p, ok)
		}
	})

	t.Run("four arg with mode", func(t *testing.T) {
		p, ok := decodePreedit([]any{text, uint32(0), false, uint32(1)})
		if !ok || p.text != "にほん" || p.cursor != 0 || p.visible {
			t.Fatalf("got %+v, %v", p, ok)
		}
	})

	t.Run("first clause focused", func(t *testing.T) {
		conv := dbus.MakeVariant(ibusText("今日はいい天気ですね", mozcClause1()...))
		p, ok := decodePreedit([]any{conv, uint32(0), true})
		if !ok || p.selStart != 0 || p.selEnd != 3 {
			t.Fatalf("got %+v, %v; want clause [0,3)", p, ok)
		}
	})

	t.Run("second clause focused", func(t *testing.T) {
		conv := dbus.MakeVariant(ibusText("今日はいい天気ですね", mozcClause2()...))
		p, ok := decodePreedit([]any{conv, uint32(3), true})
		if !ok || p.selStart != 3 || p.selEnd != 10 {
			t.Fatalf("got %+v, %v; want clause [3,10)", p, ok)
		}
	})

	t.Run("unconverted has no clause", func(t *testing.T) {
		raw := dbus.MakeVariant(ibusText("きょう",
			ibusAttr(attrTypeUnderline, 1, 0, 3)))
		p, ok := decodePreedit([]any{raw, uint32(3), true})
		if !ok || p.selStart != 0 || p.selEnd != 0 {
			t.Fatalf("got %+v, %v; want no clause", p, ok)
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
			if _, ok := decodePreedit(tc.body); ok {
				t.Errorf("decodePreedit(%v) accepted a malformed body", tc.body)
			}
		})
	}
}

func TestDecodeAttrSel(t *testing.T) {
	attrList := func(attrs ...dbus.Variant) any {
		return dbus.MakeVariant([]any{
			"IBusAttrList", map[string]dbus.Variant{}, attrs,
		})
	}

	tests := []struct {
		name       string
		in         any
		start, end uint32
		ok         bool
	}{
		{"mozc clause 1", attrList(mozcClause1()...), 0, 3, true},
		{"mozc clause 2", attrList(mozcClause2()...), 3, 10, true},
		{
			"background wins over double underline", attrList(
				ibusAttr(attrTypeUnderline, attrUnderlineDouble, 0, 2),
				ibusAttr(attrTypeBackground, 0xd1eaff, 4, 9),
			), 4, 9, true,
		},
		{
			"double underline alone", attrList(
				ibusAttr(attrTypeUnderline, attrUnderlineDouble, 2, 5),
			), 2, 5, true,
		},
		{
			"single underline only", attrList(
				ibusAttr(attrTypeUnderline, 1, 0, 4),
			), 0, 0, false,
		},
		{"no attributes", attrList(), 0, 0, false},
		{
			"empty range", attrList(
				ibusAttr(attrTypeBackground, 0xd1eaff, 3, 3),
			), 0, 0, false,
		},
		{
			"inverted range", attrList(
				ibusAttr(attrTypeBackground, 0xd1eaff, 5, 2),
			), 0, 0, false,
		},
		{
			"skips malformed, keeps good", attrList(
				dbus.MakeVariant([]any{"IBusAttribute", map[string]dbus.Variant{}}),
				ibusAttr(attrTypeBackground, 0xd1eaff, 1, 4),
			), 1, 4, true,
		},
		{
			"wrong attribute name", attrList(
				dbus.MakeVariant([]any{
					"IBusText", map[string]dbus.Variant{},
					attrTypeBackground, uint32(0), uint32(0), uint32(3),
				}),
			), 0, 0, false,
		},
		{
			"index not uint32", attrList(
				dbus.MakeVariant([]any{
					"IBusAttribute", map[string]dbus.Variant{},
					attrTypeBackground, uint32(0), int32(0), uint32(3),
				}),
			), 0, 0, false,
		},
		{"wrong list name", dbus.MakeVariant(
			[]any{"IBusText", map[string]dbus.Variant{}, []dbus.Variant{}},
		), 0, 0, false},
		{"list too short", dbus.MakeVariant(
			[]any{"IBusAttrList", map[string]dbus.Variant{}},
		), 0, 0, false},
		{"attrs not a slice", dbus.MakeVariant(
			[]any{"IBusAttrList", map[string]dbus.Variant{}, "nope"},
		), 0, 0, false},
		{"not a struct", dbus.MakeVariant(uint32(7)), 0, 0, false},
		{"nil", nil, 0, 0, false},
		{"deeply nested variant", dbus.MakeVariant(dbus.MakeVariant(
			attrList(ibusAttr(attrTypeBackground, 0xd1eaff, 0, 3)),
		)), 0, 3, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end, ok := decodeAttrSel(tc.in)
			if start != tc.start || end != tc.end || ok != tc.ok {
				t.Errorf("decodeAttrSel = %d, %d, %v; want %d, %d, %v",
					start, end, ok, tc.start, tc.end, tc.ok)
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
