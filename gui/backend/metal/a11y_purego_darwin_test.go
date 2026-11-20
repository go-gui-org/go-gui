//go:build darwin && arm64 && !ios

package metal

// Tests for the purego accessibility tree (#1317). They build a tree
// with no window and no view: a send to nil does nothing, and the
// screen offset of a nil view is (0, 0). The tests then ask the
// elements what VoiceOver asks, through the Objective-C runtime.

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// a11yTestTree starts a tree for token and removes it at cleanup.
func a11yTestTree(t *testing.T, token uintptr) *a11yCtx {
	t.Helper()
	a11yInitView(token, 0, 0)
	t.Cleanup(func() { a11yDestroy(token) })
	ctx := a11yCtxs[token]
	if ctx == nil {
		t.Fatal("a11yInitView made no tree")
	}
	return ctx
}

func axString(e objc.ID, sel string) string {
	return darwinffi.GoString(uintptr(e.Send(darwinffi.Sel(sel))))
}

func axBool(e objc.ID, sel string) bool {
	return objc.Send[bool](e, darwinffi.Sel(sel))
}

// axChildren returns the accessibilityChildren of e.
func axChildren(e objc.ID) []objc.ID {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	arr := e.Send(darwinffi.Sel("accessibilityChildren"))
	n := objc.Send[uint](arr, darwinffi.Sel("count"))
	out := make([]objc.ID, n)
	for i := range out {
		out[i] = arr.Send(darwinffi.Sel("objectAtIndex:"), uint(i))
	}
	return out
}

func sameIDs(a, b []objc.ID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// a11yTestNodes is a small tree: a disabled button that holds a checked
// checkbox, and a group whose parent index is out of range.
var a11yTestNodes = []gui.A11yNode{
	{
		Role: gui.AccessRoleButton, State: gui.AccessStateDisabled,
		Label: "Save", Description: "Saves the file",
		ParentIdx: -1, X: 10, Y: 20, W: 30, H: 40,
	},
	{
		Role: gui.AccessRoleCheckbox, State: gui.AccessStateChecked,
		Label: "Wrap", Value: "ignored", ParentIdx: 0,
	},
	{Role: gui.AccessRoleGroup, Label: "Orphan", ParentIdx: 99},
}

func TestA11yElementsAnswerFromNodes(t *testing.T) {
	ctx := a11yTestTree(t, 101)
	a11ySyncBridge(101, a11yTestNodes, len(a11yTestNodes), 1, 600)

	root := ctx.root.id
	save, wrap, orphan := ctx.elems[0].id, ctx.elems[1].id, ctx.elems[2].id

	// Tree shape. The out-of-range parent goes to the root, and the
	// root lists it as a child.
	if got := axChildren(root); !sameIDs(got, []objc.ID{save, orphan}) {
		t.Errorf("root children = %v, want [save orphan]", got)
	}
	if got := axChildren(save); !sameIDs(got, []objc.ID{wrap}) {
		t.Errorf("save children = %v, want [wrap]", got)
	}
	if got := axChildren(wrap); len(got) != 0 {
		t.Errorf("wrap children = %v, want none", got)
	}
	if p := wrap.Send(darwinffi.Sel("accessibilityParent")); p != save {
		t.Errorf("wrap parent = %v, want save", p)
	}
	if p := orphan.Send(darwinffi.Sel("accessibilityParent")); p != root {
		t.Errorf("orphan parent = %v, want root", p)
	}

	// Role, texts and value.
	roles := a11ySymbols().roles
	if r := save.Send(darwinffi.Sel("accessibilityRole")); r != roles[gui.AccessRoleButton] {
		t.Errorf("save role = %q", darwinffi.GoString(uintptr(r)))
	}
	if r := root.Send(darwinffi.Sel("accessibilityRole")); r != roles[gui.AccessRoleGroup] {
		t.Errorf("root role = %q", darwinffi.GoString(uintptr(r)))
	}
	if got := axString(root, "accessibilityLabel"); got != "Application" {
		t.Errorf("root label = %q", got)
	}
	if got := axString(save, "accessibilityLabel"); got != "Save" {
		t.Errorf("save label = %q", got)
	}
	if got := axString(save, "accessibilityHelp"); got != "Saves the file" {
		t.Errorf("save help = %q", got)
	}
	if got := axString(wrap, "accessibilityValue"); got != "1" {
		t.Errorf("checked checkbox value = %q, want \"1\"", got)
	}

	// The frame: a nil view is at (0, 0), so only the flip applies.
	// y = 600 - 20 - 40.
	frame := objc.Send[a11yRect](save, darwinffi.Sel("accessibilityFrame"))
	if want := (a11yRect{X: 10, Y: 540, W: 30, H: 40}); frame != want {
		t.Errorf("save frame = %+v, want %+v", frame, want)
	}

	// States.
	if axBool(save, "isAccessibilityEnabled") {
		t.Error("a disabled button reports enabled")
	}
	if !axBool(wrap, "isAccessibilityEnabled") {
		t.Error("the checkbox reports disabled")
	}
	if !axBool(save, "isAccessibilityElement") {
		t.Error("isAccessibilityElement = NO")
	}
	if !axBool(wrap, "isAccessibilityFocused") || axBool(save, "isAccessibilityFocused") {
		t.Error("focus is not on node 1 only")
	}
}

// The cgo root had node index 0, so it reported the focus of node 0 and
// sent its actions as actions of node 0.
func TestA11yRootIsNotNodeZero(t *testing.T) {
	const token = 102
	ctx := a11yTestTree(t, token)
	var got [][2]int
	setA11yCallback(token, func(action, index int) {
		got = append(got, [2]int{action, index})
	})
	t.Cleanup(func() { clearA11yCallback(token) })
	a11ySyncBridge(token, a11yTestNodes, len(a11yTestNodes), 0, 600)

	root := ctx.root.id
	if axBool(root, "isAccessibilityFocused") {
		t.Error("root reports the focus of node 0")
	}
	if axBool(root, "accessibilityPerformPress") {
		t.Error("root press returned YES")
	}
	if len(got) != 0 {
		t.Errorf("root press sent actions %v", got)
	}
}

func TestA11yActionsRouteToNode(t *testing.T) {
	const token = 103
	ctx := a11yTestTree(t, token)
	var got [][2]int
	setA11yCallback(token, func(action, index int) {
		got = append(got, [2]int{action, index})
	})
	t.Cleanup(func() { clearA11yCallback(token) })
	a11ySyncBridge(token, a11yTestNodes, len(a11yTestNodes), -1, 600)

	wrap := ctx.elems[1].id
	for _, sel := range []string{
		"accessibilityPerformPress",
		"accessibilityPerformIncrement",
		"accessibilityPerformDecrement",
		"accessibilityPerformConfirm",
		"accessibilityPerformCancel",
	} {
		if !axBool(wrap, sel) {
			t.Errorf("%s returned NO", sel)
		}
	}
	want := [][2]int{
		{gui.A11yActionPress, 1},
		{gui.A11yActionIncrement, 1},
		{gui.A11yActionDecrement, 1},
		{gui.A11yActionConfirm, 1},
		{gui.A11yActionCancel, 1},
	}
	if len(got) != len(want) {
		t.Fatalf("actions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("action %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// CFStringCreateWithCString returns NULL for text that is not valid
// UTF-8. The label must keep its valid text.
func TestA11yInvalidUTF8Label(t *testing.T) {
	ctx := a11yTestTree(t, 104)
	nodes := []gui.A11yNode{{Role: gui.AccessRoleButton, Label: "a\xffb", ParentIdx: -1}}
	a11ySyncBridge(104, nodes, 1, -1, 600)
	if got := axString(ctx.elems[0].id, "accessibilityLabel"); got != "a�b" {
		t.Errorf("label = %q, want %q", got, "a�b")
	}
}

// The cgo a11yAnnounce put a nil NSString in a dictionary literal for
// text that is not valid UTF-8, and NSInvalidArgumentException stopped
// the process. This test stops the test binary on that code.
func TestA11yAnnounceInvalidUTF8(t *testing.T) {
	a11yAnnounce("bad \xff text")
	a11yAnnounce("")
}

// A sync that keeps a text keeps its CFString, so a steady tree makes
// no new strings.
func TestA11yTextKeptWhenUnchanged(t *testing.T) {
	ctx := a11yTestTree(t, 105)
	nodes := []gui.A11yNode{{Label: "Same", ParentIdx: -1}}
	a11ySyncBridge(105, nodes, 1, -1, 600)
	first := ctx.elems[0].label.ns
	a11ySyncBridge(105, nodes, 1, -1, 600)
	if ctx.elems[0].label.ns != first {
		t.Error("an unchanged label made a new CFString")
	}
	nodes[0].Label = "Other"
	a11ySyncBridge(105, nodes, 1, -1, 600)
	if got := axString(ctx.elems[0].id, "accessibilityLabel"); got != "Other" {
		t.Errorf("changed label = %q", got)
	}
}

func TestA11ySyncShrinkAndClear(t *testing.T) {
	ctx := a11yTestTree(t, 106)
	a11ySyncBridge(106, a11yTestNodes, len(a11yTestNodes), -1, 600)
	orphan := ctx.elems[2].id

	// One node left: the pool keeps node 2, but it is out of the tree.
	a11ySyncBridge(106, a11yTestNodes[:1], 1, -1, 600)
	if got := axChildren(ctx.root.id); !sameIDs(got, []objc.ID{ctx.elems[0].id}) {
		t.Errorf("root children = %v, want [save]", got)
	}
	if got := axChildren(ctx.elems[0].id); len(got) != 0 {
		t.Errorf("save keeps children %v after they left", got)
	}
	if got := orphan.Send(darwinffi.Sel("accessibilityLabel")); got != 0 {
		t.Errorf("a node out of the tree still has a label")
	}

	// No nodes: the root is empty.
	a11ySyncBridge(106, nil, 0, -1, 600)
	if got := axChildren(ctx.root.id); len(got) != 0 {
		t.Errorf("root children = %v after clear", got)
	}
}

func TestA11yDestroyForgetsElements(t *testing.T) {
	a11yInitView(107, 0, 0)
	a11ySyncBridge(107, a11yTestNodes, len(a11yTestNodes), -1, 600)
	save := a11yCtxs[107].elems[0].id
	// AppKit can keep an element after the tree goes. Keep one here, so
	// that it is still an object after destroy.
	save.Send(darwinffi.Sel("retain"))
	defer save.Send(darwinffi.Sel("release"))

	a11yDestroy(107)
	if a11yCtxs[107] != nil {
		t.Error("the tree is still registered")
	}
	if _, ok := a11yRefs[save]; ok {
		t.Error("the element is still registered")
	}
	if got := save.Send(darwinffi.Sel("accessibilityLabel")); got != 0 {
		t.Error("an element of a removed tree still has a label")
	}
	if got := axChildren(save); len(got) != 0 {
		t.Errorf("an element of a removed tree has children %v", got)
	}
	// A second destroy, and a sync to the removed tree, do nothing.
	a11yDestroy(107)
	a11ySyncBridge(107, a11yTestNodes, len(a11yTestNodes), -1, 600)
}

func TestA11yScreenRect(t *testing.T) {
	// The view is at (100, 200) on the screen and is 600 high. A gui
	// rect at the top-left corner goes to the top of the view.
	got := a11yScreenRect(100, 200, 0, 0, 50, 20, 600)
	if want := (a11yRect{X: 100, Y: 780, W: 50, H: 20}); got != want {
		t.Errorf("top-left rect = %+v, want %+v", got, want)
	}
	got = a11yScreenRect(100, 200, 5, 580, 10, 20, 600)
	if want := (a11yRect{X: 105, Y: 200, W: 10, H: 20}); got != want {
		t.Errorf("bottom rect = %+v, want %+v", got, want)
	}
}

// Focus that leaves the tree and comes back to the same node is a
// change, and VoiceOver must hear of it. The cgo code kept the last
// posted node through the time with no focus, so the return was not
// posted.
func TestA11yFocusPostedAfterNoFocus(t *testing.T) {
	ctx := a11yTestTree(t, 108)
	var posts []objc.ID
	orig := a11yPostFocus
	a11yPostFocus = func(e objc.ID) { posts = append(posts, e) }
	t.Cleanup(func() { a11yPostFocus = orig })

	n := len(a11yTestNodes)
	for _, focus := range []int{1, 1, -1, 1, 99, 1, 0} {
		a11ySyncBridge(108, a11yTestNodes, n, focus, 600)
	}
	wrap, save := ctx.elems[1].id, ctx.elems[0].id
	// 1 posts; 1 again does not; -1 and 99 are no focus; each return to
	// 1 posts; 0 posts.
	want := []objc.ID{wrap, wrap, wrap, save}
	if !sameIDs(posts, want) {
		t.Errorf("focus posts = %v, want %v", posts, want)
	}
}

// A parent index that is the node itself, or a later node, would make a
// cycle that no walk from the root reaches and that AppKit follows for
// ever. Only an earlier node (the tree is in pre-order) is a valid
// parent; everything else attaches to the root.
func TestA11yParentCycleGoesToRoot(t *testing.T) {
	ctx := a11yTestTree(t, 102)
	nodes := []gui.A11yNode{
		{Role: gui.AccessRoleGroup, Label: "Self", ParentIdx: 0},
		{Role: gui.AccessRoleGroup, Label: "Forward", ParentIdx: 2},
		{Role: gui.AccessRoleGroup, Label: "Back", ParentIdx: 1},
		{Role: gui.AccessRoleButton, Label: "Ok", ParentIdx: -1},
	}
	a11ySyncBridge(102, nodes, len(nodes), -1, 600)

	root := ctx.root.id
	self, fwd, back, ok := ctx.elems[0].id, ctx.elems[1].id, ctx.elems[2].id, ctx.elems[3].id

	// Self and Forward are not valid parents of themselves: both go to
	// the root. Back names the earlier node Forward, which is valid.
	if got := axChildren(root); !sameIDs(got, []objc.ID{self, fwd, ok}) {
		t.Errorf("root children = %v, want [self forward ok]", got)
	}
	if got := axChildren(self); len(got) != 0 {
		t.Errorf("self children = %v, want none", got)
	}
	if got := axChildren(fwd); !sameIDs(got, []objc.ID{back}) {
		t.Errorf("forward children = %v, want [back]", got)
	}
	for _, e := range []objc.ID{self, fwd} {
		if p := e.Send(darwinffi.Sel("accessibilityParent")); p != root {
			t.Errorf("parent of %v = %v, want root", e, p)
		}
	}
}
