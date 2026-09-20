//go:build !darwin && !linux && !windows && !(js && wasm)

package gui

// Default font families for every GOOS without a named system UI font
// here. The per-platform files cover darwin, linux, windows and
// js/wasm; without this one a build for freebsd, openbsd, netbsd or
// any other target failed to compile on two missing constants rather
// than falling back.
//
// Empty is the same answer the Linux file gives: the text shaper picks
// its own default family, which is the right behaviour where the
// toolkit has no opinion about the system font.
const defaultFontFamily = ""
const defaultMonoFontFamily = "monospace"
