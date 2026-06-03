//go:build linux

// Package sni provides StatusNotifierItem (SNI) system tray
// support for Linux via D-Bus. SNI is the modern tray protocol
// supported by KDE, GNOME (via extension), XFCE, LXQt, etc.
package sni

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"strings"
	"sync"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	watcherBus  = "org.kde.StatusNotifierWatcher"
	watcherPath = "/StatusNotifierWatcher"
	watcherReg  = watcherBus + ".RegisterStatusNotifierItem"

	sniIface   = "org.kde.StatusNotifierItem"
	sniPath    = "/StatusNotifierItem"
	propsIface = "org.freedesktop.DBus.Properties"

	menuIface = "com.canonical.dbusmenu"
	menuPath  = "/MenuBar"
)

// Tray manages SNI tray entries on the session bus.
type Tray struct {
	mu      sync.Mutex
	entries map[int]*entry
	nextID  int
}

type entry struct {
	id         int
	conn       *dbus.Conn
	busName    string
	tooltip    string
	iconPixmap []sniPixmap
	menuNodes  []menuNode
	actionCb   func(string)
	revision   uint32
}

type sniPixmap struct {
	Width  int32
	Height int32
	Data   []byte
}

type menuNode struct {
	dbusID     int32
	actionID   string
	label      string
	separator  bool
	disabled   bool
	checked    bool
	childStart int
	childCount int
}

// --- D-Bus handler types ---

type sniHandler struct {
	tray    *Tray
	entryID int
}

type menuHandler struct {
	tray    *Tray
	entryID int
}

// Create registers a new SNI tray icon with menu.
func (t *Tray) Create(
	cfg gui.SystemTrayCfg, actionCb func(string),
) (int, error) {
	conn, err := openPrivateConn()
	if err != nil {
		return 0, fmt.Errorf("sni: session bus: %w", err)
	}

	t.mu.Lock()
	if t.entries == nil {
		t.entries = make(map[int]*entry)
	}
	t.nextID++
	id := t.nextID
	t.mu.Unlock()

	busName := fmt.Sprintf(
		"org.kde.StatusNotifierItem-%d-%d", os.Getpid(), id)
	reply, reqErr := conn.RequestName(busName,
		dbus.NameFlagDoNotQueue)
	if reqErr != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		_ = conn.Close()
		return 0, fmt.Errorf("sni: request name %s: %v",
			busName, reqErr)
	}

	e := &entry{
		id:       id,
		conn:     conn,
		busName:  busName,
		tooltip:  cfg.Tooltip,
		actionCb: actionCb,
	}

	if len(cfg.IconPNG) > 0 {
		if px, pxErr := pngToPixmap(cfg.IconPNG); pxErr == nil {
			e.iconPixmap = []sniPixmap{px}
		}
	}
	e.menuNodes = buildMenuNodes(cfg.Menu)

	// Export D-Bus handlers.
	sh := &sniHandler{tray: t, entryID: id}
	_ = conn.Export(sh, sniPath, propsIface)
	_ = conn.Export(sh, sniPath, sniIface)

	mh := &menuHandler{tray: t, entryID: id}
	_ = conn.Export(mh, menuPath, menuIface)

	exportIntrospect(conn)

	t.mu.Lock()
	t.entries[id] = e
	t.mu.Unlock()

	// Register with watcher.
	watcher := conn.Object(watcherBus,
		dbus.ObjectPath(watcherPath))
	call := watcher.Call(watcherReg, 0, busName)
	if call.Err != nil {
		t.mu.Lock()
		delete(t.entries, id)
		t.mu.Unlock()
		_, _ = conn.ReleaseName(busName)
		_ = conn.Close()
		return 0, fmt.Errorf("sni: register: %w", call.Err)
	}

	return id, nil
}

// Update replaces the tray icon, tooltip, and menu for an
// existing entry.
func (t *Tray) Update(id int, cfg gui.SystemTrayCfg) {
	t.mu.Lock()
	e, ok := t.entries[id]
	if !ok {
		t.mu.Unlock()
		return
	}

	e.tooltip = cfg.Tooltip
	if len(cfg.IconPNG) > 0 {
		if px, err := pngToPixmap(cfg.IconPNG); err == nil {
			e.iconPixmap = []sniPixmap{px}
		}
	}
	e.menuNodes = buildMenuNodes(cfg.Menu)
	if cfg.OnAction != nil {
		e.actionCb = cfg.OnAction
	}
	e.revision++
	rev := e.revision
	conn := e.conn
	t.mu.Unlock()

	// Signal updates to the SNI host.
	_ = conn.Emit(sniPath, sniIface+".NewIcon")
	_ = conn.Emit(sniPath, sniIface+".NewTitle")
	_ = conn.Emit(menuPath,
		menuIface+".LayoutUpdated", rev, int32(0))
}

// Remove unregisters and cleans up a tray entry.
func (t *Tray) Remove(id int) {
	t.mu.Lock()
	e, ok := t.entries[id]
	if !ok {
		t.mu.Unlock()
		return
	}
	delete(t.entries, id)
	t.mu.Unlock()

	_, _ = e.conn.ReleaseName(e.busName)
	_ = e.conn.Close()
}

// --- SNI Properties handler ---

// Get implements org.freedesktop.DBus.Properties.Get.
func (h *sniHandler) Get(
	iface, prop string,
) (dbus.Variant, *dbus.Error) {
	if iface != "" && iface != sniIface {
		return dbus.MakeVariant(""),
			dbus.MakeFailedError(
				fmt.Errorf("unknown interface %s", iface))
	}
	h.tray.mu.Lock()
	e, ok := h.tray.entries[h.entryID]
	if !ok {
		h.tray.mu.Unlock()
		return dbus.MakeVariant(""), nil
	}

	var v dbus.Variant
	switch prop {
	case "Category":
		v = dbus.MakeVariant("ApplicationStatus")
	case "Id":
		v = dbus.MakeVariant("go-gui")
	case "Title":
		v = dbus.MakeVariant(e.tooltip)
	case "Status":
		v = dbus.MakeVariant("Active")
	case "IconName":
		v = dbus.MakeVariant("")
	case "IconPixmap":
		v = dbus.MakeVariant(encodePixmaps(e.iconPixmap))
	case "OverlayIconName":
		v = dbus.MakeVariant("")
	case "OverlayIconPixmap":
		v = dbus.MakeVariant([]pixmapDBus{})
	case "AttentionIconName":
		v = dbus.MakeVariant("")
	case "AttentionIconPixmap":
		v = dbus.MakeVariant([]pixmapDBus{})
	case "AttentionMovieName":
		v = dbus.MakeVariant("")
	case "ToolTip":
		v = dbus.MakeVariant(tooltipDBus{
			IconName:   "",
			IconPixmap: []pixmapDBus{},
			Title:      e.tooltip,
			Body:       "",
		})
	case "Menu":
		v = dbus.MakeVariant(dbus.ObjectPath(menuPath))
	case "ItemIsMenu":
		v = dbus.MakeVariant(false)
	case "IconThemePath":
		v = dbus.MakeVariant("")
	default:
		v = dbus.MakeVariant("")
	}
	h.tray.mu.Unlock()
	return v, nil
}

// GetAll implements org.freedesktop.DBus.Properties.GetAll.
func (h *sniHandler) GetAll(
	iface string,
) (map[string]dbus.Variant, *dbus.Error) {
	if iface != sniIface {
		return nil, nil
	}
	h.tray.mu.Lock()
	e, ok := h.tray.entries[h.entryID]
	if !ok {
		h.tray.mu.Unlock()
		return nil, nil
	}

	m := map[string]dbus.Variant{
		"Category":            dbus.MakeVariant("ApplicationStatus"),
		"Id":                  dbus.MakeVariant("go-gui"),
		"Title":               dbus.MakeVariant(e.tooltip),
		"Status":              dbus.MakeVariant("Active"),
		"IconName":            dbus.MakeVariant(""),
		"IconPixmap":          dbus.MakeVariant(encodePixmaps(e.iconPixmap)),
		"OverlayIconName":     dbus.MakeVariant(""),
		"OverlayIconPixmap":   dbus.MakeVariant([]pixmapDBus{}),
		"AttentionIconName":   dbus.MakeVariant(""),
		"AttentionIconPixmap": dbus.MakeVariant([]pixmapDBus{}),
		"AttentionMovieName":  dbus.MakeVariant(""),
		"ToolTip": dbus.MakeVariant(tooltipDBus{
			IconName:   "",
			IconPixmap: []pixmapDBus{},
			Title:      e.tooltip,
			Body:       "",
		}),
		"Menu":          dbus.MakeVariant(dbus.ObjectPath(menuPath)),
		"ItemIsMenu":    dbus.MakeVariant(false),
		"IconThemePath": dbus.MakeVariant(""),
	}
	h.tray.mu.Unlock()
	return m, nil
}

// Set implements org.freedesktop.DBus.Properties.Set (no-op).
func (h *sniHandler) Set(
	_, _ string, _ dbus.Variant,
) *dbus.Error {
	return nil
}

// Activate handles left-click on the tray icon.
func (h *sniHandler) Activate(_ int32, _ int32) *dbus.Error {
	return nil
}

// SecondaryActivate handles middle-click on the tray icon.
func (h *sniHandler) SecondaryActivate(
	_ int32, _ int32,
) *dbus.Error {
	return nil
}

// Scroll handles scroll on the tray icon.
func (h *sniHandler) Scroll(
	_ int32, _ string,
) *dbus.Error {
	return nil
}

// --- DBusMenu handler ---

// menuLayout is a single node in the DBusMenu GetLayout
// response. The D-Bus signature is (ia{sv}av).
type menuLayout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

// GetLayout returns the menu tree starting at parentId.
func (h *menuHandler) GetLayout(
	parentId int32,
	recursionDepth int32,
	propertyNames []string,
) (uint32, menuLayout, *dbus.Error) {
	h.tray.mu.Lock()
	e, ok := h.tray.entries[h.entryID]
	if !ok {
		h.tray.mu.Unlock()
		return 0, menuLayout{}, nil
	}
	// Snapshot: slice header is safe since Update replaces
	// the entire slice (not in-place mutation).
	nodes := e.menuNodes
	rev := e.revision
	h.tray.mu.Unlock()

	layout := buildLayout(nodes, parentId, recursionDepth, 0)
	return rev, layout, nil
}

// Event handles menu item activation.
func (h *menuHandler) Event(
	id int32,
	eventId string,
	_ dbus.Variant,
	_ uint32,
) *dbus.Error {
	if eventId != "clicked" {
		return nil
	}
	h.tray.mu.Lock()
	e, ok := h.tray.entries[h.entryID]
	if !ok {
		h.tray.mu.Unlock()
		return nil
	}
	var actionID string
	if int(id) >= 0 && int(id) < len(e.menuNodes) {
		actionID = e.menuNodes[id].actionID
	}
	cb := e.actionCb
	h.tray.mu.Unlock()

	if cb != nil && actionID != "" {
		cb(actionID)
	}
	return nil
}

// AboutToShow is called before a menu is displayed.
func (h *menuHandler) AboutToShow(_ int32) (bool, *dbus.Error) {
	return false, nil
}

// GetGroupProperties returns properties for a list of item IDs.
func (h *menuHandler) GetGroupProperties(
	ids []int32, _ []string,
) ([]groupPropEntry, *dbus.Error) {
	h.tray.mu.Lock()
	e, ok := h.tray.entries[h.entryID]
	if !ok {
		h.tray.mu.Unlock()
		return nil, nil
	}

	result := make([]groupPropEntry, 0, len(ids))
	for _, reqID := range ids {
		if int(reqID) >= 0 && int(reqID) < len(e.menuNodes) {
			result = append(result, groupPropEntry{
				ID:    reqID,
				Props: menuNodeProps(&e.menuNodes[reqID]),
			})
		}
	}
	h.tray.mu.Unlock()
	return result, nil
}

// groupPropEntry maps to D-Bus struct (ia{sv}).
type groupPropEntry struct {
	ID    int32
	Props map[string]dbus.Variant
}

// --- Icon conversion ---

// pixmapDBus is the D-Bus serialization of one SNI icon frame.
// Signature: (iiay).
type pixmapDBus struct {
	Width  int32
	Height int32
	Data   []byte
}

// tooltipDBus is the SNI ToolTip property. Signature:
// (sa(iiay)ss).
type tooltipDBus struct {
	IconName   string
	IconPixmap []pixmapDBus
	Title      string
	Body       string
}

func encodePixmaps(pixmaps []sniPixmap) []pixmapDBus {
	out := make([]pixmapDBus, len(pixmaps))
	for i := range pixmaps {
		out[i] = pixmapDBus{
			Width:  pixmaps[i].Width,
			Height: pixmaps[i].Height,
			Data:   pixmaps[i].Data,
		}
	}
	return out
}

// maxIconDim guards against memory bombs from oversized PNGs.
const maxIconDim = 1024

func pngToPixmap(pngData []byte) (sniPixmap, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return sniPixmap{}, err
	}
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return sniPixmap{}, fmt.Errorf("sni: empty icon %dx%d", w, h)
	}
	if w > maxIconDim || h > maxIconDim {
		return sniPixmap{}, fmt.Errorf(
			"sni: icon too large %dx%d (max %d)",
			w, h, maxIconDim)
	}
	data := make([]byte, w*h*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			off := ((y-bounds.Min.Y)*w + (x - bounds.Min.X)) * 4
			// SNI: ARGB32 network byte order (big-endian).
			data[off+0] = byte(a >> 8)
			data[off+1] = byte(r >> 8)
			data[off+2] = byte(g >> 8)
			data[off+3] = byte(b >> 8)
		}
	}
	return sniPixmap{
		Width:  int32(w),
		Height: int32(h),
		Data:   data,
	}, nil
}

// --- Menu tree flattening ---

// buildMenuNodes converts NativeMenuItemCfg tree to a flat slice
// with sequential IDs. ID 0 is the virtual root.
func buildMenuNodes(
	items []gui.NativeMenuItemCfg,
) []menuNode {
	nodes := []menuNode{{dbusID: 0}} // root node
	appendItems(items, 0, &nodes)
	// Patch root children range.
	nodes[0].childStart = 1
	nodes[0].childCount = len(items)
	return nodes
}

func appendItems(
	items []gui.NativeMenuItemCfg,
	_ int,
	nodes *[]menuNode,
) {
	baseIdx := len(*nodes)
	// Reserve slots for this level.
	for range items {
		*nodes = append(*nodes, menuNode{})
	}

	for i, item := range items {
		idx := baseIdx + i
		n := &(*nodes)[idx]
		n.dbusID = int32(idx)
		n.actionID = item.ID
		n.label = item.Text
		n.separator = item.Separator
		n.disabled = item.Disabled
		n.checked = item.Checked

		if len(item.Submenu) > 0 {
			childStart := len(*nodes)
			appendItems(item.Submenu, idx, nodes)
			// Re-derive pointer after possible realloc.
			n = &(*nodes)[idx]
			n.childStart = childStart
			n.childCount = len(item.Submenu)
		}
	}
}

// buildLayout builds the recursive GetLayout response for a
// given node. dbusID == slice index, so direct indexing is used.
func buildLayout(
	nodes []menuNode,
	id int32,
	maxDepth int32,
	curDepth int32,
) menuLayout {
	if int(id) < 0 || int(id) >= len(nodes) {
		return menuLayout{ID: id}
	}
	node := &nodes[id]

	props := menuNodeProps(node)
	var children []dbus.Variant

	if node.childCount > 0 &&
		(maxDepth < 0 || curDepth < maxDepth) {
		children = make([]dbus.Variant, 0, node.childCount)
		for ci := range node.childCount {
			childIdx := node.childStart + ci
			if childIdx >= len(nodes) {
				break
			}
			child := buildLayout(nodes,
				int32(childIdx),
				maxDepth, curDepth+1)
			children = append(children,
				dbus.MakeVariant(child))
		}
	}

	return menuLayout{
		ID:         id,
		Properties: props,
		Children:   children,
	}
}

func menuNodeProps(node *menuNode) map[string]dbus.Variant {
	props := make(map[string]dbus.Variant, 4)

	if node.dbusID == 0 {
		// Root: children-display only.
		props["children-display"] = dbus.MakeVariant("submenu")
		return props
	}

	if node.separator {
		props["type"] = dbus.MakeVariant("separator")
		return props
	}

	// Escape underscores for DBusMenu mnemonic format.
	label := strings.ReplaceAll(node.label, "_", "__")
	props["label"] = dbus.MakeVariant(label)
	props["enabled"] = dbus.MakeVariant(!node.disabled)
	props["visible"] = dbus.MakeVariant(true)

	if node.checked {
		props["toggle-type"] = dbus.MakeVariant("checkmark")
		props["toggle-state"] = dbus.MakeVariant(int32(1))
	}

	if node.childCount > 0 {
		props["children-display"] = dbus.MakeVariant("submenu")
	}

	return props
}

// --- D-Bus helpers ---

func openPrivateConn() (*dbus.Conn, error) {
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return nil, err
	}
	if err = conn.Auth(nil); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err = conn.Hello(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func exportIntrospect(conn *dbus.Conn) {
	sniIntro := introspect.Node{
		Name: sniPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{Name: sniIface},
			{Name: propsIface},
		},
	}
	_ = conn.Export(
		introspect.NewIntrospectable(&sniIntro),
		sniPath,
		"org.freedesktop.DBus.Introspectable",
	)

	menuIntro := introspect.Node{
		Name: menuPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{Name: menuIface},
		},
	}
	_ = conn.Export(
		introspect.NewIntrospectable(&menuIntro),
		menuPath,
		"org.freedesktop.DBus.Introspectable",
	)
}
