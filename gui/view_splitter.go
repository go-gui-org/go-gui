package gui

import "fmt"

// splitterButtonSuffix maps SplitterCollapsed → button ID suffix.
var splitterButtonSuffix = [3]string{
	":button:0",
	":button:1",
	":button:2",
}

const splitterDefaultRatio = float32(0.5)

// SplitterOrientation controls how panes are arranged.
type SplitterOrientation uint8

// SplitterOrientation values.
const (
	SplitterHorizontal SplitterOrientation = iota
	SplitterVertical
)

var splitterOrientationText = [2][]byte{
	SplitterHorizontal: []byte("horizontal"),
	SplitterVertical:   []byte("vertical"),
}

// MarshalText implements encoding.TextMarshaler.
func (o SplitterOrientation) MarshalText() ([]byte, error) {
	if int(o) < len(splitterOrientationText) {
		return splitterOrientationText[o], nil
	}
	return nil, fmt.Errorf("unknown SplitterOrientation %d", o)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (o *SplitterOrientation) UnmarshalText(text []byte) error {
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
	SplitterCollapseNone SplitterCollapsed = iota
	SplitterCollapseFirst
	SplitterCollapseSecond
)

var splitterCollapsedText = [3][]byte{
	SplitterCollapseNone:   []byte("none"),
	SplitterCollapseFirst:  []byte("first"),
	SplitterCollapseSecond: []byte("second"),
}

// MarshalText implements encoding.TextMarshaler.
func (c SplitterCollapsed) MarshalText() ([]byte, error) {
	if int(c) < len(splitterCollapsedText) {
		return splitterCollapsedText[c], nil
	}
	return nil, fmt.Errorf("unknown SplitterCollapsed %d", c)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (c *SplitterCollapsed) UnmarshalText(text []byte) error {
	switch string(text) {
	case "none":
		*c = SplitterCollapseNone
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
		c = SplitterCollapseNone
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
	CollapsedSize float32
	Collapsible   bool
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
	OnChange func(float32, SplitterCollapsed, *Event, *Window)
	ID       string

	A11YLabel           string
	A11YDescription     string
	First               SplitterPaneCfg
	Second              SplitterPaneCfg
	Ratio               Opt[float32]
	HandleSize          Opt[float32]
	DragStep            Opt[float32]
	DragStepLarge       Opt[float32]
	SizeBorder          Opt[float32]
	Radius              Opt[float32]
	RadiusBorder        Opt[float32]
	IDFocus             uint32
	ColorHandle         Color
	ColorHandleHover    Color
	ColorHandleActive   Color
	ColorHandleBorder   Color
	ColorGrip           Color
	ColorButton         Color
	ColorButtonHover    Color
	ColorButtonActive   Color
	ColorButtonIcon     Color
	Sizing              Sizing
	Orientation         SplitterOrientation
	Collapsed           SplitterCollapsed
	ShowCollapseButtons bool
	Disabled            bool
	Invisible           bool
}

// splitterCore holds callback-relevant fields.
type splitterCore struct {
	onChange      func(float32, SplitterCollapsed, *Event, *Window)
	id            string
	first         splitterPaneCore
	second        splitterPaneCore
	idFocus       uint32
	ratio         float32
	handleSize    float32
	dragStep      float32
	dragStepLarge float32
	orientation   SplitterOrientation
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
	s := &DefaultSplitterStyle
	return &splitterCore{
		id:          cfg.ID,
		idFocus:     cfg.IDFocus,
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
		handleSize:    cfg.HandleSize.Get(s.HandleSize),
		dragStep:      cfg.DragStep.Get(s.DragStep),
		dragStepLarge: cfg.DragStepLarge.Get(s.DragStepLarge),
		disabled:      cfg.Disabled,
	}
}

func applySplitterDefaults(cfg *SplitterCfg) {
	s := &DefaultSplitterStyle
	if cfg.Sizing == (Sizing{}) {
		cfg.Sizing = FillFill
	}
	if !cfg.ColorHandle.IsSet() {
		cfg.ColorHandle = s.ColorHandle
	}
	if !cfg.ColorHandleHover.IsSet() {
		cfg.ColorHandleHover = s.ColorHandleHover
	}
	if !cfg.ColorHandleActive.IsSet() {
		cfg.ColorHandleActive = s.ColorHandleActive
	}
	if !cfg.ColorHandleBorder.IsSet() {
		cfg.ColorHandleBorder = s.ColorHandleBorder
	}
	if !cfg.ColorGrip.IsSet() {
		cfg.ColorGrip = s.ColorGrip
	}
	if !cfg.ColorButton.IsSet() {
		cfg.ColorButton = s.ColorButton
	}
	if !cfg.ColorButtonHover.IsSet() {
		cfg.ColorButtonHover = s.ColorButtonHover
	}
	if !cfg.ColorButtonActive.IsSet() {
		cfg.ColorButtonActive = s.ColorButtonActive
	}
	if !cfg.ColorButtonIcon.IsSet() {
		cfg.ColorButtonIcon = s.ColorButtonIcon
	}
}

// Splitter creates a two-pane splitter with drag/keyboard/collapse.
func Splitter(cfg SplitterCfg) View {
	applySplitterDefaults(&cfg)
	core := newSplitterCore(&cfg)

	return Canvas(ContainerCfg{
		ID:              cfg.ID,
		IDFocus:         cfg.IDFocus,
		A11YRole:        AccessRoleSplitter,
		A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
		A11YDescription: cfg.A11YDescription,
		Sizing:          cfg.Sizing,
		Padding:         NoPadding,
		Clip:            true,
		Disabled:        cfg.Disabled,
		Invisible:       cfg.Invisible,
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			splitterOnKeydown(core, e, w)
		},
		AmendLayout: func(layout *Layout, w *Window) {
			splitterAmendLayout(core, layout, w)
		},
		Content: []View{
			splitterPane(cfg.ID+":pane:first", cfg.First.Content),
			splitterHandleView(&cfg, core),
			splitterPane(cfg.ID+":pane:second", cfg.Second.Content),
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

func splitterHandleView(cfg *SplitterCfg, core *splitterCore) View {
	content := make([]View, 0, 3)
	if cfg.ShowCollapseButtons &&
		(cfg.First.Collapsible || cfg.Second.Collapsible) {
		if cfg.First.Collapsible {
			content = append(content,
				splitterButton(cfg, core, SplitterCollapseFirst))
		}
		content = append(content, splitterGrip(cfg))
		if cfg.Second.Collapsible {
			content = append(content,
				splitterButton(cfg, core, SplitterCollapseSecond))
		}
	} else {
		content = append(content, splitterGrip(cfg))
	}

	orientation := cfg.Orientation
	colorHover := cfg.ColorHandleHover
	colorActive := cfg.ColorHandleActive

	s := &DefaultSplitterStyle
	handleSize := cfg.HandleSize.Get(s.HandleSize)
	var handleWidth, handleHeight float32
	if orientation == SplitterHorizontal {
		handleWidth = handleSize
	} else {
		handleHeight = handleSize
	}

	handleCfg := ContainerCfg{
		ID:          cfg.ID + ":handle",
		Sizing:      FixedFixed,
		Width:       handleWidth,
		Height:      handleHeight,
		Padding:     NoPadding,
		Spacing:     SomeF(1),
		Color:       cfg.ColorHandle,
		ColorBorder: cfg.ColorHandleBorder,
		SizeBorder:  cfg.SizeBorder,
		Radius:      cfg.Radius,
		HAlign:      HAlignCenter,
		VAlign:      VAlignMiddle,
		OnClick: func(_ *Layout, e *Event, w *Window) {
			splitterOnHandleClick(core, e, w)
		},
		OnHover: func(layout *Layout, e *Event, w *Window) {
			splitterOnHandleHover(orientation, colorHover,
				colorActive, layout, e, w)
		},
		Content: content,
	}

	if orientation == SplitterHorizontal {
		return Column(handleCfg)
	}
	return Row(handleCfg)
}

func splitterGrip(cfg *SplitterCfg) View {
	s := &DefaultSplitterStyle
	handleSize := cfg.HandleSize.Get(s.HandleSize)
	isHoriz := cfg.Orientation == SplitterHorizontal
	var w, h float32
	if isHoriz {
		w = f32Max(2, handleSize*0.35)
		h = f32Max(14, handleSize*2.0)
	} else {
		w = f32Max(14, handleSize*2.0)
		h = f32Max(2, handleSize*0.35)
	}
	return Rectangle(RectangleCfg{
		Width:  w,
		Height: h,
		Color:  cfg.ColorGrip,
		Radius: cfg.RadiusBorder.Get(s.RadiusBorder),
		Sizing: FixedFixed,
	})
}

func splitterButton(cfg *SplitterCfg, core *splitterCore,
	target SplitterCollapsed) View {
	s := &DefaultSplitterStyle
	size := f32Max(4, cfg.HandleSize.Get(s.HandleSize)-2)
	ts := TextStyle{
		Color: cfg.ColorButtonIcon,
		Size:  size,
	}
	return Button(ButtonCfg{
		ID:         cfg.ID + splitterButtonSuffix[target],
		Width:      size,
		Height:     size,
		Sizing:     FixedFixed,
		Padding:    NoPadding,
		Color:      cfg.ColorButton,
		ColorHover: cfg.ColorButtonHover,
		ColorClick: cfg.ColorButtonActive,
		ColorFocus: cfg.ColorButtonHover,
		Radius:     cfg.RadiusBorder,
		OnClick: func(_ *Layout, e *Event, w *Window) {
			splitterOnButtonClick(core, target, e, w)
		},
		Content: []View{
			Text(TextCfg{
				Text:      splitterButtonIcon(core, target),
				TextStyle: ts,
			}),
		},
	})
}

func splitterButtonIcon(core *splitterCore, target SplitterCollapsed) string {
	current := splitterEffectiveCollapsed(core, core.collapsed)
	if core.orientation == SplitterHorizontal {
		if target == SplitterCollapseFirst {
			if current == SplitterCollapseFirst {
				return "▶"
			}
			return "◀"
		}
		if current == SplitterCollapseSecond {
			return "◀"
		}
		return "▶"
	}
	if target == SplitterCollapseFirst {
		if current == SplitterCollapseFirst {
			return "▼"
		}
		return "▲"
	}
	if current == SplitterCollapseSecond {
		return "▲"
	}
	return "▼"
}

// --- Event handlers ---

func splitterOnKeydown(core *splitterCore, e *Event, w *Window) {
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

	nextRatio := splitterClampRatio(core, available, core.ratio)
	nextCollapsed := splitterEffectiveCollapsed(core, core.collapsed)
	handled := false

	isNone := e.Modifiers == ModNone

	switch e.KeyCode {
	case KeyLeft:
		nextRatio, handled = splitterArrowStep(core,
			SplitterHorizontal, -1, e.Modifiers, available, nextRatio)
	case KeyRight:
		nextRatio, handled = splitterArrowStep(core,
			SplitterHorizontal, +1, e.Modifiers, available, nextRatio)
	case KeyUp:
		nextRatio, handled = splitterArrowStep(core,
			SplitterVertical, -1, e.Modifiers, available, nextRatio)
	case KeyDown:
		nextRatio, handled = splitterArrowStep(core,
			SplitterVertical, +1, e.Modifiers, available, nextRatio)
	case KeyHome:
		if isNone && core.first.collapsible {
			nextCollapsed = SplitterCollapseFirst
			handled = true
		}
	case KeyEnd:
		if isNone && core.second.collapsible {
			nextCollapsed = SplitterCollapseSecond
			handled = true
		}
	case KeyEnter:
		if isNone {
			nextCollapsed, handled = splitterToggleCollapse(
				core, nextCollapsed)
		}
	default:
		if e.CharCode == CharSpace && isNone {
			nextCollapsed, handled = splitterToggleCollapse(
				core, nextCollapsed)
		}
	}
	// Arrow keys clear collapse state.
	if handled {
		switch e.KeyCode {
		case KeyLeft, KeyRight, KeyUp, KeyDown:
			nextCollapsed = SplitterCollapseNone
		}
	}

	if handled {
		splitterEmitChange(core, nextRatio, nextCollapsed, e, w)
	}
}

func splitterOnHandleClick(core *splitterCore, e *Event, w *Window) {
	if core.disabled {
		return
	}
	splitterSetCursor(core.orientation, w)
	splitterFocus(core, w)

	idFocus := core.idFocus
	w.MouseLock(MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			splitterOnDragMove(core, e, w)
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			w.MouseUnlock()
			if idFocus > 0 {
				w.SetIDFocus(idFocus)
			}
		},
	})
	e.IsHandled = true
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
	splitterEmitChange(core, ratio, SplitterCollapseNone, e, w)
}

func splitterOnHandleHover(
	orientation SplitterOrientation,
	colorHover, colorActive Color,
	layout *Layout, e *Event, w *Window,
) {
	splitterSetCursor(orientation, w)
	layout.Shape.Color = colorHover
	if e.MouseButton == MouseLeft {
		layout.Shape.Color = colorActive
	}
	e.IsHandled = true
}

func splitterOnButtonClick(
	core *splitterCore,
	target SplitterCollapsed,
	e *Event, w *Window,
) {
	if core.disabled {
		return
	}
	validTarget := splitterEffectiveCollapsed(core, target)
	if validTarget == SplitterCollapseNone {
		return
	}
	ratio := splitterCurrentRatio(core, w)
	current := splitterEffectiveCollapsed(core, core.collapsed)
	next := validTarget
	if current == validTarget {
		next = SplitterCollapseNone
	}
	splitterEmitChange(core, ratio, next, e, w)
}

// --- AmendLayout ---

func splitterAmendLayout(core *splitterCore, layout *Layout, w *Window) {
	if len(layout.Children) < 3 {
		return
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
	splitterResetPositions(child, true, AxisNone, 0, 0)
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
	layoutFillWidths(child)
	layoutWrapText(child, w)
	layoutHeights(child)
	layoutFillHeights(child)
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
	} else if parentAxis == AxisNone {
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

func splitterMainSize(layout *Layout, orientation SplitterOrientation) float32 {
	if orientation == SplitterHorizontal {
		return layout.Shape.Width
	}
	return layout.Shape.Height
}

func splitterHandleSizeFromLayout(
	layout *Layout,
	orientation SplitterOrientation,
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
	if active != SplitterCollapseNone {
		return active
	}
	if core.first.collapsible {
		return SplitterCollapseFirst
	}
	if core.second.collapsible {
		return SplitterCollapseSecond
	}
	return SplitterCollapseNone
}

func splitterArrowStep(core *splitterCore, orient SplitterOrientation,
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
	if target == SplitterCollapseNone {
		return current, false
	}
	if current == target {
		return SplitterCollapseNone, true
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
		return SplitterCollapseNone
	case SplitterCollapseSecond:
		if core.second.collapsible {
			return SplitterCollapseSecond
		}
		return SplitterCollapseNone
	default:
		return SplitterCollapseNone
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
		core.onChange(state.Ratio, state.Collapsed, e, w)
	}
	splitterFocus(core, w)
	e.IsHandled = true
}

func splitterFocus(core *splitterCore, w *Window) {
	if core.idFocus > 0 {
		w.SetIDFocus(core.idFocus)
	}
}

func splitterSetCursor(orientation SplitterOrientation, w *Window) {
	if orientation == SplitterHorizontal {
		w.SetMouseCursorEW()
	} else {
		w.SetMouseCursorNS()
	}
}
