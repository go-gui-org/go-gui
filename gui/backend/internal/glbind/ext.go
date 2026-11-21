//go:build !js && !darwin

package glbind

// Extension entry points sit outside the two tables InitWithProcAddrFunc
// fills: those hold OpenGL 3.3 core, which every driver go-gui runs on must
// resolve, while an extension may be missing. They are bound by their own
// Init function, after the backend has checked that the context lists the
// extension, and are called on the raw path like the hot core entry points.
var extEGLImageTargetTexture2DOES uintptr

// InitEGLImage binds glEGLImageTargetTexture2DOES (GL_OES_EGL_image) and
// reports whether it resolved. Call it with a current context, and only
// once the context lists the extension: eglGetProcAddress may return an
// address for a name the driver does not support.
func InitEGLImage(getProcAddr func(name string) uintptr) bool {
	extEGLImageTargetTexture2DOES = getProcAddr("glEGLImageTargetTexture2DOES")
	return extEGLImageTargetTexture2DOES != 0
}

// EGLImageTargetTexture2DOES gives the texture bound to target the storage
// of image, an EGLImage: a driver handle, so uintptr. The DMA-BUF import
// (#1326) calls it once per plane of each new frame.
func EGLImageTargetTexture2DOES(target uint32, image uintptr) {
	sysN(extEGLImageTargetTexture2DOES, uintptr(target), image)
}
