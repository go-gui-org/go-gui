package gui

import (
	"math"
	"time"
)

// orbCycleLen is the geometry-seconds loop of one orb animation
// tick cycle. Progress runs 0..1 over the keyframe duration and
// maps to 0..orbCycleLen geometry seconds, so Speed only changes
// how fast the loop runs, never which frames exist. 12 covers the
// slowest golden instant (5.1) and the morph cycle (~6.9).
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
// zero or invalid Speed means 1. Speed, Pause state, and Design
// are sampled on first render; use a different widget ID to
// apply new parameters.
func ThinkingOrb(cfg ThinkingOrbCfg) View {
	RequireID("ThinkingOrb", cfg.ID)
	return &thinkingOrbView{cfg: cfg}
}

// ThinkingOrbLabel creates an orb beside a status line. Text
// shimmers while live and holds still at full strength under
// Reduce Motion.
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
	still := cfg.Paused || w.prefersReducedMotion() ||
		w.HeadlessRender()
	progress := StateReadOr(w, nsThinkingOrb, eid, float32(0))
	geomT := float64(progress) * orbCycleLen
	if still {
		geomT = orbStillT
		if cfg.Paused && progress != 0 {
			geomT = float64(progress) * orbCycleLen
		}
	}

	custom := cfg.Color.IsSet()
	baseColor := cfg.Color
	if !custom {
		baseColor = guiTheme.TextStyleDef.Color
	}
	dark := thinkingOrbDarkGround()
	frame := orbFrame(cfg.Design, cfg.Size, geomT)

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
				ID:      "cv",
				Sizing:  FillFill,
				Clip:    true,
				Version: uint64(math.Float32bits(progress)),
				OnDraw: func(dc *DrawContext) {
					thinkingOrbDraw(dc, frame, float64(length),
						dark, custom, baseColor)
				},
			}),
		},
	}), w)
}

// thinkingOrbAmendLayout registers the repeating progress tick
// keyed by the effective ID. Still orbs (paused, reduced
// motion, headless) register nothing and keep their frame.
func thinkingOrbAmendLayout(layout *Layout, w *Window,
	design ThinkingOrbDesign, size ThinkingOrbSize, speed float64,
	paused bool, id string) {
	if paused || w.prefersReducedMotion() || w.HeadlessRender() {
		return
	}
	animID := ScopeID(id, "orb")
	if !w.touchViewBoundAnimation(animID) {
		w.animationAddViewBound(&KeyframeAnimation{
			AnimID:   animID,
			Duration: thinkingOrbDuration(speed),
			Repeat:   true,
			Keyframes: []Keyframe{
				{At: 0, Value: 0},
				{At: 1, Value: 1},
			},
			OnValue: func(val float32, win *Window) {
				StateMap[string, float32](
					win, nsThinkingOrb, capModerate).Set(id, val)
			},
		})
	}
}

// thinkingOrbDraw paints lines first, then dots far to near (the
// frame order). Box units map onto the canvas with independent
// axis scales; radii and widths take the smaller one. Ink is
// matte gray from the white field, mirrored on dark grounds. A
// caller color replaces the gray with its own RGB at the dot
// alpha.
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
		gray := orbInkByte(line.white, dark)
		alpha := orbAlphaByte(line.a)
		dc.Line(float32(float64(line.x1)*sx), float32(float64(line.y1)*sy),
			float32(float64(line.x2)*sx), float32(float64(line.y2)*sy),
			RGBA(gray, gray, gray, alpha), float32(line.w*rs))
	}
	for i := range frame.dots {
		dot := &frame.dots[i]
		alpha := orbAlphaByte(dot.a)
		var col Color
		if custom {
			col = RGBA(base.R, base.G, base.B, alpha)
		} else {
			gray := orbInkByte(dot.white, dark)
			col = RGBA(gray, gray, gray, alpha)
		}
		dc.FilledCircle(float32(dot.x*sx), float32(dot.y*sy),
			float32(dot.r*rs), col)
	}
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
	if w.prefersReducedMotion() || w.HeadlessRender() {
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
			Label:       a11yLabel(cfg.A11YLabel, cfg.Text),
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
