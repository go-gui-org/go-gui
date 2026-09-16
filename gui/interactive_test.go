package gui

import (
	"strings"
	"testing"
)

// Tests for Interactive (issue #650). The hover, press and focus records
// are set directly on the window: how they are recorded is covered by
// window_interaction_state_test.go. These tests cover what Interactive
// reads from those records and what it gives the builder.

// interactiveProbe returns an Interactive view for id whose builder
// stores the state it gets in *got and returns a column with rootID.
func interactiveProbe(id, rootID string, got *InteractionState) View {
	return Interactive(id, func(s InteractionState) View {
		*got = s
		return Column(ContainerCfg{ID: rootID, SizeBorder: NoBorder})
	})
}

func TestInteractiveStatePerPhase(t *testing.T) {
	tests := []struct {
		name         string
		hover, press string
		focus        string
		want         InteractionState
	}{
		{name: "idle"},
		{name: "hovered", hover: "ok",
			want: InteractionState{Hovered: true}},
		{name: "hovered descendant", hover: "ok:label",
			want: InteractionState{Hovered: true}},
		{name: "pressed off widget", press: "ok",
			want: InteractionState{Pressed: true}},
		{name: "armed", hover: "ok", press: "ok",
			want: InteractionState{Hovered: true, Pressed: true, Armed: true}},
		{name: "focused", focus: "ok",
			want: InteractionState{Focused: true, FocusWithin: true}},
		{name: "other widget", hover: "okay", press: "cancel", focus: "cancel"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWindow()
			w.viewState.hoverTargetID = tc.hover
			w.viewState.pressTargetID = tc.press
			w.setFocusLocked(tc.focus)
			var got InteractionState
			generateViewLayout(interactiveProbe("ok", "ok", &got), w)
			if got != tc.want {
				t.Fatalf("state = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Under an ID-bearing parent the state is read by the effective ID, so
// a record for the bare leaf does not count.
func TestInteractiveReadsScopedID(t *testing.T) {
	w := newTestWindow()
	var got InteractionState
	view := Row(ContainerCfg{ID: "row", SizeBorder: NoBorder,
		Content: []View{interactiveProbe("ok", "ok", &got)}})

	w.viewState.hoverTargetID = "ok"
	generateViewLayout(view, w)
	if got.Hovered {
		t.Fatal("bare leaf record must not hover a scoped widget")
	}

	w.viewState.hoverTargetID = "row:ok"
	w.viewState.pressTargetID = "row:ok"
	layout := generateViewLayout(view, w)
	if !got.Armed {
		t.Fatalf("state = %+v, want armed under scope", got)
	}
	if eid := layout.Children[0].Shape.idKey(); eid != "row:ok" {
		t.Fatalf("root effective ID = %q, want row:ok", eid)
	}
}

// A leaf holding ":" is absolute and is read as spelled.
func TestInteractiveAbsoluteID(t *testing.T) {
	w := newTestWindow()
	var got InteractionState
	w.viewState.hoverTargetID = "dlg:ok"
	generateViewLayout(Row(ContainerCfg{ID: "row", SizeBorder: NoBorder,
		Content: []View{interactiveProbe("dlg:ok", "dlg:ok", &got)}}), w)
	if !got.Hovered {
		t.Fatal("absolute ID must be read as spelled")
	}
}

func TestInteractiveNilBuild(t *testing.T) {
	w := newTestWindow()
	layout := generateViewLayout(Interactive("ok",
		func(InteractionState) View { return nil }), w)
	if layout.Shape == nil || len(layout.Children) != 0 {
		t.Fatalf("nil build must give an empty normalized layout, got %+v", layout)
	}
	layout = generateViewLayout(Interactive("ok", nil), w)
	if layout.Shape == nil {
		t.Fatal("nil builder must give an empty normalized layout")
	}
}

func TestInteractiveIDMismatchWarns(t *testing.T) {
	buf := captureDebugMask(t, DebugMissingIDs)
	w := newTestWindow()
	var got InteractionState

	generateViewLayout(interactiveProbe("ok", "ok", &got), w)
	if s := buf.String(); s != "" {
		t.Fatalf("matching root ID must stay silent, got %q", s)
	}

	generateViewLayout(interactiveProbe("ok", "okay", &got), w)
	s := buf.String()
	if !strings.Contains(s, `"ok"`) || !strings.Contains(s, `"okay"`) {
		t.Fatalf("mismatch must name both IDs, got %q", s)
	}
}

// With the category off, the mismatch check reports nothing.
func TestInteractiveIDMismatchMasked(t *testing.T) {
	buf := captureDebugMask(t, 0)
	w := newTestWindow()
	var got InteractionState
	generateViewLayout(interactiveProbe("ok", "", &got), w)
	if s := buf.String(); s != "" {
		t.Fatalf("masked check must stay silent, got %q", s)
	}
}

// An empty id can never match a hovered, pressed or focused shape, so it
// is reported although the root ID is empty too.
func TestInteractiveEmptyIDWarns(t *testing.T) {
	buf := captureDebugMask(t, DebugMissingIDs)
	w := newTestWindow()
	w.viewState.hoverTargetID = "x"
	var got InteractionState
	generateViewLayout(interactiveProbe("", "", &got), w)
	if got != (InteractionState{}) {
		t.Fatalf("empty id must read no state, got %+v", got)
	}
	if !strings.Contains(buf.String(), "Interactive(\"\")") {
		t.Fatalf("empty id must be reported, got %q", buf.String())
	}
}

// Focused is the widget itself only: a focused descendant does not
// count, unlike Hovered and Pressed.
func TestInteractiveFocusedDescendantDoesNotCount(t *testing.T) {
	w := newTestWindow()
	w.setFocusLocked("ok:field")
	var got InteractionState
	generateViewLayout(interactiveProbe("ok", "ok", &got), w)
	if got.Focused {
		t.Fatal("a focused descendant must not set Focused")
	}
}

// FocusWithin counts the widget and its ID-bearing descendants, the same
// test Hovered and Pressed use: a wrapper sees that its inner Input has
// focus (#664). A sibling whose ID only starts with the same letters
// does not count.
func TestInteractiveFocusWithin(t *testing.T) {
	tests := []struct {
		name, focus string
		want        bool
	}{
		{name: "self", focus: "ok", want: true},
		{name: "descendant", focus: "ok:field", want: true},
		{name: "deep descendant", focus: "ok:row:field", want: true},
		{name: "prefix sibling", focus: "okay"},
		{name: "other widget", focus: "cancel"},
		{name: "no focus", focus: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWindow()
			w.setFocusLocked(tc.focus)
			var got InteractionState
			generateViewLayout(interactiveProbe("ok", "ok", &got), w)
			if got.FocusWithin != tc.want {
				t.Fatalf("FocusWithin = %v, want %v", got.FocusWithin, tc.want)
			}
		})
	}
}
