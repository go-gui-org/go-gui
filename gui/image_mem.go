package gui

import (
	"container/list"
	"strings"
	"sync"
	"sync/atomic"
)

// In-memory image registry.
//
// Widgets that must draw imagery no gradient can express — a hue wheel, an
// HSL saturation/lightness plane, an alpha checkerboard — generate a pixel
// buffer and register it here. UseImage returns a Src string usable anywhere
// ImageCfg.Src or DrawContext.Image accepts one, and every GPU backend
// resolves that string through LookupImage instead of touching the disk.
//
// Callers are expected to *content-key*: the key names the inputs the buffer
// was generated from ("colorpicker/wheel/l55"), so a buffer that must change
// is registered under a new key and the stale variant ages out on its own.
// There is deliberately no invalidation API — an app that mutates a buffer
// in place has no way to tell the backends their uploaded texture is stale,
// because the backend texture caches key on the same string. New pixels
// reach the backends only as a new registration: under a new key, or under
// the same key after DropImage or at a new size (see the serial below).
// Registering a live key again at the same size keeps its first buffer and
// serial, and mutating the registered slice is never seen.
//
// RegisterEncodedImage entries are pinned. They are typically registered
// once, at startup, with no generator to rebuild them each frame, so the
// budget neither counts nor evicts them; only DropImage removes them.
//
// Backends learn about removals by polling, not by being called: DropImage
// runs on any goroutine, while a texture must be deleted on its window's
// render thread. Each registration carries a serial that is never reused,
// and every removal bumps a generation counter. A backend that sees the
// generation change checks its cached textures against ImageSerial and
// deletes those whose registration is gone, so a drop, an eviction or a
// re-registration of the same key releases the texture and never draws the
// old pixels (#1230).
//
// SECURITY: a mem: buffer bypasses WindowCfg.AllowedImageRoots,
// MaxImageBytes and MaxImagePixels. Those bound *untrusted* input — a file
// path or a URL an attacker might steer. An in-process pixel buffer is
// trusted by construction; the registry's own byte budget is its bound.

// memImagePrefix marks a Src string as naming a registered buffer rather
// than a path, URL, or data URL.
const memImagePrefix = "mem:"

// defaultMemImageBudget caps the registry's total pixel bytes. Content-keyed
// callers register a new key per distinct appearance (one per hue, one per
// lightness), so the budget — not a call to DropImage — is what keeps a long
// drag from growing memory without limit.
const defaultMemImageBudget = 32 << 20 // 32 MiB

// maxMemImageDim caps one registered buffer's edge, in pixels. 4096 is
// beyond any generated control imagery and keeps the dimension × 4 byte
// count far below int overflow on every platform.
const maxMemImageDim = 4096

// memImage is one registered buffer. Pix is NRGBA8, straight (non-
// premultiplied) alpha, W*H*4 bytes, matching imgload.DecodeNRGBA so
// backends upload it with no conversion.
//
// src is the prefixed Src string, built once at registration. Holding it
// here is what lets a generator hand the same string back every frame
// without allocating (see memImageSrc).
type memImage struct {
	src    string
	pix    []byte
	elem   *list.Element // position in the LRU order; value is the key
	w      int
	h      int
	serial uint64 // names this registration; set once, never reused
	pinned bool   // outside the budget; only DropImage removes it
}

// memImageRegistry is a byte-budgeted LRU. It is process-global rather than
// per-window because backends resolve a Src string with no *Window in hand.
//
// data is keyed by the bare key, not the prefixed Src string, so a lookup
// can be spelled data[string(keyBytes)] — which the compiler performs
// without allocating.
//
// lastSerial is read and written only under mu, in the same critical
// section that inserts the entry carrying the new value, so no two
// registrations share a serial. gen is atomic because backends read it
// every frame without taking mu.
type memImageRegistry struct {
	data       map[string]*memImage
	order      list.List // front = least recently used
	bytes      int       // pixel bytes of unpinned entries
	budget     int
	lastSerial uint64
	gen        atomic.Uint64 // bumped by every removal
	mu         sync.Mutex
}

var memImages = memImageRegistry{budget: defaultMemImageBudget}

// UseImage registers pix under key and returns the Src string naming it.
// Passing a key that is already registered with the same dimensions is a
// no-op that refreshes the entry's recency, so callers may call it
// unconditionally each frame; use HasImage to skip generating the buffer at
// all. Call it while building each frame rather than keeping the returned
// Src: once the budget evicts the key, a kept Src resolves to nothing until
// the key is registered again. For an image that is registered once and
// kept, such as a PNG from go:embed, use [RegisterEncodedImage] instead.
//
// The registry takes ownership of pix. Callers must not mutate it
// afterwards: backends cache an uploaded texture under the same key and
// would never observe the change.
//
// pix must be W*H*4 bytes of NRGBA8. A mismatched length, or a
// non-positive dimension, registers nothing and returns "".
//
// exportaudit:keep — the registry is app-facing: it exists so callers
// can draw imagery gradients cannot express.
func UseImage(key string, w, h int, pix []byte) string {
	return useImage(key, w, h, pix, false)
}

// useImage is UseImage with a pin flag. A pinned entry is outside the budget;
// registering a key that is already present at the same size pins it.
func useImage(key string, w, h int, pix []byte, pin bool) string {
	if key == "" || w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return ""
	}
	// w*h*4 above is int arithmetic: for dimensions past a few thousand
	// pixels it can wrap, passing the length check with a corrupt
	// stride. The cap bounds the wrapped product below int64 range on
	// every platform, and the int64 comparison then rejects any buffer
	// whose true byte count does not match its dimensions.
	if w > maxMemImageDim || h > maxMemImageDim ||
		int64(w)*int64(h)*4 != int64(len(pix)) {
		return ""
	}
	memImages.mu.Lock()
	defer memImages.mu.Unlock()

	if mi, ok := memImages.data[key]; ok {
		// Same key, same size: keep the uploaded texture valid by
		// leaving the stored buffer alone. Only recency changes.
		if mi.w == w && mi.h == h {
			if pin {
				memImages.pinLocked(mi)
			}
			memImages.order.MoveToBack(mi.elem)
			return mi.src
		}
		memImages.removeLocked(key)
	}

	if memImages.data == nil {
		memImages.data = make(map[string]*memImage)
	}
	src := memImagePrefix + key
	elem := memImages.order.PushBack(key)
	memImages.lastSerial++
	memImages.data[key] = &memImage{
		src: src, pix: pix, elem: elem, w: w, h: h,
		serial: memImages.lastSerial, pinned: pin,
	}
	// A pinned entry adds no budgeted bytes, so it cannot take the
	// registry over its budget; skipping evictLocked also spares a scan
	// past the pinned entries under mu on every pinned insert.
	if !pin {
		memImages.bytes += len(pix)
		memImages.evictLocked()
	}
	return src
}

// pinImage pins an already registered key and returns its Src. It lets a
// repeat RegisterEncodedImage skip the decode: a key names immutable content.
func pinImage(key string) (string, bool) {
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	mi, ok := memImages.data[key]
	if !ok {
		return "", false
	}
	memImages.pinLocked(mi)
	memImages.order.MoveToBack(mi.elem)
	return mi.src, true
}

// memImageSrc returns the registered Src string for a key held as bytes,
// marking it most recently used. Generators build their content key into a
// scratch buffer each frame and call this: the map lookup on a []byte-to-
// string conversion does not allocate, and the returned string is the one
// stored at registration, so a cache hit costs no allocation at all.
func memImageSrc(key []byte) (string, bool) {
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	mi, ok := memImages.data[string(key)]
	if !ok {
		return "", false
	}
	memImages.order.MoveToBack(mi.elem)
	return mi.src, true
}

// HasImage reports whether key is registered, and marks it most recently
// used. Generators gate on it so a repeat frame costs a map lookup rather
// than a buffer allocation and a pixel loop.
//
// exportaudit:keep — half of the app-facing generate-once idiom with
// UseImage.
func HasImage(key string) bool {
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	mi, ok := memImages.data[key]
	if ok {
		memImages.order.MoveToBack(mi.elem)
	}
	return ok
}

// DropImage unregisters key. Content-keyed callers do not need this — the
// budget evicts stale variants — but an app that registers one large buffer
// for a screen it has left can reclaim the bytes immediately.
//
// For an image from [RegisterEncodedImage] it is the only release: the budget
// never evicts those, so an app that registers images over time (for example
// from the network) drops each one it no longer shows.
//
// Each window's backend deletes its texture of the image on its next frame,
// and a key registered again after the drop draws its new pixels.
//
// exportaudit:keep — app-facing reclaim for the registry.
func DropImage(key string) {
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	memImages.removeLocked(key)
}

// SetMemImageBudget caps the registry's total pixel bytes, evicting the
// least recently used entries until the new budget is met. A non-positive
// budget restores the default. Images registered by RegisterEncodedImage
// neither count toward the budget nor are evicted by it.
//
// exportaudit:keep — app-facing tuning for the registry.
func SetMemImageBudget(bytes int) {
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	if bytes <= 0 {
		bytes = defaultMemImageBudget
	}
	memImages.budget = bytes
	memImages.evictLocked()
}

// MemImage is a registered buffer as LookupImage returns it.
//
// exportaudit:keep — LookupImage's result, consumed by gui/backend/*.
type MemImage struct {
	// Pix is W*H*4 bytes of NRGBA8. It aliases registry storage and must
	// be treated as read-only.
	Pix []byte
	W   int
	H   int
	// Serial names this registration. A key registered again, after a
	// drop, an eviction or at a new size, gets a new Serial, so a backend
	// that stores it with an uploaded texture can tell the texture is
	// stale. It is never 0.
	Serial uint64
}

// LookupImage resolves a Src string to a registered buffer, marking it most
// recently used. It returns ok=false for any string that is not a mem:
// source, and for a mem: source whose key is not registered; backends
// check IsMemImage first so a dropped mem: source never reaches their path
// handling.
//
// The pixels and the serial come from one locked lookup, so a backend
// never pairs one registration's pixels with another's serial.
//
// exportaudit:keep — consumed by gui/backend/* to resolve mem: sources.
func LookupImage(src string) (MemImage, bool) {
	if !isMemImage(src) {
		return MemImage{}, false
	}
	key := src[len(memImagePrefix):] // substring; no allocation
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	mi, found := memImages.data[key]
	if !found {
		return MemImage{}, false
	}
	memImages.order.MoveToBack(mi.elem)
	return MemImage{Pix: mi.pix, W: mi.w, H: mi.h, Serial: mi.serial}, true
}

// ImageSerial reports the serial of the registration src names now, or
// ok=false when src is not a mem: source or its key is not registered.
// Unlike LookupImage it leaves the LRU order alone: a backend checking its
// cached textures must not keep images alive that nothing draws.
//
// exportaudit:keep — consumed by gui/backend/* to release stale textures.
func ImageSerial(src string) (serial uint64, ok bool) {
	if !isMemImage(src) {
		return 0, false
	}
	key := src[len(memImagePrefix):]
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	mi, found := memImages.data[key]
	if !found {
		return 0, false
	}
	return mi.serial, true
}

// ImageRegistryGen returns a counter that changes whenever an image leaves
// the registry: DropImage, budget eviction, or registration of a key at a
// new size. A backend keeps the value it last checked its textures at and
// checks again (with ImageSerial) only when the counter has moved, so a
// frame with no removal costs one atomic load.
//
// exportaudit:keep — consumed by gui/backend/* to release stale textures.
func ImageRegistryGen() uint64 {
	return memImages.gen.Load()
}

// IsMemImage reports whether src names an in-memory buffer, registered or
// not. Backends use it to keep a dropped mem: source out of their file
// path handling.
//
// exportaudit:keep — consumed by gui/backend/* to route mem: sources.
func IsMemImage(src string) bool {
	return isMemImage(src)
}

// isMemImage reports whether src names a registered in-memory buffer. It
// sits beside isDataURL and isHTTPURL (gui/image_download.go) as the third
// non-filesystem source form.
func isMemImage(src string) bool {
	return strings.HasPrefix(src, memImagePrefix)
}

// removeLocked deletes one entry and its bytes. Every removal goes through
// here — DropImage, budget eviction, re-registration at a new size — so the
// generation bump below is what tells backends their texture may be stale.
// Caller holds mu.
func (r *memImageRegistry) removeLocked(key string) {
	mi, ok := r.data[key]
	if !ok {
		return
	}
	r.order.Remove(mi.elem)
	delete(r.data, key)
	if !mi.pinned {
		r.bytes -= len(mi.pix)
	}
	r.gen.Add(1)
}

// pinLocked moves mi out of the budget. Caller holds mu.
func (r *memImageRegistry) pinLocked(mi *memImage) {
	if !mi.pinned {
		mi.pinned = true
		r.bytes -= len(mi.pix)
	}
}

// evictLocked drops least-recently-used entries until the registry fits its
// budget. The most recent entry is never evicted: a single buffer larger
// than the whole budget is still the one the caller is about to draw.
// Pinned entries are skipped, and "most recent" means the newest unpinned
// entry: a pinned one adds nothing to the budget, so registering or drawing
// it must not cost a generated buffer its place. Caller holds mu.
func (r *memImageRegistry) evictLocked() {
	if r.bytes <= r.budget {
		return
	}
	newest := r.order.Back()
	for newest != nil && r.data[newest.Value.(string)].pinned {
		newest = newest.Prev()
	}
	for e := r.order.Front(); e != nil && e != newest && r.bytes > r.budget; {
		next := e.Next()
		if key := e.Value.(string); !r.data[key].pinned {
			r.removeLocked(key)
		}
		e = next
	}
}
