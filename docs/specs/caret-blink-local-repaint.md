# Caret Blink Repaints the Caret, Not the Window

## Problem

A caret blink invalidated every renderer in the window. Each 600 ms blink tick,
`BlinkCursorAnimation.Update` toggled the `inputCursorOn` atomic and reported
`animationRefreshRenderOnly`; the animation loop translated that into
`commandMarkRenderOnlyRefresh`, which set `w.refreshRenderOnly` and made the
next `FrameFn` run `updateRenderOnly` → `buildRenderers` → `renderLayout` over
the whole layout tree.

That walk re-runs every shape's draw code. For a consumer with an expensive draw
surface (go-term's grid: per-cell glyph lookups, run-key hashing, per-run slice
allocation) an idle focused input re-rendered the entire window 1.7×/s just to
toggle one blinking rectangle.

## Root cause

The caret is not a standalone renderer. `renderInputCursor` emits the caret rect
inline during the tree walk, and its visibility is decided by reading the blink
atomic at emit time. So the only way to change the caret's appearance was to
re-walk the tree. Additionally, backends present a frame only when `FrameFn`
returns true, and true implied a rebuild — there was no "present without
rebuild" path.

## Fix

Scope the caret repaint to the caret by patching the single caret `RenderCmd` in
place. The caret rect is now **always emitted** for the focused caret-drawing
shape — transparent while the blink state is off — and its index plus true color
are recorded in window state during `buildRenderers` (`caretCmdState` on
`Window`). Because the render list only changes on a rebuild, and every rebuild
re-records the caret, the recorded index is stable between frames.

The blink animation then needs no refresh at all:

- `BlinkCursorAnimation.RefreshKind` returns `animationRefreshNone`.
- Each toggle enqueues `commandToggleCaretBlink` (via `AnimationCommands`, run
  by `flushCommands` on the main thread), which patches
  `w.renderers[caretCmd.idx].Color` to the caret color or `ColorTransparent` per
  the current blink state, and sets `w.renderersDirty`.
- `FrameFn` ORs `renderersDirty` into its return value (and clears it), so the
  backend presents the patched list without a rebuild.

Cost per blink tick: O(1) and one frame present. The tree walk, `syncIME`
probing, and every shape's draw code are untouched.

### Why patch-in-place rather than the alternatives

- **Per-shape dirty marks / subtree rebuild** would need to replay ancestor
  bracket context (clip, stencil, filter, rotate) — that context is carried by
  mutable walk state (`w.clipRadius`, `w.stencilDepth`, `w.inFilter`), not per
  node.
- **Caret as its own shape** would need clip-context capture at emit time and
  changes draw ordering.
- The patched cmd keeps its position in the flat list, so it stays inside
  exactly the brackets (scroll clip, stencil, filter) it was drawn in.

## Invariants

- Caret geometry (position, size) can only change via typing, scrolling, or
  focus — all of which rebuild and re-record.
- Only the focused caret-drawing shape emits and records; focus is exclusive.
- With no caret recorded (IME composing, headless render, nothing focused) the
  toggle is a no-op and does not mark the window dirty.
- A `Pulsar` (which toggles text in the view function) still promotes blink
  ticks to a layout refresh via its own `Animate` — unchanged behavior.

## Window focus

The blink is gated on the window holding OS focus as well as on the focused
widget drawing a caret. A background window receives no key events, so its caret
marks an insertion point nobody can type into — and the blink is not free: it
holds the 16 ms animation ticker open and calls `wakeMain` twice a second for
the whole time the app sits idle.

`syncBlinkCursor` (`gui/ime_context.go`) folds `Window.focused` into the caret
signal. Losing focus retires the animation, which empties `w.animations` and
parks the ticker in `animationLoop`; `handleFocusedEvent` calls
`resetBlinkCursorVisible` on the way back, so the caret returns solid with a
fresh phase rather than mid-blink, and the rebuild `EventFn` queues re-registers
the animation.

`renderInputCursor` paints the caret transparent while the window is unfocused,
and `commandToggleCaretBlink` carries the same gate — a `Pulsar` keeps the blink
animation registered in a background window, and its toggles must not paint the
caret back in. The caret rect is still emitted, so the invariant above holds:
the render list has one shape across focus states and the recorded slot stays
valid.

## Files

- `gui/render_text.go` — `renderInputCursor` always emits; `caretCmdState`,
  `emitCaretCmd`, `commandToggleCaretBlink`.
- `gui/window.go` — `caretCmd`, `renderersDirty` fields.
- `gui/window_update.go` — `buildRenderers` resets `caretCmd`; `FrameFn`
  presents and clears `renderersDirty`.
- `gui/ime_context.go` — `syncBlinkCursor` window-focus gate.
- `gui/window_event.go` — `handleFocusedEvent` resets the blink phase.
- `gui/animation.go` / `gui/animation_loop.go` — `RefreshKind` none; the toggle
  command is enqueued per tick.
