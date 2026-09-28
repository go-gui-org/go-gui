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
	w.retainDialogFocus(&dialog, nil)

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
	w.retainDialogFocus(&dialog, nil)

	if got := w.FocusID(); got != yes {
		t.Fatalf("focus = %q, want %q (in-dialog focus preserved)", got, yes)
	}
}

// TestRetainDialogFocus_KeepsAboveDialogFocus verifies that focus on a
// layer above the dialog — a float lifted out of it (a Select dropdown,
// a menu, #819) or the inspector panel — is left alone, not reasserted
// to the dialog.
func TestRetainDialogFocus_KeepsAboveDialogFocus(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	dialog := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)

	// Stand-in for a lifted float layer: a focusable shape outside the
	// dialog subtree, the way an extracted dropdown sits above it.
	above := generateViewLayout(Button(ButtonCfg{
		ID:      "above-btn",
		Content: []View{Text(TextCfg{Text: "Above"})},
	}), w)

	w.SetFocus("above-btn")
	w.retainDialogFocus(&dialog, []*Layout{&above})

	if got := w.FocusID(); got != "above-btn" {
		t.Fatalf("focus = %q, want above-btn (above-dialog focus preserved)", got)
	}
}

// TestRetainDialogFocus_NoFocusReasserts verifies that no focus (id 0),
// which would spuriously match the dialog root, is treated as escaped.
func TestRetainDialogFocus_NoFocusReasserts(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	dialog := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)

	w.ClearFocus()
	w.retainDialogFocus(&dialog, nil)

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
	w.retainDialogFocus(nil, nil)
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
	w.retainDialogFocus(&Layout{}, nil)
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

// TestDialogEscapeWithFocusedChildKeyHandler drives Escape through full
// dispatch with a focused dialog child that holds its own OnKeyDown. A
// child that declines the key must not veto the dialog root's Escape
// handling: the dialog still dismisses and OnCancelNo still fires.
// Regression test: the per-dispatch dedup (markServed) once suppressed
// the dialog root whenever any focused child ran first, so Escape died
// while Enter (handled by the child itself) kept working.
func TestDialogEscapeWithFocusedChildKeyHandler(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "root"})
	})
	cancelled := false
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		FocusID:    "keys",
		OnCancelNo: func(_ *Window) { cancelled = true },
		CustomView: func(*Window) View {
			return Column(ContainerCfg{
				ID:        "keys",
				Focusable: true,
				OnKeyDown: func(ctx EventCtx) {
					// Declines everything: no Consume.
				},
			})
		},
	})
	w.TestRender(nil)
	if !w.DialogIsVisible() {
		t.Fatal("dialog did not open")
	}
	down := Event{Type: EventKeyDown, KeyCode: KeyEscape}
	w.EventFn(&down)
	w.TestRender(nil)
	if w.DialogIsVisible() {
		t.Error("dialog still visible after Escape")
	}
	if !cancelled {
		t.Error("OnCancelNo did not fire")
	}
	if !down.IsHandled {
		t.Error("expected Escape consumed by the dialog root")
	}
}

// TestDialogEscapeChildConsumeOverrides verifies the other half of the
// contract: a focused child that consumes Escape overrides the dialog
// root, so the dialog stays open. Post-order dispatch reaches the child
// first, and its Consume short-circuits before the dialog root runs.
func TestDialogEscapeChildConsumeOverrides(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "root"})
	})
	cancelled := false
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		FocusID:    "keys",
		OnCancelNo: func(_ *Window) { cancelled = true },
		CustomView: func(*Window) View {
			return Column(ContainerCfg{
				ID:        "keys",
				Focusable: true,
				OnKeyDown: func(ctx EventCtx) {
					if ctx.Event.KeyCode == KeyEscape {
						ctx.Consume()
					}
				},
			})
		},
	})
	w.TestRender(nil)
	if !w.DialogIsVisible() {
		t.Fatal("dialog did not open")
	}
	down := Event{Type: EventKeyDown, KeyCode: KeyEscape}
	w.EventFn(&down)
	w.TestRender(nil)
	if !w.DialogIsVisible() {
		t.Error("consuming child must override: dialog dismissed")
	}
	if cancelled {
		t.Error("OnCancelNo fired despite the override")
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
	cfg := DialogCfg{
		Title:      "Custom",
		DialogType: DialogCustom,
		CustomView: func(*Window) View {
			return Text(TextCfg{Text: "custom content"})
		},
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

// dialogTexts collects every text string in a dialog layout, in tree
// order, so a test can check what the dialog shows.
func dialogTexts(l *Layout, out []string) []string {
	if l.Shape != nil && l.Shape.TC != nil && l.Shape.TC.Text != "" {
		out = append(out, l.Shape.TC.Text)
	}
	for i := range l.Children {
		out = dialogTexts(&l.Children[i], out)
	}
	return out
}

// TestDialogCustomViewReadsStateEachFrame is the regression test for
// issue #787: content built by CustomView must read window state on
// every frame, not once at the Dialog() call.
func TestDialogCustomViewReadsStateEachFrame(t *testing.T) {
	type app struct{ text string }
	w := NewWindow(WindowCfg{State: &app{text: "before"}})
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		CustomView: func(w *Window) View {
			return Text(TextCfg{Text: State[app](w).text})
		},
	})

	first := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)
	if got := dialogTexts(&first, nil); len(got) != 1 || got[0] != "before" {
		t.Fatalf("first frame texts = %q, want [before]", got)
	}

	// A state change after Dialog() must show on the next frame.
	State[app](w).text = "after"
	second := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)
	if got := dialogTexts(&second, nil); len(got) != 1 || got[0] != "after" {
		t.Fatalf("second frame texts = %q, want [after]", got)
	}
}

// TestDialogCustomViewNilResult checks that a CustomView returning nil
// shows no content and does not panic.
func TestDialogCustomViewNilResult(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		Title:      "T",
		CustomView: func(*Window) View { return nil },
	})
	layout := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)
	if got := dialogTexts(&layout, nil); len(got) != 1 || got[0] != "T" {
		t.Fatalf("texts = %q, want [T]", got)
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
		DialogType: DialogCustom,
		CustomView: func(*Window) View {
			return Text(TextCfg{Text: "no buttons"})
		},
		OnCancelNo: func(_ *Window) { cancelled = true },
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

// TestDialogConfirmDefaultButtonOutOfRange pins the safe fallback now that
// DefaultButton is exported: the field is a plain uint8, so a caller can hand
// over a value that names no button. Anything that is not DialogButtonYes
// focuses the base id ("No"), never a scoped id with no shape behind it.
func TestDialogConfirmDefaultButtonOutOfRange(t *testing.T) {
	w := &Window{}
	w.Dialog(DialogCfg{
		DialogType:    DialogConfirm,
		Title:         "Quit?",
		DefaultButton: DialogButton(200),
	})
	if got := w.FocusID(); got != w.dialogCfg.FocusID {
		t.Fatalf("focus = %q, want base %q", got, w.dialogCfg.FocusID)
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
	w.retainDialogFocus(&dialog, nil)

	want := ScopeIDN(w.dialogCfg.FocusID, "", 1)
	if got := w.FocusID(); got != want {
		t.Fatalf("focus = %q, want Yes button %q", got, want)
	}
}

func TestDialogMarksLayoutRefresh(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.refreshLayout.Store(false)
	w.dialogCfg = DialogCfg{} // visible=false

	w.Dialog(DialogCfg{DialogType: DialogMessage, Title: "Hi"})
	if !w.refreshLayout.Load() {
		t.Error("Dialog should mark layout refresh")
	}
}

func TestDialogDismissMarksLayoutRefresh(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{DialogType: DialogMessage, Title: "Hi"})
	w.refreshLayout.Store(false)

	w.DialogDismiss()
	if !w.refreshLayout.Load() {
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
			w := NewTestWindow(t, WindowCfg{})
			t.Cleanup(w.WindowCleanup)
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

// Dialog buttons read their labels from the active locale, so a
// German app shows German buttons instead of hard-coded English.
func TestDialogButtonsUseLocaleStrings(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	SetLocale(LocaleDeDE)
	de := CurrentLocale()

	w := &Window{}
	for _, tt := range []struct {
		kind dialogType
		want []string
	}{
		{DialogMessage, []string{de.StrOK}},
		{DialogConfirm, []string{de.StrYes, de.StrNo}},
		{DialogPrompt, []string{de.StrOK, de.StrCancel}},
	} {
		layout := generateViewLayout(dialogViewGenerator(DialogCfg{
			Title: "T", Body: "B", DialogType: tt.kind,
		}), w)
		for _, want := range tt.want {
			if !mdHasText(layout, want) {
				t.Errorf("dialog type %d: missing button label %q", tt.kind, want)
			}
		}
	}
	if de.StrOK == "OK" && de.StrYes == "Yes" {
		t.Fatal("de-DE preset must translate OK/Yes for this test to mean anything")
	}
}

// arrangeDialog runs cfg through the full arrange pipeline and returns
// the dialog's resolved shape.
func arrangeDialog(t *testing.T, cfg DialogCfg) *Shape {
	t.Helper()
	w := newTestWindow()
	layout := generateViewLayout(dialogViewGenerator(cfg), w)
	layoutArrange(&layout, w)
	return layout.Shape
}

// The config from issue #708 must arrange at exactly the stated size.
// Before the fix the theme's MaxWidth (300) beat the caller's MinWidth
// and Width, and the dialog came out 300x400.
func TestDialogHonorsWidthHeightIssue708(t *testing.T) {
	s := arrangeDialog(t, DialogCfg{
		DialogType: DialogCustom,
		MinWidth:   SomeF(400),
		Width:      400,
		MaxHeight:  400,
		Height:     400,
		Title:      "Settings",
		CustomView: func(*Window) View { return Text(TextCfg{Text: "body"}) },
	})
	if s.Width != 400 || s.Height != 400 {
		t.Fatalf("dialog = %vx%v, want 400x400", s.Width, s.Height)
	}
}

// Height alone pins the height. Before the fix Height was only a seed
// on a Fit column, so the content height was added on top of it.
func TestDialogHeightIsFixed(t *testing.T) {
	s := arrangeDialog(t, DialogCfg{
		DialogType: DialogCustom,
		Height:     250,
		Title:      "Settings",
		CustomView: func(*Window) View { return Text(TextCfg{Text: "body"}) },
	})
	if s.Height != 250 {
		t.Fatalf("Height = %v, want 250", s.Height)
	}
}

// Width alone pins the width. Before the fix Width was only a seed
// on a Fit column, so the content width was added on top of it.
func TestDialogWidthIsFixed(t *testing.T) {
	s := arrangeDialog(t, DialogCfg{
		DialogType: DialogCustom,
		Width:      250,
		Title:      "Settings",
		CustomView: func(*Window) View { return Text(TextCfg{Text: "body"}) },
	})
	if s.Width != 250 {
		t.Fatalf("Width = %v, want 250", s.Width)
	}
}

// A caller MinWidth above the theme's MaxWidth wins, when the caller
// states no MaxWidth. A theme default must not override a caller value.
func TestDialogMinWidthBeatsThemeMaxWidth(t *testing.T) {
	want := DefaultDialogStyle.MaxWidth + 100
	s := arrangeDialog(t, DialogCfg{
		DialogType: DialogMessage,
		Body:       "short",
		MinWidth:   SomeF(want),
	})
	if s.Width != want {
		t.Fatalf("Width = %v, want %v", s.Width, want)
	}
}

// Without size fields the theme bounds still apply, so message text
// wraps inside MaxWidth.
func TestDialogThemeWidthBoundsStillApply(t *testing.T) {
	s := arrangeDialog(t, DialogCfg{DialogType: DialogMessage, Body: "short"})
	if s.Width < DefaultDialogStyle.MinWidth || s.Width > DefaultDialogStyle.MaxWidth {
		t.Fatalf("Width = %v, want within [%v, %v]", s.Width,
			DefaultDialogStyle.MinWidth, DefaultDialogStyle.MaxWidth)
	}
}

// openDialogWith renders an empty main view, then shows a DialogCustom
// whose body is content, and renders the frame that shows it.
func openDialogWith(t *testing.T, content func(*Window) View) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "root"})
	})
	w.Dialog(DialogCfg{DialogType: DialogCustom, CustomView: content})
	w.TestRender(nil)
	if !w.DialogIsVisible() {
		t.Fatal("dialog did not open")
	}
	return w
}

// resolveOne returns the one effective ID the last frame stamped for
// leaf.
func resolveOne(t *testing.T, w *Window, leaf string) string {
	t.Helper()
	ids := w.ResolveID(leaf)
	if len(ids) != 1 {
		t.Fatalf("ResolveID(%q) = %q, want one ID", leaf, ids)
	}
	return ids[0]
}

// TestDialogSelectOpens is the regression test for issue #810: a
// Select inside a dialog must open its dropdown and keep it open. The
// per-frame interaction fixup closed every select popup on each frame
// the dialog was visible, so the dropdown never showed.
func TestDialogSelectOpens(t *testing.T) {
	w := openDialogWith(t, func(*Window) View {
		return Select(SelectCfg{
			ID:          "time-select",
			Placeholder: "time",
			Options: []SelectOption{
				{Label: "seconds", Value: "seconds"},
				{Label: "minutes", Value: "minutes"},
				{Label: "hours", Value: "hours"},
			},
			OnSelect: func([]string, EventCtx) {},
		})
	})
	id := resolveOne(t, w, "time-select")
	if err := w.TestClick(id); err != nil {
		t.Fatal(err)
	}
	// Two more frames: the fixup runs on every one of them.
	w.TestRender(nil)
	w.TestRender(nil)
	if !StateReadOr(w, nsSelect, id, false) {
		t.Fatal("select dropdown closed after the click")
	}
	if _, ok := w.layout.FindByID(ScopeID(id, "dropdown")); !ok {
		t.Fatal("select dropdown not in the tree")
	}
}

// TestDialogSelectDropdownAboveDialog is the regression test for issue
// #819: a Select dropdown inside a dialog stayed nested in the dialog
// layer, so the dialog bounds and any scroll viewport clipped it. The
// dropdown must lift to its own layer above the dialog, the way a
// main-tree dropdown lifts above the app.
func TestDialogSelectDropdownAboveDialog(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "root"})
	})
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		Width:      300,
		Height:     150,
		Title:      "Settings",
		CustomView: func(*Window) View {
			return Column(ContainerCfg{
				ID:         "settings_content",
				Sizing:     FillFill,
				Scrollable: true,
				ScrollMode: ScrollVerticalOnly,
				Content: []View{
					Select(SelectCfg{
						ID:          "unit-select",
						Placeholder: "unit",
						Selected:    []string{"dollar"},
						Options: []SelectOption{
							{Label: "$", Value: "dollar"},
							{Label: "P", Value: "rub"},
							{Label: "E", Value: "euro"},
							{Label: "Y", Value: "yen"},
							{Label: "F", Value: "franc"},
							{Label: "L", Value: "lira"},
						},
						OnSelect: func([]string, EventCtx) {},
					}),
				},
			})
		},
	})
	w.TestRender(nil)
	if !w.DialogIsVisible() {
		t.Fatal("dialog did not open")
	}
	id := resolveOne(t, w, "unit-select")
	if err := w.TestClick(id); err != nil {
		t.Fatal(err)
	}
	// Two more frames: the dropdown must stay open and settled.
	w.TestRender(nil)
	w.TestRender(nil)
	if !StateReadOr(w, nsSelect, id, false) {
		t.Fatal("select dropdown closed after the click")
	}
	dropID := ScopeID(id, "dropdown")
	dialogIdx := layerIndex(w, reservedDialogID)
	if dialogIdx < 0 {
		t.Fatal("dialog layer not in the tree")
	}
	dropIdx := -1
	for i := range w.layout.Children {
		if _, found := w.layout.Children[i].findByID(dropID); found {
			if dropIdx >= 0 {
				t.Fatal("dropdown in more than one layer")
			}
			dropIdx = i
		}
	}
	if dropIdx < 0 {
		t.Fatal("select dropdown not in the tree")
	}
	if dropIdx == dialogIdx {
		t.Fatal("dropdown nested in the dialog layer, clipped by " +
			"dialog bounds and scroll viewport (#819)")
	}
	if dropIdx < dialogIdx {
		t.Fatal("dropdown layered below the dialog")
	}
	drop, found := w.layout.FindByID(dropID)
	if !found {
		t.Fatal("select dropdown not in the tree")
	}
	shape := drop.Shape
	clip := shape.shapeClip
	const eps = float32(0.001)
	if clip.X > shape.X+eps || clip.Y > shape.Y+eps ||
		clip.X+clip.Width < shape.X+shape.Width-eps ||
		clip.Y+clip.Height < shape.Y+shape.Height-eps {
		t.Fatalf("dropdown clip %v cuts shape bounds (%v, %v, %v, %v) (#819)",
			clip, shape.X, shape.Y, shape.Width, shape.Height)
	}
	// An option in the lifted layer must take the click: route the
	// press at the first option and the dropdown must close.
	opt := firstClickable(&w.layout.Children[dropIdx])
	if opt == nil {
		t.Fatal("no clickable option in the dropdown")
	}
	c := opt.Shape.shapeClip
	x, y := c.X+c.Width/2, c.Y+c.Height/2
	down := Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y}
	w.EventFn(&down)
	w.settle()
	up := Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: x, MouseY: y}
	w.EventFn(&up)
	w.settle()
	if StateReadOr(w, nsSelect, id, false) {
		t.Fatal("dropdown still open after an option click (#819)")
	}
}

// TestDialogComboboxOpens is the Combobox half of issue #810.
func TestDialogComboboxOpens(t *testing.T) {
	w := openDialogWith(t, func(*Window) View {
		return Combobox(ComboboxCfg{
			ID: "unit",
			Options: []SelectOption{
				{Label: "seconds", Value: "seconds"},
				{Label: "minutes", Value: "minutes"},
			},
			OnSelect: func(string, EventCtx) {},
		})
	})
	id := resolveOne(t, w, "unit")
	if err := w.TestClick(id); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	w.TestRender(nil)
	if !StateReadOr(w, nsCombobox, id, false) {
		t.Fatal("combobox dropdown closed after the click")
	}
}

// TestDialogKeepsDragLock checks that a mouse lock taken while a
// dialog is visible (a slider or scrollbar drag inside the dialog)
// survives the following frames. Only the dialog's opening ends a
// gesture (issue #810).
func TestDialogKeepsDragLock(t *testing.T) {
	w := openDialogWith(t, func(*Window) View {
		return Text(TextCfg{Text: "body"})
	})
	w.MouseLock(MouseLockCfg{MouseMove: func(EventCtx) {}})
	w.TestRender(nil)
	w.TestRender(nil)
	if !w.mouseIsLocked() {
		t.Fatal("mouse lock cancelled while the dialog stayed open")
	}
}

// TestDialogReopenCutsAgain checks the dismiss path: a Select opened in
// the main view after the first dialog was dismissed is closed when a
// second dialog opens (issue #810).
func TestDialogReopenCutsAgain(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	view := func(*Window) View {
		return Column(ContainerCfg{ID: "root", Content: []View{
			Select(SelectCfg{
				ID: "bg-select",
				Options: []SelectOption{
					{Label: "a", Value: "a"},
					{Label: "b", Value: "b"},
				},
				OnSelect: func([]string, EventCtx) {},
			}),
		}})
	}
	w.TestRender(view)
	w.Dialog(DialogCfg{DialogType: DialogMessage, Body: "one"})
	w.TestRender(nil)
	w.DialogDismiss()
	w.TestRender(nil)

	id := resolveOne(t, w, "bg-select")
	if err := w.TestClick(id); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if !StateReadOr(w, nsSelect, id, false) {
		t.Fatal("background select did not open")
	}

	w.Dialog(DialogCfg{DialogType: DialogMessage, Body: "two"})
	w.TestRender(nil)
	if StateReadOr(w, nsSelect, id, false) {
		t.Fatal("second dialog left the background select open")
	}
}

// TestDialogReplaceCutsAgain checks that Dialog called while a dialog is
// still visible cuts again, even when the new cfg is a copy of the shown
// one (so it carries interactionCut = true). A Select opened inside the
// first dialog must close when the replacement opens (issue #810).
func TestDialogReplaceCutsAgain(t *testing.T) {
	w := openDialogWith(t, func(*Window) View {
		return Select(SelectCfg{
			ID: "in-select",
			Options: []SelectOption{
				{Label: "a", Value: "a"},
				{Label: "b", Value: "b"},
			},
			OnSelect: func([]string, EventCtx) {},
		})
	})
	id := resolveOne(t, w, "in-select")
	if err := w.TestClick(id); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if !StateReadOr(w, nsSelect, id, false) {
		t.Fatal("select in the dialog did not open")
	}

	w.Dialog(w.dialogCfg)
	w.TestRender(nil)
	if StateReadOr(w, nsSelect, id, false) {
		t.Fatal("replacement dialog left the select open")
	}
}
