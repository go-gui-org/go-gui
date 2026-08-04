//go:build linux

package ibus

import "github.com/godbus/dbus/v5"

// IBus marshals its objects as IBusSerializable, whose base layout is
// (name, attachments) followed by the subtype's own fields. IBusText is
// therefore (sa{sv}sv): name, attachments, text, attribute list. godbus
// decodes a struct with no static target type into []any, so the text
// sits at index 2.
const (
	textFieldName = 0
	textFieldText = 2
	textFields    = 3
)

// decodeText extracts the string from a marshalled IBusText.
//
// The value crosses an untyped D-Bus boundary from a process go-gui does
// not control, so every step is checked rather than asserted: a
// malformed or unexpected shape returns false and the caller drops the
// signal instead of panicking in the render path.
//
// Variant wrapping is unwrapped iteratively rather than recursively: a
// hostile daemon can nest wrappers arbitrarily deep, and recursion
// would grow the stack without bound.
func decodeText(v any) (string, bool) {
	for {
		switch t := v.(type) {
		case dbus.Variant:
			v = t.Value()
		case string:
			// Not a shape IBus sends, but harmless to accept.
			return t, true
		case []any:
			if len(t) < textFields {
				return "", false
			}
			if name, ok := t[textFieldName].(string); !ok || name != "IBusText" {
				return "", false
			}
			s, ok := t[textFieldText].(string)
			return s, ok
		default:
			return "", false
		}
	}
}

// decodePreedit reads the body of an UpdatePreeditText or
// UpdatePreeditTextWithMode signal. Both carry (text, cursorPos,
// visible); the WithMode variant appends a mode argument that go-gui
// does not use, so the two are handled by the same path.
func decodePreedit(body []any) (text string, cursor uint32, visible, ok bool) {
	if len(body) < 3 {
		return "", 0, false, false
	}
	text, ok = decodeText(body[0])
	if !ok {
		return "", 0, false, false
	}
	cursor, ok = body[1].(uint32)
	if !ok {
		return "", 0, false, false
	}
	visible, ok = body[2].(bool)
	if !ok {
		return "", 0, false, false
	}
	return text, cursor, visible, true
}

// decodeForwardKey reads the body of a ForwardKeyEvent signal, which the
// engine sends for keys it declined to consume after all.
func decodeForwardKey(body []any) (keyval, keycode, state uint32, ok bool) {
	if len(body) < 3 {
		return 0, 0, 0, false
	}
	keyval, ok = body[0].(uint32)
	if !ok {
		return 0, 0, 0, false
	}
	keycode, ok = body[1].(uint32)
	if !ok {
		return 0, 0, 0, false
	}
	state, ok = body[2].(uint32)
	if !ok {
		return 0, 0, 0, false
	}
	return keyval, keycode, state, true
}
