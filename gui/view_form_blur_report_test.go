package gui

import "testing"

// An app that still reports blur through the deprecated
// FormOnFieldEvent, here from OnEnter while the widget keeps focus
// (#1257). When the form finds the field's widget, the report does
// nothing and the form validates once, when focus really leaves. When
// it does not, the report validates at once, as it always did.
func TestFormOnFieldEventBlurReport(t *testing.T) {
	tests := []struct {
		name     string
		widgetID string
		// after Enter, then after Tab
		wantRuns    [2]int
		wantTouched [2]bool
	}{
		{"widget found", "email", [2]int{0, 1}, [2]bool{false, true}},
		{"widget not found", "w-email", [2]int{1, 1}, [2]bool{true, true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := 0
			counting := func(FormFieldSnapshot, FormSnapshot) []FormIssue {
				runs++
				return nil
			}
			cfg := FormFieldAdapterCfg{
				FieldID: "email", Value: "a@b",
				SyncValidators: []FormSyncValidator{counting},
			}
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(func(w *Window) View {
				FormRegisterFieldByID(w, "f", cfg)
				return Form(FormCfg{ID: "f", Content: []View{
					Input(InputCfg{ID: tt.widgetID, Text: "a@b",
						OnEnter: func(ctx EventCtx) {
							FormOnFieldEvent(ctx.Window, ctx.Layout, cfg,
								FormTriggerBlur)
							ctx.Consume()
						}}),
					Input(InputCfg{ID: "other"}),
				}})
			})
			id := ScopeID(formLayoutID("f"), tt.widgetID)
			if err := w.TestFocus(id); err != nil {
				t.Fatal(err)
			}
			w.TestRender(nil)

			check := func(step int, when string) {
				t.Helper()
				fs, _ := w.FormFieldState("f", "email")
				if runs != tt.wantRuns[step] || fs.Touched != tt.wantTouched[step] {
					t.Fatalf("after %s: validator ran %d times, touched=%v; "+
						"want %d, %v", when, runs, fs.Touched,
						tt.wantRuns[step], tt.wantTouched[step])
				}
			}
			if err := w.TestKey(id, KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
			w.TestRender(nil)
			check(0, "Enter")
			if _, err := w.TestTab(TabForward); err != nil {
				t.Fatal(err)
			}
			w.TestRender(nil)
			check(1, "Tab")
		})
	}
}
