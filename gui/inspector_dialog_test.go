package gui

import (
	"strconv"
	"strings"
	"testing"
)

// Issue #811: with a dialog open, the inspector panel took no clicks and a
// pick-click on a dialog widget selected the app widget behind the dialog.

// inspectorDialogWindow renders an app view, opens a custom dialog with a
// button in it, and turns the inspector on. The returned window has
// settled: its layers are main, dialog, inspector (in that order after
// the fix).
func inspectorDialogWindow(t *testing.T) *Window {
	t.Helper()
	requireInspector(t)
	w := NewTestWindow(t, WindowCfg{})
	w.inspectorEnabled = true
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "app", Sizing: FillFill})
	})
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		FocusID:    "dlg-btn",
		CustomView: func(*Window) View {
			return Button(ButtonCfg{
				ID:      "dlg-btn",
				Content: []View{Text(TextCfg{Text: "OK"})},
			})
		},
	})
	w.TestRender(nil)
	w.TestRender(nil)
	if !w.DialogIsVisible() {
		t.Fatal("dialog did not open")
	}
	return w
}

// layerIndex returns the index of the top-level layer whose root has the
// given key, or -1.
func layerIndex(w *Window, key string) int {
	for i := range w.layout.Children {
		if s := w.layout.Children[i].Shape; s != nil && s.idKey() == key {
			return i
		}
	}
	return -1
}

// firstClickable returns the first descendant of ly (pre-order) that has
// an OnClick handler and a visible clip.
func firstClickable(ly *Layout) *Layout {
	if ly.Shape != nil && ly.Shape.hasEvents() &&
		ly.Shape.events.OnClick != nil &&
		ly.Shape.shapeClip.Width > 0 && ly.Shape.shapeClip.Height > 0 {
		return ly
	}
	for i := range ly.Children {
		if got := firstClickable(&ly.Children[i]); got != nil {
			return got
		}
	}
	return nil
}

func TestInspectorPanelAboveDialog(t *testing.T) {
	w := inspectorDialogWindow(t)
	n := len(w.layout.Children)
	if n < 3 {
		t.Fatalf("layers = %d, want >= 3", n)
	}
	if !isInspectorLayer(&w.layout.Children[n-1]) {
		t.Fatalf("top layer is %q, want the inspector panel",
			w.layout.Children[n-1].Shape.idKey())
	}
	if got := layerIndex(w, reservedDialogID); got != n-2 {
		t.Fatalf("dialog layer = %d, want %d (just under the inspector)",
			got, n-2)
	}
}

func TestDialogRouteIncludesInspectorLayer(t *testing.T) {
	w := inspectorDialogWindow(t)
	route := dialogRoute(w)
	if route == nil || len(route.Children) != 2 {
		t.Fatalf("route children = %d, want 2 (dialog + inspector)",
			len(route.Children))
	}
	if route.Children[0].Shape.idKey() != reservedDialogID {
		t.Fatalf("route[0] = %q, want dialog", route.Children[0].Shape.idKey())
	}
	if !isInspectorLayer(&route.Children[1]) {
		t.Fatalf("route[1] = %q, want inspector", route.Children[1].Shape.idKey())
	}
}

// A dialog opened since the last frame is visible but not yet a layer: the
// stale layers are app, inspector. The route must not widen to the app
// layer under the panel, or a click leaks past the modal dialog.
func TestDialogRouteStaleLayoutExcludesAppLayer(t *testing.T) {
	requireInspector(t)
	w := NewTestWindow(t, WindowCfg{})
	w.inspectorEnabled = true
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "app", Sizing: FillFill})
	})
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	if !w.DialogIsVisible() {
		t.Fatal("dialog not visible")
	}
	if layerIndex(w, reservedDialogID) >= 0 {
		t.Fatal("layout not stale: dialog layer already present")
	}
	route := dialogRoute(w)
	if route == &w.layout {
		t.Fatal("route is the whole root")
	}
	for i := range route.Children {
		if &route.Children[i] == &w.layout.Children[0] {
			t.Fatal("route includes the app layer")
		}
	}
	if route == &w.layout.Children[0] {
		t.Fatal("route is the app layer")
	}
}

func TestInspectorPanelClickWithDialogOpen(t *testing.T) {
	w := inspectorDialogWindow(t)
	tree, ok := w.layout.FindByID(inspectorTreeID)
	if !ok {
		t.Fatal("inspector tree not rendered")
	}
	row := firstClickable(tree)
	if row == nil {
		t.Fatal("no clickable tree row")
	}
	c := row.Shape.shapeClip
	x, y := c.X+c.Width/2, c.Y+c.Height/2
	down := Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y}
	w.EventFn(&down)
	w.settle()
	up := Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: x, MouseY: y}
	w.EventFn(&up)
	w.settle()
	if got := inspectorSelectedPath(w); got == "" {
		t.Fatal("click on an inspector tree row selected nothing while a " +
			"dialog is open")
	}
	if !w.DialogIsVisible() {
		t.Fatal("click on the inspector panel dismissed the dialog")
	}
}

func TestInspectorPickInsideDialog(t *testing.T) {
	w := inspectorDialogWindow(t)
	dlg := layerIndex(w, reservedDialogID)
	if dlg < 0 {
		t.Fatal("no dialog layer")
	}
	btn, ok := w.layout.Children[dlg].FindByID(ScopeID(reservedDialogID, "dlg-btn"))
	if !ok {
		t.Fatal("dialog button not rendered")
	}
	c := btn.Shape.shapeClip
	x, y := c.X+c.Width/2, c.Y+c.Height/2
	down := Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y}
	w.EventFn(&down)
	w.settle()

	sel := inspectorSelectedPath(w)
	head, _, _ := strings.Cut(sel, IDSep)
	if head != strconv.Itoa(dlg) {
		t.Fatalf("picked %q, want a path in dialog layer %d", sel, dlg)
	}
	node, ok := inspectorFindByPath(&w.layout, sel)
	if !ok || node.Shape == nil {
		t.Fatalf("picked path %q does not resolve", sel)
	}
	if !node.Shape.PointInShape(x, y) {
		t.Fatalf("picked node %q does not contain the pick point", sel)
	}
}

func TestInspectorTreeListsDialogLayer(t *testing.T) {
	w := inspectorDialogWindow(t)
	dlg := layerIndex(w, reservedDialogID)
	want := strconv.Itoa(dlg)
	found := false
	for _, n := range w.inspectorTreeCache {
		if n.ID == want {
			found = true
		}
		if strings.Contains(n.Text, inspectorScrollPanel) {
			t.Fatalf("tree lists the inspector's own layer: %q", n.Text)
		}
	}
	if !found {
		t.Fatalf("tree roots do not include dialog layer %q", want)
	}
}

func TestInspectorTreeKeepsFocusWithDialogOpen(t *testing.T) {
	w := inspectorDialogWindow(t)
	w.SetFocus(inspectorTreeID)
	w.TestRender(nil)
	w.TestRender(nil)
	if got := w.FocusID(); got != inspectorTreeID {
		t.Fatalf("focus = %q, want %q (dialog took it back)",
			got, inspectorTreeID)
	}
}
