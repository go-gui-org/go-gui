package gui

import "testing"

// Issue #1099: a mouse lock belongs to the button whose press took it.
// Another button's press and release during the drag must not end the
// lock, and must not reach the widgets under the pointer.

// lockButtonProbe counts what a test lock and the tree under it see.
type lockButtonProbe struct {
	lockUps    int
	lockDowns  int
	treeDowns  int
	treeUps    int
	lockOnDown bool
	// lockDown, when set, is the lock's MouseDown callback.
	lockDown func(EventCtx)
}

// lockButtonWindow renders one fill container. A press on it takes a
// lock when lockOnDown is set; otherwise it only counts.
func lockButtonWindow(t *testing.T, p *lockButtonProbe) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			ID:     "pad",
			Sizing: FillFill,
			OnMouseDown: func(ctx EventCtx) {
				p.treeDowns++
				if p.lockOnDown && !ctx.Window.mouseIsLocked() {
					ctx.Window.MouseLock(MouseLockCfg{
						MouseDown: p.lockDown,
						MouseMove: func(EventCtx) {},
						MouseUp: func(lctx EventCtx) {
							p.lockUps++
							lctx.Window.MouseUnlock()
						},
					})
				}
				ctx.Consume()
			},
			OnMouseUp: func(ctx EventCtx) {
				p.treeUps++
				ctx.Consume()
			},
		})
	})
	return w
}

func lockButtonSend(w *Window, typ EventType, btn MouseButton) {
	w.EventFn(&Event{Type: typ, MouseButton: btn, MouseX: 20, MouseY: 20})
}

func TestMouseLockIgnoresOtherButton(t *testing.T) {
	for _, tc := range []struct {
		name         string
		owner, other MouseButton
	}{
		{"left drag, right click", MouseLeft, MouseRight},
		{"left drag, middle click", MouseLeft, MouseMiddle},
		{"right drag, left click", MouseRight, MouseLeft},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &lockButtonProbe{lockOnDown: true}
			w := lockButtonWindow(t, p)
			lockButtonSend(w, EventMouseDown, tc.owner)
			if !w.mouseIsLocked() {
				t.Fatal("owner press did not take the lock")
			}
			lockButtonSend(w, EventMouseDown, tc.other)
			lockButtonSend(w, EventMouseUp, tc.other)
			if !w.mouseIsLocked() || p.lockUps != 0 {
				t.Fatalf("other button ended the lock: locked=%v ups=%d",
					w.mouseIsLocked(), p.lockUps)
			}
			// The other press and release belong to no one: the tree
			// under the drag must not see them either.
			if p.treeDowns != 1 || p.treeUps != 0 {
				t.Fatalf("tree saw the other button: downs=%d ups=%d",
					p.treeDowns, p.treeUps)
			}
			lockButtonSend(w, EventMouseUp, tc.owner)
			if w.mouseIsLocked() || p.lockUps != 1 {
				t.Fatalf("owner release did not end the lock: locked=%v ups=%d",
					w.mouseIsLocked(), p.lockUps)
			}
		})
	}
}

// The release of a dropped press reaches no widget, even when it
// comes after the drag ended: no widget saw the press.
func TestMouseLockDroppedReleaseAfterUnlock(t *testing.T) {
	p := &lockButtonProbe{lockOnDown: true}
	w := lockButtonWindow(t, p)
	lockButtonSend(w, EventMouseDown, MouseLeft)
	lockButtonSend(w, EventMouseDown, MouseRight)
	lockButtonSend(w, EventMouseUp, MouseLeft)
	if w.mouseIsLocked() {
		t.Fatal("left release did not end the lock")
	}
	lockButtonSend(w, EventMouseUp, MouseRight)
	if p.treeUps != 0 {
		t.Fatalf("dropped press's release reached the tree: ups=%d", p.treeUps)
	}
	// The record is spent: the next right click is a normal one.
	p.lockOnDown = false
	lockButtonSend(w, EventMouseDown, MouseRight)
	lockButtonSend(w, EventMouseUp, MouseRight)
	if p.treeDowns != 2 || p.treeUps != 1 {
		t.Fatalf("next right click: downs=%d ups=%d, want 2 and 1",
			p.treeDowns, p.treeUps)
	}
}

// A dropped press whose release never came does not eat the release of
// the next press of that button that reaches the tree.
func TestMouseLockLostDroppedRelease(t *testing.T) {
	p := &lockButtonProbe{lockOnDown: true}
	w := lockButtonWindow(t, p)
	lockButtonSend(w, EventMouseDown, MouseLeft)
	lockButtonSend(w, EventMouseDown, MouseRight)
	lockButtonSend(w, EventMouseUp, MouseLeft)
	// Right's release is lost; the next right press reaches the tree.
	p.lockOnDown = false
	lockButtonSend(w, EventMouseDown, MouseRight)
	lockButtonSend(w, EventMouseUp, MouseRight)
	if p.treeUps != 1 {
		t.Fatalf("release after a lost one: ups=%d, want 1", p.treeUps)
	}
}

// Capture loss clears the held buttons and the record of dropped
// presses with them: a later release with no press of its own is not
// taken for the release of the dropped one.
func TestMouseLockCaptureLossClearsDroppedPresses(t *testing.T) {
	p := &lockButtonProbe{lockOnDown: true}
	w := lockButtonWindow(t, p)
	lockButtonSend(w, EventMouseDown, MouseLeft)
	lockButtonSend(w, EventMouseDown, MouseRight)
	w.clearHeldMouseButtons()
	w.MouseCancel()
	lockButtonSend(w, EventMouseUp, MouseRight)
	if p.treeUps != 1 {
		t.Fatalf("release after capture loss: ups=%d, want 1", p.treeUps)
	}
}

// A lock that asks for presses through MouseDown still gets every
// press, the other button's included.
func TestMouseLockDownCallbackGetsOtherButton(t *testing.T) {
	p := &lockButtonProbe{lockOnDown: true}
	p.lockDown = func(EventCtx) { p.lockDowns++ }
	w := lockButtonWindow(t, p)
	lockButtonSend(w, EventMouseDown, MouseLeft)
	lockButtonSend(w, EventMouseDown, MouseRight)
	if p.lockDowns != 1 {
		t.Fatalf("lock MouseDown ran %d times, want 1", p.lockDowns)
	}
	lockButtonSend(w, EventMouseUp, MouseRight)
	if !w.mouseIsLocked() {
		t.Fatal("right release ended a left lock")
	}
	lockButtonSend(w, EventMouseUp, MouseLeft)
	if w.mouseIsLocked() {
		t.Fatal("left release did not end the lock")
	}
}

// A lock taken with no button held has no owner, so any release ends
// it, as before #1099.
func TestMouseLockWithoutPressAnyRelease(t *testing.T) {
	p := &lockButtonProbe{}
	w := lockButtonWindow(t, p)
	w.MouseLock(MouseLockCfg{MouseUp: func(ctx EventCtx) {
		p.lockUps++
		ctx.Window.MouseUnlock()
	}})
	lockButtonSend(w, EventMouseUp, MouseRight)
	if w.mouseIsLocked() || p.lockUps != 1 {
		t.Fatalf("release did not end an unowned lock: locked=%v ups=%d",
			w.mouseIsLocked(), p.lockUps)
	}
}

// A lock whose button was released without ending it (a MouseUp that
// forgot to unlock) is stale. The next release of any button reaches
// it, so the lock can still end.
func TestMouseLockStaleOwnerAnyRelease(t *testing.T) {
	p := &lockButtonProbe{}
	w := lockButtonWindow(t, p)
	lockButtonSend(w, EventMouseDown, MouseLeft)
	w.MouseLock(MouseLockCfg{MouseUp: func(ctx EventCtx) {
		p.lockUps++
		if p.lockUps == 2 {
			ctx.Window.MouseUnlock()
		}
	}})
	lockButtonSend(w, EventMouseUp, MouseLeft)
	if !w.mouseIsLocked() || p.lockUps != 1 {
		t.Fatalf("setup: locked=%v ups=%d", w.mouseIsLocked(), p.lockUps)
	}
	lockButtonSend(w, EventMouseDown, MouseRight)
	if p.treeDowns != 2 {
		t.Fatalf("press on a stale lock was dropped: tree downs=%d", p.treeDowns)
	}
	lockButtonSend(w, EventMouseUp, MouseRight)
	if w.mouseIsLocked() || p.lockUps != 2 {
		t.Fatalf("release did not reach a stale lock: locked=%v ups=%d",
			w.mouseIsLocked(), p.lockUps)
	}
}

// A lock taken after its press, past a drag threshold, with a second
// button already down: the lowest held button owns it, so left wins,
// as in heldMouseButton. The latest press (right) must not own a left
// drag.
func TestMouseLockDelayedTakesLowestHeld(t *testing.T) {
	p := &lockButtonProbe{}
	w := lockButtonWindow(t, p)
	lockButtonSend(w, EventMouseDown, MouseLeft)
	lockButtonSend(w, EventMouseDown, MouseRight)
	w.MouseLock(MouseLockCfg{MouseUp: func(ctx EventCtx) {
		p.lockUps++
		ctx.Window.MouseUnlock()
	}})
	if got := w.mouseLockOwnerButton(); got != MouseLeft {
		t.Fatalf("owner = %d, want left", got)
	}
	lockButtonSend(w, EventMouseUp, MouseRight)
	if !w.mouseIsLocked() {
		t.Fatal("right release ended a left-owned lock")
	}
	lockButtonSend(w, EventMouseUp, MouseLeft)
	if w.mouseIsLocked() || p.lockUps != 1 {
		t.Fatalf("left release did not end the lock: locked=%v ups=%d",
			w.mouseIsLocked(), p.lockUps)
	}
}

// A lock taken while a press is dispatched belongs to that press, even
// with a lower button already down: a right drag started with left
// held ends on the right release.
func TestMouseLockDispatchedPressOwnsIt(t *testing.T) {
	p := &lockButtonProbe{}
	w := lockButtonWindow(t, p)
	lockButtonSend(w, EventMouseDown, MouseLeft)
	p.lockOnDown = true
	lockButtonSend(w, EventMouseDown, MouseRight)
	if got := w.mouseLockOwnerButton(); got != MouseRight {
		t.Fatalf("owner = %d, want right", got)
	}
	lockButtonSend(w, EventMouseUp, MouseLeft)
	if !w.mouseIsLocked() {
		t.Fatal("left release ended a right-owned lock")
	}
	lockButtonSend(w, EventMouseUp, MouseRight)
	if w.mouseIsLocked() {
		t.Fatal("right release did not end the lock")
	}
}

// The touch path presses through synthMouse, not handleMouseDownEvent.
// A synthesized press of another button during a drag is dropped
// there too: no tree press, and the lock stays.
func TestMouseLockDropsSynthesizedPress(t *testing.T) {
	p := &lockButtonProbe{lockOnDown: true}
	w := lockButtonWindow(t, p)
	lockButtonSend(w, EventMouseDown, MouseRight)
	if got := w.mouseLockOwnerButton(); got != MouseRight {
		t.Fatalf("owner = %d, want right", got)
	}
	synthMouse(EventMouseDown, 20, 20, MouseLeft, &w.layout, w)
	if p.treeDowns != 1 {
		t.Fatalf("synthesized press reached the tree: downs=%d", p.treeDowns)
	}
	synthMouse(EventMouseUp, 20, 20, MouseLeft, &w.layout, w)
	if !w.mouseIsLocked() || p.lockUps != 0 {
		t.Fatalf("synthesized release ended the lock: locked=%v ups=%d",
			w.mouseIsLocked(), p.lockUps)
	}
	lockButtonSend(w, EventMouseUp, MouseRight)
	if w.mouseIsLocked() {
		t.Fatal("right release did not end the lock")
	}
}

// MouseUnlock and MouseCancel clear the owner, so the next lock taken
// with no button held is unowned again.
func TestMouseLockOwnerClearedOnUnlock(t *testing.T) {
	w := &Window{}
	w.pressMouseButton(MouseLeft)
	w.MouseLock(MouseLockCfg{MouseUp: func(EventCtx) {}})
	if w.mouseLockOwnerButton() != MouseLeft {
		t.Fatalf("owner = %d, want left", w.mouseLockOwnerButton())
	}
	w.MouseUnlock()
	if w.mouseLockOwnerButton() != MouseInvalid {
		t.Fatalf("owner after unlock = %d, want none", w.mouseLockOwnerButton())
	}
	w.MouseLock(MouseLockCfg{MouseUp: func(EventCtx) {}})
	w.MouseCancel()
	if w.mouseLockOwnerButton() != MouseInvalid {
		t.Fatalf("owner after cancel = %d, want none", w.mouseLockOwnerButton())
	}
}

// lockUserBegin starts a left drag on one widget that takes a mouse
// lock, and returns the window and the pointer the drag is at.
type lockUserBegin func(t *testing.T) (*Window, float32, float32)

// lockUserContract begins a drag through the interaction contract's
// stDrag setup: press on the target, then move past any threshold.
func lockUserContract(
	build func(*contractCtl, string) View, suffix string,
) lockUserBegin {
	return func(t *testing.T) (*Window, float32, float32) {
		row := contractRow{state: stDrag, build: build, beginIDSuffix: suffix}
		w := contractWindow(t, &contractCtl{}, build)
		px, py := contractBegin(t, w, row, contractTarget(t, w, row))
		return w, px + 10, py + 10
	}
}

// lockUserPress begins a drag with a left press at a point the setup
// picks; the press alone takes the lock.
func lockUserPress(
	setup func(t *testing.T) (*Window, float32, float32),
) lockUserBegin {
	return func(t *testing.T) (*Window, float32, float32) {
		w, x, y := setup(t)
		pressAt(w, MouseLeft, x, y)
		return w, x, y
	}
}

// Every lock user in gui, through real dispatch: a right click during
// a left drag keeps the drag, and the left release ends it (#1099).
// Drag to scroll has its own case (TestDragScrollSecondButtonKeepsPan)
// and the data grid header resize is in gui/datagrid.
func TestMouseLockUsersIgnoreOtherButton(t *testing.T) {
	rows := []struct {
		name  string
		begin lockUserBegin
	}{
		{"slider", lockUserContract(contractSlider, "")},
		{"drag reorder", lockUserContract(contractListbox, ":item:a")},
		{"splitter", lockUserContract(contractSplitter, "")},
		{"dock drag", lockUserContract(contractDock, ":tab:g1:p2")},
		{"scrollbar", lockUserPress(func(t *testing.T) (*Window, float32, float32) {
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(stockList(20))
			w.TestRender(nil)
			thumb := stockBarY(t, w).Children[thumbIndex].Shape
			return w, thumb.X + thumb.Width/2, thumb.Y + thumb.Height/2
		})},
		{"color drag", lockUserPress(func(t *testing.T) (*Window, float32, float32) {
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(func(*Window) View {
				return ColorPlane(ColorPlaneCfg{
					ID: "plane", Value: HSLA{H: 200, S: 0.5, L: 0.5, A: 1},
					OnChange: func(HSLA, EventCtx) {},
				})
			})
			x, y := hoverOver(t, w, "plane")
			return w, x, y
		})},
		{"input", lockUserPress(func(t *testing.T) (*Window, float32, float32) {
			w, ly := newInputSelWindow(t, InputCfg{ID: "in", Text: "hello world"})
			x, y := inputRunePoint(inputTextShape(t, ly), 2)
			return w, x, y
		})},
		{"text select", lockUserPress(func(t *testing.T) (*Window, float32, float32) {
			w, ly := newTextSelWindow(t, TextCfg{
				Text: "hello world", Focusable: true, ID: "txt",
			})
			x, y := textRunePoint(ly.Shape, 2)
			return w, x, y
		})},
		{"rtf select", lockUserPress(func(t *testing.T) (*Window, float32, float32) {
			h := newRtfSelectHarness(t, rtfTwoRuns())
			x, y := rtfRunePoint(h.shape(t), 2)
			return h.w, x, y
		})},
		{"markdown select", lockUserPress(func(t *testing.T) (*Window, float32, float32) {
			h := newMdSelectHarness(t)
			x, y := runePoint(h.blockByText(t, "First paragraph text"), 6)
			return h.w, x, y
		})},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			w, x, y := row.begin(t)
			if !w.mouseIsLocked() {
				t.Fatal("left press did not take the lock")
			}
			pressAt(w, MouseRight, x, y)
			releaseAt(w, MouseRight, x, y)
			if !w.mouseIsLocked() {
				t.Fatal("right click ended the left drag")
			}
			releaseAt(w, MouseLeft, x, y)
			if w.mouseIsLocked() {
				t.Fatal("left release did not end the drag")
			}
		})
	}
}
