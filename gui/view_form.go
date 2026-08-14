package gui

import (
	"slices"
	"strings"
)

// ---------- public enum types ----------

// FormValidateOn controls when field validation triggers.
type formValidateOn uint8

// FormValidateOn values.
const (
	formValidateInherit      formValidateOn = iota
	formValidateOnChange                    // every keystroke
	formValidateOnBlur                      // field loses focus
	formValidateOnBlurSubmit                // blur or submit
	formValidateOnSubmit                    // submit only
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
	Value        string
	initialValue string
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
	FieldID            string
	Value              string
	initialValue       string
	SyncValidators     []FormSyncValidator
	AsyncValidators    []FormAsyncValidator
	hasInitialValue    bool
	validateOnOverride formValidateOn
}

// ---------- FormCfg ----------

const formLayoutIDPrefix = "form:"

// FormCfg configures a Form container with runtime validation
// and submit/reset semantics.
type FormCfg struct {

	// Callbacks.
	OnSubmit    func(FormSubmitEvent, EventCtx)
	OnReset     func(FormResetEvent, EventCtx)
	errorSlot   func(string, []FormIssue) View
	summarySlot func(FormSummaryState) View
	pendingSlot func(FormPendingState) View

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
	validateOn         formValidateOn // 0 → BlurSubmit
	noSubmitOnEnter    bool           // true disables enter-to-submit
	allowInvalidSubmit bool           // true permits submit with errors
	allowPendingSubmit bool           // true permits submit while async pending
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

	if cfg.errorSlot != nil {
		fieldIDs := make([]string, 0, len(summary.issues))
		for fid := range summary.issues {
			fieldIDs = append(fieldIDs, fid)
		}
		slices.Sort(fieldIDs)
		for _, fid := range fieldIDs {
			children = append(children, cfg.errorSlot(fid, summary.issues[fid]))
		}
	}
	if cfg.summarySlot != nil {
		children = append(children, cfg.summarySlot(summary))
	}
	if cfg.pendingSlot != nil {
		children = append(children, cfg.pendingSlot(pending))
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
	// Form children append flat — they resolve in the form's enclosing
	// scope, never under "form:<id>" (appendChildViewsFlat). The field
	// registry is unaffected either way: FieldID never passes through ID
	// resolution. See issue #306.
	appendChildViewsFlat(w, &layout, children)
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
	validateOn   formValidateOn
}

type formRuntimeState struct {
	fields        map[string]*formFieldRuntime
	submitReq     bool
	resetReq      bool
	validateOn    formValidateOn
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
			validateOn: formValidateOnBlurSubmit,
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
