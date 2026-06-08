package gui

import (
	"runtime"
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
	formShouldValidate(FormValidateInherit, FormTriggerBlur)
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
	got := FormFindAncestorID(&grandchild)
	if got != "my-form" {
		t.Fatalf("expected my-form, got %q", got)
	}
}

func TestFormFindAncestorIDNotFound(t *testing.T) {
	layout := Layout{Shape: &Shape{ID: "no-form"}}
	got := FormFindAncestorID(&layout)
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
	FormOnFieldEventByID(w, formID, FormFieldAdapterCfg{
		FieldID:        "name",
		Value:          "",
		SyncValidators: []FormSyncValidator{required},
	}, FormTriggerBlur)

	issues := w.FormFieldErrors(formID, "name")
	if len(issues) != 1 || issues[0].Msg != "required" {
		t.Fatalf("expected 1 required issue, got %v", issues)
	}

	// Fix the value.
	FormOnFieldEventByID(w, formID, FormFieldAdapterCfg{
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
	onSubmit := func(_ FormSubmitEvent, _ *Window) {
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
		InitialValue:    "init@test.com",
		HasInitialValue: true,
	})

	formApplyCfg(w, formID, FormCfg{ID: formID})

	var resetEvent FormResetEvent
	onReset := func(e FormResetEvent, _ *Window) {
		resetEvent = e
	}

	state := formRuntime(w, formID)
	state.resetReq = true
	formProcessRequests(w, formID, nil, onReset)

	if resetEvent.FormID != formID {
		t.Fatalf("expected form ID %q, got %q",
			formID, resetEvent.FormID)
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

	FormOnFieldEventByID(w, formID, FormFieldAdapterCfg{
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

	callCount := 0
	slow := func(
		_ FormFieldSnapshot, _ FormSnapshot,
		_ *GridAbortSignal,
	) []FormIssue {
		callCount++
		return nil
	}

	cfg := FormFieldAdapterCfg{
		FieldID:         "f",
		Value:           "a",
		AsyncValidators: []FormAsyncValidator{slow},
	}

	FormRegisterFieldByID(w, formID, cfg)
	FormOnFieldEventByID(w, formID, cfg, FormTriggerBlur)

	// Capture the abort controller.
	state := formRuntime(w, formID)
	field := state.fields["f"]
	firstAbort := field.activeAbort
	if firstAbort == nil {
		t.Fatal("expected active abort controller")
	}

	// Re-trigger; previous should be aborted.
	cfg.Value = "b"
	FormOnFieldEventByID(w, formID, cfg, FormTriggerBlur)

	if !firstAbort.Signal.IsAborted() {
		t.Fatal("first async should be aborted")
	}
}

func TestFormShouldValidate(t *testing.T) {
	tests := []struct {
		mode    FormValidateOn
		trigger FormValidationTrigger
		want    bool
	}{
		{FormValidateOnChange, FormTriggerChange, true},
		{FormValidateOnChange, FormTriggerBlur, true},
		{FormValidateOnBlur, FormTriggerChange, false},
		{FormValidateOnBlur, FormTriggerBlur, true},
		{FormValidateOnBlur, FormTriggerSubmit, true},
		{FormValidateOnSubmit, FormTriggerChange, false},
		{FormValidateOnSubmit, FormTriggerBlur, false},
		{FormValidateOnSubmit, FormTriggerSubmit, true},
		{FormValidateOnBlurSubmit, FormTriggerChange, false},
		{FormValidateOnBlurSubmit, FormTriggerBlur, true},
	}
	for _, tt := range tests {
		got := formShouldValidate(tt.mode, tt.trigger)
		if got != tt.want {
			t.Errorf("formShouldValidate(%d, %d) = %v, want %v",
				tt.mode, tt.trigger, got, tt.want)
		}
	}
}
