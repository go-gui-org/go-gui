package gui

import (
	"strconv"
	"time"
)

// Accessibility action constants.
const (
	A11yActionPress     = 0
	A11yActionIncrement = 1
	A11yActionDecrement = 2
	A11yActionConfirm   = 3
	A11yActionCancel    = 4
)

// A11yNode is a flat accessibility node pushed to the native
// accessibility backend each frame.
type A11yNode struct {
	Label         string
	Value         string
	Description   string
	ParentIdx     int
	ChildrenStart int
	ChildrenCount int
	X, Y, W, H    float32
	ValueNum      float32
	ValueMin      float32
	ValueMax      float32
	State         AccessState
	Role          AccessRole
}

type liveNode struct {
	key   liveKey
	value string
}

// liveKey identifies a live region across syncs. A non-empty id pins
// a designed, ID-bearing region on its own, across any tree or label
// movement; label and idx keep ID-less regions distinct per position.
// A region that moves reads as new and stays silent for a frame —
// never a spurious announcement — while its next value change
// announces normally.
type liveKey struct {
	id    string
	label string
	idx   int
}

// liveKeyFor builds the identity for one live region: the effective
// ID where the shape has one, else its label and node index.
func liveKeyFor(id, label string, idx int) liveKey {
	if id != "" {
		return liveKey{id: id}
	}
	return liveKey{label: label, idx: idx}
}

// a11ySyncInterval is the minimum time between accessibility
// tree syncs. 100ms (~10Hz) is responsive enough for screen
// readers while avoiding the cost of per-frame CGo calls.
const a11ySyncInterval = 100 * time.Millisecond

// a11y holds per-window accessibility backend state.
type a11y struct {
	lastSync       time.Time // throttle sync calls
	prevLiveValues map[liveKey]string
	nodes          []A11yNode // reused across frames
	liveNodes      []liveNode // reused across frames
	// dirty is set by updateLocked (any layout rebuild) and
	// setFocusLocked (a real focus change — the focused index is part
	// of the pushed snapshot but no layout rebuild accompanies it).
	// syncA11y clears it only when it actually pushes, so a dirty tree
	// that hits the throttle retries the next frame. An idle window
	// with a clean tree never walks the tree or touches cgo (issue
	// #407).
	dirty       bool
	initialized bool
}

// initA11y lazily creates the native accessibility container.
// Called from frame loop, same pattern as IME.
func (w *Window) initA11y() {
	if w.a11y.initialized {
		return
	}
	w.a11y.initialized = true
	// The first sync must always run; the zero value of dirty is
	// false, so mark it here rather than in the struct literal.
	w.a11y.dirty = true

	if w.nativePlatform != nil {
		w.nativePlatform.A11yInit(func(action, index int) {
			// Platform action callbacks arrive on foreign threads —
			// VoiceOver re-entry, Android JNI, D-Bus workers — never
			// the main thread the layout belongs to. Queue onto it;
			// the command runs at the next frame start, against the
			// arranged tree rather than one mutating underfoot.
			w.QueueCommand(func(*Window) {
				a11yActionCallback(w, action, index)
			})
		})
	}
}

// syncA11y walks the layout tree, builds a flat node array,
// and pushes it to the native accessibility backend.
// Throttled to a11ySyncInterval to avoid expensive per-frame
// CGo calls. When the tree is clean — no layout rebuild and no
// focus change since the last push — the walk and the cgo call
// are skipped entirely (issue #407).
func (w *Window) syncA11y() {
	if w.nativePlatform == nil || !w.a11y.initialized {
		return
	}
	if w.layout.Shape == nil {
		return
	}
	// A dirty tree that hits the throttle keeps its dirty flag and
	// retries on the next frame.
	if !w.a11y.dirty {
		return
	}
	now := time.Now()
	if now.Sub(w.a11y.lastSync) < a11ySyncInterval {
		return
	}
	w.a11y.lastSync = now
	w.a11y.dirty = false

	// Reuse slices across frames.
	w.a11y.nodes = w.a11y.nodes[:0]
	w.a11y.liveNodes = w.a11y.liveNodes[:0]

	focusedIdx := a11yCollect(
		&w.layout, -1,
		&w.a11y.nodes,
		w.viewState.focusID,
		&w.a11y.liveNodes,
	)

	// An emptied tree still pushes (possibly zero nodes) so the native
	// side clears its stale content instead of keeping it.
	w.nativePlatform.A11ySync(w.a11y.nodes, len(w.a11y.nodes), focusedIdx)

	// Live region change detection.
	for _, ln := range w.a11y.liveNodes {
		if prev, ok := w.a11y.prevLiveValues[ln.key]; ok {
			if prev != ln.value {
				w.nativePlatform.A11yAnnounce(ln.value)
			}
		}
	}
	// Update previous values.
	if w.a11y.prevLiveValues == nil {
		w.a11y.prevLiveValues = make(map[liveKey]string)
	}
	clear(w.a11y.prevLiveValues)
	for _, ln := range w.a11y.liveNodes {
		w.a11y.prevLiveValues[ln.key] = ln.value
	}
}

// a11yCollect recursively walks the layout tree and appends
// nodes to the flat array. Returns the index of the focused
// node, or -1 if not found.
func a11yCollect(
	layout *Layout,
	parentIdx int,
	nodes *[]A11yNode,
	focusID string,
	live *[]liveNode,
) int {
	return a11yCollectDepth(layout, parentIdx, nodes, focusID, live, 0)
}

func a11yCollectDepth(
	layout *Layout,
	parentIdx int,
	nodes *[]A11yNode,
	focusID string,
	live *[]liveNode,
	depth int,
) int {
	focusedIdx := -1
	// Past the budget the walk stops descending, like every other
	// tree walk: the tree is not always the app's own — Markdown and
	// SVG build subtrees out of documents the app did not write —
	// so the frame drops deep input rather than the process.
	if overMaxDepth(depth) {
		return focusedIdx
	}
	if layout.Shape == nil {
		return focusedIdx
	}
	s := layout.Shape

	// Skip shapes without a11y role but recurse children.
	if s.A11YRole == AccessRoleNone {
		for i := range layout.Children {
			if fi := a11yCollectDepth(&layout.Children[i], parentIdx, nodes, focusID, live, depth+1); fi >= 0 {
				focusedIdx = fi
			}
		}
		return focusedIdx
	}

	nodeIdx := len(*nodes)

	// Build label from AccessInfo or shape text.
	label := ""
	value := ""
	description := ""
	var valueNum, valueMin, valueMax float32
	if s.a11Y != nil {
		if s.a11Y.Label != "" {
			label = s.a11Y.Label
		}
		description = s.a11Y.Description
		value = a11yValueText(s.a11Y)
		valueNum = s.a11Y.ValueNum
		valueMin = s.a11Y.ValueMin
		valueMax = s.a11Y.ValueMax
	}
	if label == "" {
		label = shapeA11yLabel(s)
	}

	state := s.A11YState
	if s.Disabled {
		state |= AccessStateDisabled
	}

	*nodes = append(*nodes, A11yNode{
		Role:        s.A11YRole,
		State:       state,
		Label:       label,
		Value:       value,
		Description: description,
		X:           s.X,
		Y:           s.Y,
		W:           s.Width,
		H:           s.Height,
		ValueNum:    valueNum,
		ValueMin:    valueMin,
		ValueMax:    valueMax,
		ParentIdx:   parentIdx,
	})

	if focusID != "" && s.Focusable && s.idKey() == focusID {
		focusedIdx = nodeIdx
	}

	// Track live regions, keyed by identity rather than label:
	// labels collide across regions and vanish on unlabeled values,
	// and either case announced for the wrong region.
	if state.Has(AccessStateLive) {
		*live = append(*live, liveNode{
			key:   liveKeyFor(s.idKey(), label, nodeIdx),
			value: value,
		})
	}

	// Process children.
	childrenStart := len(*nodes)
	for i := range layout.Children {
		if fi := a11yCollectDepth(&layout.Children[i], nodeIdx, nodes, focusID, live, depth+1); fi >= 0 {
			focusedIdx = fi
		}
	}
	childrenCount := len(*nodes) - childrenStart

	// Update node's children info.
	(*nodes)[nodeIdx].ChildrenStart = childrenStart
	(*nodes)[nodeIdx].ChildrenCount = childrenCount

	return focusedIdx
}

// a11yValueText formats AccessInfo numeric values as text.
func a11yValueText(info *accessInfo) string {
	if info.ValueNum == 0 && info.ValueMin == 0 && info.ValueMax == 0 {
		return ""
	}
	return strconv.FormatFloat(
		float64(info.ValueNum), 'g', -1, 32)
}

// shapeA11yLabel extracts an accessibility label from shape text.
func shapeA11yLabel(s *Shape) string {
	if s.TC != nil && s.TC.Text != "" {
		return s.TC.Text
	}
	return ""
}

// a11yActionCallback routes native accessibility actions to
// the layout node at the given index in the a11y node array.
//
// Main-thread only: it walks the live layout and runs app callbacks
// with no lock held. Platform entry points never call this directly —
// initA11y queues them through QueueCommand. Tests call it directly.
// Disabled refuses every action (see the single check below);
// layoutDisables stamps Disabled onto every descendant, so it also
// covers a disabled ancestor.
func a11yActionCallback(w *Window, action, index int) {
	if index < 0 || index >= len(w.a11y.nodes) {
		return
	}
	l := a11yFindLayout(&w.layout, index)
	if l == nil || l.Shape == nil || l.Shape.events == nil {
		return
	}
	ev := l.Shape.events
	// A disabled widget refuses every action; layoutDisables stamps
	// Disabled onto every descendant, so this also covers a
	// disabled ancestor. One check, not one per arm.
	if l.Shape.Disabled {
		return
	}
	switch action {
	case A11yActionPress:
		if ev.OnClick != nil {
			e := &Event{Type: EventMouseDown}
			playShapeSound(l, w)
			ev.OnClick(EventCtx{l, e, w})
		}
	case A11yActionIncrement:
		if ev.OnKeyDown != nil {
			e := &Event{Type: EventKeyDown, KeyCode: KeyUp}
			ev.OnKeyDown(EventCtx{l, e, w})
		}
	case A11yActionDecrement:
		if ev.OnKeyDown != nil {
			e := &Event{Type: EventKeyDown, KeyCode: KeyDown}
			ev.OnKeyDown(EventCtx{l, e, w})
		}
	case A11yActionConfirm:
		if ev.OnKeyDown != nil {
			e := &Event{Type: EventKeyDown, KeyCode: KeyEnter}
			ev.OnKeyDown(EventCtx{l, e, w})
		}
	case A11yActionCancel:
		if ev.OnKeyDown != nil {
			e := &Event{Type: EventKeyDown, KeyCode: KeyEscape}
			ev.OnKeyDown(EventCtx{l, e, w})
		}
	}
}

// a11yFindLayout walks the layout tree in the same order as
// a11yCollect and returns the layout at the given node index.
func a11yFindLayout(layout *Layout, target int) *Layout {
	counter := 0
	return a11yFindLayoutWalk(layout, target, &counter, 0)
}

func a11yFindLayoutWalk(layout *Layout, target int, counter *int, depth int) *Layout {
	if overMaxDepth(depth) {
		return nil
	}
	if layout.Shape == nil {
		return nil
	}
	// Skip shapes without a11y role but recurse children
	// (same logic as a11yCollect).
	if layout.Shape.A11YRole == AccessRoleNone {
		for i := range layout.Children {
			if found := a11yFindLayoutWalk(&layout.Children[i], target, counter, depth+1); found != nil {
				return found
			}
		}
		return nil
	}
	if *counter == target {
		return layout
	}
	*counter++
	for i := range layout.Children {
		if found := a11yFindLayoutWalk(&layout.Children[i], target, counter, depth+1); found != nil {
			return found
		}
	}
	return nil
}

// WindowCleanup releases resources. Called by the backend
// during window destruction.
func (w *Window) WindowCleanup() {
	w.cleanupOnce.Do(func() {
		if w.cancelCtx != nil {
			w.cancelCtx()
		}
		w.stopAnimationLoop()
		w.releaseAllFileAccess()
		if w.nativePlatform != nil {
			w.nativePlatform.A11yDestroy()
		}
		w.clearViewStateLocked()
		w.renderGuardWarned = 0
		unregisterWindow(w)
	})
}
