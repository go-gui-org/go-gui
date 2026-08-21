package gui

import (
	"slices"
	"strings"
)

// ---------- public enum types ----------

// FormValidateOn controls when field validation triggers.
// exportaudit:keep — caller-facing config (issue #372)
type FormValidateOn uint8

// FormValidateOn values.
const (
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateInherit FormValidateOn = iota
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateOnChange // every keystroke
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateOnBlur // field loses focus
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateOnBlurSubmit // blur or submit
	// exportaudit:keep — caller-facing config (issue #372)
	FormValidateOnSubmit // submit only
)

// FormIssueKind distinguishes error from warning.
type formIssueKind uint8

// FormValidationTrigger indicates which user action triggered
// validation.
// exportaudit:keep — reachable from an exported signature
type FormValidationTrigger uint8

// FormValidationTrigger values.
const (
	FormTriggerChange FormValidationTrigger = iota
	FormTriggerBlur
	formTriggerSubmit
)

// ---------- public data types ----------

// FormIssue is a single validation issue for a field.
type FormIssue struct {
	// exportaudit:keep — json-tagged or same-named member
	Code string
	Msg  string
	Kind formIssueKind
}

// FormFieldSnapshot is a read-only snapshot of one field,
// passed to validators.
type FormFieldSnapshot struct {
	formID  string
	FieldID string
	Value   string
	Touched bool
	Dirty   bool
}

// FormFieldState is the public view of a field's runtime state.
// exportaudit:keep — reachable from an exported signature
type FormFieldState struct {
	Value string
	// InitialValue is the pristine value dirty-state is measured
	// against.
	// exportaudit:keep — caller-facing state (issue #372)
	InitialValue string
	// exportaudit:keep — field name collides with datagrid.Errors
	Errors  []FormIssue
	Touched bool
	Dirty   bool
	Pending bool
}

// FormSnapshot is a read-only snapshot of the entire form,
// passed to validators.
type FormSnapshot struct {
	Values map[string]string
	Fields map[string]FormFieldState
	formID string
}

// FormSummaryState aggregates validation state across all
// fields.
// exportaudit:keep — reachable from an exported signature
type FormSummaryState struct {
	issues       map[string][]FormIssue
	InvalidCount int
	PendingCount int
	Valid        bool
	Pending      bool
}

// FormPendingState lists fields with pending async validation.
// exportaudit:keep — reachable from an exported signature
type FormPendingState struct {
	formID       string
	FieldIDs     []string
	PendingCount int
}

// FormSubmitEvent is delivered to OnSubmit handlers.
type FormSubmitEvent struct {
	State   FormSummaryState
	Values  map[string]string
	formID  string
	Valid   bool
	Pending bool
}

// FormResetEvent is delivered to OnReset handlers.
type FormResetEvent struct {
	Values map[string]string
	formID string
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
	FieldID string
	Value   string
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
	OnReset  func(FormResetEvent, EventCtx)
	// ErrorSlot replaces the per-field error line renderer.
	// exportaudit:keep — caller-facing config (issue #372)
	ErrorSlot func(string, []FormIssue) View
	// SummarySlot replaces the validation summary renderer.
	// exportaudit:keep — caller-facing config (issue #372)
	SummarySlot func(FormSummaryState) View
	// PendingSlot replaces the async-pending renderer.
	// exportaudit:keep — caller-facing config (issue #372)
	PendingSlot func(FormPendingState) View

	// Identity — required for validation runtime.
	ID string `gui:"required"`

	Content    []View
	Padding    Padding
	Spacing    Opt[float32]
	SizeBorder Opt[float32]
	Radius     Opt[float32]

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
	// NoSubmitOnEnter disables enter-to-submit.
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
}

// ---------- formView ----------

type formView struct {
	cfg     FormCfg
	content []View
}

// Form creates a form container with runtime validation and
// submit/reset semantics.
func Form(cfg FormCfg) View {
	RequireID("Form", cfg.ID)
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
	onSubmit := cfg.OnSubmit
	onReset := cfg.OnReset
	formApplyCfg(w, formID, cfg)

	summary := w.FormSummary(formID)
	pending := w.FormPendingState(formID)
	children := make([]View, len(fv.content), len(fv.content)+3)
	copy(children, fv.content)

	if cfg.ErrorSlot != nil {
		fieldIDs := make([]string, 0, len(summary.issues))
		for fid := range summary.issues {
			fieldIDs = append(fieldIDs, fid)
		}
		slices.Sort(fieldIDs)
		for _, fid := range fieldIDs {
			children = append(children, cfg.ErrorSlot(fid, summary.issues[fid]))
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
		AmendLayout: func(ctx EventCtx) {
			formCleanupStale(ctx.Window, formID)
			formProcessRequests(ctx.Window, formID, onSubmit, onReset)
		},
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
