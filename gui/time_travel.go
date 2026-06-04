package gui

import (
	"log"
	"sync"
	"time"
)

// Snapshotter is implemented by user state types that opt into
// time-travel debugging. Snapshot returns a deep-ish copy of the
// receiver; Restore overwrites the receiver from a prior
// Snapshot result. Optional Size reports an approximate byte
// cost for the byte-capped ring buffer; unimplemented types
// use snapshotDefaultSize.
type Snapshotter interface {
	Snapshot() any
	Restore(any)
}

// snapshotSizer is the optional Size() int extension to
// Snapshotter. Types that can cheaply report their heap cost
// implement it to improve byte-cap eviction accuracy.
type snapshotSizer interface {
	Size() int
}

// snapshotDefaultSize is the per-entry byte estimate used when a
// Snapshot value does not implement snapshotSizer. A conservative
// guess; users with fat state should implement Size().
const snapshotDefaultSize = 1024

// snapshotMaxEntryBytes caps a single entry's reported size so a
// misbehaving (or adversarial) Snapshotter.Size() cannot overflow
// the ring's totalBytes counter. 1 GiB is far above any realistic
// single snapshot and well clear of int32 overflow on 32-bit
// platforms.
const snapshotMaxEntryBytes = 1 << 30

// snapshotEntry records a single point in history.
type snapshotEntry struct {
	snap       any
	namespaces map[string]any
	when       time.Time
	cause      string
	bytes      int
}

// snapshotableNamespaces is the set of StateMap namespaces
// whose BoundedMap contents are captured alongside user state
// during time-travel scrub. Default: scroll positions and
// widget-local focus rings. Users / widget authors can extend
// via RegisterNamespaceSnapshot.
var snapshotableNamespaces = struct {
	mu  sync.RWMutex
	set map[string]struct{}
}{
	set: map[string]struct{}{
		nsScrollX:      {},
		nsScrollY:      {},
		nsInputFocus:   {},
		nsListBoxFocus: {},
		nsTreeFocus:    {},
	},
}

// RegisterNamespaceSnapshot marks a StateMap namespace as
// snapshotable. During time-travel scrub, its BoundedMap
// contents are captured with each snapshot and restored on
// rewind. Intended for widget authors whose internal state
// should rewind with the scrubber (e.g. cursor position, tab
// index). Caches (SVG, tessellation) should stay live — do
// not register them. Safe to call from any goroutine; typical
// use is once at package init.
func RegisterNamespaceSnapshot(ns string) {
	if ns == "" {
		return
	}
	snapshotableNamespaces.mu.Lock()
	snapshotableNamespaces.set[ns] = struct{}{}
	snapshotableNamespaces.mu.Unlock()
}

// isNamespaceSnapshotable reports whether ns is whitelisted.
func isNamespaceSnapshotable(ns string) bool {
	snapshotableNamespaces.mu.RLock()
	_, ok := snapshotableNamespaces.set[ns]
	snapshotableNamespaces.mu.RUnlock()
	return ok
}

// cloneableMap is implemented by BoundedMap; used to snapshot
// whitelisted namespaces without knowing element types.
type cloneableMap interface {
	cloneAny() any
	restoreAny(src any)
}

// snapshotWhitelistedNamespaces returns a freshly-allocated map
// of {namespace -> cloned BoundedMap} for every whitelisted
// namespace currently present in the registry. Returns nil
// when nothing was captured so the hot path avoids allocation
// in the common case. Holds the whitelist RLock once across
// the registry walk rather than re-acquiring per iteration.
func snapshotWhitelistedNamespaces(w *Window) map[string]any {
	if w == nil || len(w.viewState.registry.maps) == 0 {
		return nil
	}
	var out map[string]any
	snapshotableNamespaces.mu.RLock()
	defer snapshotableNamespaces.mu.RUnlock()
	for ns, m := range w.viewState.registry.maps {
		if _, ok := snapshotableNamespaces.set[ns]; !ok {
			continue
		}
		cm, ok := m.(cloneableMap)
		if !ok {
			continue
		}
		if out == nil {
			out = make(map[string]any)
		}
		out[ns] = cm.cloneAny()
	}
	return out
}

// restoreWhitelistedNamespaces overwrites each whitelisted
// namespace's BoundedMap with the clone captured in entry.
// Namespaces absent from entry (e.g. registered after the
// snapshot) are left untouched. Type-mismatched entries
// (generic param changed between capture and restore, e.g.
// during hot reload) are skipped via recover rather than
// aborting the entire restore.
func restoreWhitelistedNamespaces(w *Window, entry map[string]any) {
	if w == nil || len(entry) == 0 {
		return
	}
	for ns, src := range entry {
		dst, ok := w.viewState.registry.maps[ns].(cloneableMap)
		if !ok {
			continue
		}
		safeRestoreAny(dst, src, ns)
	}
}

// safeRestoreAny invokes dst.restoreAny(src) and recovers from
// a type-assertion panic so a single out-of-sync namespace does
// not break scrub for the rest. Logs the skip for visibility.
func safeRestoreAny(dst cloneableMap, src any, ns string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("gui: time-travel: namespace %q restore "+
				"skipped: %v", ns, r)
		}
	}()
	dst.restoreAny(src)
}

// snapshotRing is a byte-capped FIFO ring of Snapshot values.
// Push appends; when total bytes exceed maxBytes, the oldest
// entries are evicted until the buffer fits. Concurrent safe
// via an internal RWMutex — independent of any Window mutex to
// avoid cross-window lock cycles during debug-window reads.
type snapshotRing struct {
	entries    []snapshotEntry
	totalBytes int
	maxBytes   int
	mu         sync.RWMutex
}

// newSnapshotRing returns a ring with the given byte cap. A
// non-positive cap is replaced with defaultHistoryBytes.
func newSnapshotRing(maxBytes int) *snapshotRing {
	if maxBytes <= 0 {
		maxBytes = defaultHistoryBytes
	}
	return &snapshotRing{maxBytes: maxBytes}
}

// defaultHistoryBytes is the default byte cap when
// WindowCfg.HistoryBytes is zero and DebugTimeTravel is on.
const defaultHistoryBytes = 64 << 20 // 64 MiB

// push appends a snapshot captured at when with the given cause
// label. Evicts oldest entries until totalBytes <= maxBytes.
func (r *snapshotRing) push(snap any, when time.Time, cause string) {
	if snap == nil {
		return
	}
	r.pushEntry(snapshotEntry{
		snap:  snap,
		when:  when,
		cause: cause,
	})
}

// pushEntry appends a pre-built entry. Computes bytes from the
// entry's snap field and evicts oldest to fit maxBytes.
func (r *snapshotRing) pushEntry(e snapshotEntry) {
	if e.snap == nil {
		return
	}
	e.bytes = snapshotDefaultSize
	if s, ok := e.snap.(snapshotSizer); ok {
		if n := s.Size(); n > 0 {
			e.bytes = n
		}
	}
	if e.bytes > snapshotMaxEntryBytes {
		e.bytes = snapshotMaxEntryBytes
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	r.totalBytes += e.bytes
	r.evictLocked()
}

// evictLocked drops oldest entries until totalBytes fits under
// maxBytes. Caller must hold r.mu for writing.
func (r *snapshotRing) evictLocked() {
	if r.totalBytes <= r.maxBytes {
		return
	}
	// Find first index i such that removing [0,i) fits cap.
	i := 0
	for i < len(r.entries) && r.totalBytes > r.maxBytes {
		r.totalBytes -= r.entries[i].bytes
		i++
	}
	if i == 0 {
		return
	}
	// Slide survivors to the front so the underlying array can be
	// reused without unbounded growth. Clear evicted slots so
	// snapshot values become eligible for GC.
	n := copy(r.entries, r.entries[i:])
	for j := n; j < len(r.entries); j++ {
		r.entries[j] = snapshotEntry{}
	}
	r.entries = r.entries[:n]
}

// len returns the number of entries in the ring.
func (r *snapshotRing) len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// bytes returns the current approximate byte usage.
func (r *snapshotRing) bytes() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalBytes
}

// at returns entry idx (0 = oldest), or zero-value and false if
// idx is out of range.
func (r *snapshotRing) at(idx int) (snapshotEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if idx < 0 || idx >= len(r.entries) {
		return snapshotEntry{}, false
	}
	return r.entries[idx], true
}

// last returns the most recent entry, or zero-value and false
// if the ring is empty.
func (r *snapshotRing) last() (snapshotEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := len(r.entries)
	if n == 0 {
		return snapshotEntry{}, false
	}
	return r.entries[n-1], true
}

// shouldSnapshot reports whether an event of this type should
// trigger a post-dispatch snapshot. Plain mouse motion, IME
// composition churn, and focus bookkeeping are skipped —
// they either don't mutate observable state or fire far too
// often to be useful in a scrub timeline.
func shouldSnapshot(t EventType) bool {
	switch t {
	case EventMouseMove,
		EventIMEComposition,
		EventFocused,
		EventUnfocused,
		EventResized:
		return false
	}
	return true
}

// captureSnapshot pushes a post-dispatch snapshot onto the
// window's history if enabled and the user state opts in.
// Safe to call on a nil Window or when history is unset.
func (w *Window) captureSnapshot(e *Event) {
	if w == nil || w.history == nil || e == nil {
		return
	}
	if !shouldSnapshot(e.Type) {
		return
	}
	s, ok := w.state.(Snapshotter)
	if !ok {
		return
	}
	ns := snapshotWhitelistedNamespaces(w)
	w.history.pushEntry(snapshotEntry{
		snap:       s.Snapshot(),
		namespaces: ns,
		when:       time.Now(),
		cause:      eventCause(e),
	})
}

// eventCause builds a short label describing the event that
// produced a snapshot. Intended for the debug-window timeline;
// the format is informational and may change.
func eventCause(e *Event) string {
	if e == nil {
		return ""
	}
	switch e.Type {
	case EventChar:
		return "char"
	case EventKeyDown:
		return "key"
	case EventMouseDown:
		return "mouse-down"
	case EventMouseUp:
		return "mouse-up"
	case EventMouseScroll:
		return "scroll"
	case EventFileDropped:
		return "file-drop"
	case EventTouchesBegan:
		return "touch-begin"
	case EventTouchesMoved:
		return "touch-move"
	case EventTouchesEnded:
		return "touch-end"
	case EventTouchesCancelled:
		return "touch-cancel"
	}
	return "event"
}

// EnableHistory turns on time-travel snapshot capture with a
// byte cap. A non-positive cap selects defaultHistoryBytes.
// Idempotent — a second call with a different cap updates the
// cap and evicts any now-oversized entries under the ring's
// own mutex; existing entries are preserved otherwise.
// Requires the window's user state to implement Snapshotter;
// otherwise no snapshots are ever pushed (but calling the
// method is still safe).
func (w *Window) EnableHistory(maxBytes int) {
	if w == nil {
		return
	}
	if maxBytes <= 0 {
		maxBytes = defaultHistoryBytes
	}
	if w.history == nil {
		w.history = newSnapshotRing(maxBytes)
		return
	}
	w.history.mu.Lock()
	w.history.maxBytes = maxBytes
	w.history.evictLocked()
	w.history.mu.Unlock()
}

// HistoryLen returns the number of snapshots currently held.
// Zero when history is disabled. Safe to call from any goroutine.
func (w *Window) HistoryLen() int {
	if w == nil || w.history == nil {
		return 0
	}
	return w.history.len()
}

// OpenDebugWindow queues a secondary Window that hosts the
// time-travel scrubber for this window. Requires the window
// to be part of an App (multi-window mode) and to have
// history enabled; otherwise it logs and returns. Non-blocking:
// the actual window is created on the next App event loop tick.
// Safe to call from any goroutine.
func (w *Window) OpenDebugWindow() {
	if w == nil {
		return
	}
	app := w.App()
	if app == nil {
		log.Println("gui: OpenDebugWindow: no App (single-window mode)")
		return
	}
	if w.history == nil {
		w.EnableHistory(0)
	}
	ctrl := &TimeTravelController{App: w}
	if n := w.history.len(); n > 0 {
		ctrl.Cursor = n - 1
	}
	title := "Time Travel"
	if w.Config.Title != "" {
		// Keep the composed title compact for the scrubber's
		// title bar. sanitizeTitle also enforces an outer cap.
		const maxParentTitle = 256
		parent := w.Config.Title
		if len(parent) > maxParentTitle {
			parent = parent[:maxParentTitle]
		}
		title = "Time Travel — " + parent
	}
	app.OpenWindow(WindowCfg{
		State:  ctrl,
		Title:  title,
		Width:  300,
		Height: 150,
		OnInit: func(dw *Window) {
			dw.UpdateView(debugWindowView)
		},
		// Closing the debug window must unfreeze the app,
		// otherwise the user is stranded with input blocked
		// and no scrubber to resume from.
		OnCloseRequest: func(dw *Window) {
			if ctrl.App != nil {
				ctrl.App.Resume()
			}
			dw.Close()
		},
	})
}

// debugWindowView is the view generator installed on the
// debug window. Reads the TimeTravelController from its state
// slot and delegates to ctrl.View.
func debugWindowView(dw *Window) View {
	return State[TimeTravelController](dw).View(dw)
}

// Freeze enters time-travel scrub mode. Subsequent user input
// events are dropped by EventFn; the frame loop keeps running
// so the debug window's restore requests take effect and the
// app window continues to repaint. Idempotent. No-op when
// history is disabled. Safe to call from any goroutine.
func (w *Window) Freeze() {
	if w == nil || w.history == nil {
		return
	}
	w.frozen.Store(true)
}

// Resume exits time-travel scrub mode. Clears the virtual
// clock pin so w.Now() returns live time again. Idempotent.
// Safe to call from any goroutine.
func (w *Window) Resume() {
	if w == nil {
		return
	}
	w.frozen.Store(false)
	w.virtualNow.Store(nil)
	w.UpdateWindow()
}

// IsFrozen reports whether the window is currently in a
// read-only time-travel scrub.
func (w *Window) IsFrozen() bool {
	if w == nil {
		return false
	}
	return w.frozen.Load()
}

// PostRestore posts a restore request for history entry idx.
// Intended for cross-goroutine calls from the debug window.
// The request is serialized through the app window's command
// queue so it runs on the main thread under w.mu, avoiding
// torn reads against a concurrent view fn. No-op when history
// is disabled. Safe to call from any goroutine.
func (w *Window) PostRestore(idx int) {
	if w == nil || w.history == nil {
		return
	}
	w.QueueCommand(func(w *Window) {
		w.restoreLocked(idx)
	})
}

// restoreLocked performs the actual Snapshotter.Restore under
// w.mu. Intended to run inside a queued command on the main
// thread. Also pins the virtual clock to the entry's timestamp
// so clock-driven views render consistently with the snapshot.
func (w *Window) restoreLocked(idx int) {
	if w.history == nil {
		return
	}
	entry, ok := w.history.at(idx)
	if !ok {
		return
	}
	s, ok := w.state.(Snapshotter)
	if !ok {
		return
	}
	w.mu.Lock()
	s.Restore(entry.snap)
	restoreWhitelistedNamespaces(w, entry.namespaces)
	w.mu.Unlock()
	when := entry.when
	w.setVirtualNow(&when)
	w.UpdateWindow()
}
