package gui

import "unsafe"

// shaderHashCacheMax bounds ShaderHashCache. A frame rarely draws more
// than a handful of distinct shaders; an app that builds a new *Shader
// every frame would otherwise grow the map without limit.
const shaderHashCacheMax = 64

// ShaderHashCache memoizes ShaderHash per *Shader. ShaderHash walks
// every byte of all four source strings, and a backend calls it on
// every draw of every frame to find the compiled pipeline. A ported
// Shadertoy shader is many KB of source, so the walk costs tens of
// microseconds per draw (BenchmarkShaderHash). A hit here costs a map
// lookup and four pointer compares, with no allocation.
//
// Keep one per window, used from the goroutine that draws it. It is
// not safe for concurrent use.
//
// The zero value is ready to use.
type ShaderHashCache struct {
	entries map[*Shader]shaderHashEntry
}

// shaderHashEntry records the strings a hash was computed from. Go
// strings are immutable, so the same data pointer and length means the
// same bytes. The entry holds the pointers, so the GC cannot free the
// bytes and reuse the address for other text. A field reassigned on
// the same *Shader gets a new pointer (or length) and misses.
type shaderHashEntry struct {
	src  [4]*byte
	lens [4]int
	// channels is Shader.Surface != nil, which changes the wrapper.
	channels bool
	hash     uint64
}

// Hash returns ShaderHash(s), computed once per *Shader while its
// source strings stay the same. A nil shader hashes to 0.
func (c *ShaderHashCache) Hash(s *Shader) uint64 {
	if s == nil {
		return 0
	}
	key := shaderHashEntry{
		src: [4]*byte{
			unsafe.StringData(s.Metal), unsafe.StringData(s.GLSL),
			unsafe.StringData(s.MetalDecls), unsafe.StringData(s.GLSLDecls),
		},
		lens:     [4]int{len(s.Metal), len(s.GLSL), len(s.MetalDecls), len(s.GLSLDecls)},
		channels: s.Surface != nil,
	}
	if e, ok := c.entries[s]; ok && e.src == key.src && e.lens == key.lens && e.channels == key.channels {
		return e.hash
	}
	if c.entries == nil {
		c.entries = make(map[*Shader]shaderHashEntry, 4)
	} else if len(c.entries) >= shaderHashCacheMax {
		// Dropping everything is simpler than LRU and cheap: the
		// shaders still on screen re-hash once on the next frame.
		clear(c.entries)
	}
	key.hash = ShaderHash(s)
	c.entries[s] = key
	return key.hash
}
