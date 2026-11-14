package gui

import "testing"

// formFindAncestorID finds the nearest form above a layout. A "form:"
// prefix alone is not enough: a part of a composite widget inside a
// form carries an absolute ID under the form's scope, so the lookup
// also requires that the form exists (#1257).
func TestFormFindAncestorID(t *testing.T) {
	w := newTestWindow()
	formRuntime(w, "outer")
	formRuntime(w, "my-form")

	chain := func(ids ...string) *Layout {
		var parent *Layout
		for _, id := range ids {
			parent = &Layout{Shape: &Shape{ID: id}, Parent: parent}
		}
		return parent
	}
	tests := []struct {
		name string
		leaf *Layout
		want string
	}{
		{"plain child", chain("form:my-form", "row", "input"), "my-form"},
		{"no form", chain("panel", "input"), ""},
		{
			// ColorFields inside a form: the hex field and its
			// container both start with "form:my-form:".
			"absolute part ID",
			chain("form:my-form", "form:my-form:cf", "form:my-form:cf:hex"),
			"my-form",
		},
		{
			"nested form takes the inner one",
			chain("form:outer", "panel", "form:my-form", "input"),
			"my-form",
		},
		{"prefix with no such form", chain("form:ghost", "input"), ""},
		{"nil layout", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formFindAncestorID(w, tt.leaf); got != tt.want {
				t.Fatalf("formFindAncestorID = %q, want %q", got, tt.want)
			}
		})
	}
}

// Moved from view_form_test.go when the lookup started to require the
// form's state. formRuntime stands in for the form's first frame,
// which creates that state before any event can reach it.

func TestFormRegisterFieldViaLayout(t *testing.T) {
	w := newTestWindow()
	formRuntime(w, "layout-form")
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

func TestFormOnFieldEventViaLayout(t *testing.T) {
	w := newTestWindow()
	formRuntime(w, "evt-form")
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
