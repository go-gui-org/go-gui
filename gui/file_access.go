package gui

import "sync"

// Grant identifies a security-scoped bookmark. Release via
// Window.ReleaseFileAccess when access is no longer needed.
type Grant struct {
	ID uint64 // 0 = no grant (no-op on release)
}

// accessiblePath pairs a filesystem path with an optional
// security-scoped grant. On macOS sandboxed apps the grant
// keeps the path accessible across relaunches.
type accessiblePath struct {
	Path  string
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

// SetFileAccessAppID sets the app identifier used for
// bookmark persistence. Call before RestoreFileAccess.
func (w *Window) SetFileAccessAppID(appID string) {
	w.fileAccess.mu.Lock()
	w.fileAccess.appID = appID
	w.fileAccess.mu.Unlock()
}

// RestoreFileAccess loads and activates persisted
// security-scoped bookmarks. Call in OnInit.
func (w *Window) RestoreFileAccess() {
	w.fileAccess.mu.Lock()
	appID := w.fileAccess.appID
	w.fileAccess.mu.Unlock()

	if appID == "" || w.nativePlatform == nil {
		return
	}
	entries := w.nativePlatform.BookmarkLoadAll(appID)
	for _, entry := range entries {
		if entry.Path != "" {
			w.storeBookmark(entry.Path, entry.Data)
		}
	}
}

// ReleaseFileAccess releases a single bookmark grant.
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
	data := bm.data
	w.fileAccess.mu.Unlock()

	if len(data) > 0 && w.nativePlatform != nil {
		w.nativePlatform.BookmarkStopAccess(data)
	}
}

// ReleaseAllFileAccess releases every active grant.
// Called automatically during window cleanup.
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
// persists via NativePlatform if app_id is set.
func (w *Window) storeBookmark(path string, data []byte) Grant {
	w.fileAccess.mu.Lock()
	appID := w.fileAccess.appID
	if w.fileAccess.grants == nil {
		w.fileAccess.grants = make(map[uint64]bookmarkGrant)
	}
	id := w.fileAccess.nextID
	if id == 0 {
		id = 1
	}
	w.fileAccess.nextID = id + 1
	w.fileAccess.grants[id] = bookmarkGrant{path: path, data: data}
	w.fileAccess.mu.Unlock()

	if len(data) > 0 && appID != "" && w.nativePlatform != nil {
		w.nativePlatform.BookmarkPersist(appID, path, data)
	}
	return Grant{ID: id}
}

// FileAccessGrantCount returns the number of active grants.
// Intended for testing.
func (w *Window) FileAccessGrantCount() int {
	w.fileAccess.mu.Lock()
	n := len(w.fileAccess.grants)
	w.fileAccess.mu.Unlock()
	return n
}
