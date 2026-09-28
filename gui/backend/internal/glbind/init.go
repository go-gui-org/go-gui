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

// rawBinding pairs a package-level address variable with the GL entry-point
// name that fills it. Its entry points are called through sysN rather than a
// purego wrapper; raw.go explains why and what may join it.
type rawBinding struct {
	addr *uintptr
	name string
}

// bindings lists the GL entry points bound through purego. Order is
// irrelevant; all of them are OpenGL 3.3 core, so a driver advertising a 3.3
// context must resolve all of them.
//
// Together with rawBindings it covers every entry point this package exposes.
// TestTablesPartitionEntryPoints holds the two tables to that contract.
func bindings() []binding {
	return []binding{
		{&pfnAttachShader, "glAttachShader"},
		{&pfnBindRenderbuffer, "glBindRenderbuffer"},
		{&pfnBlendFunc, "glBlendFunc"},
		{&pfnCheckFramebufferStatus, "glCheckFramebufferStatus"},
		{&pfnClearColor, "glClearColor"},
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
		{&pfnFramebufferRenderbuffer, "glFramebufferRenderbuffer"},
		{&pfnFramebufferTexture2D, "glFramebufferTexture2D"},
		{&pfnGenBuffers, "glGenBuffers"},
		{&pfnGenFramebuffers, "glGenFramebuffers"},
		{&pfnGenRenderbuffers, "glGenRenderbuffers"},
		{&pfnGenTextures, "glGenTextures"},
		{&pfnGenVertexArrays, "glGenVertexArrays"},
		{&pfnGetError, "glGetError"},
		{&pfnGetIntegerv, "glGetIntegerv"},
		{&pfnGetProgramInfoLog, "glGetProgramInfoLog"},
		{&pfnGetProgramiv, "glGetProgramiv"},
		{&pfnGetShaderInfoLog, "glGetShaderInfoLog"},
		{&pfnGetShaderiv, "glGetShaderiv"},
		{&pfnGetUniformLocation, "glGetUniformLocation"},
		{&pfnLinkProgram, "glLinkProgram"},
		{&pfnReadPixels, "glReadPixels"},
		{&pfnRenderbufferStorage, "glRenderbufferStorage"},
		{&pfnRenderbufferStorageMultisample, "glRenderbufferStorageMultisample"},
		{&pfnShaderSource, "glShaderSource"},
		{&pfnTexImage2D, "glTexImage2D"},
		{&pfnTexParameteri, "glTexParameteri"},
	}
}

// rawBindings lists the per-frame GL entry points, resolved to a bare address
// and called through sysN. Membership is a performance decision with two hard
// constraints — no float parameters, and no pointer parameters routed through
// sysN — both spelled out in raw.go. Read that before adding a row.
func rawBindings() []rawBinding {
	return []rawBinding{
		{&addrActiveTexture, "glActiveTexture"},
		{&addrBindBuffer, "glBindBuffer"},
		{&addrBindFramebuffer, "glBindFramebuffer"},
		{&addrBindTexture, "glBindTexture"},
		{&addrBindVertexArray, "glBindVertexArray"},
		{&addrBlitFramebuffer, "glBlitFramebuffer"},
		{&addrBufferData, "glBufferData"},
		{&addrBufferSubData, "glBufferSubData"},
		{&addrClear, "glClear"},
		{&addrColorMask, "glColorMask"},
		{&addrDisable, "glDisable"},
		{&addrDrawArrays, "glDrawArrays"},
		{&addrDrawElements, "glDrawElements"},
		{&addrEnable, "glEnable"},
		{&addrEnableVertexAttribArray, "glEnableVertexAttribArray"},
		{&addrScissor, "glScissor"},
		{&addrStencilFunc, "glStencilFunc"},
		{&addrStencilOp, "glStencilOp"},
		{&addrTexSubImage2D, "glTexSubImage2D"},
		{&addrUniform1i, "glUniform1i"},
		{&addrUniformMatrix4fv, "glUniformMatrix4fv"},
		{&addrUseProgram, "glUseProgram"},
		{&addrVertexAttribPointer, "glVertexAttribPointer"},
		{&addrViewport, "glViewport"},
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
// Registration is two-pass across both tables: every entry point is resolved
// before any is bound, so a failure leaves the package fully unbound rather
// than half-bound. Callers must treat any error as "do not proceed".
func InitWithProcAddrFunc(getProcAddr func(name string) uintptr) error {
	table := bindings()
	rawTable := rawBindings()

	procs := make([]uintptr, len(table))
	for i, b := range table {
		proc := getProcAddr(b.name)
		if proc == 0 {
			return fmt.Errorf("glbind: %s unavailable", b.name)
		}
		procs[i] = proc
	}
	rawProcs := make([]uintptr, len(rawTable))
	for i, b := range rawTable {
		proc := getProcAddr(b.name)
		if proc == 0 {
			return fmt.Errorf("glbind: %s unavailable", b.name)
		}
		rawProcs[i] = proc
	}

	for i, b := range table {
		purego.RegisterFunc(b.fptr, procs[i])
	}
	for i, b := range rawTable {
		*b.addr = rawProcs[i]
	}
	return nil
}
