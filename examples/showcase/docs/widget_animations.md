Tween, spring, and keyframe animation types. All implement
the `Animation` interface and are managed via `w.AnimationAdd`,
`w.AnimationRemove`, and `w.HasAnimation`. The animation loop ticks
at ~60 fps (16ms). Each animation receives an `OnValue` callback
with the interpolated value and an optional `OnDone` callback.

## Tween

Interpolates from A to B over a fixed duration with easing.
Default: 300ms, EaseOutCubic.

```go
a := gui.NewTweenAnimation("slide", 0, 200,
    func(v float32, w *gui.Window) {
        gui.State[App](w).Offset = v
    })
a.Duration = 500 * time.Millisecond
a.Easing = gui.EaseInOutCubic
w.AnimationAdd(a)
```

## TweenAnimation Properties

| Property | Type          | Description                          |
|----------|---------------|--------------------------------------|
| AnimID   | string        | Unique animation identifier          |
| Duration | time.Duration | Animation length (default 300ms)     |
| Easing   | EasingFn      | Easing function (default EaseOutCubic) |
| From     | float32       | Start value                          |
| To       | float32       | End value                            |
| OnValue  | func(float32, *Window) | Called each tick with current value |
| OnDone   | func(*Window) | Called when animation completes       |

## Spring

Physics-based spring motion. Natural feel for interactive
elements — no fixed duration, settles based on physics.

```go
a := gui.NewSpringAnimation("bounce",
    func(v float32, w *gui.Window) {
        gui.State[App](w).Scale = v
    })
a.Config = gui.SpringBouncy
a.SpringTo(0, 1)
w.AnimationAdd(a)
```

## SpringAnimation Properties

| Property | Type          | Description                          |
|----------|---------------|--------------------------------------|
| AnimID   | string        | Unique animation identifier          |
| Config   | SpringCfg     | Spring physics parameters            |
| OnValue  | func(float32, *Window) | Called each tick              |
| OnDone   | func(*Window) | Called when spring comes to rest      |

## Spring Presets

| Preset        | Stiffness | Damping | Character              |
|---------------|-----------|---------|------------------------|
| SpringDefault | 100       | 10      | General purpose        |
| SpringGentle  | 50        | 8       | Soft, slow             |
| SpringBouncy  | 300       | 15      | Energetic, overshoots  |
| SpringStiff   | 500       | 30      | Fast, minimal bounce   |

## Keyframes

Multi-waypoint interpolation with per-segment easing.
Default: 500ms duration.

```go
a := gui.NewKeyframeAnimation("pulse",
    []gui.Keyframe{
        {At: 0.0, Value: 1.0, Easing: gui.EaseLinear},
        {At: 0.5, Value: 1.5, Easing: gui.EaseOutCubic},
        {At: 1.0, Value: 1.0, Easing: gui.EaseInCubic},
    },
    func(v float32, w *gui.Window) {
        gui.State[App](w).Scale = v
    })
a.Repeat = true
w.AnimationAdd(a)
```

## KeyframeAnimation Properties

| Property  | Type          | Description                          |
|-----------|---------------|--------------------------------------|
| AnimID    | string        | Unique animation identifier          |
| Duration  | time.Duration | Total animation length (default 500ms) |
| Keyframes | []Keyframe    | Waypoints with position and easing   |
| Repeat    | bool          | Loop continuously                    |
| OnValue   | func(float32, *Window) | Called each tick              |
| OnDone    | func(*Window) | Called when animation completes       |

## Keyframe

| Field  | Type     | Description                          |
|--------|----------|--------------------------------------|
| At     | float32  | Position 0.0-1.0                     |
| Value  | float32  | Value at this waypoint               |
| Easing | EasingFn | Easing TO this keyframe              |

## Easing Functions

| Function       | Character                            |
|----------------|--------------------------------------|
| EaseLinear     | Constant speed                       |
| EaseInQuad     | Slow start (quadratic)               |
| EaseOutQuad    | Slow end (quadratic)                 |
| EaseInOutQuad  | Slow start and end (quadratic)       |
| EaseInCubic    | Slow start (cubic)                   |
| EaseOutCubic   | Slow end (cubic, default tween)      |
| EaseInOutCubic | Slow start and end (cubic)           |
| EaseInBack     | Pulls back before accelerating       |
| EaseOutBack    | Overshoots then settles              |
| EaseOutElastic | Oscillates like a spring             |
| EaseOutBounce  | Bouncing ball                        |

## Window Animation API

| Method                | Description                          |
|-----------------------|--------------------------------------|
| w.AnimationAdd(a)     | Register or replace animation by ID  |
| w.AnimationRemove(id) | Stop and remove animation            |
| w.HasAnimation(id)    | Check if animation is active         |
