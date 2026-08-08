package gui

import (
	"strings"
	"testing"
)

// collapseTree builds root -> outer -> inner, all covering the same
// 100x100 box so a click at (5,5) hits every level. Parent pointers are
// wired, which the collapse check walks. Returns the root.
func collapseTree(outer, inner *eventHandlers) *Layout {
	box := drawClip{X: 0, Y: 0, Width: 100, Height: 100}
	root := &Layout{
		Shape: &Shape{shapeClip: box},
		Children: []Layout{{
			Shape: &Shape{ID: "outer", events: outer, shapeClip: box},
			Children: []Layout{{
				Shape: &Shape{ID: "inner", events: inner, shapeClip: box},
			}},
		}},
	}
	layoutParents(root, nil)
	return root
}

// clickCollapse dispatches a left-click at (5,5) and returns whatever
// the debug gate wrote.
func clickCollapse(t *testing.T, root *Layout) string {
	t.Helper()
	buf := captureDebug(t)
	mouseDownHandler(root, false, &Event{MouseX: 5, MouseY: 5}, &Window{})
	return buf.String()
}

// The hazard itself: the inner handler never says Consume, so today the
// pre-mark stops the event, and under §4.3b it would not.
func TestCollapseWarnsWhenAncestorAlsoHandles(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnClick: func(EventCtx) {}},
		&eventHandlers{OnClick: func(EventCtx) {}},
	)
	got := clickCollapse(t, root)
	if !strings.Contains(got, `OnClick on "inner"`) ||
		!strings.Contains(got, `"outer"`) {
		t.Errorf("expected a collapse finding naming both shapes, got %q", got)
	}
}

// An explicit Consume() is what the collapse asks for, so such a site
// is already correct and must stay quiet.
func TestCollapseSilentOnExplicitConsume(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnClick: func(EventCtx) {}},
		&eventHandlers{OnClick: func(ctx EventCtx) { ctx.Consume() }},
	)
	if got := clickCollapse(t, root); got != "" {
		t.Errorf("explicit Consume() is collapse-safe, got %q", got)
	}
}

// Bubble() means the event already reaches the ancestor today, so the
// collapse changes nothing for this site.
func TestCollapseSilentOnBubble(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnClick: func(EventCtx) {}},
		&eventHandlers{OnClick: func(ctx EventCtx) { ctx.Bubble() }},
	)
	if got := clickCollapse(t, root); got != "" {
		t.Errorf("Bubble() already propagates, got %q", got)
	}
}

// The common case §4.3.1 could not count: a handler with no handling
// ancestor. Most of the 138 example sites are expected to land here.
func TestCollapseSilentWithoutAncestorHandler(t *testing.T) {
	root := collapseTree(nil, &eventHandlers{OnClick: func(EventCtx) {}})
	if got := clickCollapse(t, root); got != "" {
		t.Errorf("no ancestor handler means no hazard, got %q", got)
	}
}

// Dispatch skips disabled subtrees, so a disabled ancestor is not a
// second handler.
func TestCollapseSilentWhenAncestorDisabled(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnClick: func(EventCtx) {}},
		&eventHandlers{OnClick: func(EventCtx) {}},
	)
	root.Children[0].Shape.Disabled = true
	if got := clickCollapse(t, root); got != "" {
		t.Errorf("disabled ancestor never receives the click, got %q", got)
	}
}

// ClickButton is part of the ancestor's dispatch condition, so an
// ancestor listening for right-click only is not reached by a left one.
func TestCollapseRespectsAncestorClickButton(t *testing.T) {
	root := collapseTree(
		&eventHandlers{
			OnClick: func(EventCtx) {}, ClickButton: MouseRight,
		},
		&eventHandlers{OnClick: func(EventCtx) {}},
	)
	if got := clickCollapse(t, root); got != "" {
		t.Errorf("button filter excludes the ancestor, got %q", got)
	}
}

// The check is diagnostics only; with the gate off it must not fire.
func TestCollapseSilentWhenDebugOff(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnClick: func(EventCtx) {}},
		&eventHandlers{OnClick: func(EventCtx) {}},
	)
	buf := captureDebug(t)
	Debug(false)
	mouseDownHandler(root, false, &Event{MouseX: 5, MouseY: 5}, &Window{})
	if got := buf.String(); got != "" {
		t.Errorf("gate off must be silent, got %q", got)
	}
}

// Findings are warn-once per window, so holding a mouse button down for
// a second does not produce sixty identical lines.
func TestCollapseWarnsOncePerWindow(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnClick: func(EventCtx) {}},
		&eventHandlers{OnClick: func(EventCtx) {}},
	)
	buf := captureDebug(t)
	w := &Window{}
	for range 3 {
		mouseDownHandler(root, false, &Event{MouseX: 5, MouseY: 5}, w)
	}
	if n := strings.Count(buf.String(), "OnClick on"); n != 1 {
		t.Errorf("want 1 finding across 3 dispatches, got %d", n)
	}
}

// OnMouseUp is consume-class too, and shares the ancestor rule with
// OnClick.
func TestCollapseCoversMouseUp(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnMouseUp: func(EventCtx) {}},
		&eventHandlers{OnMouseUp: func(EventCtx) {}},
	)
	buf := captureDebug(t)
	mouseUpHandler(root, &Event{MouseX: 5, MouseY: 5}, &Window{})
	if !strings.Contains(buf.String(), `OnMouseUp on "inner"`) {
		t.Errorf("expected an OnMouseUp finding, got %q", buf.String())
	}
}

// OnChar reaches only the focused target and a window has one focus ID,
// so nesting two OnChar handlers is not a hazard. Pins the reasoning in
// wouldReach rather than an accident of the tree.
func TestCollapseSilentForNestedOnChar(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnChar: func(EventCtx) {}},
		&eventHandlers{OnChar: func(EventCtx) {}},
	)
	for _, l := range []*Layout{
		&root.Children[0], &root.Children[0].Children[0],
	} {
		l.Shape.Focusable = true
	}
	buf := captureDebug(t)
	w := &Window{}
	w.SetFocus("inner")
	charHandler(root, &Event{CharCode: 'x'}, w)
	if got := buf.String(); got != "" {
		t.Errorf("only one shape can hold focus, got %q", got)
	}
}

// Notify-class dispatch never pre-marks, so the check must not run for
// it at all — an unconsumed OnMouseMove under another is by design.
func TestCollapseIgnoresNotifyClass(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnMouseMove: func(EventCtx) {}},
		&eventHandlers{OnMouseMove: func(EventCtx) {}},
	)
	buf := captureDebug(t)
	mouseMoveHandler(root, &Event{MouseX: 5, MouseY: 5}, &Window{})
	if got := buf.String(); got != "" {
		t.Errorf("notify-class is out of scope, got %q", got)
	}
}

// The unnamed generic evConsume carries no ancestor rule, so callers
// that have not been given a specific kind stay silent rather than
// guessing.
func TestCollapseSkipsUnnamedConsumeClass(t *testing.T) {
	root := collapseTree(
		&eventHandlers{OnClick: func(EventCtx) {}},
		&eventHandlers{OnClick: func(EventCtx) {}},
	)
	inner := &root.Children[0].Children[0]
	buf := captureDebug(t)
	callRelative(inner, &Event{MouseX: 5, MouseY: 5}, &Window{},
		inner.Shape.events.OnClick, evConsume)
	if got := buf.String(); got != "" {
		t.Errorf("evConsume names no event, got %q", got)
	}
}
