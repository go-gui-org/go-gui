package gui_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The form works out change, blur and Enter itself (#1257). Every test
// here wires its widgets the way an app now should: it registers each
// field every frame and stores edits in its own state, with no
// FormOnFieldEvent call. The first frame passes the view, later ones
// pass nil, because TestRender(view) calls SetView, which clears the
// form's state.

type detectApp struct {
	Email, Age, Password, Confirm, Notes string
}

// detectRequired reports an error for an empty value or "bad".
func detectRequired(
	f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
	if f.Value == "" || f.Value == "bad" {
		return []gui.FormIssue{{Code: "REQUIRED", Msg: "required"}}
	}
	return nil
}

// detectConfirmMatches compares the confirm field with the password.
func detectConfirmMatches(
	f gui.FormFieldSnapshot, form gui.FormSnapshot,
) []gui.FormIssue {
	if f.Value != form.Values["password"] {
		return []gui.FormIssue{{Code: "MISMATCH", Msg: "passwords differ"}}
	}
	return nil
}

type detectForm struct {
	cfg     gui.FormCfg
	submits int
	resets  int
	// widgetID, when set, gives the email Input a different ID and
	// links it through FormFieldAdapterCfg.WidgetID.
	widgetID string
	// onEnter, when set, is the email Input's OnEnter.
	onEnter func(gui.EventCtx)
	// confirmMode is the confirm field's ValidateOnOverride.
	confirmMode gui.FormValidateOn
}

func (d *detectForm) view(w *gui.Window) gui.View {
	a := gui.State[detectApp](w)
	reg := func(id, value string, vals ...gui.FormSyncValidator) {
		cfg := gui.FormFieldAdapterCfg{
			FieldID: id, Value: value, SyncValidators: vals,
		}
		if id == "email" {
			cfg.WidgetID = d.widgetID
		}
		if id == "confirm" {
			cfg.ValidateOnOverride = d.confirmMode
		}
		gui.FormRegisterFieldByID(w, "f", cfg)
	}
	reg("email", a.Email, detectRequired)
	reg("age", a.Age, detectRequired)
	reg("password", a.Password)
	reg("confirm", a.Confirm, detectConfirmMatches)

	emailID := "email"
	if d.widgetID != "" {
		emailID = d.widgetID
	}
	cfg := d.cfg
	cfg.ID = "f"
	cfg.OnSubmit = func(gui.FormSubmitEvent, gui.EventCtx) { d.submits++ }
	cfg.OnReset = func(e gui.FormResetEvent, ctx gui.EventCtx) {
		d.resets++
		s := gui.State[detectApp](ctx.Window)
		s.Email = e.Values["email"]
		s.Age = e.Values["age"]
		s.Password = e.Values["password"]
		s.Confirm = e.Values["confirm"]
	}
	set := func(dst *string) func(string, gui.EventCtx) {
		return func(s string, _ gui.EventCtx) { *dst = s }
	}
	cfg.Content = []gui.View{
		gui.Input(gui.InputCfg{ID: emailID, Text: a.Email,
			OnTextChanged: set(&a.Email), OnEnter: d.onEnter}),
		gui.NumericInput(gui.NumericInputCfg{ID: "age", Text: a.Age,
			OnTextChanged: set(&a.Age),
			OnValueCommit: func(_ gui.Opt[float64], text string, ctx gui.EventCtx) {
				gui.State[detectApp](ctx.Window).Age = text
			}}),
		gui.Input(gui.InputCfg{ID: "password", Text: a.Password,
			OnTextChanged: set(&a.Password)}),
		gui.Input(gui.InputCfg{ID: "confirm", Text: a.Confirm,
			OnTextChanged: set(&a.Confirm)}),
		gui.Input(gui.InputCfg{ID: "notes", Text: a.Notes,
			Mode: gui.InputMultiline, OnTextChanged: set(&a.Notes)}),
	}
	return gui.Form(cfg)
}

func newDetectWindow(
	t *testing.T, d *detectForm, a *detectApp,
) *gui.Window {
	t.Helper()
	w := gui.NewTestWindow(t, gui.WindowCfg{State: a})
	w.TestRender(d.view)
	return w
}

func detectID(t *testing.T, w *gui.Window, leaf string) string {
	t.Helper()
	ids := w.ResolveID(leaf)
	if len(ids) != 1 {
		t.Fatalf("ResolveID(%q) = %v, want one ID", leaf, ids)
	}
	return ids[0]
}

func detectFocus(t *testing.T, w *gui.Window, leaf string) string {
	t.Helper()
	id := detectID(t, w, leaf)
	if err := w.TestFocus(id); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	return id
}

func detectField(t *testing.T, w *gui.Window, id string) gui.FormFieldState {
	t.Helper()
	fs, ok := w.FormFieldState("f", id)
	if !ok {
		t.Fatalf("field %q not registered", id)
	}
	return fs
}

func TestFormBlurValidatesWithoutWiring(t *testing.T) {
	d := &detectForm{}
	w := newDetectWindow(t, d, &detectApp{})
	detectFocus(t, w, "email")
	if _, err := w.TestTab(gui.TabForward); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	fs := detectField(t, w, "email")
	if !fs.Touched || len(fs.Errors) != 1 {
		t.Fatalf("after tab-away: touched=%v errors=%v; want touched and "+
			"one error", fs.Touched, fs.Errors)
	}
}

func TestFormChangeValidatesWhileFocused(t *testing.T) {
	d := &detectForm{cfg: gui.FormCfg{ValidateOn: gui.FormValidateOnChange}}
	w := newDetectWindow(t, d, &detectApp{})
	id := detectFocus(t, w, "email")
	if err := w.TestType(id, "bad"); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if fs := detectField(t, w, "email"); len(fs.Errors) != 1 {
		t.Fatalf("after typing %q: errors=%v, want one", fs.Value, fs.Errors)
	}
	if err := w.TestType(id, "x"); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if fs := detectField(t, w, "email"); len(fs.Errors) != 0 {
		t.Fatalf("after typing to %q: errors=%v, want none", fs.Value, fs.Errors)
	}
}

// A value the app sets while nothing has focus is not an edit: it is
// not validated then. Submit validates it.
func TestFormAppChangeIsCheckedOnSubmit(t *testing.T) {
	d := &detectForm{cfg: gui.FormCfg{ValidateOn: gui.FormValidateOnChange}}
	a := &detectApp{Email: "ok", Age: "3", Password: "p", Confirm: "p"}
	w := newDetectWindow(t, d, a)
	a.Email = "bad"
	w.TestRender(nil)
	if fs := detectField(t, w, "email"); len(fs.Errors) != 0 || fs.Touched {
		t.Fatalf("app change validated: touched=%v errors=%v", fs.Touched, fs.Errors)
	}
	gui.FormRequestSubmit(w, "f")
	w.TestRender(nil)
	if d.submits != 0 {
		t.Fatal("submit went through with an invalid value the app set")
	}
	if fs := detectField(t, w, "email"); len(fs.Errors) != 1 {
		t.Fatalf("after submit: errors=%v, want one", fs.Errors)
	}
}

func TestFormEnterSubmits(t *testing.T) {
	tests := []struct {
		name    string
		d       *detectForm
		focus   string
		want    int
		comment string
	}{
		{"single-line Input", &detectForm{}, "email", 1, ""},
		{"NumericInput", &detectForm{}, "age", 1, ""},
		{"NoSubmitOnEnter", &detectForm{cfg: gui.FormCfg{NoSubmitOnEnter: true}},
			"email", 0, "the form opted out"},
		{"OnEnter consumed", &detectForm{onEnter: func(ctx gui.EventCtx) { ctx.Consume() }},
			"email", 0, "the app's OnEnter kept the key"},
		{"OnEnter declined", &detectForm{onEnter: func(gui.EventCtx) {}},
			"email", 1, ""},
		{"multi-line Input", &detectForm{}, "notes", 0, "Enter is a newline"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &detectApp{Email: "ok", Age: "3", Password: "p", Confirm: "p"}
			w := newDetectWindow(t, tt.d, a)
			id := detectFocus(t, w, tt.focus)
			if err := w.TestKey(id, gui.KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
			w.TestRender(nil)
			if tt.d.submits != tt.want {
				t.Fatalf("OnSubmit ran %d times, want %d %s",
					tt.d.submits, tt.want, tt.comment)
			}
		})
	}
}

// NumericInput commits on blur through a deferred callback with no
// layout. The form still sees the blur, because it watches focus.
func TestFormNumericInputBlurValidates(t *testing.T) {
	d := &detectForm{}
	w := newDetectWindow(t, d, &detectApp{})
	detectFocus(t, w, "age")
	if _, err := w.TestTab(gui.TabForward); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if fs := detectField(t, w, "age"); !fs.Touched || len(fs.Errors) != 1 {
		t.Fatalf("after tab-away: touched=%v errors=%v", fs.Touched, fs.Errors)
	}
}

// An edit of the password re-checks the confirm field, which already
// showed a mismatch, while focus is still in the password field.
func TestFormEditRechecksCheckedFields(t *testing.T) {
	d := &detectForm{}
	w := newDetectWindow(t, d, &detectApp{})
	pw := detectFocus(t, w, "password")
	if err := w.TestType(pw, "ab"); err != nil {
		t.Fatal(err)
	}
	confirm := detectFocus(t, w, "confirm")
	if err := w.TestType(confirm, "abc"); err != nil {
		t.Fatal(err)
	}
	detectFocus(t, w, "email") // blur confirm: it is checked now
	if fs := detectField(t, w, "confirm"); len(fs.Errors) != 1 {
		t.Fatalf("confirm %q vs password %q: errors=%v, want one",
			fs.Value, "ab", fs.Errors)
	}
	pw = detectFocus(t, w, "password")
	if err := w.TestType(pw, "c"); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if fs := detectField(t, w, "confirm"); len(fs.Errors) != 0 {
		t.Fatalf("password now matches: confirm errors=%v, want none", fs.Errors)
	}
}

// A field set to validate on submit only keeps its issues until the
// next submit, even when a field its rule reads changes.
func TestFormEditSkipsSubmitOnlyFields(t *testing.T) {
	d := &detectForm{confirmMode: gui.FormValidateOnSubmit}
	a := &detectApp{Email: "ok", Age: "3", Confirm: "abc"}
	w := newDetectWindow(t, d, a)
	gui.FormRequestSubmit(w, "f")
	w.TestRender(nil)
	if fs := detectField(t, w, "confirm"); len(fs.Errors) != 1 {
		t.Fatalf("after submit: confirm errors=%v, want one", fs.Errors)
	}
	// Typed into an empty field, so the result is exactly "abc": the
	// caret of a field the app filled starts at the beginning.
	pw := detectFocus(t, w, "password")
	if err := w.TestType(pw, "abc"); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if fs := detectField(t, w, "confirm"); len(fs.Errors) != 1 {
		t.Fatalf("submit-only confirm was re-checked on a password edit: "+
			"errors=%v", fs.Errors)
	}
}

func TestFormWidgetIDLinksField(t *testing.T) {
	d := &detectForm{widgetID: "email-input"}
	w := newDetectWindow(t, d, &detectApp{})
	detectFocus(t, w, "email-input")
	if _, err := w.TestTab(gui.TabForward); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if fs := detectField(t, w, "email"); !fs.Touched || len(fs.Errors) != 1 {
		t.Fatalf("blur on the linked widget: touched=%v errors=%v",
			fs.Touched, fs.Errors)
	}
}

func TestFormResetRestoresThroughOnReset(t *testing.T) {
	d := &detectForm{}
	a := &detectApp{Email: "first"}
	w := newDetectWindow(t, d, a)
	id := detectFocus(t, w, "email")
	if err := w.TestType(id, "x"); err != nil {
		t.Fatal(err)
	}
	detectFocus(t, w, "age") // blur email: touched and checked
	gui.FormRequestReset(w, "f")
	w.TestRender(nil)
	w.TestRender(nil)
	fs := detectField(t, w, "email")
	if d.resets != 1 || a.Email != "first" || fs.Value != "first" ||
		fs.Dirty || fs.Touched || len(fs.Errors) != 0 {
		t.Fatalf("after reset: resets=%d app=%q field=%q dirty=%v "+
			"touched=%v errors=%v", d.resets, a.Email, fs.Value,
			fs.Dirty, fs.Touched, fs.Errors)
	}
}

func TestFormWithoutIDIsReported(t *testing.T) {
	w := gui.NewTestWindow(t, gui.WindowCfg{})
	w.TestRender(func(*gui.Window) gui.View {
		return gui.Form(gui.FormCfg{Content: []gui.View{ // requiredid:ignore — the ID-less case under test
			gui.Text(gui.TextCfg{ID: "hello", Text: "hello"}),
		}})
	})
	if len(w.ResolveID("hello")) != 1 {
		t.Error("an ID-less form must still show its content")
	}
	found := false
	for _, f := range w.TestFindings(gui.DebugMissingIDs) {
		if strings.Contains(f, "Form has no ID") {
			found = true
		}
	}
	if !found {
		t.Error("DebugMissingIDs did not report the ID-less form")
	}
}

// NumericInput clamps to Min and Max when it loses focus, in a deferred
// callback after the frame that saw the blur. The form validates the
// blur once that clamped value has arrived, so no error for the typed
// value is left on a field that no longer holds it.
func TestFormBlurValidatesCommittedValue(t *testing.T) {
	type st struct{ Age string }
	inRange := func(f gui.FormFieldSnapshot, _ gui.FormSnapshot) []gui.FormIssue {
		v, err := strconv.ParseFloat(f.Value, 64)
		if err != nil || v < 0 || v > 120 {
			return []gui.FormIssue{{Msg: "out of range: " + f.Value}}
		}
		return nil
	}
	for _, tt := range []struct{ typed, want string }{
		{"-5", "0"},
		{"150", "120"},
	} {
		t.Run(tt.typed, func(t *testing.T) {
			view := func(w *gui.Window) gui.View {
				a := gui.State[st](w)
				gui.FormRegisterFieldByID(w, "f", gui.FormFieldAdapterCfg{
					FieldID: "age", Value: a.Age,
					SyncValidators: []gui.FormSyncValidator{inRange},
				})
				return gui.Form(gui.FormCfg{ID: "f", Content: []gui.View{
					gui.NumericInput(gui.NumericInputCfg{
						ID: "age", Text: a.Age,
						Min: gui.Some(0.0), Max: gui.Some(120.0),
						OnTextChanged: func(s string, ctx gui.EventCtx) {
							gui.State[st](ctx.Window).Age = s
						},
						OnValueCommit: func(_ gui.Opt[float64], s string, ctx gui.EventCtx) {
							gui.State[st](ctx.Window).Age = s
						},
					}),
					gui.Input(gui.InputCfg{ID: "other"}),
				}})
			}
			w := gui.NewTestWindow(t, gui.WindowCfg{State: &st{}})
			w.TestRender(view)
			id := detectFocus(t, w, "age")
			if err := w.TestType(id, tt.typed); err != nil {
				t.Fatal(err)
			}
			if _, err := w.TestTab(gui.TabForward); err != nil {
				t.Fatal(err)
			}
			w.TestRender(nil)
			w.TestRender(nil)
			fs := detectField(t, w, "age")
			if fs.Value != tt.want || !fs.Touched || len(fs.Errors) != 0 {
				t.Fatalf("typed %q, tabbed away: value=%q touched=%v errors=%v; "+
					"want %q, touched, no errors", tt.typed, fs.Value,
					fs.Touched, fs.Errors, tt.want)
			}
		})
	}
}
