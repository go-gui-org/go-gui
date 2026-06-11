Touch gesture recognition system.

Recognizes tap, double-tap, long-press, pan, swipe, pinch, and rotate
from raw touch input. Single-touch automatically synthesizes mouse
events for backward compatibility.

## Gesture Types

| Type       | Touches | Trigger                                  |
|------------|---------|------------------------------------------|
| Tap        | 1       | Down + up < 300ms, < 10px movement       |
| DoubleTap  | 1       | Two taps within 300ms                    |
| LongPress  | 1       | Held > 500ms without movement            |
| Pan        | 1       | Down + move > 10px                       |
| Swipe      | 1       | Fast pan ending > 500 px/s               |
| Pinch      | 2       | Two-finger spread/squeeze                |
| Rotate     | 2       | Two-finger twist                         |

## Event Fields

| Field             | Type         | Description                         |
|-------------------|--------------|-------------------------------------|
| OnGesture         | callback     | Receives all gesture events         |
| e.GestureType     | GestureType  | Tap, DoubleTap, LongPress, Pan, .. |
| e.GesturePhase    | GesturePhase | Began, Changed, Ended, Cancelled   |
| e.GestureDX/DY    | float32      | Pan/swipe translation delta         |
| e.PinchScale      | float32      | Cumulative scale (1.0 = unchanged)  |
| e.GestureRotation | float32      | Cumulative rotation in radians      |
| e.VelocityX/Y     | float32      | Swipe velocity in px/s              |
| e.CentroidX/Y     | float32      | Center of active touches            |

Pan gestures automatically scroll containers with `IDScroll > 0`.
Available on `ContainerCfg` and `DrawCanvasCfg`.
Use Chrome DevTools touch emulation or a touchscreen to test.
