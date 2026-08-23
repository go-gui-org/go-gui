package gui

import (
	"testing"
	"time"
)

func TestMarkLayoutRefreshClearsRenderOnly(t *testing.T) {
	w := &Window{refreshRenderOnly: true}
	w.markLayoutRefresh()
	if !w.refreshLayout {
		t.Error("refreshLayout should be true")
	}
	if w.refreshRenderOnly {
		t.Error("refreshRenderOnly should be false")
	}
}

func TestMarkRenderOnlyRefreshSetsWhenLayoutNotPending(t *testing.T) {
	w := &Window{}
	w.markRenderOnlyRefresh()
	if w.refreshLayout {
		t.Error("refreshLayout should be false")
	}
	if !w.refreshRenderOnly {
		t.Error("refreshRenderOnly should be true")
	}
}

func TestMarkRenderOnlyRefreshSkipsWhenLayoutPending(t *testing.T) {
	w := &Window{refreshLayout: true}
	w.markRenderOnlyRefresh()
	if !w.refreshLayout {
		t.Error("refreshLayout should remain true")
	}
	if w.refreshRenderOnly {
		t.Error("refreshRenderOnly should remain false")
	}
}

func TestMaxAnimationRefreshKindPrefersLayout(t *testing.T) {
	kind := maxAnimationRefreshKind(animationRefreshRenderOnly, AnimationRefreshLayout)
	if kind != AnimationRefreshLayout {
		t.Errorf("got %d, want layout", kind)
	}
}

func TestMaxAnimationRefreshKindPrefersRenderOnlyOverNone(t *testing.T) {
	kind := maxAnimationRefreshKind(animationRefreshNone, animationRefreshRenderOnly)
	if kind != animationRefreshRenderOnly {
		t.Errorf("got %d, want render_only", kind)
	}
}

func TestBlinkCursorAnimationRefreshKindIsNone(t *testing.T) {
	a := newBlinkCursorAnimation()
	if a.RefreshKind() != animationRefreshNone {
		t.Errorf("got %d, want none", a.RefreshKind())
	}
}

func TestCommandToggleCaretBlinkPatchesInPlace(t *testing.T) {
	w := makeWindow()
	w.viewState.inputCursorOn.Store(true)
	w.caretCmd = caretCmdState{idx: 2, color: RGB(0, 255, 0), ok: true}
	w.renderers = []RenderCmd{
		{Kind: RenderRect},
		{Kind: RenderText},
		{Kind: RenderRect, Color: RGB(1, 2, 3)},
	}

	commandToggleCaretBlink(w)

	if got := w.renderers[2].Color; got != RGB(0, 255, 0) {
		t.Errorf("caret color = %v, want %v", got, RGB(0, 255, 0))
	}
	if w.renderers[0].Color != (Color{}) {
		t.Error("non-caret renderers must be untouched")
	}
	if !w.renderersDirty {
		t.Error("toggle should mark renderersDirty")
	}

	// Blink off: the same slot goes transparent.
	w.viewState.inputCursorOn.Store(false)
	commandToggleCaretBlink(w)
	if w.renderers[2].Color != ColorTransparent {
		t.Error("caret should go transparent when blink off")
	}
}

func TestCommandToggleCaretBlinkNoCaretIsNoOp(t *testing.T) {
	w := makeWindow()
	w.renderers = []RenderCmd{{Kind: RenderRect}}
	w.viewState.inputCursorOn.Store(true)
	commandToggleCaretBlink(w)
	if w.renderersDirty {
		t.Error("no recorded caret — toggle must not mark renderersDirty")
	}
}

func TestCommandToggleCaretBlinkStaleSlotIsNoOp(t *testing.T) {
	w := makeWindow()
	w.viewState.inputCursorOn.Store(true)
	w.renderers = []RenderCmd{{Kind: RenderText}}

	// Recorded slot no longer a rect: reset and do nothing.
	w.caretCmd = caretCmdState{idx: 0, color: RGB(0, 255, 0), ok: true}
	commandToggleCaretBlink(w)
	if w.caretCmd.ok {
		t.Error("stale caret slot should be reset")
	}
	if w.renderersDirty {
		t.Error("stale slot must not mark renderersDirty")
	}

	// Out-of-range index: same stale treatment.
	w.caretCmd = caretCmdState{idx: 5, color: RGB(0, 255, 0), ok: true}
	commandToggleCaretBlink(w)
	if w.caretCmd.ok {
		t.Error("out-of-range caret slot should be reset")
	}
	if w.renderersDirty {
		t.Error("out-of-range slot must not mark renderersDirty")
	}
}

func TestFrameFnPresentsCaretPatchWithoutRebuild(t *testing.T) {
	w := newTestWindow()
	w.renderersDirty = true
	if !w.FrameFn() {
		t.Error("FrameFn should report renderers changed")
	}
	if w.renderersDirty {
		t.Error("FrameFn should clear renderersDirty")
	}
	if w.refreshLayout || w.refreshRenderOnly {
		t.Error("blink patch must not request a rebuild")
	}
}

// focusedInputWindow returns a window whose only view is an Input
// focused at "f900" with a recorded caret after its first frame.
func focusedInputWindow(t *testing.T) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 120})
	w.SetFocus("f900")
	w.viewGenerator = func(_ *Window) View {
		return Row(ContainerCfg{
			Padding: PadAll(8),
			Content: []View{Input(InputCfg{ID: "f900"})},
		})
	}
	w.refreshLayout = true
	w.FrameFn()
	if !w.caretCmd.ok {
		t.Fatal("focused input should record a caret command")
	}
	return w
}

// TestBlinkTickPresentsWithoutRebuild drives a focused Input through a
// real blink tick (toggle + queued patch command + FrameFn) and proves
// the frame presents the patched list without rebuilding renderers.
//
// Detection is deliberately clock-independent: the toggle must not
// request any refresh, and a frame that presents when nothing was
// requested can only come from the renderersDirty patch path.
// FrameTimings are not compared — on coarse-clock platforms (Windows
// CI's default ~15.6ms timer) a tiny test frame rounds every duration
// to zero, so a timings comparison would be vacuous there (issue #404).
func TestBlinkTickPresentsWithoutRebuild(t *testing.T) {
	w := focusedInputWindow(t)
	if len(w.renderers) == 0 {
		t.Fatal("initial frame should have built renderers")
	}
	if got := w.renderers[w.caretCmd.idx].Color; got != w.caretCmd.color {
		t.Fatal("SetFocus starts the caret visible")
	}

	// The tree registered the blink animation during the initial frame
	// (syncBlinkCursor). Backdate it and toggle exactly like the
	// animation loop does.
	b := w.animations[blinkCursorAnimationID].(*BlinkCursorAnimation)
	b.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	if !updateBlinkCursor(b, w, &ac) {
		t.Fatal("backdated blink should toggle")
	}
	w.queueCommandsBatch(deferred)
	w.flushCommands()

	if w.refreshLayout || w.refreshRenderOnly {
		t.Fatal("blink toggle must not request any refresh")
	}
	if !w.FrameFn() {
		t.Fatal("blink frame must present the patched list")
	}
	if w.renderers[w.caretCmd.idx].Color != ColorTransparent {
		t.Error("caret color should go transparent via the in-place patch")
	}
	if w.renderersDirty {
		t.Error("FrameFn should clear renderersDirty")
	}
}

// TestRebuildReRecordsCaretCmd: a rebuild resets caretCmd and the new
// pass re-records it at the same slot, so the blink patch never aims
// at a stale renderer after any refresh.
func TestRebuildReRecordsCaretCmd(t *testing.T) {
	w := focusedInputWindow(t)
	before := w.caretCmd.idx

	w.markRenderOnlyRefresh()
	w.FrameFn()

	if !w.caretCmd.ok {
		t.Fatal("rebuild should re-record the caret")
	}
	if w.caretCmd.idx != before {
		t.Error("caret slot should be stable across a render-only rebuild")
	}
	if got := w.renderers[w.caretCmd.idx].Color; got != w.caretCmd.color {
		t.Errorf("re-recorded caret color = %v, want %v", got, w.caretCmd.color)
	}
}

func TestAnimateRefreshKindIsLayout(t *testing.T) {
	a := &Animate{
		AnimID:   "test",
		Callback: func(*Animate, *Window) {},
	}
	if a.RefreshKind() != AnimationRefreshLayout {
		t.Errorf("got %d, want layout", a.RefreshKind())
	}
}

func TestAnimateRefreshKindOverride(t *testing.T) {
	a := &Animate{
		AnimID:   "test",
		Callback: func(*Animate, *Window) {},
		Refresh:  animationRefreshRenderOnly,
	}
	if a.RefreshKind() != animationRefreshRenderOnly {
		t.Errorf("got %d, want render_only", a.RefreshKind())
	}
}

func TestRequestRedrawSetsRenderOnly(t *testing.T) {
	w := &Window{}
	w.RequestRedraw()
	if w.refreshLayout {
		t.Error("refreshLayout should be false")
	}
	if !w.refreshRenderOnly {
		t.Error("refreshRenderOnly should be true")
	}
}
