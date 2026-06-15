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

func formComputeSummary(
	state *formRuntimeState,
) FormSummaryState {
	var invalidCount, pendingCount int
	var issues map[string][]FormIssue
	for fieldID, field := range state.fields {
		merged := formMergeErrors(field)
		if len(merged) > 0 {
			invalidCount++
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
	onSubmit func(FormSubmitEvent, *Window),
	onReset func(FormResetEvent, *Window),
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
			}, w)
		}
	}

	if !state.submitReq {
		if stateChanged {
			w.UpdateWindow()
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
		formOnFieldEventForForm(w, formID, FormFieldAdapterCfg{
			FieldID:            fieldID,
			Value:              field.value,
			InitialValue:       field.initialValue,
			HasInitialValue:    true,
			SyncValidators:     field.syncVals,
			AsyncValidators:    field.asyncVals,
			ValidateOnOverride: field.validateOn,
		}, FormTriggerSubmit)
	}

	summary := formComputeSummary(state)
	blockedInvalid := state.blockInvalid && summary.InvalidCount > 0
	blockedPending := state.blockPending && summary.Pending
	if !blockedInvalid && !blockedPending && onSubmit != nil {
		onSubmit(FormSubmitEvent{
			FormID:  formID,
			Values:  formSnapshotFromState(formID, state).Values,
			Valid:   summary.Valid,
			Pending: summary.Pending,
			State:   summary,
		}, w)
	}
	if stateChanged {
		w.UpdateWindow()
	}
}

// ---------- public API ----------

// FormFindAncestorID walks the parent chain to find an ancestor
// form layout and returns its form ID.
func FormFindAncestorID(layout *Layout) string {
	if layout == nil {
		return ""
	}
	if layout.Shape != nil {
		formID := formDecodeLayoutID(layout.Shape.ID)
		if formID != "" {
			return formID
		}
	}
	if layout.Parent == nil {
		return ""
	}
	return FormFindAncestorID(layout.Parent)
}

// FormRegisterField registers a field with the ancestor form
// found by walking layout's parent chain. Use in AmendLayout
// or event handlers where parents are set.
func FormRegisterField(
	w *Window, layout *Layout, cfg FormFieldAdapterCfg,
) {
	if cfg.FieldID == "" {
		return
	}
	formID := FormFindAncestorID(layout)
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
		state.fields[cfg.FieldID] = field
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

// FormRegisterFieldByID registers a field with a known form ID.
// Safe to call during view construction when the form ID is
// known. Must be called every frame to prevent stale cleanup.
func FormRegisterFieldByID(
	w *Window, formID string, cfg FormFieldAdapterCfg,
) {
	if cfg.FieldID == "" || formID == "" {
		return
	}
	state := formRuntime(w, formID)
	formEnsureField(state, cfg)
}

// FormOnFieldEvent triggers validation for a field based on
// the given trigger type. Walks the parent chain to find the
// ancestor form.
func FormOnFieldEvent(
	w *Window, layout *Layout,
	cfg FormFieldAdapterCfg, trigger FormValidationTrigger,
) {
	if cfg.FieldID == "" {
		return
	}
	formID := FormFindAncestorID(layout)
	if formID == "" {
		return
	}
	formOnFieldEventForForm(w, formID, cfg, trigger)
}

// FormOnFieldEventByID triggers validation for a field with a
// known form ID.
func FormOnFieldEventByID(
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
	if trigger == FormTriggerBlur || trigger == FormTriggerSubmit {
		field.touched = true
	}

	if !formShouldValidate(field.validateOn, trigger) {
		return
	}

	// Sync validation.
	field.syncErrors = field.syncErrors[:0]
	if len(field.syncVals) > 0 {
		snapshot := formSnapshotFromState(formID, state)
		fieldSnap := formFieldSnapshot(formID, cfg.FieldID, field)
		for _, validator := range field.syncVals {
			issues := validator(fieldSnap, snapshot)
			if len(issues) > 0 {
				field.syncErrors = append(
					field.syncErrors, issues...)
			}
		}
	}

	// Abort previous async.
	if field.pending && field.activeAbort != nil {
		field.activeAbort.Abort()
	}
	field.pending = false
	field.asyncErrors = field.asyncErrors[:0]
	field.activeAbort = nil

	// Async validation.
	if len(field.asyncVals) > 0 {
		field.pending = true
		field.requestSeq++
		requestID := field.requestSeq
		controller := NewGridAbortController()
		field.activeAbort = controller
		snapshot := formSnapshotFromState(formID, state)
		fieldSnap := formFieldSnapshot(
			formID, cfg.FieldID, field)
		validators := slices.Clone(field.asyncVals)
		signal := controller.Signal
		fieldID := cfg.FieldID
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
	w.UpdateWindow()
}

// FormRequestSubmit triggers a submit request for the form.
func FormRequestSubmit(w *Window, formID string) {
	if formID == "" {
		return
	}
	state := formRuntime(w, formID)
	state.submitReq = true
	w.UpdateWindow()
}

// FormRequestReset triggers a reset request for the form.
func FormRequestReset(w *Window, formID string) {
	if formID == "" {
		return
	}
	state := formRuntime(w, formID)
	state.resetReq = true
	w.UpdateWindow()
}

// FormRequestSubmitForLayout finds the ancestor form and
// requests submit if SubmitOnEnter is enabled.
func FormRequestSubmitForLayout(w *Window, layout *Layout) {
	if layout == nil || layout.Shape == nil {
		return
	}
	formID := FormFindAncestorID(layout)
	if formID == "" {
		return
	}
	state := formRuntimeRead(w, formID)
	if state == nil || !state.submitOnEnter {
		return
	}
	FormRequestSubmit(w, formID)
}

// FormSummary returns the aggregate validation state.
func (w *Window) FormSummary(formID string) FormSummaryState {
	state := formRuntimeRead(w, formID)
	if state == nil {
		return FormSummaryState{Valid: true}
	}
	return formComputeSummary(state)
}

// FormPendingState returns which fields have pending async
// validation.
func (w *Window) FormPendingState(
	formID string,
) FormPendingState {
	state := formRuntimeRead(w, formID)
	if state == nil {
		return FormPendingState{FormID: formID}
	}
	return formComputePending(formID, state)
}

// FormFieldState returns the public state of a single field.
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

// FormFieldErrors returns validation issues for a single field.
func (w *Window) FormFieldErrors(
	formID, fieldID string,
) []FormIssue {
	fs, ok := w.FormFieldState(formID, fieldID)
	if !ok {
		return nil
	}
	return fs.Errors
}

// FormSubmit requests form submit and triggers a window update.
func (w *Window) FormSubmit(formID string) {
	FormRequestSubmit(w, formID)
}

// FormReset requests form reset and triggers a window update.
func (w *Window) FormReset(formID string) {
	FormRequestReset(w, formID)
}
