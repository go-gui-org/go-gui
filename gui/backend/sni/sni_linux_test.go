//go:build linux

package sni

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/godbus/dbus/v5"
)

func TestBuildMenuNodesEmpty(t *testing.T) {
	nodes := buildMenuNodes(nil)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 root node, got %d", len(nodes))
	}
	if nodes[0].dbusID != 0 {
		t.Error("root dbusID should be 0")
	}
	if nodes[0].childCount != 0 {
		t.Errorf("root childCount: got %d, want 0",
			nodes[0].childCount)
	}
}

func TestBuildMenuNodesFlat(t *testing.T) {
	items := []gui.NativeMenuItemCfg{
		{ID: "show", Text: "Show Window"},
		{Separator: true},
		{ID: "quit", Text: "Quit"},
	}
	nodes := buildMenuNodes(items)
	// root + 3 items = 4
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}
	if nodes[0].childStart != 1 || nodes[0].childCount != 3 {
		t.Errorf("root children: start=%d count=%d",
			nodes[0].childStart, nodes[0].childCount)
	}
	if nodes[1].actionID != "show" || nodes[1].label != "Show Window" {
		t.Errorf("node 1: id=%q label=%q",
			nodes[1].actionID, nodes[1].label)
	}
	if !nodes[2].separator {
		t.Error("node 2 should be separator")
	}
	if nodes[3].actionID != "quit" {
		t.Errorf("node 3 actionID: got %q", nodes[3].actionID)
	}
}

func TestBuildMenuNodesSubmenu(t *testing.T) {
	items := []gui.NativeMenuItemCfg{
		{
			ID:   "file",
			Text: "File",
			Submenu: []gui.NativeMenuItemCfg{
				{ID: "new", Text: "New"},
				{ID: "open", Text: "Open"},
			},
		},
		{ID: "quit", Text: "Quit"},
	}
	nodes := buildMenuNodes(items)
	// root(0) + file(1) + quit(2) + new(3) + open(4) = 5
	if len(nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(nodes))
	}
	file := nodes[1]
	if file.childStart != 3 || file.childCount != 2 {
		t.Errorf("file children: start=%d count=%d",
			file.childStart, file.childCount)
	}
	if nodes[3].actionID != "new" {
		t.Errorf("node 3: got %q, want new", nodes[3].actionID)
	}
	if nodes[4].actionID != "open" {
		t.Errorf("node 4: got %q, want open", nodes[4].actionID)
	}
}

func TestBuildMenuNodesIDEqualsIndex(t *testing.T) {
	items := []gui.NativeMenuItemCfg{
		{ID: "a", Text: "A"},
		{ID: "b", Text: "B"},
	}
	nodes := buildMenuNodes(items)
	for i, n := range nodes {
		if int(n.dbusID) != i {
			t.Errorf("node %d: dbusID=%d", i, n.dbusID)
		}
	}
}

func TestMenuNodePropsRoot(t *testing.T) {
	node := &menuNode{dbusID: 0}
	props := menuNodeProps(node)
	if v, ok := props["children-display"]; !ok {
		t.Error("root should have children-display")
	} else if v.Value() != "submenu" {
		t.Errorf("children-display: got %v", v.Value())
	}
	if _, ok := props["label"]; ok {
		t.Error("root should not have label")
	}
}

func TestMenuNodePropsSeparator(t *testing.T) {
	node := &menuNode{dbusID: 1, separator: true}
	props := menuNodeProps(node)
	if v, ok := props["type"]; !ok || v.Value() != "separator" {
		t.Error("separator should have type=separator")
	}
	if _, ok := props["label"]; ok {
		t.Error("separator should not have label")
	}
}

func TestMenuNodePropsRegular(t *testing.T) {
	node := &menuNode{
		dbusID: 1, label: "Open_File", disabled: false,
	}
	props := menuNodeProps(node)
	if v := props["label"].Value().(string); v != "Open__File" {
		t.Errorf("label: got %q, want Open__File", v)
	}
	if v := props["enabled"].Value().(bool); !v {
		t.Error("enabled should be true")
	}
}

func TestMenuNodePropsDisabled(t *testing.T) {
	node := &menuNode{dbusID: 1, label: "X", disabled: true}
	props := menuNodeProps(node)
	if v := props["enabled"].Value().(bool); v {
		t.Error("enabled should be false for disabled item")
	}
}

func TestMenuNodePropsChecked(t *testing.T) {
	node := &menuNode{dbusID: 1, label: "X", checked: true}
	props := menuNodeProps(node)
	if v, ok := props["toggle-type"]; !ok ||
		v.Value() != "checkmark" {
		t.Error("checked item should have toggle-type=checkmark")
	}
	if v, ok := props["toggle-state"]; !ok ||
		v.Value() != int32(1) {
		t.Error("checked item should have toggle-state=1")
	}
}

func TestMenuNodePropsWithChildren(t *testing.T) {
	node := &menuNode{
		dbusID: 1, label: "Sub", childCount: 2,
	}
	props := menuNodeProps(node)
	if _, ok := props["children-display"]; !ok {
		t.Error("node with children should have children-display")
	}
}

func TestBuildLayoutRoot(t *testing.T) {
	items := []gui.NativeMenuItemCfg{
		{ID: "a", Text: "Alpha"},
		{ID: "b", Text: "Beta"},
	}
	nodes := buildMenuNodes(items)
	layout := buildLayout(nodes, 0, -1, 0)
	if layout.ID != 0 {
		t.Errorf("root ID: got %d", layout.ID)
	}
	if len(layout.Children) != 2 {
		t.Fatalf("root children: got %d, want 2",
			len(layout.Children))
	}
	// Verify child variant contains menuLayout.
	child := layout.Children[0].Value().(menuLayout)
	if child.ID != 1 {
		t.Errorf("first child ID: got %d", child.ID)
	}
}

func TestBuildLayoutDepthLimit(t *testing.T) {
	items := []gui.NativeMenuItemCfg{
		{
			ID:   "parent",
			Text: "Parent",
			Submenu: []gui.NativeMenuItemCfg{
				{ID: "child", Text: "Child"},
			},
		},
	}
	nodes := buildMenuNodes(items)
	// maxDepth=1: root and its direct children, but not
	// grandchildren.
	layout := buildLayout(nodes, 0, 1, 0)
	if len(layout.Children) != 1 {
		t.Fatalf("root children: got %d", len(layout.Children))
	}
	parent := layout.Children[0].Value().(menuLayout)
	if len(parent.Children) != 0 {
		t.Errorf("depth-limited parent should have 0 children, got %d",
			len(parent.Children))
	}
}

func TestBuildLayoutOutOfBounds(t *testing.T) {
	nodes := buildMenuNodes(nil)
	layout := buildLayout(nodes, 99, -1, 0)
	if layout.ID != 99 {
		t.Errorf("ID: got %d", layout.ID)
	}
	if layout.Properties != nil {
		t.Error("out-of-bounds should have nil properties")
	}
}

func TestPngToPixmap(t *testing.T) {
	// Create a 2x2 red PNG.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	red := color.RGBA{R: 255, A: 255}
	for y := range 2 {
		for x := range 2 {
			img.SetRGBA(x, y, red)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	px, err := pngToPixmap(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if px.Width != 2 || px.Height != 2 {
		t.Errorf("size: %dx%d", px.Width, px.Height)
	}
	if len(px.Data) != 2*2*4 {
		t.Fatalf("data len: got %d, want 16", len(px.Data))
	}
	// First pixel: ARGB = [255, 255, 0, 0].
	if px.Data[0] != 255 || px.Data[1] != 255 ||
		px.Data[2] != 0 || px.Data[3] != 0 {
		t.Errorf("pixel 0: got %v", px.Data[:4])
	}
}

func TestPngToPixmapInvalidData(t *testing.T) {
	_, err := pngToPixmap([]byte("not a png"))
	if err == nil {
		t.Error("expected error for invalid PNG")
	}
}

func TestEncodePixmaps(t *testing.T) {
	in := []sniPixmap{
		{Width: 16, Height: 16, Data: make([]byte, 16*16*4)},
	}
	out := encodePixmaps(in)
	if len(out) != 1 {
		t.Fatalf("expected 1, got %d", len(out))
	}
	if out[0].Width != 16 || out[0].Height != 16 {
		t.Error("dimension mismatch")
	}
}

func TestGroupPropEntryDBusType(t *testing.T) {
	// Verify groupPropEntry can be wrapped in dbus.Variant
	// without panic.
	e := groupPropEntry{
		ID:    1,
		Props: map[string]dbus.Variant{"label": dbus.MakeVariant("X")},
	}
	v := dbus.MakeVariant(e)
	if v.Value() == nil {
		t.Error("variant should not be nil")
	}
}
