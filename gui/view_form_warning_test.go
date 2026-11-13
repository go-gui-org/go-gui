package gui

import "testing"

// A FormIssueWarning is reported but leaves its field valid (#1236).
// Before the fix, any issue counted the field as invalid, so a
// warning-only form read as invalid and blocked submit.

// formIssuesValidator returns a sync validator that always reports
// issues, whatever the value.
func formIssuesValidator(issues ...FormIssue) FormSyncValidator {
	return func(FormFieldSnapshot, FormSnapshot) []FormIssue {
		return issues
	}
}

// formValidateField registers fieldID with one validator and runs it on
// blur.
func formValidateField(
	w *Window, formID, fieldID string, val FormSyncValidator,
) {
	cfg := FormFieldAdapterCfg{
		FieldID:        fieldID,
		Value:          "v",
		SyncValidators: []FormSyncValidator{val},
	}
	FormRegisterFieldByID(w, formID, cfg)
	formOnFieldEventByID(w, formID, cfg, FormTriggerBlur)
}

func TestFormSummaryWarningKeepsFieldValid(t *testing.T) {
	warn := FormIssue{Code: "W", Msg: "advice", Kind: FormIssueWarning}
	fail := FormIssue{Code: "E", Msg: "broken", Kind: FormIssueError}
	// Validators predating Kind leave it at its zero value, which must
	// stay an error.
	unset := FormIssue{Code: "U", Msg: "no kind"}
	// An out-of-range Kind blocks rather than passes.
	odd := FormIssue{Code: "X", Msg: "odd", Kind: FormIssueKind(9)}

	tests := []struct {
		name        string
		fields      map[string][]FormIssue
		wantValid   bool
		wantInvalid int
	}{
		{"warning only", map[string][]FormIssue{
			"a": {warn},
		}, true, 0},
		{"two warnings", map[string][]FormIssue{
			"a": {warn, warn},
		}, true, 0},
		{"warning then error", map[string][]FormIssue{
			"a": {warn, fail},
		}, false, 1},
		{"zero kind is an error", map[string][]FormIssue{
			"a": {unset},
		}, false, 1},
		{"unknown kind is an error", map[string][]FormIssue{
			"a": {warn, odd},
		}, false, 1},
		{"warning field beside error field", map[string][]FormIssue{
			"a": {warn},
			"b": {fail},
		}, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWindow()
			formID := "warn-form"
			for fieldID, issues := range tt.fields {
				formValidateField(w, formID, fieldID,
					formIssuesValidator(issues...))
			}

			got := w.FormSummary(formID)
			if got.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", got.Valid, tt.wantValid)
			}
			if got.InvalidCount != tt.wantInvalid {
				t.Errorf("InvalidCount = %d, want %d",
					got.InvalidCount, tt.wantInvalid)
			}
			// Every issue stays visible, warnings included, so a
			// summary can still render them.
			if len(got.Issues) != len(tt.fields) {
				t.Fatalf("Issues has %d fields, want %d",
					len(got.Issues), len(tt.fields))
			}
			for fieldID, want := range tt.fields {
				if n := len(got.Issues[fieldID]); n != len(want) {
					t.Errorf("Issues[%q] has %d issues, want %d",
						fieldID, n, len(want))
				}
			}
		})
	}
}

func TestFormSubmitNotBlockedByWarning(t *testing.T) {
	w := newTestWindow()
	formID := "warn-submit"
	warn := FormIssue{Code: "W", Msg: "advice", Kind: FormIssueWarning}
	formValidateField(w, formID, "email", formIssuesValidator(warn))
	// Block invalid is the default.
	formApplyCfg(w, formID, FormCfg{ID: formID})

	var got *FormSubmitEvent
	onSubmit := func(e FormSubmitEvent, _ EventCtx) { got = &e }
	FormRequestSubmit(w, formID)
	formProcessRequests(w, formID, onSubmit, nil, soundCues{})

	if got == nil {
		t.Fatal("a warning-only form must submit")
	}
	if !got.Valid || !got.State.Valid || got.State.InvalidCount != 0 {
		t.Errorf("submit event = Valid %v, State.Valid %v, "+
			"InvalidCount %d; want true, true, 0",
			got.Valid, got.State.Valid, got.State.InvalidCount)
	}
	issues := got.State.Issues["email"]
	if len(issues) != 1 || issues[0].Code != "W" ||
		issues[0].Kind != FormIssueWarning {
		t.Errorf("State.Issues[email] = %v, want the warning", issues)
	}
}

func TestFormSubmitBlockedByErrorBesideWarning(t *testing.T) {
	w := newTestWindow()
	formID := "warn-error-submit"
	warn := FormIssue{Code: "W", Msg: "advice", Kind: FormIssueWarning}
	fail := FormIssue{Code: "E", Msg: "broken", Kind: FormIssueError}
	formValidateField(w, formID, "email", formIssuesValidator(warn))
	formValidateField(w, formID, "name", formIssuesValidator(fail))
	formApplyCfg(w, formID, FormCfg{ID: formID})

	submitted := false
	onSubmit := func(FormSubmitEvent, EventCtx) { submitted = true }
	FormRequestSubmit(w, formID)
	formProcessRequests(w, formID, onSubmit, nil, soundCues{})

	if submitted {
		t.Fatal("an error on another field must still block submit")
	}
}
