//go:build !js && !darwin

package glbind

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// binding pairs a package-level function pointer with the GL entry-point name
// that fills it.
type binding struct {
	fptr any
	name string
}

// bindings lists every GL entry point this package exposes. Order is
// irrelevant; all of them are OpenGL 3.3 core, so a driver advertising a 3.3
// context must resolve all of them.
func bindings() []binding {
	return []binding{
		{&pfnActiveTexture, "glActiveTexture"},
		{&pfnAttachShader, "glAttachShader"},
		{&pfnBindBuffer, "glBindBuffer"},
		{&pfnBindFramebuffer, "glBindFramebuffer"},
		{&pfnBindRenderbuffer, "glBindRenderbuffer"},
		{&pfnBindTexture, "glBindTexture"},
		{&pfnBindVertexArray, "glBindVertexArray"},
		{&pfnBlendFunc, "glBlendFunc"},
		{&pfnBufferData, "glBufferData"},
		{&pfnBufferSubData, "glBufferSubData"},
		{&pfnCheckFramebufferStatus, "glCheckFramebufferStatus"},
		{&pfnClear, "glClear"},
		{&pfnClearColor, "glClearColor"},
		{&pfnColorMask, "glColorMask"},
		{&pfnCompileShader, "glCompileShader"},
		{&pfnCreateProgram, "glCreateProgram"},
		{&pfnCreateShader, "glCreateShader"},
		{&pfnDeleteBuffers, "glDeleteBuffers"},
		{&pfnDeleteFramebuffers, "glDeleteFramebuffers"},
		{&pfnDeleteProgram, "glDeleteProgram"},
		{&pfnDeleteRenderbuffers, "glDeleteRenderbuffers"},
		{&pfnDeleteShader, "glDeleteShader"},
		{&pfnDeleteTextures, "glDeleteTextures"},
		{&pfnDeleteVertexArrays, "glDeleteVertexArrays"},
		{&pfnDisable, "glDisable"},
		{&pfnDrawArrays, "glDrawArrays"},
		{&pfnDrawElements, "glDrawElements"},
		{&pfnEnable, "glEnable"},
		{&pfnEnableVertexAttribArray, "glEnableVertexAttribArray"},
		{&pfnFramebufferRenderbuffer, "glFramebufferRenderbuffer"},
		{&pfnFramebufferTexture2D, "glFramebufferTexture2D"},
		{&pfnGenBuffers, "glGenBuffers"},
		{&pfnGenFramebuffers, "glGenFramebuffers"},
		{&pfnGenRenderbuffers, "glGenRenderbuffers"},
		{&pfnGenTextures, "glGenTextures"},
		{&pfnGenVertexArrays, "glGenVertexArrays"},
		{&pfnGetProgramInfoLog, "glGetProgramInfoLog"},
		{&pfnGetProgramiv, "glGetProgramiv"},
		{&pfnGetShaderInfoLog, "glGetShaderInfoLog"},
		{&pfnGetShaderiv, "glGetShaderiv"},
		{&pfnGetUniformLocation, "glGetUniformLocation"},
		{&pfnLinkProgram, "glLinkProgram"},
		{&pfnRenderbufferStorage, "glRenderbufferStorage"},
		{&pfnScissor, "glScissor"},
		{&pfnShaderSource, "glShaderSource"},
		{&pfnStencilFunc, "glStencilFunc"},
		{&pfnStencilOp, "glStencilOp"},
		{&pfnTexImage2D, "glTexImage2D"},
		{&pfnTexParameteri, "glTexParameteri"},
		{&pfnTexSubImage2D, "glTexSubImage2D"},
		{&pfnUniform1i, "glUniform1i"},
		{&pfnUniformMatrix4fv, "glUniformMatrix4fv"},
		{&pfnUseProgram, "glUseProgram"},
		{&pfnVertexAttribPointer, "glVertexAttribPointer"},
		{&pfnViewport, "glViewport"},
	}
}

// InitWithProcAddrFunc resolves and binds every GL entry point this package
// exposes, using the caller's platform proc-address function. It must be
// called with a current GL context, before any other function in this package.
//
// getProcAddr takes the C entry-point name and returns the function address,
// or 0 if the driver does not export it. Addresses are uintptr rather than
// unsafe.Pointer because they point at driver code, not Go memory — the
// unsafe.Pointer spelling would be a genuine go vet unsafeptr violation on the
// Windows side, which obtains addresses from LazyProc.
//
// A missing symbol is reported as an error naming it, rather than deferred to
// a nil-pointer panic at the first draw call.
//
// Registration is two-pass: every entry point is resolved before any is
// bound, so a failure leaves the package fully unbound rather than
// half-bound. Callers must treat any error as "do not proceed".
func InitWithProcAddrFunc(getProcAddr func(name string) uintptr) error {
	table := bindings()
	procs := make([]uintptr, len(table))
	for i, b := range table {
		proc := getProcAddr(b.name)
		if proc == 0 {
			return fmt.Errorf("glbind: %s unavailable", b.name)
		}
		procs[i] = proc
	}
	for i, b := range table {
		purego.RegisterFunc(b.fptr, procs[i])
	}
	return nil
}
