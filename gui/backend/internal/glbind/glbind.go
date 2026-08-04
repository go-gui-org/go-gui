//go:build !js && !darwin

// Package glbind is a CGo-free binding for the subset of OpenGL 3.3 core that
// gui/backend/gl uses.
//
// It replaces github.com/go-gl/gl, which was the sole CGo dependency of the
// Linux and Windows backends. Entry points are resolved through the caller's
// platform proc-address function (eglGetProcAddress on Linux,
// wglGetProcAddress + opengl32.dll on Windows) and bound with
// github.com/ebitengine/purego — the same mechanism gui/backend/gl already
// uses for EGL itself.
//
// Signatures deliberately mirror go-gl's exactly, including SCREAMING_SNAKE
// enum names and the `bool` spelling of GLboolean parameters, so that call
// sites need nothing beyond an import swap. purego marshals Go `bool` as
// C `_Bool` (one byte), which matches GLboolean.
//
// Call InitWithProcAddrFunc before any other function in this package; every
// wrapper below dereferences a function pointer that is nil until then.
package glbind

import "unsafe"

// C-level entry points, populated by InitWithProcAddrFunc. Parameter types are
// the purego equivalents of the GL types: GLenum/GLuint -> uint32, GLint/GLsizei
// -> int32, GLboolean -> bool, GLsizeiptr/GLintptr -> int, pointers ->
// unsafe.Pointer or a typed Go pointer (purego passes both as void*).
var (
	pfnActiveTexture           func(texture uint32)
	pfnAttachShader            func(program, shader uint32)
	pfnBindBuffer              func(target, buffer uint32)
	pfnBindFramebuffer         func(target, framebuffer uint32)
	pfnBindRenderbuffer        func(target, renderbuffer uint32)
	pfnBindTexture             func(target, texture uint32)
	pfnBindVertexArray         func(array uint32)
	pfnBlendFunc               func(sfactor, dfactor uint32)
	pfnBufferData              func(target uint32, size int, data unsafe.Pointer, usage uint32)
	pfnBufferSubData           func(target uint32, offset, size int, data unsafe.Pointer)
	pfnCheckFramebufferStatus  func(target uint32) uint32
	pfnClear                   func(mask uint32)
	pfnClearColor              func(red, green, blue, alpha float32)
	pfnColorMask               func(red, green, blue, alpha bool)
	pfnCompileShader           func(shader uint32)
	pfnCreateProgram           func() uint32
	pfnCreateShader            func(xtype uint32) uint32
	pfnDeleteBuffers           func(n int32, buffers *uint32)
	pfnDeleteFramebuffers      func(n int32, framebuffers *uint32)
	pfnDeleteProgram           func(program uint32)
	pfnDeleteRenderbuffers     func(n int32, renderbuffers *uint32)
	pfnDeleteShader            func(shader uint32)
	pfnDeleteTextures          func(n int32, textures *uint32)
	pfnDeleteVertexArrays      func(n int32, arrays *uint32)
	pfnDisable                 func(cap uint32)
	pfnDrawArrays              func(mode uint32, first, count int32)
	pfnDrawElements            func(mode uint32, count int32, xtype uint32, indices unsafe.Pointer)
	pfnEnable                  func(cap uint32)
	pfnEnableVertexAttribArray func(index uint32)
	pfnFramebufferRenderbuffer func(target, attachment, renderbuffertarget, renderbuffer uint32)
	pfnFramebufferTexture2D    func(target, attachment, textarget, texture uint32, level int32)
	pfnGenBuffers              func(n int32, buffers *uint32)
	pfnGenFramebuffers         func(n int32, framebuffers *uint32)
	pfnGenRenderbuffers        func(n int32, renderbuffers *uint32)
	pfnGenTextures             func(n int32, textures *uint32)
	pfnGenVertexArrays         func(n int32, arrays *uint32)
	pfnGetProgramInfoLog       func(program uint32, bufSize int32, length *int32, infoLog *uint8)
	pfnGetProgramiv            func(program, pname uint32, params *int32)
	pfnGetShaderInfoLog        func(shader uint32, bufSize int32, length *int32, infoLog *uint8)
	pfnGetShaderiv             func(shader, pname uint32, params *int32)
	pfnGetUniformLocation      func(program uint32, name *uint8) int32
	pfnLinkProgram             func(program uint32)
	pfnRenderbufferStorage     func(target, internalformat uint32, width, height int32)
	pfnScissor                 func(x, y, width, height int32)
	pfnShaderSource            func(shader uint32, count int32, xstring **uint8, length *int32)
	pfnStencilFunc             func(xfunc uint32, ref int32, mask uint32)
	pfnStencilOp               func(fail, zfail, zpass uint32)
	pfnTexImage2D              func(target uint32, level, internalformat, width, height, border int32, format, xtype uint32, pixels unsafe.Pointer)
	pfnTexParameteri           func(target, pname uint32, param int32)
	pfnTexSubImage2D           func(target uint32, level, xoffset, yoffset, width, height int32, format, xtype uint32, pixels unsafe.Pointer)
	pfnUniform1i               func(location, v0 int32)
	pfnUniformMatrix4fv        func(location, count int32, transpose bool, value *float32)
	pfnUseProgram              func(program uint32)
	// The last parameter is declared uintptr rather than unsafe.Pointer for
	// the same reason go-gl declares it uintptr_t: with a buffer bound to
	// ARRAY_BUFFER it is a byte offset, not a pointer into any address space.
	// Keeping it uintptr end to end avoids a bogus unsafe.Pointer conversion
	// that go vet's unsafeptr check would (correctly) reject.
	pfnVertexAttribPointer func(index uint32, size int32, xtype uint32, normalized bool, stride int32, offset uintptr)
	pfnViewport            func(x, y, width, height int32)
)

func ActiveTexture(texture uint32) { pfnActiveTexture(texture) }

func AttachShader(program uint32, shader uint32) { pfnAttachShader(program, shader) }

func BindBuffer(target uint32, buffer uint32) { pfnBindBuffer(target, buffer) }

func BindFramebuffer(target uint32, framebuffer uint32) { pfnBindFramebuffer(target, framebuffer) }

func BindRenderbuffer(target uint32, renderbuffer uint32) {
	pfnBindRenderbuffer(target, renderbuffer)
}

func BindTexture(target uint32, texture uint32) { pfnBindTexture(target, texture) }

func BindVertexArray(array uint32) { pfnBindVertexArray(array) }

func BlendFunc(sfactor uint32, dfactor uint32) { pfnBlendFunc(sfactor, dfactor) }

func BufferData(target uint32, size int, data unsafe.Pointer, usage uint32) {
	pfnBufferData(target, size, data, usage)
}

func BufferSubData(target uint32, offset int, size int, data unsafe.Pointer) {
	pfnBufferSubData(target, offset, size, data)
}

func CheckFramebufferStatus(target uint32) uint32 { return pfnCheckFramebufferStatus(target) }

func Clear(mask uint32) { pfnClear(mask) }

func ClearColor(red float32, green float32, blue float32, alpha float32) {
	pfnClearColor(red, green, blue, alpha)
}

func ColorMask(red bool, green bool, blue bool, alpha bool) {
	pfnColorMask(red, green, blue, alpha)
}

func CompileShader(shader uint32) { pfnCompileShader(shader) }

func CreateProgram() uint32 { return pfnCreateProgram() }

func CreateShader(xtype uint32) uint32 { return pfnCreateShader(xtype) }

func DeleteBuffers(n int32, buffers *uint32) { pfnDeleteBuffers(n, buffers) }

func DeleteFramebuffers(n int32, framebuffers *uint32) { pfnDeleteFramebuffers(n, framebuffers) }

func DeleteProgram(program uint32) { pfnDeleteProgram(program) }

func DeleteRenderbuffers(n int32, renderbuffers *uint32) {
	pfnDeleteRenderbuffers(n, renderbuffers)
}

func DeleteShader(shader uint32) { pfnDeleteShader(shader) }

func DeleteTextures(n int32, textures *uint32) { pfnDeleteTextures(n, textures) }

func DeleteVertexArrays(n int32, arrays *uint32) { pfnDeleteVertexArrays(n, arrays) }

func Disable(cap uint32) { pfnDisable(cap) }

func DrawArrays(mode uint32, first int32, count int32) { pfnDrawArrays(mode, first, count) }

func DrawElements(mode uint32, count int32, xtype uint32, indices unsafe.Pointer) {
	pfnDrawElements(mode, count, xtype, indices)
}

func Enable(cap uint32) { pfnEnable(cap) }

func EnableVertexAttribArray(index uint32) { pfnEnableVertexAttribArray(index) }

func FramebufferRenderbuffer(target uint32, attachment uint32, renderbuffertarget uint32, renderbuffer uint32) {
	pfnFramebufferRenderbuffer(target, attachment, renderbuffertarget, renderbuffer)
}

func FramebufferTexture2D(target uint32, attachment uint32, textarget uint32, texture uint32, level int32) {
	pfnFramebufferTexture2D(target, attachment, textarget, texture, level)
}

func GenBuffers(n int32, buffers *uint32) { pfnGenBuffers(n, buffers) }

func GenFramebuffers(n int32, framebuffers *uint32) { pfnGenFramebuffers(n, framebuffers) }

func GenRenderbuffers(n int32, renderbuffers *uint32) { pfnGenRenderbuffers(n, renderbuffers) }

func GenTextures(n int32, textures *uint32) { pfnGenTextures(n, textures) }

func GenVertexArrays(n int32, arrays *uint32) { pfnGenVertexArrays(n, arrays) }

func GetProgramInfoLog(program uint32, bufSize int32, length *int32, infoLog *uint8) {
	pfnGetProgramInfoLog(program, bufSize, length, infoLog)
}

func GetProgramiv(program uint32, pname uint32, params *int32) {
	pfnGetProgramiv(program, pname, params)
}

func GetShaderInfoLog(shader uint32, bufSize int32, length *int32, infoLog *uint8) {
	pfnGetShaderInfoLog(shader, bufSize, length, infoLog)
}

func GetShaderiv(shader uint32, pname uint32, params *int32) {
	pfnGetShaderiv(shader, pname, params)
}

func GetUniformLocation(program uint32, name *uint8) int32 {
	return pfnGetUniformLocation(program, name)
}

func LinkProgram(program uint32) { pfnLinkProgram(program) }

func RenderbufferStorage(target uint32, internalformat uint32, width int32, height int32) {
	pfnRenderbufferStorage(target, internalformat, width, height)
}

func Scissor(x int32, y int32, width int32, height int32) { pfnScissor(x, y, width, height) }

func ShaderSource(shader uint32, count int32, xstring **uint8, length *int32) {
	pfnShaderSource(shader, count, xstring, length)
}

func StencilFunc(xfunc uint32, ref int32, mask uint32) { pfnStencilFunc(xfunc, ref, mask) }

func StencilOp(fail uint32, zfail uint32, zpass uint32) { pfnStencilOp(fail, zfail, zpass) }

func TexImage2D(target uint32, level int32, internalformat int32, width int32, height int32, border int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	pfnTexImage2D(target, level, internalformat, width, height, border, format, xtype, pixels)
}

func TexParameteri(target uint32, pname uint32, param int32) {
	pfnTexParameteri(target, pname, param)
}

func TexSubImage2D(target uint32, level int32, xoffset int32, yoffset int32, width int32, height int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	pfnTexSubImage2D(target, level, xoffset, yoffset, width, height, format, xtype, pixels)
}

func Uniform1i(location int32, v0 int32) { pfnUniform1i(location, v0) }

func UniformMatrix4fv(location int32, count int32, transpose bool, value *float32) {
	pfnUniformMatrix4fv(location, count, transpose, value)
}

func UseProgram(program uint32) { pfnUseProgram(program) }

// VertexAttribPointerWithOffset mirrors go-gl's helper of the same name: the
// final argument is a byte offset into the buffer currently bound to
// ARRAY_BUFFER, not a client-memory pointer.
func VertexAttribPointerWithOffset(index uint32, size int32, xtype uint32, normalized bool, stride int32, offset uintptr) {
	pfnVertexAttribPointer(index, size, xtype, normalized, stride, offset)
}

func Viewport(x int32, y int32, width int32, height int32) { pfnViewport(x, y, width, height) }
