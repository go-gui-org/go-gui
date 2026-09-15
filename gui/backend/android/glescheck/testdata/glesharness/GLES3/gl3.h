// Host stand-in for <GLES3/gl3.h>, used only by gles_harness_test.go.
//
// It lets gles_android.c compile and run on a desktop host with no GL context.
// Every GL call it uses is a no-op, except the few the harness observes:
// glUniformMatrix4fv records the location it wrote to, and the shader and program
// queries report success so glesBuildCustomPipeline returns a pipeline.
//
// Add a prototype here when gles_android.c starts to call a new GL function.
#ifndef GLES_HARNESS_GL3_H
#define GLES_HARNESS_GL3_H

#include <stddef.h>
#include <string.h>

typedef unsigned int GLenum;
typedef unsigned int GLuint;
typedef int GLint;
typedef int GLsizei;
typedef unsigned char GLboolean;
typedef unsigned int GLbitfield;
typedef float GLfloat;
typedef char GLchar;
typedef ptrdiff_t GLsizeiptr;

// The values only need to be distinct; nothing reaches a real driver.
#define GL_FALSE 0
#define GL_TRUE 1
#define GL_ALWAYS 0x0207
#define GL_LEQUAL 0x0203
#define GL_KEEP 0x1E00
#define GL_INCR 0x1E02
#define GL_DECR 0x1E03
#define GL_TRIANGLES 0x0004
#define GL_TRIANGLE_FAN 0x0006
#define GL_BLEND 0x0BE2
#define GL_DEPTH_TEST 0x0B71
#define GL_SCISSOR_TEST 0x0C11
#define GL_STENCIL_TEST 0x0B90
#define GL_SRC_ALPHA 0x0302
#define GL_ONE_MINUS_SRC_ALPHA 0x0303
#define GL_COLOR_BUFFER_BIT 0x4000
#define GL_STENCIL_BUFFER_BIT 0x0400
#define GL_ARRAY_BUFFER 0x8892
#define GL_ELEMENT_ARRAY_BUFFER 0x8893
#define GL_STATIC_DRAW 0x88E4
#define GL_DYNAMIC_DRAW 0x88E8
#define GL_FLOAT 0x1406
#define GL_UNSIGNED_BYTE 0x1401
#define GL_UNSIGNED_SHORT 0x1403
#define GL_VERTEX_SHADER 0x8B31
#define GL_FRAGMENT_SHADER 0x8B30
#define GL_COMPILE_STATUS 0x8B81
#define GL_LINK_STATUS 0x8B82
#define GL_FRAMEBUFFER 0x8D40
#define GL_RENDERBUFFER 0x8D41
#define GL_COLOR_ATTACHMENT0 0x8CE0
#define GL_STENCIL_ATTACHMENT 0x8D20
#define GL_STENCIL_INDEX8 0x8D48
#define GL_TEXTURE0 0x84C0
#define GL_TEXTURE_2D 0x0DE1
#define GL_TEXTURE_MAG_FILTER 0x2800
#define GL_TEXTURE_MIN_FILTER 0x2801
#define GL_TEXTURE_WRAP_S 0x2802
#define GL_TEXTURE_WRAP_T 0x2803
#define GL_CLAMP_TO_EDGE 0x812F
#define GL_LINEAR 0x2601
#define GL_RGBA 0x1908
#define GL_RGBA8 0x8058

// Observed state, defined in harness.c.
extern int harnessUniformWrites;
extern GLint harnessLastUniformLoc;

// Uniform locations the harness hands out, so a test can tell which uniform a
// setter wrote to.
#define HARNESS_LOC_MVP 7
#define HARNESS_LOC_TM 9

static inline void glUniformMatrix4fv(GLint loc, GLsizei n, GLboolean t,
                                      const GLfloat* m) {
    (void)n; (void)t; (void)m;
    harnessUniformWrites++;
    harnessLastUniformLoc = loc;
}

static inline GLint glGetUniformLocation(GLuint p, const GLchar* name) {
    (void)p;
    if (strcmp(name, "mvp") == 0) return HARNESS_LOC_MVP;
    if (strcmp(name, "tm") == 0) return HARNESS_LOC_TM;
    return -1;
}

static inline GLuint glCreateProgram(void) { return 1; }
static inline GLuint glCreateShader(GLenum t) { (void)t; return 1; }
static inline void glGetShaderiv(GLuint s, GLenum p, GLint* v) { (void)s; (void)p; *v = 1; }
static inline void glGetProgramiv(GLuint s, GLenum p, GLint* v) { (void)s; (void)p; *v = 1; }

// Generators hand out a nonzero name so "0 = none" checks pass.
static inline void harnessGen(GLsizei n, GLuint* out) {
    for (GLsizei i = 0; i < n; i++) out[i] = 1;
}
static inline void glGenBuffers(GLsizei n, GLuint* o) { harnessGen(n, o); }
static inline void glGenFramebuffers(GLsizei n, GLuint* o) { harnessGen(n, o); }
static inline void glGenRenderbuffers(GLsizei n, GLuint* o) { harnessGen(n, o); }
static inline void glGenTextures(GLsizei n, GLuint* o) { harnessGen(n, o); }
static inline void glGenVertexArrays(GLsizei n, GLuint* o) { harnessGen(n, o); }

// Everything else does nothing.
static inline void glActiveTexture(GLenum a) { (void)a; }
static inline void glAttachShader(GLuint a, GLuint b) { (void)a; (void)b; }
static inline void glBindBuffer(GLenum a, GLuint b) { (void)a; (void)b; }
static inline void glBindFramebuffer(GLenum a, GLuint b) { (void)a; (void)b; }
static inline void glBindRenderbuffer(GLenum a, GLuint b) { (void)a; (void)b; }
static inline void glBindTexture(GLenum a, GLuint b) { (void)a; (void)b; }
static inline void glBindVertexArray(GLuint a) { (void)a; }
static inline void glBlendFunc(GLenum a, GLenum b) { (void)a; (void)b; }
static inline void glBufferData(GLenum a, GLsizeiptr b, const void* c, GLenum d) { (void)a; (void)b; (void)c; (void)d; }
static inline void glClear(GLbitfield a) { (void)a; }
static inline void glClearColor(GLfloat a, GLfloat b, GLfloat c, GLfloat d) { (void)a; (void)b; (void)c; (void)d; }
static inline void glColorMask(GLboolean a, GLboolean b, GLboolean c, GLboolean d) { (void)a; (void)b; (void)c; (void)d; }
static inline void glCompileShader(GLuint a) { (void)a; }
static inline void glDeleteBuffers(GLsizei a, const GLuint* b) { (void)a; (void)b; }
static inline void glDeleteFramebuffers(GLsizei a, const GLuint* b) { (void)a; (void)b; }
static inline void glDeleteProgram(GLuint a) { (void)a; }
static inline void glDeleteRenderbuffers(GLsizei a, const GLuint* b) { (void)a; (void)b; }
static inline void glDeleteShader(GLuint a) { (void)a; }
static inline void glDeleteTextures(GLsizei a, const GLuint* b) { (void)a; (void)b; }
static inline void glDeleteVertexArrays(GLsizei a, const GLuint* b) { (void)a; (void)b; }
static inline void glDisable(GLenum a) { (void)a; }
static inline void glDrawArrays(GLenum a, GLint b, GLsizei c) { (void)a; (void)b; (void)c; }
static inline void glDrawElements(GLenum a, GLsizei b, GLenum c, const void* d) { (void)a; (void)b; (void)c; (void)d; }
static inline void glEnable(GLenum a) { (void)a; }
static inline void glEnableVertexAttribArray(GLuint a) { (void)a; }
static inline void glFlush(void) {}
static inline void glFramebufferRenderbuffer(GLenum a, GLenum b, GLenum c, GLuint d) { (void)a; (void)b; (void)c; (void)d; }
static inline void glFramebufferTexture2D(GLenum a, GLenum b, GLenum c, GLuint d, GLint e) { (void)a; (void)b; (void)c; (void)d; (void)e; }
static inline void glGetProgramInfoLog(GLuint a, GLsizei b, GLsizei* c, GLchar* d) { (void)a; (void)b; (void)c; if (b > 0) d[0] = 0; }
static inline void glGetShaderInfoLog(GLuint a, GLsizei b, GLsizei* c, GLchar* d) { (void)a; (void)b; (void)c; if (b > 0) d[0] = 0; }
static inline void glLinkProgram(GLuint a) { (void)a; }
static inline void glRenderbufferStorage(GLenum a, GLenum b, GLsizei c, GLsizei d) { (void)a; (void)b; (void)c; (void)d; }
static inline void glScissor(GLint a, GLint b, GLsizei c, GLsizei d) { (void)a; (void)b; (void)c; (void)d; }
static inline void glShaderSource(GLuint a, GLsizei b, const GLchar* const* c, const GLint* d) { (void)a; (void)b; (void)c; (void)d; }
static inline void glStencilFunc(GLenum a, GLint b, GLuint c) { (void)a; (void)b; (void)c; }
static inline void glStencilOp(GLenum a, GLenum b, GLenum c) { (void)a; (void)b; (void)c; }
static inline void glTexImage2D(GLenum a, GLint b, GLint c, GLsizei d, GLsizei e, GLint f, GLenum g, GLenum h, const void* i) { (void)a; (void)b; (void)c; (void)d; (void)e; (void)f; (void)g; (void)h; (void)i; }
static inline void glTexParameteri(GLenum a, GLenum b, GLint c) { (void)a; (void)b; (void)c; }
static inline void glTexSubImage2D(GLenum a, GLint b, GLint c, GLint d, GLsizei e, GLsizei f, GLenum g, GLenum h, const void* i) { (void)a; (void)b; (void)c; (void)d; (void)e; (void)f; (void)g; (void)h; (void)i; }
static inline void glUniform1i(GLint a, GLint b) { (void)a; (void)b; }
static inline void glUseProgram(GLuint a) { (void)a; }
static inline void glVertexAttribPointer(GLuint a, GLint b, GLenum c, GLboolean d, GLsizei e, const void* f) { (void)a; (void)b; (void)c; (void)d; (void)e; (void)f; }
static inline void glViewport(GLint a, GLint b, GLsizei c, GLsizei d) { (void)a; (void)b; (void)c; (void)d; }

#endif
