package gui_test

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// This file is the source of every Go block in the showcase forms
// guide, examples/showcase/docs/widget_forms.md (#1257). Each block is
// a marked region here, quoted byte for byte and pinned by
// examples/showcase/sound_snippet_test.go. The tests at the end run the
// examples through real frames.

// doc:snippet-begin form-basic
//
// Quoted in the forms guide under "Basic Usage".

// signupApp is the app's own state. The form never changes it.
type signupApp struct {
	Email    string
	Password string
	Message  string
}

func signupView(w *gui.Window) gui.View {
	app := gui.State[signupApp](w)

	// Register every field each frame, with its current value. The
	// form finds each field's widget by its ID and validates the field
	// when the user edits it, when focus leaves it and on submit.
	gui.FormRegisterFieldByID(w, "signup", gui.FormFieldAdapterCfg{
		FieldID:        "email",
		Value:          app.Email,
		SyncValidators: []gui.FormSyncValidator{validateEmail},
	})
	gui.FormRegisterFieldByID(w, "signup", gui.FormFieldAdapterCfg{
		FieldID:        "password",
		Value:          app.Password,
		SyncValidators: []gui.FormSyncValidator{validatePassword},
	})

	return gui.Form(gui.FormCfg{
		ID: "signup",
		OnSubmit: func(e gui.FormSubmitEvent, ctx gui.EventCtx) {
			gui.State[signupApp](ctx.Window).Message = "Welcome, " + e.Values["email"]
		},
		OnReset: resetSignup, // see "Submit and Reset"
		Content: []gui.View{
			gui.Input(gui.InputCfg{
				ID:   "email", // the field's ID
				Text: app.Email,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[signupApp](ctx.Window).Email = s
				},
			}),
			fieldIssues(w, "signup", "email"), // see "Showing Validation Results"
			gui.Input(gui.InputCfg{
				ID:         "password",
				Text:       app.Password,
				IsPassword: true,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[signupApp](ctx.Window).Password = s
				},
			}),
			fieldIssues(w, "signup", "password"),
			gui.Button(gui.ButtonCfg{
				ID:      "signup-submit",
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Sign up"})},
				OnClick: func(ctx gui.EventCtx) {
					gui.FormRequestSubmit(ctx.Window, "signup")
					ctx.Consume()
				},
			}),
			gui.Text(gui.TextCfg{Text: app.Message}),
		},
	})
}

// doc:snippet-end form-basic

// doc:snippet-begin form-widget-id
//
// Quoted in the forms guide under "Linking a Field to Its Widget".

// emailField registers the email field and builds its Input, whose ID
// is not the field's ID. WidgetID links the two.
func emailField(w *gui.Window, email string) gui.View {
	gui.FormRegisterFieldByID(w, "signup", gui.FormFieldAdapterCfg{
		FieldID:        "email",
		WidgetID:       "signup-email",
		Value:          email,
		SyncValidators: []gui.FormSyncValidator{validateEmail},
	})
	return gui.Input(gui.InputCfg{
		ID:   "signup-email",
		Text: email,
		OnTextChanged: func(s string, ctx gui.EventCtx) {
			gui.State[signupApp](ctx.Window).Email = s
		},
	})
}

// doc:snippet-end form-widget-id

// doc:snippet-begin form-sync-validators
//
// Quoted in the forms guide under "Sync Validators".

func validateEmail(
	f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
	v := strings.TrimSpace(f.Value)
	if v == "" {
		return []gui.FormIssue{{Code: "EMAIL_REQUIRED", Msg: "email is required"}}
	}
	if !strings.Contains(v, "@") {
		return []gui.FormIssue{{Code: "EMAIL_INVALID", Msg: "email must contain @"}}
	}
	return nil
}

func validatePassword(
	f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
	if len(f.Value) < 8 {
		return []gui.FormIssue{{Code: "PASSWORD_SHORT", Msg: "use at least 8 characters"}}
	}
	return nil
}

// doc:snippet-end form-sync-validators

// doc:snippet-begin form-async-validator
//
// Quoted in the forms guide under "Async Validators".

// usernameTaken stands in for a request to the app's server.
func usernameTaken(name string) bool {
	return name == "admin"
}

// checkUsernameFree runs on its own goroutine. A newer edit of the
// field aborts it, so it checks the signal before the slow step and
// before it reports.
func checkUsernameFree(
	f gui.FormFieldSnapshot, _ gui.FormSnapshot, signal *gui.GridAbortSignal,
) []gui.FormIssue {
	if signal.IsAborted() {
		return nil
	}
	taken := usernameTaken(f.Value)
	if signal.IsAborted() || !taken {
		return nil
	}
	return []gui.FormIssue{{Code: "USERNAME_TAKEN", Msg: "this name is taken"}}
}

// doc:snippet-end form-async-validator

// doc:snippet-begin form-cross-field
//
// Quoted in the forms guide under "Cross-Field Validation".

// confirmMatchesPassword reads the password from the form snapshot.
func confirmMatchesPassword(
	f gui.FormFieldSnapshot, form gui.FormSnapshot,
) []gui.FormIssue {
	if f.Value != form.Values["password"] {
		return []gui.FormIssue{{Code: "PASSWORD_MISMATCH", Msg: "passwords do not match"}}
	}
	return nil
}

// registerConfirm registers the rule on the confirm field only. When
// the password changes, the form re-checks confirm by itself.
func registerConfirm(w *gui.Window, confirm string) {
	gui.FormRegisterFieldByID(w, "signup", gui.FormFieldAdapterCfg{
		FieldID:        "confirm",
		Value:          confirm,
		SyncValidators: []gui.FormSyncValidator{confirmMatchesPassword},
	})
}

// doc:snippet-end form-cross-field

// doc:snippet-begin form-issue-kinds
//
// Quoted in the forms guide under "Issue Codes and Kinds".

func validateEmailWithAdvice(
	f gui.FormFieldSnapshot, form gui.FormSnapshot,
) []gui.FormIssue {
	if issues := validateEmail(f, form); issues != nil {
		return issues // errors: they block submit
	}
	if strings.HasSuffix(f.Value, "@example.com") {
		return []gui.FormIssue{{
			Code: "EMAIL_EXAMPLE",
			Kind: gui.FormIssueWarning, // shown, but does not block submit
			Msg:  "example.com cannot receive mail",
		}}
	}
	return nil
}

// doc:snippet-end form-issue-kinds

// doc:snippet-begin form-results
//
// Quoted in the forms guide under "Showing Validation Results".

// fieldIssues shows a field's issues under its widget. The form stores
// them; the view reads them every frame.
func fieldIssues(w *gui.Window, formID, fieldID string) gui.View {
	fs, ok := w.FormFieldState(formID, fieldID)
	if !ok || len(fs.Errors) == 0 {
		return nil // a nil child is skipped
	}
	theme := gui.CurrentTheme()
	views := make([]gui.View, 0, len(fs.Errors))
	for _, issue := range fs.Errors {
		style := theme.TextStyleError
		if issue.Kind == gui.FormIssueWarning {
			style = theme.TextStyleWarning
		}
		views = append(views, gui.Text(gui.TextCfg{Text: issue.Msg, TextStyle: style}))
	}
	return gui.Column(gui.ContainerCfg{
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content:    views,
	})
}

// signupStatus sums up the whole form.
func signupStatus(w *gui.Window) gui.View {
	summary := w.FormSummary("signup")
	text := "Ready to submit"
	switch {
	case summary.Pending:
		text = "Checking …"
	case !summary.Valid:
		text = fmt.Sprintf("%d fields need attention", summary.InvalidCount)
	}
	return gui.Text(gui.TextCfg{Text: text})
}

// doc:snippet-end form-results

// doc:snippet-begin form-reset
//
// Quoted in the forms guide under "Submit and Reset".

// resetSignup is the form's OnReset. The form resets its own copy of
// each field; the app copies the initial values into its state.
func resetSignup(e gui.FormResetEvent, ctx gui.EventCtx) {
	app := gui.State[signupApp](ctx.Window)
	app.Email = e.Values["email"]
	app.Password = e.Values["password"]
	app.Message = ""
}

// doc:snippet-end form-reset

// doc:snippet-begin form-allow-invalid
//
// Quoted in the forms guide under "Allow Submit Despite Errors or
// Pending".

// draftForm saves whatever the user typed, issues and all.
func draftForm(content []gui.View) gui.View {
	return gui.Form(gui.FormCfg{
		ID:                 "draft",
		AllowInvalidSubmit: true, // submit even with errors
		AllowPendingSubmit: true, // submit while async validators run
		OnSubmit: func(e gui.FormSubmitEvent, ctx gui.EventCtx) {
			for fieldID, issues := range e.State.Issues {
				log.Printf("field %s: %v", fieldID, issues)
			}
			if e.Pending {
				log.Printf("still checking: %v",
					ctx.Window.FormPendingState(e.FormID).FieldIDs)
			}
		},
		Content: content,
	})
}

// doc:snippet-end form-allow-invalid

// doc:snippet-begin form-slots
//
// Quoted in the forms guide under "Custom Slots".

// slotForm lets the form place its issues, summary and pending line
// after the content.
func slotForm(content []gui.View) gui.View {
	return gui.Form(gui.FormCfg{
		ID: "slots",
		ErrorSlot: func(fieldID string, issues []gui.FormIssue) gui.View {
			theme := gui.CurrentTheme()
			views := make([]gui.View, 0, len(issues))
			for _, issue := range issues {
				style := theme.TextStyleError
				if issue.Kind == gui.FormIssueWarning {
					style = theme.TextStyleWarning
				}
				views = append(views, gui.Text(gui.TextCfg{
					Text:      fieldID + ": " + issue.Msg,
					TextStyle: style,
				}))
			}
			return gui.Column(gui.ContainerCfg{
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content:    views,
			})
		},
		SummarySlot: func(s gui.FormSummaryState) gui.View {
			return gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("%d fields need attention", s.InvalidCount),
			})
		},
		PendingSlot: func(p gui.FormPendingState) gui.View {
			if p.PendingCount == 0 {
				return nil
			}
			return gui.Text(gui.TextCfg{
				Text: "Checking " + strings.Join(p.FieldIDs, ", ") + " …",
			})
		},
		Content: content,
	})
}

// doc:snippet-end form-slots

// doc:snippet-begin form-initial-value
//
// Quoted in the forms guide under "InitialValue".

// registerUsername measures "dirty" against the saved name, and reset
// goes back to it.
func registerUsername(w *gui.Window, username, saved string) {
	gui.FormRegisterFieldByID(w, "profile", gui.FormFieldAdapterCfg{
		FieldID:         "username",
		Value:           username,
		InitialValue:    saved,
		HasInitialValue: true, // "" can be a real initial value
	})
}

// doc:snippet-end form-initial-value

// --- tests ---------------------------------------------------------------

func exampleFocus(t *testing.T, w *gui.Window, leaf string) string {
	t.Helper()
	ids := w.ResolveID(leaf)
	if len(ids) != 1 {
		t.Fatalf("ResolveID(%q) = %v, want one ID", leaf, ids)
	}
	if err := w.TestFocus(ids[0]); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	return ids[0]
}

func TestFormExampleSignup(t *testing.T) {
	app := &signupApp{}
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app})
	w.TestRender(signupView)

	// Tab out of the empty email: its error shows.
	exampleFocus(t, w, "email")
	if _, err := w.TestTab(gui.TabForward); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if issues := w.FormFieldErrors("signup", "email"); len(issues) != 1 ||
		issues[0].Code != "EMAIL_REQUIRED" {
		t.Fatalf("email issues after tab-away = %v", issues)
	}

	// Fill both fields and press Enter in the password: it submits.
	if err := w.TestType(exampleFocus(t, w, "email"), "ann@x.org"); err != nil {
		t.Fatal(err)
	}
	pw := exampleFocus(t, w, "password")
	if err := w.TestType(pw, "longenough"); err != nil {
		t.Fatal(err)
	}
	if err := w.TestKey(pw, gui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if app.Message != "Welcome, ann@x.org" {
		t.Fatalf("Message = %q after Enter", app.Message)
	}

	// Reset puts the app's state back through resetSignup.
	gui.FormRequestReset(w, "signup")
	w.TestRender(nil)
	w.TestRender(nil)
	fs, _ := w.FormFieldState("signup", "email")
	if app.Email != "" || app.Message != "" || fs.Dirty || fs.Touched {
		t.Fatalf("after reset: app=%+v field=%+v", *app, fs)
	}
}

func TestFormExampleValidators(t *testing.T) {
	snap := func(v string) gui.FormFieldSnapshot { return gui.FormFieldSnapshot{Value: v} }
	if got := validateEmailWithAdvice(snap("a@example.com"), gui.FormSnapshot{}); len(got) != 1 ||
		got[0].Kind != gui.FormIssueWarning {
		t.Errorf("example.com: %v, want one warning", got)
	}
	form := gui.FormSnapshot{Values: map[string]string{"password": "secret12"}}
	if got := confirmMatchesPassword(snap("secret13"), form); len(got) != 1 {
		t.Errorf("mismatch not reported: %v", got)
	}
	if got := confirmMatchesPassword(snap("secret12"), form); got != nil {
		t.Errorf("match reported: %v", got)
	}

	ctrl := gui.NewGridAbortController()
	if got := checkUsernameFree(snap("admin"), gui.FormSnapshot{}, ctrl.Signal); len(got) != 1 {
		t.Errorf("taken name not reported: %v", got)
	}
	ctrl.Abort()
	if got := checkUsernameFree(snap("admin"), gui.FormSnapshot{}, ctrl.Signal); got != nil {
		t.Errorf("aborted check reported: %v", got)
	}
}

// The remaining examples build and render without duplicate IDs.
func TestFormExampleViewsRender(t *testing.T) {
	w := gui.NewTestWindow(t, gui.WindowCfg{State: &signupApp{}})
	w.TestRender(func(w *gui.Window) gui.View {
		app := gui.State[signupApp](w)
		registerConfirm(w, "")
		registerUsername(w, "ann", "ann")
		return gui.Column(gui.ContainerCfg{Content: []gui.View{
			gui.Form(gui.FormCfg{ID: "signup", Content: []gui.View{
				emailField(w, app.Email),
				signupStatus(w),
			}}),
			draftForm([]gui.View{gui.Input(gui.InputCfg{ID: "note"})}),
			slotForm([]gui.View{gui.Input(gui.InputCfg{ID: "name"})}),
		}})
	})
	if dups := w.TestDuplicateIDs(); len(dups) != 0 {
		t.Fatalf("duplicate IDs: %v", dups)
	}
	if len(w.ResolveID("signup-email")) != 1 {
		t.Error("the WidgetID example's Input is missing")
	}
}
