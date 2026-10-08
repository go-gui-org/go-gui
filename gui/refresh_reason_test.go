package gui

import (
	"strings"
	"testing"
)

// clearLayoutRefresh drops the layout bits and keeps the render bits.
// Test-only.
func (w *Window) clearLayoutRefresh() { w.refresh.And(uint32(^refreshLayoutMask)) }

// clearRenderRefresh drops the render bits and keeps the layout bits.
// Test-only.
func (w *Window) clearRenderRefresh() { w.refresh.And(uint32(^refreshRenderMask)) }

func TestRefreshReasonStringJoinsBitsInOrder(t *testing.T) {
	cases := []struct {
		r    refreshReason
		want string
	}{
		{0, "none"},
		{refreshInput, "input"},
		{refreshInvalidate | refreshInput, "input|invalidate"},
		{refreshRenderSvg | refreshRender, "render|svg"},
		{refreshTest | refreshRender, "test|render"},
		{refreshAnimation | refreshRenderAnimation, "animation|render-animation"},
	}
	for _, c := range cases {
		if got := c.r.String(); got != c.want {
			t.Errorf("%#x: got %q, want %q", uint32(c.r), got, c.want)
		}
	}
}

// Every reason bit has its own name, so a new bit cannot log as nothing
// and a full pass that also took a render bit cannot log a cause that
// reads as a layout cause.
func TestRefreshReasonEveryBitNamed(t *testing.T) {
	var named refreshReason
	names := map[string]bool{}
	for _, n := range refreshReasonNames {
		if named&n.bit != 0 {
			t.Fatalf("bit %#x named twice", uint32(n.bit))
		}
		if names[n.name] {
			t.Fatalf("name %q used for two bits", n.name)
		}
		names[n.name] = true
		named |= n.bit
	}
	if want := refreshTest<<1 - 1; named&refreshLayoutMask != want {
		t.Errorf("layout bits named %#x, want %#x", uint32(named&refreshLayoutMask), uint32(want))
	}
	if want := (refreshRenderSvg<<1 - 1) &^ refreshLayoutMask; named&refreshRenderMask != want {
		t.Errorf("render bits named %#x, want %#x", uint32(named&refreshRenderMask), uint32(want))
	}
}

func TestMarkRefreshMasksTheOtherKind(t *testing.T) {
	w := &Window{}
	// A layout reason passed to the render mark, or the reverse, must not
	// change which pass runs.
	w.markRenderOnlyRefresh(refreshInput)
	w.markLayoutRefresh(refreshRender)
	if w.refreshPending() {
		t.Fatalf("wrong-kind reasons set %#x", w.refresh.Load())
	}
}

func TestLayoutRefreshWinsOverRender(t *testing.T) {
	w := &Window{}
	w.markRenderOnlyRefresh(refreshRenderSvg)
	w.markLayoutRefresh(refreshInput)
	if !w.layoutPending() || w.renderPending() {
		t.Fatal("layout request must hide the render-only request")
	}
	// The full pass takes both, so the log shows every cause it covered.
	if got := w.takeLayoutRefresh(); got != refreshInput|refreshRenderSvg {
		t.Errorf("took %v", got)
	}
	if w.refreshPending() {
		t.Error("full pass must clear every bit")
	}
}

func TestTakeRenderRefreshKeepsLayoutBits(t *testing.T) {
	w := &Window{}
	w.markRenderOnlyRefresh(refreshRender)
	w.markLayoutRefresh(refreshInvalidate)
	if got := w.takeRenderRefresh(); got != refreshRender {
		t.Errorf("took %v, want render", got)
	}
	if !w.layoutPending() {
		t.Error("render pass dropped a layout request")
	}
}

// The reasons a frame records name what asked for it.
func TestFrameRecordsRebuildReasons(t *testing.T) {
	w := &Window{focused: true}
	w.SetView(func(*Window) View { return Column(ContainerCfg{}) })
	w.FrameFn()
	if !w.lastRefreshFull || w.lastRefresh&refreshView == 0 {
		t.Errorf("SetView frame: full=%v reasons=%v", w.lastRefreshFull, w.lastRefresh)
	}

	w.EventFn(&Event{Type: EventMouseMove, MouseX: 1, MouseY: 1})
	w.FrameFn()
	if !w.lastRefreshFull || w.lastRefresh != refreshInput {
		t.Errorf("input frame: full=%v reasons=%v", w.lastRefreshFull, w.lastRefresh)
	}

	w.InvalidateRender()
	w.FrameFn()
	if w.lastRefreshFull || w.lastRefresh != refreshRender {
		t.Errorf("render frame: full=%v reasons=%v", w.lastRefreshFull, w.lastRefresh)
	}
}

// A queued command that runs in the settle loop names itself as the cause
// of the pass that follows it.
func TestSettleRecordsCommandReason(t *testing.T) {
	w := &Window{}
	w.SetView(func(*Window) View { return Column(ContainerCfg{}) })
	w.settle()
	w.QueueCommand(func(*Window) {})
	w.settle()
	if !w.lastRefreshFull || w.lastRefresh&refreshCommand == 0 {
		t.Errorf("command pass: full=%v reasons=%v", w.lastRefreshFull, w.lastRefresh)
	}
}

// A steady stream of one cause prints once; a change prints again.
func TestDebugRebuildsLogsChangesOnly(t *testing.T) {
	buf := captureDebugMask(t, DebugRebuilds)
	w := &Window{}
	w.noteRefresh(refreshInput, true)
	w.noteRefresh(refreshInput, true)
	w.noteRefresh(refreshInput, true)
	w.noteRefresh(refreshRenderSvg, false)
	w.noteRefresh(refreshRenderSvg, false)
	w.noteRefresh(refreshInput, true)
	want := "gui: rebuild layout: input\n" +
		"gui: rebuild render: svg\n" +
		"gui: rebuild layout: input\n"
	if got := buf.String(); got != want {
		t.Errorf("log:\n%s\nwant:\n%s", got, want)
	}
}

// Turning the gate off and on again reports the pass in front of it, even
// when its reasons match the last pass logged.
func TestDebugRebuildsReportsAfterRegate(t *testing.T) {
	buf := captureDebugMask(t, DebugRebuilds)
	w := &Window{}
	w.noteRefresh(refreshInput, true)
	DebugCategories(0)
	w.noteRefresh(refreshInput, true)
	DebugCategories(DebugRebuilds)
	w.noteRefresh(refreshInput, true)
	if got := strings.Count(buf.String(), "rebuild layout: input"); got != 2 {
		t.Errorf("got %d lines, want 2:\n%s", got, buf.String())
	}
}

func TestDebugRebuildsNotInDebugAll(t *testing.T) {
	if DebugAll&DebugRebuilds != 0 {
		t.Error("DebugRebuilds logs normal operation; keep it out of DebugAll")
	}
	buf := captureDebugMask(t, DebugAll)
	w := &Window{}
	w.noteRefresh(refreshInput, true)
	if buf.Len() != 0 {
		t.Errorf("DebugAll printed rebuild log: %q", buf.String())
	}
}

// With the category off, recording why a frame rebuilds costs no
// allocation: mark, take and note are all a frame does.
func TestRefreshReasonZeroAllocWhenOff(t *testing.T) {
	captureDebugMask(t, 0)
	w := &Window{}
	allocs := testing.AllocsPerRun(100, func() {
		w.markLayoutRefresh(refreshInput)
		w.markRenderOnlyRefresh(refreshRenderSvg)
		w.noteRefresh(w.takeLayoutRefresh(), true)
		w.markRenderOnlyRefresh(refreshRender)
		w.noteRefresh(w.takeRenderRefresh(), false)
	})
	if allocs != 0 {
		t.Errorf("allocs = %v, want 0", allocs)
	}
}
