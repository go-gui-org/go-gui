package gui

import (
	"sync/atomic"
	"testing"
	"time"
)

// Async validators and submit (#1257 review). An async result is reused
// while its field's value is unchanged; a submit that only waits for a
// running check goes ahead, or is rejected, when the check returns; a
// value change stops a running check and cancels a waiting submit.

type asyncApp struct{ Name string }

// asyncForm builds a form "f" with one field "name" and the given async
// validator, and counts OnSubmit calls.
func asyncForm(
	t *testing.T, value string, val FormAsyncValidator, submits *int,
) (*Window, *asyncApp) {
	t.Helper()
	app := &asyncApp{Name: value}
	w := NewTestWindow(t, WindowCfg{State: app})
	w.TestRender(func(w *Window) View {
		a := State[asyncApp](w)
		FormRegisterFieldByID(w, "f", FormFieldAdapterCfg{
			FieldID: "name", Value: a.Name,
			AsyncValidators: []FormAsyncValidator{val},
		})
		return Form(FormCfg{
			ID:       "f",
			OnSubmit: func(FormSubmitEvent, EventCtx) { *submits++ },
			Content:  []View{Input(InputCfg{ID: "name", Text: a.Name})},
		})
	})
	return w, app
}

// asyncSettle runs frames, with the queued commands that carry async
// results, until form "f" has nothing pending or a deadline passes.
func asyncSettle(t *testing.T, w *Window) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		w.flushCommands()
		w.TestRender(nil)
		if !w.FormSummary("f").Pending {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("async validation did not finish")
}

// gatedValidator blocks until release is closed, counts its runs and
// reports "taken" for the value "admin".
func gatedValidator(release <-chan struct{}, runs *atomic.Int32) FormAsyncValidator {
	return func(f FormFieldSnapshot, _ FormSnapshot, _ *GridAbortSignal) []FormIssue {
		runs.Add(1)
		<-release
		if f.Value == "admin" {
			return []FormIssue{{Msg: "taken: " + f.Value}}
		}
		return nil
	}
}

// Before, every submit restarted the check and the default refused a
// pending submit, so OnSubmit never ran.
func TestFormSubmitWithAsyncValidator(t *testing.T) {
	var runs atomic.Int32
	quick := func(FormFieldSnapshot, FormSnapshot, *GridAbortSignal) []FormIssue {
		runs.Add(1)
		return nil
	}
	submits := 0
	w, _ := asyncForm(t, "ann", quick, &submits)
	for i := 1; i <= 2; i++ {
		FormRequestSubmit(w, "f")
		w.TestRender(nil)
		asyncSettle(t, w)
		if submits != i {
			t.Fatalf("submit %d: OnSubmit ran %d times, want %d", i, submits, i)
		}
	}
	if n := runs.Load(); n != 1 {
		t.Errorf("async validator ran %d times for one value, want 1", n)
	}
}

func TestFormSubmitWaitsForRunningCheck(t *testing.T) {
	for _, tt := range []struct {
		value       string
		wantSubmits int
		wantIssues  int
	}{
		{"ann", 1, 0},
		{"admin", 0, 1},
	} {
		t.Run(tt.value, func(t *testing.T) {
			release := make(chan struct{})
			var runs atomic.Int32
			submits := 0
			w, _ := asyncForm(t, tt.value, gatedValidator(release, &runs), &submits)
			FormRequestSubmit(w, "f")
			w.TestRender(nil)
			w.TestRender(nil)
			if submits != 0 || !formRuntimeRead(w, "f").submitWaiting {
				t.Fatalf("while the check runs: OnSubmit=%d waiting=%v; "+
					"want 0, true", submits, formRuntimeRead(w, "f").submitWaiting)
			}
			close(release)
			asyncSettle(t, w)
			w.TestRender(nil)
			issues := w.FormFieldErrors("f", "name")
			if submits != tt.wantSubmits || len(issues) != tt.wantIssues ||
				formRuntimeRead(w, "f").submitWaiting {
				t.Fatalf("after the check: OnSubmit=%d issues=%v waiting=%v; "+
					"want %d, %d issues, false", submits, issues,
					formRuntimeRead(w, "f").submitWaiting,
					tt.wantSubmits, tt.wantIssues)
			}
		})
	}
}

// A value change while a check runs stops it, so its result for the
// old value never lands. A submit waiting for that check starts over:
// it checks the new value, then goes ahead.
func TestFormValueChangeStopsRunningCheck(t *testing.T) {
	release := make(chan struct{})
	var runs atomic.Int32
	submits := 0
	w, app := asyncForm(t, "admin", gatedValidator(release, &runs), &submits)
	FormRequestSubmit(w, "f")
	w.TestRender(nil)
	firstSeq := formRuntimeRead(w, "f").fields["name"].requestSeq

	app.Name = "xadmin"
	w.TestRender(nil)
	field := formRuntimeRead(w, "f").fields["name"]
	if field.requestSeq == firstSeq || field.asyncFor != "xadmin" ||
		!formRuntimeRead(w, "f").submitWaiting {
		t.Fatalf("after the change: check for %q (seq %d, first %d), "+
			"waiting=%v; want a new check of %q and a waiting submit",
			field.asyncFor, field.requestSeq, firstSeq,
			formRuntimeRead(w, "f").submitWaiting, "xadmin")
	}
	close(release)
	asyncSettle(t, w)
	w.TestRender(nil)
	fs, _ := w.FormFieldState("f", "name")
	if len(fs.Errors) != 0 || submits != 1 {
		t.Fatalf("value=%q errors=%v OnSubmit=%d; want no errors (the "+
			"old check's result must not land) and one submit",
			fs.Value, fs.Errors, submits)
	}
}

// Clicking a submit button moves focus out of the field in the same
// event, and the field commits its text one pass later (NumericInput
// clamps -5 to 0). The submit waits for that commit, so OnSubmit gets
// the committed value, with or without an async validator.
func TestFormSubmitAfterBlurCommit(t *testing.T) {
	quick := func(FormFieldSnapshot, FormSnapshot, *GridAbortSignal) []FormIssue {
		return nil
	}
	for _, withAsync := range []bool{false, true} {
		name := "sync only"
		if withAsync {
			name = "with async validator"
		}
		t.Run(name, func(t *testing.T) {
			type st struct{ Age string }
			var submitted []string
			w := NewTestWindow(t, WindowCfg{State: &st{}})
			w.TestRender(func(w *Window) View {
				a := State[st](w)
				cfg := FormFieldAdapterCfg{FieldID: "age", Value: a.Age}
				if withAsync {
					cfg.AsyncValidators = []FormAsyncValidator{quick}
				}
				FormRegisterFieldByID(w, "f", cfg)
				return Form(FormCfg{
					ID: "f",
					OnSubmit: func(e FormSubmitEvent, _ EventCtx) {
						submitted = append(submitted, e.Values["age"])
					},
					Content: []View{
						NumericInput(NumericInputCfg{
							ID: "age", Text: a.Age, Min: Some(0.0),
							OnTextChanged: func(s string, ctx EventCtx) {
								State[st](ctx.Window).Age = s
							},
							OnValueCommit: func(_ Opt[float64], s string, ctx EventCtx) {
								State[st](ctx.Window).Age = s
							},
						}),
						Button(ButtonCfg{
							ID:      "go",
							Content: []View{Text(TextCfg{Text: "Go"})},
							OnClick: func(ctx EventCtx) {
								FormRequestSubmit(ctx.Window, "f")
								ctx.Consume()
							},
						}),
					},
				})
			})
			age := ScopeID(formLayoutID("f"), "age")
			if err := w.TestType(age, "-5"); err != nil {
				t.Fatal(err)
			}
			if err := w.TestClick(ScopeID(formLayoutID("f"), "go")); err != nil {
				t.Fatal(err)
			}
			asyncSettle(t, w)
			w.TestRender(nil)
			if len(submitted) != 1 || submitted[0] != "0" {
				t.Fatalf("OnSubmit got %v, want one submit of the committed %q",
					submitted, "0")
			}
		})
	}
}

// While a submit waits for async validators, every frame asks whether
// it can be decided. That question must not allocate, even when a field
// has issues (a warning here) whose summary would build a map.
func TestFormWaitingSubmitDoesNotAllocate(t *testing.T) {
	w := newTestWindow()
	state := formRuntime(w, "f")
	formApplyCfg(w, "f", FormCfg{ID: "f"})
	field := &formFieldRuntime{
		pending:    true,
		syncErrors: []FormIssue{{Msg: "advice", Kind: FormIssueWarning}},
	}
	state.fields["name"] = field
	state.submitWaiting = true
	allocs := testing.AllocsPerRun(100, func() {
		if formFinishSubmit(w, "f", state, nil, soundCues{}) {
			t.Fatal("a pending field must keep the submit waiting")
		}
	})
	if allocs != 0 {
		t.Fatalf("a waiting submit allocated %.0f times per frame, want 0", allocs)
	}
}

// Validating a field again with an unchanged value reuses the async
// result; a new value asks again.
func TestFormAsyncResultReusedForSameValue(t *testing.T) {
	var runs atomic.Int32
	quick := func(FormFieldSnapshot, FormSnapshot, *GridAbortSignal) []FormIssue {
		runs.Add(1)
		return nil
	}
	w := newTestWindow()
	cfg := FormFieldAdapterCfg{
		FieldID: "name", Value: "ann",
		AsyncValidators: []FormAsyncValidator{quick},
	}
	waitRuns := func(n int32) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for runs.Load() < n && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		// A further, unwanted run would show by now.
		time.Sleep(20 * time.Millisecond)
	}

	FormRegisterFieldByID(w, "f", cfg)
	formOnFieldEventByID(w, "f", cfg, FormTriggerBlur)
	// Let the first check run before anything can stop it.
	waitRuns(1)
	formOnFieldEventByID(w, "f", cfg, FormTriggerSubmit)
	waitRuns(1)
	if n := runs.Load(); n != 1 {
		t.Fatalf("submit with an unchanged value: %d runs, want 1", n)
	}
	cfg.Value = "bob"
	formOnFieldEventByID(w, "f", cfg, FormTriggerBlur)
	waitRuns(2)
	if n := runs.Load(); n != 2 {
		t.Fatalf("after a new value: %d runs, want 2", n)
	}
}
