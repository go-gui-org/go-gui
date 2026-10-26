// Host harness for gles_android.c. gles_harness_test.go builds it with the stub
// GL header next to it and runs it. It prints one line per failed check and exits
// nonzero when any check fails.
#include <stdio.h>

#include <GLES3/gl3.h>
#include "gles_android.h"

int harnessUniformWrites;
GLint harnessLastUniformLoc;
int harnessDeleteCalls;
GLenum harnessActiveUnit;
GLuint harnessBoundTex;
int harnessUniform1iWrites;
GLint harnessLastUniform1iLoc, harnessLastUniform1iVal;
GLint harnessUnpackAlignment = 4;
int harnessGenTextureCalls;

static int fails;

// reset clears the recorded uniform write before the call under test.
static void reset(void) {
    harnessUniformWrites = 0;
    harnessLastUniformLoc = -999;
}

static void expect(int cond, const char* what) {
    if (cond) return;
    printf("FAIL: %s (writes=%d loc=%d)\n", what, harnessUniformWrites,
           harnessLastUniformLoc);
    fails++;
}

int main(void) {
    float m[16] = {1};
    int a = glesBuildCustomPipeline("frag");
    int b = glesBuildCustomPipeline("frag");
    expect(a >= 0 && b >= 0 && a != b, "custom pipelines build");

    // A bound custom program gets the matrices draw.go sets after binding it.
    // Before the fix both setters returned early and nothing was written.
    glesSetCustomPipeline(b);
    reset();
    glesSetMVP(m);
    expect(harnessUniformWrites == 1 && harnessLastUniformLoc == HARNESS_LOC_MVP,
           "custom glesSetMVP writes mvp");
    reset();
    glesSetTM(m);
    expect(harnessUniformWrites == 1 && harnessLastUniformLoc == HARNESS_LOC_TM,
           "custom glesSetTM writes tm");
    reset();
    glesSetTM2(m);
    expect(harnessUniformWrites == 0, "custom glesSetTM2 writes nothing");
    // The shader built-ins (#573) live only in custom programs.
    reset();
    glesSetBuiltins(m);
    expect(harnessUniformWrites == 1 &&
               harnessLastUniformLoc == HARNESS_LOC_BUILTINS,
           "custom glesSetBuiltins writes _gg_builtins");

    reset();
    glesSetMVP(NULL);
    glesSetTM(NULL);
    glesSetTM2(NULL);
    glesSetBuiltins(NULL);
    expect(harnessUniformWrites == 0, "NULL matrix writes nothing");

    // An out-of-range or unbuilt index keeps the current binding.
    glesSetCustomPipeline(-1);
    glesSetCustomPipeline(32);
    glesSetCustomPipeline(5);
    reset();
    glesSetMVP(m);
    expect(harnessUniformWrites == 1 && harnessLastUniformLoc == HARNESS_LOC_MVP,
           "bad index keeps the custom binding");

    // A frame reset leaves custom mode.
    glesBeginFrame(0, 0, 0, 0);
    reset();
    glesSetMVP(m);
    expect(harnessUniformWrites == 0, "glesSetMVP after glesBeginFrame writes nothing");

    // Deleting the bound custom pipeline unbinds it.
    glesSetCustomPipeline(a);
    glesDeleteCustomPipeline(a);
    reset();
    glesSetMVP(m);
    expect(harnessUniformWrites == 0,
           "glesSetMVP after deleting the bound pipeline writes nothing");

    // The stencil functions bind PIPE_STENCIL themselves. Uniforms are per
    // program, so the mvp the Go caller set on PIPE_SOLID does not reach it; each
    // must upload mvp to the stencil program. Before the fix nothing was written,
    // the stencil program kept a zero matrix and the clip mask was never drawn.
    float verts[36] = {0};
    expect(glesInit() == 0, "glesInit");
    // No built-in program declares the shader built-ins; the setter must
    // skip them rather than index a table they do not have.
    glesSetPipeline(0);
    reset();
    glesSetBuiltins(m);
    expect(harnessUniformWrites == 0, "built-in pipeline glesSetBuiltins writes nothing");
    reset();
    glesBeginStencilClip(verts, 1, m);
    expect(harnessUniformWrites == 1 && harnessLastUniformLoc == HARNESS_LOC_MVP,
           "glesBeginStencilClip writes mvp");
    reset();
    glesEndStencilClip(verts, 1, m);
    expect(harnessUniformWrites == 1 && harnessLastUniformLoc == HARNESS_LOC_MVP,
           "glesEndStencilClip writes mvp");

    // Context loss: the names below belong to a context that is gone.
    // glesAbandon forgets them without deleting one, since in the new context a
    // name can belong to a live object, and the rebuild starts every allocator
    // over.
    int t1 = glesCreateTexture(4, 4, NULL, 0);
    int t2 = glesCreateTexture(4, 4, NULL, 0);
    int c1 = glesBuildCustomPipeline("frag");
    int c2 = glesBuildCustomPipeline("frag");
    expect(t1 >= 1 && t2 > t1 && c1 >= 0 && c2 > c1,
           "textures and custom pipelines before the loss");
    harnessDeleteCalls = 0;
    glesAbandon();
    expect(harnessDeleteCalls == 0, "glesAbandon deletes nothing");
    expect(glesInit() == 0, "glesInit after glesAbandon");
    expect(glesCreateTexture(4, 4, NULL, 0) == 1,
           "texture IDs start over after glesAbandon");
    expect(glesBuildCustomPipeline("frag") == 0 &&
               glesBuildCustomPipeline("frag") == 1,
           "custom pipeline indices start over after glesAbandon");
    expect(harnessDeleteCalls == 0, "the rebuild deletes nothing");
    // The counter does see a delete, so the zeros above mean something.
    glesDeleteTexture(1);
    expect(harnessDeleteCalls == 1, "glesDeleteTexture is counted");

    // Shader channels (gui.Shader.Surface).
    // A channel program's samplers point at units 1..4, one write each.
    int ch = glesBuildCustomPipeline("frag");
    harnessUniform1iWrites = 0;
    glesSetChannelUnits(ch);
    expect(harnessUniform1iWrites == 4 &&
               harnessLastUniform1iLoc == HARNESS_LOC_CHANNEL0 + 3 &&
               harnessLastUniform1iVal == 4,
           "glesSetChannelUnits points iChannel0..3 at units 1..4");
    harnessUniform1iWrites = 0;
    glesSetChannelUnits(-1);
    glesSetChannelUnits(31);
    expect(harnessUniform1iWrites == 0, "glesSetChannelUnits ignores a bad index");
    // It binds the program to write the units, so draw.go must bind the
    // pipeline again afterwards; setting the matrices before that is a no-op.
    glesSetCustomPipeline(ch);
    reset();
    glesSetMVP(m);
    expect(harnessUniformWrites == 1 && harnessLastUniformLoc == HARNESS_LOC_MVP,
           "custom glesSetMVP after glesSetChannelUnits writes mvp");

    // Plane textures upload with an unpack alignment of 1 and restore 4.
    unsigned char plane[3] = {1, 2, 3};
    int pt = glesCreatePlaneTexture(1, 3, 1, plane);
    expect(pt >= 1, "glesCreatePlaneTexture returns an ID");
    expect(harnessUnpackAlignment == 4, "glesCreatePlaneTexture restores the unpack alignment");
    glesUpdatePlaneTexture(pt, 1, 3, 1, plane);
    expect(harnessUnpackAlignment == 4, "glesUpdatePlaneTexture restores the unpack alignment");

    // A channel binds its plane, or the transparent texture for ID 0, and
    // leaves unit 0 active for the draws that follow.
    int gens = harnessGenTextureCalls;
    harnessBoundTex = 0;
    glesBindChannel(1, 0);
    expect(harnessBoundTex != 0, "glesBindChannel(0) binds the transparent texture");
    expect(harnessActiveUnit == GL_TEXTURE0, "glesBindChannel leaves unit 0 active");
    glesBindChannel(2, 0);
    glesBindChannel(0, pt);
    expect(harnessGenTextureCalls == gens + 1,
           "the transparent texture is created once");
    harnessActiveUnit = 0;
    glesBindChannel(4, pt);
    glesBindChannel(-1, pt);
    expect(harnessActiveUnit == 0, "glesBindChannel ignores a channel out of range");

    // Context loss forgets the transparent texture without deleting it, and
    // the next bind creates a new one in the new context.
    harnessDeleteCalls = 0;
    glesAbandon();
    expect(harnessDeleteCalls == 0, "glesAbandon deletes no channel texture");
    gens = harnessGenTextureCalls;
    glesBindChannel(0, 0);
    expect(harnessGenTextureCalls == gens + 1,
           "the transparent texture is created again after glesAbandon");

    return fails != 0;
}
