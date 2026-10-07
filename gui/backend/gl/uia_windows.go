//go:build windows && !js && (amd64 || arm64)

package gl

import (
	"math"
	"sync"
	"sync/atomic"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

// Screen readers (Narrator, NVDA) read the window through UI
// Automation. The window answers WM_GETOBJECT with a fragment root
// whose fragments are the nodes of the last A11ySync: COM objects
// built in Go, whose vtables are syscall callbacks, with the control
// patterns of their roles (uia_map_windows.go).
//
// Adapted from mygo's provider (github.com/egoist/mygo,
// internal/windows/surface_access.go, MIT); see THIRD_PARTY_NOTICES.
// Two departures: mygo reports ProviderOptions_ClientSideProvider,
// where a provider answering WM_GETOBJECT is server side; and mygo
// relies on one UI thread, where this provider reports no COM
// threading, so UIA may call in from its own threads. The tree is
// therefore guarded by uiaProvider.mu, reference counts are atomic,
// and no UIA function is called with mu held: several call back into
// the provider before returning.
//
// Two methods take doubles, which Go callbacks cannot read: thunks
// in assembly move their bits to integer registers first (uia_*.s).

var (
	uiaCore  = windows.NewLazySystemDLL("uiautomationcore.dll")
	oleaut32 = windows.NewLazySystemDLL("oleaut32.dll")

	pUiaReturnRawElementProvider            = uiaCore.NewProc("UiaReturnRawElementProvider")
	pUiaHostProviderFromHwnd                = uiaCore.NewProc("UiaHostProviderFromHwnd")
	pUiaRaiseAutomationEvent                = uiaCore.NewProc("UiaRaiseAutomationEvent")
	pUiaRaiseAutomationPropertyChangedEvent = uiaCore.NewProc("UiaRaiseAutomationPropertyChangedEvent")
	pUiaRaiseStructureChangedEvent          = uiaCore.NewProc("UiaRaiseStructureChangedEvent")
	pUiaRaiseNotificationEvent              = uiaCore.NewProc("UiaRaiseNotificationEvent")
	pUiaClientsAreListening                 = uiaCore.NewProc("UiaClientsAreListening")
	pUiaDisconnectProvider                  = uiaCore.NewProc("UiaDisconnectProvider")

	pSysAllocStringLen     = oleaut32.NewProc("SysAllocStringLen")
	pSysFreeString         = oleaut32.NewProc("SysFreeString")
	pSafeArrayCreateVector = oleaut32.NewProc("SafeArrayCreateVector")
	pSafeArrayPutElement   = oleaut32.NewProc("SafeArrayPutElement")

	pClientToScreen = user32.NewProc("ClientToScreen")
	pNotifyWinEvent = user32.NewProc("NotifyWinEvent")
)

const (
	uiaRootObjectID = -25 // UiaRootObjectId
	objidClient     = -4  // OBJID_CLIENT
	// objidClientLong is OBJID_CLIENT as the LONG argument's register
	// value.
	objidClientLong = 0xFFFFFFFC

	eventObjectFocus = 0x8005 // EVENT_OBJECT_FOCUS

	providerOptionsServerSide = 0x2

	sOK                    = 0
	ePointer               = 0x80004003
	eNoInterface           = 0x80004002
	eInvalidArg            = 0x80070057
	uiaElementNotEnabled   = 0x80040200
	uiaElementNotAvailable = 0x80040201
	uiaInvalidOperation    = 0x80131509

	vtEmpty   = 0
	vtI4      = 3
	vtR8      = 5
	vtBSTR    = 8
	vtBool    = 11
	vtUnknown = 13

	// UIA_*PropertyId.
	uiaControlTypeProperty        = 30003
	uiaLocalizedControlTypeProp   = 30004
	uiaNameProperty               = 30005
	uiaHasKeyboardFocusProperty   = 30008
	uiaIsKeyboardFocusableProp    = 30009
	uiaIsEnabledProperty          = 30010
	uiaHelpTextProperty           = 30013
	uiaFrameworkIDProperty        = 30024
	uiaIsRequiredForFormProperty  = 30025
	uiaItemStatusProperty         = 30026
	uiaValueValueProperty         = 30045
	uiaRangeValueValueProperty    = 30047
	uiaExpandCollapseStateProp    = 30070
	uiaSelectionItemIsSelected    = 30079
	uiaToggleStateProperty        = 30086
	uiaIsDataValidForFormProperty = 30103
	uiaLiveSettingProperty        = 30135
	uiaFullDescriptionProperty    = 30159
	uiaIsDialogProperty           = 30174

	// UIA_*EventId.
	uiaFocusChangedEvent    = 20005
	uiaElementSelectedEvent = 20012

	structureChildrenInvalidated = 2

	notificationKindOther             = 4
	notificationCurrentThenMostRecent = 4
	liveSettingPolite                 = 1

	// maxUIAText caps a string handed to UIA, in UTF-16 units. Labels
	// come from app and document text (Markdown, RTF); a reader gains
	// nothing past this and each read copies the whole string.
	maxUIAText = 8192
)

// UIA control pattern IDs, by interface.
var uiaPatternIfaces = map[uintptr]int{
	10000: ifaceInvoke,
	10001: ifaceSelection,
	10002: ifaceValue,
	10003: ifaceRangeValue,
	10005: ifaceExpandCollapse,
	10010: ifaceSelectionItem,
	10015: ifaceToggle,
}

var (
	iidIUnknown = mustGUID("{00000000-0000-0000-C000-000000000046}")
	uiaIIDs     = [uiaIfaces]windows.GUID{
		ifaceSimple:         mustGUID("{d6dd68d1-86fd-4332-8666-9abedea2d24c}"),
		ifaceFragment:       mustGUID("{f7063da8-8359-439c-9297-bbc5299a7d87}"),
		ifaceRoot:           mustGUID("{620ce2a5-ab8f-40a9-86cb-de3c75599b58}"),
		ifaceInvoke:         mustGUID("{54fcb24b-e18e-47a2-b4d3-eccbe77599a2}"),
		ifaceToggle:         mustGUID("{56d00bd0-c4f4-433c-a836-1a52a57e0892}"),
		ifaceSelectionItem:  mustGUID("{2acad808-b2d4-452d-a407-91ff1ad167b2}"),
		ifaceSelection:      mustGUID("{fb8b03af-3bdf-48d4-bd36-1a65793be168}"),
		ifaceRangeValue:     mustGUID("{36dc7aef-33e6-4691-afe1-2be7274b3d33}"),
		ifaceValue:          mustGUID("{c7935180-6fb3-4201-b174-7df73adbf64a}"),
		ifaceExpandCollapse: mustGUID("{d847d3a5-cab0-4a98-8c32-ecb45c59ad24}"),
	}
)

func mustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString(s)
	if err != nil {
		panic(err)
	}
	return g
}

// The thunks, in assembly, called by UI Automation only; and the
// callbacks they go on to, with the doubles' bits as integers.
func uiaThunks() (fromPoint, setValue uintptr)
func uiaFromPointThunk() //nolint:unused // address taken in uia_*.s
func uiaSetValueThunk()  //nolint:unused // address taken in uia_*.s

var (
	uiaFromPointCallback uintptr //nolint:unused // read by uia_*.s
	uiaSetValueCallback  uintptr //nolint:unused // read by uia_*.s

	uiaOnce  sync.Once
	uiaVtbls [uiaIfaces][]uintptr

	// uiaLive keeps every element COM holds a reference to reachable:
	// COM keeps only a uintptr into the element.
	uiaLiveMu sync.Mutex
	uiaLive   = map[*uiaElement]struct{}{}

	uiaSerial atomic.Int32
)

// variant is VARIANT, with the members used here.
type variant struct {
	vt  uint16
	_   [3]uint16
	val uint64
	_   uintptr
}

// uiaRect is UiaRect.
type uiaRect struct{ left, top, width, height float64 }

// uiaIface is one interface pointer of an element: COM calls its
// methods with its address.
type uiaIface struct {
	vtbl *uintptr
	e    *uiaElement
}

// uiaElement is one node as UI Automation sees it, or the root.
type uiaElement struct {
	ifaces [uiaIfaces]uiaIface
	p      *uiaProvider
	refs   atomic.Int32
	dead   atomic.Bool
	serial int32 // runtime ID; never reused in the process
	idx    int   // node index, -1 for the root; guarded by p.mu
}

func (e *uiaElement) ptr(i int) uintptr { return uintptr(unsafe.Pointer(&e.ifaces[i])) }

func (e *uiaElement) addRef() int32 { return e.refs.Add(1) }

func (e *uiaElement) release() int32 {
	r := e.refs.Add(-1)
	if r == 0 {
		uiaLiveMu.Lock()
		delete(uiaLive, e)
		uiaLiveMu.Unlock()
	}
	return r
}

func uiaOf(this uintptr) *uiaElement { return (*uiaIface)(ptrFromLParam(this)).e }

// uiaProvider is what UI Automation sees of one window.
type uiaProvider struct {
	hwnd uintptr
	act  func(action, index int)
	root *uiaElement

	mu     sync.Mutex
	nodes  []gui.A11yNode
	elems  []*uiaElement
	links  uiaLinks
	focus  int
	scale  float32
	closed bool

	// Main thread only (Sync): the buffers swapped with nodes and
	// elems, and the events of one sync, raised once mu is released.
	spareNodes []gui.A11yNode
	spareElems []*uiaElement
	events     []uiaEvent
	dropped    []*uiaElement
}

func newUIAProvider(hwnd uintptr, act func(action, index int)) *uiaProvider {
	uiaOnce.Do(initUIAVtbls)
	p := &uiaProvider{hwnd: hwnd, act: act, focus: -1, scale: 1}
	p.root = p.newElement(-1)
	p.links.build(nil)
	return p
}

func (p *uiaProvider) newElement(idx int) *uiaElement {
	e := &uiaElement{p: p, idx: idx, serial: uiaSerial.Add(1)}
	for i := range e.ifaces {
		e.ifaces[i] = uiaIface{vtbl: &uiaVtbls[i][0], e: e}
	}
	e.refs.Store(1) // the provider's
	uiaLiveMu.Lock()
	uiaLive[e] = struct{}{}
	uiaLiveMu.Unlock()
	return e
}

// getObject answers WM_GETOBJECT. UIA asks for the root
// (UiaRootObjectId) and, with IDs from 0 up, for the elements of its
// events; OBJID_CLIENT gets UIA's MSAA view of the tree. Other MSAA
// objects (title bar, scroll bars) stay the window's.
func (p *uiaProvider) getObject(wparam, lparam uintptr) (uintptr, bool) {
	id := int32(uint32(lparam))
	if id < 0 && id != uiaRootObjectID && id != objidClient {
		return 0, false
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return 0, false
	}
	r, _, _ := pUiaReturnRawElementProvider.Call(p.hwnd, wparam, lparam, p.root.ptr(ifaceSimple))
	return r, true
}

// destroy disconnects every element; COM's remaining references then
// answer UIA_E_ELEMENTNOTAVAILABLE.
func (p *uiaProvider) destroy() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	all := make([]*uiaElement, 0, len(p.elems)+1)
	all = append(all, p.elems...)
	all = append(all, p.root)
	p.elems, p.nodes = nil, nil
	p.links.build(nil)
	for _, e := range all {
		e.dead.Store(true)
	}
	p.mu.Unlock()
	pUiaReturnRawElementProvider.Call(p.hwnd, 0, 0, 0)
	for _, e := range all {
		e.disconnect()
	}
}

func (e *uiaElement) disconnect() {
	if pUiaDisconnectProvider.Find() == nil {
		pUiaDisconnectProvider.Call(e.ptr(ifaceSimple))
	}
	e.release()
}

func uiaListening() bool {
	r, _, _ := pUiaClientsAreListening.Call()
	return r != 0
}

// uiaVal is one side of a property change.
type uiaVal struct {
	s  string
	f  float64
	i  int32
	vt uint16
}

func uiaBoolVal(b bool) uiaVal {
	if b {
		return uiaVal{vt: vtBool, i: 1}
	}
	return uiaVal{vt: vtBool}
}

// uiaEvent is one event of a sync, held until mu is released.
type uiaEvent struct {
	e        *uiaElement
	old, new uiaVal
	id       int32
	kind     uint8
}

const (
	uiaEvAutomation = iota
	uiaEvProperty
	uiaEvStructure
	uiaEvFocus
)

// sync shows UIA a new tree. An element is kept across syncs while its
// index, role and parent element stay, so a client's reference and
// runtime ID survive value, label and focus changes; anything else is
// a new element, and the old one is disconnected.
func (p *uiaProvider) sync(nodes []gui.A11yNode, focus int, scale float32) {
	notify := uiaListening()
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	old, oldElems := p.nodes, p.elems
	var oldFocus *uiaElement
	if p.focus >= 0 && p.focus < len(oldElems) {
		oldFocus = oldElems[p.focus]
	}
	cur := p.spareNodes[:0]
	cur = append(cur, nodes...)
	elems := p.spareElems[:0]
	structural := len(cur) != len(old)
	for i := range cur {
		n := &cur[i]
		var e *uiaElement
		if i < len(old) && p.reusable(&old[i], n, i, elems, oldElems) {
			e = oldElems[i]
			if notify {
				p.diff(e, &old[i], n)
			}
		} else {
			e = p.newElement(i)
			structural = true
			if i < len(oldElems) {
				p.dropped = append(p.dropped, oldElems[i])
			}
		}
		e.idx = i
		elems = append(elems, e)
	}
	for i := len(cur); i < len(oldElems); i++ {
		p.dropped = append(p.dropped, oldElems[i])
	}
	for _, e := range p.dropped {
		e.dead.Store(true)
	}
	clear(oldElems) // drop the spare's pointers
	p.spareNodes, p.spareElems = old[:0], oldElems[:0]
	p.nodes, p.elems = cur, elems
	p.links.build(cur)
	if focus < -1 || focus >= len(cur) {
		focus = -1
	}
	p.focus = focus
	if scale > 0 {
		p.scale = scale
	}
	if notify {
		if structural {
			p.queue(uiaEvent{e: p.root, kind: uiaEvStructure})
		}
		if focus >= 0 && elems[focus] != oldFocus {
			p.queue(uiaEvent{e: elems[focus], kind: uiaEvFocus})
		}
	}
	p.mu.Unlock()

	for i := range p.events {
		p.raise(&p.events[i])
		p.events[i].e.release()
	}
	clear(p.events)
	p.events = p.events[:0]
	for _, e := range p.dropped {
		e.disconnect()
	}
	clear(p.dropped)
	p.dropped = p.dropped[:0]
}

// reusable reports whether the element at index i still stands for
// the node there: same role, and the same parent element.
func (p *uiaProvider) reusable(o, n *gui.A11yNode, i int, elems, oldElems []*uiaElement) bool {
	if o.Role != n.Role || o.ParentIdx != n.ParentIdx {
		return false
	}
	pi := n.ParentIdx
	if pi < 0 || pi >= i {
		return true
	}
	return pi < len(oldElems) && elems[pi] == oldElems[pi]
}

// queue adds an event, holding a reference to its element until it
// is raised.
func (p *uiaProvider) queue(ev uiaEvent) {
	ev.e.addRef()
	p.events = append(p.events, ev)
}

// diff queues the property changes of a kept element.
func (p *uiaProvider) diff(e *uiaElement, o, n *gui.A11yNode) {
	prop := func(id int32, before, after uiaVal) {
		p.queue(uiaEvent{e: e, kind: uiaEvProperty, id: id, old: before, new: after})
	}
	if o.Label != n.Label {
		prop(uiaNameProperty, uiaVal{vt: vtBSTR, s: o.Label}, uiaVal{vt: vtBSTR, s: n.Label})
	}
	if od, nd := o.State.Has(gui.AccessStateDisabled), n.State.Has(gui.AccessStateDisabled); od != nd {
		prop(uiaIsEnabledProperty, uiaBoolVal(!od), uiaBoolVal(!nd))
	}
	r := n.Role
	if uiaSupports(r, ifaceToggle) {
		if before, after := uiaToggleState(o), uiaToggleState(n); before != after {
			prop(uiaToggleStateProperty, uiaVal{vt: vtI4, i: before}, uiaVal{vt: vtI4, i: after})
		}
	}
	if uiaSupports(r, ifaceSelectionItem) {
		if before, after := uiaIsSelected(o), uiaIsSelected(n); before != after {
			prop(uiaSelectionItemIsSelected, uiaBoolVal(before), uiaBoolVal(after))
			if after {
				p.queue(uiaEvent{e: e, kind: uiaEvAutomation, id: uiaElementSelectedEvent})
			}
		}
	}
	if uiaSupports(r, ifaceRangeValue) && o.ValueNum != n.ValueNum {
		prop(uiaRangeValueValueProperty,
			uiaVal{vt: vtR8, f: float64(o.ValueNum)}, uiaVal{vt: vtR8, f: float64(n.ValueNum)})
	}
	if uiaSupports(r, ifaceValue) && o.Value != n.Value {
		prop(uiaValueValueProperty, uiaVal{vt: vtBSTR, s: o.Value}, uiaVal{vt: vtBSTR, s: n.Value})
	}
	if uiaSupports(r, ifaceExpandCollapse) {
		if before, after := uiaExpandState(o), uiaExpandState(n); before != after {
			prop(uiaExpandCollapseStateProp, uiaVal{vt: vtI4, i: before}, uiaVal{vt: vtI4, i: after})
		}
	}
}

// raise hands one queued event to UIA. Called without mu.
func (p *uiaProvider) raise(ev *uiaEvent) {
	sp := ev.e.ptr(ifaceSimple)
	switch ev.kind {
	case uiaEvAutomation:
		pUiaRaiseAutomationEvent.Call(sp, uintptr(ev.id))
	case uiaEvFocus:
		pUiaRaiseAutomationEvent.Call(sp, uiaFocusChangedEvent)
		// Clients that follow focus through WinEvents (MSAA, and
		// System.Windows.Automation) then ask the window for it.
		pNotifyWinEvent.Call(eventObjectFocus, p.hwnd, objidClientLong, 0)
	case uiaEvStructure:
		// The root's runtime ID is its window's, so none is passed.
		pUiaRaiseStructureChangedEvent.Call(sp, structureChildrenInvalidated, 0, 0)
	case uiaEvProperty:
		before, after := ev.old.variant(), ev.new.variant()
		// VARIANTs go by value; both ABIs pass a struct this size as a
		// pointer to a copy.
		pUiaRaiseAutomationPropertyChangedEvent.Call(sp, uintptr(ev.id),
			uintptr(unsafe.Pointer(&before)), uintptr(unsafe.Pointer(&after)))
		freeVariant(&before)
		freeVariant(&after)
	}
}

func (v uiaVal) variant() variant {
	switch v.vt {
	case vtBSTR:
		return variant{vt: vtBSTR, val: uint64(uiaBSTR(v.s))}
	case vtBool:
		return boolVariant(v.i != 0)
	case vtR8:
		return variant{vt: vtR8, val: math.Float64bits(v.f)}
	case vtI4:
		return variant{vt: vtI4, val: uint64(uint32(v.i))}
	}
	return variant{}
}

func freeVariant(v *variant) {
	if v.vt == vtBSTR && v.val != 0 {
		pSysFreeString.Call(uintptr(v.val))
	}
	*v = variant{}
}

func boolVariant(b bool) variant {
	if b {
		return variant{vt: vtBool, val: 0xFFFF} // VARIANT_TRUE
	}
	return variant{vt: vtBool}
}

// uiaBSTR allocates a BSTR of s, capped at maxUIAText UTF-16 units.
// The caller, or the client it is handed to, frees it.
func uiaBSTR(s string) uintptr {
	u := make([]uint16, 0, min(len(s), maxUIAText))
	for _, r := range s {
		if len(u)+2 > maxUIAText {
			break
		}
		u = utf16.AppendRune(u, r)
	}
	var first *uint16
	if len(u) > 0 {
		first = &u[0]
	}
	b, _, _ := pSysAllocStringLen.Call(uintptr(unsafe.Pointer(first)), uintptr(len(u)))
	return b
}

// announce raises a notification Narrator and NVDA speak, on the root.
// UiaRaiseNotificationEvent arrived in Windows 10 1709; earlier
// systems say nothing.
func (p *uiaProvider) announce(text string) {
	if text == "" || pUiaRaiseNotificationEvent.Find() != nil || !uiaListening() {
		return
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return
	}
	msg, activity := uiaBSTR(text), uiaBSTR("go-gui.announce")
	pUiaRaiseNotificationEvent.Call(p.root.ptr(ifaceSimple),
		notificationKindOther, notificationCurrentThenMostRecent, msg, activity)
	pSysFreeString.Call(msg)
	pSysFreeString.Call(activity)
}

// node copies the element's node. ok is false for a dead element and
// for the root, whose properties are its window's.
func (p *uiaProvider) node(e *uiaElement) (n gui.A11yNode, idx int, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e.dead.Load() || e.idx < 0 || e.idx >= len(p.nodes) {
		return n, -1, false
	}
	return p.nodes[e.idx], e.idx, true
}

// perform runs an action of assistive technology on the element. The
// callback queues it onto the main thread.
func (p *uiaProvider) perform(e *uiaElement, action int) uintptr {
	n, idx, ok := p.node(e)
	switch {
	case !ok:
		return uiaElementNotAvailable
	case n.State.Has(gui.AccessStateDisabled):
		return uiaElementNotEnabled
	}
	if p.act != nil {
		p.act(action, idx)
	}
	return sOK
}

// clientOrigin returns the screen position of the client area's
// top-left corner, in physical pixels.
func (p *uiaProvider) clientOrigin() (float64, float64) {
	var pt pointW
	pClientToScreen.Call(p.hwnd, uintptr(unsafe.Pointer(&pt)))
	return float64(pt.x), float64(pt.y)
}

// screenRect returns a node's bounds in physical screen pixels.
func (p *uiaProvider) screenRect(n *gui.A11yNode, scale float32, ox, oy float64) uiaRect {
	s := float64(scale)
	return uiaRect{
		left:   ox + float64(n.X)*s,
		top:    oy + float64(n.Y)*s,
		width:  float64(n.W) * s,
		height: float64(n.H) * s,
	}
}

// at returns the element at a screen point, the last of the pre-order
// walk that contains it: the deepest, and the topmost of overlapping.
func (p *uiaProvider) at(x, y float64) *uiaElement {
	ox, oy := p.clientOrigin()
	p.mu.Lock()
	defer p.mu.Unlock()
	s := float64(p.scale)
	lx, ly := float32((x-ox)/s), float32((y-oy)/s)
	for i := len(p.nodes) - 1; i >= 0; i-- {
		n := &p.nodes[i]
		if n.W > 0 && n.H > 0 && lx >= n.X && ly >= n.Y && lx < n.X+n.W && ly < n.Y+n.H {
			return p.elems[i]
		}
	}
	return nil
}

// navigate returns the element in a NavigateDirection, or nil.
func (p *uiaProvider) navigate(e *uiaElement, dir uintptr) *uiaElement {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e.dead.Load() {
		return nil
	}
	rootKey := len(p.nodes)
	key := e.idx
	if key < 0 {
		key = rootKey
	} else if key >= rootKey {
		return nil
	}
	pick := func(i int32) *uiaElement {
		if i < 0 {
			return nil
		}
		return p.elems[i]
	}
	switch dir {
	case 0: // NavigateDirection_Parent
		if key == rootKey {
			return nil
		}
		if pi := p.links.parent(p.nodes, key); pi != rootKey {
			return p.elems[pi]
		}
		return p.root
	case 1: // NextSibling
		if key != rootKey {
			return pick(p.links.next[key])
		}
	case 2: // PreviousSibling
		if key != rootKey {
			return pick(p.links.prev[key])
		}
	case 3: // FirstChild
		return pick(p.links.first[key])
	case 4: // LastChild
		return pick(p.links.last[key])
	}
	return nil
}

// containerOf returns the index of the nearest ancestor holding the
// node's Selection, or -1. Called with mu held.
func (p *uiaProvider) containerOf(idx int) int {
	for i := p.links.parent(p.nodes, idx); i < len(p.nodes); i = p.links.parent(p.nodes, i) {
		if uiaSelectionContainer(p.nodes[i].Role) {
			return i
		}
	}
	return -1
}
