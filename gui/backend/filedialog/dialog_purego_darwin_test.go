//go:build darwin && !ios

package filedialog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// These tests build the parts of a panel that need no window, so they
// run headless. NSAlert and the panels are not tested here: AppKit
// aborts when one is created off the main thread, and go test runs
// each test on its own goroutine.

func TestContentTypes(t *testing.T) {
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	if got := contentTypes(nil); got != 0 {
		t.Errorf("contentTypes(nil) = %#x, want 0 (allow all)", got)
	}
	// An extension with bytes that are not UTF-8 is skipped.
	types := contentTypes([]string{"png", "txt", "\xff"})
	if types == 0 {
		t.Fatal("contentTypes(png, txt) = 0")
	}
	if n := objc.Send[uint](types, dl.selCount); n != 2 {
		t.Errorf("contentTypes count = %d, want 2", n)
	}
	if got := contentTypes([]string{"\xff"}); got != 0 {
		t.Errorf("contentTypes(invalid only) = %#x, want 0", got)
	}
}

func TestPathsCopiesFileSystemPaths(t *testing.T) {
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	dir := t.TempDir()
	// A non-ASCII name: fileSystemRepresentation must round-trip it.
	want := filepath.Join(dir, "café.txt")
	if err := os.WriteFile(want, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	urls := dl.clsArray.Send(dl.selArrayCap, uint(1))
	urls.Send(dl.selAddObject, dl.clsURL.Send(dl.selFileURL, darwinffi.NSString(want)))

	got := paths(urls)
	if len(got) != 1 {
		t.Fatalf("paths = %d entries, want 1", len(got))
	}
	// fileSystemRepresentation gives the decomposed (NFD) form of
	// "é", as the cgo form did, so the bytes differ from want. Compare
	// the files instead.
	gotInfo, err := os.Stat(got[0].Path)
	if err != nil {
		t.Fatalf("path %q: %v", got[0].Path, err)
	}
	wantInfo, _ := os.Stat(want)
	if !os.SameFile(gotInfo, wantInfo) {
		t.Errorf("path = %q, want %q", got[0].Path, want)
	}
	empty := dl.clsArray.Send(dl.selArrayCap, uint(0))
	if got := paths(empty); got != nil {
		t.Errorf("paths(empty) = %v, want nil", got)
	}
}

func TestAlertStyle(t *testing.T) {
	tests := []struct {
		level gui.NativeAlertLevel
		want  int
	}{
		{gui.AlertInfo, nsAlertStyleInfo},
		{gui.AlertWarning, nsAlertStyleWarning},
		{gui.AlertCritical, nsAlertStyleCritical},
		{gui.NativeAlertLevel(99), nsAlertStyleInfo},
	}
	for _, tc := range tests {
		if got := alertStyle(tc.level); got != tc.want {
			t.Errorf("alertStyle(%d) = %d, want %d", tc.level, got, tc.want)
		}
	}
}
