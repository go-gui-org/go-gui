package gui

import (
	"bytes"
	"testing"
)

// mockBookmarkPlatform records bookmark backend calls.
type mockBookmarkPlatform struct {
	noopNativePlatform
	entries   []BookmarkEntry
	persisted []BookmarkEntry
	stopped   [][]byte
}

func (m *mockBookmarkPlatform) BookmarkLoadAll(_ string) []BookmarkEntry {
	return m.entries
}

func (m *mockBookmarkPlatform) BookmarkPersist(_, path string,
	data []byte) {
	m.persisted = append(m.persisted,
		BookmarkEntry{Path: path, Data: data})
}

func (m *mockBookmarkPlatform) BookmarkStopAccess(data []byte) {
	m.stopped = append(m.stopped, data)
}

func TestFileAccessStoreBookmark(t *testing.T) {
	w := &Window{}
	g := w.storeBookmark("/path/to/file", nil)
	if g.ID == 0 {
		t.Error("expected non-zero grant ID")
	}
	if w.fileAccessGrantCount() != 1 {
		t.Errorf("grant count: got %d, want 1", w.fileAccessGrantCount())
	}
}

func TestFileAccessStoreMultiple(t *testing.T) {
	w := &Window{}
	g1 := w.storeBookmark("/a", nil)
	g2 := w.storeBookmark("/b", nil)
	if g1.ID == g2.ID {
		t.Error("expected unique grant IDs")
	}
	if w.fileAccessGrantCount() != 2 {
		t.Errorf("grant count: got %d, want 2", w.fileAccessGrantCount())
	}
}

func TestFileAccessStoreEmptyPath(t *testing.T) {
	w := &Window{}
	g := w.storeBookmark("", []byte{1, 2, 3})
	if g.ID != 0 {
		t.Errorf("empty path grant: got %d, want 0", g.ID)
	}
	if w.fileAccessGrantCount() != 0 {
		t.Errorf("grant count: got %d, want 0", w.fileAccessGrantCount())
	}
}

func TestFileAccessStoreCopiesData(t *testing.T) {
	w := &Window{}
	data := []byte{1, 2, 3}
	g := w.storeBookmark("/file", data)
	data[0] = 9
	w.fileAccess.mu.Lock()
	kept := w.fileAccess.grants[g.ID].data
	w.fileAccess.mu.Unlock()
	if !bytes.Equal(kept, []byte{1, 2, 3}) {
		t.Errorf("stored blob mutated: %v", kept)
	}
}

func TestFileAccessReleaseGrant(t *testing.T) {
	w := &Window{}
	g := w.storeBookmark("/file", nil)
	w.ReleaseFileAccess(g)
	if w.fileAccessGrantCount() != 0 {
		t.Errorf("grant count: got %d, want 0", w.fileAccessGrantCount())
	}
}

func TestFileAccessReleaseZeroGrant(t *testing.T) {
	w := &Window{}
	w.storeBookmark("/file", nil)
	w.ReleaseFileAccess(Grant{ID: 0}) // no-op
	if w.fileAccessGrantCount() != 1 {
		t.Errorf("grant count: got %d, want 1", w.fileAccessGrantCount())
	}
}

func TestFileAccessReleaseUnknownGrant(t *testing.T) {
	w := &Window{}
	w.storeBookmark("/file", nil)
	w.ReleaseFileAccess(Grant{ID: 999}) // unknown
	if w.fileAccessGrantCount() != 1 {
		t.Errorf("grant count: got %d, want 1", w.fileAccessGrantCount())
	}
}

func TestFileAccessReleaseStopsAccess(t *testing.T) {
	mock := &mockBookmarkPlatform{}
	w := &Window{}
	w.nativePlatform = mock
	g := w.storeBookmark("/file", []byte{7})
	w.ReleaseFileAccess(g)
	if len(mock.stopped) != 1 || !bytes.Equal(mock.stopped[0],
		[]byte{7}) {
		t.Errorf("stopped: %v", mock.stopped)
	}
}

func TestFileAccessReleaseAll(t *testing.T) {
	w := &Window{}
	w.storeBookmark("/a", nil)
	w.storeBookmark("/b", nil)
	w.storeBookmark("/c", nil)
	w.ReleaseAllFileAccess()
	if w.fileAccessGrantCount() != 0 {
		t.Errorf("grant count: got %d, want 0", w.fileAccessGrantCount())
	}
}

func TestFileAccessReleaseAllEmpty(t *testing.T) {
	w := &Window{}
	w.ReleaseAllFileAccess() // should not panic
	if w.fileAccessGrantCount() != 0 {
		t.Errorf("expected 0")
	}
}

func TestFileAccessRestoreNoAppID(t *testing.T) {
	w := &Window{}
	w.RestoreFileAccess() // no-op, no panic
	if w.fileAccessGrantCount() != 0 {
		t.Errorf("grant count: got %d, want 0", w.fileAccessGrantCount())
	}
}

func TestFileAccessSetAppID(t *testing.T) {
	mock := &mockBookmarkPlatform{
		entries: []BookmarkEntry{{Path: "/restored"}},
	}
	w := &Window{}
	w.nativePlatform = mock
	w.SetFileAccessAppID("com.example.app")
	w.RestoreFileAccess()
	if w.fileAccessGrantCount() != 1 {
		t.Errorf("grant count: got %d, want 1", w.fileAccessGrantCount())
	}
}

func TestFileAccessRestoreClearsThenReloads(t *testing.T) {
	mock := &mockBookmarkPlatform{
		entries: []BookmarkEntry{{Path: "/new", Data: []byte{2}}},
	}
	w := &Window{}
	w.nativePlatform = mock
	w.SetFileAccessAppID("com.example.app")
	w.storeBookmark("/old", []byte{1})
	w.RestoreFileAccess()
	if w.fileAccessGrantCount() != 1 {
		t.Fatalf("grant count: got %d, want 1", w.fileAccessGrantCount())
	}
	w.fileAccess.mu.Lock()
	foundOld := false
	foundNew := false
	for _, bm := range w.fileAccess.grants {
		if bm.path == "/old" {
			foundOld = true
		}
		if bm.path == "/new" {
			foundNew = true
		}
	}
	w.fileAccess.mu.Unlock()
	if foundOld || !foundNew {
		t.Errorf("old=%v new=%v, want old=false new=true", foundOld,
			foundNew)
	}
	if len(mock.stopped) != 1 || !bytes.Equal(mock.stopped[0],
		[]byte{1}) {
		t.Errorf("stopped: %v", mock.stopped)
	}
}

func TestFileAccessRestoreSkipsEmptyPath(t *testing.T) {
	mock := &mockBookmarkPlatform{
		entries: []BookmarkEntry{
			{Path: "", Data: []byte{1}},
			{Path: "/kept"},
		},
	}
	w := &Window{}
	w.nativePlatform = mock
	w.SetFileAccessAppID("com.example.app")
	w.RestoreFileAccess()
	if w.fileAccessGrantCount() != 1 {
		t.Errorf("grant count: got %d, want 1", w.fileAccessGrantCount())
	}
}

func TestFileAccessRestoreNoRepersist(t *testing.T) {
	mock := &mockBookmarkPlatform{
		entries: []BookmarkEntry{{Path: "/a", Data: []byte{5}}},
	}
	w := &Window{}
	w.nativePlatform = mock
	w.SetFileAccessAppID("com.example.app")
	w.RestoreFileAccess()
	if len(mock.persisted) != 0 {
		t.Errorf("persisted on restore: %v", mock.persisted)
	}
}

func TestFileAccessRestoreNilPlatformWithAppID(t *testing.T) {
	w := &Window{}
	w.SetFileAccessAppID("com.example.app")
	w.RestoreFileAccess() // nil platform: no-op, no panic
	if w.fileAccessGrantCount() != 0 {
		t.Errorf("grant count: got %d, want 0", w.fileAccessGrantCount())
	}
}

func TestGrantZeroID(t *testing.T) {
	g := Grant{}
	if g.ID != 0 {
		t.Error("zero value grant should have ID 0")
	}
}

func TestAccessiblePathFields(t *testing.T) {
	ap := AccessiblePath{Path: "/file", Grant: Grant{ID: 42}}
	if ap.Path != "/file" || ap.Grant.ID != 42 {
		t.Errorf("fields: %+v", ap)
	}
}
