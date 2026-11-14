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
- **Async validators follow their field's value.** Found in review; both defects
  predate this issue. Every submit restarted the async validators, and by
  default a pending submit is refused, so a form with an async validator never
  submitted. An edit in the default mode did not stop a running check either,
  and its result for the old value landed on the new one. Now:
  1. An async validator runs again only when its field's value has changed
     (`asyncFor`, `asyncCurrent`). A blur or a submit with the same value reuses
     the result, running or done.
  2. A submit that only waits for running checks stays armed (`submitWaiting`).
     Each pass calls `formFinishSubmit` again. When the checks are done, it
     submits, or rejects if they reported an error, with one cue either way. The
     wait is decided by `formSubmitGate`, which allocates nothing: it runs every
     frame of the wait, and `formComputeSummary` builds a map once any field has
     issues.
  3. Every value change, registered or reported, stops the field's running check
     (`formStopAsync`): `requestSeq` moves on, so a queued result is dropped. A
     submit waiting for that check starts over (`submitReq`), so it checks what
     it will send. Reset stops the check too.
  4. A submit requested in the pass where focus left a field (`blurPending`)
     waits one pass. A click on a submit button moves focus in the same event,
     and the field's deferred blur commit has not reached the app yet. Deciding
     at once sent `-5` while NumericInput had clamped the field to `0`, and with
     an async validator the commit then stopped the check and lost the submit.

  An async validator that also reads other fields does not re-run when only
  those change; such a rule belongs in a sync validator.

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
- **Restarting async validators on every trigger** (the behaviour before). It
  made a form with an async validator unable to submit, and it asked a server
  again for a value it had just checked.
- **Reusing async results without an armed submit.** A click while a check runs
  would be rejected with the Error cue, and the user would have to click again.
- **Restarting all checks on submit, then waiting.** It submits, but asks the
  server again on every submit.
- **Telling apps to set `AllowPendingSubmit`.** It skips the check instead of
  waiting for it.
- **Cancelling a waiting submit when a value changes.** The first form of this
  fix. Nothing told the user: no cue, no `OnSubmit`, and the click was lost. A
  field's own blur commit after a click triggered it every time.

## Tests

- `gui/view_form_detect_test.go`: blur and change without wiring, an app-set
  value checked on submit, Enter for Input, NumericInput, `NoSubmitOnEnter`, an
  `OnEnter` that consumes or not, multi-line Input; NumericInput's blur commit;
  re-check and the submit-only skip; `WidgetID`; reset; the ID-less form.
- `gui/view_form_ancestor_test.go`: the form lookup, including absolute part
  IDs.
- `gui/view_form_blur_report_test.go`: `FormOnFieldEvent` blur reports.
- `gui/view_form_async_test.go`: submit with an async validator, a submit that
  waits for a check that passes or fails, a value change that stops a running
  check and makes a waiting submit start over, result reuse for an unchanged
  value, a submit right after a blur commit, and a waiting submit that allocates
  nothing. `TestSoundFormSubmitWaitsForPending` in `gui/sound_nonclick_test.go`:
  one cue, when the wait ends.
- `gui/view_color_fields_enter_test.go`,
  `gui/datagrid/view_data_grid_jump_form_test.go`, and the palette tests: Enter
  in go-gui's own fields does not submit a form around them.
- `gui/form_example_test.go`: every Go block of the forms guide, run through
  real frames and pinned by `examples/showcase/sound_snippet_test.go`.
