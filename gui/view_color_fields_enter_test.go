package gui

import "testing"

// Enter in a color field applies the typed color and stays with the
// field. Since #1257, Enter in a single-line Input submits the Form
// around it unless a handler consumed the key, so the color fields
// consume it.
func TestColorFieldEnterDoesNotSubmitForm(t *testing.T) {
	// ColorFields builds its parts' IDs under its effective ID, which
	// sits in the form's scope.
	fields := ScopeID(formLayoutID("f"), "cf")
	for _, leaf := range []string{
		ScopeID(fields, "hex"),
		ScopeIDN(fields, "rgb", 0),
	} {
		t.Run(leaf, func(t *testing.T) {
			submits := 0
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(func(*Window) View {
				return Form(FormCfg{
					ID:       "f",
					OnSubmit: func(FormSubmitEvent, EventCtx) { submits++ },
					Content: []View{ColorFields(ColorFieldsCfg{
						ID:       "cf",
						OnChange: func(HSLA, EventCtx) {},
					})},
				})
			})
			ids := w.ResolveID(leaf)
			if len(ids) != 1 {
				t.Fatalf("ResolveID(%q) = %v, want one ID", leaf, ids)
			}
			if err := w.TestFocus(ids[0]); err != nil {
				t.Fatal(err)
			}
			w.TestRender(nil)
			if err := w.TestKey(ids[0], KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
			w.TestRender(nil)
			if submits != 0 {
				t.Fatalf("Enter in %s submitted the form %d times", leaf, submits)
			}
		})
	}
}
