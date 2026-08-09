package gui

import (
	"sync"
	"testing"
	"time"
)

// testState is a minimal Snapshotter used across ring tests.
type testState struct {
	counter int
	text    string
}

func (s *testState) Snapshot() any {
	return &testState{counter: s.counter, text: s.text}
}

func (s *testState) Restore(v any) {
	src := v.(*testState)
	s.counter = src.counter
	s.text = src.text
}

// sizedState reports its own byte cost to exercise snapshotSizer.
type sizedState struct {
	size int
}

func (s *sizedState) Snapshot() any { return &sizedState{size: s.size} }
func (s *sizedState) Restore(v any) { *s = *v.(*sizedState) }
func (s *sizedState) Size() int     { return s.size }

// TestSnapshotRingPush verifies basic push/len/bytes/at/last.
func TestSnapshotRingPush(t *testing.T) {
	r := newSnapshotRing(1 << 20)
	now := time.Now()
	r.push((&testState{counter: 1}).Snapshot(), now, "evt-1")
	r.push((&testState{counter: 2}).Snapshot(), now.Add(time.Millisecond), "evt-2")
	if got := r.len(); got != 2 {
		t.Fatalf("len = %d, want 2", got)
	}
	if got := r.bytes(); got != 2*snapshotDefaultSize {
		t.Fatalf("bytes = %d, want %d", got, 2*snapshotDefaultSize)
	}
	e, ok := r.at(0)
	if !ok || e.cause != "evt-1" {
		t.Fatalf("at(0) = %+v ok=%v", e, ok)
	}
	e, ok = r.last()
	if !ok || e.cause != "evt-2" {
		t.Fatalf("last = %+v ok=%v", e, ok)
	}
	if _, ok := r.at(5); ok {
		t.Fatal("at(5) should fail")
	}
}

// TestSnapshotRingRestoreRoundTrip ensures Snapshot/Restore
// round-trips value fields through a ring entry.
func TestSnapshotRingRestoreRoundTrip(t *testing.T) {
	s := &testState{counter: 7, text: "hello"}
	r := newSnapshotRing(1 << 20)
	r.push(s.Snapshot(), time.Now(), "init")
	s.counter = 42
	s.text = "changed"
	e, ok := r.at(0)
	if !ok {
		t.Fatal("missing entry")
	}
	s.Restore(e.snap)
	if s.counter != 7 || s.text != "hello" {
		t.Fatalf("restored = %+v, want counter=7 text=hello", s)
	}
}

// TestSnapshotRingByteCap confirms eviction keeps total bytes
// under the cap.
func TestSnapshotRingByteCap(t *testing.T) {
	const cap = 1024
	r := newSnapshotRing(cap)
	for range 10 {
		r.push(&sizedState{size: 400}, time.Now(), "e")
	}
	if got := r.bytes(); got > cap {
		t.Fatalf("bytes = %d, want <= %d", got, cap)
	}
	// 400 * 2 = 800 fits under 1024; 400 * 3 = 1200 does not.
	if got := r.len(); got != 2 {
		t.Fatalf("len = %d, want 2 (400-byte entries under 1024 cap)", got)
	}
}

// TestSnapshotRingEvictsOldest checks that eviction is FIFO:
// the surviving entries are the newest ones.
func TestSnapshotRingEvictsOldest(t *testing.T) {
	r := newSnapshotRing(2 * snapshotDefaultSize)
	r.push((&testState{counter: 1}).Snapshot(), time.Now(), "a")
	r.push((&testState{counter: 2}).Snapshot(), time.Now(), "b")
	r.push((&testState{counter: 3}).Snapshot(), time.Now(), "c")
	if got := r.len(); got != 2 {
		t.Fatalf("len = %d, want 2", got)
	}
	e0, _ := r.at(0)
	e1, _ := r.at(1)
	if e0.cause != "b" || e1.cause != "c" {
		t.Fatalf("survivors = %q, %q, want b, c", e0.cause, e1.cause)
	}
}

// TestSnapshotRingNilSnap ignores nil pushes.
func TestSnapshotRingNilSnap(t *testing.T) {
	r := newSnapshotRing(1 << 20)
	r.push(nil, time.Now(), "nope")
	if r.len() != 0 {
		t.Fatalf("nil push should not add entry")
	}
}

// TestSnapshotRingDefaultCap applies defaultHistoryBytes when
// maxBytes is non-positive.
func TestSnapshotRingDefaultCap(t *testing.T) {
	r := newSnapshotRing(0)
	if r.maxBytes != defaultHistoryBytes {
		t.Fatalf("maxBytes = %d, want %d", r.maxBytes, defaultHistoryBytes)
	}
}

// TestSnapshotRingSizerZeroFallback treats Size()<=0 as the
// default estimate, so a misbehaving Size() cannot starve the
// byte cap.
func TestSnapshotRingSizerZeroFallback(t *testing.T) {
	r := newSnapshotRing(2 * snapshotDefaultSize)
	r.push(&sizedState{size: 0}, time.Now(), "z")
	if got := r.bytes(); got != snapshotDefaultSize {
		t.Fatalf("bytes = %d, want %d", got, snapshotDefaultSize)
	}
}

// TestEventCaptureRecordsHistory confirms EventFn pushes a
// snapshot when history is enabled and state implements
// Snapshotter.
func TestEventCaptureRecordsHistory(t *testing.T) {
	w := &Window{state: &testState{counter: 1}, focused: true}
	w.history = newSnapshotRing(1 << 20)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	if got := w.history.len(); got != 1 {
		t.Fatalf("len = %d, want 1", got)
	}
	e, _ := w.history.last()
	if e.cause != "mouse-down" {
		t.Fatalf("cause = %q, want mouse-down", e.cause)
	}
}

// TestEventCaptureSkipsMouseMove confirms plain motion events
// are skipped to keep the timeline usable.
func TestEventCaptureSkipsMouseMove(t *testing.T) {
	w := &Window{state: &testState{}, focused: true}
	w.history = newSnapshotRing(1 << 20)
	w.EventFn(&Event{Type: EventMouseMove})
	if got := w.history.len(); got != 0 {
		t.Fatalf("len = %d, want 0 (MouseMove skipped)", got)
	}
}

// TestEventCaptureSkipsWhenDisabled confirms a nil history
// means zero snapshots and zero overhead.
func TestEventCaptureSkipsWhenDisabled(t *testing.T) {
	w := &Window{state: &testState{}, focused: true}
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	if w.history != nil {
		t.Fatal("history should stay nil")
	}
}

// TestEventCaptureNonSnapshotter leaves history empty when
// user state does not implement Snapshotter.
func TestEventCaptureNonSnapshotter(t *testing.T) {
	type plain struct{}
	w := &Window{state: &plain{}, focused: true}
	w.history = newSnapshotRing(1 << 20)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	if got := w.history.len(); got != 0 {
		t.Fatalf("len = %d, want 0 (state is not Snapshotter)", got)
	}
}

// TestEventFrozenDropsEvents verifies the freeze atomic blocks
// all user input.
func TestEventFrozenDropsEvents(t *testing.T) {
	w := &Window{state: &testState{}, focused: true}
	w.history = newSnapshotRing(1 << 20)
	w.frozen.Store(true)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	w.EventFn(&Event{Type: EventKeyDown})
	if got := w.history.len(); got != 0 {
		t.Fatalf("len = %d, want 0 (frozen)", got)
	}
}

// TestFreezeResume toggles the atomic and confirms EventFn
// resumes accepting events after Resume.
func TestFreezeResume(t *testing.T) {
	w := &Window{state: &testState{}, focused: true}
	w.history = newSnapshotRing(1 << 20)
	w.Freeze()
	if !w.isFrozen() {
		t.Fatal("expected frozen")
	}
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	if got := w.history.len(); got != 0 {
		t.Fatalf("frozen len = %d, want 0", got)
	}
	w.resume()
	if w.isFrozen() {
		t.Fatal("expected unfrozen")
	}
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	if got := w.history.len(); got != 1 {
		t.Fatalf("resumed len = %d, want 1", got)
	}
}

// TestFreezeDisabledNoOp confirms Freeze is a no-op when
// history was never enabled.
func TestFreezeDisabledNoOp(t *testing.T) {
	w := &Window{state: &testState{}, focused: true}
	w.Freeze()
	if w.isFrozen() {
		t.Fatal("Freeze should be no-op without history")
	}
}

// TestPostRestoreRoundTrip posts a restore through the
// command queue, flushes, and confirms state reverts and the
// virtual clock pins to the entry's timestamp.
func TestPostRestoreRoundTrip(t *testing.T) {
	s := &testState{counter: 1, text: "a"}
	w := &Window{state: s, focused: true}
	w.history = newSnapshotRing(1 << 20)

	pinned := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w.history.push(s.Snapshot(), pinned, "init")

	s.counter = 99
	s.text = "mutated"

	w.Freeze()
	w.postRestore(0)
	w.flushCommands()

	if s.counter != 1 || s.text != "a" {
		t.Fatalf("after restore: %+v, want {1, a}", s)
	}
	if got := w.Now(); !got.Equal(pinned) {
		t.Fatalf("virtualNow = %v, want %v", got, pinned)
	}

	w.resume()
	if w.virtualNow.Load() != nil {
		t.Fatal("Resume should clear virtualNow")
	}
}

// TestPostRestoreOutOfRange silently no-ops.
func TestPostRestoreOutOfRange(t *testing.T) {
	s := &testState{counter: 7}
	w := &Window{state: s}
	w.history = newSnapshotRing(1 << 20)
	w.postRestore(42)
	w.flushCommands()
	if s.counter != 7 {
		t.Fatalf("counter mutated on bad idx: %d", s.counter)
	}
}

// TestPostRestoreDisabledNoOp confirms PostRestore is a no-op
// when history is nil — and does not queue a command.
func TestPostRestoreDisabledNoOp(t *testing.T) {
	w := &Window{state: &testState{}}
	w.postRestore(0)
	if len(w.commands) != 0 {
		t.Fatalf("queued %d commands with disabled history", len(w.commands))
	}
}

// TestNamespaceSnapshotRoundTrip captures a whitelisted
// namespace's BoundedMap and restores it after mutation.
func TestNamespaceSnapshotRoundTrip(t *testing.T) {
	const ns = "test.snap.ns"
	t.Cleanup(func() {
		snapshotableNamespaces.mu.Lock()
		delete(snapshotableNamespaces.set, ns)
		snapshotableNamespaces.mu.Unlock()
	})
	registerNamespaceSnapshot(ns)

	w := &Window{state: &testState{}, focused: true}
	w.history = newSnapshotRing(1 << 20)
	sm := StateMap[string, int](w, ns, capFew)
	sm.Set("scroll", 42)

	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})

	sm.Set("scroll", 999)
	if got, _ := sm.Get("scroll"); got != 999 {
		t.Fatalf("pre-restore Get = %d, want 999", got)
	}

	w.postRestore(0)
	w.flushCommands()

	if got, _ := sm.Get("scroll"); got != 42 {
		t.Fatalf("post-restore Get = %d, want 42", got)
	}
}

// TestNamespaceNotWhitelisted leaves unregistered namespaces
// alone — they stay at their live value after restore.
func TestNamespaceNotWhitelisted(t *testing.T) {
	const ns = "test.not.whitelisted"
	w := &Window{state: &testState{}, focused: true}
	w.history = newSnapshotRing(1 << 20)
	sm := StateMap[string, int](w, ns, capFew)
	sm.Set("k", 1)

	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})

	sm.Set("k", 2)
	w.postRestore(0)
	w.flushCommands()

	if got, _ := sm.Get("k"); got != 2 {
		t.Fatalf("non-whitelisted ns should not rewind: got %d, want 2", got)
	}
}

// TestRegisterNamespaceSnapshotEmpty silently ignores empty.
func TestRegisterNamespaceSnapshotEmpty(t *testing.T) {
	registerNamespaceSnapshot("")
	if isNamespaceSnapshotable("") {
		t.Fatal("empty namespace should not be whitelisted")
	}
}

// TestScrollNamespacesPreregistered confirms the default
// whitelist includes scroll and focus namespaces.
func TestScrollNamespacesPreregistered(t *testing.T) {
	for _, ns := range []string{
		nsScrollX, nsScrollY, nsInputFocus,
		nsListBoxFocus, nsTreeFocus,
	} {
		if !isNamespaceSnapshotable(ns) {
			t.Fatalf("%s should be in default whitelist", ns)
		}
	}
}

// TestSnapshotRingConcurrent exercises the RWMutex under -race.
func TestSnapshotRingConcurrent(t *testing.T) {
	r := newSnapshotRing(1 << 20)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 1000 {
			r.push((&testState{counter: i}).Snapshot(), time.Now(), "w")
		}
	}()
	go func() {
		defer wg.Done()
		for range 1000 {
			_ = r.len()
			_ = r.bytes()
			_, _ = r.at(0)
			_, _ = r.last()
		}
	}()
	wg.Wait()
}
