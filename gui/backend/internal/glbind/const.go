//go:build !js && !darwin

package glbind

// OpenGL enum values. Only the constants referenced by gui/backend/gl are
// defined here — this is deliberately not a full GL header. Values are the
// canonical ones from the Khronos registry (gl.xml).
const (
	ALWAYS               = 0x0207
	ARRAY_BUFFER         = 0x8892
	BLEND                = 0x0BE2
	CLAMP_TO_EDGE        = 0x812F
	COLOR_ATTACHMENT0    = 0x8CE0
	COLOR_BUFFER_BIT     = 0x00004000
	COMPILE_STATUS       = 0x8B81
	CULL_FACE            = 0x0B44
	DECR                 = 0x1E03
	DEPTH_TEST           = 0x0B71
	DYNAMIC_DRAW         = 0x88E8
	ELEMENT_ARRAY_BUFFER = 0x8893
	FALSE                = 0
	FLOAT                = 0x1406
	FRAGMENT_SHADER      = 0x8B30
	FRAMEBUFFER          = 0x8D40
	FRAMEBUFFER_COMPLETE = 0x8CD5
	INCR                 = 0x1E02
	INFO_LOG_LENGTH      = 0x8B84
	KEEP                 = 0x1E00
	LEQUAL               = 0x0203
	LINEAR               = 0x2601
	LINK_STATUS          = 0x8B82
	ONE_MINUS_SRC_ALPHA  = 0x0303
	RENDERBUFFER         = 0x8D41
	RGBA                 = 0x1908
	RGBA8                = 0x8058
	SCISSOR_TEST         = 0x0C11
	SRC_ALPHA            = 0x0302
	STATIC_DRAW          = 0x88E4
	STENCIL_ATTACHMENT   = 0x8D20
	STENCIL_BUFFER_BIT   = 0x00000400
	STENCIL_INDEX8       = 0x8D48
	STENCIL_TEST         = 0x0B90
	TEXTURE0             = 0x84C0
	TEXTURE_2D           = 0x0DE1
	TEXTURE_MAG_FILTER   = 0x2800
	TEXTURE_MIN_FILTER   = 0x2801
	TEXTURE_WRAP_S       = 0x2802
	TEXTURE_WRAP_T       = 0x2803
	TRIANGLES            = 0x0004
	TRIANGLE_FAN         = 0x0006
	UNSIGNED_BYTE        = 0x1401
	UNSIGNED_SHORT       = 0x1403
	VERTEX_SHADER        = 0x8B31
)
