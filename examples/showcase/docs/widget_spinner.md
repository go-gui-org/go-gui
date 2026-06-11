Animated mathematical curve rendered as a particle trail.
Twenty-one named curve types are available, each with sensible
defaults. Override ParamA/B/D for custom tuning. Optional slow
rotation adds visual depth.

## Usage

```go
gui.MathSpinner(gui.MathSpinnerCfg{
    ID:        "loading",
    CurveType: gui.CurveRose,
    Size:      80,
}, w)
```

## With Color and Rotation

```go
gui.MathSpinner(gui.MathSpinnerCfg{
    ID:        "search",
    CurveType: gui.CurveHypotrochoid,
    Size:      100,
    Color:     gui.RGB(46, 160, 67),
    Rotate:    true,
}, w)
```

## Custom Parameters

```go
gui.MathSpinner(gui.MathSpinnerCfg{
    ID:          "custom",
    CurveType:   gui.CurveLissajous,
    Size:        120,
    ParamA:      gui.SomeF(5),
    ParamB:      gui.SomeF(6),
    ParamD:      gui.SomeF(1.57),
    StrokeWidth: 3,
    Speed:       1.5,
    TrailLength: 0.5,
    Particles:   80,
}, w)
```

## Key Properties

| Property    | Type         | Description                                     |
|-------------|--------------|-------------------------------------------------|
| ID          | string       | Unique identifier (drives animation)            |
| CurveType   | CurveType    | Named curve constant (21 available)             |
| Size        | float32      | Convenience square size (default 48)            |
| Width       | float32      | Explicit width (overrides Size)                 |
| Height      | float32      | Explicit height (overrides Size)                |
| Sizing      | Sizing       | Combined axis sizing mode                       |
| MinWidth    | float32      | Minimum width                                   |
| MaxWidth    | float32      | Maximum width                                   |
| MinHeight   | float32      | Minimum height                                  |
| MaxHeight   | float32      | Maximum height                                  |
| ParamA      | Opt[float32] | Curve-specific parameter A                      |
| ParamB      | Opt[float32] | Curve-specific parameter B                      |
| ParamD      | Opt[float32] | Curve-specific parameter D                      |

## Appearance

| Property    | Type    | Description                                        |
|-------------|---------|----------------------------------------------------|
| Color       | Color   | Particle and ghost path color (default ColorActive) |
| StrokeWidth | float32 | Base particle radius (default 2.5)                 |
| TrailLength | float32 | Fraction of curve visible as trail, 0-1 (def 0.35) |
| Particles   | int     | Number of dots in trail, 2-500 (default 60)        |

## Animation

| Property | Type    | Description                                       |
|----------|---------|---------------------------------------------------|
| Speed    | float32 | Animation speed multiplier (default 1)            |
| Rotate   | bool    | Slowly rotate entire curve (30s per revolution)   |

## Curve Types

| Constant              | Family       | Description                         |
|-----------------------|--------------|-------------------------------------|
| CurveOriginalThinking | Epitrochoid  | 7-petal ring (R=7, k=7, d=3)       |
| CurveThinkingFive     | Epitrochoid  | 5-petal ring                        |
| CurveThinkingNine     | Epitrochoid  | 9-petal ring                        |
| CurveRoseOrbit        | Rose Orbit   | Orbit with petal modulation         |
| CurveRose             | Rose         | 5-petal rose r=a cos(kθ)           |
| CurveRoseTwo          | Rose         | 2-petal rose                        |
| CurveRoseThree        | Rose         | 3-petal rose                        |
| CurveRoseFour         | Rose         | 4-petal rose                        |
| CurveLissajous        | Lissajous    | Crossed sine paths                  |
| CurveLemniscate       | Lemniscate   | Bernoulli infinity sign             |
| CurveHypotrochoid     | Hypotrochoid | Inner spirograph (R=8, r=3, d=5)   |
| CurveThreePetalSpiral | Hypotrochoid | 3-petal spirograph                  |
| CurveFourPetalSpiral  | Hypotrochoid | 4-petal spirograph                  |
| CurveFivePetalSpiral  | Hypotrochoid | 5-petal spirograph                  |
| CurveSixPetalSpiral   | Hypotrochoid | 6-petal spirograph                  |
| CurveButterfly        | Butterfly    | Butterfly curve with exponential    |
| CurveCardioid         | Cardioid     | Heart-shaped r=a(1-cosθ)           |
| CurveCardioidHeart    | Cardioid     | Upright heart r=a(1+cosθ)          |
| CurveHeartWave        | Heart Wave   | x^(2/3) envelope with sine ripples |
| CurveSpiral           | Spiral       | Archimedean spiral                  |
| CurveFourier          | Fourier      | Multi-harmonic sum                  |
