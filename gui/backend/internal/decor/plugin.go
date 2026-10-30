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
		path := mappedPath(line)
		if strings.HasPrefix(filepath.Base(path), "libdecor-0.so") {
			return filepath.Join(filepath.Dir(path), "libdecor", "plugins-1")
		}
	}
	return ""
}

// mappedPath is the file a line of /proc/self/maps maps, or "" for a
// mapping with no file ([stack], anonymous). The path is the last field
// and starts with '/'. A file replaced on disk since it was mapped (a
// package upgrade) ends in " (deleted)", which is cut off.
func mappedPath(line string) string {
	i := strings.IndexByte(line, '/')
	if i < 0 {
		return ""
	}
	return strings.TrimSuffix(strings.TrimRight(line[i:], "\n"), " (deleted)")
}

// gtkPluginLoaded reports whether libdecor's GTK plugin is loaded. Call it
// after libdecor_new: libdecor loads every plugin to read its description,
// then unloads all but the one it picked. That includes the GTK plugin
// when it failed to start and libdecor fell back to cairo. So the GTK
// plugin still loaded means it draws the frames. GTK itself being loaded
// means nothing: the app may use GTK, and GTK may stay loaded after the
// plugin is gone. With no /proc the answer is false, so the frame variant
// is never changed on a guess.
func gtkPluginLoaded() bool {
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return false
	}
	return mapsHaveGTKPlugin(string(maps))
}

// mapsHaveGTKPlugin is gtkPluginLoaded on the text of /proc/self/maps.
func mapsHaveGTKPlugin(maps string) bool {
	for line := range strings.Lines(maps) {
		if filepath.Base(mappedPath(line)) == "libdecor-gtk.so" {
			return true
		}
	}
	return false
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
