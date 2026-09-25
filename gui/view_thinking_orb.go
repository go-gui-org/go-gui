package gui

import (
	"cmp"
	"math"
	"time"
)

// orbCycleLen is the geometry seconds one keyframe tick period
// adds to the orb clock. Progress runs 0..1 over the keyframe
// duration and each step adds its share of orbCycleLen, so Speed
// only changes how fast the clock runs, never which frames exist.
// The clock itself never wraps (see orbClock): no design repeats
// at a fixed period, so a wrap would show as a jump.
const orbCycleLen = 12.0

// orbStillT is the geometry-time representative frame for still
// rendering: paused with no stored progress, Reduce Motion, and
// headless captures. It matches the upstream still frame.
const orbStillT = 0.6

// ThinkingOrbCfg configures a semantic loading orb for AI and
// agent interfaces. Every design is a dotted 3D form drawn in
// monochrome ink that follows the theme; pick the design that
// says what the agent is doing.
type ThinkingOrbCfg struct {
	ID        string `gui:"required"`
	Design    ThinkingOrbDesign
	Size      ThinkingOrbSize // ergonomics-audit:opt-plain — zero is Regular, the tuned default, not "unset"
	Speed     float32
	Paused    bool
	Color     Color
	Padding   Padding
	Sizing    Sizing
	Width     float32
	Height    float32
	MinWidth  float32
	MaxWidth  float32
	MinHeight float32
	MaxHeight float32
	OnClick   func(EventCtx)
	A11YCfg
}

// ThinkingOrbLabelCfg configures an orb beside a shimmering
// status line. It reads as one accessibility element: its title.
type ThinkingOrbLabelCfg struct {
	ID        string `gui:"required"`
	Text      string
	Design    ThinkingOrbDesign
	Size      ThinkingOrbSize // ergonomics-audit:opt-plain — zero is Regular, the tuned default, not "unset"
	Speed     float32
	Paused    bool
	TextStyle TextStyle
	Color     Color
	Padding   Padding
	Sizing    Sizing
	A11YCfg
}

// ThinkingOrb creates a semantic loading orb. The zero Design is
// Working (safe general busy) and the zero Size is Regular. A
// zero or invalid Speed means 1. Speed, Paused, and Design apply
// live: a change continues from the current frame, with no jump.
func ThinkingOrb(cfg ThinkingOrbCfg) View {
	RequireID("ThinkingOrb", cfg.ID)
	return &thinkingOrbView{cfg: cfg}
}

// ThinkingOrbLabel creates an orb beside a status line. Text
// shimmers while live and holds still at full strength when
// Paused and under Reduce Motion.
func ThinkingOrbLabel(cfg ThinkingOrbLabelCfg) View {
	RequireID("ThinkingOrbLabel", cfg.ID)
	return &thinkingOrbLabelView{cfg: cfg}
}

type thinkingOrbView struct {
	cfg ThinkingOrbCfg
	// noA11Y suppresses the orb's own accessibility node so a
	// label wrapper reads as one element.
	noA11Y bool
}

type thinkingOrbLabelView struct {
	cfg ThinkingOrbLabelCfg
}

// orbClock is one orb's geometry clock, kept per effective ID in
// the nsThinkingOrb StateMap. The keyframe tick only supplies
// progress steps; t adds them up and never wraps, so the motion
// stays continuous across tick loops, pauses and speed changes.
type orbClock struct {
	// t is the geometry time in seconds on the orb clock.
	t float64
	// speed is the effective speed the running tick was built
	// for. A different speed rebuilds the tick.
	speed float64
	// prev is the last progress value the running tick sent.
	prev float32
	// gen names the running tick. A value that an old tick
	// queued before it was removed carries an old gen and is
	// dropped, so it cannot move the clock.
	gen uint32
}

// advance adds the step from prev to val to the clock. Progress
// wraps from 1 to 0 at each tick loop, so a negative step is a
// wrap and adds the rest of the loop.
func (c *orbClock) advance(val float32) {
	step := float64(val - c.prev)
	if step < 0 {
		step++
	}
	c.t += step * orbCycleLen
	c.prev = val
}

// thinkingOrbDuration converts an effective speed (resolved tuned
// speed times caller multiplier) to the keyframe period. The
// seconds clamp before conversion so extreme speeds cannot make
// a zero or overflowing duration.
func thinkingOrbDuration(speed float64) time.Duration {
	secs := orbCycleLen / speed
	if secs < 0.05 {
		secs = 0.05
	}
	if secs > 3600 {
		secs = 3600
	}
	return time.Duration(secs * float64(time.Second))
}

// thinkingOrbDarkGround reports whether the installed theme is
// dark: its text reads lighter than its panel. The orb mirrors
// ink on dark grounds so near dots read brightest.
func thinkingOrbDarkGround() bool {
	return srgbLuminance(guiTheme.TextStyleDef.Color) >=
		srgbLuminance(guiTheme.ColorPanel)
}

func (v *thinkingOrbView) GenerateLayout(w *Window) Layout {
	cfg := v.cfg
	if cfg.Design > ThinkingOrbShaping {
		cfg.Design = ThinkingOrbWorking
	}
	// mathSpinnerPositive also sanitizes orb numerics: its !(v >
	// 0) form catches NaN and its finite test rejects infinities.
	speed := mathSpinnerPositive(cfg.Speed, 1)
	resolved := orbResolve(cfg.Design, cfg.Size)
	effSpeed := resolved.speed * float64(speed)

	length := float32(cfg.Size.Length())
	width := mathSpinnerPositive(cfg.Width, length)
	height := mathSpinnerPositive(cfg.Height, length)

	eid := w.EffID(cfg.ID)
	clk := StateReadOr(w, nsThinkingOrb, eid, orbClock{})
	geomT := clk.t
	// Reduce Motion and headless captures show the representative
	// frame. A paused orb holds the frame it reached; one paused
	// before its first tick has no frame yet and shows the
	// representative one too.
	if w.prefersReducedMotion() || w.HeadlessRender() ||
		(cfg.Paused && clk.t == 0) {
		geomT = orbStillT
	}

	custom := cfg.Color.IsSet()
	baseColor := cfg.Color
	dark := thinkingOrbDarkGround()
	design := cfg.Design
	size := cfg.Size

	role := AccessRoleImage
	state := AccessStateBusy | AccessStateLive
	var access *accessInfo
	if v.noA11Y {
		role = AccessRoleNone
		state = AccessStateNone
	} else {
		access = &accessInfo{
			Label:       a11yLabel(cfg.A11YLabel, cfg.Design.A11YLabel()),
			Description: cfg.A11YDescription,
		}
	}

	amendDesign := cfg.Design
	amendSize := cfg.Size
	amendSpeed := effSpeed
	amendPaused := cfg.Paused

	return generateViewLayout(Row(ContainerCfg{
		ID:         cfg.ID,
		A11YRole:   role,
		A11YState:  state,
		a11Y:       access,
		Sizing:     cfg.Sizing.Or(FixedFixed),
		Padding:    cfg.Padding.Or(NoPadding),
		SizeBorder: NoBorder,
		Width:      width,
		Height:     height,
		MinWidth:   cfg.MinWidth,
		MaxWidth:   cfg.MaxWidth,
		MinHeight:  cfg.MinHeight,
		MaxHeight:  cfg.MaxHeight,
		OnClick:    cfg.OnClick,
		AmendLayout: func(ctx EventCtx) {
			thinkingOrbAmendLayout(ctx.Layout, ctx.Window,
				amendDesign, amendSize, amendSpeed, amendPaused,
				ctx.Layout.Shape.idKey())
		},
		Content: []View{
			DrawCanvas(DrawCanvasCfg{
				ID:     "cv",
				Sizing: FillFill,
				Clip:   true,
				// The canvas redraws only when Version changes, so
				// it folds in every input OnDraw reads.
				Version: thinkingOrbVersion(geomT, design, size,
					dark, custom, baseColor),
				OnDraw: func(dc *DrawContext) {
					// Built here, not in GenerateLayout, so a cache
					// hit skips the geometry. The frame aliases the
					// window's scratch buffers and is used up before
					// this returns.
					frame := orbFrameInto(&w.scratch.orb,
						design, size, geomT)
					thinkingOrbDraw(dc, frame, float64(length),
						dark, custom, baseColor)
				},
			}),
		},
	}), w)
}

// thinkingOrbVersion folds every input of the orb drawing into
// the canvas Version: the frame time, design, size, ground
// polarity and caller color. A still orb keeps one frame time, so
// without the other inputs a theme, Color or Design change would
// leave the cached drawing on screen.
func thinkingOrbVersion(t float64, design ThinkingOrbDesign,
	size ThinkingOrbSize, dark, custom bool, base Color) uint64 {
	// FNV-1a style mixing: cheap, and a one-bit change in any
	// input changes the result.
	const prime = 1099511628211
	v := math.Float64bits(t)
	v = (v ^ uint64(design)) * prime
	v = (v ^ uint64(size)) * prime
	var flags uint64
	if dark {
		flags |= 1
	}
	if custom {
		flags |= 2
		// The color only counts when set: an unset color draws
		// gray whatever its bytes are.
		flags |= uint64(base.R)<<8 | uint64(base.G)<<16 |
			uint64(base.B)<<24 | uint64(base.A)<<32
	}
	return (v ^ flags) * prime
}

// thinkingOrbAmendLayout runs the repeating progress tick keyed
// by the effective ID. Still orbs (paused, reduced motion,
// headless) remove the tick at once, so the frame stops where it
// is. A speed change (Speed or Design) rebuilds the tick; the
// clock carries on from its current time.
func thinkingOrbAmendLayout(layout *Layout, w *Window,
	design ThinkingOrbDesign, size ThinkingOrbSize, speed float64,
	paused bool, id string) {
	sm := StateMap[string, orbClock](w, nsThinkingOrb, capModerate)
	clk, _ := sm.Get(id)
	if paused || w.prefersReducedMotion() || w.HeadlessRender() {
		// speed != 0 means a tick was built and not yet stopped, so
		// the work below runs once, on the live-to-still frame. A
		// still orb on later frames returns here without building
		// the scoped ID string.
		if clk.speed == 0 {
			return
		}
		// Bump gen first: a value the old tick already queued then
		// fails the gen check and cannot move the paused frame.
		clk.gen++
		clk.speed = 0
		sm.Set(id, clk)
		// A view-bound tick left alone runs on for up to
		// animViewBoundStale and keeps moving the frame.
		w.AnimationRemove(ScopeID(id, "orb"))
		return
	}
	animID := ScopeID(id, "orb")
	if w.touchViewBoundAnimation(animID) && clk.speed == speed {
		return
	}
	// New tick: it starts at progress 0, so prev restarts at 0 and
	// the first step adds only the time since the start.
	w.AnimationRemove(animID)
	clk.speed = speed
	clk.prev = 0
	clk.gen++
	gen := clk.gen
	sm.Set(id, clk)
	w.animationAddViewBound(&KeyframeAnimation{
		AnimID:   animID,
		Duration: thinkingOrbDuration(speed),
		Repeat:   true,
		Keyframes: []Keyframe{
			{At: 0, Value: 0},
			{At: 1, Value: 1},
		},
		OnValue: func(val float32, win *Window) {
			clocks := StateMap[string, orbClock](
				win, nsThinkingOrb, capModerate)
			cur, ok := clocks.Get(id)
			if !ok || cur.gen != gen {
				return
			}
			cur.advance(val)
			clocks.Set(id, cur)
		},
	})
}

// thinkingOrbDraw paints lines first, then dots far to near (the
// frame order). Box units map onto the canvas with independent
// axis scales; radii and widths take the smaller one. Ink comes
// from orbInk for lines and dots alike.
func thinkingOrbDraw(dc *DrawContext, frame orbFrameResult,
	boxLen float64, dark bool, custom bool, base Color) {
	if !(boxLen > 0) || !(dc.Width > 0) || !(dc.Height > 0) {
		return
	}
	sx := float64(dc.Width) / boxLen
	sy := float64(dc.Height) / boxLen
	rs := sx
	if sy < rs {
		rs = sy
	}
	for i := range frame.lines {
		line := &frame.lines[i]
		dc.Line(float32(float64(line.x1)*sx), float32(float64(line.y1)*sy),
			float32(float64(line.x2)*sx), float32(float64(line.y2)*sy),
			orbInk(line.white, line.a, dark, custom, base),
			float32(line.w*rs))
	}
	for i := range frame.dots {
		dot := &frame.dots[i]
		dc.FilledCircle(float32(dot.x*sx), float32(dot.y*sy),
			float32(dot.r*rs),
			orbInk(dot.white, dot.a, dark, custom, base))
	}
}

// orbInk returns the color of one mark. Unset ink is matte gray
// from the white field, mirrored on dark grounds. A caller color
// replaces the gray with its own RGB, and its alpha scales the
// mark alpha, so a half-transparent color draws half as strong.
func orbInk(white, a float64, dark, custom bool, base Color) Color {
	if custom {
		return RGBA(base.R, base.G, base.B,
			orbAlphaByte(a*float64(base.A)/255))
	}
	gray := orbInkByte(white, dark)
	return RGBA(gray, gray, gray, orbAlphaByte(a))
}

// orbInkByte maps a white field (0 darkest) to a gray byte,
// mirrored on dark grounds so near dots read brightest.
func orbInkByte(white float64, dark bool) uint8 {
	w := white
	if w < 0 {
		w = 0
	}
	if w > 1 {
		w = 1
	}
	if dark {
		w = 1 - w
	}
	return uint8(math.Floor(w*255 + 0.5))
}

// orbAlphaByte maps a unit opacity to a byte.
func orbAlphaByte(a float64) uint8 {
	if !(a > 0) {
		return 0
	}
	if a >= 1 {
		return 255
	}
	return uint8(math.Floor(a*255 + 0.5))
}

func (v *thinkingOrbLabelView) GenerateLayout(w *Window) Layout {
	cfg := v.cfg
	if cfg.Design > ThinkingOrbShaping {
		cfg.Design = ThinkingOrbWorking
	}
	style := cfg.TextStyle
	if style == (TextStyle{}) {
		style = guiTheme.TextStyleDef
	}
	anim := TextAnimCfg{Kind: TextAnimShimmer, Repeat: true}
	// A paused orb beside moving text still reads as busy, so the
	// text holds still with the orb.
	if cfg.Paused || w.prefersReducedMotion() || w.HeadlessRender() {
		anim = TextAnimCfg{}
	}
	orbCfg := ThinkingOrbCfg{
		ID:     "orb",
		Design: cfg.Design,
		Size:   cfg.Size,
		Speed:  cfg.Speed,
		Paused: cfg.Paused,
		Color:  cfg.Color,
		Sizing: FitFit,
	}
	inner := &thinkingOrbView{cfg: orbCfg, noA11Y: true}

	return generateViewLayout(Row(ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleImage,
		A11YState: AccessStateBusy | AccessStateLive,
		a11Y: &accessInfo{
			// The inner orb has no node of its own, so empty Text
			// falls back to the design name, as a bare orb reads.
			Label: a11yLabel(cfg.A11YLabel,
				cmp.Or(cfg.Text, cfg.Design.A11YLabel())),
			Description: cfg.A11YDescription,
		},
		Sizing:     cfg.Sizing.Or(FitFit),
		Padding:    cfg.Padding.Or(NoPadding),
		SizeBorder: NoBorder,
		Spacing:    SomeF(guiTheme.SpacingSmall),
		HAlign:     HAlignStart,
		VAlign:     VAlignMiddle,
		Content: []View{
			inner,
			Text(TextCfg{
				ID:        "text",
				Text:      cfg.Text,
				TextStyle: style,
				Anim:      anim,
			}),
		},
	}), w)
}
