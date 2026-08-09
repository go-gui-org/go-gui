package gui

import "testing"

// TestRetainDialogFocus_RestoresStolenFocus verifies that when a
// focus-claiming widget steals idFocus away from a visible modal dialog,
// retainDialogFocus reasserts the dialog's focus id so keyboard routing
// (Tab/Esc/Enter) keeps working.
func TestRetainDialogFocus_RestoresStolenFocus(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	dialog := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)

	// Simulate a widget re-asserting focus onto itself (id 42, not in
	// the dialog subtree).
	w.SetFocus("f42")
	w.retainDialogFocus(&dialog)

	if got := w.FocusID(); got != w.dialogCfg.FocusID {
		t.Fatalf("focus = %q, want dialog focus %q", got, w.dialogCfg.FocusID)
	}
}

// TestRetainDialogFocus_KeepsDialogFocus verifies focus already inside
// the dialog subtree (e.g. after Tab) is left untouched.
func TestRetainDialogFocus_KeepsDialogFocus(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	dialog := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)

	// Confirm dialog's "Yes" button uses IDFocus+1; a legitimate Tab
	// target inside the dialog must be preserved.
	yes := ScopeIDN(w.dialogCfg.FocusID, "", 1)
	w.SetFocus(yes)
	w.retainDialogFocus(&dialog)

	if got := w.FocusID(); got != yes {
		t.Fatalf("focus = %q, want %q (in-dialog focus preserved)", got, yes)
	}
}

// TestRetainDialogFocus_NoFocusReasserts verifies that no focus (id 0),
// which would spuriously match the dialog root, is treated as escaped.
func TestRetainDialogFocus_NoFocusReasserts(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	dialog := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)

	w.ClearFocus()
	w.retainDialogFocus(&dialog)

	if got := w.FocusID(); got != w.dialogCfg.FocusID {
		t.Fatalf("focus = %q, want dialog focus %q", got, w.dialogCfg.FocusID)
	}
}

// TestRetainDialogFocus_NilLayoutNoPanic verifies a nil dialog layer is
// a no-op (no panic, focus untouched) rather than dereferencing it.
func TestRetainDialogFocus_NilLayoutNoPanic(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	w.SetFocus("f42")
	w.retainDialogFocus(nil)
	if got := w.FocusID(); got != "f42" {
		t.Fatalf("focus = %q, want f42 (nil layer must not touch focus)", got)
	}
}

// TestRetainDialogFocus_NilShapeNoPanic verifies a layout with a nil
// Shape (which FindLayoutByFocusID would dereference) is a no-op.
func TestRetainDialogFocus_NilShapeNoPanic(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	w.SetFocus("f42")
	w.retainDialogFocus(&Layout{})
	if got := w.FocusID(); got != "f42" {
		t.Fatalf("focus = %q, want f42 (nil Shape must not touch focus)", got)
	}
}

func TestDialogCfgDefaults(t *testing.T) {
	cfg := DialogCfg{}
	applyDialogDefaults(&cfg)

	if !cfg.Color.IsSet() {
		t.Error("expected non-zero Color")
	}
	// Scoped under the dialog container, which is where the control
	// carrying it actually sits — a bare leaf would resolve under that
	// container while SetFocus kept passing the leaf.
	wantFocus := ScopeID(reservedDialogID, dialogBaseFocusID)
	if cfg.FocusID != wantFocus {
		t.Errorf("expected FocusID=%q, got %q", wantFocus, cfg.FocusID)
	}
	if cfg.MinWidth.IsSet() {
		t.Error("expected MinWidth unset (resolved from style)")
	}
	if cfg.MaxWidth.IsSet() {
		t.Error("expected MaxWidth unset (resolved from style)")
	}
}

func TestDialogViewGeneratorReturnsView(t *testing.T) {
	cfg := DialogCfg{
		Title:      "Test Dialog",
		Body:       "Some body text",
		DialogType: DialogMessage,
	}
	v := dialogViewGenerator(cfg)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	w := &Window{}
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected non-nil shape")
	}
	if layout.Shape.ID != reservedDialogID {
		t.Errorf("expected ID=%q, got %q",
			reservedDialogID, layout.Shape.ID)
	}
}

func TestDialogShowDismissLifecycle(t *testing.T) {
	w := &Window{}
	w.SetFocus("f42")

	w.Dialog(DialogCfg{
		Title:      "Test",
		DialogType: DialogMessage,
	})
	if !w.DialogIsVisible() {
		t.Error("expected dialog visible after Dialog()")
	}
	if want := ScopeID(reservedDialogID, dialogBaseFocusID); w.FocusID() != want {
		t.Errorf("expected focus=%q, got %q", want, w.FocusID())
	}

	w.DialogDismiss()
	if w.DialogIsVisible() {
		t.Error("expected dialog hidden after Dismiss()")
	}
	if w.FocusID() != "f42" {
		t.Errorf("expected focus restored to f42, got %q",
			w.FocusID())
	}
}

func TestDialogKeyDownEscape(t *testing.T) {
	w := &Window{}
	cancelled := false
	cfg := DialogCfg{
		OnCancelNo: func(_ *Window) { cancelled = true },
	}
	handler := dialogKeyDown(cfg)
	e := &Event{KeyCode: KeyEscape}
	handler(EventCtx{nil, e, w})

	if !cancelled {
		t.Error("expected OnCancelNo to fire")
	}
	if !e.IsHandled {
		t.Error("expected event handled")
	}
}

func TestDialogKeyDownCtrlCCopiesBody(t *testing.T) {
	w := newTestWindow()
	var clipped string
	w.SetClipboardFn(func(s string) { clipped = s })

	cfg := DialogCfg{Body: "hello world"}
	handler := dialogKeyDown(cfg)
	e := &Event{KeyCode: KeyC, Modifiers: ModCtrl}
	handler(EventCtx{nil, e, w})

	if clipped != "hello world" {
		t.Fatalf("expected clipboard=%q got %q",
			"hello world", clipped)
	}
	if !e.IsHandled {
		t.Fatal("expected IsHandled=true")
	}
}

func TestDialogKeyDownSuperCCopiesBody(t *testing.T) {
	w := newTestWindow()
	var clipped string
	w.SetClipboardFn(func(s string) { clipped = s })

	cfg := DialogCfg{Body: "mac copy"}
	handler := dialogKeyDown(cfg)
	e := &Event{KeyCode: KeyC, Modifiers: ModSuper}
	handler(EventCtx{nil, e, w})

	if clipped != "mac copy" {
		t.Fatalf("expected clipboard=%q got %q",
			"mac copy", clipped)
	}
}

func TestDialogKeyDownCtrlCNoOpWhenBodyEmpty(t *testing.T) {
	w := newTestWindow()
	called := false
	w.SetClipboardFn(func(string) { called = true })

	handler := dialogKeyDown(DialogCfg{})
	e := &Event{KeyCode: KeyC, Modifiers: ModCtrl}
	handler(EventCtx{nil, e, w})

	if called {
		t.Fatal("clipboard should not be set when body empty")
	}
	if e.IsHandled {
		t.Fatal("expected IsHandled=false for empty body")
	}
}

func TestDialogPromptView(t *testing.T) {
	cfg := DialogCfg{
		Title:      "Enter name",
		Body:       "Name:",
		Reply:      "Alice",
		DialogType: DialogPrompt,
	}
	v := dialogViewGenerator(cfg)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	w := &Window{}
	layout := generateViewLayout(v, w)
	if len(layout.Children) < 3 {
		t.Fatalf("expected >=3 children (title+body+input+buttons), got %d",
			len(layout.Children))
	}
}

func TestDialogCustomView(t *testing.T) {
	custom := Text(TextCfg{Text: "custom content"})
	cfg := DialogCfg{
		Title:         "Custom",
		DialogType:    DialogCustom,
		CustomContent: []View{custom},
	}
	v := dialogViewGenerator(cfg)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	w := &Window{}
	layout := generateViewLayout(v, w)
	// Title + custom content.
	if len(layout.Children) < 2 {
		t.Fatalf("expected >=2 children, got %d", len(layout.Children))
	}
}

func TestDialogDefaultsPreserveUserSet(t *testing.T) {
	cfg := DialogCfg{
		Color:        RGBA(255, 0, 0, 255),
		AlignButtons: HAlignRight,
		MinWidth:     SomeF(400),
		MaxWidth:     SomeF(600),
	}
	applyDialogDefaults(&cfg)
	if cfg.Color != (RGBA(255, 0, 0, 255)) {
		t.Error("Color was overwritten")
	}
	if cfg.AlignButtons != HAlignRight {
		t.Error("AlignButtons was overwritten")
	}
	if cfg.MinWidth.Get(0) != 400 {
		t.Errorf("MinWidth was overwritten: %v", cfg.MinWidth)
	}
	if cfg.MaxWidth.Get(0) != 600 {
		t.Errorf("MaxWidth was overwritten: %v", cfg.MaxWidth)
	}
}

func TestDialogCustomEscapeDismisses(t *testing.T) {
	w := newTestWindow()
	cancelled := false
	w.Dialog(DialogCfg{
		DialogType:    DialogCustom,
		CustomContent: []View{Text(TextCfg{Text: "no buttons"})},
		OnCancelNo:    func(_ *Window) { cancelled = true },
	})
	if !w.DialogIsVisible() {
		t.Fatal("dialog should be visible")
	}

	v := dialogViewGenerator(w.dialogCfg)
	layout := generateViewLayout(v, w)
	e := &Event{Type: EventKeyDown, KeyCode: KeyEscape}
	keydownHandler(&layout, e, w)

	if !e.IsHandled {
		t.Error("Escape should be handled")
	}
	if !cancelled {
		t.Error("OnCancelNo should fire")
	}
}

func TestDialogAlignButtonsLeft(t *testing.T) {
	cfg := DialogCfg{AlignButtons: HAlignLeft}
	applyDialogDefaults(&cfg)
	if cfg.AlignButtons != HAlignLeft {
		t.Errorf("expected HAlignLeft, got %d", cfg.AlignButtons)
	}
}

func TestDialogMinMaxWidthResolved(t *testing.T) {
	cfg := DialogCfg{DialogType: DialogMessage}
	v := dialogViewGenerator(cfg)
	w := &Window{}
	layout := generateViewLayout(v, w)
	s := layout.Shape
	if s.MinWidth != DefaultDialogStyle.MinWidth {
		t.Errorf("MinWidth=%f, want %f",
			s.MinWidth, DefaultDialogStyle.MinWidth)
	}
	if s.MaxWidth != DefaultDialogStyle.MaxWidth {
		t.Errorf("MaxWidth=%f, want %f",
			s.MaxWidth, DefaultDialogStyle.MaxWidth)
	}
}

func TestDialogConfirmDefaultButtonNo(t *testing.T) {
	w := &Window{}
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	// Default (DialogButtonNo) focuses the base IDFocus ("No").
	if got := w.FocusID(); got != w.dialogCfg.FocusID {
		t.Fatalf("focus = %q, want No button %q", got, w.dialogCfg.FocusID)
	}
}

func TestDialogConfirmDefaultButtonYes(t *testing.T) {
	w := &Window{}
	w.Dialog(DialogCfg{
		DialogType:    DialogConfirm,
		Title:         "Quit?",
		DefaultButton: DialogButtonYes,
	})
	// DialogButtonYes focuses IDFocus+1 ("Yes").
	want := ScopeIDN(w.dialogCfg.FocusID, "", 1)
	if got := w.FocusID(); got != want {
		t.Fatalf("focus = %q, want Yes button %q", got, want)
	}
}

// TestDialogDefaultButtonYesIgnoredForNonConfirm verifies DefaultButton
// only affects confirm dialogs; a message dialog still focuses its base id.
func TestDialogDefaultButtonYesIgnoredForNonConfirm(t *testing.T) {
	w := &Window{}
	w.Dialog(DialogCfg{
		DialogType:    DialogMessage,
		Title:         "Done",
		DefaultButton: DialogButtonYes,
	})
	if got := w.FocusID(); got != w.dialogCfg.FocusID {
		t.Fatalf("focus = %q, want base %q", got, w.dialogCfg.FocusID)
	}
}

// TestRetainDialogFocus_DefaultButtonYes verifies focus reassertion
// honors DefaultButton: a confirm dialog defaulting to Yes reasserts the
// Yes button id, not the base id.
func TestRetainDialogFocus_DefaultButtonYes(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{
		DialogType:    DialogConfirm,
		Title:         "Quit?",
		DefaultButton: DialogButtonYes,
	})
	dialog := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)

	w.SetFocus("f42") // steal focus outside the dialog
	w.retainDialogFocus(&dialog)

	want := ScopeIDN(w.dialogCfg.FocusID, "", 1)
	if got := w.FocusID(); got != want {
		t.Fatalf("focus = %q, want Yes button %q", got, want)
	}
}

func TestDialogMarksLayoutRefresh(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.refreshLayout = false
	w.dialogCfg = DialogCfg{} // visible=false

	w.Dialog(DialogCfg{DialogType: DialogMessage, Title: "Hi"})
	if !w.refreshLayout {
		t.Error("Dialog should mark layout refresh")
	}
}

func TestDialogDismissMarksLayoutRefresh(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogMessage, Title: "Hi"})
	w.refreshLayout = false

	w.DialogDismiss()
	if !w.refreshLayout {
		t.Error("DialogDismiss should mark layout refresh")
	}
}

func TestDialogDoubleDismissNoPanic(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogMessage, Title: "Hi"})
	w.DialogDismiss()
	// Second dismiss must not panic.
	w.DialogDismiss()
}

func TestDialogConfirmView(t *testing.T) {
	cfg := DialogCfg{
		Title:      "Confirm?",
		Body:       "Are you sure?",
		DialogType: DialogConfirm,
	}
	v := dialogViewGenerator(cfg)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	w := &Window{}
	layout := generateViewLayout(v, w)
	if len(layout.Children) == 0 {
		t.Error("expected children for confirm dialog")
	}
}

// The dialog's controls sit under a container carrying
// reservedDialogID, while SetFocus and retainDialogFocus run outside
// layout generation and pass dialogFocusID directly. If those two
// disagree the dialog renders but keyboard focus lands nowhere: Enter
// does not trigger the default button and a prompt swallows typing.
// Every dialog shape must therefore be addressable by the ID the focus
// path uses.
func TestDialogFocusTargetIsAddressable(t *testing.T) {
	tests := []struct {
		name string
		cfg  DialogCfg
	}{
		{"message", DialogCfg{Title: "t", Body: "m"}},
		{"confirm", DialogCfg{
			Title: "t", Body: "m", DialogType: DialogConfirm}},
		{"confirm defaulting to yes", DialogCfg{
			Title:      "t",
			DialogType: DialogConfirm,
			// The Yes button is the ScopeIDN-composed sibling, so this
			// covers the composed arm of dialogFocusID too.
			DefaultButton: DialogButtonYes,
		}},
		{"prompt", DialogCfg{Title: "t", DialogType: DialogPrompt}},
		{"caller-supplied focus id", DialogCfg{
			Title: "t", Body: "m", FocusID: "mine"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := NewTestWindow(WindowCfg{})
			w.Dialog(tc.cfg)
			root := w.TestRender(func(_ *Window) View {
				return Column(ContainerCfg{Sizing: FillFill})
			})
			want := dialogFocusID(w.dialogCfg)
			if _, ok := root.FindByID(want); !ok {
				t.Fatalf("no shape with the focus ID %q", want)
			}
			if got := w.FocusID(); got != want {
				t.Fatalf("FocusID() = %q, want %q", got, want)
			}
		})
	}
}
