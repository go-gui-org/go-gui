//go:build !js && !darwin

package glbind

// This file holds the per-frame ("hot") half of the binding. Everything here
// calls its GL entry point through a raw address and sysN instead of through a
// purego.RegisterFunc closure.
//
// Why: purego.RegisterFunc returns a reflect.MakeFunc closure. Every call
// boxes its arguments into reflect.Values, allocates three argument-marshalling
// closures and a keepAlive slice. Measured on windows/amd64 against a
// three-argument kernel32 export, that is 304 B and 6 allocations per call, at
// 1246 ns/op against 165 ns/op for syscall.SyscallN on the same address. In
// the heap profile attached to #812 the wrapper accounted for 43% of every
// byte the application allocated — more than text shaping, rasterization and
// layout combined — because a frame issues hundreds of GL calls.
//
// Only entry points on the per-frame path live here. The cold ones (shader
// compilation, object creation, queries) stay on purego in glbind.go, where
// its argument marshalling is worth the type safety and a few allocations at
// startup cost nothing.
//
// Two rules govern what may be added:
//
//  1. No float or double parameters. sysN passes integer registers only;
//     a float would be delivered in the wrong register. glClearColor is the
//     one float entry point GL-side and it stays on purego for this reason.
//  2. No pointer parameters. Converting a Go pointer to uintptr is only safe
//     inside the SyscallN call expression itself, where the compiler keeps the
//     referent alive (//go:uintptrkeepalive). Routing one through sysN would
//     let the GC collect it mid-call. The five pointer-carrying hot entry
//     points therefore call SyscallN directly, in raw_windows.go and
//     raw_other.go.

// Raw entry-point addresses, populated by InitWithProcAddrFunc from
// rawBindings(). Zero until then, exactly as the pfn vars are nil until then.
var (
	addrActiveTexture           uintptr
	addrBindBuffer              uintptr
	addrBindFramebuffer         uintptr
	addrBindTexture             uintptr
	addrBindVertexArray         uintptr
	addrBlitFramebuffer         uintptr
	addrBufferData              uintptr
	addrBufferSubData           uintptr
	addrClear                   uintptr
	addrColorMask               uintptr
	addrDisable                 uintptr
	addrDrawArrays              uintptr
	addrDrawElements            uintptr
	addrEnable                  uintptr
	addrEnableVertexAttribArray uintptr
	addrScissor                 uintptr
	addrStencilFunc             uintptr
	addrStencilOp               uintptr
	addrTexSubImage2D           uintptr
	addrUniform1i               uintptr
	addrUniformMatrix4fv        uintptr
	addrUseProgram              uintptr
	addrVertexAttribPointer     uintptr
	addrViewport                uintptr
)

// b2u marshals a Go bool as GLboolean. purego does this for the cold half;
// the raw path spells it out. GL reads the low byte, so 0 and 1 suffice.
func b2u(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// Signed parameters convert to uintptr by two's-complement wrap, which
// sign-extends into the 64-bit register. A callee reading a 32-bit GLint or
// GLsizei takes the low half, so negative values (a scissor box outside the
// viewport, say) arrive intact.

func ActiveTexture(texture uint32) { sysN(addrActiveTexture, uintptr(texture)) }

func BindBuffer(target uint32, buffer uint32) {
	sysN(addrBindBuffer, uintptr(target), uintptr(buffer))
}

// BindFramebuffer is on the raw path because the antialiased frame (#823)
// binds its multisampled target, and the filter targets, several times a frame.
func BindFramebuffer(target uint32, framebuffer uint32) {
	sysN(addrBindFramebuffer, uintptr(target), uintptr(framebuffer))
}

func BindTexture(target uint32, texture uint32) {
	sysN(addrBindTexture, uintptr(target), uintptr(texture))
}

func BindVertexArray(array uint32) { sysN(addrBindVertexArray, uintptr(array)) }

// BlitFramebuffer copies a rectangle from the read framebuffer to the draw
// framebuffer. The backend calls it once a frame to resolve its multisampled
// target into the window (#823), and once per filter container.
func BlitFramebuffer(srcX0 int32, srcY0 int32, srcX1 int32, srcY1 int32, dstX0 int32, dstY0 int32, dstX1 int32, dstY1 int32, mask uint32, filter uint32) {
	sysN(addrBlitFramebuffer, uintptr(srcX0), uintptr(srcY0), uintptr(srcX1),
		uintptr(srcY1), uintptr(dstX0), uintptr(dstY0), uintptr(dstX1),
		uintptr(dstY1), uintptr(mask), uintptr(filter))
}

func Clear(mask uint32) { sysN(addrClear, uintptr(mask)) }

func ColorMask(red bool, green bool, blue bool, alpha bool) {
	sysN(addrColorMask, b2u(red), b2u(green), b2u(blue), b2u(alpha))
}

func Disable(cap uint32) { sysN(addrDisable, uintptr(cap)) }

func DrawArrays(mode uint32, first int32, count int32) {
	sysN(addrDrawArrays, uintptr(mode), uintptr(first), uintptr(count))
}

func Enable(cap uint32) { sysN(addrEnable, uintptr(cap)) }

func EnableVertexAttribArray(index uint32) {
	sysN(addrEnableVertexAttribArray, uintptr(index))
}

func Scissor(x int32, y int32, width int32, height int32) {
	sysN(addrScissor, uintptr(x), uintptr(y), uintptr(width), uintptr(height))
}

func StencilFunc(xfunc uint32, ref int32, mask uint32) {
	sysN(addrStencilFunc, uintptr(xfunc), uintptr(ref), uintptr(mask))
}

func StencilOp(fail uint32, zfail uint32, zpass uint32) {
	sysN(addrStencilOp, uintptr(fail), uintptr(zfail), uintptr(zpass))
}

func Uniform1i(location int32, v0 int32) {
	sysN(addrUniform1i, uintptr(location), uintptr(v0))
}

func UseProgram(program uint32) { sysN(addrUseProgram, uintptr(program)) }

// VertexAttribPointerWithOffset mirrors go-gl's helper of the same name: the
// final argument is a byte offset into the buffer currently bound to
// ARRAY_BUFFER, not a client-memory pointer. go-gl declares it uintptr_t for
// the same reason, and keeping it uintptr end to end avoids a bogus
// unsafe.Pointer conversion that go vet's unsafeptr check would (correctly)
// reject. It is also why this one belongs on the pointer-free path despite its
// uintptr parameter: there is nothing for the GC to keep alive.
func VertexAttribPointerWithOffset(index uint32, size int32, xtype uint32, normalized bool, stride int32, offset uintptr) {
	sysN(addrVertexAttribPointer, uintptr(index), uintptr(size), uintptr(xtype),
		b2u(normalized), uintptr(stride), offset)
}

func Viewport(x int32, y int32, width int32, height int32) {
	sysN(addrViewport, uintptr(x), uintptr(y), uintptr(width), uintptr(height))
}
