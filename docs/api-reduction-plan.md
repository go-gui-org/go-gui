# API Reduction Plan for go-gui 1.0

## Context

go-gui has 2,164 exported symbols (380 types, 235 funcs, 412 methods, 1,019 consts, 118 vars). Of these, 1,436 (70%) are never referenced in any example app. The public surface has grown organically — internal implementation details, backend-facing interfaces, and single-use helpers are all exported alongside the true app-writer API. For 1.0, the public interface must be reduced to only what app authors actually need.

**Guiding principle:** if an app author can't use it without reaching into internal machinery, it shouldn't be exported. Backend authors are a secondary audience; their surface goes through `gui/backend/` packages.

## Audit Summary

| Category | Total Exported | Used in Examples | % Unused |
|----------|---------------|-----------------|----------|
| Types | 380 | ~120 | 68% |
| Functions | 235 | ~95 | 60% |
| Methods | 412 | ~90 | 78% |
| Consts | 1,019 | ~300 | 71% |
| Vars | 118 | ~70 | 41% |
| **Total** | **2,164** | **~611** | **70%** |

Sibling projects (go-charts, go-edit, go-kite) use ~40-50 symbols each, mostly core types (Window, Layout, Color, Event, DrawContext).

## Decisions

- [x] datagrid → `gui/datagrid/` — ~80 types
- [x] print → `gui/print/` — ~25 types
- [x] animation → `gui/animation/` — ~30 types
- [x] dock → `gui/dock/` — ~15 types
- [x] form → `gui/form/` — ~30 types
- [x] SVG internals → `gui/svg/` — ~40 types (keep constructors at top level)
- [x] Shader consts → `gui/shader/` — ~40 GLSL/Metal constants
- [x] Window methods: aggressive slimming — 139 → ~50
- [x] Utility funcs → `guiutil/` — ~15 math/geometry helpers

---

## Phase 0: Pre-Work Documentation

- [ ] Rewrite `gui/doc.go` — reflect slimmed surface, reference sub-packages
- [ ] Update `docs/architecture.md` — new package layout, dependency diagram
- [ ] Create `doc.go` for each new sub-package (`datagrid/`, `print/`, `animation/`, `dock/`, `form/`, `svg/`, `shader/`, `guiutil/`)

---

## Phase 1: Unexport Internal Implementation Details (~200 symbols)

### 1a. Render/Shape Internals
- [x] Unexport `ShapeType`, all `Shape*` consts
- [x] Unexport `DrawClip`
- [x] Unexport `ShapeButtonColors`, `ShapeEffects`
- [x] Unexport `EventHandlers`
- [x] Unexport `AccessiblePath`
- [x] Unexport `RoundedImageClip`, `FilterBracketRange`
- [x] Unexport `*Shape` methods: `HasRtfLayout`, `HasEvents`, `PaddingWidth`, `PaddingHeight` (keep `PaddingLeft`, `PaddingTop` — go-charts uses them)
- [x] Unexport `WindowRect` (return type is unexported)
- [ ] Unexport `RenderCmd`, `RenderKind`, all `Render*` consts — **deferred**: used by backends (126+ refs), will move to shared package in Phase 2
- [ ] Unexport `ShapeTextConfig` — **deferred**: go-kite accesses `shape.TC.TextStyle`
- [ ] Unexport `AccessInfo` — **deferred**: `ContainerCfg.A11Y` field dependency
- [ ] Unexport `A11yNode` — **deferred**: used by backends (18 refs)
- [ ] Unexport `TessellatedPath` — **deferred**: used by `gui/svg/` sub-package

### 1b. SVG Internals → `gui/svg/`
- [ ] Move `CachedSvg`, `CachedSvgPath`, `CachedSvgTextDraw`, `CachedSvgTextPathDraw`, `CachedFilteredGroup`
- [ ] Move `SvgParsed`, `SvgParsedFilteredGroup`, `SvgParseOpts`, `SvgColor`
- [ ] Move `SvgText`, `SvgTextPath`, `SvgFilter`, `SvgGradientStop`, `SvgGradientDef`, `SvgGradientSpread`
- [ ] Move `SvgA11y`, `SvgAnimation`, `SvgPrimitive`, `SvgPrimitiveKind`, all `SvgPrim*` consts
- [ ] Move all `SvgAnim*` types and consts
- [ ] Move `SvgAlign`, `SvgAlign*`, `SvgAttrName`, `SvgAttr*` consts
- [ ] Move `StrokeCap`, `StrokeJoin`, `ButtCap`, `RoundCap`, `SquareCap`, `MiterJoin`, `RoundJoin`, `BevelJoin`
- [ ] Move `SvgParser`, `SvgParserWithOpts`, `AnimatedSvgParser`, `TessellatedPath`
- [ ] Keep `SvgCfg`, `Svg()`, `SvgSpinnerCfg`, `SvgSpinner()`, `SvgSpinnerKind`, `SvgSpinnerName`, `SvgSpinnerCount`, all `SvgSpinner*` consts

### 1c. Layout/Pipeline Internals
- [ ] Unexport `GenerateViewLayout`
- [ ] Unexport `ApplyFixedSizingConstraints`
- [ ] Unexport `ListCoreItem`, `ListCoreCfg`, `ListCorePrepared`, `ListCoreAction`, all `ListCore*` consts
- [ ] Unexport `*Layout` methods except `FindShape`, `FindLayout`, `FindByID`, `NextFocusable`, `PreviousFocusable`
- [ ] Unexport all `*View` types (`containerView`, `buttonView`, `textView`, etc.)

### 1d. Animation Internals
- [ ] Unexport `AnimationRefreshKind`, `AnimationRefreshNone`, `AnimationRefreshRenderOnly`, `AnimationRefreshLayout`

---

## Phase 2: Move Subsystems to Sub-Packages (~300 symbols)

### 2a. `gui/datagrid/` — Data Grid Subsystem
- [ ] Create `gui/datagrid/` package
- [ ] Move `DataGridCfg`, `DataGridDataSource`, `DataGridSourceStats`, `DataGridStyle`
- [ ] Move `GridColumnCfg`, `GridColumnPin`, `GridAggregateCfg`, `GridAggregateOp`
- [ ] Move `GridCellEdit`, `GridCellEditorKind`, `GridCellFormat`
- [ ] Move `GridCsvData`, `GridExportCfg`
- [ ] Move `GridRow`, `GridSelection`, `GridSort`, `GridSortDir`, `GridFilter`
- [ ] Move `GridQueryState`, `GridQuerySignature`
- [ ] Move `GridPaginationKind`, `GridPageRequest`, `GridCursorPageReq`, `GridOffsetPageReq`
- [ ] Move `GridAbortController`, `GridAbortSignal`
- [ ] Move `GridDataRequest`, `GridDataResult`, `GridDataCapabilities`
- [ ] Move `GridMutationKind`, `GridMutationRequest`, `GridMutationResult`
- [ ] Move `GridOrmDataSource`, `GridOrmColumnSpec`, `GridOrmQuerySpec`, `GridOrmPage`
- [ ] Move `GridOrmFetchFn`, `GridOrmCreateFn`, `GridOrmUpdateFn`, `GridOrmDeleteFn`, `GridOrmDeleteManyFn`, `GridOrmSQLBuilder`
- [ ] Move `InMemoryDataSource`, `NewInMemoryDataSource`
- [ ] Move `NewGridOrmDataSource`, `NewGridAbortController`
- [ ] Move `GridOrmValidateQuery`, `GridOrmBuildSQL`, `GridOrmEscapeLike`, `GridOrmValidDBField`
- [ ] Move `DataGridColumnOrderMove`
- [ ] Move `GridDataFromCSV`
- [ ] Move `GridRowsToCSV`, `GridRowsToCSVWithCfg`, `GridRowsToTSV`, `GridRowsToTSVWithCfg`
- [ ] Move `GridRowsToPDF`, `GridRowsToPDFFile`
- [ ] Move `GridRowsToXLSX`, `GridRowsToXLSXWithCfg`, `GridRowsToXLSXFile`, `GridRowsToXLSXFileWithCfg`
- [ ] Add deprecation shims in `gui/` pointing to `gui/datagrid/`
- [ ] Update go-charts imports

### 2b. `gui/print/` — Print Subsystem
- [ ] Create `gui/print/` package
- [ ] Move `PrintJob`, `NewPrintJob`, `PrintJobSource`, `PrintJobSourceKind`
- [ ] Move `PrintMargins`, `DefaultPrintMargins`, `PrintPageRange`, `NormalizePrintPageRanges`, `PrintPageSize`
- [ ] Move `PrintOrientation`, `PrintScaleMode`, `PrintDuplexMode`, `PrintColorMode`
- [ ] Move `PrintHeaderFooterCfg`, `PrintRunResult`, `PrintRunStatus`, `PrintWarning`, `PrintExportResult`, `PrintExportStatus`
- [ ] Move `PaperSize`, `PaperLetter`, `PaperLegal`, `PaperA4`, `PaperA3`
- [ ] Move all `Print*` consts
- [ ] Add deprecation shims in `gui/`

### 2c. `gui/animation/` — Animation Subsystem
- [ ] Create `gui/animation/` package
- [ ] Move `Animation` interface, `Animate`, `AnimationCommands`
- [ ] Move `BlinkCursorAnimation`, `NewBlinkCursorAnimation`
- [ ] Move `HeroTransition`, `NewHeroTransition`, `HeroTransitionCfg`
- [ ] Move `KeyframeAnimation`, `NewKeyframeAnimation`, `Keyframe`
- [ ] Move `SpringAnimation`, `NewSpringAnimation`, `SpringCfg`
- [ ] Move `SpringDefault`, `SpringGentle`, `SpringBouncy`, `SpringStiff`
- [ ] Move `TweenAnimation`, `NewTweenAnimation`
- [ ] Move `LayoutTransition`, `LayoutTransitionCfg`
- [ ] Move `EasingFn`, all `Ease*` funcs, `CubicBezier`
- [ ] Keep `AnimationTooltip()` re-export in `gui/`
- [ ] Add deprecation shims in `gui/`

### 2d. `gui/dock/` — Dock Layout Subsystem
- [ ] Create `gui/dock/` package
- [ ] Move `DockNode`, `DockNodeKind`, `DockSplitDir`, `DockDropZone`
- [ ] Move `DockPanelDef`, `DockLayoutCfg`
- [ ] Move `DockSplit`, `DockPanelGroup`, `DockNodeSanitize`
- [ ] Move all `DockTree*` funcs
- [ ] Move all `Dock*` consts
- [ ] Keep `DockLayout()` re-export in `gui/`
- [ ] Add deprecation shims in `gui/`

### 2e. `gui/form/` — Form Validation Subsystem
- [ ] Create `gui/form/` package
- [ ] Move `FormCfg`, `FormIssue`, `FormIssueKind`, `FormSnapshot`, `FormFieldSnapshot`
- [ ] Move `FormFieldState`, `FormSummaryState`, `FormPendingState`
- [ ] Move `FormSubmitEvent`, `FormResetEvent`
- [ ] Move `FormSyncValidator`, `FormAsyncValidator`
- [ ] Move `FormFieldAdapterCfg`, `FormValidateOn`, `FormValidationTrigger`
- [ ] Move `FormOnFieldEvent`, `FormOnFieldEventByID`
- [ ] Move `FormRegisterField`, `FormRegisterFieldByID`
- [ ] Move `FormRequestSubmit`, `FormRequestReset`, `FormRequestSubmitForLayout`
- [ ] Move `FormFindAncestorID`
- [ ] Move all `Form*` consts
- [ ] Keep `Form()` re-export in `gui/`
- [ ] Add deprecation shims in `gui/`

---

## Phase 3: Slim Down Method Surface

### 3a. *Window Methods — Unexport Internal (~60 methods)
- [ ] Unexport `WindowCleanup`, `UpdateWindow`, `Update`, `UpdateRenderOnly`
- [ ] Unexport `RequestRenderOnly`, `RequestRedraw`, `FrameFn`, `UpdateView`
- [ ] Unexport `EventFn`, `QueueCommand`, `QueueValueCommand`, `QueueAnimateCommand`
- [ ] Unexport `Renderers`, `Timings`, `FrameCount`, `Now`
- [ ] Unexport `SetWakeMainFn`, `SetTextMeasurer`, `TextMeasurer`
- [ ] Unexport `SetClipboardFn`, `SetClipboardGetFn`
- [ ] Unexport `SetTitleFn`, `SetSvgParser`, `SvgParser`
- [ ] Unexport `SetNativePlatform`, `NativePlatformBackend`
- [ ] Unexport `IMEComposing`, `IMECompText`, `IMECompCursor`, `IMECompSelLen`, `IMESetRect`
- [ ] Unexport `App`, `PlatformID`, `Ctx`
- [ ] Unexport `MouseCursorState`, `InputCursorOn`
- [ ] Unexport `RenderersCount`, `SetMouseCursor` (keep specific cursor helpers)

### 3b. Color Methods — Keep Utility Subset
- [ ] Keep: `RGBA8`, `ToCSSString`, `ToHexString`, `WithOpacity`, `IsSet`, `String`, `ToHSV`
- [ ] Unexport: `BGRA8`, `ABGR8`, `Add`, `Sub`, `Over`, `Eq`

### 3c. *Shape Methods — Unexport Most
- [ ] Keep: `PointInShape`
- [ ] Unexport: `HasRtfLayout`, `HasEvents`, `PaddingLeft`, `PaddingTop`, `PaddingWidth`, `PaddingHeight`

---

## Phase 4: Const and Var Cleanup

### 4a. Shader Constants → `gui/shader/`
- [ ] Create `gui/shader/` package
- [ ] Move all `Vs*`, `Fs*` GLSL and Metal constants (~40 consts)

### 4b. Theme Vars — Consolidate
- [ ] Reduce 8 theme vars → 3: `ThemeDark`, `ThemeLight`, `ThemeBlue`
- [ ] Add `Theme.WithPadding(bool)`, `Theme.WithBorders(bool)` builder methods
- [ ] Unexport `ThemeDarkCfg`, `ThemeDarkBorderedCfg`, `ThemeDarkNoPaddingCfg`, `ThemeLightCfg`, etc.

### 4c. Remove Redundant Exports
- [ ] Unexport/remove `ColorTransparent` (use `RGBA(0,0,0,0)`)
- [ ] Unexport `NoBorder`, `NoSpacing`, `NoRadius`, `NoPadding`
- [ ] Unexport `PaddingOne`, `PaddingTwo`, `PaddingThree`, `PaddingTwoThree`, `PaddingTwoFour`, `PaddingTwoFive`
- [ ] Unexport `Commit` (keep `Version`)
- [ ] Unexport `DefaultIconPNG`, `IconFontData`, `IconLookup`, `AppFontPaths`
- [ ] Unexport `ValidImageExtensions`, `BaseFontName`, `IconFontName`
- [ ] Unexport `MenuSeparatorID`, `MenuSubtitleID`
- [ ] Unexport `ScrollDeltaHome`, `ToastPersistent`

### 4d. Keep Exported
- [ ] ~120 `Icon*` consts — keep
- [ ] ~80 `Key*` consts — keep
- [ ] Color predefined vars (`Black`, `White`, `Red`, etc.) — keep
- [ ] Style defaults (`DefaultTextStyle`, `DefaultButtonStyle`, etc.) — keep

---

## Phase 5: Consolidate Widget Constructors

### 5a. Remove Aliases
- [ ] Remove `Split()` → use `Splitter()` only
- [ ] Remove `Tabs()` → use `TabControl()` only
- [ ] Remove `Checkbox()` → use `Toggle()` only
- [ ] Keep `Canvas()` — semantically meaningful
- [ ] Keep `Circle()` — semantically meaningful

### 5b. Less-Common Widgets
- Decision: keep at top level (`Breadcrumb`, `CommandPalette`, `DatePickerRoller`, `Pulsar`, `ThemePicker`, `Sidebar`). Examples under-sample their real usage.

---

## Phase 6: Native/Dialog Types — Keep As-Is

- [ ] `NativePlatform`, `NativeDialogs`, `NativePrinter`, etc. — keep (backend-facing interfaces)
- [ ] `NoopNativePlatform` — keep (test backend)
- [ ] Review for interface consolidation opportunities (follow-up)

---

## Phase 7: Utility Funcs → `guiutil/` Package

- [ ] Create `guiutil/` package
- [ ] Move `Lerp`, `PointInRectangle`
- [ ] Move `PackRGB`, `PackAlphaPos`
- [ ] Move `SampleGradientStopColor`, `NormalizeGradientStopsInto`, `GradientDir`
- [ ] Move `PreserveAlignFractions`, `SamplePathAt`
- [ ] Move `ShaderHash`, `BuildGLSLFragment`
- [ ] Move `ReorderIndices`, `ColorFromHexString`
- Keep in `gui/`: `ColorFromString`, `ColorFromHSV`, `ColorFromHSVA`, `HueColor`, `Hex`, `RGB`, `RGBA`, `PadAll`, `PadTBLR`, `NewPadding`, `CubicBezier`, all `Ease*` funcs

---

## Phase 8: Final *Window Method Review

After all prior phases, verify the remaining *Window exported methods are genuinely needed by app authors.

- [ ] Final audit: is every remaining method used in ≥1 example or sibling project?
- [ ] Document which methods are "advanced" vs "everyday"
- [ ] Target: ~50 exported methods on *Window

---

## Estimated Impact

| Phase | Symbols Removed/Moved | Breaking? |
|-------|----------------------|-----------|
| 1. Unexport internals | ~200 → unexported | No |
| 2. Sub-packages | ~300 → moved | **Yes** |
| 3. Method slim-down | ~60 → unexported | No |
| 4. Const/var cleanup | ~50 → unexported/moved | Minor |
| 5. Widget consolidation | ~3 → removed | **Yes** |
| 6. Native types | ~0 | No |
| 7. Utility funcs | ~15 → moved | **Yes** |
| 8. Window method reorg | ~50 → unexported | No |

**Net**: ~680 symbols removed. Target: ~1,480 remaining (down from 2,164).

---

## Documentation Deliverables

### Phase 0
- [ ] Rewrite `gui/doc.go` — slimmed surface overview
- [ ] Update `docs/architecture.md` — sub-package layout
- [ ] Create `doc.go` for each new sub-package

### Phase 2
- [ ] `CHANGELOG.md` — v0.25.0 migration guide with before/after imports
- [ ] `docs/ROADMAP.md` — add 1.0 API stabilization milestone
- [ ] All 25 example `README.md` files — update imports
- [ ] All 25 example `main.go` files — update imports

### Phases 3-8
- [ ] `CHANGELOG.md` entries for each phase
- [ ] `README.md` — verify against final API surface
- [ ] `docs/commands.md` — update if Command/Shortcut types change

### Final 1.0
- [ ] Create `docs/migration-v1.md` — every breaking change with before/after snippets
- [ ] `README.md` "Getting Started" verified against final API
- [ ] `go doc gui` output is scannable (~200 lines vs current 790)

---

## Migration Strategy

1. **Phase 1 first** — unexport internals. Zero breakage. Immediate win.
2. **Phase 4** — const/var cleanup. Minor breakage, easy fixes.
3. **Phases 2+7 together** — sub-package moves. Release as v0.25.0 with deprecation shims (type aliases with `// Deprecated` comments).
4. **Phase 5** — widget alias removal in v0.26.0.
5. **Phases 3+8** — method cleanup, mostly non-breaking.
6. **Cut 1.0** when surface is stable.

## Sibling Project Updates

Each breaking phase must include PRs to sibling projects. These projects import go-gui and will break on sub-package moves and alias removals. Update them in lockstep — not as an afterthought.

- [ ] **go-charts** (`~/Documents/github/go-charts`) — heavy DataGrid user. Update imports for `gui/datagrid/`, `gui/animation/`, `guiutil/`
- [ ] **go-edit** (`~/Documents/github/go-edit`) — uses DrawContext, Shortcut, NativeMenuItemCfg. Update imports for `gui/animation/`, `guiutil/`, `gui/shader/`
- [ ] **go-kite** (`~/Documents/github/go-kite`) — light user. Update imports for `gui/animation/`, `guiutil/`

### Per-Phase Sibling Checklist

#### Phase 1 (Unexport)
- [ ] go-charts: verify builds (no breakage expected)
- [ ] go-edit: verify builds (no breakage expected)
- [ ] go-kite: verify builds (no breakage expected)

#### Phase 2 (Sub-Packages)
- [ ] go-charts: update imports, verify builds
- [ ] go-edit: update imports, verify builds
- [ ] go-kite: update imports, verify builds

#### Phase 4 (Const/Var Cleanup)
- [ ] go-charts: update theme var references (`ThemeDarkCfg` → `ThemeDark`), verify builds
- [ ] go-edit: update theme var references, verify builds
- [ ] go-kite: update theme var references, verify builds

#### Phase 5 (Widget Alias Removal)
- [ ] go-charts: replace `Split` → `Splitter`, verify builds
- [ ] go-edit: replace aliases, verify builds
- [ ] go-kite: replace aliases, verify builds

#### Phase 7 (Utility Funcs)
- [ ] go-charts: update `guiutil/` imports, verify builds
- [ ] go-edit: update `guiutil/` imports, verify builds
- [ ] go-kite: update `guiutil/` imports, verify builds

#### Phases 3+8 (Method Cleanup)
- [ ] go-charts: verify builds (no breakage expected)
- [ ] go-edit: verify builds (no breakage expected)
- [ ] go-kite: verify builds (no breakage expected)

---

## Verification

### Code
- [ ] `go build ./...` passes after each phase
- [ ] `golangci-lint run ./...` clean
- [ ] All 25 examples build and run
- [ ] Sibling projects (go-charts, go-edit, go-kite) build and tests pass after each phase
- [ ] `go doc gui` output ~200 lines (vs current 790)
- [ ] `go doc` for each new sub-package produces sensible output
- [ ] `go vet ./...` clean (requiredid analyzer still works)

### Documentation
- [ ] `README.md` "Getting Started" example runs verbatim
- [ ] `CHANGELOG.md` covers every breaking change with before/after
- [ ] `docs/migration-v1.md` complete and reviewed
- [ ] Every example `README.md` has correct imports
- [ ] `docs/architecture.md` reflects new package layout
- [ ] Each sub-package has a `doc.go` with overview
