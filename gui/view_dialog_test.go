package gui

import "testing"

func TestDialogCfgDefaults(t *testing.T) {
	cfg := DialogCfg{}
	applyDialogDefaults(&cfg)

	if !cfg.Color.IsSet() {
		t.Error("expected non-zero Color")
	}
	if cfg.IDFocus != dialogBaseIDFocus {
		t.Errorf("expected IDFocus=%d, got %d",
			dialogBaseIDFocus, cfg.IDFocus)
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
	w.SetIDFocus(42)

	w.Dialog(DialogCfg{
		Title:      "Test",
		DialogType: DialogMessage,
	})
	if !w.DialogIsVisible() {
		t.Error("expected dialog visible after Dialog()")
	}
	if w.IDFocus() != dialogBaseIDFocus {
		t.Errorf("expected focus=%d, got %d",
			dialogBaseIDFocus, w.IDFocus())
	}

	w.DialogDismiss()
	if w.DialogIsVisible() {
		t.Error("expected dialog hidden after Dismiss()")
	}
	if w.IDFocus() != 42 {
		t.Errorf("expected focus restored to 42, got %d",
			w.IDFocus())
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
	handler(nil, e, w)

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
	handler(nil, e, w)

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
	handler(nil, e, w)

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
	handler(nil, e, w)

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
