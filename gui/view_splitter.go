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
	Content       []View
	MinSize       float32
	MaxSize       float32
	collapsedSize float32
	collapsible   bool
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

	A11YLabel           string
	A11YDescription     string
	First               SplitterPaneCfg
	Second              SplitterPaneCfg
	Ratio               Opt[float32]
	HandleSize          Opt[float32]
	dragStep            Opt[float32]
	dragStepLarge       Opt[float32]
	SizeBorder          Opt[float32]
	Radius              Opt[float32]
	radiusBorder        Opt[float32]
	Focusable           bool
	colorHandle         Color
	colorHandleHover    Color
	colorHandleActive   Color
	colorHandleBorder   Color
	colorGrip           Color
	colorButton         Color
	colorButtonHover    Color
	colorButtonActive   Color
	colorButtonIcon     Color
	Sizing              Sizing
	Orientation         splitterOrientation
	Collapsed           SplitterCollapsed
	ShowCollapseButtons bool
	Disabled            bool
	Invisible           bool
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
			collapsible:   cfg.First.collapsible,
			collapsedSize: cfg.First.collapsedSize,
		},
		second: splitterPaneCore{
			minSize:       cfg.Second.MinSize,
			maxSize:       cfg.Second.MaxSize,
			collapsible:   cfg.Second.collapsible,
			collapsedSize: cfg.Second.collapsedSize,
		},
		handleSize:    cfg.HandleSize.Get(s.HandleSize),
		dragStep:      cfg.dragStep.Get(s.dragStep),
		dragStepLarge: cfg.dragStepLarge.Get(s.dragStepLarge),
		disabled:      cfg.Disabled,
	}
}

func applySplitterDefaults(cfg *SplitterCfg) {
	s := &defaultSplitterStyle
	cfg.Sizing = cfg.Sizing.Or(FillFill)
	if !cfg.colorHandle.IsSet() {
		cfg.colorHandle = s.colorHandle
	}
	if !cfg.colorHandleHover.IsSet() {
		cfg.colorHandleHover = s.colorHandleHover
	}
	if !cfg.colorHandleActive.IsSet() {
		cfg.colorHandleActive = s.colorHandleActive
	}
	if !cfg.colorHandleBorder.IsSet() {
		cfg.colorHandleBorder = s.colorHandleBorder
	}
	if !cfg.colorGrip.IsSet() {
		cfg.colorGrip = s.colorGrip
	}
	if !cfg.colorButton.IsSet() {
		cfg.colorButton = s.colorButton
	}
	if !cfg.colorButtonHover.IsSet() {
		cfg.colorButtonHover = s.colorButtonHover
	}
	if !cfg.colorButtonActive.IsSet() {
		cfg.colorButtonActive = s.colorButtonActive
	}
	if !cfg.colorButtonIcon.IsSet() {
		cfg.colorButtonIcon = s.colorButtonIcon
	}
}

// Splitter creates a two-pane splitter with drag/keyboard/collapse.
func Splitter(cfg SplitterCfg) View {
	applySplitterDefaults(&cfg)
	core := newSplitterCore(&cfg)

	return Canvas(ContainerCfg{
		ID:              cfg.ID,
		Focusable:       cfg.Focusable,
		A11YRole:        AccessRoleSplitter,
		A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
		A11YDescription: cfg.A11YDescription,
		Sizing:          cfg.Sizing,
		Padding:         NoPadding,
		Clip:            true,
		Disabled:        cfg.Disabled,
		Invisible:       cfg.Invisible,
		OnKeyDown: func(ctx EventCtx) {
			splitterOnKeydown(core, ctx.Event, ctx.Window)
		},
		AmendLayout: func(ctx EventCtx) {
			splitterAmendLayout(core, ctx.Layout, ctx.Window)
		},
		Content: []View{
			splitterPane(ScopeID(cfg.ID, "pane", "first"), cfg.First.Content),
			splitterHandleView(&cfg, core),
			splitterPane(ScopeID(cfg.ID, "pane", "second"), cfg.Second.Content),
		},
	})
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
	// Splitter builds its tree with no Window in hand, so the core holds
	// leaf IDs. Amend runs on the splitter's own shape, every frame,
	// before any handler that looks the splitter up (FindByID) or moves
	// focus to it — so this is where the leaves become identities.
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
}

func splitterLayoutChild(
	child *Layout,
	x, y, width, height float32,
	w *Window,
) {
	splitterResetPositions(child, true, axisNone, 0, 0)
	child.Shape.Sizing = FixedFixed
	child.Shape.Width = f32Max(0, width)
	child.Shape.Height = f32Max(0, height)
	child.Shape.MinWidth = child.Shape.Width
	child.Shape.MaxWidth = child.Shape.Width
	child.Shape.MinHeight = child.Shape.Height
	child.Shape.MaxHeight = child.Shape.Height
	child.Shape.X = 0
	child.Shape.Y = 0

	layoutWidths(child)
	layoutFillWidths(child, &w.scratch)
	layoutWrapText(child, w)
	layoutHeights(child)
	layoutFillHeights(child, &w.scratch)
	layoutAdjustScrollOffsets(child, w)
	layoutPositions(child, x, y, w)
	layoutAmend(child, w)
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
