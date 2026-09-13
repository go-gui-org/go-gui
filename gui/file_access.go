package gui

import "sync"

// Grant identifies a security-scoped bookmark. Release via
// Window.ReleaseFileAccess when access is no longer needed.
// exportaudit:keep — reachable from an exported signature
type Grant struct {
	ID uint64 // 0 = no grant (no-op on release)
}

// AccessiblePath pairs a filesystem path with an optional
// security-scoped grant. On macOS sandboxed apps the grant
// keeps the path accessible across relaunches.
// exportaudit:keep — caller-facing config (issue #372)
type AccessiblePath struct {
	// Path is the filesystem path the user chose.
	// exportaudit:keep — caller-facing config (issue #372)
	Path string
	// Grant is the security-scoped grant that keeps Path reachable on a
	// sandboxed macOS app. The zero Grant means no scope was needed.
	// exportaudit:keep — caller-facing config (issue #372)
	Grant Grant
}

type bookmarkGrant struct {
	path string
	data []byte // macOS bookmark blob; empty on other platforms
}

type fileAccessState struct {
	grants map[uint64]bookmarkGrant
	appID  string
	nextID uint64
	mu     sync.Mutex
}

// SetFileAccessAppID sets the app ID used for bookmark
// persistence. Call it before RestoreFileAccess, typically
// in OnInit. Must be called from the main thread; it is not
// safe for concurrent use. From any other goroutine, use
// Window.QueueCommand instead.
// exportaudit:keep — caller-facing file access API (issue #372)
func (w *Window) SetFileAccessAppID(appID string) {
	w.fileAccess.mu.Lock()
	w.fileAccess.appID = appID
	w.fileAccess.mu.Unlock()
}

// RestoreFileAccess clears active grants and then loads the
// persisted bookmarks for the app ID set with
// SetFileAccessAppID. Call it in OnInit. Entries with an
// empty path are skipped. Loaded entries are recorded
// without a new persist call. Release stops access only;
// the persisted copy stays, so persist is write-only. Must
// be called from the main thread; it is not safe for
// concurrent use. From any other goroutine, use
// Window.QueueCommand instead.
// exportaudit:keep — caller-facing file access API (issue #372)
func (w *Window) RestoreFileAccess() {
	// Clear first so a second OnInit call cannot record
	// the same bookmark twice. ReleaseAll stops access
	// outside its lock.
	w.ReleaseAllFileAccess()

	w.fileAccess.mu.Lock()
	appID := w.fileAccess.appID
	w.fileAccess.mu.Unlock()

	if appID == "" || w.nativePlatform == nil {
		return
	}
	entries := w.nativePlatform.BookmarkLoadAll(appID)
	for _, entry := range entries {
		if entry.Path != "" {
			w.storeBookmarkInternal(entry.Path, entry.Data, false)
		}
	}
}

// ReleaseFileAccess releases a single bookmark grant. A zero
// Grant and an unknown ID are no-ops. Release stops access
// only; the persisted copy stays, so persist is write-only.
// A grant left held is released by WindowCleanup, so a missed
// call leaks only until close. Must be called from the main
// thread; it is not safe for concurrent use. From any other
// goroutine, use Window.QueueCommand instead.
// exportaudit:keep — caller-facing file access API (issue #372)
func (w *Window) ReleaseFileAccess(g Grant) {
	if g.ID == 0 {
		return
	}
	w.fileAccess.mu.Lock()
	bm, ok := w.fileAccess.grants[g.ID]
	if !ok {
		w.fileAccess.mu.Unlock()
		return
	}
	delete(w.fileAccess.grants, g.ID)
	stopped := bm.data
	w.fileAccess.mu.Unlock()

	if len(stopped) > 0 && w.nativePlatform != nil {
		w.nativePlatform.BookmarkStopAccess(stopped)
	}
}

// ReleaseAllFileAccess releases every active grant. It is
// called automatically during window cleanup. Release stops
// access only; persisted copies stay, so persist is
// write-only. Must be called from the main thread; it is not
// safe for concurrent use. From any other goroutine, use
// Window.QueueCommand instead.
// exportaudit:keep — caller-facing file access API (issue #372)
func (w *Window) ReleaseAllFileAccess() {
	w.fileAccess.mu.Lock()
	grants := make([]bookmarkGrant, 0, len(w.fileAccess.grants))
	for _, bm := range w.fileAccess.grants {
		grants = append(grants, bm)
	}
	w.fileAccess.grants = nil
	w.fileAccess.mu.Unlock()

	if w.nativePlatform != nil {
		for _, bm := range grants {
			if len(bm.data) > 0 {
				w.nativePlatform.BookmarkStopAccess(bm.data)
			}
		}
	}
}

// storeBookmark records a bookmark grant internally and
// persists via NativePlatform when an app ID is set.
func (w *Window) storeBookmark(path string, data []byte) Grant {
	return w.storeBookmarkInternal(path, data, true)
}

// storeBookmarkInternal records a grant. When persist is true
// it also writes the bookmark through NativePlatform. An
// empty path returns a zero Grant and records nothing.
func (w *Window) storeBookmarkInternal(path string, data []byte,
	persist bool) Grant {
	if path == "" {
		return Grant{}
	}
	// Copy the blob so later changes by the caller cannot
	// alter the stored grant. Bookmarks are rare, so the
	// single copy costs nothing on the hot path.
	var kept []byte
	if len(data) > 0 {
		kept = append([]byte(nil), data...)
	}

	w.fileAccess.mu.Lock()
	appID := w.fileAccess.appID
	if w.fileAccess.grants == nil {
		w.fileAccess.grants = make(map[uint64]bookmarkGrant)
	}
	id := w.fileAccess.nextID
	if id == 0 {
		id = 1
	}
	// Guard the uint64 wrap path: 0 stays reserved for the
	// zero Grant, and a wrapped ID must not reuse a live one.
	for {
		if _, taken := w.fileAccess.grants[id]; !taken {
			break
		}
		id++
		if id == 0 {
			id = 1
		}
	}
	w.fileAccess.nextID = id + 1
	if w.fileAccess.nextID == 0 {
		w.fileAccess.nextID = 1
	}
	w.fileAccess.grants[id] = bookmarkGrant{path: path, data: kept}
	w.fileAccess.mu.Unlock()

	if persist && len(kept) > 0 && appID != "" &&
		w.nativePlatform != nil {
		w.nativePlatform.BookmarkPersist(appID, path, kept)
	}
	return Grant{ID: id}
}

// FileAccessGrantCount returns the number of active grants.
// Intended for testing.
func (w *Window) fileAccessGrantCount() int {
	w.fileAccess.mu.Lock()
	n := len(w.fileAccess.grants)
	w.fileAccess.mu.Unlock()
	return n
}
