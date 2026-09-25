Semantic loading indicator for AI and agent interfaces. Nine hand-tuned dotted
3D designs name what the agent is doing, so "searching the web" reads
differently from "writing a reply". Ink is monochrome and follows the theme,
with depth shading mirrored on dark grounds. The geometry is a
formula-for-formula port of the MIT ThinkingOrbs engine.

## Usage

```go
gui.ThinkingOrb(gui.ThinkingOrbCfg{
    ID:     "search-status",
    Design: gui.ThinkingOrbSearching,
})
```

## Status Label

```go
gui.ThinkingOrbLabel(gui.ThinkingOrbLabelCfg{
    ID:     "search-label",
    Text:   "Searching the web…",
    Design: gui.ThinkingOrbSearching,
    Size:   gui.ThinkingOrbSmall,
})
```

The label pairs an orb with a shimmering status line. It reads as one
accessibility element: its title.

## Small Inline

```go
gui.ThinkingOrb(gui.ThinkingOrbCfg{
    ID:     "row-status",
    Design: gui.ThinkingOrbConnecting,
    Size:   gui.ThinkingOrbSmall,
})
```

Small is a separate tuned design with fewer, bigger dots and its own tempo, not
a scaled-down regular. Reach for it next to text, in toolbars, and in list rows.

## With Color

```go
gui.ThinkingOrb(gui.ThinkingOrbCfg{
    ID:     "save",
    Design: gui.ThinkingOrbComposing,
    Color:  gui.RGB(46, 160, 67),
})
```

Unset ink is matte gray that follows the theme: dark dots on light themes, light
dots on dark themes. A set color replaces the gray on dots and lines. Its alpha
scales the ink, so a half-transparent color draws half as strong.

## Key Properties

| Property  | Type              | Description                              |
| --------- | ----------------- | ---------------------------------------- |
| ID        | string            | Unique identifier (drives animation)     |
| Design    | ThinkingOrbDesign | Named state constant (9 available)       |
| Size      | ThinkingOrbSize   | Regular 64pt or Small 20pt (def Regular) |
| Width     | float32           | Explicit width (defaults to tuned size)  |
| Height    | float32           | Explicit height (defaults to tuned size) |
| Sizing    | Sizing            | Combined axis sizing mode                |
| MinWidth  | float32           | Minimum width                            |
| MaxWidth  | float32           | Maximum width                            |
| MinHeight | float32           | Minimum height                           |
| MaxHeight | float32           | Maximum height                           |

## Appearance

| Property | Type  | Description                      |
| -------- | ----- | -------------------------------- |
| Color    | Color | Dot ink (default theme gray ink) |

## Animation

| Property | Type    | Description                                 |
| -------- | ------- | ------------------------------------------- |
| Speed    | float32 | Multiplier on the tuned speed (default 1)   |
| Paused   | bool    | Freeze on the current frame (default false) |

Speed, pause state, and design apply live. A change continues from the current
frame, with no jump. Paused holds the current frame. Reduce Motion and headless
captures show one representative frame. Each orb keeps its own clock, so two
orbs that start at different times are not in phase.

## Designs

| Constant              | Reach for it when               |
| --------------------- | ------------------------------- |
| ThinkingOrbWorking    | General-purpose busy            |
| ThinkingOrbSearching  | Web search, retrieval, lookups  |
| ThinkingOrbSolving    | Reasoning, math, code           |
| ThinkingOrbListening  | Voice input, transcription      |
| ThinkingOrbConnecting | Tool calls, APIs, sync          |
| ThinkingOrbWeaving    | Planning, multi-step agents     |
| ThinkingOrbComposing  | Writing a reply                 |
| ThinkingOrbBreathing  | Idle thinking, waiting on model |
| ThinkingOrbShaping    | Design, image, layout work      |

An invalid design clamps to Working. Breathing announces "Thinking…" to
assistive tech; every other design announces its title plus an ellipsis,
overridable with `A11YCfg`.
