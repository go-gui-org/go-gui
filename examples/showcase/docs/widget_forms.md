Form container with built-in runtime validation,
submit/reset semantics, per-field state tracking (touched, dirty,
pending), configurable validation triggers, and stale field cleanup.

## Basic Usage

```go
gui.Form(gui.FormCfg{
    ID:     "my-form",
    Sizing: gui.FillFit,
    OnSubmit: func(e gui.FormSubmitEvent, w *gui.Window) {
        fmt.Println("values:", e.Values)
    },
    Content: []gui.View{ /* inputs, buttons, etc. */ },
})
```

## Field Registration

Register fields each frame so the form runtime tracks them.
Use **FormRegisterFieldByID** when the form ID is known at view
construction time (the common case). Use **FormRegisterField**
when you don't know the form ID — it walks the layout parent
chain to locate the ancestor Form.

```go
// Direct: form ID is known at view construction time.
gui.FormRegisterFieldByID(w, "my-form", gui.FormFieldAdapterCfg{
    FieldID:        "email",
    Value:          app.Email,
    SyncValidators: []gui.FormSyncValidator{validateEmail},
})
```

```go
// Parent-chain walk: useful in shared/reusable input callbacks
// where the form ID is not known at compile time.
func onEmailChange(l *gui.Layout, s string, w *gui.Window) {
    gui.State[App](w).Email = s
    gui.FormRegisterField(w, l, gui.FormFieldAdapterCfg{
        FieldID:        "email",
        Value:          s,
        SyncValidators: []gui.FormSyncValidator{validateEmail},
    })
}
```

### InitialValue

Set `InitialValue` and `HasInitialValue` to seed the "dirty"
tracking baseline — Form reset restores these values. If you
omit them, the first-registered value becomes the initial value.

```go
gui.FormRegisterFieldByID(w, "my-form", gui.FormFieldAdapterCfg{
    FieldID:         "username",
    Value:           app.Username,
    InitialValue:    app.SavedUsername, // persisted value
    HasInitialValue: true,
    SyncValidators:  []gui.FormSyncValidator{validateUsername},
})
```

## Sync Validators

```go
func validateEmail(
    f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
    if !strings.Contains(f.Value, "@") {
        return []gui.FormIssue{{Msg: "must contain @"}}
    }
    return nil
}
```

## Async Validators

```go
func checkUnique(
    f gui.FormFieldSnapshot, _ gui.FormSnapshot,
    signal *gui.GridAbortSignal,
) []gui.FormIssue {
    // Check signal.IsAborted() periodically.
    resp := fetchAPI(f.Value)
    if resp.Taken {
        return []gui.FormIssue{{Msg: "already taken"}}
    }
    return nil
}
```

## Cross-Field Validation

When a validator needs values from other fields, use the
`FormSnapshot` parameter. It carries `Values` (all field
values) and `Fields` (per-field state including errors).

```go
func passwordsMatch(
    f gui.FormFieldSnapshot, form gui.FormSnapshot,
) []gui.FormIssue {
    confirm := strings.TrimSpace(form.Values["confirmPassword"])
    if f.Value != confirm {
        return []gui.FormIssue{{Msg: "passwords do not match"}}
    }
    return nil
}
```

Register it on both fields so either change re-checks:

```go
gui.FormRegisterFieldByID(w, "signup", gui.FormFieldAdapterCfg{
    FieldID:        "password",
    Value:          app.Password,
    SyncValidators: []gui.FormSyncValidator{passwordsMatch},
})
gui.FormRegisterFieldByID(w, "signup", gui.FormFieldAdapterCfg{
    FieldID:        "confirmPassword",
    Value:          app.ConfirmPassword,
    SyncValidators: []gui.FormSyncValidator{passwordsMatch},
})
```

## Issue Codes and Kinds

`FormIssue` carries a `Code` (machine-readable) and a `Kind`
(error vs warning). Warnings do not block submit.

```go
func validateEmail(
    f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
    if f.Value == "" {
        return []gui.FormIssue{{
            Code: "EMAIL_REQUIRED",
            Kind: gui.FormIssueError,
            Msg:  "email is required",
        }}
    }
    if !strings.Contains(f.Value, "@") {
        return []gui.FormIssue{{
            Code: "EMAIL_INVALID",
            Kind: gui.FormIssueError,
            Msg:  "must contain @",
        }}
    }
    // Advisory — does not block submit.
    if strings.HasSuffix(f.Value, "@example.com") {
        return []gui.FormIssue{{
            Code: "EMAIL_DISPOSABLE",
            Kind: gui.FormIssueWarning,
            Msg:  "personal email recommended",
        }}
    }
    return nil
}
```

## Allow Submit Despite Errors or Pending

By default, `FormRequestSubmit` is blocked when any field has
errors or an async validator is running. Override per-form:

```go
gui.Form(gui.FormCfg{
    ID:                 "draft-form",
    AllowInvalidSubmit: true,  // submit even with errors
    AllowPendingSubmit: true,  // submit while async running
    OnSubmit: func(e gui.FormSubmitEvent, w *gui.Window) {
        // e.Errors is non-empty when AllowInvalidSubmit is true
        // and fields have issues.
        for fieldID, issues := range e.Errors {
            log.Printf("field %s: %v", fieldID, issues)
        }
        // e.Pending is non-empty when AllowPendingSubmit is true
        // and async validators are still running.
        if len(e.Pending) > 0 {
            log.Printf("pending: %v", e.Pending)
        }
    },
})
```

Check `FormSubmitEvent.Errors` and `FormSubmitEvent.Pending`
in your submit handler — they carry the issues that would
normally block submission.

## Custom Slots

`ErrorSlot`, `SummarySlot`, and `PendingSlot` replace the
default inline rendering for per-field errors, summary text,
and pending indicators.

```go
gui.Form(gui.FormCfg{
    ID: "custom-form",
    ErrorSlot: func(fieldID string, issues []gui.FormIssue) gui.View {
        return gui.Column(gui.ContainerCfg{
            Padding: gui.NoPadding,
            Content: func() []gui.View {
                var views []gui.View
                for _, iss := range issues {
                    c := theme.ErrorColor
                    if iss.Kind == gui.FormIssueWarning {
                        c = theme.WarningColor
                    }
                    views = append(views, gui.Text(gui.TextCfg{
                        Text:  iss.Msg,
                        Color: gui.Some(c),
                    }))
                }
                return views
            }(),
        })
    },
    PendingSlot: func(fieldIDs []string) gui.View {
        if len(fieldIDs) == 0 {
            return gui.Text(gui.TextCfg{Text: ""})
        }
        return gui.Text(gui.TextCfg{
            Text: fmt.Sprintf("Checking %s…",
                strings.Join(fieldIDs, ", ")),
        })
    },
})
```

## Event Wiring

In input callbacks, trigger validation via FormOnFieldEvent:

```go
gui.Input(gui.InputCfg{
    OnTextChanged: func(l *gui.Layout, s string, w *gui.Window) {
        gui.State[App](w).Email = s
        gui.FormOnFieldEvent(w, l, emailCfg(s),
            gui.FormTriggerChange)
    },
    OnBlur: func(l *gui.Layout, w *gui.Window) {
        gui.FormOnFieldEvent(w, l, emailCfg(app.Email),
            gui.FormTriggerBlur)
    },
})
```

## Submit / Reset

```go
gui.FormRequestSubmit(w, "my-form")
gui.FormRequestReset(w, "my-form")
```

## Querying State

```go
summary := w.FormSummary("my-form")
fs, ok := w.FormFieldState("my-form", "email")
issues := w.FormFieldErrors("my-form", "email")
pending := w.FormPendingState("my-form")
```

## Key Properties

| Property           | Type             | Description                        |
|--------------------|------------------|------------------------------------|
| ID                 | string           | Required form identifier           |
| ValidateOn         | FormValidateOn   | When to trigger (default BlurSubmit)|
| NoSubmitOnEnter    | bool             | Disable enter-key submit           |
| AllowInvalidSubmit | bool             | Permit submit with errors          |
| AllowPendingSubmit | bool             | Permit submit while async pending  |
| OnSubmit           | func             | Called on successful submit         |
| OnReset            | func             | Called after reset                  |
| ErrorSlot          | func             | Custom per-field error view         |
| SummarySlot        | func             | Custom summary view                 |
| PendingSlot        | func             | Custom pending indicator view       |

## Validation Modes

| Mode           | Change | Blur | Submit |
|----------------|--------|------|--------|
| OnChange       | yes    | yes  | yes    |
| OnBlur         | no     | yes  | yes    |
| OnBlurSubmit   | no     | yes  | yes    |
| OnSubmit       | no     | no   | yes    |
