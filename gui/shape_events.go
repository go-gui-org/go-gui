package gui

// eventHandlers holds optional event callback fields.
type eventHandlers struct {
	// One rule for all of these: the callback must call ctx.Consume()
	// to stop propagation. Until v0.55.0 the five below were
	// "consume-class" and dispatch marked their events handled before
	// the callback ran; that split is gone.
	OnChar      func(EventCtx)
	OnClick     func(EventCtx)
	OnMouseUp   func(EventCtx)
	OnMouseDown func(EventCtx)
	OnGesture   func(EventCtx)
	OnFileDrop  func(EventCtx)

	OnKeyDown     func(EventCtx)
	OnKeyUp       func(EventCtx)
	OnMouseMove   func(EventCtx)
	OnMouseScroll func(EventCtx)
	OnHover       func(EventCtx)
	OnMouseLeave  func(EventCtx)
	onIMECommit   func(string, EventCtx)

	// No originating event — ctx.Event is nil in these.
	onScroll    func(EventCtx)
	AmendLayout func(EventCtx)

	OnDraw func(*DrawContext)
	// versionFn is DrawCanvasCfg.VersionFn (here, not on Shape, to keep Shape small).
	versionFn func() uint64

	// Click filters — set by widget factories to avoid the per-frame
	// closure allocation a wrapper callback would cost.
	clickButton  clickFilter // limits which mouse button fires OnClick
	clickOnSpace bool        // fire OnClick on spacebar via OnChar dispatch
	clickOnEnter bool        // fire OnClick on Enter key via OnKeyDown dispatch

	amendOnRender bool       // ContainerCfg.amendOnRender
	hoverStatic   bool       // ContainerCfg.hoverStatic
	hover         HoverStyle // ContainerCfg.Hover, painted before OnHover
	pointerAmend  bool       // ContainerCfg.pointerAmend

	// soundCue is the cue dispatch emits when this shape's OnClick
	// fires. Resolved at generation time from the theme and the
	// widget's Cfg, and stored here rather than in a closure for the
	// same reason as the flags above (issue #446). SoundNone = silent.
	soundCue SoundCue

	// scrollbar is the settings of the stock scrollbar this shape is the
	// bar or thumb of, nil otherwise. The stock handlers are shared
	// funcs that read it here rather than closures each holding a copy,
	// which cost four allocations per bar per frame. It points into the
	// scrollbarView, which lives as long as the layout built from it.
	scrollbar *ScrollbarCfg
}
