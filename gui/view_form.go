package gui

import (
	"slices"
	"strings"
)

// ---------- public enum types ----------

// FormValidateOn controls when field validation triggers.
type FormValidateOn uint8

// FormValidateOn values.
const (
	FormValidateInherit      FormValidateOn = iota
	FormValidateOnChange                    // every keystroke
	FormValidateOnBlur                      // field loses focus
	FormValidateOnBlurSubmit                // blur or submit
	FormValidateOnSubmit                    // submit only
)

// FormIssueKind distinguishes error from warning.
type FormIssueKind uint8

// FormIssueKind values.
const (
	FormIssueError FormIssueKind = iota
	FormIssueWarning
)

// FormValidationTrigger indicates which user action triggered
// validation.
type FormValidationTrigger uint8

// FormValidationTrigger values.
const (
	FormTriggerChange FormValidationTrigger = iota
	FormTriggerBlur
	FormTriggerSubmit
)

// ---------- public data types ----------

// FormIssue is a single validation issue for a field.
type FormIssue struct {
	Code string
	Msg  string
	Kind FormIssueKind
}

// FormFieldSnapshot is a read-only snapshot of one field,
// passed to validators.
type FormFieldSnapshot struct {
	FormID  string
	FieldID string
	Value   string
	Touched bool
	Dirty   bool
}

// FormFieldState is the public view of a field's runtime state.
type FormFieldState struct {
	Value        string
	InitialValue string
	Errors       []FormIssue
	Touched      bool
	Dirty        bool
	Pending      bool
}

// FormSnapshot is a read-only snapshot of the entire form,
// passed to validators.
type FormSnapshot struct {
	Values map[string]string
	Fields map[string]FormFieldState
	FormID string
}

// FormSummaryState aggregates validation state across all
// fields.
type FormSummaryState struct {
	Issues       map[string][]FormIssue
	InvalidCount int
	PendingCount int
	Valid        bool
	Pending      bool
}

// FormPendingState lists fields with pending async validation.
type FormPendingState struct {
	FormID       string
	FieldIDs     []string
	PendingCount int
}

// FormSubmitEvent is delivered to OnSubmit handlers.
type FormSubmitEvent struct {
	State   FormSummaryState
	Values  map[string]string
	FormID  string
	Valid   bool
	Pending bool
}

// FormResetEvent is delivered to OnReset handlers.
type FormResetEvent struct {
	Values map[string]string
	FormID string
}

// ---------- validator function types ----------

// FormSyncValidator returns issues synchronously.
type FormSyncValidator func(FormFieldSnapshot, FormSnapshot) []FormIssue

// FormAsyncValidator returns issues asynchronously. Check
// signal.IsAborted() to detect cancellation.
type FormAsyncValidator func(
	FormFieldSnapshot, FormSnapshot, *GridAbortSignal,
) []FormIssue

// ---------- FormFieldAdapterCfg ----------

// FormFieldAdapterCfg configures how a field integrates with
// an ancestor form.
type FormFieldAdapterCfg struct {
	FieldID            string
	Value              string
	InitialValue       string
	SyncValidators     []FormSyncValidator
	AsyncValidators    []FormAsyncValidator
	HasInitialValue    bool
	ValidateOnOverride FormValidateOn
}

// ---------- FormCfg ----------

const formLayoutIDPrefix = "form:"

// FormCfg configures a Form container with runtime validation
// and submit/reset semantics.
type FormCfg struct {

	// Callbacks.
	OnSubmit    func(FormSubmitEvent, *Window)
	OnReset     func(FormResetEvent, *Window)
	ErrorSlot   func(string, []FormIssue) View
	SummarySlot func(FormSummaryState) View
	PendingSlot func(FormPendingState) View

	// Identity — required for validation runtime.
	ID string `gui:"required"`

	Content    []View
	Padding    Opt[Padding]
	Spacing    Opt[float32]
	SizeBorder Opt[float32]
	Radius     Opt[float32]

	Width, Height, MinWidth, MaxWidth, MinHeight, MaxHeight float32
	Color                                                   Color
	ColorBorder                                             Color

	// Container passthrough.
	Sizing Sizing

	// Validation behaviour.
	ValidateOn         FormValidateOn // 0 → BlurSubmit
	NoSubmitOnEnter    bool           // true disables enter-to-submit
	AllowInvalidSubmit bool           // true permits submit with errors
	AllowPendingSubmit bool           // true permits submit while async pending
	Disabled           bool
	Invisible          bool
}

// ---------- formView ----------

type formView struct {
	cfg     FormCfg
	content []View
}

// Form creates a form container with runtime validation and
// submit/reset semantics.
func Form(cfg FormCfg) View {
	requireID("Form", cfg.ID)
	content := make([]View, len(cfg.Content))
	copy(content, cfg.Content)
	return &formView{cfg: cfg, content: content}
}

func (fv *formView) Content() []View { return fv.content }

func (fv *formView) GenerateLayout(w *Window) Layout {
	cfg := fv.cfg
	formID := cfg.ID
	onSubmit := cfg.OnSubmit
	onReset := cfg.OnReset
	formApplyCfg(w, formID, cfg)

	summary := w.FormSummary(formID)
	pending := w.FormPendingState(formID)
	children := make([]View, len(fv.content), len(fv.content)+3)
	copy(children, fv.content)

	if cfg.ErrorSlot != nil {
		fieldIDs := make([]string, 0, len(summary.Issues))
		for fid := range summary.Issues {
			fieldIDs = append(fieldIDs, fid)
		}
		slices.Sort(fieldIDs)
		for _, fid := range fieldIDs {
			children = append(children, cfg.ErrorSlot(fid, summary.Issues[fid]))
		}
	}
	if cfg.SummarySlot != nil {
		children = append(children, cfg.SummarySlot(summary))
	}
	if cfg.PendingSlot != nil {
		children = append(children, cfg.PendingSlot(pending))
	}

	inner := Column(ContainerCfg{
		ID:          formLayoutID(formID),
		Sizing:      cfg.Sizing,
		Width:       cfg.Width,
		Height:      cfg.Height,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		MinHeight:   cfg.MinHeight,
		MaxHeight:   cfg.MaxHeight,
		Padding:     cfg.Padding,
		Spacing:     cfg.Spacing,
		Color:       cfg.Color,
		SizeBorder:  cfg.SizeBorder,
		ColorBorder: cfg.ColorBorder,
		Radius:      cfg.Radius,
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		AmendLayout: func(_ *Layout, w *Window) {
			formCleanupStale(w, formID)
			formProcessRequests(w, formID, onSubmit, onReset)
		},
	})

	layout := inner.GenerateLayout(w)
	// Clear content so outer generateViewLayout does not
	// double-process children.
	fv.content = fv.content[:0]
	for _, child := range children {
		if child != nil {
			layout.Children = append(
				layout.Children,
				generateViewLayout(child, w),
			)
		}
	}
	return layout
}

// ---------- internal runtime state ----------

type formFieldRuntime struct {
	activeAbort  *GridAbortController
	value        string
	initialValue string
	syncErrors   []FormIssue
	asyncErrors  []FormIssue
	syncVals     []FormSyncValidator
	asyncVals    []FormAsyncValidator
	requestSeq   uint64
	seenGen      uint64
	touched      bool
	dirty        bool
	pending      bool
	validateOn   FormValidateOn
}

type formRuntimeState struct {
	fields        map[string]*formFieldRuntime
	submitReq     bool
	resetReq      bool
	validateOn    FormValidateOn
	submitOnEnter bool
	blockInvalid  bool
	blockPending  bool
	disabled      bool
	layoutGen     uint64
}

// ---------- state access ----------

func formRuntime(w *Window, formID string) *formRuntimeState {
	sm := StateMap[string, *formRuntimeState](w, nsForm, capModerate)
	state, ok := sm.Get(formID)
	if !ok {
		state = &formRuntimeState{
			fields:     make(map[string]*formFieldRuntime),
			validateOn: FormValidateOnBlurSubmit,
		}
		sm.Set(formID, state)
	}
	return state
}

func formRuntimeRead(w *Window, formID string) *formRuntimeState {
	sm := StateMapRead[string, *formRuntimeState](w, nsForm)
	if sm == nil {
		return nil
	}
	state, ok := sm.Get(formID)
	if !ok {
		return nil
	}
	return state
}

// ---------- internal helpers ----------

func formLayoutID(formID string) string {
	return formLayoutIDPrefix + formID
}

func formDecodeLayoutID(layoutID string) string {
	after, ok := strings.CutPrefix(layoutID, formLayoutIDPrefix)
	if ok {
		return after
	}
	return ""
}

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
