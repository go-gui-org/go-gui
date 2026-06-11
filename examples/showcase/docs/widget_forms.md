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
Call FormRegisterFieldByID during view construction (form ID
is known), or FormRegisterField from event handlers (walks
the parent chain).

```go
gui.FormRegisterFieldByID(w, "my-form", gui.FormFieldAdapterCfg{
    FieldID:        "email",
    Value:          app.Email,
    SyncValidators: []gui.FormSyncValidator{validateEmail},
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
