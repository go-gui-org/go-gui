package gui

import (
	"container/list"
	"strings"
	"sync"
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
// because the backend texture caches key on the same string.
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

// memImage is one registered buffer. Pix is NRGBA8, straight (non-
// premultiplied) alpha, W*H*4 bytes, matching imgload.DecodeNRGBA so
// backends upload it with no conversion.
//
// src is the prefixed Src string, built once at registration. Holding it
// here is what lets a generator hand the same string back every frame
// without allocating (see memImageSrc).
type memImage struct {
	src  string
	pix  []byte
	elem *list.Element // position in the LRU order; value is the key
	w    int
	h    int
}

// memImageRegistry is a byte-budgeted LRU. It is process-global rather than
// per-window because backends resolve a Src string with no *Window in hand.
//
// data is keyed by the bare key, not the prefixed Src string, so a lookup
// can be spelled data[string(keyBytes)] — which the compiler performs
// without allocating.
type memImageRegistry struct {
	data   map[string]*memImage
	order  list.List // front = least recently used
	bytes  int
	budget int
	mu     sync.Mutex
}

var memImages = memImageRegistry{budget: defaultMemImageBudget}

// UseImage registers pix under key and returns the Src string naming it.
// Passing a key that is already registered with the same dimensions is a
// no-op that refreshes the entry's recency, so callers may call it
// unconditionally each frame; use HasImage to skip generating the buffer at
// all.
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
	if key == "" || w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return ""
	}
	memImages.mu.Lock()
	defer memImages.mu.Unlock()

	if mi, ok := memImages.data[key]; ok {
		// Same key, same size: keep the uploaded texture valid by
		// leaving the stored buffer alone. Only recency changes.
		if mi.w == w && mi.h == h {
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
	memImages.data[key] = &memImage{
		src: src, pix: pix, elem: elem, w: w, h: h,
	}
	memImages.bytes += len(pix)
	memImages.evictLocked()
	return src
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
// exportaudit:keep — app-facing reclaim for the registry.
func DropImage(key string) {
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	memImages.removeLocked(key)
}

// SetMemImageBudget caps the registry's total pixel bytes, evicting the
// least recently used entries until the new budget is met. A non-positive
// budget restores the default.
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

// LookupImage resolves a Src string to a registered buffer, marking it most
// recently used. It returns ok=false for any string that is not a mem:
// source, so backends can call it first and fall through to their path and
// URL handling unchanged.
//
// The returned slice aliases registry storage and must be treated as
// read-only.
//
// exportaudit:keep — consumed by gui/backend/* to resolve mem: sources.
func LookupImage(src string) (w, h int, pix []byte, ok bool) {
	if !isMemImage(src) {
		return 0, 0, nil, false
	}
	key := src[len(memImagePrefix):] // substring; no allocation
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	mi, found := memImages.data[key]
	if !found {
		return 0, 0, nil, false
	}
	memImages.order.MoveToBack(mi.elem)
	return mi.w, mi.h, mi.pix, true
}

// isMemImage reports whether src names a registered in-memory buffer. It
// sits beside isDataURL and isHTTPURL (gui/image_download.go) as the third
// non-filesystem source form.
func isMemImage(src string) bool {
	return strings.HasPrefix(src, memImagePrefix)
}

// removeLocked deletes one entry and its bytes. Caller holds mu.
func (r *memImageRegistry) removeLocked(src string) {
	mi, ok := r.data[src]
	if !ok {
		return
	}
	r.order.Remove(mi.elem)
	delete(r.data, src)
	r.bytes -= len(mi.pix)
}

// evictLocked drops least-recently-used entries until the registry fits its
// budget. The most recent entry is never evicted: a single buffer larger
// than the whole budget is still the one the caller is about to draw.
// Caller holds mu.
func (r *memImageRegistry) evictLocked() {
	for r.bytes > r.budget && r.order.Len() > 1 {
		front := r.order.Front()
		r.removeLocked(front.Value.(string))
	}
}
