// gogui.js — JS helpers for the go-gui web backend.
//
// Load it with a script tag before the page starts Go (go.run):
//
//   <script src="gogui.js"></script>
//
// It is optional, but a page with a strict Content-Security-Policy
// (no 'unsafe-eval') needs it for fast triangle drawing. Without it
// the backend compiles the same code with new Function, and if the
// CSP blocks that too, it draws one JS call per vertex, which is
// several times slower for SVG, charts and ThinkingOrb.
//
// The backend also embeds this file (draw_tris.go), so the page copy
// and the fallback never differ.
"use strict";

// goGuiFillTris fills the triangles in xy[s, e) as one path in the
// current fill style. xy holds x,y pairs, six floats per triangle.
globalThis.goGuiFillTris = function (ctx, xy, s, e) {
  ctx.beginPath();
  for (let i = s; i < e; i += 6) {
    ctx.moveTo(xy[i], xy[i + 1]);
    ctx.lineTo(xy[i + 2], xy[i + 3]);
    ctx.lineTo(xy[i + 4], xy[i + 5]);
    ctx.closePath();
  }
  ctx.fill();
};

// goGuiBuildTrisPath returns a Path2D of the triangles in xy[s, e), so
// a mesh that does not change can be filled again without rebuilding
// its path (the Path2D cache, draw_path2d.go).
globalThis.goGuiBuildTrisPath = function (xy, s, e) {
  const p = new Path2D();
  for (let i = s; i < e; i += 6) {
    p.moveTo(xy[i], xy[i + 1]);
    p.lineTo(xy[i + 2], xy[i + 3]);
    p.lineTo(xy[i + 4], xy[i + 5]);
    p.closePath();
  }
  return p;
};

// goGuiSvgGL draws SVG meshes with WebGL2 into an offscreen canvas
// that the backend composites onto the 2D canvas with drawImage
// (draw_webgl.go, #1045). Meshes are uploaded once and
// drawn by id; a frame's draws arrive as one record array, so Go makes
// one call per batch instead of several per mesh.
//
// Record layout, recStride floats each:
//   0 mesh id
//   1-6 affine a, b, c, d, e, f (x' = a·x + c·y + e, y' = b·x + d·y + f)
//   7-10 color r, g, b, a in 0..1 (meshes without vertex colors)
//   11 alpha scale applied to vertex colors
//   12 1 = use vertex colors
//   13-16 scissor x, y, w, h in device pixels, top-left origin;
//         w < 0 = none (the batch rect)
globalThis.goGuiSvgGL = (function () {
  const recStride = 17;
  const vs = `#version 300 es
layout(location = 0) in vec2 aPos;
layout(location = 1) in vec4 aCol;
uniform vec3 uRow0;
uniform vec3 uRow1;
uniform vec2 uView;
uniform vec2 uOff;
uniform vec4 uColor;
uniform float uAlpha;
uniform float uUseV;
out vec4 vCol;
void main() {
  vec3 p = vec3(aPos, 1.0);
  vec2 q = vec2(dot(uRow0, p), dot(uRow1, p));
  gl_Position = vec4(q.x * uView.x + uOff.x - 1.0, 1.0 - q.y * uView.y - uOff.y, 0.0, 1.0);
  vec4 v = vec4(aCol.rgb, aCol.a * uAlpha);
  vCol = mix(uColor, v, uUseV);
}`;
  const fs = `#version 300 es
precision mediump float;
in vec4 vCol;
out vec4 o;
void main() { o = vec4(vCol.rgb * vCol.a, vCol.a); }`;

  function compile(gl, type, src) {
    const sh = gl.createShader(type);
    gl.shaderSource(sh, src);
    gl.compileShader(sh);
    if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) {
      throw new Error("goGuiSvgGL: " + gl.getShaderInfoLog(sh));
    }
    return sh;
  }

  // setup builds the program and its uniform locations into s.
  function setup(s) {
    const gl = s.gl;
    const prog = gl.createProgram();
    gl.attachShader(prog, compile(gl, gl.VERTEX_SHADER, vs));
    gl.attachShader(prog, compile(gl, gl.FRAGMENT_SHADER, fs));
    gl.linkProgram(prog);
    if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
      throw new Error("goGuiSvgGL: " + gl.getProgramInfoLog(prog));
    }
    const u = (n) => gl.getUniformLocation(prog, n);
    s.prog = prog;
    s.uRow0 = u("uRow0");
    s.uRow1 = u("uRow1");
    s.uView = u("uView");
    s.uOff = u("uOff");
    s.uColor = u("uColor");
    s.uAlpha = u("uAlpha");
    s.uUseV = u("uUseV");
  }

  // stats accumulates main-thread time spent in draw and composite,
  // for the bench page to read and reset each frame.
  const stats = { drawMs: 0, compositeMs: 0, records: 0 };

  // states lists the renderers init created, so a test page can reach
  // their contexts (the bench's lose=1 drives WEBGL_lose_context).
  const states = [];

  const noop = () => {};

  return {
    recStride,
    stats,
    states,

    // nullContext stands in for the 2D context while the backend's
    // pre-pass replays a frame's clips and transforms to batch its
    // SVGs: every method is a no-op and every write is dropped.
    nullContext: new Proxy({}, { get: () => noop, set: () => true }),

    // stage copies the device-pixel rect (x, y, w, h) of the WebGL
    // canvas into the 2D staging canvas, once per frame after every
    // batch is rendered. Firefox pays for each drawImage from a WebGL
    // canvas, changed or not; drawImage from a 2D canvas is cheap, so
    // batches are composited from the copy (#1045).
    stage(s, x, y, w, h) {
      const t0 = performance.now();
      s.sctx.clearRect(x, y, w, h);
      s.sctx.drawImage(s.canvas, x, y, w, h, x, y, w, h);
      stats.compositeMs += performance.now() - t0;
    },

    // composite draws the staging canvas's device-pixel rect (ax, ay,
    // w, h) onto ctx at device position (x, y), in logical pixels;
    // ctx's own transform, clip and filter apply.
    composite(s, ctx, dpi, ax, ay, w, h, x, y) {
      const t0 = performance.now();
      ctx.drawImage(s.stage, ax, ay, w, h, x / dpi, y / dpi, w / dpi, h / dpi);
      stats.compositeMs += performance.now() - t0;
    },

    // compositeDirect is composite straight from the WebGL canvas's
    // rect (sx, sy, w, h), for a batch the frame's single render pass
    // did not hold.
    compositeDirect(s, ctx, dpi, sx, sy, w, h, x, y) {
      const t0 = performance.now();
      ctx.drawImage(s.canvas, sx, sy, w, h, x / dpi, y / dpi, w / dpi, h / dpi);
      stats.compositeMs += performance.now() - t0;
    },

    // init returns the renderer state for canvas, or null without
    // WebGL2.
    init(canvas) {
      const gl = canvas.getContext("webgl2", {
        alpha: true,
        antialias: true,
        premultipliedAlpha: true,
        depth: false,
        stencil: false,
        preserveDrawingBuffer: false,
      });
      if (!gl) return null;
      const stage = document.createElement("canvas");
      const s = { gl, canvas, stage, sctx: stage.getContext("2d"), meshes: new Map() };
      setup(s);
      states.push(s);
      return s;
    },

    // restore rebuilds the program after webglcontextrestored. Every
    // GL object of the lost context is gone, meshes included; the
    // backend uploads them again on their next draw.
    restore(s) {
      s.meshes.clear();
      setup(s);
    },

    // upload stores mesh id from bytes: posLen bytes of float32 x,y
    // pairs, then colLen bytes of RGBA8 vertex colors (0 = none). An id
    // already stored keeps its buffers and takes the new data: the
    // backend refills transient meshes every frame (draw_webgl.go).
    upload(s, id, bytes, posLen, colLen, count) {
      const gl = s.gl;
      let m = s.meshes.get(id);
      if (!m) {
        m = { vao: gl.createVertexArray(), pos: gl.createBuffer(), col: null, count };
        s.meshes.set(id, m);
      }
      m.count = count;
      gl.bindVertexArray(m.vao);
      gl.bindBuffer(gl.ARRAY_BUFFER, m.pos);
      gl.bufferData(gl.ARRAY_BUFFER, bytes, gl.STATIC_DRAW, 0, posLen);
      gl.enableVertexAttribArray(0);
      gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
      if (colLen > 0) {
        if (!m.col) m.col = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, m.col);
        gl.bufferData(gl.ARRAY_BUFFER, bytes, gl.STATIC_DRAW, posLen, colLen);
        gl.enableVertexAttribArray(1);
        gl.vertexAttribPointer(1, 4, gl.UNSIGNED_BYTE, true, 0, 0);
      } else {
        // The shader then mixes in the uniform color (uUseV = 0).
        gl.disableVertexAttribArray(1);
      }
      gl.bindVertexArray(null);
    },

    remove(s, id) {
      const m = s.meshes.get(id);
      if (!m) return;
      const gl = s.gl;
      gl.deleteVertexArray(m.vao);
      gl.deleteBuffer(m.pos);
      if (m.col) gl.deleteBuffer(m.col);
      s.meshes.delete(id);
    },

    // resize sets the drawing buffer and the staging canvas, in device
    // pixels. Changing it reallocates, so the backend calls it only
    // when the size changed. It reports whether the drawing buffer got
    // the size: a browser may give a smaller one than asked.
    resize(s, w, h) {
      s.canvas.width = w;
      s.canvas.height = h;
      s.stage.width = w;
      s.stage.height = h;
      return s.gl.drawingBufferWidth === w && s.gl.drawingBufferHeight === h;
    },

    // draw renders n records whose device-pixel rect is (x, y, w, h),
    // top-left origin, at (x+ox, y+oy) of the canvas, after clearing
    // that area: the batch's place in this frame's atlas. dpi maps
    // logical to device pixels.
    draw(s, recs, n, dpi, x, y, w, h, ox, oy) {
      const t0 = performance.now();
      stats.records += n;
      const gl = s.gl;
      const cw = s.canvas.width;
      const ch = s.canvas.height;
      gl.viewport(0, 0, cw, ch);
      gl.enable(gl.SCISSOR_TEST);
      gl.scissor(x + ox, ch - (y + oy) - h, w, h);
      gl.clearColor(0, 0, 0, 0);
      gl.clear(gl.COLOR_BUFFER_BIT);
      gl.enable(gl.BLEND);
      gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
      gl.useProgram(s.prog);
      gl.uniform2f(s.uView, (2 * dpi) / cw, (2 * dpi) / ch);
      gl.uniform2f(s.uOff, (2 * ox) / cw, (2 * oy) / ch);
      for (let i = 0; i < n; i++) {
        const o = i * recStride;
        const m = s.meshes.get(recs[o]);
        if (!m) continue;
        // Scissor to the draw's clip within the batch rect; the area
        // outside the batch rect was not cleared and is not composited.
        let sx = x;
        let sy = y;
        let sr = x + w;
        let sb = y + h;
        if (recs[o + 15] >= 0) {
          sx = Math.max(sx, recs[o + 13]);
          sy = Math.max(sy, recs[o + 14]);
          sr = Math.min(sr, recs[o + 13] + recs[o + 15]);
          sb = Math.min(sb, recs[o + 14] + recs[o + 16]);
          if (sr <= sx || sb <= sy) continue;
        }
        gl.scissor(sx + ox, ch - (sb + oy), sr - sx, sb - sy);
        gl.bindVertexArray(m.vao);
        gl.uniform3f(s.uRow0, recs[o + 1], recs[o + 3], recs[o + 5]);
        gl.uniform3f(s.uRow1, recs[o + 2], recs[o + 4], recs[o + 6]);
        gl.uniform4f(s.uColor, recs[o + 7], recs[o + 8], recs[o + 9], recs[o + 10]);
        gl.uniform1f(s.uAlpha, recs[o + 11]);
        gl.uniform1f(s.uUseV, recs[o + 12]);
        gl.drawArrays(gl.TRIANGLES, 0, m.count);
      }
      gl.bindVertexArray(null);
      stats.drawMs += performance.now() - t0;
    },
  };
})();
