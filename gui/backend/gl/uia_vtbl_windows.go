//go:build windows && !js && (amd64 || arm64)

package gl

import (
	"math"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

// The vtables of uiaElement and the helpers of their methods; the
// provider is in uia_windows.go.

// out stores an interface of an element, with a reference, at p.
func out(p uintptr, e *uiaElement, i int) uintptr {
	if p == 0 {
		return ePointer
	}
	if e == nil {
		*(*uintptr)(ptrFromLParam(p)) = 0
		return sOK
	}
	e.addRef()
	*(*uintptr)(ptrFromLParam(p)) = e.ptr(i)
	return sOK
}

func setBool(p uintptr, v bool) uintptr {
	if p == 0 {
		return ePointer
	}
	var b int32
	if v {
		b = 1
	}
	*(*int32)(ptrFromLParam(p)) = b
	return sOK
}

func setFloat(p uintptr, v float64) uintptr {
	if p == 0 {
		return ePointer
	}
	*(*float64)(ptrFromLParam(p)) = v
	return sOK
}

func setInt32(p uintptr, v int32) uintptr {
	if p == 0 {
		return ePointer
	}
	*(*int32)(ptrFromLParam(p)) = v
	return sOK
}

// property fills in a property of a node, leaving the ones it has not
// empty for UIA's defaults.
func property(n *gui.A11yNode, focused bool, id int, v *variant) {
	str := func(s string) {
		if s != "" {
			*v = variant{vt: vtBSTR, val: uint64(uiaBSTR(s))}
		}
	}
	switch id {
	case uiaControlTypeProperty:
		*v = variant{vt: vtI4, val: uint64(uint32(uiaControlType(n.Role)))}
	case uiaLocalizedControlTypeProp:
		str(uiaLocalizedType(n.Role))
	case uiaNameProperty:
		str(n.Label)
	case uiaHasKeyboardFocusProperty:
		*v = boolVariant(focused)
	case uiaIsKeyboardFocusableProp:
		*v = boolVariant(focused || uiaFocusable(n.Role))
	case uiaIsEnabledProperty:
		*v = boolVariant(!n.State.Has(gui.AccessStateDisabled))
	case uiaHelpTextProperty, uiaFullDescriptionProperty:
		str(n.Description)
	case uiaIsRequiredForFormProperty:
		*v = boolVariant(n.State.Has(gui.AccessStateRequired))
	case uiaIsDataValidForFormProperty:
		*v = boolVariant(!n.State.Has(gui.AccessStateInvalid))
	case uiaItemStatusProperty:
		if n.State.Has(gui.AccessStateBusy) {
			str("busy")
		}
	case uiaIsDialogProperty:
		*v = boolVariant(n.Role == gui.AccessRoleDialog)
	case uiaLiveSettingProperty:
		if n.State.Has(gui.AccessStateLive) {
			*v = variant{vt: vtI4, val: uint64(liveSettingPolite)}
		}
	case uiaFrameworkIDProperty:
		str("go-gui")
	}
}

//nolint:gocyclo,maintidx // one vtable per interface, built once
func initUIAVtbls() {
	cb := syscall.NewCallback
	unknown := []uintptr{
		cb(func(this, riid, pp uintptr) uintptr { // QueryInterface
			if pp == 0 {
				return ePointer
			}
			*(*uintptr)(ptrFromLParam(pp)) = 0
			if riid == 0 {
				return eInvalidArg
			}
			e := uiaOf(this)
			iid := *(*windows.GUID)(ptrFromLParam(riid))
			if iid == iidIUnknown {
				return out(pp, e, ifaceSimple)
			}
			for i, id := range uiaIIDs {
				if id == iid && e.supports(i) {
					return out(pp, e, i)
				}
			}
			return eNoInterface
		}),
		cb(func(this uintptr) uintptr { return uintptr(uiaOf(this).addRef()) }),
		cb(func(this uintptr) uintptr { return uintptr(uiaOf(this).release()) }),
	}
	vtbl := func(methods ...uintptr) []uintptr {
		return append(append([]uintptr(nil), unknown...), methods...)
	}
	alive := func(this uintptr) (*uiaElement, uintptr) {
		e := uiaOf(this)
		if e.dead.Load() {
			return nil, uiaElementNotAvailable
		}
		return e, sOK
	}
	// withNode runs fn on a copy of the element's node.
	withNode := func(this uintptr, fn func(n *gui.A11yNode) uintptr) uintptr {
		e := uiaOf(this)
		n, _, ok := e.p.node(e)
		if !ok {
			return uiaElementNotAvailable
		}
		return fn(&n)
	}

	uiaVtbls[ifaceSimple] = vtbl(
		cb(func(_, pp uintptr) uintptr { // get_ProviderOptions
			return setInt32(pp, providerOptionsServerSide)
		}),
		cb(func(this, pattern, pp uintptr) uintptr { // GetPatternProvider
			e, hr := alive(this)
			if e == nil {
				out(pp, nil, 0)
				return hr
			}
			if i, ok := uiaPatternIfaces[pattern]; ok && e.supports(i) {
				return out(pp, e, i)
			}
			return out(pp, nil, 0)
		}),
		cb(func(this, id, pp uintptr) uintptr { // GetPropertyValue
			if pp == 0 {
				return ePointer
			}
			v := (*variant)(ptrFromLParam(pp))
			*v = variant{}
			e, hr := alive(this)
			if e == nil {
				return hr
			}
			if e == e.p.root {
				return sOK // the root's are its window's
			}
			p := e.p
			p.mu.Lock()
			if e.dead.Load() || e.idx >= len(p.nodes) {
				p.mu.Unlock()
				return uiaElementNotAvailable
			}
			n := p.nodes[e.idx]
			focused := e.idx == p.focus
			if id == uiaNameProperty && n.Label == "" {
				n.Label = uiaNameFromContent(p.nodes, e.idx)
			}
			p.mu.Unlock()
			property(&n, focused, int(id), v)
			return sOK
		}),
		cb(func(this, pp uintptr) uintptr { // get_HostRawElementProvider
			e := uiaOf(this)
			if e == e.p.root && !e.dead.Load() {
				r, _, _ := pUiaHostProviderFromHwnd.Call(e.p.hwnd, pp)
				return r
			}
			return out(pp, nil, 0)
		}),
	)

	uiaVtbls[ifaceFragment] = vtbl(
		cb(func(this, dir, pp uintptr) uintptr { // Navigate
			e, hr := alive(this)
			if e == nil {
				out(pp, nil, 0)
				return hr
			}
			return out(pp, e.p.navigate(e, dir), ifaceFragment)
		}),
		cb(func(this, pp uintptr) uintptr { // GetRuntimeId
			if pp == 0 {
				return ePointer
			}
			*(*uintptr)(ptrFromLParam(pp)) = 0
			e := uiaOf(this)
			if e == e.p.root {
				return sOK // the root's is its window's
			}
			// UiaAppendRuntimeId, then the element's serial.
			id := [2]int32{3, e.serial}
			sa, _, _ := pSafeArrayCreateVector.Call(vtI4, 0, uintptr(len(id)))
			if sa == 0 {
				return 0x8007000E // E_OUTOFMEMORY
			}
			for i := range id {
				index := int32(i)
				pSafeArrayPutElement.Call(sa, uintptr(unsafe.Pointer(&index)), uintptr(unsafe.Pointer(&id[i])))
			}
			*(*uintptr)(ptrFromLParam(pp)) = sa
			return sOK
		}),
		cb(func(this, pp uintptr) uintptr { // get_BoundingRectangle
			if pp == 0 {
				return ePointer
			}
			*(*uiaRect)(ptrFromLParam(pp)) = uiaRect{}
			e, hr := alive(this)
			if e == nil {
				return hr
			}
			if e == e.p.root {
				return sOK // the root's is its window's
			}
			n, _, ok := e.p.node(e)
			if !ok {
				return uiaElementNotAvailable
			}
			e.p.mu.Lock()
			scale := e.p.scale
			e.p.mu.Unlock()
			ox, oy := e.p.clientOrigin()
			*(*uiaRect)(ptrFromLParam(pp)) = e.p.screenRect(&n, scale, ox, oy)
			return sOK
		}),
		cb(func(_, pp uintptr) uintptr { // GetEmbeddedFragmentRoots
			return out(pp, nil, 0)
		}),
		cb(func(this uintptr) uintptr { // SetFocus
			// go-gui has no focus action; UIA moves Win32 focus to the
			// window itself.
			_, hr := alive(this)
			return hr
		}),
		cb(func(this, pp uintptr) uintptr { // get_FragmentRoot
			e, hr := alive(this)
			if e == nil {
				out(pp, nil, 0)
				return hr
			}
			return out(pp, e.p.root, ifaceRoot)
		}),
	)

	uiaFromPointCallback = cb(func(this, x, y, pp uintptr) uintptr { // ElementProviderFromPoint
		e, hr := alive(this)
		if e == nil {
			out(pp, nil, 0)
			return hr
		}
		fx, fy := math.Float64frombits(uint64(x)), math.Float64frombits(uint64(y))
		return out(pp, e.p.at(fx, fy), ifaceFragment)
	})
	uiaSetValueCallback = cb(func(this, v uintptr) uintptr { // IRangeValueProvider::SetValue
		e, hr := alive(this)
		if e == nil {
			return hr
		}
		return e.p.setRangeValue(e, math.Float64frombits(uint64(v)))
	})
	fromPoint, setValue := uiaThunks()

	uiaVtbls[ifaceRoot] = vtbl(
		fromPoint,
		cb(func(this, pp uintptr) uintptr { // GetFocus
			e, hr := alive(this)
			if e == nil {
				out(pp, nil, 0)
				return hr
			}
			e.p.mu.Lock()
			var f *uiaElement
			if e.p.focus >= 0 {
				f = e.p.elems[e.p.focus]
			}
			e.p.mu.Unlock()
			return out(pp, f, ifaceFragment)
		}),
	)

	press := func(this uintptr) uintptr {
		e, hr := alive(this)
		if e == nil {
			return hr
		}
		return e.p.perform(e, gui.A11yActionPress)
	}
	uiaVtbls[ifaceInvoke] = vtbl(cb(press))

	uiaVtbls[ifaceToggle] = vtbl(
		cb(press),
		cb(func(this, pp uintptr) uintptr { // get_ToggleState
			return withNode(this, func(n *gui.A11yNode) uintptr {
				return setInt32(pp, uiaToggleState(n))
			})
		}),
	)

	selectItem := cb(func(this uintptr) uintptr { // Select, AddToSelection
		return withNode(this, func(n *gui.A11yNode) uintptr {
			if uiaIsSelected(n) {
				return sOK
			}
			return press(this)
		})
	})
	uiaVtbls[ifaceSelectionItem] = vtbl(
		selectItem,
		selectItem,
		cb(func(uintptr) uintptr { return uiaInvalidOperation }), // RemoveFromSelection
		cb(func(this, pp uintptr) uintptr { // get_IsSelected
			return withNode(this, func(n *gui.A11yNode) uintptr {
				return setBool(pp, uiaIsSelected(n))
			})
		}),
		cb(func(this, pp uintptr) uintptr { // get_SelectionContainer
			e, hr := alive(this)
			if e == nil {
				out(pp, nil, 0)
				return hr
			}
			var c *uiaElement
			e.p.mu.Lock()
			if e.idx >= 0 && e.idx < len(e.p.nodes) {
				if ci := e.p.containerOf(e.idx); ci >= 0 {
					c = e.p.elems[ci]
				}
			}
			e.p.mu.Unlock()
			return out(pp, c, ifaceSimple)
		}),
	)

	uiaVtbls[ifaceSelection] = vtbl(
		cb(func(this, pp uintptr) uintptr { // GetSelection
			if pp == 0 {
				return ePointer
			}
			*(*uintptr)(ptrFromLParam(pp)) = 0
			e, hr := alive(this)
			if e == nil {
				return hr
			}
			chosen := e.p.chosen(e)
			sa, _, _ := pSafeArrayCreateVector.Call(vtUnknown, 0, uintptr(len(chosen)))
			if sa == 0 {
				return 0x8007000E // E_OUTOFMEMORY
			}
			for i, c := range chosen {
				index := int32(i)
				// SafeArrayPutElement takes an IUnknown itself, and
				// AddRefs it.
				pSafeArrayPutElement.Call(sa, uintptr(unsafe.Pointer(&index)), c.ptr(ifaceSimple))
			}
			*(*uintptr)(ptrFromLParam(pp)) = sa
			return sOK
		}),
		cb(func(_, pp uintptr) uintptr { return setBool(pp, false) }), // get_CanSelectMultiple
		cb(func(_, pp uintptr) uintptr { return setBool(pp, false) }), // get_IsSelectionRequired
	)

	rangeOf := func(get func(n *gui.A11yNode) float64) uintptr {
		return cb(func(this, pp uintptr) uintptr {
			return withNode(this, func(n *gui.A11yNode) uintptr {
				return setFloat(pp, get(n))
			})
		})
	}
	uiaVtbls[ifaceRangeValue] = vtbl(
		setValue,
		rangeOf(func(n *gui.A11yNode) float64 { return float64(n.ValueNum) }),
		cb(func(this, pp uintptr) uintptr { // get_IsReadOnly
			return withNode(this, func(n *gui.A11yNode) uintptr {
				return setBool(pp, uiaRangeReadOnly(n))
			})
		}),
		rangeOf(func(n *gui.A11yNode) float64 { return float64(n.ValueMax) }),
		rangeOf(func(n *gui.A11yNode) float64 { return float64(n.ValueMin) }),
		rangeOf(func(n *gui.A11yNode) float64 { return float64(n.ValueMax-n.ValueMin) / 10 }),  // LargeChange
		rangeOf(func(n *gui.A11yNode) float64 { return float64(n.ValueMax-n.ValueMin) / 100 }), // SmallChange
	)

	uiaVtbls[ifaceValue] = vtbl(
		cb(func(this, _ uintptr) uintptr { // SetValue
			// go-gui has no set-value action: text arrives as keys.
			_, hr := alive(this)
			if hr != sOK {
				return hr
			}
			return uiaInvalidOperation
		}),
		cb(func(this, pp uintptr) uintptr { // get_Value
			return withNode(this, func(n *gui.A11yNode) uintptr {
				if pp == 0 {
					return ePointer
				}
				*(*uintptr)(ptrFromLParam(pp)) = uiaBSTR(n.Value)
				return sOK
			})
		}),
		cb(func(this, pp uintptr) uintptr { // get_IsReadOnly
			return withNode(this, func(n *gui.A11yNode) uintptr {
				return setBool(pp, n.State.Has(gui.AccessStateReadOnly))
			})
		}),
	)

	expand := func(open bool) uintptr {
		return cb(func(this uintptr) uintptr {
			return withNode(this, func(n *gui.A11yNode) uintptr {
				if n.State.Has(gui.AccessStateExpanded) == open {
					return sOK
				}
				return press(this)
			})
		})
	}
	uiaVtbls[ifaceExpandCollapse] = vtbl(
		expand(true),
		expand(false),
		cb(func(this, pp uintptr) uintptr { // get_ExpandCollapseState
			return withNode(this, func(n *gui.A11yNode) uintptr {
				return setInt32(pp, uiaExpandState(n))
			})
		}),
	)
}

// supports reports whether the element hands out an interface.
func (e *uiaElement) supports(i int) bool {
	switch i {
	case ifaceSimple, ifaceFragment:
		return true
	case ifaceRoot:
		return e == e.p.root
	}
	n, _, ok := e.p.node(e)
	return ok && uiaSupports(n.Role, i)
}

// chosen returns the selected items whose Selection is e's.
func (p *uiaProvider) chosen(e *uiaElement) []*uiaElement {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e.idx < 0 || e.idx >= len(p.nodes) {
		return nil
	}
	var c []*uiaElement
	for i := e.idx + 1; i < len(p.nodes); i++ {
		n := &p.nodes[i]
		if uiaSupports(n.Role, ifaceSelectionItem) && uiaIsSelected(n) && p.containerOf(i) == e.idx {
			c = append(c, p.elems[i])
		}
	}
	return c
}

// setRangeValue moves a slider one step toward v, as an arrow key
// does: go-gui's actions step, they do not set.
func (p *uiaProvider) setRangeValue(e *uiaElement, v float64) uintptr {
	n, _, ok := p.node(e)
	switch {
	case !ok:
		return uiaElementNotAvailable
	case math.IsNaN(v) || math.IsInf(v, 0):
		return eInvalidArg
	case uiaRangeReadOnly(&n):
		if n.State.Has(gui.AccessStateDisabled) {
			return uiaElementNotEnabled
		}
		return uiaInvalidOperation
	case v > float64(n.ValueNum):
		return p.perform(e, gui.A11yActionIncrement)
	case v < float64(n.ValueNum):
		return p.perform(e, gui.A11yActionDecrement)
	}
	return sOK
}
