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
	label string
	value string
}

// a11ySyncInterval is the minimum time between accessibility
// tree syncs. 100ms (~10Hz) is responsive enough for screen
// readers while avoiding the cost of per-frame CGo calls.
const a11ySyncInterval = 100 * time.Millisecond

// a11y holds per-window accessibility backend state.
type a11y struct {
	lastSync       time.Time // throttle sync calls
	prevLiveValues map[string]string
	nodes          []A11yNode // reused across frames
	liveNodes      []liveNode // reused across frames
	prevFocusID    string
	initialized    bool
}

// initA11y lazily creates the native accessibility container.
// Called from frame loop, same pattern as IME.
func (w *Window) initA11y() {
	if w.a11y.initialized {
		return
	}
	w.a11y.initialized = true

	if w.nativePlatform != nil {
		w.nativePlatform.A11yInit(func(action, index int) {
			a11yActionCallback(w, action, index)
		})
	}
}

// syncA11y walks the layout tree, builds a flat node array,
// and pushes it to the native accessibility backend.
// Throttled to a11ySyncInterval to avoid expensive per-frame
// CGo calls.
func (w *Window) syncA11y() {
	if w.nativePlatform == nil || !w.a11y.initialized {
		return
	}
	if w.layout.Shape == nil {
		return
	}
	now := time.Now()
	if now.Sub(w.a11y.lastSync) < a11ySyncInterval {
		return
	}
	w.a11y.lastSync = now

	// Reuse slices across frames.
	w.a11y.nodes = w.a11y.nodes[:0]
	w.a11y.liveNodes = w.a11y.liveNodes[:0]

	focusedIdx := a11yCollect(
		&w.layout, -1,
		&w.a11y.nodes,
		w.viewState.focusID,
		&w.a11y.liveNodes,
	)

	if len(w.a11y.nodes) == 0 {
		return
	}

	w.nativePlatform.A11ySync(w.a11y.nodes, len(w.a11y.nodes), focusedIdx)

	// Live region change detection.
	for _, ln := range w.a11y.liveNodes {
		if prev, ok := w.a11y.prevLiveValues[ln.label]; ok {
			if prev != ln.value {
				w.nativePlatform.A11yAnnounce(ln.value)
			}
		}
	}
	// Update previous values.
	if w.a11y.prevLiveValues == nil {
		w.a11y.prevLiveValues = make(map[string]string)
	}
	clear(w.a11y.prevLiveValues)
	for _, ln := range w.a11y.liveNodes {
		w.a11y.prevLiveValues[ln.label] = ln.value
	}
	w.a11y.prevFocusID = w.viewState.focusID
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
	focusedIdx := -1
	if layout.Shape == nil {
		return focusedIdx
	}
	s := layout.Shape

	// Skip shapes without a11y role but recurse children.
	if s.A11YRole == AccessRoleNone {
		for i := range layout.Children {
			if fi := a11yCollect(&layout.Children[i], parentIdx, nodes, focusID, live); fi >= 0 {
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
	if s.A11Y != nil {
		if s.A11Y.Label != "" {
			label = s.A11Y.Label
		}
		description = s.A11Y.Description
		value = a11yValueText(s.A11Y)
		valueNum = s.A11Y.ValueNum
		valueMin = s.A11Y.ValueMin
		valueMax = s.A11Y.ValueMax
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

	if focusID != "" && s.Focusable && s.ID == focusID {
		focusedIdx = nodeIdx
	}

	// Track live regions.
	if state.Has(AccessStateLive) {
		*live = append(*live, liveNode{label: label, value: value})
	}

	// Process children.
	childrenStart := len(*nodes)
	for i := range layout.Children {
		if fi := a11yCollect(&layout.Children[i], nodeIdx, nodes, focusID, live); fi >= 0 {
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
func a11yValueText(info *AccessInfo) string {
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
func a11yActionCallback(w *Window, action, index int) {
	if index < 0 || index >= len(w.a11y.nodes) {
		return
	}
	l := a11yFindLayout(&w.layout, index)
	if l == nil || l.Shape == nil || l.Shape.events == nil {
		return
	}
	ev := l.Shape.events
	switch action {
	case A11yActionPress:
		if ev.OnClick != nil {
			e := &Event{Type: EventMouseDown}
			ev.OnClick(l, e, w)
		}
	case A11yActionIncrement:
		if ev.OnKeyDown != nil {
			e := &Event{Type: EventKeyDown, KeyCode: KeyUp}
			ev.OnKeyDown(l, e, w)
		}
	case A11yActionDecrement:
		if ev.OnKeyDown != nil {
			e := &Event{Type: EventKeyDown, KeyCode: KeyDown}
			ev.OnKeyDown(l, e, w)
		}
	case A11yActionConfirm:
		if ev.OnKeyDown != nil {
			e := &Event{Type: EventKeyDown, KeyCode: KeyEnter}
			ev.OnKeyDown(l, e, w)
		}
	case A11yActionCancel:
		if ev.OnKeyDown != nil {
			e := &Event{Type: EventKeyDown, KeyCode: KeyEscape}
			ev.OnKeyDown(l, e, w)
		}
	}
}

// a11yFindLayout walks the layout tree in the same order as
// a11yCollect and returns the layout at the given node index.
func a11yFindLayout(layout *Layout, target int) *Layout {
	counter := 0
	return a11yFindLayoutWalk(layout, target, &counter)
}

func a11yFindLayoutWalk(layout *Layout, target int, counter *int) *Layout {
	if layout.Shape == nil {
		return nil
	}
	// Skip shapes without a11y role but recurse children
	// (same logic as a11yCollect).
	if layout.Shape.A11YRole == AccessRoleNone {
		for i := range layout.Children {
			if found := a11yFindLayoutWalk(&layout.Children[i], target, counter); found != nil {
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
		if found := a11yFindLayoutWalk(&layout.Children[i], target, counter); found != nil {
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
		w.ReleaseAllFileAccess()
		if w.nativePlatform != nil {
			w.nativePlatform.A11yDestroy()
		}
		w.clearViewStateLocked()
		w.renderGuardWarned = 0
	})
}
