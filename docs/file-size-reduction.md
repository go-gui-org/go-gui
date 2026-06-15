# File Size Reduction — Implementation Spec

Goal: bring every non-test Go source in `gui/` to ≤800 lines.

## Principles

1. **Pure code moves**. No logic changes, no renames, no reformatting beyond
   what `gofmt` does. Each split commits atomically — one file-in, one
   file-out — so `git blame` survives.
2. **Group by concern**. New files hold a self-contained set of related
   functions/types. Don't split mid-concern just to hit the line target.
3. **No new abstractions**. Don't introduce interfaces or indirection to make
   a split work. If a function accesses unexported helpers in the original
   package, the new file stays in the same package.
4. **No import cycles**. Every split file lands in the same package as the
   original. Zero new import paths.
5. **Build tags preserved**. `gui/backend/web/draw.go` carries
   `//go:build js && wasm`; every split file from it carries the same tag.
6. **Tests added** for any previously untested code in extracted files.
   Follow the pattern established in commit `56b2a56`.

## Current state

From `scripts/large-files.sh` (2026-06-15):

| Lines | File |
|------:|------|
| 933 | gui/svg/css/parse.go |
| 891 | gui/styles_widget.go |
| 883 | gui/view_markdown.go |
| 878 | gui/view_form.go |
| 836 | gui/theme.go |
| 836 | gui/datagrid/view_data_grid_rows.go |
| 828 | gui/view_date_picker.go |
| 824 | gui/backend/web/draw.go |
| 816 | gui/view_rtf.go |
| 809 | gui/drag_reorder.go |
| 806 | gui/svg/tessellate_scanline.go |
| 802 | gui/svg/xml.go |
| 801 | gui/datagrid/view_data_grid_crud.go |
| 801 | gui/datagrid/data_source.go |

14 files over threshold. `gui/fonts.go` (562) is already compliant.

Note: `styles_widget.go` and `theme.go` are listed as "already-addressed" in
ROADMAP.md but remain over 800 lines. This spec addresses them.

## Split plans

Each section: what stays in the original file, what moves to new files,
approximate post-split line counts.

---

### 1. gui/svg/css/parse.go — 933 → ~350 + ~330 + ~250

Three-way split:

**`parse.go`** (~350 lines)
- Constants (`maxRules`, `maxDeclsPerRule`, ...)
- `ParseStylesheet()`, `ParseFull()`, `parseCtx` struct
- Grammar event handlers: `onBeginAtRule()`, `onEndAtRule()`,
  `onBeginRuleset()`, `onDeclaration()`, `onEndRuleset()`
- `skipSkippedMedia()`, `advanceSkippedMedia()`
- `parseDeclaration()`
- `stripLineComments()`
- `vendorPrefixes`, `StripVendorPrefix()`, `joinTokens()`

**`parse_selector.go`** (~330 lines)
- `parseSelectorList()`, `expandIs()`, `splitByComma()`, `trimWS()`
- `parseComplexSelector()`, `combinatorFromDelim()`
- `skipBrackets()`, `skipFunctionArgs()`
- `parseCompound()`, `parseCompoundAt()`, `parseCompoundToken()`,
  `parseCompoundHash()`, `parseCompoundDelim()`, `parseCompoundAttr()`
- `compoundEmpty()`, `compoundIsNonEmpty()`
- `parseAttrSel()`, `parseAttrOp()`, `attrValueText()`

**`parse_pseudo.go`** (~250 lines)
- `parsePseudoClass()`
- `parseNthFormula()`

---

### 2. gui/styles_widget.go — 891 → ~290 + ~310 + ~290

Three-way split by widget category:

**`styles_widget.go`** (~290 lines)
- Basic input styles: `InputStyle`, `ScrollbarStyle`, `RadioStyle`,
  `RadioGroupStyle`, `SwitchStyle`, `ToggleStyle`, `SelectStyle`
- Each with its `Default*()` constructor

**`styles_widget_overlay.go`** (~310 lines)
- Container / overlay styles: `ListBoxStyle`, `TreeStyle`, `DialogStyle`,
  `ToastStyle`, `ToastAnchor` + consts, `TooltipStyle`, `BadgeStyle`,
  `ExpandPanelStyle`
- Each with its `Default*()` constructor

**`styles_widget_control.go`** (~290 lines)
- Control styles: `ProgressBarStyle`, `SliderStyle`, `TabControlStyle`,
  `BreadcrumbStyle`, `SplitterStyle`, `TableStyle`, `ComboboxStyle`,
  `CommandPaletteStyle`, `MenubarStyle`, `DatePickerStyle`,
  `ColorPickerStyle`, `SkeletonStyle`
- `var (...)` block
- Each with its `Default*()` constructor

---

### 3. gui/view_markdown.go — 883 → ~350 + ~530

Two-way split:

**`view_markdown.go`** (~350 lines)
- `MarkdownStyle` struct + `DefaultMarkdownStyle()`
- `MarkdownCfg` struct
- `MathFetcher`, `MermaidFetcher` types
- `SetMarkdownExternalAPIsEnabled()`, `markdownExternalAPIsEnabled`
- Diagram infrastructure: `ensureDiagramCache()`, `nextDiagramRequestID()`,
  `markdownWarnExternalAPIOnce()`, `markdownDiagramErrorView()`,
  `buildMarkdownTableData()` (data prep, not rendering)
- `Markdown()` (the main View function)
- `markdownTriggerMathFetches()`
- `markdownBuildContent()` — the block dispatcher

**`view_markdown_blocks.go`** (~530 lines)
- All block renderers:
  `mdFlushListItems()`, `mdRenderMathBlock()`, `mdRenderCodeBlock()`,
  `mdRenderTable()`, `mdRenderHR()`, `applyMdCtx()`,
  `mdRenderBlockquote()`, `mdRenderImage()`, `mdRenderHeading()`,
  `mdRenderDefTerm()`, `mdRenderDefValue()`, `mdRenderListItem()`,
  `mdRenderParagraph()`
- `renderMdMath()`, `renderMdMermaid()`
- `mdCopyButton()`, `renderMdCode()`

---

### 4. gui/view_form.go — 878 → ~380 + ~500

Two-way split:

**`view_form.go`** (~380 lines)
- Public enum types: `FormValidateOn`, `FormIssueKind`, `FormValidationTrigger`
  + consts
- Public data types: `FormIssue`, `FormFieldSnapshot`, `FormFieldState`,
  `FormSnapshot`, `FormSummaryState`, `FormPendingState`,
  `FormSubmitEvent`, `FormResetEvent`
- Validator types: `FormSyncValidator`, `FormAsyncValidator`
- `FormFieldAdapterCfg`, `FormCfg`
- `formView`, `Form()`, `(*formView).Content()`,
  `(*formView).GenerateLayout()`
- `formFieldRuntime`, `formRuntimeState`
- `formRuntime()`, `formRuntimeRead()`, `formLayoutID()`,
  `formDecodeLayoutID()`

**`view_form_process.go`** (~500 lines)
- Validation: `formShouldValidate()`, `formResolveValidateOn()`,
  `formMergeErrors()`
- Conversion helpers: `formToPublicFieldState()`,
  `formSnapshotFromState()`, `formFieldSnapshot()`,
  `formComputeSummary()`, `formComputePending()`
- Lifecycle: `formApplyCfg()`, `formCleanupStale()`, `formProcessRequests()`
  and all downstream helpers

---

### 5. gui/theme.go — 836 → ~260 + ~570

Two-way split:

**`theme.go`** (~260 lines)
- `var (...)` block
- `Theme` struct
- `ThemeCfg` struct
- `(*Theme).WithPadding()`, `(*Theme).WithBorders()`
- `CurrentTheme()`, `SetTheme()`

**`theme_maker.go`** (~570 lines)
- `ThemeMaker()` — the factory function that populates a `Theme` from
  `ThemeCfg`
- Any private helpers only called by `ThemeMaker`

---

### 6. gui/datagrid/view_data_grid_rows.go — 836 → ~390 + ~450

Two-way split:

**`view_data_grid_rows.go`** (~390 lines)
- Row rendering: `dataGridGroupHeaderRowView()`, `dataGridDetailRowView()`,
  `dataGridRowView()`
- Cell format: `dataGridResolveCellFormat()`
- Selection: `dataGridRowClick()`, `dataGridToggleSelectedRowIDs()`,
  `dataGridSelectionIsSingleRow()`, `dataGridComputeRowSelection()`,
  `dataGridAnchorRowIDEx()`, `dataGridSetAnchor()`, `dataGridRangeIndices()`
- Detail toggle: `dataGridDetailToggleControl()`,
  `dataGridNextDetailExpandedMap()`, `dataGridDetailIndent()`
- Frozen top: `dataGridScrollPadding()`, `dataGridScrollGutter()`,
  `dataGridFrozenTopZone()`, `dataGridFrozenTopViews()`,
  `dataGridFrozenTopIDSet()`, `dataGridSplitFrozenTopIndices()`

**`view_data_grid_edit.go`** (~450 lines)
- Cell editor: `dataGridCellEditorView()`,
  `dataGridMakeEditorOnKeydown()`, `dataGridTrackRowEditClick()`
- Modifier helpers: `hasAnyModifiers()`, `dataGridHasKeyboardModifiers()`
- Editor focus: `dataGridFirstEditableColumnIndex()`,
  `dataGridFirstEditableColumnIndexEx()`, `dataGridCellEditorFocusBaseID()`,
  `dataGridCellEditorFocusID()`, `dataGridEditorFocusIDFromBase()`
- Editing state: `dataGridEditingRowID()`, `dataGridSetEditingRow()`,
  `dataGridClearEditingRow()`
- Editor value parsing: `dataGridEditorBoolValue()`,
  `dataGridParseEditorDate()`

---

### 7. gui/view_date_picker.go — 828 → ~340 + ~490

Two-way split:

**`view_date_picker.go`** (~340 lines)
- Types: `DatePickerWeekdays`, `DatePickerMonths`, `DatePickerWeekdayLen`
  + consts
- `datePickerState`, `DatePickerCfg`, `datePickerView`
- `DatePicker()`, `(*datePickerView).Content()`,
  `(*datePickerView).GenerateLayout()`
- `datePickerGetState()`, `(*Window).DatePickerReset()`
- `datePickerControls()`
- `datePickerOnKeyDown()`
- `datePickerUpdateSelections()`

**`view_date_picker_calendar.go`** (~490 lines)
- Calendar rendering: `datePickerCalendar()`, `datePickerWeekdays()`,
  `datePickerMonth()`, `datePickerAdjacentCell()`
- Year/month picker: `datePickerYearMonthPicker()`,
  `datePickerRollerKeyDown()`
- Navigation: `datePickerNavMonth()`
- Helpers: `datePickerIsSelected()`, `datePickerIsDisabled()`,
  `isSameDay()`, `datePickerViewTime()`, `datePickerWeekdayIndex()`,
  `datePickerWeekdayLabel()`, `formatYearMonth()`

---

### 8. gui/backend/web/draw.go — 824 → ~580 + ~245

Two-way split. **Build tag `//go:build js && wasm` required on both files.**

**`draw.go`** (~580 lines)
- Constants, `colorCacheEntry`, `alphaLUT`, `init()`
- `(*Backend).renderersDraw()` — main dispatch
- Clip stack: `drawClip()`, `beginStencilClip()`, `endStencilClip()`,
  `rebuildClipStack()`
- Simple shapes: `drawRect()`, `drawStrokeRect()`, `drawText()`,
  `drawCircle()`, `drawLine()`, `drawShadow()`, `drawBlur()`,
  `drawGradient()`, `drawGradientBorder()`, `drawImage()`
- Complex shapes: `drawSvg()`, `drawLayout()`, `drawLayoutTransformed()`,
  `drawTextPath()`, `drawRtf()`
- Filter state: `beginFilter()`, `endFilter()`

**`draw_color.go`** (~245 lines)
- Color: `cssColorCached()`, `cssColorBuf()`, `appendUint8()`,
  `appendAlpha()`, `setFillColor()`, `setStrokeColor()`
- Rotation: `beginRotation()`, `endRotation()`
- `fillRoundedRect()`
- Utilities: `isAllowedImageSrc()`, `itoa()`, `uitoa()`, `ftoaGeneral()`

---

### 9. gui/view_rtf.go — 816 → ~430 + ~390

Two-way split:

**`view_rtf.go`** (~430 lines)
- `RtfCfg` struct
- `rtfFlatTextFromRuns()`, `rtfRuneCountFromRuns()`
- `rtfView`, `(*rtfView).Content()`, `rtfSuppressInlineObjectGlyphs()`,
  `(*rtfView).GenerateLayout()`, `RTF()`
- Hit testing: `rtfRunRect()`, `rtfHitTest()`, `rtfFindRunAtIndex()`,
  `rtfMouseMove()`
- Tooltips: `rtfTooltipAnimation()`, `rtfAmendTooltip()`, `rtfRunsKey()`,
  `rtfStyleKey()`, `rtfMathStateKey()`, `rtfTooltipView()`
- Link click: `rtfOnClick()`
- Link context menu: `rtfLinkMenuState`, `rtfLinkMenuIDFocus`,
  `showLinkContextMenu()`, `rtfLinkMenuDismiss()`, `rtfLinkMenuView()`

**`view_rtf_select.go`** (~390 lines)
- Text selection: `rtfSelectAmendLayout()`, `rtfMarkdownAmendLayout()`,
  `rtfSelectOnClick()`, `rtfSelectOnKeyDown()`
- All selection state management helpers

---

### 10. gui/drag_reorder.go — 809 → ~590 + ~220

Two-way split:

**`drag_reorder.go`** (~590 lines)
- Constants, doc comment
- `DragReorderAxis` + consts, `dragReorderState`, `dragReorderIDsMeta`
- State management: `dragReorderGet()`, `dragReorderSet()`,
  `dragReorderClear()`, `dragReorderIDsMetaSet()`,
  `dragReorderIDsMetaGet()`, `dragReorderIDsChanged()`
- `dragReorderStartCfg`, `dragReorderStart()`, `dragReorderMakeLock()`
- `dragReorderOnMouseMove()`, `dragReorderOnMouseUp()`, `dragReorderCancel()`
- `dragReorderAutoScroll()`
- `dragReorderKeyboardMove()`, `dragReorderEscape()`
- `dragReorderIDsSignature()`

**`drag_reorder_view.go`** (~220 lines)
- `dragReorderGhostShadowColor`
- `dragReorderGhostView()`, `dragReorderGapView()`
- `dragReorderCalcIndex()`, `dragReorderCalcIndexFromMids()`,
  `dragReorderItemMidsFromLayouts()`
- `ReorderIndices()`

---

### 11. gui/svg/tessellate_scanline.go — 806 → ~460 + ~350

Two-way split:

**`tessellate_scanline.go`** (~460 lines)
- `tessellatePolylines()`, `scanEdge` struct, constants
- `buildScanEdges()`, `segmentIntersectionY()`, `collectScanYs()`,
  `edgesBoundsScale()`, `xAtY()`, `scanlineTessellate()`
- Ear-clip: `appendTrapezoid()`, `appendNonDegenTri()`, `earClip()`,
  `polygonArea()`, `isEar()`, `pointInTriangle()`

**`tessellate_gradient.go`** (~350 lines)
- `resolveGradient()`, `bboxFromTriangles()`
- `projectOntoGradient()`, `projectAndSpread()`,
  `projectOntoGradientRaw()`, `applySpread()`, `projectOntoRadial()`
- `interpolateGradient()`
- `subdivideGradientTris()`, `subdivideRadialTris()`, `splitRadialTri()`,
  `splitTriAtStops()`

---

### 12. gui/svg/xml.go — 802 → ~340 + ~460

Two-way split. This file was previously split into `xml_text.go` and
`xml_defs.go`. This is a further split of the remaining core.

**`xml.go`** (~340 lines)
- Constants: `maxElementIDLen`, `maxFlatnessTolerance`
- `sanitizeFlatness()`, `clampElementID()`
- `ParseOptions`, `parseSvg()`, `parseSvgWith()`
- `parseSvgFile()`, `loadSvgFile()`, `parseSvgDimensions()`,
  `extractRootSVGOpenTag()`
- `mintSynthID()`, `(*parseState).synthGroupID()`,
  `(*parseState).synthNestedClipID()`, `(*parseState).recordGroupParent()`

**`xml_parse.go`** (~460 lines)
- Element parsers: `parseSvgContent()`, `parseGroupOrLinkElement()`,
  `parseNestedSvgElement()`, `parseShapeElement()`,
  `parseAnimationElement()`, `parseMotionAnimationElement()`,
  `parseTextContentElement()`
- `appendShape()`, `parseAnimateForDispatch()`
- `nodeHasInlineAnimation()`, `scanOpacityAnimTargets()`,
  `parseShapeInlineChildren()`

---

### 13. gui/datagrid/view_data_grid_crud.go — 801 → ~360 + ~440

Two-way split:

**`view_data_grid_crud.go`** (~360 lines)
- `dataGridCrudClearPendingChanges()`, `dataGridCrudHasUnsaved()`
- `dataGridCrudRowDeleteEnabled()`
- `dataGridRowsSignature()`, `dataGridRowsIDSignature()`
- `dataGridCrudResolveCfg()`
- `dataGridCrudToolbarRow()`, `dataGridCrudToolbarHeight()`
- `dataGridCrudDefaultCells()`
- `dataGridCrudAddRow()`
- `dataGridCrudDeleteSelected()`, `dataGridCrudDeleteRows()`
- `dataGridSelectionRemoveIDs()`
- `dataGridCrudCancel()`

**`view_data_grid_crud_save.go`** (~440 lines)
- `dataGridCrudBuildPayload()`
- `dataGridCrudReplaceCreatedRows()`
- `dataGridCrudRemapSelection()`
- `dataGridCrudApplyCellEdit()`
- `dataGridCrudMutationResult`, `dataGridCrudSaveContext`
- `dataGridCrudSave()`, `dataGridCrudExecMutations()`
- `dataGridCrudApplySaveResult()`, `dataGridCrudFinishSave()`
- `dataGridCrudRestoreOnError()`
- `cloneRows()`, `sortedMapKeys()`

---

### 14. gui/datagrid/data_source.go — 801 → ~460 + ~340

Two-way split:

**`data_source.go`** (~460 lines)
- Constants: `dataGridSourceMaxPageLimit`
- Types: `GridDataRequest`, `GridDataResult`, `GridDataCapabilities`,
  `GridMutationRequest`, `GridMutationResult`, `DataGridDataSource`
  interface
- `InMemoryDataSource`, `NewInMemoryDataSource()`
- `(*InMemoryDataSource).Capabilities()`, `FetchData()`, `MutateData()`
- `dataGridSourceInMemoryFetch()`, `dataGridSourceInMemoryMutate()`,
  `dataGridSourceOffsetBounds()`
- `errGridAborted`, `gridAbortCheck()`, `dataGridSourceSleepWithAbort()`
- Cursor ops: `dataGridSourceCursorFromIndex()`,
  `dataGridSourcePrevCursor()`, `dataGridSourceCursorToIndex()`,
  `dataGridSourceCursorToIndexOpt()`, `dataGridSourceIsDecimal()`
- Query: `dataGridSourceApplyQuery()`, `gridFilterLowered`,
  `dataGridSourceRowMatchesQuery()`
- Filter matchers: `gridContainsLower()`, `gridEqualsLower()`,
  `gridStartsWithLower()`, `gridEndsWithLower()`

**`data_source_mutate.go`** (~340 lines)
- `gridQuerySignature()`, `zeroPadHex16()`, `gridHashFilter()`
- `gridMutationApplyResult`
- `dataGridSourceApplyMutation()`, `dataGridSourceApplyCreate()`,
  `dataGridSourceApplyUpdate()`, `dataGridSourceApplyDelete()`
- `gridDeduplicateRowIDs()`, `dataGridSourceNextCreateRowID()`

---

## Naming conventions

Following existing patterns from prior splits (`56b2a56`, `dba2ba9`):

| Pattern | Example |
|---------|---------|
| Extracted by concern | `render_svg_anim_attr.go`, `style_compute.go` |
| Extracted by lifecycle phase | `view_data_grid_cache.go`, `walker_postprocess.go` |
| Extracted by sub-component | `view_input_keys.go`, `view_splitter_handle.go` |

Guidelines:
- Primary file keeps the base name (e.g., `draw.go`, `parse.go`)
- Extracted files get `_<concern>` suffix: `draw_color.go`, `parse_selector.go`
- No `_util` or `_helpers` suffixes — name the concern, not the role
- One concern per extracted file
- Filename must make the split boundary obvious to someone reading the
  directory listing

## Build tag files

Only `gui/backend/web/draw.go` carries a build tag (`//go:build js && wasm`).
The split files (`draw.go`, `draw_color.go`) must both carry the same tag
verbatim on line 1.

## Complexity reduction overlap

Two of the target files carry `//nolint:gocyclo` suppressions:

| File | Line | Context |
|------|------|---------|
| `gui/drag_reorder.go:328` | 328 | `//nolint:gocyclo // drag state machine` |
| `gui/svg/xml.go:330` | 330 | `//nolint:gocyclo // SVG element switch` |

The split should, where natural, extract code such that the nolint directive
stays with the function that needs it, and the extracted file's functions
are individually under the complexity threshold. This directly addresses the
ROADMAP "Complexity Reduction" goal of reducing 20+ `//nolint:gocyclo`
suppressions.

## Execution order

Prioritization by impact (lines over threshold) and risk (build tag,
cross-file dependencies):

1. `gui/styles_widget.go` — lowest risk, style structs have no deps within the file
2. `gui/theme.go` — one function dominates; clean split point
3. `gui/svg/tessellate_scanline.go` — clean concern boundary (scanline vs gradient)
4. `gui/datagrid/view_data_grid_rows.go` — clean boundary (rows vs editor)
5. `gui/datagrid/view_data_grid_crud.go` — clean boundary (crud ops vs save)
6. `gui/datagrid/data_source.go` — clean boundary (source vs mutate)
7. `gui/view_date_picker.go` — clear split (core vs calendar render)
8. `gui/view_rtf.go` — clear split (main vs selection)
9. `gui/drag_reorder.go` — moderate risk (internal deps across split boundary)
10. `gui/view_markdown.go` — moderate risk (block renderers call into main file)
11. `gui/view_form.go` — moderate risk (lifecycle calls runtime)
12. `gui/svg/xml.go` — moderate risk (element parsers call utils in main file)
13. `gui/svg/css/parse.go` — highest lines, three-way split, most internal deps
14. `gui/backend/web/draw.go` — build tag, hard to verify on desktop

## Post-split validation per file

For each split, before committing:

1. `gofmt -w` on all new files
2. `go vet ./...` — no new issues
3. `golangci-lint run ./...` — passes
4. `go test ./...` — all existing tests pass
5. `./scripts/large-files.sh` — verify file is removed from report
6. `git diff --stat` — only additions of new files + deletions from old file;
   zero net logic changes

## ROADMAP.md update

After all splits complete:
- Remove the "File-Size Reduction" bullet from Maintenance → Code Health
- Replace with: "All `gui/` source files now ≤800 lines. Enforced via CI gate."
- Mark the CI gate task ("File-Size Gate" in CI Hardening) as ready for
  implementation now that the backlog is clear.

## Unresolved questions

- `gui/styles_widget.go` and `gui/theme.go` are listed as "already-addressed"
  in ROADMAP.md but both exceed 800 lines. Was a prior split insufficient,
  or did code grow back? Confirm intent before including them.
- `gui/datagrid/view_data_grid_rows.go` (836 lines) is not listed in the
  ROADMAP targets. Should it be added to the maintenance list?
- For `gui/backend/web/draw.go`, the `js && wasm` build tag means desktop
  Go tooling can't compile-verify the split. Should this be gated on a
  wasm-capable CI check?
