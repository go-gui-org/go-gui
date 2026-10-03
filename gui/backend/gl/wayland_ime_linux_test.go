//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

func TestWlPreeditRange(t *testing.T) {
	for _, c := range []struct {
		text       string
		begin, end int32
		start, n   int32
	}{
		{"nihao", 0, 2, 0, 2},
		{"nihao", 5, 5, 5, 0},
		{"nihao", -1, -1, 5, 0}, // hidden cursor: at the end, no clause
		{"你好", 3, 6, 1, 1},      // byte offsets → characters
		{"你好", 4, 6, 1, 1},      // inside a character: cut back to its start
		{"你好", 0, 100, 0, 2},    // past the end: clamped
		{"abc", 2, 1, 1, 1},     // reversed
	} {
		s, n := wlPreeditRange(c.text, c.begin, c.end)
		if s != c.start || n != c.n {
			t.Errorf("wlPreeditRange(%q, %d, %d) = %d, %d; want %d, %d",
				c.text, c.begin, c.end, s, n, c.start, c.n)
		}
	}
}

func TestWlIMEDone(t *testing.T) {
	m := &wlIME{}
	type ev struct {
		typ  gui.EventType
		text string
	}
	got := func(out []gui.Event) []ev {
		var r []ev
		for _, e := range out {
			r = append(r, ev{e.Type, e.IMEText})
		}
		return r
	}
	same := func(a, b []ev) bool {
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
	comp, char := gui.EventIMEComposition, gui.EventChar

	// A preedit shows.
	m.preedit, m.begin, m.end = "ni", 0, 2
	if r := got(m.done(nil)); !same(r, []ev{{comp, "ni"}}) {
		t.Fatalf("preedit: %v", r)
	}
	// The commit replaces it: the preedit goes first, then the text.
	m.commit = "你好"
	if r := got(m.done(nil)); !same(r, []ev{{comp, ""}, {char, "你好"}}) {
		t.Fatalf("commit: %v", r)
	}
	// A commit with no preedit showing is just the text.
	m.commit = "x"
	if r := got(m.done(nil)); !same(r, []ev{{char, "x"}}) {
		t.Fatalf("plain commit: %v", r)
	}
	// Commit and a new preedit in one done.
	m.commit, m.preedit = "a", "b"
	if r := got(m.done(nil)); !same(r, []ev{{char, "a"}, {comp, "b"}}) {
		t.Fatalf("commit+preedit: %v", r)
	}
	// A done with no preedit_string clears the preedit.
	if r := got(m.done(nil)); !same(r, []ev{{comp, ""}}) {
		t.Fatalf("cleared: %v", r)
	}
	// Nothing pending, nothing showing: nothing.
	if r := m.done(nil); len(r) != 0 {
		t.Fatalf("idle done: %v", got(r))
	}
	// Invalid UTF-8 from the input method is dropped.
	m.commit = "\xff\xfe"
	if r := m.done(nil); len(r) != 0 {
		t.Fatalf("garbage commit: %v", got(r))
	}
}

// TestWaylandIME runs text-input-v3 against an input method this test plays
// itself, through input-method-v2 on its own connection (sway).
func TestWaylandIME(t *testing.T) {
	// gui drops IME events with no editable widget focused, so the test
	// reads what the seat emitted for the last done (wlIME.events).
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	t.Cleanup(b.Destroy)
	ww := b.plat.wl
	d := ww.d
	if d.seat == nil || !d.seat.ime.ti.Valid() {
		t.Skip("no seat or no text-input-v3")
	}
	reg := d.conn.Display.GetRegistry()
	var mgr wl.ZwpInputMethodManagerV2
	reg.SetHandlers(wl.RegistryHandlers{Global: func(name uint32, iface string, _ uint32) {
		if iface == "zwp_input_method_manager_v2" {
			mgr = wl.ZwpInputMethodManagerV2{Proxy: reg.Bind(name, &wl.ZwpInputMethodManagerV2Interface, 1)}
		}
	}})
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	reg.DestroyProxy()
	if !mgr.Valid() {
		t.Skip("no input-method-v2")
	}
	var active bool
	var dones uint32
	im := mgr.GetInputMethod(d.seat.seat)
	im.SetHandlers(wl.ZwpInputMethodV2Handlers{
		Activate:   func() { active = true },
		Deactivate: func() { active = false },
		Done:       func() { dones++ },
	})
	t.Cleanup(func() { im.Destroy(); mgr.Destroy(); _ = d.conn.Roundtrip() })

	settleWindow(t, b)
	waitFor(t, d, "text input enter", func() bool { return d.seat.ime.focus == b })
	ww.imeStart()
	ww.imeSetRect(10, 20, 0, 16)
	waitFor(t, d, "input method activation", func() bool { return active && dones > 0 })
	if !d.seat.ime.enabled {
		t.Fatal("text input not enabled")
	}

	m := &d.seat.ime
	im.SetPreeditString("ni", 0, 2)
	im.Commit(dones)
	waitFor(t, d, "preedit", func() bool { return m.showPreedit })
	if len(m.events) != 1 {
		t.Fatalf("preedit made %d events", len(m.events))
	}
	if e := m.events[0]; e.Type != gui.EventIMEComposition || e.IMEText != "ni" || e.IMEStart != 0 || e.IMELength != 2 {
		t.Fatalf("preedit event %+v", e)
	}

	im.CommitString("你好")
	im.Commit(dones)
	waitFor(t, d, "commit", func() bool { return !m.showPreedit })
	if len(m.events) != 2 {
		t.Fatalf("commit made %d events", len(m.events))
	}
	if e := m.events[0]; e.Type != gui.EventIMEComposition || e.IMEText != "" {
		t.Errorf("event before the commit: %+v, want the preedit cleared", e)
	}
	if e := m.events[1]; e.Type != gui.EventChar || e.IMEText != "你好" || e.CharCode != '你' {
		t.Errorf("commit event %+v", e)
	}

	// The caret moving is reported; the same caret again sends nothing.
	ww.imeSetRect(30, 20, 0, 16)
	ww.imeSetRect(30, 20, 0, 16)
	ww.imeStop()
	waitFor(t, d, "input method deactivation", func() bool { return !active })
	if d.seat.ime.enabled {
		t.Fatal("text input still enabled after IMEStop")
	}
}
