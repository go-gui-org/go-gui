package gui

import "fmt"

const splitterDefaultRatio = float32(0.5)

// SplitterOrientation controls how panes are arranged.
type splitterOrientation uint8

// SplitterOrientation values.
const (
	SplitterHorizontal splitterOrientation = iota
	SplitterVertical
)

var splitterOrientationText = [2][]byte{
	SplitterHorizontal: []byte("horizontal"),
	SplitterVertical:   []byte("vertical"),
}

// MarshalText implements encoding.TextMarshaler. // exportaudit:keep — stdlib interface method
func (o splitterOrientation) MarshalText() ([]byte, error) {
	if int(o) < len(splitterOrientationText) {
		return splitterOrientationText[o], nil
	}
	return nil, fmt.Errorf("unknown SplitterOrientation %d", o)
}

// UnmarshalText implements encoding.TextUnmarshaler. // exportaudit:keep — stdlib interface method
func (o *splitterOrientation) UnmarshalText(text []byte) error {
	switch string(text) {
	case "horizontal":
		*o = SplitterHorizontal
	case "vertical":
		*o = SplitterVertical
	default:
		return fmt.Errorf("unknown SplitterOrientation %q", text)
	}
	return nil
}

// SplitterCollapsed tracks which pane is collapsed, if any.
type SplitterCollapsed uint8

// SplitterCollapsed values.
const (
	splitterCollapseNone SplitterCollapsed = iota
	SplitterCollapseFirst
	SplitterCollapseSecond
)

var splitterCollapsedText = [3][]byte{
	splitterCollapseNone:   []byte("none"),
	SplitterCollapseFirst:  []byte("first"),
	SplitterCollapseSecond: []byte("second"),
}

// MarshalText implements encoding.TextMarshaler. // exportaudit:keep — stdlib interface method
func (c SplitterCollapsed) MarshalText() ([]byte, error) {
	if int(c) < len(splitterCollapsedText) {
		return splitterCollapsedText[c], nil
	}
	return nil, fmt.Errorf("unknown SplitterCollapsed %d", c)
}

// UnmarshalText implements encoding.TextUnmarshaler. // exportaudit:keep — stdlib interface method
func (c *SplitterCollapsed) UnmarshalText(text []byte) error {
	switch string(text) {
	case "none":
		*c = splitterCollapseNone
	case "first":
		*c = SplitterCollapseFirst
	case "second":
		*c = SplitterCollapseSecond
	default:
		return fmt.Errorf("unknown SplitterCollapsed %q", text)
	}
	return nil
}

// SplitterState is an app-owned persistence model.
type SplitterState struct {
	Ratio     float32           `json:"ratio"`
	Collapsed SplitterCollapsed `json:"collapsed"`
}

// SplitterStateNormalize normalizes state before persisting.
// Replaces NaN/Inf with the default ratio, clamps to [0,1],
// and resets invalid Collapsed values.
func SplitterStateNormalize(state SplitterState) SplitterState {
	r := state.Ratio
	if !f32IsFinite(r) {
		r = splitterDefaultRatio
	}
	c := state.Collapsed
	if c > SplitterCollapseSecond {
		c = splitterCollapseNone
	}
	return SplitterState{
		Ratio:     splitterNormalizeRatio(r),
		Collapsed: c,
	}
}

// SplitterPaneCfg configures one pane of a splitter.
type SplitterPaneCfg struct {
	Content []View
	MinSize float32
	MaxSize float32
	// CollapsedSize is the pane's width/height when collapsed. Zero
	// takes a small default.
	// exportaudit:keep — caller-facing config (issue #372)
	CollapsedSize float32
	// Collapsible enables the collapse button and collapse behavior
	// for this pane.
	// exportaudit:keep — caller-facing config (issue #372)
	Collapsible bool
}

// splitterPaneCore holds pane fields needed by callbacks
// (excludes Content to avoid GC false retention).
type splitterPaneCore struct {
	minSize       float32
	maxSize       float32
	collapsible   bool
	collapsedSize float32
}

// SplitterCfg configures a splitter component.
type SplitterCfg struct {
	OnChange func(float32, SplitterCollapsed, EventCtx)
	ID       string

	A11YCfg
	First      SplitterPaneCfg
	Second     SplitterPaneCfg
	Ratio      Opt[float32]
	HandleSize Opt[float32]
	// DragStep/DragStepLarge are the keyboard drag increments
	// (normal and with a modifier). Unset takes the theme defaults.
	// exportaudit:keep — caller-facing config (issue #372)
	DragStep Opt[float32]
	// exportaudit:keep — caller-facing config (issue #372)
	DragStepLarge Opt[float32]
	SizeBorder    Opt[float32]
	Radius        Opt[float32]
	// RadiusBorder rounds the handle. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusBorder Opt[float32]
	Focusable    bool
	// ColorHandle/ColorHandleHover/ColorHandleActive/
	// ColorHandleBorder/ColorGrip/ColorButton/ColorButtonHover/
	// ColorButtonActive/ColorButtonIcon theme the handle and its
	// collapse buttons. Unset takes the theme defaults.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorHandle Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorHandleHover Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorHandleActive Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorHandleBorder Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorGrip Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorButton Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorButtonHover Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorButtonActive Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorButtonIcon     Color
	Sizing              Sizing
	Orientation         splitterOrientation
	Collapsed           SplitterCollapsed
	ShowCollapseButtons bool

	// Sound overrides the theme's toggle cues for the collapse
	// toggle — Theme.Sounds.ToggleOn when a pane collapses,
	// ToggleOff when it expands again. Dragging the handle is never
	// a cue: it is continuous, not an activation (issue #468).
	// exportaudit:keep — caller-facing config (issue #468)
	Sound SoundCue

	// SoundDisabled suppresses the splitter's sound regardless of
	// the theme and of Sound above.
	// exportaudit:keep — caller-facing config (issue #468)
	SoundDisabled bool

	Disabled  bool
	Invisible bool
}

// splitterCore holds callback-relevant fields.
type splitterCore struct {
	onChange      func(float32, SplitterCollapsed, EventCtx)
	id            string
	first         splitterPaneCore
	second        splitterPaneCore
	focusID       string
	ratio         float32
	handleSize    float32
	dragStep      float32
	dragStepLarge float32
	orientation   splitterOrientation
	collapsed     SplitterCollapsed
	disabled      bool
	// Handle colors resolved from the (defaulted) cfg. The drag state
	// itself lives in window state (nsSplitterDrag), not on this core —
	// the view function rebuilds the core every frame, so a field here
	// would not survive a drag.
	colorHandle       Color
	colorHandleHover  Color
	colorHandleActive Color
	// Collapse cues, resolved at generation time. The handle's key
	// path reaches onChange directly, so dispatch never sees the
	// toggle (issue #468).
	soundCollapse SoundCue
	soundExpand   SoundCue
}

type splitterComputed struct {
	firstMain  float32
	secondMain float32
	handleMain float32
	ratio      float32
	collapsed  SplitterCollapsed
}

func newSplitterCore(cfg *SplitterCfg) *splitterCore {
	s := &defaultSplitterStyle
	return &splitterCore{
		id:          cfg.ID,
		focusID:     cfg.ID,
		orientation: cfg.Orientation,
		ratio:       cfg.Ratio.Get(splitterDefaultRatio),
		collapsed:   cfg.Collapsed,
		onChange:    cfg.OnChange,
		first: splitterPaneCore{
			minSize:       cfg.First.MinSize,
			maxSize:       cfg.First.MaxSize,
			collapsible:   cfg.First.Collapsible,
			collapsedSize: cfg.First.CollapsedSize,
		},
		second: splitterPaneCore{
			minSize:       cfg.Second.MinSize,
			maxSize:       cfg.Second.MaxSize,
			collapsible:   cfg.Second.Collapsible,
			collapsedSize: cfg.Second.CollapsedSize,
		},
		handleSize:        cfg.HandleSize.Get(s.HandleSize),
		dragStep:          cfg.DragStep.Get(s.dragStep),
		dragStepLarge:     cfg.DragStepLarge.Get(s.dragStepLarge),
		disabled:          cfg.Disabled,
		colorHandle:       cfg.ColorHandle,
		colorHandleHover:  cfg.ColorHandleHover,
		colorHandleActive: cfg.ColorHandleActive,
		soundCollapse: resolveSoundCue(
			guiTheme.Sounds.ToggleOn, cfg.Sound, cfg.SoundDisabled),
		soundExpand: resolveSoundCue(
			guiTheme.Sounds.ToggleOff, cfg.Sound, cfg.SoundDisabled),
	}
}

func applySplitterDefaults(cfg *SplitterCfg) {
	s := &defaultSplitterStyle
	cfg.Sizing = cfg.Sizing.Or(FillFill)
	// SplitterStyle's SizeBorder/Radius/radiusBorder were built and
	// then never read: unset, the handle and buttons fell through to
	// the generic container and button defaults instead. Under the
	// stock theme that only cost the corner radius (radiusMedium 5.5
	// rather than the splitter's radiusSmall 3.5), since both border
	// widths happen to be sizeBorderDef — but for a custom theme the
	// splitter's own values were dead. Seed them here so the style is
	// what renders.
	if !cfg.SizeBorder.IsSet() {
		cfg.SizeBorder = SomeF(s.SizeBorder)
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = SomeF(s.Radius)
	}
	if !cfg.RadiusBorder.IsSet() {
		cfg.RadiusBorder = SomeF(s.radiusBorder)
	}
	if !cfg.ColorHandle.IsSet() {
		cfg.ColorHandle = s.colorHandle
	}
	if !cfg.ColorHandleHover.IsSet() {
		cfg.ColorHandleHover = s.colorHandleHover
	}
	if !cfg.ColorHandleActive.IsSet() {
		cfg.ColorHandleActive = s.colorHandleActive
	}
	if !cfg.ColorHandleBorder.IsSet() {
		cfg.ColorHandleBorder = s.colorHandleBorder
	}
	if !cfg.ColorGrip.IsSet() {
		cfg.ColorGrip = s.colorGrip
	}
	if !cfg.ColorButton.IsSet() {
		cfg.ColorButton = s.colorButton
	}
	if !cfg.ColorButtonHover.IsSet() {
		cfg.ColorButtonHover = s.colorButtonHover
	}
	if !cfg.ColorButtonActive.IsSet() {
		cfg.ColorButtonActive = s.colorButtonActive
	}
	if !cfg.ColorButtonIcon.IsSet() {
		cfg.ColorButtonIcon = s.colorButtonIcon
	}
}

// splitterView implements View for the splitter. The splitter is a
// struct view rather than a plain factory because its inner IDs must be
// composed from the widget's *effective* ID, which only exists once a
// Window is in hand (see docs/specs/widget-id-per-scope-uniqueness.md).
// Composing them in the factory produced absolute strings that
// resolveShapeIDs never joins, so every part stayed window-global under
// an ID-bearing ancestor (issue #264).
type splitterView struct {
	core *splitterCore
	cfg  SplitterCfg
}

// Splitter creates a two-pane splitter with drag/keyboard/collapse.
func Splitter(cfg SplitterCfg) View {
	applySplitterDefaults(&cfg)
	return &splitterView{cfg: cfg, core: newSplitterCore(&cfg)}
}

func (sv *splitterView) GenerateLayout(w *Window) Layout {
	cfg := &sv.cfg
	core := sv.core
	// Every part hangs off the splitter's effective ID, so two splitters
	// written with the same leaf under different ID-bearing panels do not
	// collide. With no ID-bearing ancestor, id == cfg.ID and the parts
	// keep their historical spellings ("sp:handle", "sp:pane:first").
	// The root shape carries the plain leaf; the framework joins it to
	// the same string.
	id := w.EffID(cfg.ID)

	return generateViewLayout(Canvas(ContainerCfg{
		ID:        cfg.ID,
		Focusable: cfg.Focusable,
		A11YRole:  AccessRoleSplitter,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
			A11YDescription: cfg.A11YDescription,
		},
		Sizing:    cfg.Sizing,
		Padding:   NoPadding,
		Clip:      true,
		Disabled:  cfg.Disabled,
		Invisible: cfg.Invisible,
		OnKeyDown: func(ctx EventCtx) {
			splitterOnKeydown(core, ctx.Event, ctx.Window)
		},
		AmendLayout: func(ctx EventCtx) {
			splitterAmendLayout(core, ctx.Layout, ctx.Window)
		},
		Content: []View{
			splitterPane(ScopeID(id, "pane", "first"), cfg.First.Content),
			splitterHandleView(cfg, core, id),
			splitterPane(ScopeID(id, "pane", "second"), cfg.Second.Content),
		},
	}), w)
}

func splitterPane(id string, content []View) View {
	return Column(ContainerCfg{
		ID:         id,
		Sizing:     FixedFixed,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Clip:       true,
		Content:    content,
	})
}

func splitterOnDragMove(core *splitterCore, e *Event, w *Window) {
	if core.disabled {
		return
	}
	ly, ok := w.layout.FindByID(core.id)
	if !ok {
		return
	}
	mainSz := splitterMainSize(ly, core.orientation)
	handle := splitterHandleSizeFromLayout(ly, core.orientation,
		core.handleSize)
	available := f32Max(0, mainSz-handle)
	if available <= 0 {
		return
	}

	var cursorMain float32
	if core.orientation == SplitterHorizontal {
		cursorMain = e.MouseX - ly.Shape.X - (handle / 2)
	} else {
		cursorMain = e.MouseY - ly.Shape.Y - (handle / 2)
	}
	ratio := splitterClampRatio(core, available, cursorMain/available)
	splitterSetCursor(core.orientation, w)
	splitterEmitChange(core, ratio, splitterCollapseNone, e, w)
}

// --- AmendLayout ---

func splitterAmendLayout(core *splitterCore, layout *Layout, w *Window) {
	if len(layout.Children) < 3 {
		return
	}
	// The core is built in the factory and holds the leaf cfg.ID; only
	// the resolve pass (via the shape's idKey) knows the effective path,
	// so Amend — which runs on the splitter's own shape, every frame,
	// before any handler that looks the splitter up (FindByID) or moves
	// focus to it — is where the leaf becomes the identity.
	core.id = layout.Shape.idKey()
	if core.focusID != "" {
		core.focusID = core.id
	}

	mainSz := splitterMainSize(layout, core.orientation)
	computed := splitterCompute(core, mainSz)

	if core.orientation == SplitterHorizontal {
		x := layout.Shape.X
		y := layout.Shape.Y
		h := layout.Shape.Height
		splitterLayoutChild(&layout.Children[0], x, y,
			computed.firstMain, h, w)
		splitterLayoutChild(&layout.Children[1],
			x+computed.firstMain, y, computed.handleMain, h, w)
		splitterLayoutChild(&layout.Children[2],
			x+computed.firstMain+computed.handleMain, y,
			computed.secondMain, h, w)
	} else {
		x := layout.Shape.X
		y := layout.Shape.Y
		wid := layout.Shape.Width
		splitterLayoutChild(&layout.Children[0], x, y,
			wid, computed.firstMain, w)
		splitterLayoutChild(&layout.Children[1], x,
			y+computed.firstMain, wid, computed.handleMain, w)
		splitterLayoutChild(&layout.Children[2], x,
			y+computed.firstMain+computed.handleMain,
			wid, computed.secondMain, w)
	}

	// The handle's active (button-held) color is painted here, in the
	// last pass that can write the shape's color before the render:
	// layoutHover bails while the mouse is locked (a splitter drag runs
	// under MouseLock) and the lock's MouseMove handler paints nothing,
	// so a click-time paint alone would be overwritten by the next
	// frame's regeneration of the handle (which reseeds the base
	// color). The pressed flag is keyed by the splitter's effective ID
	// — the same key splitterOnHandleClick writes — and cleared on
	// MouseUp and by the lock's Cancel hook (capture revoked without a
	// release, issue #237). Guarding on the window's actual lock state
	// makes a stale flag unable to pin the handle active even if some
	// future path clears the lock without either.
	ps := StateMapRead[string, bool](w, nsSplitterDrag)
	if ps != nil && w.mouseIsLocked() {
		if pressed, ok := ps.Get(layout.Shape.idKey()); ok && pressed {
			layout.Children[1].Shape.Color = core.colorHandleActive
		}
	}
}

func splitterLayoutChild(
	child *Layout,
	x, y, width, height float32,
	w *Window,
) {
	splitterResetPositions(child, true, axisNone, 0, 0)
	child.Shape.Sizing = FixedFixed
	splitterPinShapeSize(child.Shape, width, height)
	child.Shape.X = 0
	child.Shape.Y = 0

	layoutWidths(child)
	layoutFillWidths(child, &w.scratch)
	layoutWrapText(child, w)
	layoutHeights(child)
	layoutFillHeights(child, &w.scratch)

	// Re-apply the pin after the fit passes. The fit passes recompute
	// the pane's size from its content's minimums, which grows a
	// pinned-0 (collapsed) pane back into a sliver that draws underneath
	// the handle (issue #263). The re-assert is what the downstream
	// scroll, position, amend and render passes see, so a collapsed pane
	// reaches exactly its computed size. For panes pinned >0 it is a
	// no-op — those sizes already survived the fit passes — and it makes
	// the sizing contract structural: pane sizes come from splitterCompute
	// (ratio/min/max), never from content, which is clipped (Clip: true)
	// when larger.
	splitterPinShapeSize(child.Shape, width, height)

	layoutAdjustScrollOffsets(child, w)
	layoutPositions(child, x, y, w)
	layoutAmend(child, w)
}

// splitterPinShapeSize pins a shape to the exact size splitterCompute
// decided. Applied once before the fit passes (so the fixed sizing is
// seeded) and re-applied after them; see splitterLayoutChild for why.
func splitterPinShapeSize(shape *Shape, width, height float32) {
	shape.Width = f32Max(0, width)
	shape.Height = f32Max(0, height)
	shape.MinWidth = shape.Width
	shape.MaxWidth = shape.Width
	shape.MinHeight = shape.Height
	shape.MaxHeight = shape.Height
}

func splitterResetPositions(layout *Layout, isRoot bool,
	parentAxis Axis, parentOldX, parentOldY float32) {
	oldX := layout.Shape.X
	oldY := layout.Shape.Y
	if isRoot {
		layout.Shape.X = 0
		layout.Shape.Y = 0
	} else if parentAxis == axisNone {
		layout.Shape.X = oldX - parentOldX
		layout.Shape.Y = oldY - parentOldY
	} else {
		layout.Shape.X = 0
		layout.Shape.Y = 0
	}
	for i := range layout.Children {
		splitterResetPositions(&layout.Children[i], false,
			layout.Shape.Axis, oldX, oldY)
	}
}

// --- Pure computation helpers ---

func splitterCompute(core *splitterCore, mainSize float32) splitterComputed {
	handle := splitterHandleSize(core.handleSize, mainSize)
	available := f32Max(0, mainSize-handle)
	ratio := splitterClampRatio(core, available, core.ratio)
	collapsed := splitterEffectiveCollapsed(core, core.collapsed)

	var first, second float32
	switch collapsed {
	case SplitterCollapseFirst:
		first, second = splitterCollapsedFirst(core, available)
	case SplitterCollapseSecond:
		first, second = splitterCollapsedSecond(core, available)
	default:
		first = splitterClampFirstSize(core, available, ratio*available)
		second = f32Max(0, available-first)
		if available > 0 {
			ratio = first / available
		} else {
			ratio = splitterDefaultRatio
		}
	}
	return splitterComputed{
		firstMain:  first,
		secondMain: second,
		handleMain: handle,
		ratio:      ratio,
		collapsed:  collapsed,
	}
}

func splitterCollapsedFirst(core *splitterCore, available float32) (float32, float32) {
	firstTarget := f32Clamp(core.first.collapsedSize, 0, available)
	secondMin := f32Max(0, core.second.minSize)
	secondMax := splitterLimitMax(core.second.maxSize, available)
	secondMin = min(secondMin, secondMax)
	second := f32Clamp(available-firstTarget, secondMin, secondMax)
	first := f32Max(0, available-second)
	first = f32Min(first, splitterLimitMax(core.first.maxSize, available))
	second = f32Max(0, available-first)
	return first, second
}

func splitterCollapsedSecond(core *splitterCore, available float32) (float32, float32) {
	secondTarget := f32Clamp(core.second.collapsedSize, 0, available)
	firstMin := f32Max(0, core.first.minSize)
	firstMax := splitterLimitMax(core.first.maxSize, available)
	firstMin = min(firstMin, firstMax)
	first := f32Clamp(available-secondTarget, firstMin, firstMax)
	second := f32Max(0, available-first)
	second = f32Min(second, splitterLimitMax(core.second.maxSize, available))
	return f32Max(0, available-second), f32Max(0, second)
}

func splitterMainSize(layout *Layout, orientation splitterOrientation) float32 {
	if orientation == SplitterHorizontal {
		return layout.Shape.Width
	}
	return layout.Shape.Height
}

func splitterHandleSizeFromLayout(
	layout *Layout,
	orientation splitterOrientation,
	fallback float32,
) float32 {
	if len(layout.Children) > 1 {
		handle := layout.Children[1]
		if orientation == SplitterHorizontal {
			return handle.Shape.Width
		}
		return handle.Shape.Height
	}
	return fallback
}

func splitterHandleSize(handleSize, mainSize float32) float32 {
	size := f32Max(1, handleSize)
	if mainSize <= 0 {
		return size
	}
	return f32Min(size, mainSize)
}

func splitterClampRatio(core *splitterCore, available, ratio float32) float32 {
	if available <= 0 {
		return splitterDefaultRatio
	}
	target := splitterNormalizeRatio(ratio) * available
	first := splitterClampFirstSize(core, available, target)
	return first / available
}

func splitterClampFirstSize(core *splitterCore, available, target float32) float32 {
	lower, upper := splitterBounds(core, available)
	lower = f32Clamp(lower, 0, available)
	upper = f32Clamp(upper, 0, available)
	if lower <= upper {
		return f32Clamp(target, lower, upper)
	}
	return f32Clamp(target, upper, lower)
}

func splitterBounds(core *splitterCore, available float32) (float32, float32) {
	firstMin := f32Max(0, core.first.minSize)
	firstMax := splitterLimitMax(core.first.maxSize, available)
	firstMin = min(firstMin, firstMax)
	secondMin := f32Max(0, core.second.minSize)
	secondMax := splitterLimitMax(core.second.maxSize, available)
	secondMin = min(secondMin, secondMax)
	lower := f32Max(firstMin, available-secondMax)
	upper := f32Min(firstMax, available-secondMin)
	return lower, upper
}

func splitterLimitMax(value, available float32) float32 {
	if value > 0 {
		return f32Clamp(value, 0, available)
	}
	return available
}

func splitterNormalizeRatio(ratio float32) float32 {
	return f32Clamp(ratio, 0, 1)
}

func splitterCurrentRatio(core *splitterCore, w *Window) float32 {
	ly, ok := w.layout.FindByID(core.id)
	if !ok {
		return splitterNormalizeRatio(core.ratio)
	}
	mainSz := splitterMainSize(ly, core.orientation)
	handle := splitterHandleSizeFromLayout(ly, core.orientation,
		core.handleSize)
	return splitterClampRatio(core, f32Max(0, mainSz-handle), core.ratio)
}

func splitterToggleTarget(core *splitterCore, current SplitterCollapsed) SplitterCollapsed {
	active := splitterEffectiveCollapsed(core, current)
	if active != splitterCollapseNone {
		return active
	}
	if core.first.collapsible {
		return SplitterCollapseFirst
	}
	if core.second.collapsible {
		return SplitterCollapseSecond
	}
	return splitterCollapseNone
}

func splitterArrowStep(core *splitterCore, orient splitterOrientation,
	sign float32, mod Modifier, available, ratio float32,
) (float32, bool) {
	if core.orientation != orient {
		return ratio, false
	}
	if mod != ModNone && mod != ModShift {
		return ratio, false
	}
	step := core.dragStep
	if mod == ModShift {
		step = core.dragStepLarge
	}
	return splitterClampRatio(core, available,
		ratio+sign*splitterStep(step)), true
}

func splitterToggleCollapse(core *splitterCore,
	current SplitterCollapsed,
) (SplitterCollapsed, bool) {
	target := splitterToggleTarget(core, current)
	if target == splitterCollapseNone {
		return current, false
	}
	if current == target {
		return splitterCollapseNone, true
	}
	return target, true
}

// splitterStep returns step, falling back to 0.02 as a safety net.
// applySplitterDefaults normally guarantees a non-zero value from the
// theme, but this guards against direct splitterCore construction in
// tests or internal callers.
func splitterStep(step float32) float32 {
	if step > 0 {
		return step
	}
	return 0.02
}

func splitterEffectiveCollapsed(core *splitterCore, collapsed SplitterCollapsed) SplitterCollapsed {
	switch collapsed {
	case SplitterCollapseFirst:
		if core.first.collapsible {
			return SplitterCollapseFirst
		}
		return splitterCollapseNone
	case SplitterCollapseSecond:
		if core.second.collapsible {
			return SplitterCollapseSecond
		}
		return splitterCollapseNone
	default:
		return splitterCollapseNone
	}
}

func splitterEmitChange(
	core *splitterCore,
	ratio float32, collapsed SplitterCollapsed,
	e *Event, w *Window,
) {
	state := SplitterStateNormalize(SplitterState{
		Ratio:     ratio,
		Collapsed: collapsed,
	})
	if core.onChange != nil {
		core.onChange(state.Ratio, state.Collapsed, EventCtx{nil, e, w})
	}
	splitterFocus(core, w)
	e.IsHandled = true
}

func splitterFocus(core *splitterCore, w *Window) {
	if core.focusID != "" {
		w.SetFocus(core.focusID)
	}
}

func splitterSetCursor(orientation splitterOrientation, w *Window) {
	if orientation == SplitterHorizontal {
		w.SetMouseCursorEW()
	} else {
		w.SetMouseCursorNS()
	}
}
