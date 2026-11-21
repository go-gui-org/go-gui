//go:build darwin && arm64 && !ios

package metal

// The accessibility tree goes through purego (#1317).
//
// Go owns the whole tree. Each node has one GoGuiA11yElement, a
// subclass of NSAccessibilityElement with no state of its own: its
// methods find their node through a11yRefs and answer from the Go copy.
// The sync is throttled and runs only when the tree changes (#407), so
// the objc.Send form, which allocates per call, is acceptable here.
//
// All the state below is main-thread only, as in the cgo code: AppKit
// calls the element methods on the main thread, and the backend calls
// init, sync and destroy from the main loop. No lock guards it, because
// AppKit can call an element method while a sync posts a notification,
// and a held lock would then deadlock.

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// a11yCallbacks routes VoiceOver actions to the owning window: one
// live tree per window, so one callback per window. Guarded by
// a11yCbMu: the frame thread sets the callbacks, and the element
// methods read them on the main thread.
var (
	a11yCbMu      sync.Mutex
	a11yCallbacks = make(map[uintptr]func(action, index int))
)

func setA11yCallback(token uintptr, cb func(action, index int)) {
	a11yCbMu.Lock()
	defer a11yCbMu.Unlock()
	if cb == nil {
		delete(a11yCallbacks, token)
		return
	}
	a11yCallbacks[token] = cb
}

func clearA11yCallback(token uintptr) {
	a11yCbMu.Lock()
	defer a11yCbMu.Unlock()
	delete(a11yCallbacks, token)
}

// goA11yAction sends one VoiceOver action to the callback of the
// window that token names. The lock is released before the call, so
// the callback can register or clear callbacks itself.
func goA11yAction(action, index int, token uintptr) {
	a11yCbMu.Lock()
	cb := a11yCallbacks[token]
	a11yCbMu.Unlock()
	if cb != nil {
		cb(action, index)
	}
}

// maxA11yNodes caps the accessibility node count, so a buggy or
// hostile caller cannot make an unbounded number of elements.
const maxA11yNodes = 50000

// a11yRect is NSRect: four CGFloats. It goes in four float registers,
// as an argument and as a return value.
type a11yRect struct {
	X, Y, W, H float64
}

// nsAccessibilityPriorityHigh is NSAccessibilityPriorityHigh.
const nsAccessibilityPriorityHigh = 90

// a11yRoleNames gives the AppKit role constant for each gui.AccessRole,
// in the same mapping as the cgo code. The entries are symbol names in
// AppKit, read once by a11ySymbols.
var a11yRoleNames = [...]string{
	gui.AccessRoleNone:         "NSAccessibilityUnknownRole",
	gui.AccessRoleButton:       "NSAccessibilityButtonRole",
	gui.AccessRoleCheckbox:     "NSAccessibilityCheckBoxRole",
	gui.AccessRoleColorWell:    "NSAccessibilityColorWellRole",
	gui.AccessRoleComboBox:     "NSAccessibilityComboBoxRole",
	gui.AccessRoleDateField:    "NSAccessibilityTextFieldRole",
	gui.AccessRoleDialog:       "NSAccessibilitySheetRole",
	gui.AccessRoleDisclosure:   "NSAccessibilityDisclosureTriangleRole",
	gui.AccessRoleGrid:         "NSAccessibilityTableRole",
	gui.AccessRoleGridCell:     "NSAccessibilityCellRole",
	gui.AccessRoleGroup:        "NSAccessibilityGroupRole",
	gui.AccessRoleHeading:      "NSAccessibilityStaticTextRole",
	gui.AccessRoleImage:        "NSAccessibilityImageRole",
	gui.AccessRoleLink:         "NSAccessibilityLinkRole",
	gui.AccessRoleList:         "NSAccessibilityListRole",
	gui.AccessRoleListItem:     "NSAccessibilityGroupRole",
	gui.AccessRoleMenu:         "NSAccessibilityMenuRole",
	gui.AccessRoleMenuBar:      "NSAccessibilityMenuBarRole",
	gui.AccessRoleMenuItem:     "NSAccessibilityMenuItemRole",
	gui.AccessRoleProgressBar:  "NSAccessibilityProgressIndicatorRole",
	gui.AccessRoleRadioButton:  "NSAccessibilityRadioButtonRole",
	gui.AccessRoleRadioGroup:   "NSAccessibilityRadioGroupRole",
	gui.AccessRoleScrollArea:   "NSAccessibilityScrollAreaRole",
	gui.AccessRoleScrollBar:    "NSAccessibilityScrollBarRole",
	gui.AccessRoleSlider:       "NSAccessibilitySliderRole",
	gui.AccessRoleSplitter:     "NSAccessibilitySplitGroupRole",
	gui.AccessRoleStaticText:   "NSAccessibilityStaticTextRole",
	gui.AccessRoleSwitchToggle: "NSAccessibilityCheckBoxRole",
	gui.AccessRoleTab:          "NSAccessibilityTabGroupRole",
	gui.AccessRoleTabItem:      "NSAccessibilityRadioButtonRole",
	gui.AccessRoleTextField:    "NSAccessibilityTextFieldRole",
	gui.AccessRoleTextArea:     "NSAccessibilityTextAreaRole",
	gui.AccessRoleToolbar:      "NSAccessibilityToolbarRole",
	gui.AccessRoleTree:         "NSAccessibilityOutlineRole",
	gui.AccessRoleTreeItem:     "NSAccessibilityRowRole",
}

// The build fails when gui adds a role that the table above does not
// give: the array index is then out of range.
var _ = [1]struct{}{}[len(a11yRoleNames)-int(gui.AccessRoleTreeItem)-1]

// a11ySyms holds the AppKit symbols. They are read on first use, not
// at init, so an app that never starts the accessibility tree never
// reads them.
type a11ySyms struct {
	roles [len(a11yRoleNames)]objc.ID

	postNotification     func(element, name uintptr)
	postNotificationInfo func(element, name, info uintptr)

	focusChanged      objc.ID // NSAccessibilityFocusedUIElementChangedNotification
	announceRequested objc.ID // NSAccessibilityAnnouncementRequestedNotification
	announceKey       objc.ID // NSAccessibilityAnnouncementKey
	priorityKey       objc.ID // NSAccessibilityPriorityKey

	// The values of a checkbox: "1" and "0". +1 CFStrings that live for
	// the life of the process.
	one, zero objc.ID
}

var a11ySymbols = sync.OnceValue(func() *a11ySyms {
	appkit := darwinffi.Open(darwinffi.PathAppKit)
	s := &a11ySyms{}
	for i, name := range a11yRoleNames {
		s.roles[i] = objc.ID(darwinffi.Data(appkit, name))
	}
	purego.RegisterLibFunc(&s.postNotification, appkit, "NSAccessibilityPostNotification")
	purego.RegisterLibFunc(&s.postNotificationInfo, appkit, "NSAccessibilityPostNotificationWithUserInfo")
	s.focusChanged = objc.ID(darwinffi.Data(appkit, "NSAccessibilityFocusedUIElementChangedNotification"))
	s.announceRequested = objc.ID(darwinffi.Data(appkit, "NSAccessibilityAnnouncementRequestedNotification"))
	s.announceKey = objc.ID(darwinffi.Data(appkit, "NSAccessibilityAnnouncementKey"))
	s.priorityKey = objc.ID(darwinffi.Data(appkit, "NSAccessibilityPriorityKey"))
	s.one = objc.ID(darwinffi.CFString("1"))
	s.zero = objc.ID(darwinffi.CFString("0"))
	return s
})

// a11yClass registers GoGuiA11yElement once. A class can be registered
// only once per process, so tests and a second window share it.
//
// Without cgo nothing links AppKit at load time, so loadObjc opens it
// first. Before that, NSAccessibilityElement is not a class, the
// superclass is nil, and the first send to an element stops the app.
var a11yClass = sync.OnceValue(func() objc.Class {
	loadObjc()
	cls, err := objc.RegisterClass("GoGuiA11yElement",
		objc.GetClass("NSAccessibilityElement"), nil, nil, a11yElementMethods())
	if err != nil {
		panic("metal: " + err.Error())
	}
	return cls
})

// a11yText is one string of a node: the Go text and the CFString made
// from it. ns is +1 and belongs to the node. NSString is toll-free
// bridged to CFString, so ns is a valid NSString return value.
type a11yText struct {
	s  string
	ns objc.ID
}

// set makes ns hold s. A text that did not change since the last sync
// keeps its CFString, so a steady tree makes no new strings.
//
// Text that is not valid UTF-8 has each bad byte replaced by U+FFFD.
// CFStringCreateWithCString returns NULL for such text, and the cgo
// code then gave VoiceOver no label at all. As with C.CString, the text
// ends at its first NUL byte.
func (t *a11yText) set(s string) {
	if t.ns != 0 && t.s == s {
		return
	}
	t.release()
	t.s = s
	t.ns = objc.ID(darwinffi.CFString(strings.ToValidUTF8(s, "�")))
}

func (t *a11yText) release() {
	darwinffi.CFRelease(uintptr(t.ns))
	t.ns = 0
}

// a11yElem is the Go copy of one node, plus its element.
type a11yElem struct {
	id    objc.ID // GoGuiA11yElement, +1, owned by the context
	role  gui.AccessRole
	state gui.AccessState
	label a11yText
	value a11yText
	help  a11yText
	frame a11yRect // screen coordinates, bottom-left origin

	// parent is the index of the parent node, or -1 for the root.
	parent   int
	children []int
}

func (e *a11yElem) releaseTexts() {
	e.label.release()
	e.value.release()
	e.help.release()
}

// a11yCtx is the tree of one window.
type a11yCtx struct {
	token  uintptr // the window handle, passed back with actions
	window objc.ID // NSWindow
	view   objc.ID // its content view, the parent of the root

	root a11yElem
	// elems is a pool of nodes. It grows and does not shrink, so the
	// elements stay the same objects from one sync to the next. Only
	// elems[:active] are in the tree.
	elems  []a11yElem
	active int

	focused     int // the focused node for isAccessibilityFocused, or -1
	prevFocused int // the focus last posted to VoiceOver, or -1
}

// a11yRef finds the node of an element: idx -1 is the root.
type a11yRef struct {
	ctx *a11yCtx
	idx int
}

var (
	a11yCtxs = map[uintptr]*a11yCtx{}
	a11yRefs = map[objc.ID]a11yRef{}

	// a11yIDBuf holds the elements for one accessibilityChildren reply.
	// It is in the heap, as arrayWithObjects:count: needs.
	a11yIDBuf []objc.ID
)

// a11yLookup returns the node of the element self. It returns a nil
// node for an element whose window is gone or whose node is not in the
// tree now: AppKit can keep an element after the tree drops it.
func a11yLookup(self objc.ID) (*a11yCtx, *a11yElem, int) {
	ref, ok := a11yRefs[self]
	if !ok {
		return nil, nil, 0
	}
	if ref.idx < 0 {
		return ref.ctx, &ref.ctx.root, -1
	}
	if ref.idx >= ref.ctx.active {
		return nil, nil, 0
	}
	return ref.ctx, &ref.ctx.elems[ref.idx], ref.idx
}

// a11yNewElem makes the element for node idx of ctx.
func a11yNewElem(ctx *a11yCtx, idx int) objc.ID {
	id := objc.ID(a11yClass()).Send(darwinffi.Sel("alloc")).Send(darwinffi.Sel("init"))
	a11yRefs[id] = a11yRef{ctx: ctx, idx: idx}
	return id
}

// a11yElementMethods are the methods of GoGuiA11yElement. Each one is
// what the cgo GUIAccessibilityElement answered from its properties.
func a11yElementMethods() []objc.MethodDef {
	flag := func(f gui.AccessState) func(objc.ID, objc.SEL) bool {
		return func(self objc.ID, _ objc.SEL) bool {
			_, e, _ := a11yLookup(self)
			return e != nil && e.state&f != 0
		}
	}
	text := func(pick func(*a11yElem) objc.ID) func(objc.ID, objc.SEL) objc.ID {
		return func(self objc.ID, _ objc.SEL) objc.ID {
			if _, e, _ := a11yLookup(self); e != nil {
				return pick(e)
			}
			return 0
		}
	}
	// perform sends one action. The root is not a node of the app's
	// tree, so it has no action. The cgo root sent its actions for node
	// 0, because its index was never set.
	perform := func(action int) func(objc.ID, objc.SEL) bool {
		return func(self objc.ID, _ objc.SEL) bool {
			ctx, e, idx := a11yLookup(self)
			if e == nil || idx < 0 {
				return false
			}
			goA11yAction(action, idx, ctx.token)
			return true
		}
	}
	sel := darwinffi.Sel
	return []objc.MethodDef{
		{Cmd: sel("accessibilityRole"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			s := a11ySymbols()
			if _, e, _ := a11yLookup(self); e != nil && int(e.role) < len(s.roles) {
				return s.roles[e.role]
			}
			return s.roles[gui.AccessRoleNone]
		}},
		{Cmd: sel("accessibilityLabel"), Fn: text(func(e *a11yElem) objc.ID { return e.label.ns })},
		{Cmd: sel("accessibilityValue"), Fn: text(func(e *a11yElem) objc.ID {
			// A checkbox or a toggle gives its checked state as "1" or "0".
			if e.role == gui.AccessRoleCheckbox || e.role == gui.AccessRoleSwitchToggle {
				if e.state&gui.AccessStateChecked != 0 {
					return a11ySymbols().one
				}
				return a11ySymbols().zero
			}
			return e.value.ns
		})},
		{Cmd: sel("accessibilityHelp"), Fn: text(func(e *a11yElem) objc.ID { return e.help.ns })},
		{Cmd: sel("accessibilityFrame"), Fn: func(self objc.ID, _ objc.SEL) a11yRect {
			if _, e, _ := a11yLookup(self); e != nil {
				return e.frame
			}
			return a11yRect{}
		}},
		{Cmd: sel("accessibilityParent"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			ctx, e, idx := a11yLookup(self)
			switch {
			case e == nil:
				return 0
			case idx < 0:
				return ctx.view
			case e.parent < 0:
				return ctx.root.id
			default:
				return ctx.elems[e.parent].id
			}
		}},
		{Cmd: sel("accessibilityChildren"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			ctx, e, _ := a11yLookup(self)
			if e == nil {
				return darwinffi.Class("NSArray").Send(sel("array"))
			}
			return a11yArray(ctx, e.children)
		}},
		{Cmd: sel("isAccessibilityElement"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("isAccessibilityEnabled"), Fn: func(self objc.ID, _ objc.SEL) bool {
			_, e, _ := a11yLookup(self)
			return e != nil && e.state&(gui.AccessStateReadOnly|gui.AccessStateDisabled) == 0
		}},
		{Cmd: sel("isAccessibilityExpanded"), Fn: flag(gui.AccessStateExpanded)},
		{Cmd: sel("isAccessibilitySelected"), Fn: flag(gui.AccessStateSelected)},
		{Cmd: sel("isAccessibilityRequired"), Fn: flag(gui.AccessStateRequired)},
		{Cmd: sel("isAccessibilityModal"), Fn: flag(gui.AccessStateModal)},
		{Cmd: sel("isAccessibilityFocused"), Fn: func(self objc.ID, _ objc.SEL) bool {
			// The root is never focused. The cgo root had index 0, so it
			// reported focus whenever node 0 had it.
			ctx, e, idx := a11yLookup(self)
			return e != nil && idx >= 0 && idx == ctx.focused
		}},
		{Cmd: sel("accessibilityPerformPress"), Fn: perform(gui.A11yActionPress)},
		{Cmd: sel("accessibilityPerformIncrement"), Fn: perform(gui.A11yActionIncrement)},
		{Cmd: sel("accessibilityPerformDecrement"), Fn: perform(gui.A11yActionDecrement)},
		{Cmd: sel("accessibilityPerformConfirm"), Fn: perform(gui.A11yActionConfirm)},
		{Cmd: sel("accessibilityPerformCancel"), Fn: perform(gui.A11yActionCancel)},
	}
}

// a11yArray returns an autoreleased NSArray of the elements of the
// nodes idxs. AppKit calls the element methods inside its own
// autorelease pool.
func a11yArray(ctx *a11yCtx, idxs []int) objc.ID {
	if len(idxs) == 0 {
		return darwinffi.Class("NSArray").Send(darwinffi.Sel("array"))
	}
	a11yIDBuf = a11yIDBuf[:0]
	for _, i := range idxs {
		a11yIDBuf = append(a11yIDBuf, ctx.elems[i].id)
	}
	return darwinffi.Class("NSArray").Send(darwinffi.Sel("arrayWithObjects:count:"),
		unsafe.Pointer(&a11yIDBuf[0]), uint(len(a11yIDBuf)))
}

// a11yInit starts the tree of one window. Each window owns its tree:
// a second window does not change the tree of the first. window is the
// NSWindow, or 0 when the handle has none.
func a11yInit(token uintptr, window objc.ID) {
	if window == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	view := window.Send(darwinffi.Sel("contentView"))
	if view == 0 {
		return
	}
	a11yInitView(token, window, view)
}

// a11yInitView is a11yInit after the content view is known. A test
// calls it with no window and no view: the sends to them are then sends
// to nil, which do nothing.
func a11yInitView(token uintptr, window, view objc.ID) {
	ctx := a11yCtxs[token]
	if ctx == nil {
		ctx = &a11yCtx{token: token, focused: -1}
		ctx.root.id = a11yNewElem(ctx, -1)
		ctx.root.role = gui.AccessRoleGroup
		ctx.root.parent = -1
		ctx.root.label.set("Application")
		a11yCtxs[token] = ctx
	}
	ctx.window, ctx.view = window, view
	ctx.active = 0
	ctx.root.children = ctx.root.children[:0]
	ctx.prevFocused = -1

	// The root is the one accessibility child of the content view.
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	children := darwinffi.Class("NSArray").Send(darwinffi.Sel("arrayWithObject:"), ctx.root.id)
	view.Send(darwinffi.Sel("setAccessibilityChildren:"), children)
}

// a11ySyncBridge makes the tree of one window match nodes.
func a11ySyncBridge(token uintptr, nodes []gui.A11yNode, count, focusedIdx int, windowH float32) {
	// Clamp first: a count past the slice end reads out of bounds, and
	// a count clamped to zero must take the clearing path below.
	count = min(count, len(nodes), maxA11yNodes)
	ctx := a11yCtxs[token]
	if ctx == nil {
		return
	}
	if count <= 0 {
		// An emptied tree clears rather than lingers: VoiceOver then
		// sees nothing, not the previous content.
		ctx.root.children = ctx.root.children[:0]
		ctx.active = 0
		ctx.focused = -1
		ctx.prevFocused = -1
		return
	}

	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	for len(ctx.elems) < count {
		ctx.elems = append(ctx.elems, a11yElem{id: a11yNewElem(ctx, len(ctx.elems))})
	}
	ctx.root.children = ctx.root.children[:0]
	for i := range count {
		ctx.elems[i].children = ctx.elems[i].children[:0]
	}

	ox, oy := a11yScreenOrigin(ctx)
	for i := range count {
		n := &nodes[i]
		e := &ctx.elems[i]
		e.role = n.Role
		e.state = n.State
		e.label.set(n.Label)
		e.value.set(n.Value)
		e.help.set(n.Description)
		e.frame = a11yScreenRect(ox, oy, n.X, n.Y, n.W, n.H, windowH)
		// The tree is in pre-order, so a valid parent is an earlier node.
		// A parent that is out of range, the node itself or a later node
		// attaches the node to the root. A later or self parent would make
		// a cycle that AppKit walks for ever, and bad input cannot grow
		// the pool.
		if p := n.ParentIdx; p >= 0 && p < i {
			e.parent = p
			ctx.elems[p].children = append(ctx.elems[p].children, i)
		} else {
			e.parent = -1
			ctx.root.children = append(ctx.root.children, i)
		}
	}
	ctx.active = count

	// The root covers the whole view.
	bounds := objc.Send[a11yRect](ctx.view, darwinffi.Sel("bounds"))
	ctx.root.frame = a11yScreenRect(ox, oy, 0, 0, float32(bounds.W), float32(bounds.H), windowH)

	ctx.focused = focusedIdx
	// prevFocused follows every sync, also a sync with no focus or a
	// focus out of range. The cgo code changed it only on a post, so a
	// focus that left and came back to the same node was not posted.
	changed := focusedIdx != ctx.prevFocused
	ctx.prevFocused = focusedIdx
	if changed && focusedIdx >= 0 && focusedIdx < count {
		a11yPostFocus(ctx.elems[focusedIdx].id)
	}
}

// a11yPostFocus tells VoiceOver that element e has the focus now. A
// test replaces it to count the posts.
var a11yPostFocus = func(e objc.ID) {
	s := a11ySymbols()
	s.postNotification(uintptr(e), uintptr(s.focusChanged))
}

// a11yScreenOrigin returns where the bottom-left corner of the content
// view is on the screen. The content view is not flipped, scaled or
// turned, so a rect in it goes to the screen by this one offset. Two
// sends per sync replace the two sends per node of the cgo code.
func a11yScreenOrigin(ctx *a11yCtx) (x, y float64) {
	r := objc.Send[a11yRect](ctx.view, darwinffi.Sel("convertRect:toView:"), a11yRect{}, objc.ID(0))
	r = objc.Send[a11yRect](ctx.window, darwinffi.Sel("convertRectToScreen:"), r)
	return r.X, r.Y
}

// a11yScreenRect turns a gui rect (top-left origin, in the view) into
// a screen rect (bottom-left origin). (ox, oy) is the screen position
// of the bottom-left corner of the view, and windowH its height.
func a11yScreenRect(ox, oy float64, x, y, w, h, windowH float32) a11yRect {
	return a11yRect{
		X: ox + float64(x),
		Y: oy + float64(windowH-y-h),
		W: float64(w),
		H: float64(h),
	}
}

// a11yDestroy removes the tree of one window and releases its elements.
func a11yDestroy(token uintptr) {
	ctx := a11yCtxs[token]
	if ctx == nil {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	ctx.view.Send(darwinffi.Sel("setAccessibilityChildren:"), objc.ID(0))

	// AppKit can keep an element after this. Its methods then find no
	// node and answer with zero values.
	release := func(e *a11yElem) {
		e.releaseTexts()
		delete(a11yRefs, e.id)
		e.id.Send(darwinffi.Sel("release"))
	}
	release(&ctx.root)
	for i := range ctx.elems {
		release(&ctx.elems[i])
	}
	delete(a11yCtxs, token)
}

// a11yAnnounce asks VoiceOver to speak text, at high priority.
//
// Text that is not valid UTF-8 has each bad byte replaced by U+FFFD.
// The cgo code made a nil NSString from such text, and the dictionary
// literal that held it threw NSInvalidArgumentException, which stopped
// the app.
func a11yAnnounce(text string) {
	s := a11ySymbols()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	str := darwinffi.CFString(strings.ToValidUTF8(text, "�"))
	if str == 0 {
		return
	}
	defer darwinffi.CFRelease(str)
	info := darwinffi.CFMutableDict()
	defer darwinffi.CFRelease(info)
	darwinffi.CFDictSet(info, uintptr(s.announceKey), str)
	priority := darwinffi.Class("NSNumber").Send(darwinffi.Sel("numberWithInteger:"), nsAccessibilityPriorityHigh)
	darwinffi.CFDictSet(info, uintptr(s.priorityKey), uintptr(priority))

	// NSApp is a global that +[NSApplication sharedApplication] sets.
	// Read it at each call: before the app exists it is nil.
	app := darwinffi.Data(darwinffi.Open(darwinffi.PathAppKit), "NSApp")
	s.postNotificationInfo(app, uintptr(s.announceRequested), info)
}
