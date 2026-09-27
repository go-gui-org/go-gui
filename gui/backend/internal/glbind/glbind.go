//go:build !js && !darwin

// Package glbind is a CGo-free binding for the subset of OpenGL 3.3 core that
// gui/backend/gl uses.
//
// It replaces github.com/go-gl/gl, which was the sole CGo dependency of the
// Linux and Windows backends. Entry points are resolved through the caller's
// platform proc-address function (eglGetProcAddress on Linux,
// wglGetProcAddress + opengl32.dll on Windows).
//
// The binding has two halves, because the call mechanism that reads best is
// not the one a frame can afford:
//
//   - Cold entry points — shader compilation, object creation, queries, and
//     glClearColor, the one entry point with float parameters — are bound with
//     github.com/ebitengine/purego, the same mechanism gui/backend/gl uses for
//     EGL itself. They live in this file and call through the pfn vars below.
//   - Per-frame entry points are called through a raw address instead, because
//     purego's reflect-based wrapper allocates on every call. They live in
//     raw.go, raw_windows.go and raw_other.go, which explain why.
//
// Signatures deliberately mirror go-gl's exactly in both halves, including
// SCREAMING_SNAKE enum names and the `bool` spelling of GLboolean parameters,
// so that call sites need nothing beyond an import swap. purego marshals Go
// `bool` as C `_Bool` (one byte), which matches GLboolean; the raw half spells
// the same marshalling out in b2u.
//
// Call InitWithProcAddrFunc before any other function in this package; every
// wrapper dereferences a function pointer, or an address, that is zero until
// then.
package glbind

import "unsafe"

// C-level entry points, populated by InitWithProcAddrFunc. Parameter types are
// the purego equivalents of the GL types: GLenum/GLuint -> uint32, GLint/GLsizei
// -> int32, GLboolean -> bool, GLsizeiptr/GLintptr -> int, pointers ->
// unsafe.Pointer or a typed Go pointer (purego passes both as void*).
var (
	pfnAttachShader            func(program, shader uint32)
	pfnBindFramebuffer         func(target, framebuffer uint32)
	pfnBindRenderbuffer        func(target, renderbuffer uint32)
	pfnBlendFunc               func(sfactor, dfactor uint32)
	pfnCheckFramebufferStatus  func(target uint32) uint32
	pfnClearColor              func(red, green, blue, alpha float32)
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
	pfnShaderSource            func(shader uint32, count int32, xstring **uint8, length *int32)
	pfnTexImage2D              func(target uint32, level, internalformat, width, height, border int32, format, xtype uint32, pixels unsafe.Pointer)
	pfnTexParameteri           func(target, pname uint32, param int32)
)

func AttachShader(program uint32, shader uint32) { pfnAttachShader(program, shader) }

func BindFramebuffer(target uint32, framebuffer uint32) { pfnBindFramebuffer(target, framebuffer) }

func BindRenderbuffer(target uint32, renderbuffer uint32) {
	pfnBindRenderbuffer(target, renderbuffer)
}

func BlendFunc(sfactor uint32, dfactor uint32) { pfnBlendFunc(sfactor, dfactor) }

func CheckFramebufferStatus(target uint32) uint32 { return pfnCheckFramebufferStatus(target) }

func ClearColor(red float32, green float32, blue float32, alpha float32) {
	pfnClearColor(red, green, blue, alpha)
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

func ShaderSource(shader uint32, count int32, xstring **uint8, length *int32) {
	pfnShaderSource(shader, count, xstring, length)
}

func TexImage2D(target uint32, level int32, internalformat int32, width int32, height int32, border int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	pfnTexImage2D(target, level, internalformat, width, height, border, format, xtype, pixels)
}

func TexParameteri(target uint32, pname uint32, param int32) {
	pfnTexParameteri(target, pname, param)
}
