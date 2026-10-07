//go:build windows && !js && (amd64 || arm64)

package gl

import (
	"math"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

// uiaTestTree is a window holding a dialog with a button, a checkbox
// and a list of two items (the second selected), then a slider:
//
//	0 dialog
//	  1 button
//	  2 checkbox (checked)
//	  3 list
//	    4 item
//	    5 item (selected)
//	6 slider
func uiaTestTree() []gui.A11yNode {
	return []gui.A11yNode{
		{Role: gui.AccessRoleDialog, Label: "Prefs", ParentIdx: -1, ChildrenStart: 1, ChildrenCount: 5, W: 200, H: 200},
		{Role: gui.AccessRoleButton, Label: "OK", ParentIdx: 0, X: 10, Y: 10, W: 50, H: 20},
		{Role: gui.AccessRoleCheckbox, Label: "Wrap", ParentIdx: 0, State: gui.AccessStateChecked, X: 10, Y: 40, W: 50, H: 20},
		{Role: gui.AccessRoleList, Label: "Fruit", ParentIdx: 0, ChildrenStart: 4, ChildrenCount: 2, X: 10, Y: 70, W: 100, H: 60},
		{Role: gui.AccessRoleListItem, Label: "Apple", ParentIdx: 3, X: 10, Y: 70, W: 100, H: 30},
		{Role: gui.AccessRoleListItem, Label: "Pear", ParentIdx: 3, State: gui.AccessStateSelected, X: 10, Y: 100, W: 100, H: 30},
		{Role: gui.AccessRoleSlider, Label: "Volume", ParentIdx: -1, ValueNum: 5, ValueMin: 0, ValueMax: 10, X: 0, Y: 210, W: 200, H: 20},
	}
}

func TestUIALinksDirectChildren(t *testing.T) {
	nodes := uiaTestTree()
	var l uiaLinks
	l.build(nodes)
	root := len(nodes)
	// ChildrenCount spans descendants; links must name direct
	// children only.
	if l.first[0] != 1 || l.last[0] != 3 {
		t.Fatalf("dialog children = %d..%d, want 1..3", l.first[0], l.last[0])
	}
	if l.next[3] != -1 || l.next[2] != 3 || l.prev[1] != -1 || l.prev[3] != 2 {
		t.Fatalf("sibling links wrong: next=%v prev=%v", l.next, l.prev)
	}
	if l.first[root] != 0 || l.last[root] != 6 || l.next[0] != 6 {
		t.Fatalf("root children wrong: first=%d last=%d", l.first[root], l.last[root])
	}
	if l.first[3] != 4 || l.last[3] != 5 {
		t.Fatalf("list children = %d..%d, want 4..5", l.first[3], l.last[3])
	}
	if l.first[1] != -1 {
		t.Fatalf("leaf has a child: %d", l.first[1])
	}
}

func TestUIALinksRejectForwardParent(t *testing.T) {
	// A parent index at or past the node cannot be right in a
	// pre-order walk; the node hangs off the root instead.
	nodes := []gui.A11yNode{
		{Role: gui.AccessRoleGroup, ParentIdx: 1},
		{Role: gui.AccessRoleButton, ParentIdx: 99},
		{Role: gui.AccessRoleButton, ParentIdx: 2},
	}
	var l uiaLinks
	l.build(nodes)
	root := len(nodes)
	for i := range nodes {
		if p := l.parent(nodes, i); p != root {
			t.Errorf("node %d parent = %d, want root", i, p)
		}
	}
	if l.first[root] != 0 || l.last[root] != 2 {
		t.Fatalf("root children = %d..%d", l.first[root], l.last[root])
	}
}

func TestUIAControlTypes(t *testing.T) {
	cases := []struct {
		role gui.AccessRole
		want int32
	}{
		{gui.AccessRoleButton, uiaCtlButton},
		{gui.AccessRoleCheckbox, uiaCtlCheckBox},
		{gui.AccessRoleTextField, uiaCtlEdit},
		{gui.AccessRoleTextArea, uiaCtlEdit},
		{gui.AccessRoleSlider, uiaCtlSlider},
		{gui.AccessRoleList, uiaCtlList},
		{gui.AccessRoleListItem, uiaCtlListItem},
		{gui.AccessRoleDialog, uiaCtlPane},
		{gui.AccessRoleRadioButton, uiaCtlRadioButton},
		{gui.AccessRoleTabItem, uiaCtlTabItem},
		{gui.AccessRoleHeading, uiaCtlText},
		{gui.AccessRoleNone, uiaCtlCustom},
	}
	for _, c := range cases {
		if got := uiaControlType(c.role); got != c.want {
			t.Errorf("role %d: control type %d, want %d", c.role, got, c.want)
		}
	}
	// Every role maps to a known control type; Custom only where meant.
	for r := gui.AccessRoleButton; r <= gui.AccessRoleTreeItem; r++ {
		if uiaControlType(r) == uiaCtlCustom && r != gui.AccessRoleGridCell {
			t.Errorf("role %d falls through to Custom", r)
		}
	}
}

func TestUIAPatterns(t *testing.T) {
	cases := []struct {
		role  gui.AccessRole
		iface int
		want  bool
	}{
		{gui.AccessRoleButton, ifaceInvoke, true},
		{gui.AccessRoleButton, ifaceToggle, false},
		{gui.AccessRoleCheckbox, ifaceToggle, true},
		{gui.AccessRoleCheckbox, ifaceInvoke, false},
		{gui.AccessRoleSwitchToggle, ifaceToggle, true},
		{gui.AccessRoleSlider, ifaceRangeValue, true},
		{gui.AccessRoleProgressBar, ifaceRangeValue, true},
		{gui.AccessRoleTextField, ifaceValue, true},
		{gui.AccessRoleTextField, ifaceInvoke, false},
		{gui.AccessRoleRadioButton, ifaceSelectionItem, true},
		{gui.AccessRoleListItem, ifaceSelectionItem, true},
		{gui.AccessRoleList, ifaceSelection, true},
		{gui.AccessRoleRadioGroup, ifaceSelection, true},
		{gui.AccessRoleComboBox, ifaceExpandCollapse, true},
		{gui.AccessRoleStaticText, ifaceInvoke, false},
	}
	for _, c := range cases {
		if got := uiaSupports(c.role, c.iface); got != c.want {
			t.Errorf("role %d iface %d: %v, want %v", c.role, c.iface, got, c.want)
		}
	}
}

func TestUIARangeReadOnly(t *testing.T) {
	slider := gui.A11yNode{Role: gui.AccessRoleSlider}
	if uiaRangeReadOnly(&slider) {
		t.Fatal("slider read-only")
	}
	slider.State = gui.AccessStateDisabled
	if !uiaRangeReadOnly(&slider) {
		t.Fatal("disabled slider writable")
	}
	bar := gui.A11yNode{Role: gui.AccessRoleProgressBar}
	if !uiaRangeReadOnly(&bar) {
		t.Fatal("progress bar writable")
	}
}

// --- COM level: calls through the real vtables, in process. ---

// uiaCall calls method i of the interface at obj.
func uiaCall(obj uintptr, i int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(ptrFromLParam(obj))
	fn := *(*uintptr)(ptrFromLParam(vtbl + uintptr(i)*unsafe.Sizeof(uintptr(0))))
	a := append([]uintptr{obj}, args...)
	r, _, _ := syscall.SyscallN(fn, a...)
	return r
}

// Vtable slots past IUnknown's three.
const (
	slotQI      = 0
	slotRelease = 2

	slotGetPattern  = 4 // IRawElementProviderSimple
	slotGetProperty = 5

	slotNavigate = 3 // IRawElementProviderFragment
	slotRect     = 5

	slotGetFocus = 4 // IRawElementProviderFragmentRoot

	slotInvoke      = 3 // IInvokeProvider
	slotToggleState = 4 // IToggleProvider::get_ToggleState
	slotRangeValue  = 4 // IRangeValueProvider::get_Value
	slotIsSelected  = 6 // ISelectionItemProvider::get_IsSelected
)

func newTestProvider(t *testing.T) (*uiaProvider, *[]int) {
	t.Helper()
	var acts []int
	p := newUIAProvider(0, func(action, index int) { acts = append(acts, action<<8|index) })
	t.Cleanup(p.destroy)
	p.sync(uiaTestTree(), 2, 1)
	return p, &acts
}

func qi(t *testing.T, obj uintptr, iid windows.GUID) uintptr {
	t.Helper()
	var out uintptr
	if hr := uiaCall(obj, slotQI, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&out))); hr != sOK {
		t.Fatalf("QueryInterface hr=%#x", hr)
	}
	return out
}

func navigate(t *testing.T, frag uintptr, dir uintptr) uintptr {
	t.Helper()
	var out uintptr
	if hr := uiaCall(frag, slotNavigate, dir, uintptr(unsafe.Pointer(&out))); hr != sOK {
		t.Fatalf("Navigate(%d) hr=%#x", dir, hr)
	}
	return out
}

func propI4(t *testing.T, simple uintptr, id int) (variant, uintptr) {
	t.Helper()
	var v variant
	hr := uiaCall(simple, slotGetProperty, uintptr(id), uintptr(unsafe.Pointer(&v)))
	return v, hr
}

func TestUIANavigateAndProperties(t *testing.T) {
	p, _ := newTestProvider(t)
	rootFrag := qi(t, p.root.ptr(ifaceSimple), uiaIIDs[ifaceFragment])
	defer uiaCall(rootFrag, slotRelease)

	dialog := navigate(t, rootFrag, 3) // FirstChild
	defer uiaCall(dialog, slotRelease)
	if uiaOf(dialog) != p.elems[0] {
		t.Fatal("root's first child is not the dialog")
	}
	button := navigate(t, dialog, 3)
	defer uiaCall(button, slotRelease)
	check := navigate(t, button, 1) // NextSibling
	defer uiaCall(check, slotRelease)
	list := navigate(t, check, 1)
	defer uiaCall(list, slotRelease)
	if uiaOf(list) != p.elems[3] {
		t.Fatal("button's second next sibling is not the list")
	}
	if end := navigate(t, list, 1); end != 0 {
		t.Fatal("list has a next sibling inside the dialog")
	}
	parent := navigate(t, dialog, 0)
	defer uiaCall(parent, slotRelease)
	if uiaOf(parent) != p.root {
		t.Fatal("dialog's parent is not the root")
	}
	if up := navigate(t, rootFrag, 0); up != 0 {
		t.Fatal("root has a parent")
	}

	simple := qi(t, button, uiaIIDs[ifaceSimple])
	defer uiaCall(simple, slotRelease)
	v, hr := propI4(t, simple, uiaControlTypeProperty)
	if hr != sOK || v.vt != vtI4 || int32(v.val) != uiaCtlButton {
		t.Fatalf("ControlType = %+v hr=%#x", v, hr)
	}
	v, _ = propI4(t, simple, uiaNameProperty)
	if v.vt != vtBSTR || windows.UTF16PtrToString((*uint16)(ptrFromLParam(uintptr(v.val)))) != "OK" {
		t.Fatalf("Name = %+v", v)
	}
	freeVariant(&v)
	v, _ = propI4(t, simple, uiaHasKeyboardFocusProperty)
	if v.vt != vtBool || v.val != 0 {
		t.Fatalf("button HasKeyboardFocus = %+v, want false", v)
	}

	checkSimple := qi(t, check, uiaIIDs[ifaceSimple])
	defer uiaCall(checkSimple, slotRelease)
	v, _ = propI4(t, checkSimple, uiaHasKeyboardFocusProperty)
	if v.vt != vtBool || v.val == 0 {
		t.Fatalf("checkbox HasKeyboardFocus = %+v, want true", v)
	}

	// The root's properties are its window's: empty.
	v, hr = propI4(t, p.root.ptr(ifaceSimple), uiaNameProperty)
	if hr != sOK || v.vt != vtEmpty {
		t.Fatalf("root Name = %+v hr=%#x", v, hr)
	}
}

func TestUIAFocusAndPatterns(t *testing.T) {
	p, acts := newTestProvider(t)
	root := qi(t, p.root.ptr(ifaceSimple), uiaIIDs[ifaceRoot])
	defer uiaCall(root, slotRelease)
	var focus uintptr
	if hr := uiaCall(root, slotGetFocus, uintptr(unsafe.Pointer(&focus))); hr != sOK || uiaOf(focus) != p.elems[2] {
		t.Fatalf("GetFocus hr=%#x, want the checkbox", hr)
	}
	uiaCall(focus, slotRelease)

	// A button has Invoke, not Toggle; Invoke presses it.
	btn := p.elems[1].ptr(ifaceSimple)
	var pat uintptr
	uiaCall(btn, slotGetPattern, 10015, uintptr(unsafe.Pointer(&pat)))
	if pat != 0 {
		t.Fatal("button has the Toggle pattern")
	}
	uiaCall(btn, slotGetPattern, 10000, uintptr(unsafe.Pointer(&pat)))
	if pat == 0 {
		t.Fatal("button lacks Invoke")
	}
	if hr := uiaCall(pat, slotInvoke); hr != sOK {
		t.Fatalf("Invoke hr=%#x", hr)
	}
	uiaCall(pat, slotRelease)
	if len(*acts) != 1 || (*acts)[0] != gui.A11yActionPress<<8|1 {
		t.Fatalf("actions = %v, want press on node 1", *acts)
	}

	var state int32
	tog := qi(t, p.elems[2].ptr(ifaceSimple), uiaIIDs[ifaceToggle])
	uiaCall(tog, slotToggleState, uintptr(unsafe.Pointer(&state)))
	uiaCall(tog, slotRelease)
	if state != 1 {
		t.Fatalf("ToggleState = %d, want On", state)
	}

	var sel int32
	item := qi(t, p.elems[5].ptr(ifaceSimple), uiaIIDs[ifaceSelectionItem])
	uiaCall(item, slotIsSelected, uintptr(unsafe.Pointer(&sel)))
	uiaCall(item, slotRelease)
	if sel == 0 {
		t.Fatal("selected list item reports unselected")
	}

	var val float64
	rv := qi(t, p.elems[6].ptr(ifaceSimple), uiaIIDs[ifaceRangeValue])
	uiaCall(rv, slotRangeValue, uintptr(unsafe.Pointer(&val)))
	uiaCall(rv, slotRelease)
	if val != 5 {
		t.Fatalf("RangeValue = %v, want 5", val)
	}

	// An interface the role lacks is refused.
	var none uintptr
	iid := uiaIIDs[ifaceRangeValue]
	if hr := uiaCall(btn, slotQI, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&none))); hr != eNoInterface || none != 0 {
		t.Fatalf("button QI RangeValue hr=%#x out=%#x", hr, none)
	}
}

func TestUIASetRangeValueSteps(t *testing.T) {
	p, acts := newTestProvider(t)
	e := p.elems[6]
	if hr := p.setRangeValue(e, 7); hr != sOK {
		t.Fatalf("hr=%#x", hr)
	}
	if hr := p.setRangeValue(e, 1); hr != sOK {
		t.Fatalf("hr=%#x", hr)
	}
	if hr := p.setRangeValue(e, 5); hr != sOK {
		t.Fatalf("hr=%#x", hr)
	}
	if hr := p.setRangeValue(e, math.NaN()); hr != eInvalidArg {
		t.Fatalf("NaN hr=%#x", hr)
	}
	want := []int{gui.A11yActionIncrement<<8 | 6, gui.A11yActionDecrement<<8 | 6}
	if len(*acts) != len(want) || (*acts)[0] != want[0] || (*acts)[1] != want[1] {
		t.Fatalf("actions = %v, want %v", *acts, want)
	}
}

func TestUIADisabledRefusesActions(t *testing.T) {
	p, acts := newTestProvider(t)
	nodes := uiaTestTree()
	nodes[1].State |= gui.AccessStateDisabled
	p.sync(nodes, -1, 1)
	if hr := p.perform(p.elems[1], gui.A11yActionPress); hr != uiaElementNotEnabled {
		t.Fatalf("hr=%#x, want UIA_E_ELEMENTNOTENABLED", hr)
	}
	if len(*acts) != 0 {
		t.Fatalf("disabled button acted: %v", *acts)
	}
}

func TestUIASyncKeepsAndReplacesElements(t *testing.T) {
	p, _ := newTestProvider(t)
	before := append([]*uiaElement(nil), p.elems...)
	held := before[1]
	held.addRef() // a client's reference

	// A value or label change keeps every element.
	nodes := uiaTestTree()
	nodes[1].Label = "Okay"
	nodes[6].ValueNum = 6
	p.sync(nodes, 1, 1)
	for i, e := range p.elems {
		if e != before[i] {
			t.Fatalf("element %d replaced on a value change", i)
		}
	}
	if p.focus != 1 {
		t.Fatalf("focus = %d, want 1", p.focus)
	}

	// A role change at an index replaces that element and, through
	// the parent link, nothing else.
	nodes[1].Role = gui.AccessRoleLink
	p.sync(nodes, -1, 1)
	if p.elems[1] == held {
		t.Fatal("element kept across a role change")
	}
	if !held.dead.Load() {
		t.Fatal("replaced element still live")
	}
	if p.elems[0] != before[0] || p.elems[2] != before[2] {
		t.Fatal("siblings replaced with the changed node")
	}
	// The client's reference now answers not available.
	var out uintptr
	if hr := uiaCall(held.ptr(ifaceFragment), slotNavigate, 0, uintptr(unsafe.Pointer(&out))); hr != uiaElementNotAvailable {
		t.Fatalf("dead Navigate hr=%#x", hr)
	}
	if _, ok := uiaLive[held]; !ok {
		t.Fatal("element freed while a client holds it")
	}
	held.release()
	uiaLiveMu.Lock()
	_, ok := uiaLive[held]
	uiaLiveMu.Unlock()
	if ok {
		t.Fatal("element kept after its last release")
	}

	// A shorter tree drops the tail.
	p.sync(nodes[:2], -1, 1)
	if len(p.elems) != 2 || !before[6].dead.Load() {
		t.Fatal("dropped node's element still live")
	}
}

func TestUIARectAndHitTest(t *testing.T) {
	p, _ := newTestProvider(t)
	p.sync(uiaTestTree(), -1, 2) // 200% scale
	ox, oy := p.clientOrigin()
	var r uiaRect
	uiaCall(p.elems[1].ptr(ifaceFragment), slotRect, uintptr(unsafe.Pointer(&r)))
	want := uiaRect{ox + 20, oy + 20, 100, 40}
	if r != want {
		t.Fatalf("rect = %+v, want %+v", r, want)
	}
	// The deepest node at a point wins over its ancestors.
	if e := p.at(ox+30, oy+210); e != p.elems[5] {
		t.Fatal("hit test missed the second list item")
	}
	if e := p.at(ox-1, oy-1); e != nil {
		t.Fatal("hit outside every node")
	}
}

func TestUIAStringsAreCapped(t *testing.T) {
	long := make([]byte, maxUIAText*2)
	for i := range long {
		long[i] = 'a'
	}
	b := uiaBSTR(string(long))
	defer pSysFreeString.Call(b)
	if n := len(windows.UTF16PtrToString((*uint16)(ptrFromLParam(b)))); n > maxUIAText {
		t.Fatalf("BSTR length %d over cap", n)
	}
	// An embedded NUL must not panic, as windows.StringToUTF16 would.
	nul := uiaBSTR("a\x00b")
	pSysFreeString.Call(nul)
}

func TestUIADestroyDisconnects(t *testing.T) {
	p := newUIAProvider(0, nil)
	p.sync(uiaTestTree(), -1, 1)
	elems := append([]*uiaElement(nil), p.elems...)
	p.destroy()
	p.destroy() // idempotent
	for i, e := range elems {
		if !e.dead.Load() {
			t.Fatalf("element %d live after destroy", i)
		}
	}
	if !p.root.dead.Load() {
		t.Fatal("root live after destroy")
	}
	p.sync(uiaTestTree(), -1, 1) // ignored, no panic
	if len(p.elems) != 0 {
		t.Fatal("sync after destroy rebuilt the tree")
	}
}

// TestUIADoubleThunks calls the two methods that take doubles through
// their vtable slots, so the assembly thunk runs. Go's windows/amd64
// calls copy the first integer arguments to XMM1-3 as well, which is
// what the thunk reads; arm64 calls do not set the FP registers.
func TestUIADoubleThunks(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("SyscallN does not set FP registers on " + runtime.GOARCH)
	}
	p, acts := newTestProvider(t)
	p.sync(uiaTestTree(), -1, 2)
	ox, oy := p.clientOrigin()

	root := qi(t, p.root.ptr(ifaceSimple), uiaIIDs[ifaceRoot])
	defer uiaCall(root, slotRelease)
	var hit uintptr
	hr := uiaCall(root, 3, // ElementProviderFromPoint
		uintptr(math.Float64bits(ox+30)), uintptr(math.Float64bits(oy+210)),
		uintptr(unsafe.Pointer(&hit)))
	if hr != sOK || hit == 0 || uiaOf(hit) != p.elems[5] {
		t.Fatalf("ElementProviderFromPoint hr=%#x, missed the list item", hr)
	}
	uiaCall(hit, slotRelease)

	rv := qi(t, p.elems[6].ptr(ifaceSimple), uiaIIDs[ifaceRangeValue])
	defer uiaCall(rv, slotRelease)
	if hr := uiaCall(rv, 3, uintptr(math.Float64bits(9))); hr != sOK { // SetValue
		t.Fatalf("SetValue hr=%#x", hr)
	}
	if len(*acts) != 1 || (*acts)[0] != gui.A11yActionIncrement<<8|6 {
		t.Fatalf("actions = %v, want one increment on node 6", *acts)
	}
}

func TestUIANameFromContent(t *testing.T) {
	nodes := []gui.A11yNode{
		{Role: gui.AccessRoleButton, ParentIdx: -1, ChildrenStart: 1, ChildrenCount: 3},
		{Role: gui.AccessRoleStaticText, Label: "Save", ParentIdx: 0},
		{Role: gui.AccessRoleGroup, ParentIdx: 0, ChildrenStart: 3, ChildrenCount: 1},
		{Role: gui.AccessRoleStaticText, Label: "all", ParentIdx: 2},
		{Role: gui.AccessRoleStaticText, Label: "outside", ParentIdx: -1},
		{Role: gui.AccessRoleGroup, ParentIdx: -1, ChildrenStart: 6, ChildrenCount: 1},
		{Role: gui.AccessRoleStaticText, Label: "group text", ParentIdx: 5},
		// A descendant span past the array is clamped, not trusted.
		{Role: gui.AccessRoleLink, ParentIdx: -1, ChildrenStart: 8, ChildrenCount: 50},
	}
	if got := uiaNameFromContent(nodes, 0); got != "Save all" {
		t.Fatalf("button name = %q, want %q", got, "Save all")
	}
	if got := uiaNameFromContent(nodes, 5); got != "" {
		t.Fatalf("group took a name from content: %q", got)
	}
	if got := uiaNameFromContent(nodes, 7); got != "" {
		t.Fatalf("link past the end = %q", got)
	}
}
