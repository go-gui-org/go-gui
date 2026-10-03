// Package decor binds libdecor through purego, for the experimental Wayland
// backend (#919). libdecor draws a window frame (title bar, borders,
// buttons) on compositors that leave that to the client, GNOME's mutter
// above all. It is optional: Load fails on a system without it, and the
// window then has no frame.
//
// libdecor makes the xdg_surface and xdg_toplevel for a surface itself and
// reports the toplevel's configure events through a Handler. The caller
// keeps rendering into its own wl_surface; libdecor adds the frame around
// it as subsurfaces.
//
// Not safe for concurrent use: every call, and every Handler method, runs
// on the goroutine that drives the Wayland connection.
package decor
