//go:build linux && !android && (amd64 || arm64)

package decor

import (
	"os"
	"path/filepath"
	"testing"
)

// A /proc/self/maps excerpt with libdecor mapped from the Debian/Ubuntu
// multiarch directory.
const mapsUbuntu = `7f1a2c000000-7f1a2c004000 r--p 00000000 103:02 1234 /usr/lib/x86_64-linux-gnu/libwayland-client.so.0.22.0
7f1a2c010000-7f1a2c013000 r--p 00000000 103:02 5678 /usr/lib/x86_64-linux-gnu/libdecor-0.so.0.200.2
7f1a2c013000-7f1a2c018000 r-xp 00003000 103:02 5678 /usr/lib/x86_64-linux-gnu/libdecor-0.so.0.200.2
7ffd1c000000-7ffd1c021000 rw-p 00000000 00:00 0 [stack]
`

func TestPluginDir(t *testing.T) {
	for _, c := range []struct {
		name, env, maps, want string
	}{
		{"env wins", "/opt/decor", mapsUbuntu, "/opt/decor"},
		{"beside the library", "", mapsUbuntu, "/usr/lib/x86_64-linux-gnu/libdecor/plugins-1"},
		{"fedora lib64", "", "7f00-7f01 r-xp 0 0:0 1 /usr/lib64/libdecor-0.so.0.200.2\n", "/usr/lib64/libdecor/plugins-1"},
		// Not loaded, or a name that only looks alike: unknown.
		{"not mapped", "", "7f00-7f01 r-xp 0 0:0 1 /usr/lib/libwayland-client.so.0\n", ""},
		{"lookalike", "", "7f00-7f01 r-xp 0 0:0 1 /usr/lib/libdecor-0.so.0-old/x.so\n", ""},
		{"empty", "", "", ""},
	} {
		if got := pluginDir(c.env, c.maps); got != c.want {
			t.Errorf("%s: pluginDir = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDirHasPlugin(t *testing.T) {
	dir := t.TempDir()
	if dirHasPlugin(filepath.Join(dir, "missing")) {
		t.Error("a missing directory has a plugin")
	}
	if dirHasPlugin(dir) {
		t.Error("an empty directory has a plugin")
	}
	// A stray file that is not a shared object is not a plugin.
	if err := os.WriteFile(filepath.Join(dir, "README"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if dirHasPlugin(dir) {
		t.Error("a directory with only a README has a plugin")
	}
	if err := os.WriteFile(filepath.Join(dir, "libdecor-gtk.so"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !dirHasPlugin(dir) {
		t.Error("libdecor-gtk.so is not seen as a plugin")
	}
}
