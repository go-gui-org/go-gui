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
