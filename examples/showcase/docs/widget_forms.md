A Form groups input fields, checks them with validators and submits them. It
keeps each field's state (touched, dirty, pending), validates a field when the
user edits it, when focus leaves it and on submit, and submits on Enter or from
a button.

## Basic Usage

A form needs an ID. The app registers each field every frame with its current
value, and gives the field's widget the field's ID. The widget's callbacks only
store what the user types; the form works out the rest.

```go
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
```

`FormRequestSubmit` validates every field first. If async validators still run,
the submit waits for them. `OnSubmit` runs only when no field has an error.

## Linking a Field to Its Widget

The form finds a field's widget by its ID, inside the form. When the widget's ID
is not the field's ID, name it in `WidgetID`:

```go
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
```

A field whose widget the form does not find is still validated on submit. It
gets no validation when the user edits it or leaves it.

## When Validation Runs

| Event                                | What the form does                                                                                    |
| ------------------------------------ | ----------------------------------------------------------------------------------------------------- |
| The user edits a field               | Runs its validators if its mode includes edits. Re-runs the sync validators of fields already checked |
| Focus leaves a field                 | Marks the field touched. Runs its validators if its mode includes blur                                |
| Submit                               | Marks every field touched and runs every field's validators, whatever the mode                        |
| The app changes a value by itself    | Nothing until submit. Only a change while the field's widget has focus counts as an edit              |
| Any value changes                    | Stops the field's async check that still runs. A submit that waits for it starts over                 |
| A field's async validator comes back | Stores its issues and redraws the window                                                              |

`FormCfg.ValidateOn` sets the mode for the whole form, and
`FormFieldAdapterCfg.ValidateOnOverride` for one field:

| Mode                                 | Edit | Blur | Submit |
| ------------------------------------ | ---- | ---- | ------ |
| `FormValidateOnChange`               | yes  | yes  | yes    |
| `FormValidateOnBlur`                 | no   | yes  | yes    |
| `FormValidateOnBlurSubmit` (default) | no   | yes  | yes    |
| `FormValidateOnSubmit`               | no   | no   | yes    |

`FormValidateOnBlur` and `FormValidateOnBlurSubmit` behave the same. A field set
to `FormValidateOnSubmit` keeps its issues until the next submit: edits of other
fields do not re-check it.

## Sync Validators

A sync validator gets a copy of the field and of the whole form, and returns the
field's issues, or nil when the value is fine. It runs on the UI thread, so it
must be quick.

```go
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
```

## Async Validators

An async validator is for a slow check, such as asking a server. It runs on its
own goroutine, and the field is pending until it returns. It runs again only
when the field's value has changed: a blur or a submit with the same value
reuses its result. A change of the value aborts it, so it checks the signal
between slow steps:

```go
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
```

Register it with `AsyncValidators: []gui.FormAsyncValidator{checkUsernameFree}`.
A submit while it runs waits for it, then goes ahead or is rejected. Because it
follows its own field's value, a rule that also reads other fields belongs in a
sync validator.

## Cross-Field Validation

A rule that compares fields reads the other values from the form snapshot.
Register it on one field. When another field changes, the form re-checks every
field it has already checked, so the rule follows edits of both fields:

```go
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
```

## Issue Codes and Kinds

`FormIssue` carries a `Code` for code to test, a `Msg` for the user and a
`Kind`. An error (`FormIssueError`, the zero value) makes the field invalid and
blocks submit. A warning (`FormIssueWarning`) is shown, but does not block
submit.

```go
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
```

## Showing Validation Results

The form calls nothing when it validates. It stores each field's issues and
state, and redraws the window. The view reads them every frame:

- `w.FormFieldErrors(formID, fieldID)`: the field's issues, warnings included.
- `w.FormFieldState(formID, fieldID)`: value, issues, touched, dirty, pending.
- `w.FormSummary(formID)`: whether the form is valid, how many fields are
  invalid, whether a check is pending, and all issues.
- `w.FormPendingState(formID)`: the fields whose async validators run.

```go
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
```

## Submit and Reset

Submit comes from `FormRequestSubmit`, usually in a button's `OnClick`, or from
Enter in a single-line Input inside the form. To keep Enter from submitting, set
`FormCfg.NoSubmitOnEnter`, or call `ctx.Consume()` in that Input's `OnEnter`.
While async validators run, a submit waits for them. An edit in the meantime
makes it start over, so it checks what it sends. A submit made just as focus
leaves a field, for example by clicking the submit button, waits one frame for
that field's committed value.

`OnSubmit` and `OnReset` run after the frame, outside the frame lock, so they
may call window functions such as `SetFocus` or `SetView`, for example to move
on to the next page.

`FormRequestReset` sets every field back to its initial value and clears its
issues and touched state. go-gui cannot change the app's state, so `OnReset`
copies the initial values into it:

```go
// resetSignup is the form's OnReset. The form resets its own copy of
// each field; the app copies the initial values into its state.
func resetSignup(e gui.FormResetEvent, ctx gui.EventCtx) {
	app := gui.State[signupApp](ctx.Window)
	app.Email = e.Values["email"]
	app.Password = e.Values["password"]
	app.Message = ""
}
```

### InitialValue

A field's initial value is the first value registered, unless the registration
names one. Dirty is measured against it, and reset goes back to it:

```go
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
```

## Allow Submit Despite Errors or Pending

By default, a submit is rejected while any field has an error, and waits while
an async validator runs. A form can let it through at once and look at the event
itself:

```go
// draftForm saves whatever the user typed, issues and all.
func draftForm(content []gui.View) gui.View {
	return gui.Form(gui.FormCfg{
		ID:                 "draft",
		AllowInvalidSubmit: true, // submit even with errors
		AllowPendingSubmit: true, // submit without waiting for async validators
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
```

## Custom Slots

Instead of placing issues in its view, the app can let the form add them after
its content. The form calls `ErrorSlot` once for each field with issues, and
`SummarySlot` and `PendingSlot` every frame:

```go
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
```

## Upgrading from FormOnFieldEvent

Earlier versions had the app report every edit and blur with `FormOnFieldEvent`.
It is deprecated and keeps working. To move:

1. Remove the `FormOnFieldEvent` calls from `OnTextChanged`, `OnBlur` and the
   other callbacks.
2. Give each field's widget the field's ID, or set `WidgetID`.
3. Give the form an ID if it has none. A form without an ID does nothing.
4. Check Enter: in a single-line Input inside the form it now submits.

## Key Properties

| Property           | Type           | Description                                                |
| ------------------ | -------------- | ---------------------------------------------------------- |
| ID                 | string         | Required. Names the form for registration, submit, reset   |
| ValidateOn         | FormValidateOn | When validators run (default `FormValidateOnBlurSubmit`)   |
| NoSubmitOnEnter    | bool           | Enter in a field does not submit                           |
| AllowInvalidSubmit | bool           | Submit even with errors                                    |
| AllowPendingSubmit | bool           | Submit without waiting for async validators                |
| OnSubmit           | func           | Called after a submit that is not blocked                  |
| OnReset            | func           | Called after reset; copies the initial values to app state |
| ErrorSlot          | func           | Builds the issues of one field                             |
| SummarySlot        | func           | Builds a summary of the form                               |
| PendingSlot        | func           | Builds the line for fields being checked                   |
