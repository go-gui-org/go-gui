package gui

import (
	"testing"
	"time"
)

func TestA11yCollectEmpty(t *testing.T) {
	layout := Layout{}
	var nodes []A11yNode
	var live []liveNode
	idx := a11yCollect(&layout, -1, &nodes, 0, &live)
	if idx != -1 {
		t.Errorf("focusedIdx: got %d, want -1", idx)
	}
	if len(nodes) != 0 {
		t.Errorf("nodes: got %d, want 0", len(nodes))
	}
}

func TestA11yCollectNilShape(t *testing.T) {
	layout := Layout{Shape: nil}
	var nodes []A11yNode
	var live []liveNode
	idx := a11yCollect(&layout, -1, &nodes, 0, &live)
	if idx != -1 || len(nodes) != 0 {
		t.Error("nil shape should produce no nodes")
	}
}

func TestA11yCollectSkipNoneRole(t *testing.T) {
	layout := Layout{
		Shape: &Shape{A11YRole: AccessRoleNone},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 0 {
		t.Errorf("AccessRoleNone should not emit a node, got %d", len(nodes))
	}
}

func TestA11yCollectSingleButton(t *testing.T) {
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleButton,
			A11Y:     &AccessInfo{Label: "OK"},
			X:        10, Y: 20, Width: 100, Height: 30,
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.Role != AccessRoleButton {
		t.Errorf("Role: got %d", n.Role)
	}
	if n.Label != "OK" {
		t.Errorf("Label: got %q", n.Label)
	}
	if n.X != 10 || n.Y != 20 || n.W != 100 || n.H != 30 {
		t.Errorf("bounds: %g,%g %gx%g", n.X, n.Y, n.W, n.H)
	}
	if n.ParentIdx != -1 {
		t.Errorf("ParentIdx: got %d, want -1", n.ParentIdx)
	}
}

func TestA11yCollectLabelFromText(t *testing.T) {
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleButton,
			TC:       &ShapeTextConfig{Text: "Submit"},
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Label != "Submit" {
		t.Errorf("Label: got %q, want %q", nodes[0].Label, "Submit")
	}
}

func TestA11yCollectA11YLabelOverridesText(t *testing.T) {
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleButton,
			A11Y:     &AccessInfo{Label: "Close dialog"},
			TC:       &ShapeTextConfig{Text: "X"},
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if nodes[0].Label != "Close dialog" {
		t.Errorf("Label: got %q", nodes[0].Label)
	}
}

func TestA11yCollectWithChildren(t *testing.T) {
	layout := Layout{
		Shape: &Shape{A11YRole: AccessRoleGroup},
		Children: []Layout{
			{Shape: &Shape{A11YRole: AccessRoleButton, A11Y: &AccessInfo{Label: "A"}}},
			{Shape: &Shape{A11YRole: AccessRoleButton, A11Y: &AccessInfo{Label: "B"}}},
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	// Parent is first node (group).
	if nodes[0].ChildrenStart != 1 || nodes[0].ChildrenCount != 2 {
		t.Errorf("parent children: start=%d count=%d",
			nodes[0].ChildrenStart, nodes[0].ChildrenCount)
	}
	// Children have parent_idx = 0.
	if nodes[1].ParentIdx != 0 || nodes[2].ParentIdx != 0 {
		t.Errorf("child parent: %d, %d", nodes[1].ParentIdx, nodes[2].ParentIdx)
	}
}

func TestA11yCollectNoneRolePassesChildren(t *testing.T) {
	// Parent has no role but child does.
	layout := Layout{
		Shape: &Shape{A11YRole: AccessRoleNone},
		Children: []Layout{
			{Shape: &Shape{A11YRole: AccessRoleButton, A11Y: &AccessInfo{Label: "Child"}}},
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node from child, got %d", len(nodes))
	}
	if nodes[0].Label != "Child" {
		t.Errorf("Label: got %q", nodes[0].Label)
	}
	// Parent was -1 (root context), child inherits that.
	if nodes[0].ParentIdx != -1 {
		t.Errorf("ParentIdx: got %d, want -1", nodes[0].ParentIdx)
	}
}

func TestA11yCollectFocusTracking(t *testing.T) {
	layout := Layout{
		Shape: &Shape{A11YRole: AccessRoleGroup},
		Children: []Layout{
			{Shape: &Shape{A11YRole: AccessRoleButton, IDFocus: 1}},
			{Shape: &Shape{A11YRole: AccessRoleButton, IDFocus: 2}},
		},
	}
	var nodes []A11yNode
	var live []liveNode
	idx := a11yCollect(&layout, -1, &nodes, 2, &live)
	if idx != 2 { // third node (index 2) has IDFocus=2
		t.Errorf("focusedIdx: got %d, want 2", idx)
	}
}

func TestA11yCollectNoFocus(t *testing.T) {
	layout := Layout{
		Shape: &Shape{A11YRole: AccessRoleButton, IDFocus: 1},
	}
	var nodes []A11yNode
	var live []liveNode
	idx := a11yCollect(&layout, -1, &nodes, 99, &live)
	if idx != -1 {
		t.Errorf("focusedIdx: got %d, want -1", idx)
	}
}

func TestA11yCollectLiveRegion(t *testing.T) {
	layout := Layout{
		Shape: &Shape{
			A11YRole:  AccessRoleStaticText,
			A11YState: AccessStateLive,
			A11Y:      &AccessInfo{Label: "status", ValueNum: 42},
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(live) != 1 {
		t.Fatalf("expected 1 live node, got %d", len(live))
	}
	if live[0].label != "status" {
		t.Errorf("label: got %q", live[0].label)
	}
}

func TestA11yCollectValueFields(t *testing.T) {
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleSlider,
			A11Y: &AccessInfo{
				Label:    "Volume",
				ValueNum: 75,
				ValueMin: 0,
				ValueMax: 100,
			},
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.ValueNum != 75 {
		t.Errorf("ValueNum: got %g, want 75", n.ValueNum)
	}
	if n.ValueMin != 0 {
		t.Errorf("ValueMin: got %g, want 0", n.ValueMin)
	}
	if n.ValueMax != 100 {
		t.Errorf("ValueMax: got %g, want 100", n.ValueMax)
	}
}

func TestA11yCollectValueFieldsNilA11Y(t *testing.T) {
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleButton,
			TC:       &ShapeTextConfig{Text: "OK"},
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.ValueNum != 0 || n.ValueMin != 0 || n.ValueMax != 0 {
		t.Errorf("expected zero values, got %g/%g/%g",
			n.ValueNum, n.ValueMin, n.ValueMax)
	}
}

func TestA11yValueText(t *testing.T) {
	tests := []struct {
		info *AccessInfo
		want string
	}{
		{&AccessInfo{}, ""},
		{&AccessInfo{ValueNum: 42}, "42"},
		{&AccessInfo{ValueNum: 0.5, ValueMin: 0, ValueMax: 1}, "0.5"},
		{&AccessInfo{ValueNum: 0, ValueMin: 0, ValueMax: 0}, ""},
	}
	for _, tt := range tests {
		got := a11yValueText(tt.info)
		if got != tt.want {
			t.Errorf("a11yValueText(%+v) = %q, want %q", tt.info, got, tt.want)
		}
	}
}

func TestShapeA11yLabel(t *testing.T) {
	s := &Shape{TC: &ShapeTextConfig{Text: "Hello"}}
	if got := shapeA11yLabel(s); got != "Hello" {
		t.Errorf("got %q", got)
	}
	s2 := &Shape{}
	if got := shapeA11yLabel(s2); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestA11yDeepNesting(t *testing.T) {
	// 3-level nesting: root > group > button.
	layout := Layout{
		Shape: &Shape{A11YRole: AccessRoleGroup},
		Children: []Layout{
			{
				Shape: &Shape{A11YRole: AccessRoleGroup},
				Children: []Layout{
					{Shape: &Shape{A11YRole: AccessRoleButton, A11Y: &AccessInfo{Label: "Deep"}}},
				},
			},
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	// Root's child is at index 1.
	if nodes[2].ParentIdx != 1 {
		t.Errorf("deep child parent: got %d, want 1", nodes[2].ParentIdx)
	}
}

func TestA11yCollectDisabledState(t *testing.T) {
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleButton,
			Disabled: true,
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if !nodes[0].State.Has(AccessStateDisabled) {
		t.Error("expected AccessStateDisabled in state")
	}
}

func TestA11yCollectDisabledPreservesExplicitState(t *testing.T) {
	layout := Layout{
		Shape: &Shape{
			A11YRole:  AccessRoleCheckbox,
			A11YState: AccessStateChecked,
			Disabled:  true,
		},
	}
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&layout, -1, &nodes, 0, &live)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	s := nodes[0].State
	if !s.Has(AccessStateChecked) {
		t.Error("expected AccessStateChecked")
	}
	if !s.Has(AccessStateDisabled) {
		t.Error("expected AccessStateDisabled")
	}
}

// --- a11yActionCallback tests ---

func TestA11yActionCallbackPress(t *testing.T) {
	clicked := false
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleButton,
			events: &eventHandlers{
				OnClick: func(_ *Layout, _ *Event, _ *Window) {
					clicked = true
				},
			},
		},
	}
	w := newTestWindow()
	w.layout = layout
	// Build node array so index is valid.
	w.a11y.nodes = w.a11y.nodes[:0]
	var live []liveNode
	a11yCollect(&w.layout, -1, &w.a11y.nodes, 0, &live)

	a11yActionCallback(w, A11yActionPress, 0)
	if !clicked {
		t.Fatal("expected OnClick to fire")
	}
}

func TestA11yActionCallbackIncrement(t *testing.T) {
	var gotKey KeyCode
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleSlider,
			events: &eventHandlers{
				OnKeyDown: func(_ *Layout, e *Event, _ *Window) {
					gotKey = e.KeyCode
				},
			},
		},
	}
	w := newTestWindow()
	w.layout = layout
	w.a11y.nodes = w.a11y.nodes[:0]
	var live []liveNode
	a11yCollect(&w.layout, -1, &w.a11y.nodes, 0, &live)

	a11yActionCallback(w, A11yActionIncrement, 0)
	if gotKey != KeyUp {
		t.Fatalf("expected KeyUp, got %d", gotKey)
	}
}

func TestA11yActionCallbackDecrement(t *testing.T) {
	var gotKey KeyCode
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleSlider,
			events: &eventHandlers{
				OnKeyDown: func(_ *Layout, e *Event, _ *Window) {
					gotKey = e.KeyCode
				},
			},
		},
	}
	w := newTestWindow()
	w.layout = layout
	w.a11y.nodes = w.a11y.nodes[:0]
	var live []liveNode
	a11yCollect(&w.layout, -1, &w.a11y.nodes, 0, &live)

	a11yActionCallback(w, A11yActionDecrement, 0)
	if gotKey != KeyDown {
		t.Fatalf("expected KeyDown, got %d", gotKey)
	}
}

func TestA11yActionCallbackOutOfBounds(_ *testing.T) {
	w := newTestWindow()
	w.a11y.nodes = nil
	// Should not panic.
	a11yActionCallback(w, A11yActionPress, 5)
	a11yActionCallback(w, A11yActionPress, -1)
}

func TestA11yActionCallbackNilEvents(_ *testing.T) {
	layout := Layout{
		Shape: &Shape{A11YRole: AccessRoleButton},
	}
	w := newTestWindow()
	w.layout = layout
	w.a11y.nodes = w.a11y.nodes[:0]
	var live []liveNode
	a11yCollect(&w.layout, -1, &w.a11y.nodes, 0, &live)
	// Should not panic — no events on shape.
	a11yActionCallback(w, A11yActionPress, 0)
}

func TestA11yActionCallbackConfirmCancel(t *testing.T) {
	var keys []KeyCode
	layout := Layout{
		Shape: &Shape{
			A11YRole: AccessRoleButton,
			events: &eventHandlers{
				OnKeyDown: func(_ *Layout, e *Event, _ *Window) {
					keys = append(keys, e.KeyCode)
				},
			},
		},
	}
	w := newTestWindow()
	w.layout = layout
	w.a11y.nodes = w.a11y.nodes[:0]
	var live []liveNode
	a11yCollect(&w.layout, -1, &w.a11y.nodes, 0, &live)

	a11yActionCallback(w, A11yActionConfirm, 0)
	a11yActionCallback(w, A11yActionCancel, 0)
	if len(keys) != 2 {
		t.Fatalf("expected 2 key events, got %d", len(keys))
	}
	if keys[0] != KeyEnter {
		t.Errorf("confirm: expected KeyEnter, got %d", keys[0])
	}
	if keys[1] != KeyEscape {
		t.Errorf("cancel: expected KeyEscape, got %d", keys[1])
	}
}

func TestA11yFindLayoutWithChildren(t *testing.T) {
	layout := Layout{
		Shape: &Shape{A11YRole: AccessRoleGroup},
		Children: []Layout{
			{Shape: &Shape{
				A11YRole: AccessRoleButton,
				A11Y:     &AccessInfo{Label: "A"},
			}},
			{Shape: &Shape{
				A11YRole: AccessRoleButton,
				A11Y:     &AccessInfo{Label: "B"},
			}},
		},
	}
	// Index 0 = group, 1 = A, 2 = B.
	if l := a11yFindLayout(&layout, 0); l == nil ||
		l.Shape.A11YRole != AccessRoleGroup {
		t.Error("index 0 should be group")
	}
	if l := a11yFindLayout(&layout, 1); l == nil ||
		l.Shape.A11Y.Label != "A" {
		t.Error("index 1 should be A")
	}
	if l := a11yFindLayout(&layout, 2); l == nil ||
		l.Shape.A11Y.Label != "B" {
		t.Error("index 2 should be B")
	}
	if l := a11yFindLayout(&layout, 99); l != nil {
		t.Error("out of range should return nil")
	}
}

func TestA11yFindLayoutSkipsNoneRole(t *testing.T) {
	// Wrapper with no role, child with role.
	layout := Layout{
		Shape: &Shape{A11YRole: AccessRoleNone},
		Children: []Layout{
			{Shape: &Shape{
				A11YRole: AccessRoleButton,
				A11Y:     &AccessInfo{Label: "Inner"},
			}},
		},
	}
	if l := a11yFindLayout(&layout, 0); l == nil ||
		l.Shape.A11Y.Label != "Inner" {
		t.Error("index 0 should skip None and find Inner")
	}
}

func TestWindowCleanup(t *testing.T) {
	w := &Window{}
	w.storeBookmark("/a", nil)
	w.storeBookmark("/b", nil)
	w.WindowCleanup()
	if w.FileAccessGrantCount() != 0 {
		t.Errorf("grants not released: %d", w.FileAccessGrantCount())
	}
}

// --- syncA11y tests ---

// mockA11yPlatform records A11ySync and A11yAnnounce calls for
// testing the syncA11y integration path.
type mockA11yPlatform struct {
	NoopNativePlatform
	synced   []A11yNode
	syncCnt  int
	focusIdx int
	announce []string
}

func (m *mockA11yPlatform) A11ySync(
	nodes []A11yNode, count, focusedIdx int,
) {
	// Copy nodes to avoid aliasing reused slice.
	m.synced = append(m.synced[:0], nodes[:count]...)
	m.syncCnt = count
	m.focusIdx = focusedIdx
}

func (m *mockA11yPlatform) A11yAnnounce(text string) {
	m.announce = append(m.announce, text)
}

func newA11yWindow() *Window {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	w.SetNativePlatform(&mockA11yPlatform{})
	return w
}

func TestSyncA11yNoPlatform(t *testing.T) {
	w := newTestWindow()
	// syncA11y should no-op when nativePlatform is nil.
	// Must not panic.
	w.syncA11y()
}

func TestSyncA11yNotInitialized(t *testing.T) {
	w := newA11yWindow()
	w.layout = Layout{
		Shape: &Shape{A11YRole: AccessRoleButton},
	}
	// Platform set but a11y not initialized — should no-op.
	w.syncA11y()
	if mp := w.nativePlatform.(*mockA11yPlatform); mp.syncCnt != 0 {
		t.Error("A11ySync should not be called before init")
	}
}

func TestSyncA11yNilShape(t *testing.T) {
	w := newA11yWindow()
	w.a11y.initialized = true
	w.layout = Layout{} // Shape is nil
	w.syncA11y()
	if mp := w.nativePlatform.(*mockA11yPlatform); mp.syncCnt != 0 {
		t.Error("A11ySync should not be called with nil shape")
	}
}

func TestSyncA11yEmptyNodes(t *testing.T) {
	w := newA11yWindow()
	w.a11y.initialized = true
	w.layout = Layout{
		Shape: &Shape{A11YRole: AccessRoleNone},
	}
	w.syncA11y()
	if mp := w.nativePlatform.(*mockA11yPlatform); mp.syncCnt != 0 {
		t.Error("A11ySync should not be called when nodes are empty")
	}
}

func TestSyncA11yBuildsAndSyncsTree(t *testing.T) {
	w := newA11yWindow()
	w.a11y.initialized = true
	w.layout = Layout{
		Shape: &Shape{A11YRole: AccessRoleGroup},
		Children: []Layout{
			{Shape: &Shape{
				A11YRole: AccessRoleButton,
				A11Y:     &AccessInfo{Label: "OK"},
				X:        10, Y: 20, Width: 80, Height: 30,
			}},
			{Shape: &Shape{
				A11YRole: AccessRoleButton,
				A11Y:     &AccessInfo{Label: "Cancel"},
				X:        100, Y: 20, Width: 80, Height: 30,
			}},
		},
	}
	w.a11y.lastSync = time.Time{} // reset throttle
	w.syncA11y()

	mp := w.nativePlatform.(*mockA11yPlatform)
	if mp.syncCnt != 3 {
		t.Fatalf("syncCnt: got %d, want 3", mp.syncCnt)
	}
	// Node 0: group.
	n0 := mp.synced[0]
	if n0.Role != AccessRoleGroup {
		t.Errorf("node 0 role: got %d, want AccessRoleGroup", n0.Role)
	}
	if n0.ChildrenStart != 1 || n0.ChildrenCount != 2 {
		t.Errorf("node 0 children: start=%d count=%d, want 1,2",
			n0.ChildrenStart, n0.ChildrenCount)
	}
	// Node 1: OK button.
	n1 := mp.synced[1]
	if n1.Role != AccessRoleButton || n1.Label != "OK" {
		t.Errorf("node 1: role=%d label=%q, want Button/OK",
			n1.Role, n1.Label)
	}
	if n1.ParentIdx != 0 {
		t.Errorf("node 1 parent: got %d, want 0", n1.ParentIdx)
	}
	if n1.X != 10 || n1.Y != 20 || n1.W != 80 || n1.H != 30 {
		t.Errorf("node 1 bounds: %g,%g %gx%g", n1.X, n1.Y, n1.W, n1.H)
	}
	// Node 2: Cancel button.
	n2 := mp.synced[2]
	if n2.Role != AccessRoleButton || n2.Label != "Cancel" {
		t.Errorf("node 2: role=%d label=%q, want Button/Cancel",
			n2.Role, n2.Label)
	}
	if n2.ParentIdx != 0 {
		t.Errorf("node 2 parent: got %d, want 0", n2.ParentIdx)
	}
}

func TestSyncA11yTrackedFocus(t *testing.T) {
	w := newA11yWindow()
	w.a11y.initialized = true
	w.viewState.idFocus = 2
	w.layout = Layout{
		Shape: &Shape{A11YRole: AccessRoleGroup},
		Children: []Layout{
			{Shape: &Shape{
				A11YRole: AccessRoleButton,
				IDFocus:  1,
			}},
			{Shape: &Shape{
				A11YRole: AccessRoleButton,
				IDFocus:  2,
			}},
		},
	}
	w.a11y.lastSync = time.Time{}
	w.syncA11y()

	mp := w.nativePlatform.(*mockA11yPlatform)
	if mp.focusIdx != 2 {
		t.Errorf("focusIdx: got %d, want 2", mp.focusIdx)
	}
}

func TestSyncA11yThrottle(t *testing.T) {
	w := newA11yWindow()
	w.a11y.initialized = true
	w.layout = Layout{
		Shape: &Shape{A11YRole: AccessRoleButton},
	}
	w.a11y.lastSync = time.Time{}
	w.syncA11y()

	// Second call within throttle window should no-op.
	w.syncA11y()
	mp := w.nativePlatform.(*mockA11yPlatform)
	if mp.syncCnt != 1 {
		t.Errorf("throttle failed: %d calls, want 1", mp.syncCnt)
	}
}

func TestSyncA11yLiveRegionAnnounce(t *testing.T) {
	w := newA11yWindow()
	w.a11y.initialized = true
	w.layout = Layout{
		Shape: &Shape{
			A11YRole:  AccessRoleStaticText,
			A11YState: AccessStateLive,
			A11Y:      &AccessInfo{Label: "status", ValueNum: 100},
		},
	}
	w.a11y.lastSync = time.Time{}
	// First sync sets baseline.
	w.syncA11y()

	// Change the live region value.
	w.layout.Shape.A11Y.ValueNum = 200
	w.a11y.lastSync = time.Time{}
	w.syncA11y()

	mp := w.nativePlatform.(*mockA11yPlatform)
	if len(mp.announce) != 1 {
		t.Fatalf("expected 1 announce, got %d", len(mp.announce))
	}
	if mp.announce[0] != "200" {
		t.Errorf("announced value: got %q, want %q",
			mp.announce[0], "200")
	}
}

func TestSyncA11yLiveRegionNoChange(t *testing.T) {
	w := newA11yWindow()
	w.a11y.initialized = true
	w.layout = Layout{
		Shape: &Shape{
			A11YRole:  AccessRoleStaticText,
			A11YState: AccessStateLive,
			A11Y:      &AccessInfo{Label: "status", ValueNum: 50},
		},
	}
	w.a11y.lastSync = time.Time{}
	// First sync sets baseline.
	w.syncA11y()

	// Same value — no announce.
	w.a11y.lastSync = time.Time{}
	w.syncA11y()

	mp := w.nativePlatform.(*mockA11yPlatform)
	if len(mp.announce) != 0 {
		t.Errorf("expected 0 announces, got %d", len(mp.announce))
	}
}

func TestSyncA11yInitLazy(t *testing.T) {
	w := newA11yWindow()
	// Set layout before initA11y is called.
	w.layout = Layout{
		Shape: &Shape{A11YRole: AccessRoleButton},
	}
	// initA11y should set initialized=true.
	w.initA11y()
	if !w.a11y.initialized {
		t.Fatal("expected initialized after initA11y")
	}
	// Second call is idempotent.
	w.initA11y()
	if !w.a11y.initialized {
		t.Fatal("init should remain true")
	}
}

func TestSyncA11yNodesReused(t *testing.T) {
	w := newA11yWindow()
	w.a11y.initialized = true
	w.layout = Layout{
		Shape: &Shape{A11YRole: AccessRoleButton},
	}
	w.a11y.lastSync = time.Time{}
	w.syncA11y()

	// Capture the backing array pointer.
	oldCap := cap(w.a11y.nodes)
	nodesPtr := &w.a11y.nodes[:1][0]

	// Second sync should reuse the slice.
	w.a11y.lastSync = time.Time{}
	w.syncA11y()
	if cap(w.a11y.nodes) != oldCap {
		t.Error("node slice cap changed — expected reuse")
	}
	nodesPtr2 := &w.a11y.nodes[:1][0]
	if nodesPtr != nodesPtr2 {
		t.Error("node backing array reallocated — expected reuse")
	}
}

func TestWindowCleanupClearsRegistryAndContext(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.stopAnimationLoop()
	// Populate state registry.
	sm := StateMap[string, int](w, "test.ns", 10)
	sm.Set("a", 1)
	sm.Set("b", 2)
	if w.viewState.registry.entryCount("test.ns") != 2 {
		t.Fatal("registry not populated")
	}
	// Populate render guard.
	w.renderGuardWarned = 1 << RenderRect
	// Cleanup should clear all.
	w.WindowCleanup()
	if w.viewState.registry.entryCount("test.ns") != 0 {
		t.Errorf("registry not cleared: %d entries",
			w.viewState.registry.entryCount("test.ns"))
	}
	if w.renderGuardWarned != 0 {
		t.Error("renderGuardWarned not cleared")
	}
	if w.Ctx().Err() == nil {
		t.Error("context not cancelled after cleanup")
	}
}
