// bench.js — frame-time harness for web_svg_bench.
//
// 1. Calibrate: measure the display's frame interval on an idle page.
// 2. Start Go. The web backend calls goGuiOnFrame(layoutMs, drawMs,
//    painted) once per animation frame.
// 3. Warm up (tessellation, font load), then measure for a fixed time.
// 4. Report on the overlay, the console and window.benchResult, and
//    POST to /result when ?post=1 (serve.go records it).
//
// URL parameters, besides the workload ones main.go reads:
//   warmup    seconds before measuring  (default 3)
//   duration  seconds measured          (default 10)
//   label     free text stored with the result
//   post      1 = POST the result to /result
"use strict";

(() => {
  const params = new URLSearchParams(location.search);
  const num = (name, def) => {
    const v = parseFloat(params.get(name));
    return v > 0 ? v : def;
  };
  const warmupMs = num("warmup", 3) * 1000;
  const durationMs = num("duration", 10) * 1000;
  const overlay = document.getElementById("bench");
  // svg=canvas2d opts out of WebGL2; the backend overwrites the global
  // with the mode in effect (webgl, path2d or canvas2d).
  if (params.get("svg") !== null) globalThis.goGuiSvgMode = params.get("svg");

  const deltas = [];
  const layoutMs = [];
  const drawMs = [];
  let phase = "calibrate";

  const cmdCounts = [];
  const svgCmdCounts = [];
  const svgTriCounts = [];
  const compositeCounts = [];
  const filledCounts = [];
  const directCounts = [];
  // lose=1: lose the SVG WebGL context 2 s into the measurement and
  // restore it 2 s later, counting WebGL composites per phase.
  const lossTest =
    params.get("lose") === "1"
      ? { phase: "before", ext: null, frames: {}, composites: {}, filled: {} }
      : null;

  // Per painted frame: JS time inside the WebGL helper, and Go
  // allocation and GC per phase (bench_hook.go has the order).
  const jsDrawMs = [];
  const jsCompositeMs = [];
  const layoutAllocKB = [];
  const drawAllocKB = [];
  let layoutGCs = 0;
  let drawGCs = 0;
  let gcCPUMs = 0;
  let measuredFrames = 0;

  globalThis.goGuiOnFrame = (
    layout, draw, painted, cmds, svgCmds, svgTris, composites,
    layoutAllocB, drawAllocB, layoutGC, drawGC, gcCPU, filled, direct,
  ) => {
    // The helper's counters cover this frame; reset them either way.
    const glStats = globalThis.goGuiSvgGL?.stats;
    const jsDraw = glStats ? glStats.drawMs : 0;
    const jsComp = glStats ? glStats.compositeMs : 0;
    if (glStats) {
      glStats.drawMs = 0;
      glStats.compositeMs = 0;
      glStats.records = 0;
    }
    if (phase !== "measure") return;
    measuredFrames++;
    layoutMs.push(layout);
    layoutAllocKB.push((layoutAllocB || 0) / 1024);
    layoutGCs += layoutGC || 0;
    drawGCs += drawGC || 0;
    gcCPUMs += gcCPU || 0;
    if (!painted) return;
    drawMs.push(draw);
    jsDrawMs.push(jsDraw);
    jsCompositeMs.push(jsComp);
    drawAllocKB.push((drawAllocB || 0) / 1024);
    cmdCounts.push(cmds);
    svgCmdCounts.push(svgCmds);
    svgTriCounts.push(svgTris);
    compositeCounts.push(composites || 0);
    filledCounts.push(filled || 0);
    directCounts.push(direct || 0);
    if (lossTest) {
      const p = lossTest.phase;
      lossTest.frames[p] = (lossTest.frames[p] || 0) + 1;
      lossTest.composites[p] = (lossTest.composites[p] || 0) + (composites || 0);
      lossTest.filled[p] = (lossTest.filled[p] || 0) + (filled || 0);
    }
  };

  // postShot saves the canvas as shots/<name>.png (serve.go).
  const postShot = (name) =>
    new Promise((resolve) =>
      document.getElementById("go-gui-canvas").toBlob(resolve, "image/png"),
    ).then((png) =>
      fetch("/shot?name=" + encodeURIComponent(name), { method: "POST", body: png }),
    );

  const pct = (sorted, p) =>
    sorted.length === 0
      ? 0
      : sorted[Math.min(sorted.length - 1, Math.floor((p / 100) * sorted.length))];
  const mean = (a) => (a.length === 0 ? 0 : a.reduce((s, v) => s + v, 0) / a.length);
  const round = (v) => Math.round(v * 100) / 100;
  const stats = (a) => {
    const s = [...a].sort((x, y) => x - y);
    return {
      mean: round(mean(s)),
      p50: round(pct(s, 50)),
      p95: round(pct(s, 95)),
      p99: round(pct(s, 99)),
      max: round(s.length ? s[s.length - 1] : 0),
    };
  };

  // Calibrate on an idle page: the median of 60 frame intervals.
  function calibrate() {
    return new Promise((resolve) => {
      const samples = [];
      let last;
      const tick = (t) => {
        if (last !== undefined) samples.push(t - last);
        last = t;
        if (samples.length < 60) {
          requestAnimationFrame(tick);
          return;
        }
        samples.sort((x, y) => x - y);
        resolve(samples[samples.length >> 1]);
      };
      requestAnimationFrame(tick);
    });
  }

  // gpuRenderer names the WebGL renderer, so a software fallback
  // (SwiftShader, llvmpipe, WARP) is visible in the result.
  function gpuRenderer() {
    try {
      const gl = document.createElement("canvas").getContext("webgl2");
      if (!gl) return "no webgl2";
      const ext = gl.getExtension("WEBGL_debug_renderer_info");
      const name = gl.getParameter(ext ? ext.UNMASKED_RENDERER_WEBGL : gl.RENDERER);
      gl.getExtension("WEBGL_lose_context")?.loseContext();
      return name;
    } catch (e) {
      return "unknown: " + e;
    }
  }

  function report(refreshMs) {
    // Dropped = vsyncs that elapsed minus frames shown. Rounding each
    // interval on its own over-counts: 1.6 refreshes would round to 2.
    const elapsed = deltas.reduce((s, d) => s + d, 0);
    const expected = elapsed / refreshMs;
    const dropped = Math.max(0, Math.round(expected) - deltas.length);
    const result = {
      label: params.get("label") || "",
      mode: globalThis.goGuiSvgMode || "canvas2d",
      workload: Object.fromEntries(
        ["rows", "cols", "cell", "asset", "period", "xf"].map((k) => [k, params.get(k) || "default"]),
      ),
      userAgent: navigator.userAgent,
      gpu: gpuRenderer(),
      dpr: devicePixelRatio,
      viewport: `${innerWidth}x${innerHeight}`,
      refreshMs: round(refreshMs),
      frames: deltas.length,
      fps: round(deltas.length / (durationMs / 1000)),
      droppedFrames: dropped,
      droppedPct: round((100 * dropped) / expected),
      frameMs: stats(deltas),
      goLayoutMs: stats(layoutMs),
      goDrawMs: stats(drawMs),
      perFrame: {
        cmds: Math.round(mean(cmdCounts)),
        svgCmds: Math.round(mean(svgCmdCounts)),
        svgTris: Math.round(mean(svgTriCounts)),
        composites: Math.round(mean(compositeCounts)),
        filledBatches: Math.round(mean(filledCounts)),
        directBatches: Math.round(mean(directCounts)),
      },
      lossTest: lossTest && {
        frames: lossTest.frames,
        composites: lossTest.composites,
        filled: lossTest.filled,
      },
      // Go's draw time includes the synchronous JS calls it makes;
      // goOnly is what remains for Go itself.
      drawSplitMs: {
        jsDraw: round(mean(jsDrawMs)),
        jsComposite: round(mean(jsCompositeMs)),
        goOnly: round(mean(drawMs) - mean(jsDrawMs) - mean(jsCompositeMs)),
      },
      gc: {
        layoutAllocKB: round(mean(layoutAllocKB)),
        drawAllocKB: round(mean(drawAllocKB)),
        cyclesPerSec: round((layoutGCs + drawGCs) / (durationMs / 1000)),
        layoutCycles: layoutGCs,
        drawCycles: drawGCs,
        cpuMsPerFrame: round(measuredFrames ? gcCPUMs / measuredFrames : 0),
      },
    };
    globalThis.benchResult = result;
    console.log("bench result", JSON.stringify(result));
    overlay.textContent =
      `mode      ${result.mode}  ${result.viewport} @${result.dpr}\n` +
      `gpu       ${result.gpu}\n` +
      `fps       ${result.fps}  (refresh ${result.refreshMs} ms)\n` +
      `dropped   ${dropped} (${result.droppedPct}%)\n` +
      `frame ms  p50 ${result.frameMs.p50}  p95 ${result.frameMs.p95}  p99 ${result.frameMs.p99}  max ${result.frameMs.max}\n` +
      `go layout mean ${result.goLayoutMs.mean}  p95 ${result.goLayoutMs.p95}\n` +
      `go draw   mean ${result.goDrawMs.mean}  p95 ${result.goDrawMs.p95}\n` +
      `per frame ${result.perFrame.cmds} cmds, ${result.perFrame.svgCmds} svg, ${result.perFrame.svgTris} tris, ${result.perFrame.composites} composites (${result.perFrame.directBatches} direct), ${result.perFrame.filledBatches} filled
` +
      `draw split js ${result.drawSplitMs.jsDraw} + composite ${result.drawSplitMs.jsComposite} + go ${result.drawSplitMs.goOnly} ms
` +
      `alloc KB  layout ${result.gc.layoutAllocKB}  draw ${result.gc.drawAllocKB}
` +
      `gc        ${result.gc.cyclesPerSec}/s (layout ${result.gc.layoutCycles}, draw ${result.gc.drawCycles}), ~${result.gc.cpuMsPerFrame} ms/frame`;
    // The screenshot goes first: the result POST is what tells a
    // driver script the run is over.
    const shot =
      params.get("shot") === "1" ? postShot(result.label || result.mode) : Promise.resolve();
    if (params.get("post") === "1") {
      shot
        .catch((e) => console.warn("bench: shot failed", e))
        .then(() => fetch("/result", { method: "POST", body: JSON.stringify(result) }))
        .catch((e) => console.warn("bench: post failed", e));
    }
  }

  async function main() {
    const refreshMs = await calibrate();
    overlay.textContent = `refresh ${round(refreshMs)} ms — loading…`;

    const go = new Go();
    const { instance } = await WebAssembly.instantiateStreaming(
      fetch("main.wasm"),
      go.importObject,
    );
    go.run(instance);

    const label = params.get("label") || "lose";
    const driveLoss = (elapsed) => {
      if (lossTest.phase === "before" && elapsed >= 2000) {
        const st = globalThis.goGuiSvgGL?.states?.[0];
        lossTest.ext = st && st.gl.getExtension("WEBGL_lose_context");
        if (!lossTest.ext) {
          lossTest.phase = "unsupported";
          return;
        }
        lossTest.ext.loseContext();
        lossTest.phase = "lost";
        setTimeout(() => postShot(label + "-lost"), 800);
      } else if (lossTest.phase === "lost" && elapsed >= 4000) {
        lossTest.ext.restoreContext();
        lossTest.phase = "restored";
        setTimeout(() => postShot(label + "-restored"), 1500);
      }
    };

    const start = performance.now();
    let last;
    let measureStart;
    phase = "warmup";
    const tick = (t) => {
      if (phase === "warmup" && t - start >= warmupMs) {
        phase = "measure";
        measureStart = t;
        last = t;
        overlay.textContent = "measuring…";
      } else if (phase === "measure") {
        deltas.push(t - last);
        last = t;
        if (lossTest) driveLoss(t - measureStart);
        if (t - measureStart >= durationMs) {
          phase = "done";
          report(refreshMs);
          return;
        }
      }
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }

  main().catch((e) => {
    overlay.textContent = "bench failed: " + e;
    console.error(e);
  });
})();
