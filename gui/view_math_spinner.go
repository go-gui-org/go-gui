package gui

import (
	"math"
	"sync"
	"time"
)

const ghostSteps = 200

type mathSpinnerGhostBuf struct {
	pts [(ghostSteps + 1) * 2]float32
}

var mathSpinnerGhostPool = sync.Pool{
	New: func() any { return &mathSpinnerGhostBuf{} },
}

// CurveType selects the mathematical curve rendered by a MathSpinner.
type CurveType uint8

// CurveType constants. Each constant maps to a specific parametric
// curve with baked-in default parameters. Override ParamA/B/D in
// MathSpinnerCfg to customize.
const (
	CurveOriginalThinking CurveType = iota // epitrochoid R=7,k=7,d=3
	CurveThinkingFive                      // epitrochoid R=7,k=5,d=3
	CurveThinkingNine                      // epitrochoid R=7,k=9,d=3
	CurveRoseOrbit                         // rose orbit orbit=7,petals=7,amp=3
	CurveRose                              // rose r=a·cos(kθ) a=9,k=5
	CurveRoseTwo                           // rose a=9,k=2
	CurveRoseThree                         // rose a=9,k=3
	CurveRoseFour                          // rose a=9,k=4
	CurveLissajous                         // lissajous a=3,b=4,δ=π/2
	CurveLemniscate                        // bernoulli lemniscate a=1
	CurveHypotrochoid                      // spirograph R=8,r=3,d=5
	CurveThreePetalSpiral                  // hypotrochoid R=3,r=1,d=3
	CurveFourPetalSpiral                   // hypotrochoid R=4,r=1,d=3
	CurveFivePetalSpiral                   // hypotrochoid R=5,r=1,d=3
	CurveSixPetalSpiral                    // hypotrochoid R=6,r=1,d=3
	CurveButterfly                         // butterfly turns=12,cosW=2,pow=5
	CurveCardioid                          // cardioid glow r=a(1-cosθ)
	CurveCardioidHeart                     // cardioid heart r=a(1+cosθ)
	CurveHeartWave                         // heart wave b=6.4,root=3.3,amp=0.9
	CurveSpiral                            // archimedean spiral turns=4
	CurveFourier                           // fourier multi-harmonic
)

// mathSpinnerFamily identifies the underlying math equation.
type mathSpinnerFamily uint8

const (
	familyEpitrochoid mathSpinnerFamily = iota
	familyRoseOrbit
	familyRose
	familyLissajous
	familyLemniscate
	familyHypotrochoid
	familyButterfly
	familyCardioid
	familyHeartWave
	familySpiral
	familyFourier
)

type mathSpinnerDefaults struct {
	family  mathSpinnerFamily
	a, b, d float32
}

var mathSpinnerCurveDefaults = [...]mathSpinnerDefaults{
	CurveOriginalThinking: {familyEpitrochoid, 7, 7, 3},
	CurveThinkingFive:     {familyEpitrochoid, 7, 5, 3},
	CurveThinkingNine:     {familyEpitrochoid, 7, 9, 3},
	CurveRoseOrbit:        {familyRoseOrbit, 7, 7, 3},
	CurveRose:             {familyRose, 9, 5, 0},
	CurveRoseTwo:          {familyRose, 9, 2, 0},
	CurveRoseThree:        {familyRose, 9, 3, 0},
	CurveRoseFour:         {familyRose, 9, 4, 0},
	CurveLissajous:        {familyLissajous, 3, 4, math.Pi / 2},
	CurveLemniscate:       {familyLemniscate, 1, 0, 0},
	CurveHypotrochoid:     {familyHypotrochoid, 8, 3, 5},
	CurveThreePetalSpiral: {familyHypotrochoid, 3, 1, 3},
	CurveFourPetalSpiral:  {familyHypotrochoid, 4, 1, 3},
	CurveFivePetalSpiral:  {familyHypotrochoid, 5, 1, 3},
	CurveSixPetalSpiral:   {familyHypotrochoid, 6, 1, 3},
	CurveButterfly:        {familyButterfly, 12, 2, 5},
	CurveCardioid:         {familyCardioid, 1, 0, 0},
	CurveCardioidHeart:    {familyCardioid, 1, 1, 0},
	CurveHeartWave:        {familyHeartWave, 6.4, 3.3, 0.9},
	CurveSpiral:           {familySpiral, 4, 8, 8.5},
	CurveFourier:          {familyFourier, 17, 15, 0},
}

// MathSpinnerCfg configures a mathematical curve spinner.
type MathSpinnerCfg struct {
	ID          string
	CurveType   CurveType
	Color       Color
	StrokeWidth float32
	Speed       float32
	Size        float32
	ParamA      Opt[float32]
	ParamB      Opt[float32]
	ParamD      Opt[float32]
	TrailLength float32
	Particles   int
	Rotate      bool
	Sizing      Sizing
	Width       float32
	Height      float32
	MinWidth    float32
	MaxWidth    float32
	MinHeight   float32
	MaxHeight   float32
}

// MathSpinner creates an animated mathematical curve loading indicator.
func MathSpinner(cfg MathSpinnerCfg, w *Window) View {
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.ColorActive
	}
	if cfg.StrokeWidth <= 0 {
		cfg.StrokeWidth = 2.5
	}
	if cfg.Speed <= 0 {
		cfg.Speed = 1
	}
	if cfg.Size <= 0 {
		cfg.Size = 48
	}
	if cfg.TrailLength <= 0 {
		cfg.TrailLength = 0.35
	}
	if cfg.Particles <= 0 {
		cfg.Particles = 60
	}
	if cfg.Particles < 2 {
		cfg.Particles = 2
	}
	if cfg.Particles > 500 {
		cfg.Particles = 500
	}
	if cfg.TrailLength > 1 {
		cfg.TrailLength = 1
	}

	// Clamp CurveType to valid range.
	ct := cfg.CurveType
	if ct > CurveFourier {
		ct = CurveOriginalThinking
	}

	// Apply baked-in param defaults when user hasn't set them.
	defs := mathSpinnerCurveDefaults[ct]
	paramA := cfg.ParamA.Get(defs.a)
	paramB := cfg.ParamB.Get(defs.b)
	paramD := cfg.ParamD.Get(defs.d)

	width := cfg.Width
	height := cfg.Height
	if width <= 0 && height <= 0 {
		width = cfg.Size
		height = cfg.Size
	}

	id := cfg.ID
	animID := "math_spinner_" + id
	dur := time.Duration(float64(5*time.Second) / float64(cfg.Speed))

	if !w.touchViewBoundAnimation(animID) {
		w.animationAddViewBound(&KeyframeAnimation{
			AnimID:   animID,
			Duration: dur,
			Repeat:   true,
			Keyframes: []Keyframe{
				{At: 0, Value: 0},
				{At: 1, Value: 1},
			},
			OnValue: func(v float32, w *Window) {
				StateMap[string, float32](w, nsMathSpinner, capModerate).Set(id, v)
			},
		})
	}

	progress := StateReadOr(w, nsMathSpinner, id, float32(0))

	// Optional slow rotation.
	rotKey := id + "_rot"
	rotAnimID := "math_spinner_rot_" + id
	if cfg.Rotate {
		if !w.touchViewBoundAnimation(rotAnimID) {
			w.animationAddViewBound(&KeyframeAnimation{
				AnimID:   rotAnimID,
				Duration: 30 * time.Second,
				Repeat:   true,
				Keyframes: []Keyframe{
					{At: 0, Value: 0},
					{At: 1, Value: 1},
				},
				OnValue: func(v float32, w *Window) {
					StateMap[string, float32](
						w, nsMathSpinner, capModerate).Set(rotKey, v)
				},
			})
		}
	}
	rotation := StateReadOr(w, nsMathSpinner, rotKey, float32(0))

	family := defs.family
	particles := cfg.Particles
	trailSpan := cfg.TrailLength
	strokeWidth := cfg.StrokeWidth
	color := cfg.Color

	sizing := cfg.Sizing
	if sizing == (Sizing{}) {
		sizing = FixedFixed
	}

	return Row(ContainerCfg{
		ID:         cfg.ID,
		Sizing:     sizing,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Width:      width,
		Height:     height,
		MinWidth:   cfg.MinWidth,
		MaxWidth:   cfg.MaxWidth,
		MinHeight:  cfg.MinHeight,
		MaxHeight:  cfg.MaxHeight,
		Content: []View{
			DrawCanvas(DrawCanvasCfg{
				ID:      id + "_cv",
				Sizing:  FillFill,
				Clip:    true,
				Version: uint64(math.Float32bits(progress + rotation)),
				OnDraw: func(dc *DrawContext) {
					mathSpinnerDraw(dc, family, progress, rotation,
						particles, trailSpan, strokeWidth,
						paramA, paramB, paramD, color)
				},
			}),
		},
	})
}

func mathSpinnerDraw(
	dc *DrawContext,
	family mathSpinnerFamily,
	progress, rotation float32,
	particles int,
	trailSpan, strokeWidth float32,
	paramA, paramB, paramD float32,
	color Color,
) {
	if dc.Width <= 0 || dc.Height <= 0 {
		return
	}
	cx := dc.Width / 2
	cy := dc.Height / 2
	scale := min(dc.Width, dc.Height)/2 - strokeWidth*2
	if scale <= 0 {
		return
	}

	// Precompute rotation sin/cos (rotation is 0-1, map to full turn).
	rotAngle := float64(rotation) * 2 * math.Pi
	rotSin, rotCos := math.Sincos(rotAngle)
	sinR := float32(rotSin)
	cosR := float32(rotCos)

	// Draw faint ghost path of the full curve.
	ghostBuf := mathSpinnerGhostPool.Get().(*mathSpinnerGhostBuf)
	ghostPts := ghostBuf.pts[:]
	for i := range ghostSteps + 1 {
		param := float32(i) / float32(ghostSteps)
		px, py := mathSpinnerCurvePoint(family, param, paramA, paramB, paramD)
		ghostPts[i*2] = cx + (px*cosR-py*sinR)*scale
		ghostPts[i*2+1] = cy + (px*sinR+py*cosR)*scale
	}
	ghostColor := RGBA(color.R, color.G, color.B, 30)
	dc.PolylineJoined(ghostPts, ghostColor, strokeWidth*0.8)
	mathSpinnerGhostPool.Put(ghostBuf)

	// Draw particle trail.
	for i := range particles {
		tailOffset := float32(i) / float32(particles-1)
		param := mathSpinnerNormalize(progress - tailOffset*trailSpan)
		px, py := mathSpinnerCurvePoint(family, param, paramA, paramB, paramD)

		// Apply rotation around origin before scaling.
		rx := px*cosR - py*sinR
		ry := px*sinR + py*cosR
		px = rx
		py = ry
		fade := float32(math.Pow(float64(1-tailOffset), 0.56))
		alpha := uint8(10 + fade*245)
		radius := strokeWidth*0.4 + fade*strokeWidth*1.2
		c := RGBA(color.R, color.G, color.B, alpha)
		dc.FilledCircle(cx+px*scale, cy+py*scale, radius, c)
	}
}

func mathSpinnerNormalize(t float32) float32 {
	return t - float32(math.Floor(float64(t)))
}

// mathSpinnerClampPoint replaces NaN/Inf with 0 and clamps to [-2,2].
func mathSpinnerClampPoint(x, y float32) (float32, float32) {
	if x != x || x > 2 || x < -2 { // NaN or out of range
		x = 0
	}
	if y != y || y > 2 || y < -2 {
		y = 0
	}
	return x, y
}

func mathSpinnerCurvePoint(
	family mathSpinnerFamily,
	progress, a, b, d float32,
) (float32, float32) {
	var px, py float32
	switch family {
	case familyEpitrochoid:
		px, py = mathSpinnerEpitrochoid(progress, a, b, d)
	case familyRoseOrbit:
		px, py = mathSpinnerRoseOrbit(progress, a, b, d)
	case familyRose:
		px, py = mathSpinnerRose(progress, a, b)
	case familyLissajous:
		px, py = mathSpinnerLissajous(progress, a, b, d)
	case familyLemniscate:
		px, py = mathSpinnerLemniscate(progress)
	case familyHypotrochoid:
		px, py = mathSpinnerHypotrochoid(progress, a, b, d)
	case familyButterfly:
		px, py = mathSpinnerButterfly(progress, a, b, d)
	case familyCardioid:
		px, py = mathSpinnerCardioid(progress, a, b)
	case familyHeartWave:
		px, py = mathSpinnerHeartWave(progress, a, b, d)
	case familySpiral:
		px, py = mathSpinnerSpiral(progress, a, b, d)
	case familyFourier:
		px, py = mathSpinnerFourier(progress, a, b)
	}
	return mathSpinnerClampPoint(px, py)
}

// epitrochoid: x = R·cos(t) - d·cos(k·t)
func mathSpinnerEpitrochoid(
	progress, bigR, k, d float32,
) (float32, float32) {
	t := float64(progress) * 2 * math.Pi
	norm := bigR + d
	if norm == 0 {
		return 0, 0
	}
	x := bigR*float32(math.Cos(t)) - d*float32(math.Cos(float64(k)*t))
	y := bigR*float32(math.Sin(t)) - d*float32(math.Sin(float64(k)*t))
	return x / norm, y / norm
}

// roseOrbit: r = orbit - amp·cos(petals·t)
func mathSpinnerRoseOrbit(
	progress, orbit, petals, amp float32,
) (float32, float32) {
	t := float64(progress) * 2 * math.Pi
	r := orbit - amp*float32(math.Cos(float64(petals)*t))
	norm := orbit + amp
	if norm == 0 {
		return 0, 0
	}
	return float32(math.Cos(t)) * r / norm,
		float32(math.Sin(t)) * r / norm
}

// rose: r = a·cos(k·θ)
func mathSpinnerRose(progress, a, k float32) (float32, float32) {
	if a == 0 {
		return 0, 0
	}
	t := float64(progress) * 2 * math.Pi
	r := a * float32(math.Cos(float64(k)*t))
	return r * float32(math.Cos(t)) / a,
		r * float32(math.Sin(t)) / a
}

// lissajous: x = sin(a·t + phase), y = sin(b·t)
func mathSpinnerLissajous(
	progress, ax, by, phase float32,
) (float32, float32) {
	t := float64(progress) * 2 * math.Pi
	return float32(math.Sin(float64(ax)*t+float64(phase))) * 0.9,
		float32(math.Sin(float64(by)*t)) * 0.9
}

// lemniscate: x = cos(t)/(1+sin²t), y = sin(t)·cos(t)/(1+sin²t)
func mathSpinnerLemniscate(progress float32) (float32, float32) {
	t := float64(progress) * 2 * math.Pi
	sinT, cosT := math.Sincos(t)
	denom := 1 + sinT*sinT
	return float32(cosT / denom), float32(sinT * cosT / denom)
}

// hypotrochoid: x = (R-r)cos(t) + d·cos((R-r)t/r)
func mathSpinnerHypotrochoid(
	progress, bigR, r, d float32,
) (float32, float32) {
	if r == 0 {
		return 0, 0
	}
	t := float64(progress) * 2 * math.Pi
	diff := bigR - r
	ratio := float64(diff / r)
	norm := diff + d
	if norm == 0 {
		return 0, 0
	}
	x := diff*float32(math.Cos(t)) + d*float32(math.Cos(ratio*t))
	y := diff*float32(math.Sin(t)) - d*float32(math.Sin(ratio*t))
	return x / norm, y / norm
}

// butterfly: s = exp(cos t) - cosW·cos(4t) - sin(t/12)^pow
func mathSpinnerButterfly(
	progress, turns, cosWeight, power float32,
) (float32, float32) {
	t := float64(progress) * math.Pi * float64(turns)
	sinVal := math.Sin(t / 12)
	powTerm := math.Pow(math.Abs(sinVal), float64(power))
	if sinVal < 0 && int(power)%2 == 1 {
		powTerm = -powTerm
	}
	s := float32(math.Exp(math.Cos(t))) -
		cosWeight*float32(math.Cos(4*t)) -
		float32(powTerm)
	x := float32(math.Sin(t)) * s / 4.5
	y := float32(math.Cos(t)) * s / 4.5
	return x, y
}

// cardioid: r = a·(1-cosθ) or r = a·(1+cosθ) rotated
func mathSpinnerCardioid(
	progress, a, variant float32,
) (float32, float32) {
	t := float64(progress) * 2 * math.Pi
	cosT := float32(math.Cos(t))
	sinT := float32(math.Sin(t))
	if a == 0 {
		return 0, 0
	}
	if variant > 0.5 {
		r := a * (1 + cosT)
		return -sinT * r / (2 * a), -cosT * r / (2 * a)
	}
	r := a * (1 - cosT)
	return cosT * r / (2 * a), sinT * r / (2 * a)
}

// heartWave: y = |x|^(2/3) + amp·√(root-x²)·sin(b·π·x)
func mathSpinnerHeartWave(
	progress, b, root, amp float32,
) (float32, float32) {
	if root <= 0 {
		return 0, 0
	}
	xLimit := float32(math.Sqrt(float64(root)))
	x := -xLimit + progress*xLimit*2
	safeRoot := max(float32(0), root-x*x)
	wave := amp * float32(math.Sqrt(float64(safeRoot))) *
		float32(math.Sin(float64(b)*math.Pi*float64(x)))
	curve := float32(math.Pow(math.Abs(float64(x)), 2.0/3.0))
	y := curve + wave
	return x / xLimit, -(y - 1.75) / (xLimit * 1.2)
}

// spiral: r = base + (1-cos t)·amp; θ = turns·t
func mathSpinnerSpiral(
	progress, turns, baseR, rAmp float32,
) (float32, float32) {
	t := float64(progress) * 2 * math.Pi
	angle := t * float64(turns)
	radius := baseR + (1-float32(math.Cos(t)))*rAmp
	norm := baseR + 2*rAmp
	if norm == 0 {
		return 0, 0
	}
	return float32(math.Cos(angle)) * radius / norm,
		float32(math.Sin(angle)) * radius / norm
}

// fourier: sum of harmonic terms on x and y axes.
// a = x1 amplitude, b = y1 amplitude.
func mathSpinnerFourier(progress, x1, y1 float32) (float32, float32) {
	t := float64(progress) * 2 * math.Pi
	x3 := x1 * 0.44
	x5 := x1 * 0.19
	y2 := y1 * 0.55
	y4 := y1 * 0.28
	x := x1*float32(math.Cos(t)) +
		x3*float32(math.Cos(3*t+0.6)) +
		x5*float32(math.Sin(5*t-0.4))
	y := y1*float32(math.Sin(t)) +
		y2*float32(math.Sin(2*t+0.25)) -
		y4*float32(math.Cos(4*t-0.5))
	norm := x1 + x3 + x5
	if norm == 0 {
		return 0, 0
	}
	return x / norm, y / norm
}
