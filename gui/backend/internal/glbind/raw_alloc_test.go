//go:build !js && !darwin

package glbind

import (
	"testing"
	"unsafe"
)

// The gate for #812. The raw half of the binding exists for exactly one
// property — a GL call that allocates nothing — and that property is invisible
// in the source: it depends on whether escape analysis keeps sysN's variadic
// slice on the stack, and on syscall.SyscallN not retaining its arguments. A
// refactor that reintroduces a reflect-based wrapper, or that routes a pointer
// through a helper carrying //go:uintptrescapes, compiles, passes every other
// test in this package and silently restores the 304 bytes per call that #812
// measured. Only an allocation count catches it.
//
// Calling a real GL entry point needs a current context, which a unit test does
// not have. The wrappers are driven against a harmless platform export instead
// (harmlessProcAddr): the arguments are meaningless to it and its return value
// is discarded. What is exercised is the Go side — argument marshalling and the
// call sequence — which is where the allocations were.

// hotCalls names each wrapper on the raw path with a call that passes
// representative arguments. A wrapper added to raw.go without a row here is
// reported by TestHotCallsCoverRawBindings.
//
// The pointer-carrying calls declare their referent *inside* the closure, which
// is what gui/backend/gl does: gpu.BuildQuad returns a vertex array by value
// and drawShader builds its `var tm [16]float32` per call, both function
// locals. Hoisting them out here would move the allocation to setup time and
// measure zero on every platform, hiding the one real per-platform difference
// this gate exists to pin down (ptrArgAllocs).
func hotCalls() map[string]func() {
	return map[string]func(){
		"glActiveTexture":   func() { ActiveTexture(TEXTURE0) },
		"glBindBuffer":      func() { BindBuffer(ARRAY_BUFFER, 1) },
		"glBindTexture":     func() { BindTexture(TEXTURE_2D, 1) },
		"glBindVertexArray": func() { BindVertexArray(1) },
		"glBufferData": func() {
			var pixels [4]byte
			BufferData(ARRAY_BUFFER, 4, unsafe.Pointer(&pixels[0]), DYNAMIC_DRAW)
		},
		"glBufferSubData": func() {
			var pixels [4]byte
			BufferSubData(ARRAY_BUFFER, 0, 4, unsafe.Pointer(&pixels[0]))
		},
		"glClear":                   func() { Clear(COLOR_BUFFER_BIT) },
		"glColorMask":               func() { ColorMask(true, true, true, false) },
		"glDisable":                 func() { Disable(BLEND) },
		"glDrawArrays":              func() { DrawArrays(TRIANGLES, 0, 6) },
		"glDrawElements":            func() { DrawElements(TRIANGLES, 6, UNSIGNED_SHORT, nil) },
		"glEnable":                  func() { Enable(BLEND) },
		"glEnableVertexAttribArray": func() { EnableVertexAttribArray(0) },
		"glScissor":                 func() { Scissor(-1, -1, 640, 480) },
		"glStencilFunc":             func() { StencilFunc(ALWAYS, 0, 0xff) },
		"glStencilOp":               func() { StencilOp(KEEP, KEEP, INCR) },
		"glTexSubImage2D": func() {
			var pixels [4]byte
			TexSubImage2D(TEXTURE_2D, 0, 0, 0, 1, 1, RGBA, UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
		},
		"glUniform1i": func() { Uniform1i(0, 1) },
		"glUniformMatrix4fv": func() {
			var matrix [16]float32
			UniformMatrix4fv(0, 1, false, &matrix[0])
		},
		"glUseProgram":          func() { UseProgram(1) },
		"glVertexAttribPointer": func() { VertexAttribPointerWithOffset(0, 2, FLOAT, false, 16, 8) },
		"glViewport":            func() { Viewport(0, 0, 640, 480) },
	}
}

// TestHotCallsCoverRawBindings keeps the alloc gate honest. Without it, adding
// a row to rawBindings and forgetting one here leaves the new wrapper
// unmeasured — the failure mode the gate is supposed to prevent.
func TestHotCallsCoverRawBindings(t *testing.T) {
	calls := hotCalls()
	for _, b := range rawBindings() {
		if calls[b.name] == nil {
			t.Errorf("%s is on the raw path with no entry in hotCalls", b.name)
		}
		delete(calls, b.name)
	}
	for name := range calls {
		t.Errorf("hotCalls has %s, which is not in rawBindings", name)
	}
}

// pointerArgCalls names the wrappers that hand a live Go pointer to the driver,
// and so answer to ptrArgAllocs rather than to zero.
//
// glDrawElements is not among them although its parameter is an
// unsafe.Pointer: every call site passes nil, because the indices come from the
// bound ELEMENT_ARRAY_BUFFER. A nil has no referent to keep alive, so it costs
// nothing on any platform. Should a call site ever pass client memory, add it
// here — and read raw.go first.
var pointerArgCalls = map[string]bool{
	"glBufferData":       true,
	"glBufferSubData":    true,
	"glTexSubImage2D":    true,
	"glUniformMatrix4fv": true,
}

// TestRawCallsDoNotAllocate is the gate itself.
func TestRawCallsDoNotAllocate(t *testing.T) {
	proc := harmlessProcAddr(t)
	for _, b := range rawBindings() {
		*b.addr = proc
	}
	t.Cleanup(func() {
		for _, b := range rawBindings() {
			*b.addr = 0
		}
	})

	for name, call := range hotCalls() {
		t.Run(name, func(t *testing.T) {
			want := 0.0
			if pointerArgCalls[name] {
				want = ptrArgAllocs
			}
			// One warm-up call: the first call through a wrapper can fault in
			// pages that AllocsPerRun would count as the wrapper's own.
			call()
			if got := testing.AllocsPerRun(100, call); got > want {
				t.Errorf("%s allocates %.1f times per call, want at most %.0f",
					name, got, want)
			}
		})
	}
}

// BenchmarkRawCall records the per-call cost of the raw path so a regression
// shows up as a number, not only as a failed assertion.
func BenchmarkRawCall(b *testing.B) {
	proc := harmlessProcAddr(b)
	addrDrawArrays = proc
	b.Cleanup(func() { addrDrawArrays = 0 })

	b.ReportAllocs()
	for b.Loop() {
		DrawArrays(TRIANGLES, 0, 6)
	}
}
