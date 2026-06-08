package gui

import "testing"

func TestFileAccessStoreBookmark(t *testing.T) {
	w := &Window{}
	g := w.storeBookmark("/path/to/file", nil)
	if g.ID == 0 {
		t.Error("expected non-zero grant ID")
	}
	if w.FileAccessGrantCount() != 1 {
		t.Errorf("grant count: got %d, want 1", w.FileAccessGrantCount())
	}
}

func TestFileAccessStoreMultiple(t *testing.T) {
	w := &Window{}
	g1 := w.storeBookmark("/a", nil)
	g2 := w.storeBookmark("/b", nil)
	if g1.ID == g2.ID {
		t.Error("expected unique grant IDs")
	}
	if w.FileAccessGrantCount() != 2 {
		t.Errorf("grant count: got %d, want 2", w.FileAccessGrantCount())
	}
}

func TestFileAccessReleaseGrant(t *testing.T) {
	w := &Window{}
	g := w.storeBookmark("/file", nil)
	w.ReleaseFileAccess(g)
	if w.FileAccessGrantCount() != 0 {
		t.Errorf("grant count: got %d, want 0", w.FileAccessGrantCount())
	}
}

func TestFileAccessReleaseZeroGrant(t *testing.T) {
	w := &Window{}
	w.storeBookmark("/file", nil)
	w.ReleaseFileAccess(Grant{ID: 0}) // no-op
	if w.FileAccessGrantCount() != 1 {
		t.Errorf("grant count: got %d, want 1", w.FileAccessGrantCount())
	}
}

func TestFileAccessReleaseUnknownGrant(t *testing.T) {
	w := &Window{}
	w.storeBookmark("/file", nil)
	w.ReleaseFileAccess(Grant{ID: 999}) // unknown
	if w.FileAccessGrantCount() != 1 {
		t.Errorf("grant count: got %d, want 1", w.FileAccessGrantCount())
	}
}

func TestFileAccessReleaseAll(t *testing.T) {
	w := &Window{}
	w.storeBookmark("/a", nil)
	w.storeBookmark("/b", nil)
	w.storeBookmark("/c", nil)
	w.ReleaseAllFileAccess()
	if w.FileAccessGrantCount() != 0 {
		t.Errorf("grant count: got %d, want 0", w.FileAccessGrantCount())
	}
}

func TestFileAccessReleaseAllEmpty(t *testing.T) {
	w := &Window{}
	w.ReleaseAllFileAccess() // should not panic
	if w.FileAccessGrantCount() != 0 {
		t.Errorf("expected 0")
	}
}

func TestFileAccessRestoreNoAppID(t *testing.T) {
	_ = t
	w := &Window{}
	w.RestoreFileAccess() // no-op, no panic
}

func TestFileAccessSetAppID(t *testing.T) {
	w := &Window{}
	w.SetFileAccessAppID("com.example.app")
	w.fileAccess.mu.Lock()
	appID := w.fileAccess.appID
	w.fileAccess.mu.Unlock()
	if appID != "com.example.app" {
		t.Errorf("appID: got %q", appID)
	}
}

func TestGrantZeroID(t *testing.T) {
	g := Grant{}
	if g.ID != 0 {
		t.Error("zero value grant should have ID 0")
	}
}

func TestAccessiblePathFields(t *testing.T) {
	ap := accessiblePath{Path: "/file", Grant: Grant{ID: 42}}
	if ap.Path != "/file" || ap.Grant.ID != 42 {
		t.Errorf("fields: %+v", ap)
	}
}
