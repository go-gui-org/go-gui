package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestNewWindowSetsFields(t *testing.T) {
	type S struct{ X int }
	st := &S{X: 42}
	w := NewWindow(WindowCfg{
		State:  st,
		Title:  "test",
		Width:  800,
		Height: 600,
	})
	if w.windowWidth != 800 {
		t.Errorf("width = %d, want 800", w.windowWidth)
	}
	if w.windowHeight != 600 {
		t.Errorf("height = %d, want 600", w.windowHeight)
	}
	if !w.focused {
		t.Error("want focused=true")
	}
	if !w.refreshLayout {
		t.Error("want refreshLayout=true")
	}
	if State[S](w).X != 42 {
		t.Errorf("state.X = %d, want 42", State[S](w).X)
	}
	if w.Config.Title != "test" {
		t.Errorf("Config.Title = %q, want test", w.Config.Title)
	}
}

func TestUpdateViewSetsGenerator(t *testing.T) {
	w := NewWindow(WindowCfg{Width: 100, Height: 100})
	called := false
	w.UpdateView(func(_ *Window) View {
		called = true
		return Text(TextCfg{Text: "hi"})
	})
	if w.viewGenerator == nil {
		t.Fatal("viewGenerator nil after UpdateView")
	}
	if !w.refreshLayout {
		t.Error("want refreshLayout=true after UpdateView")
	}
	// Call generator to verify it works.
	w.viewGenerator(w)
	if !called {
		t.Error("generator not called")
	}
}

func TestFrameFnCallsUpdate(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  100,
		Height: 100,
	})
	updated := false
	w.viewGenerator = func(_ *Window) View {
		updated = true
		return Text(TextCfg{Text: "x"})
	}
	w.refreshLayout = true
	got := w.FrameFn()
	if !updated {
		t.Error("FrameFn did not call Update")
	}
	if w.refreshLayout {
		t.Error("refreshLayout should be cleared")
	}
	if !got {
		t.Error("FrameFn should return true when layout refreshed")
	}
}

func TestFrameFnNoopWhenNoRefresh(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  100,
		Height: 100,
	})
	called := false
	w.viewGenerator = func(_ *Window) View {
		called = true
		return Text(TextCfg{Text: "x"})
	}
	w.refreshLayout = false
	w.refreshRenderOnly = false
	got := w.FrameFn()
	if called {
		t.Error("FrameFn should not call generator when no refresh")
	}
	if got {
		t.Error("FrameFn should return false when no refresh")
	}
}

func TestFrameFnReturnsTrueOnRenderOnly(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  100,
		Height: 100,
	})
	// Build initial layout so UpdateRenderOnly has something.
	w.viewGenerator = func(_ *Window) View {
		return Text(TextCfg{Text: "x"})
	}
	w.refreshLayout = true
	w.FrameFn()

	w.refreshRenderOnly = true
	got := w.FrameFn()
	if !got {
		t.Error("FrameFn should return true on render-only refresh")
	}
	if w.refreshRenderOnly {
		t.Error("refreshRenderOnly should be cleared")
	}
}

func TestRenderTextEmitsCommand(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	w.viewGenerator = func(_ *Window) View {
		return Text(TextCfg{
			Text: "hello",
			TextStyle: TextStyle{
				Color: RGB(255, 255, 255),
				Size:  16,
			},
		})
	}
	w.refreshLayout = true
	w.FrameFn()

	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderText && r.Text == "hello" {
			found = true
			if r.FontSize != 16 {
				t.Errorf("FontSize = %f, want 16", r.FontSize)
			}
			break
		}
	}
	if !found {
		t.Error("no RenderText command with text 'hello'")
	}
}

func TestTextFallbackMeasurement(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	// No textMeasurer set — should use placeholder.
	tv := Text(TextCfg{
		Text: "test",
		TextStyle: TextStyle{
			Size: 20,
		},
	})
	layout := tv.GenerateLayout(w)
	wantW := float32(len("test")) * 20 * 0.6
	if layout.Shape.Width != wantW {
		t.Errorf("width = %f, want %f", layout.Shape.Width, wantW)
	}
	wantH := float32(20 * 1.4)
	if layout.Shape.Height != wantH {
		t.Errorf("height = %f, want %f", layout.Shape.Height, wantH)
	}
}

type mockTextMeasurer struct{}

func (m *mockTextMeasurer) TextWidth(text string, _ TextStyle) float32 {
	return float32(len(text)) * 10
}
func (m *mockTextMeasurer) TextHeight(_ string, _ TextStyle) float32 {
	return 20
}
func (m *mockTextMeasurer) FontAscent(s TextStyle) float32 { return s.Size * 0.8 }
func (m *mockTextMeasurer) FontHeight(_ TextStyle) float32 {
	return 22
}
func (m *mockTextMeasurer) LayoutText(_ string, _ TextStyle, _ float32) (glyph.Layout, error) {
	return glyph.Layout{Height: 22}, nil
}

func TestTextWithMeasurer(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	w.SetTextMeasurer(&mockTextMeasurer{})

	tv := Text(TextCfg{
		Text: "abc",
		TextStyle: TextStyle{
			Size: 16,
		},
	})
	layout := tv.GenerateLayout(w)
	if layout.Shape.Width != 30 {
		t.Errorf("width = %f, want 30", layout.Shape.Width)
	}
	if layout.Shape.Height != 22 {
		t.Errorf("height = %f, want 22", layout.Shape.Height)
	}
}

func TestWindowTextWidthFallback(t *testing.T) {
	w := NewWindow(WindowCfg{Width: 50, Height: 50})
	got := w.TextWidth("test", TextStyle{Size: 20})
	want := float32(len("test")) * 20 * 0.6
	if got != want {
		t.Errorf("TextWidth() = %f, want %f", got, want)
	}
}

func TestWindowTextWidthUsesMeasurer(t *testing.T) {
	w := NewWindow(WindowCfg{Width: 50, Height: 50})
	w.SetTextMeasurer(&mockTextMeasurer{})

	got := w.TextWidth("abcd", TextStyle{Size: 16})
	if got != 40 {
		t.Errorf("TextWidth() = %f, want 40", got)
	}
}

func TestRenderersAccessor(t *testing.T) {
	w := NewWindow(WindowCfg{Width: 50, Height: 50})
	w.renderers = append(w.renderers, RenderCmd{Kind: RenderRect})
	r := w.Renderers()
	if len(r) != 1 || r[0].Kind != RenderRect {
		t.Error("Renderers() mismatch")
	}
}

func TestMouseCursorStateAccessor(t *testing.T) {
	w := NewWindow(WindowCfg{Width: 50, Height: 50})
	w.setMouseCursor(CursorIBeam)
	if w.MouseCursorState() != CursorIBeam {
		t.Errorf("got %d, want CursorIBeam", w.MouseCursorState())
	}
}

func TestPasswordMaskInRenderText(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	shape := &Shape{
		shapeType: shapeText,
		Width:     100,
		Height:    20,
		Opacity:   1.0,
		TC: &shapeTextConfig{
			Text:           "secret",
			textIsPassword: true,
			TextStyle:      &TextStyle{Color: RGB(255, 255, 255), Size: 16},
		},
	}
	clip := drawClip{X: 0, Y: 0, Width: 200, Height: 200}
	renderText(shape, clip, w)

	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderText {
			found = true
			for _, ch := range r.Text {
				if ch != '•' {
					t.Errorf("expected password char, got %c", ch)
				}
			}
		}
	}
	if !found {
		t.Error("no RenderText command emitted")
	}
}

func TestRenderTextWrapSetsWidth(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	shape := &Shape{
		shapeType: shapeText,
		Width:     250,
		Height:    20,
		Opacity:   1.0,
		TC: &shapeTextConfig{
			Text:     "wrap me",
			TextMode: TextModeWrap,
			TextStyle: &TextStyle{
				Color: RGB(255, 255, 255), Size: 16,
			},
		},
	}
	clip := drawClip{X: 0, Y: 0, Width: 400, Height: 400}
	renderText(shape, clip, w)

	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderText && r.Text == "wrap me" {
			found = true
			if r.W != 250 {
				t.Errorf("W = %f, want 250", r.W)
			}
		}
	}
	if !found {
		t.Error("no RenderText command emitted")
	}
}

func TestRenderTextNoWrapOmitsWidth(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	shape := &Shape{
		shapeType: shapeText,
		Width:     250,
		Height:    20,
		Opacity:   1.0,
		TC: &shapeTextConfig{
			Text: "no wrap",
			TextStyle: &TextStyle{
				Color: RGB(255, 255, 255), Size: 16,
			},
		},
	}
	clip := drawClip{X: 0, Y: 0, Width: 400, Height: 400}
	renderText(shape, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderText && r.Text == "no wrap" {
			if r.W != 0 {
				t.Errorf("W = %f, want 0 for non-wrap text",
					r.W)
			}
		}
	}
}

func TestQueueCommandWakesMain(t *testing.T) {
	w := &Window{}
	woken := false
	w.wakeMainFn = func() { woken = true }
	w.QueueCommand(func(_ *Window) {})
	if !woken {
		t.Error("QueueCommand should call wakeMain")
	}
}

// TestRefreshRequestsWakeMain pins the contract that setting a refresh
// flag also wakes the backend. Backends block indefinitely when FrameFn
// reports nothing to draw, so a request that does not wake is only
// painted when unrelated input happens to arrive — a frozen window for
// any async caller (background samplers, image downloads, datagrid data
// sources). macOS masked this for a long time because AppKit's idle
// event traffic ends the block on its own; X11 is genuinely silent.
func TestRefreshRequestsWakeMain(t *testing.T) {
	tests := []struct {
		name string
		call func(*Window)
		// The flag each entry must raise. Asserted alongside the wake so
		// the test cannot pass on a wake that schedules nothing — the two
		// halves are only useful together.
		wantLayout     bool
		wantRenderOnly bool
	}{
		{
			name:       "UpdateWindow",
			call:       func(w *Window) { w.UpdateWindow() },
			wantLayout: true,
		},
		{
			name:           "RequestRedraw",
			call:           func(w *Window) { w.RequestRedraw() },
			wantRenderOnly: true,
		},
		{
			name: "UpdateView",
			call: func(w *Window) {
				w.UpdateView(func(*Window) View { return nil })
			},
			wantLayout: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWindow(WindowCfg{})
			// markRenderOnlyRefresh defers to a pending full rebuild, so
			// start from a clean slate rather than whatever NewWindow left.
			w.refreshLayout = false
			w.refreshRenderOnly = false
			wakes := 0
			w.wakeMainFn = func() { wakes++ }
			tt.call(w)
			if wakes != 1 {
				t.Errorf("wakeMain called %d times, want 1", wakes)
			}
			if w.refreshLayout != tt.wantLayout {
				t.Errorf("refreshLayout = %v, want %v",
					w.refreshLayout, tt.wantLayout)
			}
			if w.refreshRenderOnly != tt.wantRenderOnly {
				t.Errorf("refreshRenderOnly = %v, want %v",
					w.refreshRenderOnly, tt.wantRenderOnly)
			}
		})
	}
}

func TestSetClipboard(t *testing.T) {
	w := &Window{}
	var got string
	w.SetClipboardFn(func(s string) { got = s })
	w.SetClipboard("hello")
	if got != "hello" {
		t.Errorf("clipboard = %q, want hello", got)
	}
}

func TestSetClipboardNilSafe(t *testing.T) {
	_ = t
	w := &Window{}
	// Should not panic when no fn set.
	w.SetClipboard("ignored")
}

func TestSetIDFocusClearsInputSelections(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	setInputState(w, "f10", inputState{selectBeg: 1, selectEnd: 5})
	w.SetFocus("f20")

	is := getInputState(w, "f10")
	if is.selectBeg != 0 || is.selectEnd != 0 {
		t.Errorf("selection not cleared: beg=%d end=%d",
			is.selectBeg, is.selectEnd)
	}
}

func TestSetIDFocusEnablesCursorBlink(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	w.viewState.inputCursorOn.Store(false)
	w.SetFocus("f42")

	if !w.viewState.inputCursorOn.Load() {
		t.Error("inputCursorOn should be true after SetIDFocus")
	}
}

func TestMouseLockAndUnlock(t *testing.T) {
	w := &Window{}
	if w.mouseIsLocked() {
		t.Error("should not be locked initially")
	}
	w.MouseLock(MouseLockCfg{
		MouseMove: func(ctx EventCtx) {},
	})
	if !w.mouseIsLocked() {
		t.Error("should be locked after MouseLock")
	}
	w.MouseUnlock()
	if w.mouseIsLocked() {
		t.Error("should not be locked after MouseUnlock")
	}
}

func TestPointerOverAppInside(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	e := &Event{MouseX: 400, MouseY: 300}
	if !w.pointerOverApp(e) {
		t.Error("center point should be inside")
	}
}

func TestPointerOverAppOutside(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	e := &Event{MouseX: -1, MouseY: 300}
	if w.pointerOverApp(e) {
		t.Error("negative X should be outside")
	}
	e2 := &Event{MouseX: 900, MouseY: 300}
	if w.pointerOverApp(e2) {
		t.Error("beyond width should be outside")
	}
}

func TestCloseAndCloseRequested(t *testing.T) {
	w := &Window{}
	if w.CloseRequested() {
		t.Error("should not be close-requested initially")
	}
	w.Close()
	if !w.CloseRequested() {
		t.Error("should be close-requested after Close()")
	}
}

func TestResetBlinkCursorVisible(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	w.viewState.inputCursorOn.Store(false)
	resetBlinkCursorVisible(w)
	if !w.viewState.inputCursorOn.Load() {
		t.Error("inputCursorOn should be true after reset")
	}
}

func TestWindowCtxNonNil(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	if w.Ctx() == nil {
		t.Error("Ctx() should not be nil for NewWindow")
	}
}

func TestWindowCtxNilFallback(t *testing.T) {
	w := &Window{}
	ctx := w.Ctx()
	if ctx == nil {
		t.Error("Ctx() should return background context for nil ctx")
	}
}

func TestUpdateViewPreservesIDFocus(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	w.viewState.focusID = "f42"
	w.UpdateView(func(_ *Window) View {
		return Text(TextCfg{Text: "hi"})
	})
	if w.FocusID() != "f42" {
		t.Errorf("FocusID = %q, want f42 after UpdateView",
			w.FocusID())
	}
}

func TestClearViewStateResetsIDFocus(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	w.viewState.focusID = "f42"
	w.clearViewState()
	if w.FocusID() != "" {
		t.Errorf("FocusID = %q, want empty after ClearViewState",
			w.FocusID())
	}
}

func TestIsFocusMatchesAndZero(t *testing.T) {
	w := &Window{}
	w.viewState.focusID = "f10"
	if !w.IsFocus("f10") {
		t.Error("IsFocus(f10) should be true")
	}
	if w.IsFocus("f99") {
		t.Error("IsFocus(f99) should be false")
	}
	w.viewState.focusID = ""
	if w.IsFocus("") {
		t.Error("IsFocus(empty) should always be false")
	}
}

func TestAllocShapeNilWindow(t *testing.T) {
	var w *Window
	src := Shape{Width: 42}
	got := w.allocShape(src)
	if got == nil || got.Width != 42 {
		t.Error("allocShape nil window should return heap copy")
	}
}

func TestAllocShapeUsesPool(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	src := Shape{Width: 99}
	got := w.allocShape(src)
	if got == nil || got.Width != 99 {
		t.Error("allocShape should return pooled copy")
	}
}

func TestGetClipboardNilSafe(t *testing.T) {
	w := &Window{}
	got := w.GetClipboard()
	if got != "" {
		t.Errorf("GetClipboard = %q, want empty for nil fn", got)
	}
}

func TestSetPrimary(t *testing.T) {
	w := &Window{}
	var got string
	w.SetPrimaryFn(func(s string) { got = s })
	w.SetPrimary("hello")
	if got != "hello" {
		t.Errorf("primary = %q, want hello", got)
	}
}

func TestSetPrimaryNilSafe(t *testing.T) {
	w := &Window{}
	// Should not panic when no fn set.
	w.SetPrimary("ignored")
}

func TestGetPrimaryNilSafe(t *testing.T) {
	w := &Window{}
	got := w.GetPrimary()
	if got != "" {
		t.Errorf("GetPrimary = %q, want empty for nil fn", got)
	}
}

func TestUpdateProducesRenderers(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	w.viewGenerator = func(_ *Window) View {
		return Text(TextCfg{Text: "render me"})
	}
	w.refreshLayout = true
	w.FrameFn()

	if w.refreshLayout {
		t.Error("refreshLayout should be cleared")
	}
	if len(w.renderers) == 0 {
		t.Error("expected renderers after Update")
	}
}

func TestUpdateRenderOnlyClearsFlag(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	w.viewGenerator = func(_ *Window) View {
		return Text(TextCfg{Text: "x"})
	}
	// Build initial layout.
	w.refreshLayout = true
	w.FrameFn()

	w.refreshRenderOnly = true
	got := w.FrameFn()
	if w.refreshRenderOnly {
		t.Error("refreshRenderOnly should be cleared")
	}
	if !got {
		t.Error("FrameFn should return true on render-only")
	}
}

func TestStatePanicsOnWrongType(t *testing.T) {
	w := NewWindow(WindowCfg{
		State: new(string),
		Width: 50, Height: 50,
	})
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for wrong type")
		}
	}()
	State[int](w)
}

func TestWindowSizeAndRect(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	width, height := w.WindowSize()
	if width != 800 || height != 600 {
		t.Errorf("size = %dx%d, want 800x600", width, height)
	}
	rect := w.windowRect()
	if rect.Width != 800 || rect.Height != 600 {
		t.Errorf("rect = %fx%f, want 800x600", rect.Width, rect.Height)
	}
	if rect.X != 0 || rect.Y != 0 {
		t.Errorf("rect origin = %f,%f, want 0,0", rect.X, rect.Y)
	}
}
