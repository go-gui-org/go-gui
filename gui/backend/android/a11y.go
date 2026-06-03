//go:build android

package android

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

// a11y state shared between Go (writer) and Kotlin (reader via
// gomobile-exported functions).
var (
	a11yMu       sync.RWMutex
	a11yNodes    []gui.A11yNode
	a11yCount    int
	a11yFocused  int
	a11yActionCb func(int, int)
	a11yAnnounce string
)

// initA11y stores the action callback.
func initA11y(cb func(action, index int)) {
	a11yMu.Lock()
	a11yActionCb = cb
	a11yMu.Unlock()
}

// syncA11y copies the node slice under write lock.
func syncA11y(nodes []gui.A11yNode, count, focusedIdx int) {
	a11yMu.Lock()
	// Grow or reuse backing slice.
	if cap(a11yNodes) < count {
		a11yNodes = make([]gui.A11yNode, count)
	} else {
		a11yNodes = a11yNodes[:count]
	}
	copy(a11yNodes, nodes[:count])
	a11yCount = count
	a11yFocused = focusedIdx
	a11yMu.Unlock()
}

// destroyA11y clears all state.
func destroyA11y() {
	a11yMu.Lock()
	a11yNodes = nil
	a11yCount = 0
	a11yFocused = -1
	a11yActionCb = nil
	a11yAnnounce = ""
	a11yMu.Unlock()
}

// setA11yAnnounce stores text for Kotlin to pick up and
// announce via announceForAccessibility.
func setA11yAnnounce(text string) {
	a11yMu.Lock()
	a11yAnnounce = text
	a11yMu.Unlock()
}

// --- Gomobile-exported query functions ---
// Kotlin's AccessibilityNodeProvider calls these to build
// AccessibilityNodeInfo objects on demand.

// A11yNodeCount returns the number of accessibility nodes.
func A11yNodeCount() int32 {
	a11yMu.RLock()
	n := int32(a11yCount)
	a11yMu.RUnlock()
	return n
}

// A11yNodeRole returns the AccessRole for node at index.
func A11yNodeRole(index int32) int32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return int32(a11yNodes[index].Role)
}

// A11yNodeLabel returns the label for node at index.
func A11yNodeLabel(index int32) string {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return ""
	}
	return a11yNodes[index].Label
}

// A11yNodeValue returns the value for node at index.
func A11yNodeValue(index int32) string {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return ""
	}
	return a11yNodes[index].Value
}

// A11yNodeDescription returns the description for node at index.
func A11yNodeDescription(index int32) string {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return ""
	}
	return a11yNodes[index].Description
}

// A11yNodeBoundsX returns the X coordinate of node bounds.
func A11yNodeBoundsX(index int32) float32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return a11yNodes[index].X
}

// A11yNodeBoundsY returns the Y coordinate of node bounds.
func A11yNodeBoundsY(index int32) float32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return a11yNodes[index].Y
}

// A11yNodeBoundsW returns the width of node bounds.
func A11yNodeBoundsW(index int32) float32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return a11yNodes[index].W
}

// A11yNodeBoundsH returns the height of node bounds.
func A11yNodeBoundsH(index int32) float32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return a11yNodes[index].H
}

// A11yNodeState returns the AccessState bitmask for node.
func A11yNodeState(index int32) int64 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return int64(a11yNodes[index].State)
}

// A11yNodeValueNum returns the numeric value.
func A11yNodeValueNum(index int32) float32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return a11yNodes[index].ValueNum
}

// A11yNodeValueMin returns the minimum value.
func A11yNodeValueMin(index int32) float32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return a11yNodes[index].ValueMin
}

// A11yNodeValueMax returns the maximum value.
func A11yNodeValueMax(index int32) float32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return a11yNodes[index].ValueMax
}

// A11yNodeParent returns the parent index (-1 for root).
func A11yNodeParent(index int32) int32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return -1
	}
	return int32(a11yNodes[index].ParentIdx)
}

// A11yNodeChildStart returns the children start index.
func A11yNodeChildStart(index int32) int32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return int32(a11yNodes[index].ChildrenStart)
}

// A11yNodeChildCount returns the number of children.
func A11yNodeChildCount(index int32) int32 {
	a11yMu.RLock()
	defer a11yMu.RUnlock()
	if int(index) >= a11yCount {
		return 0
	}
	return int32(a11yNodes[index].ChildrenCount)
}

// A11yFocusedIndex returns the index of the focused node.
func A11yFocusedIndex() int32 {
	a11yMu.RLock()
	v := int32(a11yFocused)
	a11yMu.RUnlock()
	return v
}

// A11yPerformAction is called from Kotlin when the user
// performs an accessibility action on a virtual node.
func A11yPerformAction(index, action int32) {
	a11yMu.RLock()
	cb := a11yActionCb
	a11yMu.RUnlock()
	if cb != nil {
		cb(int(action), int(index))
	}
}

// PendingA11yAnnouncement returns and clears the pending
// accessibility announcement text.
func PendingA11yAnnouncement() string {
	a11yMu.Lock()
	t := a11yAnnounce
	a11yAnnounce = ""
	a11yMu.Unlock()
	return t
}
