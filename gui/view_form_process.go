package gui

import (
	"slices"
)

func formShouldValidate(
	mode FormValidateOn, trigger FormValidationTrigger,
) bool {
	switch mode {
	case FormValidateInherit:
		// Should not reach here — formResolveValidateOn resolves
		// Inherit before validation.
		panic("gui: formShouldValidate called with unresolved " +
			"FormValidateInherit")
	case FormValidateOnChange:
		return true
	case FormValidateOnBlur, FormValidateOnBlurSubmit:
		return trigger == FormTriggerBlur ||
			trigger == FormTriggerSubmit
	case FormValidateOnSubmit:
		return trigger == FormTriggerSubmit
	default:
		return false
	}
}

func formResolveValidateOn(
	override, fallback FormValidateOn,
) FormValidateOn {
	if override == FormValidateInherit {
		return fallback
	}
	return override
}

func formMergeErrors(field *formFieldRuntime) []FormIssue {
	if len(field.syncErrors) == 0 {
		return field.asyncErrors
	}
	if len(field.asyncErrors) == 0 {
		return field.syncErrors
	}
	merged := make([]FormIssue, 0, len(field.syncErrors)+len(field.asyncErrors))
	merged = append(merged, field.syncErrors...)
	merged = append(merged, field.asyncErrors...)
	return merged
}

func formToPublicFieldState(
	field *formFieldRuntime,
) FormFieldState {
	return FormFieldState{
		Value:        field.value,
		InitialValue: field.initialValue,
		Touched:      field.touched,
		Dirty:        field.dirty,
		Pending:      field.pending,
		Errors:       formMergeErrors(field),
	}
}

func formSnapshotFromState(
	formID string, state *formRuntimeState,
) FormSnapshot {
	values := make(map[string]string, len(state.fields))
	fields := make(map[string]FormFieldState, len(state.fields))
	for fid, field := range state.fields {
		values[fid] = field.value
		fields[fid] = formToPublicFieldState(field)
	}
	return FormSnapshot{
		FormID: formID,
		Values: values,
		Fields: fields,
	}
}

func formFieldSnapshot(
	formID, fieldID string, field *formFieldRuntime,
) FormFieldSnapshot {
	return FormFieldSnapshot{
		FormID:  formID,
		FieldID: fieldID,
		Value:   field.value,
		Touched: field.touched,
		Dirty:   field.dirty,
	}
}

// formHasBlockingIssue reports whether issues make a field invalid. A
// warning does not (#1236). Any other Kind does, so an out-of-range
// value blocks submit instead of slipping through.
func formHasBlockingIssue(issues []FormIssue) bool {
	for i := range issues {
		if issues[i].Kind != FormIssueWarning {
			return true
		}
	}
	return false
}

func formComputeSummary(
	state *formRuntimeState,
) FormSummaryState {
	var invalidCount, pendingCount int
	var issues map[string][]FormIssue
	for fieldID, field := range state.fields {
		merged := formMergeErrors(field)
		if len(merged) > 0 {
			if formHasBlockingIssue(merged) {
				invalidCount++
			}
			if issues == nil {
				issues = make(map[string][]FormIssue, 4)
			}
			issues[fieldID] = merged
		}
		if field.pending {
			pendingCount++
		}
	}
	return FormSummaryState{
		Valid:        invalidCount == 0 && pendingCount == 0,
		Pending:      pendingCount > 0,
		InvalidCount: invalidCount,
		PendingCount: pendingCount,
		Issues:       issues,
	}
}

func formComputePending(
	formID string, state *formRuntimeState,
) FormPendingState {
	var ids []string
	for fid, field := range state.fields {
		if field.pending {
			if ids == nil {
				ids = make([]string, 0, 4)
			}
			ids = append(ids, fid)
		}
	}
	slices.Sort(ids)
	return FormPendingState{
		FormID:       formID,
		FieldIDs:     ids,
		PendingCount: len(ids),
	}
}

func formApplyCfg(w *Window, formID string, cfg FormCfg) {
	state := formRuntime(w, formID)
	vo := cfg.ValidateOn
	if vo == FormValidateInherit {
		vo = FormValidateOnBlurSubmit
	}
	state.validateOn = vo
	state.submitOnEnter = !cfg.NoSubmitOnEnter
	state.blockInvalid = !cfg.AllowInvalidSubmit
	state.blockPending = !cfg.AllowPendingSubmit
	state.disabled = cfg.Disabled
}

// formCleanupStale removes fields not seen this layout
// generation. Called from the form column's AmendLayout.
// Increments layoutGen at the end so registrations in the
// NEXT frame use the new value.
func formCleanupStale(w *Window, formID string) {
	state := formRuntime(w, formID)
	gen := state.layoutGen
	if len(state.fields) > 0 {
		stale := make([]string, 0, 4)
		for fieldID, field := range state.fields {
			if field.seenGen != gen {
				stale = append(stale, fieldID)
			}
		}
		for _, fieldID := range stale {
			field := state.fields[fieldID]
			if field.pending && field.activeAbort != nil {
				field.activeAbort.Abort()
			}
			delete(state.fields, fieldID)
		}
	}
	state.layoutGen++
}

func formProcessRequests(
	w *Window,
	formID string,
	onSubmit func(FormSubmitEvent, EventCtx),
	onReset func(FormResetEvent, EventCtx),
	cues soundCues,
) {
	state := formRuntime(w, formID)
	stateChanged := false
	if state.disabled {
		state.submitReq = false
		state.resetReq = false
		return
	}

	if state.resetReq {
		values := make(map[string]string, len(state.fields))
		for fieldID, field := range state.fields {
			if field.pending && field.activeAbort != nil {
				field.activeAbort.Abort()
			}
			field.value = field.initialValue
			// The reset value is not an edit: the next pass compares
			// against it, and nothing counts as checked yet.
			field.prevValue = field.initialValue
			field.checked = false
			field.blurPending = false
			field.dirty = false
			field.touched = false
			field.pending = false
			field.syncErrors = field.syncErrors[:0]
			field.asyncErrors = field.asyncErrors[:0]
			field.activeAbort = nil
			values[fieldID] = field.initialValue
		}
		state.resetReq = false
		stateChanged = true
		if onReset != nil {
			onReset(FormResetEvent{
				FormID: formID,
				Values: values,
			}, EventCtx{nil, nil, w})
		}
	}

	if !state.submitReq {
		if stateChanged {
			w.InvalidateLayout()
		}
		return
	}

	state.submitReq = false
	stateChanged = true
	fieldIDs := make([]string, 0, len(state.fields))
	for fid := range state.fields {
		fieldIDs = append(fieldIDs, fid)
	}
	slices.Sort(fieldIDs)
	for _, fieldID := range fieldIDs {
		field := state.fields[fieldID]
		if field == nil {
			continue
		}
		formRunTrigger(w, formID, state, fieldID, field, FormTriggerSubmit)
	}

	summary := formComputeSummary(state)
	blockedInvalid := state.blockInvalid && summary.InvalidCount > 0
	blockedPending := state.blockPending && summary.Pending
	// Before the callback and independent of consumption, the same
	// ordering every other cue keeps. The submitReq latch above makes
	// this one-shot even though AmendLayout runs every frame; emitting
	// inline is safe because playSoundCue takes no lock (issue #469).
	if blockedInvalid || blockedPending {
		playSoundCue(cues.reject, w)
	} else {
		playSoundCue(cues.act, w)
	}
	if !blockedInvalid && !blockedPending && onSubmit != nil {
		onSubmit(FormSubmitEvent{
			FormID:  formID,
			Values:  formSnapshotFromState(formID, state).Values,
			Valid:   summary.Valid,
			Pending: summary.Pending,
			State:   summary,
		}, EventCtx{nil, nil, w})
	}
	if stateChanged {
		w.InvalidateLayout()
	}
}

// ---------- public API ----------

// formFindAncestorID walks the parent chain to the nearest form and
// returns its form ID, or "" when layout is in no form. The "form:"
// prefix alone does not identify a form's container: a part of a
// composite widget inside a form carries an absolute ID under the
// form's scope ("form:f:cf:hex"), which decodes to a form "f:cf" that
// does not exist. So a candidate counts only when that form has
// runtime state.
func formFindAncestorID(w *Window, layout *Layout) string {
	for l := layout; l != nil; l = l.Parent {
		if l.Shape == nil {
			continue
		}
		if formID := formDecodeLayoutID(l.Shape.ID); formID != "" &&
			formRuntimeRead(w, formID) != nil {
			return formID
		}
	}
	return ""
}

// formRegisterField registers a field with the ancestor form
// found by walking layout's parent chain. Use in AmendLayout
// or event handlers where parents are set.
func formRegisterField(
	w *Window, layout *Layout, cfg FormFieldAdapterCfg,
) {
	if cfg.FieldID == "" {
		return
	}
	formID := formFindAncestorID(w, layout)
	if formID == "" {
		return
	}
	FormRegisterFieldByID(w, formID, cfg)
}

// formEnsureField creates or updates a field's registration in
// the form state. Returns the field runtime for further mutation.
func formEnsureField(
	state *formRuntimeState, cfg FormFieldAdapterCfg,
) *formFieldRuntime {
	field, exists := state.fields[cfg.FieldID]
	if !exists {
		field = &formFieldRuntime{}
		if cfg.HasInitialValue {
			field.initialValue = cfg.InitialValue
		} else {
			field.initialValue = cfg.Value
		}
		// The value a field starts with is not an edit.
		field.prevValue = cfg.Value
		state.fields[cfg.FieldID] = field
	}
	field.widgetID = cfg.WidgetID
	if field.widgetID == "" {
		field.widgetID = cfg.FieldID
	}
	field.value = cfg.Value
	field.dirty = field.value != field.initialValue
	field.syncVals = cfg.SyncValidators
	field.asyncVals = cfg.AsyncValidators
	field.validateOn = formResolveValidateOn(
		cfg.ValidateOnOverride, state.validateOn)
	field.seenGen = state.layoutGen
	return field
}

// FormRegisterFieldByID registers a field with the form formID. Call it
// every frame from the view, with the field's current value: a field
// that is not registered in a frame is dropped at the end of it.
//
// The form links the field to the widget whose ID is cfg.FieldID, or
// cfg.WidgetID when set. It validates the field when the user edits it,
// when focus leaves it and on submit; FormCfg.ValidateOn and
// cfg.ValidateOnOverride choose which of these run the validators.
// Nothing is called back. The view reads the results every frame with
// (*Window).FormFieldErrors, FormFieldState and FormSummary, or shows
// them through FormCfg.ErrorSlot and the other slots. The form redraws
// the window when they change.
func FormRegisterFieldByID(
	w *Window, formID string, cfg FormFieldAdapterCfg,
) {
	if cfg.FieldID == "" || formID == "" {
		return
	}
	state := formRuntime(w, formID)
	formEnsureField(state, cfg)
}

// FormOnFieldEvent validates a field now, for trigger. It finds the
// form by walking up from layout and updates the field from cfg. On a
// blur or submit trigger it marks the field touched. When the field's
// mode covers trigger, it runs the sync validators, restarts the async
// ones and stores the results; on a change it also re-checks the other
// fields already checked. Nothing is notified: the view reads the
// results as described at FormRegisterFieldByID. A blur trigger does
// nothing for a field whose widget the form has found, because the
// form runs that blur itself when focus leaves the widget.
//
// Deprecated: the form detects edits and blur on its fields' widgets
// itself (#1257), with the same results. Remove the call and register
// the field every frame with FormRegisterFieldByID. In a deferred
// callback such as Input's OnBlur, ctx.Layout is nil and this call
// does nothing.
//
// exportaudit:keep — deprecated; apps still call it until they migrate
func FormOnFieldEvent(
	w *Window, layout *Layout,
	cfg FormFieldAdapterCfg, trigger FormValidationTrigger,
) {
	if cfg.FieldID == "" {
		return
	}
	formID := formFindAncestorID(w, layout)
	if formID == "" {
		return
	}
	formOnFieldEventForForm(w, formID, cfg, trigger)
}

// formOnFieldEventByID triggers validation for a field with a
// known form ID.
func formOnFieldEventByID(
	w *Window, formID string,
	cfg FormFieldAdapterCfg, trigger FormValidationTrigger,
) {
	if cfg.FieldID == "" || formID == "" {
		return
	}
	formOnFieldEventForForm(w, formID, cfg, trigger)
}

func formOnFieldEventForForm(
	w *Window,
	formID string,
	cfg FormFieldAdapterCfg,
	trigger FormValidationTrigger,
) {
	state := formRuntime(w, formID)
	field := formEnsureField(state, cfg)
	if trigger == FormTriggerBlur && field.widgetEff != "" {
		// The form found this field's widget and runs its blur itself
		// when focus leaves it. A reported blur, often sent while the
		// widget still has focus (a commit on Enter), would validate
		// the field twice.
		return
	}
	formRunTrigger(w, formID, state, cfg.FieldID, field, trigger)
	if trigger == FormTriggerChange {
		// The caller reported this edit itself, so the amend pass must
		// not count it a second time.
		field.prevValue = field.value
		formRecheckOthers(formID, state, cfg.FieldID)
	}
}

// formRunTrigger marks the field touched on blur or submit, then runs
// its validators when its mode covers trigger.
func formRunTrigger(
	w *Window, formID string, state *formRuntimeState,
	fieldID string, field *formFieldRuntime,
	trigger FormValidationTrigger,
) {
	if trigger == FormTriggerBlur || trigger == FormTriggerSubmit {
		field.touched = true
	}
	if !formShouldValidate(field.validateOn, trigger) {
		return
	}
	formRunSync(formID, state, fieldID, field)
	formStartAsync(w, formID, state, fieldID, field)
}

// formRunSync replaces the field's sync issues with what its sync
// validators report for the current values.
func formRunSync(
	formID string, state *formRuntimeState,
	fieldID string, field *formFieldRuntime,
) {
	field.checked = true
	field.syncErrors = field.syncErrors[:0]
	if len(field.syncVals) == 0 {
		return
	}
	snapshot := formSnapshotFromState(formID, state)
	fieldSnap := formFieldSnapshot(formID, fieldID, field)
	for _, validator := range field.syncVals {
		issues := validator(fieldSnap, snapshot)
		if len(issues) > 0 {
			field.syncErrors = append(field.syncErrors, issues...)
		}
	}
}

// formStartAsync aborts the field's running async validators and
// starts them again for the current values.
func formStartAsync(
	w *Window, formID string, state *formRuntimeState,
	fieldID string, field *formFieldRuntime,
) {
	if field.pending && field.activeAbort != nil {
		field.activeAbort.Abort()
	}
	field.pending = false
	field.asyncErrors = field.asyncErrors[:0]
	field.activeAbort = nil

	if len(field.asyncVals) > 0 {
		field.pending = true
		field.requestSeq++
		requestID := field.requestSeq
		controller := NewGridAbortController()
		field.activeAbort = controller
		snapshot := formSnapshotFromState(formID, state)
		fieldSnap := formFieldSnapshot(formID, fieldID, field)
		validators := slices.Clone(field.asyncVals)
		signal := controller.Signal
		go func() {
			var issues []FormIssue
			for _, validator := range validators {
				if signal.IsAborted() {
					return
				}
				result := validator(
					fieldSnap, snapshot, signal)
				if len(result) > 0 {
					issues = append(issues, result...)
				}
			}
			if signal.IsAborted() {
				return
			}
			w.QueueCommand(func(w *Window) {
				formApplyAsyncResult(
					w, formID, fieldID,
					requestID, issues)
			})
		}()
	}
}

func formApplyAsyncResult(
	w *Window, formID, fieldID string,
	requestID uint64, issues []FormIssue,
) {
	state := formRuntimeRead(w, formID)
	if state == nil {
		return
	}
	field, ok := state.fields[fieldID]
	if !ok {
		return
	}
	if requestID != field.requestSeq {
		return
	}
	field.pending = false
	field.activeAbort = nil
	field.asyncErrors = slices.Clone(issues)
	w.InvalidateLayout()
}

// FormRequestSubmit asks the form to submit, typically from a submit
// button's OnClick; Enter in a single-line Input inside the form calls
// it too. In its next frame the form validates every field. It then
// calls OnSubmit, unless an error or a running async validator blocks
// the submit (see FormCfg.AllowInvalidSubmit and AllowPendingSubmit).
// A blocked submit calls nothing; the issues show through the read
// functions and slots.
func FormRequestSubmit(w *Window, formID string) {
	if formID == "" {
		return
	}
	state := formRuntime(w, formID)
	state.submitReq = true
	w.InvalidateLayout()
}

// FormRequestReset asks the form to reset. In its next frame every
// field goes back to its initial value and loses its issues and its
// touched state. Then OnReset gets the initial values, and must copy
// them into the app's state (see FormResetEvent).
func FormRequestReset(w *Window, formID string) {
	if formID == "" {
		return
	}
	state := formRuntime(w, formID)
	state.resetReq = true
	w.InvalidateLayout()
}

// formRequestSubmitForLayout finds the ancestor form and requests
// submit unless the form sets NoSubmitOnEnter. Input's Enter handling
// calls it (#1257).
func formRequestSubmitForLayout(w *Window, layout *Layout) {
	if layout == nil || layout.Shape == nil {
		return
	}
	formID := formFindAncestorID(w, layout)
	if formID == "" {
		return
	}
	state := formRuntimeRead(w, formID)
	if state == nil || !state.submitOnEnter {
		return
	}
	FormRequestSubmit(w, formID)
}

// FormSummary returns the validation state of the whole form, as of
// its last validation. Read it in the view, every frame: it changes
// after an edit, a blur, a submit or an async result, and the form
// redraws the window when it does. A form that has not been built yet
// reads as valid, so a submit button can be enabled from the start.
func (w *Window) FormSummary(formID string) FormSummaryState {
	state := formRuntimeRead(w, formID)
	if state == nil {
		return FormSummaryState{Valid: true}
	}
	return formComputeSummary(state)
}

// FormPendingState returns the fields whose async validators run, for
// a "checking …" line. Read it in the view, like FormSummary.
func (w *Window) FormPendingState(
	formID string,
) FormPendingState {
	state := formRuntimeRead(w, formID)
	if state == nil {
		return FormPendingState{FormID: formID}
	}
	return formComputePending(formID, state)
}

// FormFieldState returns a field's state: value, issues, touched,
// dirty and pending. ok is false when the field is not registered.
// Read it in the view, like FormSummary.
func (w *Window) FormFieldState(
	formID, fieldID string,
) (FormFieldState, bool) {
	state := formRuntimeRead(w, formID)
	if state == nil {
		return FormFieldState{}, false
	}
	field, ok := state.fields[fieldID]
	if !ok {
		return FormFieldState{}, false
	}
	return formToPublicFieldState(field), true
}

// FormFieldErrors returns a field's issues, warnings included, or nil
// when it has none or is not registered. The slice is the form's own
// and the next validation reuses it: read it in the view, and copy it
// before keeping it past the frame.
func (w *Window) FormFieldErrors(
	formID, fieldID string,
) []FormIssue {
	fs, ok := w.FormFieldState(formID, fieldID)
	if !ok {
		return nil
	}
	return fs.Errors
}

// formSubmit requests form submit and triggers a window update.
func (w *Window) formSubmit(formID string) {
	FormRequestSubmit(w, formID)
}

// formReset requests form reset and triggers a window update.
func (w *Window) formReset(formID string) {
	FormRequestReset(w, formID)
}
