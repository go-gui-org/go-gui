//go:build linux

package ibus

import "github.com/godbus/dbus/v5"

// IBus marshals its objects as IBusSerializable, whose base layout is
// (name, attachments) followed by the subtype's own fields. IBusText is
// therefore (sa{sv}sv): name, attachments, text, attribute list. godbus
// decodes a struct with no static target type into []any, so the text
// sits at index 2.
//
// The attribute list is optional here: only a minimum of three fields is
// required, so an engine that omits it still yields its text.
const (
	textFieldName  = 0
	textFieldText  = 2
	textFieldAttrs = 3
	textFields     = 3
)

// The attribute list is IBusAttrList, (sa{sv}av): name, attachments,
// attributes. Each element is an IBusAttribute, (sa{sv}uuuu): name,
// attachments, type, value, start index, end index. The indices are
// character offsets, matching the rune-based clamping in gui.imeUpdate.
const (
	attrListFieldName  = 0
	attrListFieldAttrs = 2
	attrListFields     = 3

	attrFieldName  = 0
	attrFieldType  = 2
	attrFieldValue = 3
	attrFieldStart = 4
	attrFieldEnd   = 5
	attrFields     = 6

	attrTypeUnderline  = 1
	attrTypeBackground = 3

	attrUnderlineDouble = 2
)

// unwrap strips variant wrappers.
//
// Iteratively rather than recursively: a hostile daemon can nest
// wrappers arbitrarily deep, and recursion would grow the stack without
// bound.
func unwrap(v any) any {
	for {
		t, isVariant := v.(dbus.Variant)
		if !isVariant {
			return v
		}
		v = t.Value()
	}
}

// decodeText extracts the string from a marshalled IBusText.
func decodeText(v any) (string, bool) {
	s, _, _, ok := decodeTextSel(v)
	return s, ok
}

// decodeTextSel extracts the string from a marshalled IBusText along
// with the range of the selected clause, if the engine marked one.
// A zero-width range means no clause is selected.
//
// The value crosses an untyped D-Bus boundary from a process go-gui does
// not control, so every step is checked rather than asserted: a
// malformed or unexpected shape returns false and the caller drops the
// signal instead of panicking in the render path. A malformed *attribute
// list* is weaker than that — the text is still good, so it only costs
// the clause highlight.
func decodeTextSel(v any) (text string, selStart, selEnd uint32, ok bool) {
	switch t := unwrap(v).(type) {
	case string:
		// Not a shape IBus sends, but harmless to accept.
		return t, 0, 0, true
	case []any:
		if len(t) < textFields {
			return "", 0, 0, false
		}
		if name, isStr := t[textFieldName].(string); !isStr || name != "IBusText" {
			return "", 0, 0, false
		}
		text, ok = t[textFieldText].(string)
		if !ok {
			return "", 0, 0, false
		}
		if len(t) > textFieldAttrs {
			selStart, selEnd, _ = decodeAttrSel(t[textFieldAttrs])
		}
		return text, selStart, selEnd, true
	default:
		return "", 0, 0, false
	}
}

// decodeAttrSel finds the range of the selected clause in an
// IBusAttrList.
//
// Which attribute marks it is engine convention, not specification.
// ibus-mozc emits both a background attribute and a double underline
// over the focused segment, so background wins and a lone double
// underline serves as the fallback for engines that use only that.
// Anything else — the single underline mozc puts over the unconverted
// remainder, foreground colour — is not a selection marker.
func decodeAttrSel(v any) (start, end uint32, ok bool) {
	list, isSlice := unwrap(v).([]any)
	if !isSlice || len(list) < attrListFields {
		return 0, 0, false
	}
	if name, isStr := list[attrListFieldName].(string); !isStr || name != "IBusAttrList" {
		return 0, 0, false
	}
	attrs, isSlice := unwrap(list[attrListFieldAttrs]).([]dbus.Variant)
	if !isSlice {
		return 0, 0, false
	}

	var underStart, underEnd uint32
	haveUnder := false
	for _, a := range attrs {
		typ, val, s, e, valid := decodeAttr(a)
		if !valid || e <= s {
			continue
		}
		switch {
		case typ == attrTypeBackground:
			return s, e, true
		case typ == attrTypeUnderline && val == attrUnderlineDouble && !haveUnder:
			underStart, underEnd, haveUnder = s, e, true
		}
	}
	if haveUnder {
		return underStart, underEnd, true
	}
	return 0, 0, false
}

// decodeAttr reads one IBusAttribute.
func decodeAttr(v any) (typ, val, start, end uint32, ok bool) {
	f, isSlice := unwrap(v).([]any)
	if !isSlice || len(f) < attrFields {
		return 0, 0, 0, 0, false
	}
	if name, isStr := f[attrFieldName].(string); !isStr || name != "IBusAttribute" {
		return 0, 0, 0, 0, false
	}
	fields := [4]uint32{}
	for i, idx := range [4]int{
		attrFieldType, attrFieldValue, attrFieldStart, attrFieldEnd,
	} {
		fields[i], ok = f[idx].(uint32)
		if !ok {
			return 0, 0, 0, 0, false
		}
	}
	return fields[0], fields[1], fields[2], fields[3], true
}

// preedit is one decoded UpdatePreeditText body.
type preedit struct {
	text             string
	cursor           uint32
	selStart, selEnd uint32
	visible          bool
}

// decodePreedit reads the body of an UpdatePreeditText or
// UpdatePreeditTextWithMode signal. Both carry (text, cursorPos,
// visible); the WithMode variant appends a mode argument that go-gui
// does not use, so the two are handled by the same path.
func decodePreedit(body []any) (preedit, bool) {
	if len(body) < 3 {
		return preedit{}, false
	}
	p := preedit{}
	var ok bool
	if p.text, p.selStart, p.selEnd, ok = decodeTextSel(body[0]); !ok {
		return preedit{}, false
	}
	if p.cursor, ok = body[1].(uint32); !ok {
		return preedit{}, false
	}
	if p.visible, ok = body[2].(bool); !ok {
		return preedit{}, false
	}
	return p, true
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
