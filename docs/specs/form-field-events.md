# Spec: the form works out field events itself

Issue #1257. Status: **implemented** (unreleased). Found while fixing #1236.

## Problem

`Form` did not know which widget belonged to which field. The app reported every
event by hand. It registered each field every frame with
`FormRegisterFieldByID`, then called `FormOnFieldEvent` from each widget's
callbacks with the same field config again. Two parts of this could not work:

- **Validation on blur never ran.** Input calls `OnBlur` after the frame (#394),
  with `ctx.Layout == nil`. `FormOnFieldEvent` needs that layout to find its
  form, so it returned without doing anything. The showcase form had this
  wiring.
- **Enter never submitted.** Nothing in go-gui requested a submit on Enter, so
  `FormCfg.NoSubmitOnEnter` turned off a feature that did not exist. The #230
  sweep had also made `FormRequestSubmitForLayout` private, the function an app
  could wire itself.

The code, the showcase guide and the wiki each described a different API. Four
of the guide's 14 Go blocks did not compile, and the wiki's names did not exist.

## Decision

Each frame, the form compares what it sees with the last frame and turns the
difference into the triggers the app used to report. The app only registers its
fields, which it already did.

- **Link.** A field's widget is the widget inside the form whose leaf ID is the
  `FieldID`, or `FormFieldAdapterCfg.WidgetID` when set. The form records the
  widget's effective ID in its pass (`formLocateWidgets`). Focus is in the field
  when the focused effective ID equals that ID or starts with it plus `:`. A
  composite builds its parts' IDs under its own, popups included, so the prefix
  covers them.
- **Change.** A value that differs from the last frame while the widget has
  focus. A value the app sets while the widget has no focus is taken over
  without validation; submit validates every field anyway.
- **Blur.** Focus that left the widget. It is validated one pass later
  (`blurPending`). A widget commits its text on blur in a deferred callback
  (Input's normalize, NumericInput's clamp to Min and Max), so the pass that
  sees focus leave still holds the old text. Validating there left an error for
  `-5` on a field the clamp had set to `0`.
- **Re-check.** After a change, the form re-runs the sync validators of every
  field it has already checked, so a rule such as "confirm matches password"
  follows an edit of the password. Async validators are not re-run. A field set
  to `FormValidateOnSubmit` is skipped: its issues change on submit only. The
  test is `formShouldValidate(mode, FormTriggerBlur)`, not `FormTriggerChange`:
  the default mode, `FormValidateOnBlurSubmit`, does not validate on change, and
  the re-check exists for it.
- **Enter.** `inputCommitEnter` requests submit from the form around a
  single-line Input, unless the form sets `NoSubmitOnEnter` or the Input's
  `OnEnter` consumed the key. go-gui's own fields that use Enter for something
  else consume it: the command palette, the data grid's jump-to-row boxes and
  the color picker's fields.
- **ID.** `FormCfg.ID` is required again (`gui:"required"`). An app registers
  fields and submits by the form's ID, so a generated one was useless. An
  ID-less form shows its content and nothing else, and `DebugMissingIDs` reports
  it. `autoLeafWindow`, which existed only for Form, is deleted.
- **Deprecation.** `FormOnFieldEvent` keeps working. For a field whose widget
  the form finds, its blur trigger does nothing, because the form runs that blur
  itself. Before, a blur reported while the widget still had focus (a commit on
  Enter) validated the field twice.
- **Finding the form.** `formFindAncestorID` accepts a `form:` prefix only when
  that form exists. A part of a composite inside a form carries an absolute ID
  under the form's scope (`form:f:cf:hex`). Taken as a form ID, it named a form
  `f:cf`, and Enter and `FormOnFieldEvent` found nothing.

Nothing is called back when the form validates. It stores the results and
redraws the window. The view reads them every frame with `FormFieldErrors`,
`FormFieldState`, `FormSummary` and `FormPendingState`, or through the slots.

## Rejected Approaches

- **Validator fields on every input widget's Cfg**, each widget talking to its
  form. About ten Cfgs would grow a shared block of fields and event code.
  Custom widgets would still need the manual route, so there would be two APIs.
- **Making the six functions the #230 sweep took private public again**
  (`FormRegisterField`, `FormOnFieldEventByID`, `FormFindAncestorID`,
  `FormRequestSubmitForLayout`, `(*Window).FormSubmit`, `FormReset`). It keeps
  the manual wiring and its duplicated field config. Two of the six only repeat
  `FormRequestSubmit` and `FormRequestReset`.
- **The wiki's model**: validators registered once, the form reading values from
  its widgets. The app owns the values; Input shows what the app passes each
  frame. The form would need code per widget type, and custom widgets would be
  left out.
- **Giving a deferred `OnBlur` a `Layout` again.** It runs after the frame, when
  the arena has reused that layout; the app would get a dangling reference.
- **Counting any value difference as a change.** Data the app loads would be
  validated before the user touched the field, and errors would show early.
- **A `DependsOn []string` field for cross-field rules.** New API that an app
  forgets to fill in. Re-running a sync validator on an unchanged value gives
  the same result, so the re-check needs no declaration.
- **Re-checking only fields whose mode validates on change.** Proposed in
  review. It switches the re-check off in the default mode.
- **Validating a blur at once and again when the committed value arrives.**
  Proposed in review. It validates twice and shows the stale error for a frame.
  `blurPending` validates once, with the committed value.
- **Skipping a blur validation when nothing changed since the last one** (to
  remove the double run of a reported blur). Extra state in the core, kept only
  for a deprecated path. The rule "the form owns blur for a field whose widget
  it finds" is one condition.

## Tests

- `gui/view_form_detect_test.go`: blur and change without wiring, an app-set
  value checked on submit, Enter for Input, NumericInput, `NoSubmitOnEnter`, an
  `OnEnter` that consumes or not, multi-line Input; NumericInput's blur commit;
  re-check and the submit-only skip; `WidgetID`; reset; the ID-less form.
- `gui/view_form_ancestor_test.go`: the form lookup, including absolute part
  IDs.
- `gui/view_form_blur_report_test.go`: `FormOnFieldEvent` blur reports.
- `gui/view_color_fields_enter_test.go`,
  `gui/datagrid/view_data_grid_jump_form_test.go`, and the palette tests: Enter
  in go-gui's own fields does not submit a form around them.
- `gui/form_example_test.go`: every Go block of the forms guide, run through
  real frames and pinned by `examples/showcase/sound_snippet_test.go`.
