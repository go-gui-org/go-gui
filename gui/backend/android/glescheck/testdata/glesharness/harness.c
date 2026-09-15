// Host harness for gles_android.c. gles_harness_test.go builds it with the stub
// GL header next to it and runs it. It prints one line per failed check and exits
// nonzero when any check fails.
#include <stdio.h>

#include <GLES3/gl3.h>
#include "gles_android.h"

int harnessUniformWrites;
GLint harnessLastUniformLoc;

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

    reset();
    glesSetMVP(NULL);
    glesSetTM(NULL);
    glesSetTM2(NULL);
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
    reset();
    glesBeginStencilClip(verts, 1, m);
    expect(harnessUniformWrites == 1 && harnessLastUniformLoc == HARNESS_LOC_MVP,
           "glesBeginStencilClip writes mvp");
    reset();
    glesEndStencilClip(verts, 1, m);
    expect(harnessUniformWrites == 1 && harnessLastUniformLoc == HARNESS_LOC_MVP,
           "glesEndStencilClip writes mvp");

    return fails != 0;
}
