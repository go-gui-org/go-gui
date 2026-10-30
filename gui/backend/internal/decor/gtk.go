//go:build linux && !android && (amd64 || arm64)

package decor

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// The light or dark frame variant (#1192).
//
// libdecor's GTK plugin draws the frame with GTK 3 widgets, and GTK takes
// the variant from the GtkSettings property
// gtk-application-prefer-dark-theme. The plugin sets it once, at start-up,
// from the desktop's color-scheme (libdecor 0.2.3 on; 0.2.2 never sets it,
// so frames start light), and never reads the desktop again. So the
// backend sets it itself, through GObject. The plugin loaded GTK into the
// process, and dlopen with RTLD_NOLOAD finds it there without loading
// anything. The setting is per process, so every frame shares one variant.
// It is GTK's own setting: GTK 3 widgets the app shows itself switch too.
//
// Only a Context whose frames the GTK plugin draws touches the setting
// (Context.gtk, set in New). With the cairo plugin, which draws fixed
// colours, GTK may still be loaded: by the app, or left behind by a GTK
// plugin that failed to start. Changing the setting then would restyle the
// app's own GTK windows and leave the frames as they were. libdecor 0.1
// has no GTK plugin at all.

// rtldNoload is glibc's and musl's RTLD_NOLOAD, which purego does not
// export: dlopen returns a library only when it is already loaded.
const rtldNoload = 0x4

// gTypeBoolean is G_TYPE_BOOLEAN: G_TYPE_MAKE_FUNDAMENTAL(5).
const gTypeBoolean = 5 << 2

// gValue is a GValue: a GType and two words of data.
type gValue struct {
	typ  uintptr
	data [2]uint64
}

// GTK and GObject entry points, resolved by loadGTK.
var (
	fnSettingsGetDefault uintptr
	fnValueInit          uintptr
	fnValueGetBoolean    uintptr
	fnValueSetBoolean    uintptr
	fnObjectGetProperty  uintptr
	fnObjectSetProperty  uintptr

	gtkTried bool
)

// propPreferDark is the property's name, NUL-terminated for C.
var propPreferDark = []byte("gtk-application-prefer-dark-theme\x00")

// loadGTK resolves the GTK calls from the GTK the plugin loaded. It runs
// once, for a Context whose frames the GTK plugin draws, so GTK is loaded
// by the first call. false means it is not.
func loadGTK() bool {
	if gtkTried {
		return fnObjectSetProperty != 0
	}
	gtkTried = true
	lib, err := purego.Dlopen("libgtk-3.so.0", purego.RTLD_NOW|rtldNoload)
	if err != nil {
		return false
	}
	// GObject's calls resolve through GTK's handle: dlsym searches a
	// library's dependencies too.
	syms := []struct {
		fn   *uintptr
		name string
	}{
		{&fnSettingsGetDefault, "gtk_settings_get_default"},
		{&fnValueInit, "g_value_init"},
		{&fnValueGetBoolean, "g_value_get_boolean"},
		{&fnValueSetBoolean, "g_value_set_boolean"},
		{&fnObjectGetProperty, "g_object_get_property"},
		{&fnObjectSetProperty, "g_object_set_property"},
	}
	for _, s := range syms {
		addr, symErr := purego.Dlsym(lib, s.name)
		if symErr != nil || addr == 0 {
			for _, c := range syms {
				*c.fn = 0
			}
			return false
		}
		*s.fn = addr
	}
	return true
}

// gtkSettings returns the default GtkSettings, or 0 when the GTK plugin
// does not draw c's frames.
func (c *Context) gtkSettings() uintptr {
	if !c.gtk || !loadGTK() {
		return 0
	}
	s, _, _ := purego.SyscallN(fnSettingsGetDefault)
	return s
}

// preferDark reads the property from settings s.
func preferDark(s uintptr) bool {
	var v gValue
	purego.SyscallN(fnValueInit, uintptr(unsafe.Pointer(&v)), gTypeBoolean)
	purego.SyscallN(fnObjectGetProperty, s, uintptr(unsafe.Pointer(&propPreferDark[0])),
		uintptr(unsafe.Pointer(&v)))
	r, _, _ := purego.SyscallN(fnValueGetBoolean, uintptr(unsafe.Pointer(&v)))
	return int32(r) != 0 // gboolean is a C int
}

// DarkVariant reports whether the GTK plugin draws c's frames dark. ok is
// false when the GTK plugin does not draw them.
func (c *Context) DarkVariant() (dark, ok bool) {
	s := c.gtkSettings()
	if s == 0 {
		return false, false
	}
	return preferDark(s), true
}

// SetDarkVariant asks the GTK plugin for dark frames, or light ones. It
// reports whether the setting changed: false when it already held, or when
// the GTK plugin does not draw c's frames. A frame shows the change at its
// next repaint, which Frame.Repaint makes.
func (c *Context) SetDarkVariant(dark bool) bool {
	s := c.gtkSettings()
	if s == 0 || preferDark(s) == dark {
		return false
	}
	var b uintptr
	if dark {
		b = 1
	}
	var v gValue
	purego.SyscallN(fnValueInit, uintptr(unsafe.Pointer(&v)), gTypeBoolean)
	purego.SyscallN(fnValueSetBoolean, uintptr(unsafe.Pointer(&v)), b)
	purego.SyscallN(fnObjectSetProperty, s, uintptr(unsafe.Pointer(&propPreferDark[0])),
		uintptr(unsafe.Pointer(&v)))
	return true
}
