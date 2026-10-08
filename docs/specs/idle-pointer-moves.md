# Spec: idle pointer moves skip the rebuild

Issue: #973

Status: **implemented** on branch `perf/973-skip-idle-mouse-move`, not yet
released.

## Problem

`EventFn` asked for a full layout rebuild after every event. The view phase is
the per-frame allocator, so a pointer crossing an idle window paid the full view
cost on every frame. `DebugRebuilds` (#970) showed it as `input` on every frame
while the pointer moved.

The rebuild was not only waste. Hover is computed during arrange, not at event
time: `layoutArrange` fires `OnHover` and `OnMouseLeave` from the pointer
position and records the `IsHovered` target. Handlers that change state on a
move also relied on the blanket rebuild. The go-charts crosshairs, for example,
move with the pointer inside one shape and ask for no refresh.

## Decision

A pointer move is **idle** when the last arranged frame proves nothing reacts to
it. An idle move records the pointer position and asks for no rebuild and no
wake. `idleMoveAt` (`gui/window_idle_move.go`) replays the arrange-time
decisions for the position the last arrange saw (`arrangedMouseX/Y`) and for the
new one. Any answer that differs, or that it cannot prove, means a rebuild.

A move is not idle when:

1. a drag holds the mouse, a `Window.OnEvent` handler is set, or the menu is in
   keyboard navigation mode;
2. the pointer was outside the window;
3. any tooltip is pending or shown;
4. the `IsHovered` target at the new position differs;
5. a shape with `OnMouseMove` is under the new or the previous position;
6. a shape with `OnHover` or `OnMouseLeave` is entered or left;
7. the pointer stays inside an `OnHover` shape whose hover is not marked
   `hoverStatic`;
8. a `pointerAmend` shape (WithTooltip) holds the old or the new position;
9. for a mouse move, the cursor is not the one a real move would leave: the
   arrow, or the cursor the last arrange pass set (`arrangeSetCursor`,
   `arrangedCursor`). A cursor set at event time, by an RTF link's `OnMouseMove`
   or by a drag, would otherwise stay after the pointer leaves.

The walk looks at every shape, not only those dispatch would reach. A shape
dispatch would skip only costs a rebuild, never a stale frame.

A one-finger touch drag under the pan threshold takes the same check. Its
`mousePosX/Y` does not move, so only the target and `OnMouseMove` tests can
fail.

### `hoverStatic`

An unexported `ContainerCfg` and `eventHandlers` flag. It marks an `OnHover`
whose result depends only on whether the pointer is over the shape, never on
where. Set for Button (without an app `OnHover`), list rows (without
`OnItemHover`), listbox rows, tree rows, select options and menu items. Each was
checked to read no pointer position and to call no app code.

### `ContainerCfg.Hover` (#977)

Code outside package `gui` cannot set `hoverStatic`. It uses the exported
`ContainerCfg.Hover` (`HoverStyle{Color, Cursor}`) instead: gui paints the fill
and the cursor during arrange, before any `OnHover`. A shape whose only hover is
a `HoverStyle` is static by construction, because no app code runs. A shape with
both is not static. The datagrid rows and sortable header cells use it.

The datagrid also had an `OnMouseMove` on the grid root. It hit-tested the
header cells on every move to record the hovered column, so rule 5 rebuilt every
move over the grid, rows included. It is gone. The grid now reads the hovered
column at generation time with `IsHovered` on each header cell. A new hover
target asks for one more layout pass in the same frame, so the answer is never a
frame behind. The header controls (resize, pin, reorder) have absolute IDs, so a
pointer over one does not make its cell hovered. The grid also checks the
control IDs of the columns that can show controls: the one hovered last frame,
the focused one and the one being resized. Without this check the controls hide
under the pointer and show again on the next frame.

The resize handle keeps its `OnHover`: it picks the pressed color from the held
mouse button, which `HoverStyle` does not express.

### `OnMouseLeave` counts arrange passes

The hover record for `OnMouseLeave` stored the frame the pointer was last
inside, and a record older than one frame read as stale. A frame that arranges
nothing (a caret blink, and now an idle move) aged the record, and the leave was
lost. It now stores `arrangePass`, which only an arrange pass moves. This was a
latent bug before #973; skipping moves made it common.

### Counting

`Window.idleMovesSkipped` counts every idle move, for tests. `idleMovesPending`
counts the ones since the last pass; `noteRefresh` prints
`gui: skipped N idle moves` in front of that pass when `DebugRebuilds` is on.

## Rejected Approaches

- **An exported `HoverStatic bool` flag on `ContainerCfg`** (#977). Rejected:
  setting it on an `OnHover` that reads the position fails silently as a stale
  frame. `HoverStyle` cannot be set wrong, and it also removes the per-frame
  hover closure.
- **A second callback, `OnHoverStatic`** (#977). Rejected: the same capability
  as the flag, plus a precedence rule when both callbacks are set.
- **An internal seam only `gui/datagrid` can reach** (#977). Rejected: it covers
  only the datagrid, and needs an `any`-typed hook to avoid an import cycle.
- **`ColorHover` on `ContainerCfg`** (#977). Rejected: `gui/CLAUDE.md` makes
  `Colors` the only spelling for per-state colors (#721).
- **Datagrid header hover from an `OnMouseMove` on the header row only, with an
  `OnMouseLeave` to clear it** (#977). Rejected: the leave runs during arrange,
  after generation, so the controls could stay one frame too long. `IsHovered`
  was built for reads at generation time (#587).

- **Handlers ask for a refresh** (like `ctx.Consume()`): a move rebuilds only if
  a handler requested it. Rejected: about 50 handlers in the sibling repos must
  change, mostly in go-charts, and a missed one fails as a silent stale frame.
- **`OnHover` means enter and leave only**: rebuild on hover transitions, not on
  moves inside a shape. Rejected: a semantic break. It freezes the go-charts
  crosshairs and every handler that reads the pointer position.
- **Render-only refresh for moves**: `OnHover` rewrites shape colors during
  arrange, and a render-only pass skips arrange, so hover would freeze.
- **Conservative check only (no `hoverStatic`)**: correct, but every move over a
  button or a list row still rebuilds, which is most of the win.

## Consequence for apps

A view that reads changing state without asking for a refresh (a clock, data
another goroutine writes) used to catch up whenever the mouse moved. It no
longer does. Call `InvalidateLayout` when the state changes, or run an
animation.
