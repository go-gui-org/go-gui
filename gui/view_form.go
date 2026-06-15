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
	RequireID("Form", cfg.ID)
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
