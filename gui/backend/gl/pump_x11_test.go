//go:build linux && !js && !android

package gl

import (
	"testing"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// fakeXResult is one scripted WaitForEvent return.
type fakeXResult struct {
	ev  xgb.Event
	err xgb.Error
}

// fakeXSource replays a fixed script of WaitForEvent results, then reports a
// closed connection (nil, nil) the way xgb does once its eventChan closes.
type fakeXSource struct{ script []fakeXResult }

func (f *fakeXSource) WaitForEvent() (xgb.Event, xgb.Error) {
	if len(f.script) == 0 {
		return nil, nil
	}
	r := f.script[0]
	f.script = f.script[1:]
	return r.ev, r.err
}

// Issue #701: the pump reads only the source it was handed, never the
// platformState.conn field that destroy() nils on shutdown. It forwards
// events, skips X errors without stopping, and closes ch when the source
// reports the connection closed.
func TestPumpXEvents(t *testing.T) {
	src := &fakeXSource{script: []fakeXResult{
		{ev: xproto.ExposeEvent{Count: 0}},
		{err: xproto.WindowError{}}, // an X error must not end the pump
		{ev: xproto.ExposeEvent{Count: 1}},
	}}
	ch := make(chan xgb.Event, 4)

	done := make(chan struct{})
	go func() {
		defer close(done)
		pumpXEvents(src, ch)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pumpXEvents did not return after the source closed")
	}

	var got []uint16
	for ev := range ch { // ranging to the end also proves ch was closed
		got = append(got, ev.(xproto.ExposeEvent).Count)
	}
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("forwarded Expose counts %v, want [0 1]", got)
	}
}

// Issue #701: in RunApp, events queued for a window before closeWindow ran
// must not reach its destroyed backend. Destroy zeroes plat.window and
// closeWindow deletes the map entry; either alone marks the backend dead.
func TestLiveBackend(t *testing.T) {
	live := &Backend{}
	live.plat.window = 7
	dead := &Backend{} // plat.window zeroed by destroy()
	backends := map[uint32]*Backend{7: live}

	cases := []struct {
		name string
		te   taggedEvent
		want bool
	}{
		{"live backend", taggedEvent{b: live}, true},
		{"closed marker", taggedEvent{b: live, closed: true}, false},
		{"destroyed backend", taggedEvent{b: dead}, false},
	}
	for _, c := range cases {
		if got := liveBackend(backends, c.te); got != c.want {
			t.Errorf("%s: liveBackend=%v, want %v", c.name, got, c.want)
		}
	}

	// A backend removed from the map but whose window id was not yet
	// zeroed is dead too: the map, not the id, is the authority.
	delete(backends, 7)
	if liveBackend(backends, taggedEvent{b: live}) {
		t.Error("unregistered backend reported live")
	}
}
