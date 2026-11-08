// Package memtex keeps a renderer's textures of gui's in-memory images
// (mem: sources) in step with the registry.
//
// gui cannot call into a renderer when an image leaves the registry: the
// removal runs on any goroutine, while a texture must be deleted on its
// window's render thread. So the registry counts removals and stamps every
// registration with a serial, and a renderer polls: once per frame it asks
// whether the count moved (Sweeper.Due), and only then checks each cached
// texture against the serial it was uploaded from (Current) (#1230).
package memtex

import "github.com/go-gui-org/go-gui/gui"

// Sweeper remembers the registry generation a renderer last checked its
// textures at. Its zero value is ready to use. It is confined to the
// renderer's thread.
type Sweeper struct {
	gen uint64
}

// Due reports whether an image has left the registry since the last call
// that returned true. A frame with no removal costs one atomic load.
//
// It records the generation before the caller checks its textures, so a
// removal during that check makes the next call return true again.
func (s *Sweeper) Due() bool {
	gen := gui.ImageRegistryGen()
	if gen == s.gen {
		return false
	}
	s.gen = gen
	return true
}

// Current reports whether a texture cached under key, uploaded from the
// registration with the given serial, may stay. Keys that are not mem:
// sources (file paths, URLs) always may: their cache's own eviction owns
// them. A mem: texture stays only while its key is still registered under
// the same serial — not dropped, evicted or registered again.
func Current(key string, serial uint64) bool {
	if !gui.IsMemImage(key) {
		return true
	}
	now, ok := gui.ImageSerial(key)
	return ok && now == serial
}
