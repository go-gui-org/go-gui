//go:build js || android || ios

package main

// startAmbience is a no-op on targets where gui/audio does not build
// (wasm/mobile). The orrery runs without background sound there.
func startAmbience() {}
