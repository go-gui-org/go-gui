package gui

import "testing"

// Submit edge cases from the CodeRabbit review of #1260.

// OnSubmit and OnReset run after the frame, outside the frame lock, so
// they may call window functions. Before, the form called them from
// its AmendLayout, and SetFocus or SetView there panicked.
func TestFormCallbacksMayCallWindowAPI(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	submits, resets := 0, 0
	var view func(*Window) View
	view = func(w *Window) View {
		FormRegisterFieldByID(w, "f", FormFieldAdapterCfg{FieldID: "name", Value: "ann"})
		return Form(FormCfg{
			ID: "f",
			OnSubmit: func(_ FormSubmitEvent, ctx EventCtx) {
				submits++
				ctx.Window.SetFocus(ScopeID(formLayoutID("f"), "name"))
				ctx.Window.SetView(view)
			},
			OnReset: func(_ FormResetEvent, ctx EventCtx) {
				resets++
				ctx.Window.SetFocus(ScopeID(formLayoutID("f"), "name"))
			},
			Content: []View{Input(InputCfg{ID: "name", Text: "ann"})},
		})
	}
	w.TestRender(view)
	FormRequestSubmit(w, "f")
	w.TestRender(nil)
	FormRequestReset(w, "f")
	w.TestRender(nil)
	if submits != 1 || resets != 1 {
		t.Fatalf("OnSubmit ran %d times, OnReset %d times; want 1 and 1",
			submits, resets)
	}
}

// A submit that waits for async validators does not finish in a pass
// where focus left a field: that field's blur commit may still change
// its value.
func TestFormWaitingSubmitWaitsForBlurCommit(t *testing.T) {
	w := newTestWindow()
	FormRegisterFieldByID(w, "f", FormFieldAdapterCfg{FieldID: "age", Value: "-5"})
	formApplyCfg(w, "f", FormCfg{ID: "f"})
	state := formRuntime(w, "f")
	state.submitWaiting = true // its async check has just returned
	state.fields["age"].blurPending = true

	submits := 0
	onSubmit := func(FormSubmitEvent, EventCtx) { submits++ }
	formProcessRequests(w, "f", onSubmit, nil, soundCues{})
	if submits != 0 || !state.submitWaiting {
		t.Fatalf("with a blur commit on its way: OnSubmit=%d waiting=%v; "+
			"want 0, true", submits, state.submitWaiting)
	}
	state.fields["age"].blurPending = false
	formProcessRequests(w, "f", onSubmit, nil, soundCues{})
	if submits != 1 {
		t.Fatalf("after the commit: OnSubmit=%d, want 1", submits)
	}
}

// A field that appears while a submit waits is validated before the
// submit sends its value.
func TestFormNewFieldRestartsWaitingSubmit(t *testing.T) {
	w := newTestWindow()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	slow := func(FormFieldSnapshot, FormSnapshot, *GridAbortSignal) []FormIssue {
		<-release
		return nil
	}
	required := func(f FormFieldSnapshot, _ FormSnapshot) []FormIssue {
		if f.Value == "" {
			return []FormIssue{{Msg: "required"}}
		}
		return nil
	}
	FormRegisterFieldByID(w, "f", FormFieldAdapterCfg{
		FieldID: "name", Value: "ann",
		AsyncValidators: []FormAsyncValidator{slow},
	})
	formApplyCfg(w, "f", FormCfg{ID: "f"})
	submits := 0
	onSubmit := func(FormSubmitEvent, EventCtx) { submits++ }
	FormRequestSubmit(w, "f")
	formProcessRequests(w, "f", onSubmit, nil, soundCues{})
	state := formRuntime(w, "f")
	if !state.submitWaiting {
		t.Fatal("the submit should wait for the running check")
	}

	FormRegisterFieldByID(w, "f", FormFieldAdapterCfg{
		FieldID: "city", Value: "",
		SyncValidators: []FormSyncValidator{required},
	})
	formProcessRequests(w, "f", onSubmit, nil, soundCues{})
	if submits != 0 || state.submitWaiting {
		t.Fatalf("after the new field: OnSubmit=%d waiting=%v; want a "+
			"rejected submit", submits, state.submitWaiting)
	}
	if issues := w.FormFieldErrors("f", "city"); len(issues) != 1 {
		t.Fatalf("the new field was not validated: issues=%v", issues)
	}
}
