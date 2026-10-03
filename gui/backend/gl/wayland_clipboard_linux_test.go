//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/go-gui-org/go-gui/gui"
)

func TestWlMimeRank(t *testing.T) {
	o := &wlOffer{}
	for _, m := range []string{"image/png", "text/plain", "UTF8_STRING", "text/html"} {
		o.addMime(m)
	}
	if o.mime != "UTF8_STRING" {
		t.Errorf("best of the offer = %q, want UTF8_STRING", o.mime)
	}
	o.addMime("text/plain;charset=utf-8")
	o.addMime("STRING")
	if o.mime != "text/plain;charset=utf-8" {
		t.Errorf("best = %q, want text/plain;charset=utf-8", o.mime)
	}
	n := &wlOffer{}
	n.addMime("image/png")
	if n.mime != "" {
		t.Errorf("an offer without text took %q", n.mime)
	}
}

func pipe(t *testing.T) (r, w int) {
	t.Helper()
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	return p[0], p[1]
}

func TestWlServeAndReadText(t *testing.T) {
	r, w := pipe(t)
	text := strings.Repeat("héllo ", 50000) // > one pipe buffer and one read page
	go wlServeText(w, text)
	got, err := wlReadText(r, time.Second, 1<<30)
	if err != nil || got != text {
		t.Fatalf("read %d bytes (err %v), want %d", len(got), err, len(text))
	}
}

func TestWlReadTextLimit(t *testing.T) {
	r, w := pipe(t)
	go wlServeText(w, strings.Repeat("x", 1000))
	got, err := wlReadText(r, time.Second, 100)
	if err != nil || got != strings.Repeat("x", 100) {
		t.Fatalf("got %d bytes (err %v), want the first 100", len(got), err)
	}
}

func TestWlReadTextTimeout(t *testing.T) {
	r, w := pipe(t)
	defer func() { _ = unix.Close(w) }()
	if _, err := unix.Write(w, []byte("part")); err != nil {
		t.Fatal(err)
	}
	// The writer never closes: the read must give up, keeping what came.
	start := time.Now()
	got, err := wlReadText(r, 100*time.Millisecond, 1<<20)
	if err == nil || got != "part" {
		t.Fatalf("got %q, err %v; want \"part\" and a timeout", got, err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("the timeout did not bound the read")
	}
}

// fakeOffer stands in for a protocol offer.
type fakeOffer struct {
	ptr       uintptr
	destroyed *[]uintptr
}

func (f fakeOffer) Receive(string, int) {}
func (f fakeOffer) Destroy()            { *f.destroyed = append(*f.destroyed, f.ptr) }
func (f fakeOffer) Ptr() uintptr        { return f.ptr }

func TestWlOfferLifetime(t *testing.T) {
	var destroyed []uintptr
	s := &wlSeat{clip: wlClip{offers: map[uintptr]*wlOffer{}}}
	off := func(p uintptr) fakeOffer { return fakeOffer{p, &destroyed} }

	s.trackOffer(off(1)).addMime("text/plain")
	s.setOffer(wlSelClipboard, 1)
	if o := s.clip.sel[wlSelClipboard].offer; o == nil || o.mime != "text/plain" {
		t.Fatalf("selection offer = %+v", o)
	}
	// A new selection frees the one it replaces.
	s.trackOffer(off(2))
	s.setOffer(wlSelClipboard, 2)
	if len(destroyed) != 1 || destroyed[0] != 1 {
		t.Fatalf("destroyed %v, want [1]", destroyed)
	}
	// The same offer again is not freed.
	s.setOffer(wlSelClipboard, 2)
	if len(destroyed) != 1 {
		t.Fatalf("destroyed %v after re-setting the same offer", destroyed)
	}
	// An empty selection frees the last.
	s.setOffer(wlSelClipboard, 0)
	if s.clip.sel[wlSelClipboard].offer != nil || len(destroyed) != 2 {
		t.Fatalf("empty selection: offer %v, destroyed %v", s.clip.sel[wlSelClipboard].offer, destroyed)
	}
	if len(s.clip.offers) != 0 {
		t.Fatalf("%d offers left", len(s.clip.offers))
	}
}

// fakeSource stands in for a protocol source.
type fakeSource struct{ n *int }

func (f fakeSource) Destroy() { *f.n++ }

func TestWlCancelSource(t *testing.T) {
	var destroyed int
	s := &wlSeat{}
	cur, old := fakeSource{&destroyed}, fakeSource{new(int)}
	s.clip.sel[wlSelPrimary] = wlSelection{source: cur, text: "mine"}
	// An old source's cancel leaves the current one.
	s.cancelSource(wlSelPrimary, old)
	if !s.clip.sel[wlSelPrimary].owned() || *old.n != 1 {
		t.Fatal("cancelling an old source dropped the current one")
	}
	s.cancelSource(wlSelPrimary, cur)
	sel := s.clip.sel[wlSelPrimary]
	if sel.owned() || sel.text != "" || destroyed != 1 {
		t.Fatalf("after cancel: owned %v text %q destroyed %d", sel.owned(), sel.text, destroyed)
	}
}

// TestWaylandClipboard copies and pastes both selections with another
// client (wl-copy, wl-paste), from a goroutine other than the loop's, as an
// app would.
func TestWaylandClipboard(t *testing.T) {
	requireInjection(t)
	for _, tool := range []string{"wl-copy", "wl-paste"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	t.Setenv("GOGUI_WAYLAND", "1")

	type result struct{ clip, primary, pasted, pastedPrimary string }
	res := make(chan result, 1)
	run := func(args ...string) string {
		out, err := exec.Command(args[0], args[1:]...).Output()
		if err != nil {
			t.Errorf("%v: %v", args, err)
		}
		return string(out)
	}
	// wl-copy forks a child that serves the selection until replaced.
	// It inherits a stdout pipe, so Output would wait for it forever:
	// it runs with no output.
	copyText := func(args ...string) {
		if err := exec.Command("wl-copy", args...).Run(); err != nil {
			t.Errorf("wl-copy %v: %v", args, err)
		}
	}
	steps := func(w *gui.Window, d *wlDisplay) {
		defer func() { w.Close(); d.conn.Wake() }()
		time.Sleep(500 * time.Millisecond)
		// A key press gives the window focus and an input serial,
		// which the compositor checks before taking a selection.
		run("wtype", "-s", "300", "x")
		copyText("from outside")
		copyText("-p", "primary outside")
		time.Sleep(200 * time.Millisecond)
		var r result
		r.clip, r.primary = w.GetClipboard(), w.GetPrimary()

		// A copy names the serial of an input event newer than the
		// current selection (wlroots refuses an older one), as a real
		// Ctrl+C does.
		run("wtype", "-s", "300", "x")
		w.SetClipboard("from go-gui")
		w.SetPrimary("primary go-gui")
		time.Sleep(200 * time.Millisecond)
		r.pasted = run("wl-paste", "-n")
		r.pastedPrimary = run("wl-paste", "-n", "-p")
		res <- r
	}
	w := waylandTestWindow(gui.WindowCfg{
		OnInit: func(w *gui.Window) { go steps(w, wlShared) },
	})
	done := make(chan error, 1)
	go func() { done <- runE(w) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return")
	}
	r := <-res
	for _, c := range []struct{ name, got, want string }{
		{"GetClipboard", r.clip, "from outside"},
		{"GetPrimary", r.primary, "primary outside"},
		{"wl-paste", r.pasted, "from go-gui"},
		{"wl-paste -p", r.pastedPrimary, "primary go-gui"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestWaylandClipboardLocal checks the fallback without a selection
// device: text stays in the process.
func TestWaylandClipboardLocal(t *testing.T) {
	requireWayland(t)
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	defer b.Destroy()
	d := b.plat.wl.d
	seat := d.seat
	d.seat = nil // as on a compositor with no seat
	defer func() { d.seat = seat }()
	b.plat.w.SetClipboard("local")
	b.plat.w.SetPrimary("local primary")
	if got := b.plat.w.GetClipboard(); got != "local" {
		t.Errorf("GetClipboard = %q", got)
	}
	if got := b.plat.w.GetPrimary(); got != "local primary" {
		t.Errorf("GetPrimary = %q", got)
	}
}

// TestWaylandClipboardStaleWindow checks a copy and a paste posted for a
// window that was destroyed before the loop ran them. Its thread id is
// gone, so work that checked the thread again re-posted itself on every
// pass (a spin), and a paste stalled the loop for 2×clipReadTimeout.
func TestWaylandClipboardStaleWindow(t *testing.T) {
	requireWayland(t)
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	defer b.Destroy()
	d := b.plat.wl.d
	seat := d.seat
	d.seat = nil // the in-process fallback, so no compositor is involved
	defer func() { d.seat = seat }()
	stale := &Backend{} // lockedTid 0, as after Destroy
	queued := func() int {
		d.callsMu.Lock()
		defer d.callsMu.Unlock()
		return len(d.calls)
	}

	d.setSelection(stale, wlSelClipboard, "copied")
	d.runPosted()
	if n := queued(); n != 0 {
		t.Fatalf("copy re-posted itself: %d calls queued", n)
	}
	if got := d.localSel[wlSelClipboard]; got != "copied" {
		t.Fatalf("copy not applied: %q", got)
	}

	got := make(chan string, 1)
	go func() { got <- d.getSelection(stale, wlSelClipboard) }()
	deadline := time.Now().Add(clipReadTimeout)
	for {
		start := time.Now()
		d.runPosted()
		if time.Since(start) > clipReadTimeout/2 {
			t.Fatal("paste blocked the loop")
		}
		select {
		case s := <-got:
			if s != "copied" {
				t.Fatalf("paste = %q", s)
			}
			if n := queued(); n != 0 {
				t.Fatalf("paste re-posted itself: %d calls queued", n)
			}
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("paste never answered")
		}
		time.Sleep(time.Millisecond)
	}
}
