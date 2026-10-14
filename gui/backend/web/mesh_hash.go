//go:build js && wasm

package web

import (
	"encoding/binary"
	"math/bits"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// meshHash fingerprints r's triangles and vertex colors, so the mesh
// caches notice a buffer rewritten in place under the same address.
//
// It is a change detector, not a general-purpose hash: it runs over
// every cached mesh every frame (megabytes on an SVG-heavy page), and
// hash/maphash's software fallback — what wasm gets, having no AES —
// made it a third of the frame. This takes 8 bytes per step with one
// 64-bit xor and multiply, which wasm runs natively, in four lanes so
// the engine can overlap them.
//
// Each lane step h = (h ^ w) * k is a bijection of h for odd k, so a
// buffer differing from the hashed one in a single word always changes
// its lane, and the lanes are combined by bijective steps too.
func meshHash(r *gui.RenderCmd) uint64 {
	h := hashWords(0, unsafe.Slice((*byte)(unsafe.Pointer(&r.Triangles[0])),
		len(r.Triangles)*4))
	if n := len(r.VertexColors); n > 0 {
		h = hashWords(h, unsafe.Slice((*byte)(unsafe.Pointer(&r.VertexColors[0])),
			n*int(unsafe.Sizeof(r.VertexColors[0]))))
	}
	return h
}

const (
	meshHashK    = 0x9E3779B97F4A7C15 // odd: multiplication is a bijection
	meshHashSeed = 0x243F6A8885A308D3
)

// hashWords folds b into h. The length is mixed in first, so buffers
// that differ only by trailing zero bytes hash differently.
func hashWords(h uint64, b []byte) uint64 {
	l0 := (h ^ meshHashSeed ^ uint64(len(b))) * meshHashK
	l1, l2, l3 := l0+1, l0+2, l0+3
	for len(b) >= 32 {
		l0 = (l0 ^ binary.LittleEndian.Uint64(b)) * meshHashK
		l1 = (l1 ^ binary.LittleEndian.Uint64(b[8:])) * meshHashK
		l2 = (l2 ^ binary.LittleEndian.Uint64(b[16:])) * meshHashK
		l3 = (l3 ^ binary.LittleEndian.Uint64(b[24:])) * meshHashK
		b = b[32:]
	}
	for len(b) >= 8 {
		l0 = (l0 ^ binary.LittleEndian.Uint64(b)) * meshHashK
		b = b[8:]
	}
	if len(b) > 0 {
		var tail [8]byte
		copy(tail[:], b)
		l0 = (l0 ^ binary.LittleEndian.Uint64(tail[:])) * meshHashK
	}
	// Combine: each step is a bijection of the running value, so a
	// change in any one lane reaches the result.
	h = l0
	h = (bits.RotateLeft64(h, 23) ^ l1) * meshHashK
	h = (bits.RotateLeft64(h, 23) ^ l2) * meshHashK
	h = (bits.RotateLeft64(h, 23) ^ l3) * meshHashK
	return h ^ h>>32
}
