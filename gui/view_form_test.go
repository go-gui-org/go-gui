package gui

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestFormEmptyIDPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty Form.ID")
		}
	}()
	Form(FormCfg{ //requiredid:ignore
		Content: []View{
			Text(TextCfg{Text: "hello"}),
		},
	})
}

func TestFormValidateInheritPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unresolved FormValidateInherit")
		}
	}()
	formShouldValidate(formValidateInherit, FormTriggerBlur)
}

func TestFormGeneratesLayout(t *testing.T) {
	w := newTestWindow()
	v := Form(FormCfg{
		ID: "test-form",
		Content: []View{
			Text(TextCfg{Text: "field"}),
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected layout shape")
	}
	want := "form:test-form"
	if layout.Shape.ID != want {
		t.Fatalf("expected ID %q, got %q", want, layout.Shape.ID)
	}
}

func TestFormFindAncestorID(t *testing.T) {
	parent := Layout{
		Shape: &Shape{ID: "form:my-form"},
	}
	child := Layout{
		Shape:  &Shape{ID: "input-1"},
		Parent: &parent,
	}
	grandchild := Layout{
		Shape:  &Shape{ID: "input-2"},
		Parent: &child,
	}
	got := formFindAncestorID(&grandchild)
	if got != "my-form" {
		t.Fatalf("expected my-form, got %q", got)
	}
}

func TestFormFindAncestorIDNotFound(t *testing.T) {
	layout := Layout{Shape: &Shape{ID: "no-form"}}
	got := formFindAncestorID(&layout)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestFormRegisterAndCleanup(t *testing.T) {
	w := newTestWindow()
	formID := "cleanup-form"

	// Gen 0: register field, then cleanup increments to gen 1.
	FormRegisterFieldByID(w, formID, FormFieldAdapterCfg{
		FieldID: "field-a",
		Value:   "hello",
	})
	state := formRuntime(w, formID)
	if len(state.fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(state.fields))
	}
	formCleanupStale(w, formID) // gen 0→1

	// Gen 1: do NOT re-register field-a. Register field-b.
	FormRegisterFieldByID(w, formID, FormFieldAdapterCfg{
		FieldID: "field-b",
		Value:   "world",
	})
	formCleanupStale(w, formID) // gen 1→2; field-a stale

	if _, ok := state.fields["field-a"]; ok {
		t.Fatal("field-a should be cleaned up")
	}
	if _, ok := state.fields["field-b"]; !ok {
		t.Fatal("field-b should still exist")
	}
}

func TestFormSyncValidation(t *testing.T) {
	w := newTestWindow()
	formID := "sync-form"

	required := func(
		f FormFieldSnapshot, _ FormSnapshot,
	) []FormIssue {
		if f.Value == "" {
			return []FormIssue{{Msg: "required"}}
		}
		return nil
	}

	FormRegisterFieldByID(w, formID, FormFieldAdapterCfg{
		FieldID:        "name",
		Value:          "",
		SyncValidators: []FormSyncValidator{required},
	})

	// Trigger blur validation.
	formOnFieldEventByID(w, formID, FormFieldAdapterCfg{
		FieldID:        "name",
		Value:          "",
		SyncValidators: []FormSyncValidator{required},
	}, FormTriggerBlur)

	issues := w.FormFieldErrors(formID, "name")
	if len(issues) != 1 || issues[0].Msg != "required" {
		t.Fatalf("expected 1 required issue, got %v", issues)
	}

	// Fix the value.
	formOnFieldEventByID(w, formID, FormFieldAdapterCfg{
		FieldID:        "name",
		Value:          "Alice",
		SyncValidators: []FormSyncValidator{required},
	}, FormTriggerBlur)

	issues = w.FormFieldErrors(formID, "name")
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %v", issues)
	}
}

func TestFormSubmitBlock(t *testing.T) {
	w := newTestWindow()
	formID := "block-form"

	required := func(
		f FormFieldSnapshot, _ FormSnapshot,
	) []FormIssue {
		if f.Value == "" {
			return []FormIssue{{Msg: "required"}}
		}
		return nil
	}

	FormRegisterFieldByID(w, formID, FormFieldAdapterCfg{
		FieldID:        "name",
		Value:          "",
		SyncValidators: []FormSyncValidator{required},
	})

	var submitted bool
	onSubmit := func(_ FormSubmitEvent, ctx EventCtx) {
		submitted = true
	}

	// Apply form config (block invalid is default).
	formApplyCfg(w, formID, FormCfg{ID: formID})

	// Request and process submit.
	state := formRuntime(w, formID)
	state.submitReq = true
	formProcessRequests(w, formID, onSubmit, nil)

	if submitted {
		t.Fatal("should not submit when field is invalid")
	}

	// Now fix the value and retry.
	FormRegisterFieldByID(w, formID, FormFieldAdapterCfg{
		FieldID:        "name",
		Value:          "Alice",
		SyncValidators: []FormSyncValidator{required},
	})
	state.submitReq = true
	formProcessRequests(w, formID, onSubmit, nil)

	if !submitted {
		t.Fatal("should submit when field is valid")
	}
}

func TestFormReset(t *testing.T) {
	w := newTestWindow()
	formID := "reset-form"

	FormRegisterFieldByID(w, formID, FormFieldAdapterCfg{
		FieldID:         "email",
		Value:           "changed@test.com",
		initialValue:    "init@test.com",
		hasInitialValue: true,
	})

	formApplyCfg(w, formID, FormCfg{ID: formID})

	var resetEvent FormResetEvent
	onReset := func(e FormResetEvent, ctx EventCtx) {
		resetEvent = e
	}

	state := formRuntime(w, formID)
	state.resetReq = true
	formProcessRequests(w, formID, nil, onReset)

	if resetEvent.formID != formID {
		t.Fatalf("expected form ID %q, got %q",
			formID, resetEvent.formID)
	}
	if resetEvent.Values["email"] != "init@test.com" {
		t.Fatalf("expected reset to initial value, got %q",
			resetEvent.Values["email"])
	}

	// Field should be reset.
	fs, ok := w.FormFieldState(formID, "email")
	if !ok {
		t.Fatal("field should exist")
	}
	if fs.Value != "init@test.com" {
		t.Fatalf("expected value init@test.com, got %q", fs.Value)
	}
	if fs.Dirty || fs.Touched {
		t.Fatal("field should not be dirty or touched after reset")
	}
}

func TestFormAsyncValidation(t *testing.T) {
	w := newTestWindow()
	formID := "async-form"

	asyncVal := func(
		f FormFieldSnapshot, _ FormSnapshot,
		_ *GridAbortSignal,
	) []FormIssue {
		if f.Value == "bad" {
			return []FormIssue{{Msg: "bad value"}}
		}
		return nil
	}

	FormRegisterFieldByID(w, formID, FormFieldAdapterCfg{
		FieldID:         "field",
		Value:           "bad",
		AsyncValidators: []FormAsyncValidator{asyncVal},
	})

	formOnFieldEventByID(w, formID, FormFieldAdapterCfg{
		FieldID:         "field",
		Value:           "bad",
		AsyncValidators: []FormAsyncValidator{asyncVal},
	}, FormTriggerBlur)

	// Field should be pending.
	fs, _ := w.FormFieldState(formID, "field")
	if !fs.Pending {
		t.Fatal("field should be pending")
	}

	// Wait for goroutine to queue its command.
	var cmds []queuedCommand
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runtime.Gosched()
		w.commandsMu.Lock()
		cmds = append(cmds, w.commands...)
		w.commands = nil
		w.commandsMu.Unlock()
		if len(cmds) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	for _, cmd := range cmds {
		if cmd.windowFn != nil {
			cmd.windowFn(w)
		}
	}

	fs, _ = w.FormFieldState(formID, "field")
	if fs.Pending {
		t.Fatal("field should not be pending after async completes")
	}
	if len(fs.Errors) != 1 || fs.Errors[0].Msg != "bad value" {
		t.Fatalf("expected async error, got %v", fs.Errors)
	}
}

func TestFormAbortOnRevalidation(t *testing.T) {
	w := newTestWindow()
	formID := "abort-form"

	var callCount atomic.Int32
	slow := func(
		_ FormFieldSnapshot, _ FormSnapshot,
		_ *GridAbortSignal,
	) []FormIssue {
		callCount.Add(1)
		return nil
	}

	cfg := FormFieldAdapterCfg{
		FieldID:         "f",
		Value:           "a",
		AsyncValidators: []FormAsyncValidator{slow},
	}

	FormRegisterFieldByID(w, formID, cfg)
	formOnFieldEventByID(w, formID, cfg, FormTriggerBlur)

	// Capture the abort controller.
	state := formRuntime(w, formID)
	field := state.fields["f"]
	firstAbort := field.activeAbort
	if firstAbort == nil {
		t.Fatal("expected active abort controller")
	}

	// Re-trigger; previous should be aborted.
	cfg.Value = "b"
	formOnFieldEventByID(w, formID, cfg, FormTriggerBlur)

	if !firstAbort.Signal.IsAborted() {
		t.Fatal("first async should be aborted")
	}
}

func TestFormShouldValidate(t *testing.T) {
	tests := []struct {
		mode    formValidateOn
		trigger FormValidationTrigger
		want    bool
	}{
		{formValidateOnChange, FormTriggerChange, true},
		{formValidateOnChange, FormTriggerBlur, true},
		{formValidateOnBlur, FormTriggerChange, false},
		{formValidateOnBlur, FormTriggerBlur, true},
		{formValidateOnBlur, formTriggerSubmit, true},
		{formValidateOnSubmit, FormTriggerChange, false},
		{formValidateOnSubmit, FormTriggerBlur, false},
		{formValidateOnSubmit, formTriggerSubmit, true},
		{formValidateOnBlurSubmit, FormTriggerChange, false},
		{formValidateOnBlurSubmit, FormTriggerBlur, true},
	}
	for _, tt := range tests {
		got := formShouldValidate(tt.mode, tt.trigger)
		if got != tt.want {
			t.Errorf("formShouldValidate(%d, %d) = %v, want %v",
				tt.mode, tt.trigger, got, tt.want)
		}
	}
}

// --- Layout-walking variants -----------------------------------------
//
// Only the *ByID entry points were covered; these are the ones widgets
// actually call from AmendLayout, where the form ID must be recovered
// from the parent chain.

// formLayoutFixture builds a form -> child parent chain and returns the
// child, which is what a widget's AmendLayout would hold.
func formLayoutFixture(formID string) *Layout {
	parent := &Layout{Shape: &Shape{ID: formLayoutID(formID)}}
	return &Layout{Shape: &Shape{ID: "input-1"}, Parent: parent}
}

func TestFormRegisterFieldViaLayout(t *testing.T) {
	w := newTestWindow()
	child := formLayoutFixture("layout-form")

	formRegisterField(w, child, FormFieldAdapterCfg{
		FieldID: "email",
		Value:   "a@b.c",
	})

	fs, ok := w.FormFieldState("layout-form", "email")
	if !ok {
		t.Fatal("field not registered against the ancestor form")
	}
	if fs.Value != "a@b.c" {
		t.Errorf("value = %q, want %q", fs.Value, "a@b.c")
	}
}

func TestFormRegisterFieldNoOps(t *testing.T) {
	// Each of these must leave the form state untouched rather than
	// panic — widgets call this unconditionally from AmendLayout.
	cases := []struct {
		name   string
		layout *Layout
		cfg    FormFieldAdapterCfg
	}{
		{"nil layout", nil, FormFieldAdapterCfg{FieldID: "f"}},
		{"empty field id", formLayoutFixture("f1"), FormFieldAdapterCfg{}},
		{
			"no ancestor form",
			&Layout{Shape: &Shape{ID: "loose-input"}},
			FormFieldAdapterCfg{FieldID: "f"},
		},
	}
	for _, tc := range cases {
		w := newTestWindow()
		formRegisterField(w, tc.layout, tc.cfg)
		if state := formRuntimeRead(w, "f1"); state != nil && len(state.fields) > 0 {
			t.Errorf("%s: expected no field registration, got %d",
				tc.name, len(state.fields))
		}
	}
}

func TestFormOnFieldEventViaLayout(t *testing.T) {
	w := newTestWindow()
	child := formLayoutFixture("evt-form")

	required := func(f FormFieldSnapshot, _ FormSnapshot) []FormIssue {
		if f.Value == "" {
			return []FormIssue{{Msg: "required"}}
		}
		return nil
	}
	cfg := FormFieldAdapterCfg{
		FieldID:        "name",
		Value:          "",
		SyncValidators: []FormSyncValidator{required},
	}

	formRegisterField(w, child, cfg)
	FormOnFieldEvent(w, child, cfg, FormTriggerBlur)

	issues := w.FormFieldErrors("evt-form", "name")
	if len(issues) != 1 || issues[0].Msg != "required" {
		t.Fatalf("expected the sync validator to run, got %v", issues)
	}

	fs, _ := w.FormFieldState("evt-form", "name")
	if !fs.Touched {
		t.Error("blur should mark the field touched")
	}
}

func TestFormOnFieldEventNoOps(t *testing.T) {
	w := newTestWindow()
	// Empty FieldID and a layout with no ancestor form both return
	// before touching form state.
	FormOnFieldEvent(w, formLayoutFixture("f1"), FormFieldAdapterCfg{},
		FormTriggerBlur)
	FormOnFieldEvent(w, &Layout{Shape: &Shape{ID: "loose"}},
		FormFieldAdapterCfg{FieldID: "f"}, FormTriggerBlur)

	if state := formRuntimeRead(w, "f1"); state != nil && len(state.fields) > 0 {
		t.Errorf("expected no fields, got %d", len(state.fields))
	}
}

// --- Submit / reset requests -----------------------------------------

func TestFormRequestSubmitSetsFlag(t *testing.T) {
	w := newTestWindow()
	FormRequestSubmit(w, "req-form")

	state := formRuntimeRead(w, "req-form")
	if state == nil {
		t.Fatal("expected form state to be created lazily")
	}
	if !state.submitReq {
		t.Error("submitReq not set")
	}
}

func TestFormRequestResetSetsFlag(t *testing.T) {
	w := newTestWindow()
	FormRequestReset(w, "req-form")

	state := formRuntimeRead(w, "req-form")
	if state == nil {
		t.Fatal("expected form state to be created lazily")
	}
	if !state.resetReq {
		t.Error("resetReq not set")
	}
}

func TestFormRequestEmptyIDNoOp(t *testing.T) {
	// An empty ID must not create a state entry under "".
	w := newTestWindow()
	FormRequestSubmit(w, "")
	FormRequestReset(w, "")
	if state := formRuntimeRead(w, ""); state != nil {
		t.Error("empty form ID should not create form state")
	}
}

// FormSubmit/FormReset are aliases; assert they hit the same flags so a
// future divergence is caught.
func TestFormSubmitResetMethodAliases(t *testing.T) {
	w := newTestWindow()
	w.formSubmit("alias-form")
	w.formReset("alias-form")

	state := formRuntimeRead(w, "alias-form")
	if state == nil {
		t.Fatal("expected form state")
	}
	if !state.submitReq {
		t.Error("FormSubmit did not set submitReq")
	}
	if !state.resetReq {
		t.Error("FormReset did not set resetReq")
	}
}

// --- FormRequestSubmitForLayout --------------------------------------

func TestFormRequestSubmitForLayoutSuccess(t *testing.T) {
	w := newTestWindow()
	child := formLayoutFixture("enter-form")
	formApplyCfg(w, "enter-form", FormCfg{ID: "enter-form"}) // submitOnEnter defaults on

	formRequestSubmitForLayout(w, child)

	state := formRuntimeRead(w, "enter-form")
	if state == nil || !state.submitReq {
		t.Fatal("Enter on a field should have requested submit")
	}
}

func TestFormRequestSubmitForLayoutSuppressedWhenDisabled(t *testing.T) {
	w := newTestWindow()
	child := formLayoutFixture("no-enter-form")
	formApplyCfg(w, "no-enter-form", FormCfg{
		ID:              "no-enter-form",
		noSubmitOnEnter: true,
	})

	formRequestSubmitForLayout(w, child)

	state := formRuntimeRead(w, "no-enter-form")
	if state == nil {
		t.Fatal("expected existing form state")
	}
	if state.submitReq {
		t.Error("NoSubmitOnEnter should suppress the submit request")
	}
}

func TestFormRequestSubmitForLayoutEarlyReturns(t *testing.T) {
	// nil layout, nil Shape and no-ancestor all bail before touching
	// state. The un-applied-cfg case bails on formRuntimeRead == nil:
	// a form that never rendered has no state, so Enter does nothing
	// even though the layout chain resolves.
	w := newTestWindow()
	formRequestSubmitForLayout(w, nil)
	formRequestSubmitForLayout(w, &Layout{})
	formRequestSubmitForLayout(w, &Layout{Shape: &Shape{ID: "loose"}})
	formRequestSubmitForLayout(w, formLayoutFixture("never-rendered"))

	if state := formRuntimeRead(w, "never-rendered"); state != nil {
		t.Error("expected no form state for a form that never applied its cfg")
	}
}

// --- Summary / pending -------------------------------------------------

func TestFormSummaryUnknownFormIsValid(t *testing.T) {
	// An unknown form reads as valid so callers can gate a submit
	// button before the form has rendered.
	w := newTestWindow()
	got := w.FormSummary("nope")
	if !got.Valid {
		t.Error("unknown form should read as valid")
	}
	if got.Pending || got.InvalidCount != 0 || got.PendingCount != 0 {
		t.Errorf("unknown form summary = %+v, want zero counts", got)
	}
	if got.issues != nil {
		t.Errorf("Issues = %v, want nil", got.issues)
	}
}

func TestFormSummaryIssuesNilWhenClean(t *testing.T) {
	// The Issues map is allocated lazily — a clean form must not pay
	// for it.
	w := newTestWindow()
	FormRegisterFieldByID(w, "clean", FormFieldAdapterCfg{
		FieldID: "a", Value: "ok",
	})

	got := w.FormSummary("clean")
	if !got.Valid {
		t.Error("form with no errors should be valid")
	}
	if got.issues != nil {
		t.Errorf("Issues = %v, want nil for a clean form", got.issues)
	}
}

func TestFormSummaryCountsInvalidFields(t *testing.T) {
	w := newTestWindow()
	formID := "sum-form"
	required := func(f FormFieldSnapshot, _ FormSnapshot) []FormIssue {
		if f.Value == "" {
			return []FormIssue{{Msg: "required"}}
		}
		return nil
	}
	for _, id := range []string{"a", "b"} {
		cfg := FormFieldAdapterCfg{
			FieldID:        id,
			Value:          "",
			SyncValidators: []FormSyncValidator{required},
		}
		FormRegisterFieldByID(w, formID, cfg)
		formOnFieldEventByID(w, formID, cfg, FormTriggerBlur)
	}

	got := w.FormSummary(formID)
	if got.Valid {
		t.Error("form with two failing fields should be invalid")
	}
	if got.InvalidCount != 2 {
		t.Errorf("InvalidCount = %d, want 2", got.InvalidCount)
	}
	if len(got.issues) != 2 {
		t.Errorf("Issues has %d entries, want 2", len(got.issues))
	}
}

func TestFormPendingStateUnknownForm(t *testing.T) {
	w := newTestWindow()
	got := w.FormPendingState("nope")
	if got.formID != "nope" {
		t.Errorf("FormID = %q, want %q", got.formID, "nope")
	}
	if got.FieldIDs != nil {
		t.Errorf("FieldIDs = %v, want nil", got.FieldIDs)
	}
	if got.PendingCount != 0 {
		t.Errorf("PendingCount = %d, want 0", got.PendingCount)
	}
}

func TestFormPendingStateSortsFieldIDs(t *testing.T) {
	// Map iteration is random; the ID list must come back sorted so
	// callers can render a stable "validating…" list.
	w := newTestWindow()
	formID := "pending-form"
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })

	slowVal := func(
		_ FormFieldSnapshot, _ FormSnapshot, _ *GridAbortSignal,
	) []FormIssue {
		<-block
		return nil
	}

	for _, id := range []string{"zeta", "alpha", "mid"} {
		cfg := FormFieldAdapterCfg{
			FieldID:         id,
			Value:           "v",
			AsyncValidators: []FormAsyncValidator{slowVal},
		}
		FormRegisterFieldByID(w, formID, cfg)
		formOnFieldEventByID(w, formID, cfg, FormTriggerBlur)
	}

	got := w.FormPendingState(formID)
	if got.PendingCount != 3 {
		t.Fatalf("PendingCount = %d, want 3", got.PendingCount)
	}
	want := []string{"alpha", "mid", "zeta"}
	for i, id := range want {
		if got.FieldIDs[i] != id {
			t.Errorf("FieldIDs[%d] = %q, want %q", i, got.FieldIDs[i], id)
		}
	}
}

// --- Stale async results ---------------------------------------------

// Every revalidation bumps requestSeq. A result carrying an older
// requestID belongs to an aborted run and must be dropped, or a slow
// stale validator would clobber the current field state.
func TestFormApplyAsyncResultDropsStaleRequest(t *testing.T) {
	w := newTestWindow()
	formID := "stale-form"

	cfg := FormFieldAdapterCfg{
		FieldID: "f",
		Value:   "v",
		AsyncValidators: []FormAsyncValidator{
			func(_ FormFieldSnapshot, _ FormSnapshot, _ *GridAbortSignal) []FormIssue {
				return nil
			},
		},
	}
	FormRegisterFieldByID(w, formID, cfg)
	formOnFieldEventByID(w, formID, cfg, FormTriggerBlur)

	state := formRuntimeRead(w, formID)
	field := state.fields["f"]
	current := field.requestSeq

	formApplyAsyncResult(w, formID, "f", current-1,
		[]FormIssue{{Msg: "stale"}})

	if len(field.asyncErrors) != 0 {
		t.Errorf("stale result applied: asyncErrors = %v", field.asyncErrors)
	}
	if !field.pending {
		t.Error("stale result cleared the pending flag")
	}

	// The in-flight result for the current request still lands.
	formApplyAsyncResult(w, formID, "f", current,
		[]FormIssue{{Msg: "fresh"}})
	if len(field.asyncErrors) != 1 || field.asyncErrors[0].Msg != "fresh" {
		t.Errorf("current result not applied: %v", field.asyncErrors)
	}
	if field.pending {
		t.Error("current result should have cleared pending")
	}
}

func TestFormApplyAsyncResultUnknownTargets(t *testing.T) {
	// Unknown form and unknown field both return early rather than
	// creating state — an async result can outlive its form.
	w := newTestWindow()
	formApplyAsyncResult(w, "ghost", "f", 1, []FormIssue{{Msg: "x"}})
	if formRuntimeRead(w, "ghost") != nil {
		t.Error("async result should not create form state")
	}

	FormRegisterFieldByID(w, "real", FormFieldAdapterCfg{FieldID: "a", Value: "v"})
	formApplyAsyncResult(w, "real", "missing", 1, []FormIssue{{Msg: "x"}})
	if _, ok := w.FormFieldState("real", "missing"); ok {
		t.Error("async result should not create a field")
	}
}

// FormFieldErrors returns the underlying syncErrors/asyncErrors slice
// whenever one side is empty (formMergeErrors takes that shortcut), and
// those slices are truncated with [:0] and re-appended in place on the
// next validation. A retained result therefore aliases live state.
// Callers must clone before holding across a frame; this test documents
// the contract so it is not mistaken for a defensive copy.
func TestFormFieldErrorsAliasesRuntimeSlice(t *testing.T) {
	w := newTestWindow()
	formID := "alias-errors"

	msg := "first"
	validator := func(_ FormFieldSnapshot, _ FormSnapshot) []FormIssue {
		return []FormIssue{{Msg: msg}}
	}
	cfg := FormFieldAdapterCfg{
		FieldID:        "f",
		Value:          "v",
		SyncValidators: []FormSyncValidator{validator},
	}

	FormRegisterFieldByID(w, formID, cfg)
	formOnFieldEventByID(w, formID, cfg, FormTriggerBlur)

	retained := w.FormFieldErrors(formID, "f")
	if len(retained) != 1 || retained[0].Msg != "first" {
		t.Fatalf("setup: got %v, want one 'first' issue", retained)
	}

	// Revalidate with a different message. The runtime reuses the same
	// backing array, so the retained slice observes the new value.
	msg = "second"
	formOnFieldEventByID(w, formID, cfg, FormTriggerBlur)

	if retained[0].Msg != "second" {
		t.Errorf("retained[0].Msg = %q, want %q — formMergeErrors is "+
			"documented to alias the runtime slice; if this now returns "+
			"a copy, update the doc comment on FormFieldErrors",
			retained[0].Msg, "second")
	}
}

// --- Form child ID scoping ---
//
// Form children generate under the form's *enclosing* scope, not under
// "form:<id>". The form's inner container carries an absolute ID
// ("form:myform", formLayoutID), so a scope push there would resolve
// children as form:myform:email and change every SetFocus/FindByID
// caller that uses the flat name. The tests below pin the flat behavior
// (see docs/specs/view-single-method.md, issue #306).

// TestFormChildrenStayFlat documents that w.EffID resolves a form
// child's leaf against the enclosing scope (empty here), never against
// "form:<id>". The generation-time scope is the seam the form's child
// append path controls.
func TestFormChildrenStayFlat(t *testing.T) {
	w := newTestWindow()
	formID := "flat-form"
	captured := ""
	v := Form(FormCfg{
		ID: formID,
		Content: []View{
			viewFunc(func(w *Window) View {
				captured = w.EffID("email")
				return Input(InputCfg{ID: "email"})
			}),
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != formLayoutID(formID) {
		t.Fatalf("form root ID = %q, want %q",
			layout.Shape.ID, formLayoutID(formID))
	}
	if captured != "email" {
		t.Errorf("w.EffID(\"email\") inside form = %q, want %q",
			captured, "email")
	}
	if len(layout.Children) != 1 {
		t.Fatalf("form children = %d, want 1", len(layout.Children))
	}
}

// TestFormChildrenFlatInsideIDPanel pins the same flatness when the
// form sits inside an ID-bearing panel: "flat" means the form's
// enclosing scope — the panel's, here — not window-global, because
// formLayoutID is absolute.
func TestFormChildrenFlatInsideIDPanel(t *testing.T) {
	w := newTestWindow()
	captured := ""
	form := Form(FormCfg{
		ID: "nested-form",
		Content: []View{
			viewFunc(func(w *Window) View {
				captured = w.EffID("email")
				return Input(InputCfg{ID: "email"})
			}),
		},
	})
	v := Column(ContainerCfg{
		ID:      "panel",
		Content: []View{form},
	})
	generateViewLayout(v, w)
	if captured != "panel:email" {
		t.Errorf("w.EffID(\"email\") inside panel-nested form = %q, want %q",
			captured, "panel:email")
	}
}

// TestFormFieldStateUnaffectedByScoping documents that the form field
// registry keys on FormFieldAdapterCfg.FieldID — a namespace that never
// passes through ID resolution — so no amount of child scoping can
// break field lookup. Registered fields resolve by plain FieldID.
func TestFormFieldStateUnaffectedByScoping(t *testing.T) {
	w := newTestWindow()
	formID := "field-scope-form"
	FormRegisterFieldByID(w, formID, FormFieldAdapterCfg{
		FieldID: "username",
		Value:   "alice",
	})
	v := Form(FormCfg{
		ID: formID,
		Content: []View{
			Input(InputCfg{ID: "showcase-form-username"}),
		},
	})
	generateViewLayout(v, w)
	fs, ok := w.FormFieldState(formID, "username")
	if !ok {
		t.Fatal("FormFieldState should resolve by plain FieldID")
	}
	if fs.Value != "alice" {
		t.Errorf("value = %q, want alice", fs.Value)
	}
}

// TestFormChildrenShareEventCap pins that the form's child append path
// applies the same maxEventChildren cap every container gets. Before
// this refactor the form appended children without the cap; the
// dispatch gate (hasTooManyChildren) refuses >maxEventChildren anyway,
// so the cap here only bounds generation work.
func TestFormChildrenShareEventCap(t *testing.T) {
	w := newTestWindow()
	n := maxEventChildren + 100
	children := make([]View, n)
	for i := range children {
		children[i] = &stubView{id: "child"}
	}
	layout := generateViewLayout(Form(FormCfg{
		ID:      "cap-form",
		Content: children,
	}), w)
	if len(layout.Children) != maxEventChildren {
		t.Errorf("form children = %d, want %d",
			len(layout.Children), maxEventChildren)
	}
}
