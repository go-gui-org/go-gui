package gui

import "strings"

// view_form_detect.go — the form works out field events itself (#1257).
//
// The app registers each field every frame with its current value
// (FormRegisterFieldByID). The form's AmendLayout then compares this
// frame with the last one and turns the difference into the triggers
// apps used to report by hand through FormOnFieldEvent:
//
//   - a value that differs while focus is in the field's widget is a
//     change;
//   - focus that left the field's widget is a blur. It is validated one
//     pass later: a widget commits its text on blur in a deferred
//     callback (Input's normalize, NumericInput's clamp to Min and
//     Max), so in the pass that sees focus leave, the app still holds
//     the text from before the commit.
//
// A value the app changes while the widget does not have focus, data
// loaded from a server for example, is taken over without validation.
// Submit validates every field, so it is still checked before the app
// gets OnSubmit.

// formDetectFieldEvents runs the change and blur triggers that
// happened since the last pass. Called from the form's AmendLayout,
// after formCleanupStale and before formProcessRequests.
func formDetectFieldEvents(w *Window, layout *Layout, formID string) {
	state := formRuntimeRead(w, formID)
	if state == nil || len(state.fields) == 0 || state.disabled {
		return
	}
	focusID := w.FocusID()
	if focusID != "" || formAnyFocused(state) {
		formLocateWidgets(layout, state)
	}
	acted := false
	for fieldID, field := range state.fields {
		now := formFocusIn(focusID, field.widgetEff)
		changed := field.value != field.prevValue
		field.prevValue = field.value
		switch {
		case field.blurPending:
			// Focus left at the last pass; the commit has landed.
			field.blurPending = false
			formRunTrigger(w, formID, state, fieldID, field,
				FormTriggerBlur)
			if changed {
				formRecheckOthers(formID, state, fieldID)
			}
			acted = true
		case changed && (now || field.focused):
			formRunTrigger(w, formID, state, fieldID, field,
				FormTriggerChange)
			formRecheckOthers(formID, state, fieldID)
			acted = true
		}
		if field.focused && !now {
			field.blurPending = true
			// Ask for the pass that validates it.
			acted = true
		}
		field.focused = now
	}
	if acted {
		// The view already read the old issues this frame.
		w.InvalidateLayout()
	}
}

// formRecheckOthers re-runs the sync validators of every field that
// has been checked before, except fieldID. A rule that reads another
// field, such as "confirm matches password", then follows an edit of
// that other field. Async validators are not re-run: they may be
// slow or costly, and only the edited field's value changed.
//
// A field set to validate on submit only is skipped: its issues change
// on submit and nowhere else. The test is "does its mode validate on
// blur", not "on change": the default mode, FormValidateOnBlurSubmit,
// does not validate on change, and the re-check exists for it.
func formRecheckOthers(
	formID string, state *formRuntimeState, fieldID string,
) {
	for id, other := range state.fields {
		if id == fieldID || !other.checked ||
			!formShouldValidate(other.validateOn, FormTriggerBlur) {
			continue
		}
		formRunSync(formID, state, id, other)
	}
}

// formAnyFocused reports whether focus was in one of the form's
// fields at the last pass, so a blur still needs the widget lookup
// when nothing has focus now.
func formAnyFocused(state *formRuntimeState) bool {
	for _, field := range state.fields {
		if field.focused {
			return true
		}
	}
	return false
}

// formFocusIn reports whether focusID is the widget with effective ID
// widgetEff or a part of it. A composite builds its parts' IDs under
// its own (ScopeID), popups included, so a prefix match covers them.
func formFocusIn(focusID, widgetEff string) bool {
	if focusID == "" || widgetEff == "" {
		return false
	}
	return focusID == widgetEff ||
		(strings.HasPrefix(focusID, widgetEff) &&
			focusID[len(widgetEff)] == ':')
}

// formLocateWidgets records the effective ID of each field's widget,
// found by its leaf ID inside the form. A field whose widget is not in
// the form gets an empty widgetEff: no change or blur is detected for
// it, and submit still validates it.
func formLocateWidgets(layout *Layout, state *formRuntimeState) {
	for _, field := range state.fields {
		field.widgetEff = ""
	}
	for i := range layout.Children {
		formLocateWidgetsDepth(&layout.Children[i], state, 1)
	}
}

func formLocateWidgetsDepth(
	layout *Layout, state *formRuntimeState, depth int,
) {
	if overMaxDepth(depth) {
		return
	}
	if s := layout.Shape; s != nil && s.ID != "" {
		for _, field := range state.fields {
			if field.widgetEff == "" && field.widgetID == s.ID {
				field.widgetEff = s.idKey()
			}
		}
	}
	for i := range layout.Children {
		formLocateWidgetsDepth(&layout.Children[i], state, depth+1)
	}
}
