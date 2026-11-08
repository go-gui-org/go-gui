package memtex

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func px(w, h int, fill byte) []byte {
	p := make([]byte, w*h*4)
	for i := range p {
		p[i] = fill
	}
	return p
}

func TestDueOnlyAfterRemoval(t *testing.T) {
	var s Sweeper
	s.Due() // catch up with whatever earlier tests left behind
	if s.Due() {
		t.Fatal("Due with no removal since the last call")
	}
	gui.UseImage("memtex/due", 1, 1, px(1, 1, 1))
	if s.Due() {
		t.Error("registering a new key made Due true; only removals should")
	}
	gui.DropImage("memtex/due")
	if !s.Due() {
		t.Fatal("Due false after DropImage")
	}
	if s.Due() {
		t.Error("Due stayed true without another removal")
	}
}

func TestCurrent(t *testing.T) {
	const key = "memtex/current"
	t.Cleanup(func() { gui.DropImage(key) })
	src := gui.UseImage(key, 1, 1, px(1, 1, 1))
	mi, ok := gui.LookupImage(src)
	if !ok {
		t.Fatal("not registered")
	}
	if !Current(src, mi.Serial) {
		t.Error("texture of the live registration reported stale")
	}
	if Current(src, mi.Serial+1) {
		t.Error("texture with another serial reported current")
	}
	if !Current(`C:\photos\a.png`, 0) || !Current("https://example.com/a.png", 0) {
		t.Error("a non-mem key must always stay")
	}

	gui.UseImage(key, 2, 2, px(2, 2, 2)) // registered again at a new size
	if Current(src, mi.Serial) {
		t.Error("texture of a replaced registration reported current")
	}
	gui.DropImage(key)
	if Current(src, mi.Serial) {
		t.Error("texture of a dropped image reported current")
	}
}
