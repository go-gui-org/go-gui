//go:build linux

// Package atspi provides AT-SPI2 accessibility bridge for Linux.
package atspi

import (
	"fmt"
	"sync"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	busName     = "org.a11y.Bus"
	busPath     = "/org/a11y/bus"
	busIface    = "org.a11y.Bus"
	registryBus = "org.a11y.atspi.Registry"
	registryObj = "/org/a11y/atspi/accessible/root"
	socketIface = "org.a11y.atspi.Socket"

	ifaceAccessible = "org.a11y.atspi.Accessible"
	ifaceComponent  = "org.a11y.atspi.Component"
	ifaceAction     = "org.a11y.atspi.Action"
	ifaceValue      = "org.a11y.atspi.Value"

	appPath = "/org/gui/a11y/app"
)

// Bridge manages the AT-SPI2 D-Bus connection and object tree.
type Bridge struct {
	mu             sync.Mutex
	conn           *dbus.Conn
	busName        string
	actionCallback func(action, index int)
	nodes          []gui.A11yNode
	nodeCount      int
	focusedIdx     int
	prevFocusedIdx int
}

// nodeHandler handles D-Bus method calls for a single a11y
// node (or the app root).
type nodeHandler struct {
	bridge *Bridge
	index  int // -1 for app root
}

// Init connects to the AT-SPI2 accessibility bus and registers
// as an application. If any step fails, the bridge becomes a
// no-op.
func (b *Bridge) Init(cb func(action, index int)) {
	b.actionCallback = cb
	b.prevFocusedIdx = -1
	b.focusedIdx = -1

	sessionBus, err := dbus.SessionBus()
	if err != nil {
		return
	}

	// Get AT-SPI2 bus address.
	var addr string
	obj := sessionBus.Object(busName, busPath)
	if err := obj.Call(busIface+".GetAddress", 0).Store(&addr); err != nil {
		return
	}
	if addr == "" {
		return
	}

	conn, err := dbus.Connect(addr)
	if err != nil {
		return
	}
	b.conn = conn
	b.busName = conn.Names()[0]

	// Export app root handler.
	b.export(-1, appPath)

	// Register with AT-SPI2 registry.
	plug := [2]any{b.busName, dbus.ObjectPath(appPath)}
	regObj := conn.Object(registryBus, registryObj)
	call := regObj.Call(socketIface+".Embed", 0, plug)
	if call.Err != nil {
		_ = b.conn.Close()
		b.conn = nil
		return
	}
}

// export registers D-Bus object handlers for the given node
// index (-1 = app root).
func (b *Bridge) export(index int, path string) {
	h := &nodeHandler{bridge: b, index: index}
	_ = b.conn.Export(h, dbus.ObjectPath(path), ifaceAccessible)
	_ = b.conn.Export(h, dbus.ObjectPath(path), ifaceComponent)
	_ = b.conn.Export(h, dbus.ObjectPath(path), ifaceAction)
	_ = b.conn.Export(h, dbus.ObjectPath(path), ifaceValue)

	// Minimal introspection so D-Bus tools can discover
	// interfaces.
	intro := introspect.Node{
		Name: path,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{Name: ifaceAccessible},
			{Name: ifaceComponent},
			{Name: ifaceAction},
			{Name: ifaceValue},
		},
	}
	_ = b.conn.Export(
		introspect.NewIntrospectable(&intro),
		dbus.ObjectPath(path),
		"org.freedesktop.DBus.Introspectable",
	)
}

// ensureExported exports D-Bus objects for nodes up to count.
// Called under lock.
func (b *Bridge) ensureExported(count int) {
	for i := b.nodeCount; i < count; i++ {
		path := fmt.Sprintf("/org/gui/a11y/%d", i)
		b.export(i, path)
	}
}

// Sync updates the bridge's node snapshot and emits focus
// change signals. Called every frame from the GUI thread.
func (b *Bridge) Sync(nodes []gui.A11yNode, count, focusedIdx int) {
	if b.conn == nil {
		return
	}
	b.mu.Lock()

	// Grow exported objects if needed.
	if count > b.nodeCount {
		b.ensureExported(count)
	}

	// Copy node data.
	if cap(b.nodes) < count {
		b.nodes = make([]gui.A11yNode, count)
	}
	b.nodes = b.nodes[:count]
	copy(b.nodes, nodes[:count])
	b.nodeCount = count
	b.focusedIdx = focusedIdx

	prevFocused := b.prevFocusedIdx
	b.prevFocusedIdx = focusedIdx
	b.mu.Unlock()

	// Emit focus change signal.
	if focusedIdx != prevFocused && focusedIdx >= 0 && focusedIdx < count {
		path := fmt.Sprintf("/org/gui/a11y/%d", focusedIdx)
		_ = b.conn.Emit(
			dbus.ObjectPath(path),
			"org.a11y.atspi.Event.Object",
			"object:state-changed:focused",
			"", 1, 0, dbus.MakeVariant(""),
		)
	}
}

// Destroy closes the AT-SPI2 bus connection.
func (b *Bridge) Destroy() {
	if b.conn != nil {
		_ = b.conn.Close()
		b.conn = nil
	}
}

// Announce emits an announcement signal on the app root.
func (b *Bridge) Announce(text string) {
	if b.conn == nil {
		return
	}
	_ = b.conn.Emit(
		appPath,
		"org.a11y.atspi.Event.Object",
		"object:announcement",
		text, 0, 0, dbus.MakeVariant(""),
	)
}

// --- D-Bus method handlers (nodeHandler) ---

// GetChildAtIndex returns a (busName, objectPath) reference to
// the child at the given index.
func (h *nodeHandler) GetChildAtIndex(idx int32) (string, dbus.ObjectPath, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index == -1 {
		// App root: children are top-level nodes (parentIdx == -1).
		count := 0
		for i := range h.bridge.nodeCount {
			if h.bridge.nodes[i].ParentIdx == -1 {
				if count == int(idx) {
					path := dbus.ObjectPath(fmt.Sprintf("/org/gui/a11y/%d", i))
					return h.bridge.busName, path, nil
				}
				count++
			}
		}
		return h.bridge.busName, "/org/gui/a11y/app", nil
	}

	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return h.bridge.busName, "/org/gui/a11y/app", nil
	}
	node := &h.bridge.nodes[h.index]
	childIdx := node.ChildrenStart + int(idx)
	if int(idx) < 0 || int(idx) >= node.ChildrenCount || childIdx >= h.bridge.nodeCount {
		return h.bridge.busName, "/org/gui/a11y/app", nil
	}
	path := dbus.ObjectPath(fmt.Sprintf("/org/gui/a11y/%d", childIdx))
	return h.bridge.busName, path, nil
}

// ChildCount is a D-Bus property getter.
func (h *nodeHandler) GetChildCount() (int32, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index == -1 {
		count := int32(0)
		for i := range h.bridge.nodeCount {
			if h.bridge.nodes[i].ParentIdx == -1 {
				count++
			}
		}
		return count, nil
	}
	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return 0, nil
	}
	return int32(h.bridge.nodes[h.index].ChildrenCount), nil
}

// GetName returns the accessible name.
func (h *nodeHandler) GetName() (string, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index == -1 {
		return "go-gui", nil
	}
	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return "", nil
	}
	return h.bridge.nodes[h.index].Label, nil
}

// GetDescription returns the accessible description.
func (h *nodeHandler) GetDescription() (string, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index == -1 {
		return "", nil
	}
	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return "", nil
	}
	return h.bridge.nodes[h.index].Description, nil
}

// GetRole returns the AT-SPI2 role.
func (h *nodeHandler) GetRole() (uint32, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index == -1 {
		return roleApplication, nil
	}
	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return roleInvalid, nil
	}
	role := h.bridge.nodes[h.index].Role
	if int(role) < len(atspiRole) {
		return atspiRole[role], nil
	}
	return roleInvalid, nil
}

// GetRoleName returns the role as a string.
func (h *nodeHandler) GetRoleName() (string, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index == -1 {
		return "application", nil
	}
	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return "unknown", nil
	}
	role := h.bridge.nodes[h.index].Role
	if int(role) < len(atspiRoleName) && atspiRoleName[role] != "" {
		return atspiRoleName[role], nil
	}
	return "unknown", nil
}

// GetState returns the AT-SPI2 state bitfield.
func (h *nodeHandler) GetState() ([2]uint32, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index == -1 {
		var bits [2]uint32
		setBit(&bits, stateEnabled)
		setBit(&bits, stateSensitive)
		setBit(&bits, stateVisible)
		setBit(&bits, stateShowing)
		return bits, nil
	}
	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return [2]uint32{}, nil
	}
	node := &h.bridge.nodes[h.index]
	focused := h.index == h.bridge.focusedIdx
	return atspiState(node.State, focused), nil
}

// GetParent returns the parent reference.
func (h *nodeHandler) GetParent() (string, dbus.ObjectPath, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index == -1 {
		// App root's parent is the desktop (registry root).
		return registryBus, dbus.ObjectPath(registryObj), nil
	}
	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return h.bridge.busName, appPath, nil
	}
	pIdx := h.bridge.nodes[h.index].ParentIdx
	if pIdx < 0 {
		return h.bridge.busName, appPath, nil
	}
	path := dbus.ObjectPath(fmt.Sprintf("/org/gui/a11y/%d", pIdx))
	return h.bridge.busName, path, nil
}

// --- Component interface ---

// GetExtents returns node position and size.
func (h *nodeHandler) GetExtents(_ uint32) (int32, int32, int32, int32, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return 0, 0, 0, 0, nil
	}
	n := &h.bridge.nodes[h.index]
	return int32(n.X), int32(n.Y), int32(n.W), int32(n.H), nil
}

// GetPosition returns the node position.
func (h *nodeHandler) GetPosition(_ uint32) (int32, int32, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return 0, 0, nil
	}
	n := &h.bridge.nodes[h.index]
	return int32(n.X), int32(n.Y), nil
}

// GetSize returns the node size.
func (h *nodeHandler) GetSize() (int32, int32, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return 0, 0, nil
	}
	n := &h.bridge.nodes[h.index]
	return int32(n.W), int32(n.H), nil
}

// --- Action interface ---

// GetNActions returns the number of available actions.
func (h *nodeHandler) GetNActions() (int32, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return 0, nil
	}
	role := h.bridge.nodes[h.index].Role
	if role == gui.AccessRoleSlider {
		return 5, nil // press, increment, decrement, confirm, cancel
	}
	return 3, nil // press, confirm, cancel
}

// GetActionName returns the name of the action at the given
// index.
func (h *nodeHandler) GetActionName(idx int32) (string, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	role := gui.AccessRoleNone
	if h.index >= 0 && h.index < h.bridge.nodeCount {
		role = h.bridge.nodes[h.index].Role
	}

	if role == gui.AccessRoleSlider {
		switch idx {
		case 0:
			return "press", nil
		case 1:
			return "increment", nil
		case 2:
			return "decrement", nil
		case 3:
			return "confirm", nil
		case 4:
			return "cancel", nil
		}
		return "", nil
	}

	switch idx {
	case 0:
		return "press", nil
	case 1:
		return "confirm", nil
	case 2:
		return "cancel", nil
	}
	return "", nil
}

// DoAction performs the action at the given index.
func (h *nodeHandler) DoAction(idx int32) (bool, *dbus.Error) {
	if h.bridge.actionCallback == nil {
		return false, nil
	}
	h.bridge.mu.Lock()
	nodeIdx := h.index
	role := gui.AccessRoleNone
	if nodeIdx >= 0 && nodeIdx < h.bridge.nodeCount {
		role = h.bridge.nodes[nodeIdx].Role
	}
	h.bridge.mu.Unlock()

	if nodeIdx < 0 {
		return false, nil
	}

	var action int
	if role == gui.AccessRoleSlider {
		switch idx {
		case 0:
			action = gui.A11yActionPress
		case 1:
			action = gui.A11yActionIncrement
		case 2:
			action = gui.A11yActionDecrement
		case 3:
			action = gui.A11yActionConfirm
		case 4:
			action = gui.A11yActionCancel
		default:
			return false, nil
		}
	} else {
		switch idx {
		case 0:
			action = gui.A11yActionPress
		case 1:
			action = gui.A11yActionConfirm
		case 2:
			action = gui.A11yActionCancel
		default:
			return false, nil
		}
	}

	h.bridge.actionCallback(action, nodeIdx)
	return true, nil
}

// --- Value interface ---

// GetCurrentValue returns the current numeric value.
func (h *nodeHandler) GetCurrentValue() (float64, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return 0, nil
	}
	return float64(h.bridge.nodes[h.index].ValueNum), nil
}

// GetMaximumValue returns the maximum numeric value.
func (h *nodeHandler) GetMaximumValue() (float64, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return 0, nil
	}
	return float64(h.bridge.nodes[h.index].ValueMax), nil
}

// GetMinimumValue returns the minimum numeric value.
func (h *nodeHandler) GetMinimumValue() (float64, *dbus.Error) {
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	if h.index < 0 || h.index >= h.bridge.nodeCount {
		return 0, nil
	}
	return float64(h.bridge.nodes[h.index].ValueMin), nil
}

// GetMinimumIncrement returns the smallest meaningful value
// step. Returns 0 (unspecified).
func (h *nodeHandler) GetMinimumIncrement() (float64, *dbus.Error) {
	return 0, nil
}
