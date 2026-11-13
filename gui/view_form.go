package gui

import (
	"slices"
	"strings"
)

// ---------- public enum types ----------

// FormValidateOn controls when a field's validators run. Submit runs
// them in every mode.
// exportaudit:keep — caller-facing config (issue #372)
type FormValidateOn uint8

// FormValidateOn values.
const (
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateInherit FormValidateOn = iota // take the form's mode
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateOnChange // each edit, on blur and on submit
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateOnBlur // on blur and on submit; same as FormValidateOnBlurSubmit
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateOnBlurSubmit // on blur and on submit (the default)
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateOnSubmit // on submit only
)

// FormIssueKind distinguishes error from warning.
type FormIssueKind uint8

// FormIssueKind values. A validator sets the kind. An error makes its
// field invalid and blocks submit. A warning is reported in
// FormSummaryState.Issues and FormFieldState.Errors, but leaves the
// field valid.
const (
	FormIssueError FormIssueKind = iota
	FormIssueWarning
)

// FormValidationTrigger indicates which user action triggered
// validation. Only the deprecated FormOnFieldEvent takes it: the form
// detects change and blur itself.
type FormValidationTrigger uint8

// FormValidationTrigger values.
const (
	FormTriggerChange FormValidationTrigger = iota
	FormTriggerBlur
	FormTriggerSubmit
)

// ---------- public data types ----------

// FormIssue is one validation result for a field, as a validator
// returns it.
type FormIssue struct {
	// Code is a name for the issue that code can test, chosen by the
	// app ("EMAIL_REQUIRED"). go-gui does not read it.
	// exportaudit:keep — json-tagged or same-named member
	Code string
	// Msg is the text to show the user.
	Msg string
	// Kind tells an error, which blocks submit, from a warning, which
	// does not. The zero value is FormIssueError.
	Kind FormIssueKind
}

// FormFieldSnapshot is a read-only copy of one field, passed to
// validators.
type FormFieldSnapshot struct {
	FormID  string
	FieldID string
	Value   string
	// Touched is true once focus left the field's widget or the form
	// was submitted.
	Touched bool
	// Dirty is true while Value differs from the initial value.
	Dirty bool
}

// FormFieldState is a field's state as the form holds it, read with
// (*Window).FormFieldState. It reflects the last validation; the form
// redraws the window when that changes.
type FormFieldState struct {
	// Value is the value registered in the latest frame.
	Value string
	// InitialValue is the pristine value dirty-state is measured
	// against.
	// exportaudit:keep — caller-facing state (issue #372)
	InitialValue string
	// Errors holds the issues of the last validation, warnings
	// included. Despite the name, check FormIssue.Kind to tell them
	// apart.
	// exportaudit:keep — field name collides with datagrid.Errors
	Errors []FormIssue
	// Touched is true once focus left the field's widget or the form
	// was submitted. Reset clears it.
	Touched bool
	// Dirty is true while Value differs from InitialValue.
	Dirty bool
	// Pending is true while the field's async validators run.
	Pending bool
}

// FormSnapshot is a read-only copy of the whole form, passed to
// validators. A rule that compares fields reads the other values from
// Values.
type FormSnapshot struct {
	// Values maps each registered field ID to its value.
	Values map[string]string
	// Fields maps each registered field ID to its state.
	Fields map[string]FormFieldState
	FormID string
}

// FormSummaryState aggregates validation state across all
// fields. Issues holds every field's issues, warnings included.
// InvalidCount and Valid count only fields with an error.
type FormSummaryState struct {
	// Issues maps each field that has issues to them. It is nil when
	// no field has any.
	Issues map[string][]FormIssue
	// InvalidCount is the number of fields with at least one error.
	InvalidCount int
	// PendingCount is the number of fields whose async validators run.
	PendingCount int
	// Valid is true when no field has an error and none is pending.
	Valid bool
	// Pending is true while any field's async validators run.
	Pending bool
}

// FormPendingState lists the fields whose async validators run.
type FormPendingState struct {
	FormID string
	// FieldIDs is sorted, so a "checking …" line does not reorder.
	FieldIDs     []string
	PendingCount int
}

// FormSubmitEvent is delivered to OnSubmit handlers. By default
// OnSubmit runs only for a valid form with nothing pending; with
// AllowInvalidSubmit or AllowPendingSubmit, check Valid and Pending.
type FormSubmitEvent struct {
	// State is the form's summary right after submit validated every
	// field.
	State FormSummaryState
	// Values maps each field ID to its value.
	Values  map[string]string
	FormID  string
	Valid   bool
	Pending bool
}

// FormResetEvent is delivered to OnReset handlers. Values holds each
// field's initial value. go-gui cannot change the app's state, so the
// handler copies these values into it. Otherwise the next frame
// registers the old values again and the reset does not show.
type FormResetEvent struct {
	Values map[string]string
	FormID string
}

// ---------- validator function types ----------

// FormSyncValidator checks a field and returns its issues, or nil when
// the value is fine. It runs on the UI thread while the form handles
// an edit, a blur or a submit, so it must be quick and must not call
// window functions.
type FormSyncValidator func(FormFieldSnapshot, FormSnapshot) []FormIssue

// FormAsyncValidator checks a field in the background, for a slow
// check such as asking a server. It runs on its own goroutine with
// copies taken when it started, and the field is Pending until it
// returns. A newer validation of the field aborts it: check
// signal.IsAborted() between slow steps and return early. It must not
// touch the window; the form applies its result on the UI thread and
// redraws.
type FormAsyncValidator func(
	FormFieldSnapshot, FormSnapshot, *GridAbortSignal,
) []FormIssue

// ---------- FormFieldAdapterCfg ----------

// FormFieldAdapterCfg configures how a field integrates with
// an ancestor form. Register it every frame with
// FormRegisterFieldByID. The form then detects edits and blur on the
// field's widget itself.
type FormFieldAdapterCfg struct {
	FieldID string
	// WidgetID is the ID of the widget that edits this field, as
	// written in its Cfg. Empty means the widget's ID is FieldID. The
	// form watches focus on this widget: an edit while it has focus is
	// a change, and focus leaving it is a blur.
	WidgetID string
	Value    string
	// InitialValue is the pristine value that "dirty" state is
	// measured against.
	// exportaudit:keep — caller-facing config (issue #372)
	InitialValue    string
	SyncValidators  []FormSyncValidator
	AsyncValidators []FormAsyncValidator
	// HasInitialValue marks InitialValue as set (an empty string is
	// a real value).
	// exportaudit:keep — caller-facing config (issue #372)
	HasInitialValue bool
	// ValidateOnOverride overrides the form's validate-on setting
	// for this field. Zero (FormValidateInherit) defers to the form.
	// exportaudit:keep — caller-facing config (issue #372)
	ValidateOnOverride FormValidateOn
}

// ---------- FormCfg ----------

const formLayoutIDPrefix = "form:"

// FormCfg configures a Form container with runtime validation
// and submit/reset semantics.
type FormCfg struct {

	// Callbacks.
	OnSubmit func(FormSubmitEvent, EventCtx)
	// OnReset runs after a reset. It must copy the event's Values into
	// the app's state; see FormResetEvent.
	OnReset func(FormResetEvent, EventCtx)
	// ErrorSlot, when set, is called once per field that has issues,
	// warnings included. The form appends what it returns after
	// Content. Without it the form shows no issues; the app can still
	// read them with FormFieldErrors.
	// exportaudit:keep — caller-facing config (issue #372)
	ErrorSlot func(string, []FormIssue) View
	// SummarySlot, when set, is called every frame with the form's
	// summary. The form appends what it returns after Content.
	// exportaudit:keep — caller-facing config (issue #372)
	SummarySlot func(FormSummaryState) View
	// PendingSlot, when set, is called every frame with the fields
	// whose async validators are running, an empty list included. The
	// form appends what it returns after Content.
	// exportaudit:keep — caller-facing config (issue #372)
	PendingSlot func(FormPendingState) View

	// ID names the form for FormRegisterFieldByID, FormRequestSubmit
	// and the other Form functions. Required: a form without an ID
	// cannot register fields or submit. It builds as a plain column,
	// and DebugMissingIDs reports it.
	ID string `gui:"required"`

	Content    []View
	Padding    Padding
	Spacing    Spacing
	SizeBorder Border
	Radius     Radius

	Width, Height, MinWidth, MaxWidth, MinHeight, MaxHeight float32
	Color                                                   Color
	ColorBorder                                             Color

	// Container passthrough.
	Sizing Sizing

	// Validation behaviour.
	// ValidateOn controls when field validation triggers. Zero
	// (FormValidateInherit) takes the form-level default.
	// exportaudit:keep — caller-facing config (issue #372)
	ValidateOn FormValidateOn // 0 → BlurSubmit
	// NoSubmitOnEnter stops Enter from submitting the form. By
	// default, Enter in a single-line Input inside the form requests
	// submit, unless the Input's OnEnter consumed the key.
	// exportaudit:keep — caller-facing config (issue #372)
	NoSubmitOnEnter bool
	// AllowInvalidSubmit permits submit with errors.
	// exportaudit:keep — caller-facing config (issue #372)
	AllowInvalidSubmit bool
	// AllowPendingSubmit permits submit while async pending.
	// exportaudit:keep — caller-facing config (issue #372)
	AllowPendingSubmit bool
	Disabled           bool
	Invisible          bool

	// Sound overrides the theme's success cue for a submit this form
	// accepts. SoundNone (the zero value) takes Theme.Sounds.Success,
	// which is itself silent unless the app opted in. A submit blocked
	// by validation or by a pending validator always takes the theme's
	// Error role: Sound names the acceptance, not the refusal
	// (issue #469).
	// exportaudit:keep — caller-facing config (issue #469)
	Sound SoundCue

	// SoundDisabled suppresses both submit cues regardless of the
	// theme and of Sound above.
	// exportaudit:keep — caller-facing config (issue #469)
	SoundDisabled bool
}

// ---------- formView ----------

type formView struct {
	cfg     FormCfg
	content []View
}

// Form creates a form container with runtime validation and
// submit/reset semantics. Each frame it detects edits and blur on its
// registered fields' widgets, and handles submit and reset requests.
func Form(cfg FormCfg) View {
	// A form is a full-width block whose height follows its fields;
	// the zero Sizing (FitFit) shrink-wraps it to its widest label
	// row, which no caller has meant so far.
	cfg.Sizing = cfg.Sizing.Or(FillFit)
	content := make([]View, len(cfg.Content))
	copy(content, cfg.Content)
	return &formView{cfg: cfg, content: content}
}

func (fv *formView) GenerateLayout(w *Window) Layout {
	cfg := fv.cfg
	formID := cfg.ID
	if formID == "" {
		// Nothing can register a field with an ID-less form or submit
		// it (#1257). Show the content and say why it does nothing.
		w.debugWarn(debugCheckFormNoID, "Form",
			"Form has no ID: its fields cannot register and it cannot "+
				"submit. Set FormCfg.ID.")
		layout := generateViewLayout(formColumn(cfg, "", nil), w)
		appendChildViews(w, &layout, fv.content)
		return layout
	}
	onSubmit := cfg.OnSubmit
	onReset := cfg.OnReset
	// Resolved here, at generation time, so the AmendLayout closure
	// below captures two SoundCue values rather than anything the
	// layout arena pools (issue #468's rule, issue #469's site).
	cues := resolveSoundCues(guiTheme.Sounds.Success, cfg.Sound, cfg.SoundDisabled)
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

	inner := formColumn(cfg, formLayoutID(formID), func(ctx EventCtx) {
		formCleanupStale(ctx.Window, formID)
		formDetectFieldEvents(ctx.Window, ctx.Layout, formID)
		formProcessRequests(ctx.Window, formID, onSubmit, onReset, cues)
	})

	// generateViewLayout rather than inner.GenerateLayout(w): the node
	// belongs on the one generation path (ensureLayoutShape runs). inner
	// carries no Content, so its own appendChildViews call is a no-op
	// and no scope is pushed for it.
	layout := generateViewLayout(inner, w)
	// Children resolve under the absolute "form:<id>" scope (issue #306);
	// the field registry is unaffected — FieldID never passes through ID
	// resolution.
	appendChildViews(w, &layout, children)
	return layout
}

// formColumn builds the form's own container. id is empty for an
// ID-less form, which gets no amend pass.
func formColumn(cfg FormCfg, id string, amend func(EventCtx)) View {
	return Column(ContainerCfg{
		ID:          id,
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
		AmendLayout: amend,
	})
}

// ---------- internal runtime state ----------

type formFieldRuntime struct {
	activeAbort  *GridAbortController
	value        string
	initialValue string
	// widgetID is the leaf ID of the widget that edits the field;
	// FormFieldAdapterCfg.WidgetID, or FieldID when that is empty.
	widgetID string
	// widgetEff is the effective ID the widget had in the last amend
	// pass, empty when the form did not find it.
	widgetEff string
	// prevValue is the value the last amend pass saw. A different
	// value while the widget has focus is a change.
	prevValue   string
	syncErrors  []FormIssue
	asyncErrors []FormIssue
	syncVals    []FormSyncValidator
	asyncVals   []FormAsyncValidator
	requestSeq  uint64
	seenGen     uint64
	touched     bool
	dirty       bool
	pending     bool
	// focused records whether focus was inside the field's widget at
	// the last amend pass; losing it is a blur.
	focused bool
	// blurPending marks a blur seen at the last pass. The next pass
	// validates it, once the widget's deferred blur commit has landed.
	blurPending bool
	// checked records that the sync validators have run since the
	// field was created or reset. Only checked fields are re-checked
	// when another field changes.
	checked    bool
	validateOn FormValidateOn
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
