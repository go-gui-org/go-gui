//go:build linux && !android && (amd64 || arm64)

package decor

import (
	"os"
	"path/filepath"
	"strings"
)

// HasPlugin reports whether libdecor will find a decoration plugin. Call it
// after Load.
//
// libdecor itself works with no plugin: libdecor_new succeeds, and a
// built-in fallback makes a plain xdg_toplevel that draws nothing. It says
// so only on stderr, and its API has no way to ask. On a compositor with no
// server-side decorations (weston, GNOME, Cinnamon) the window then has no
// frame and cannot be moved, and the cause is a missing distro package
// (libdecor-0-plugin-1-gtk on Debian and Ubuntu), so the caller warns.
//
// The plugin directory is found the way libdecor finds it:
// $LIBDECOR_PLUGIN_DIR, else libdecor/plugins-1 beside the loaded library
// (libdecor's compiled-in path is $libdir/libdecor/plugins-1). When the
// directory cannot be told, HasPlugin answers true: a false warning would
// send the user to install a package they already have.
func HasPlugin() bool {
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return true
	}
	dir := pluginDir(os.Getenv("LIBDECOR_PLUGIN_DIR"), string(maps))
	if dir == "" {
		return true
	}
	return dirHasPlugin(dir)
}

// pluginDir returns the directory libdecor reads plugins from, given the
// LIBDECOR_PLUGIN_DIR value and the text of /proc/self/maps, or "" when
// libdecor is not mapped.
func pluginDir(env, maps string) string {
	if env != "" {
		return env
	}
	for line := range strings.Lines(maps) {
		// The path is the last field, and starts with '/'. A mapping with
		// no file ([stack], anonymous) has none.
		i := strings.IndexByte(line, '/')
		if i < 0 {
			continue
		}
		path := strings.TrimRight(line[i:], "\n")
		if strings.HasPrefix(filepath.Base(path), "libdecor-0.so") {
			return filepath.Join(filepath.Dir(path), "libdecor", "plugins-1")
		}
	}
	return ""
}

// dirHasPlugin reports whether dir holds a shared object, which is what
// libdecor tries to load as a plugin. A missing or unreadable directory
// holds none.
func dirHasPlugin(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".so") {
			return true
		}
	}
	return false
}
