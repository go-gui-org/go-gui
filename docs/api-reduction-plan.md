# API Reduction Plan for go-gui 1.0

## Context

go-gui has 2,164 exported symbols. For 1.0, the public interface must be
reduced to only what app authors actually need.

**Guiding principle:** if an app author can't use it without reaching into
internal machinery, it shouldn't be exported.

## Progress

Branch `api-reduction`, 5 commits:

| Commit | What | ~Symbols |
|--------|------|----------|
| `96e93ab` | Phase 1a — unexport Shape/render internals (ShapeType, DrawClip, ShapeButtonColors, etc.) | 40 |
| `cda6470` | Phases 1c,4a,4c — unexport Layout/pipeline internals, move shader consts to `gui/shader/`, remove redundant exports | 45 |
| `2d030fb` | Phases 1c cont, 5 — unexport DataGrid internals (23 funcs/types), remove 3 widget aliases (Split/Tabs/Checkbox), delete 3 dead export funcs | 29 |
| `ab8b709` | Phases 3+8 — unexport 5 internal Window methods (UpdateRenderOnly, RequestRenderOnly, InputCursorOn, SetMouseCursor), delete 1 dead method (RenderersCount) | 6 |
| *next* | Phases 3+8 cont — unexport 7 more Window methods (SetState, ClearViewState, HasFocus, SetMouseCursorIBeam, SetMouseCursorResizeNESW, SetMouseCursorResizeNWSE, SetMouseCursorNotAllowed) | 7 |

**Total: ~127 symbols removed.** `go doc gui` output: ~772 lines (down from ~790).

## What's Done

### Phase 1a — Render/Shape Internals
- [x] Unexport ShapeType, all Shape* consts
- [x] Unexport DrawClip, ShapeButtonColors, ShapeEffects
- [x] Unexport EventHandlers, AccessiblePath, RoundedImageClip, FilterBracketRange
- [x] Unexport *Shape methods: HasRtfLayout, HasEvents, PaddingWidth, PaddingHeight
- [x] Unexport WindowRect

### Phase 1c — Layout/Pipeline Internals
- [x] Unexport GenerateViewLayout, ApplyFixedSizingConstraints
- [x] Unexport ListCoreItem/Cfg/Prepared/Action and all ListCore* consts
- [x] Unexport *Layout methods except FindShape, FindLayout, FindByID, NextFocusable, PreviousFocusable
- [x] Unexport all *View types (containerView, buttonView, textView, etc.)

### Phase 1c cont — DataGrid Internals
- [x] 15 funcs: GridDataFromCSV, GridRowsToCSV/TSV/PDF/XLSX variants, GridOrmValidateQuery/BuildSQL/EscapeLike/ValidDBField, DataGridColumnOrderMove, GridQuerySignature
- [x] 8 types/consts: GridCsvData, GridExportCfg, GridMutationKind, GridAggregateOp, GridCursorPageReq, GridOffsetPageReq, GridPageRequest
- [x] 3 dead funcs removed: GridRowsToPDFFile, *ToXLSXFile*

### Phase 4a — Shader Constants
- [x] Create `gui/shader/` package, move all Vs*/Fs* GLSL and Metal constants

### Phase 4c — Redundant Exports
- [x] Unexport/remove ColorTransparent, NoBorder, NoSpacing, NoRadius, PaddingOne–PaddingTwoFive, Commit, ValidImageExtensions, BaseFontName, IconFontName, MenuSeparatorID, MenuSubtitleID, ScrollDeltaHome, ToastPersistent

### Phase 5 — Widget Alias Removal
- [x] Remove Split() (use Splitter), Tabs() (use TabControl), Checkbox() (use Toggle)

### Phases 3+8 — Window Method Cleanup
- [x] Unexport UpdateRenderOnly, RequestRenderOnly, InputCursorOn, SetMouseCursor
- [x] Delete dead RenderersCount
- [x] Unexport SetState, ClearViewState, HasFocus
- [x] Unexport SetMouseCursorIBeam, SetMouseCursorResizeNESW, SetMouseCursorResizeNWSE, SetMouseCursorNotAllowed (zero usage)
- [x] Keep SetMouseCursorArrow, SetMouseCursorCrosshair, SetMouseCursorPointingHand, SetMouseCursorAll (used by sibling projects)

## What's Left (actionable)

### Phase 4b — Theme Var Consolidation (breaking)
Reduce 14 theme vars to 3: `ThemeDark`, `ThemeLight`, `ThemeBlue`.
Unexport all `*Cfg` and `*NoPadding`/`*Bordered` variants.
Add `Theme.WithPadding(bool)`, `Theme.WithBorders(bool)` builder methods.
Update 80+ references across examples and sibling projects.

### Phases 3+8 — More Window Methods
Full audit complete. 45 exported → 38 remaining. Candidates kept because:
- **Backend-injected (must stay):** Lock, Unlock, Close, MouseCursorState,
  SetWakeMainFn, SetTextMeasurer, SetClipboardGetFn, SetClipboardFn, Renderers,
  SetTitleFn, SetTitle, CloseRequested, WindowSize
- **Used by siblings:** SetMouseCursorArrow (16), SetMouseCursorPointingHand (12),
  SetMouseCursorCrosshair, SetMouseCursorAll, TextMeasurer, FrameCount,
  ClearDrawCanvasCache, GetClipboard
- **Used by examples:** IsFocus, MouseLock, MouseUnlock, Now, SetIDFocus, SetTheme,
  TextWidth, Timings, App, PlatformID, SetClipboard
- **Reasonable public API (used internally):** Ctx, IDFocus, PointerOverApp,
  SetMouseCursorNS, SetMouseCursorEW, MouseIsLocked

No clear candidates remain in Window methods. Phase 3+8 complete.

## What's Blocked

### Sub-package Extraction (Phases 2, 7)
Planned to extract DataGrid → `gui/datagrid/`, Print → `gui/print/`,
Animation → `gui/animation/`, Dock → `gui/dock/`, Form → `gui/form/`,
utility funcs → `guiutil/`.

**Blocked by circular dependency.** Subsystems import `gui/` core types
(Layout, Window, Color, Theme). Moving types from `gui/` to a sub-package
creates `gui/` → sub-pkg → `gui/`. Requires three-way split (shared types
→ `gui/internal/`). Deferred to post-1.0.

### SVG Internals (Phase 1b)
`gui/svg/` imports types from `gui/` (SvgAnimation, SvgAlign, SvgPrimitive,
etc.). Same circular dependency. Deferred to post-1.0.

### Animation Internals (Phase 1d)
All candidate types (BlinkCursorAnimation, HeroTransition, KeyframeAnimation,
SpringAnimation, TweenAnimation, LayoutTransition, AnimationRefreshKind) are
used by examples or sibling projects. Nothing safe to unexport.

### Shape/Render Backend Types (Phase 1a leftovers)
RenderCmd, ShapeTextConfig, AccessInfo, A11yNode, TessellatedPath — all used
by `gui/backend/` or `gui/svg/` sub-packages. Same circular dependency.

### Utility Funcs (Phase 7)
Lerp, PackRGB, SampleGradientStopColor, PreserveAlignFractions, ShaderHash,
etc. — all used by `gui/svg/` and `gui/backend/` sub-packages. Same issue.

## Sibling Project Status

All 4 commits verified against go-charts, go-edit, go-kite. No regressions.
Phase 4b (theme consolidation) will require updating go-charts and go-edit.
