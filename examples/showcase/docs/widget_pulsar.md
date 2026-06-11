Animated blinking text indicator for loading states. Alternates
between two text strings synced to the window's input cursor blink.
Defaults to "..." / ".." if no text is provided.

## Usage

```go
gui.Pulsar(gui.PulsarCfg{ID: "p1"}, w)
```

## Custom Text

```go
gui.Pulsar(gui.PulsarCfg{
    ID:    "typing",
    Text1: "Typing...",
    Text2: "Typing..",
    Size:  gui.SomeF(14),
}, w)
```

## Key Properties

| Property | Type    | Description                          |
|----------|---------|--------------------------------------|
| ID       | string  | Unique identifier                    |
| Text1    | string  | Text shown when cursor is on         |
| Text2    | string  | Text shown when cursor is off        |
| Width    | float32 | Fixed width (auto-estimated if 0)    |

## Appearance

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| Color     | Color        | Text color                           |
| Size      | Opt[float32] | Font size                            |
| TextStyle | TextStyle    | Full text style override             |
