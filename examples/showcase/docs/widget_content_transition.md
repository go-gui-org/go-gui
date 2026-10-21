Animates a change of a `Text` view's value. Set `TextCfg.Transition`. When the
text changes from one frame to the next, the old text gives way to the new one.
The zero `ContentTransition` animates nothing, so a `Text` without one pays no
cost.

A text with a transition needs a non-empty `ID`. The last text shown is kept by
the effective ID. Without an `ID` nothing animates, and `gui.Debug` reports it
under `DebugMissingIDs`.

## Usage

```go
// A counter: only the digits that change roll.
gui.Text(gui.TextCfg{
    ID:         "count",
    Text:       strconv.Itoa(app.Count),
    Transition: gui.ContentTransition{Kind: gui.ContentTransitionNumeric},
})

// A status word: the old word fades out while the new one fades in.
gui.Text(gui.TextCfg{
    ID:         "status",
    Text:       app.Status,
    Transition: gui.ContentTransition{Kind: gui.ContentTransitionFade},
})

// An icon: the old icon shrinks while the new one grows.
gui.Text(gui.TextCfg{
    ID:         "like-icon",
    Text:       icon,
    TextStyle:  gui.CurrentTheme().TextStyleIconMedium,
    Transition: gui.ContentTransition{Kind: gui.ContentTransitionReplace},
})
```

## Kinds

| Kind                     | What a change does                                      |
| ------------------------ | ------------------------------------------------------- |
| ContentTransitionNone    | The new text shows at once (the zero value)             |
| ContentTransitionFade    | The old text fades out while the new one fades in       |
| ContentTransitionNumeric | Each changed character rolls; unchanged ones stay still |
| ContentTransitionReplace | The old text shrinks away while the new one grows in    |

## Key Properties

| Property | Type                  | Description                           |
| -------- | --------------------- | ------------------------------------- |
| Kind     | ContentTransitionKind | How the old text gives way to the new |
| Down     | bool                  | Numeric: always roll down             |

A transition takes 300 ms.

## Numeric roll

The old and the new text are compared one character at a time, from the right.
So from 41 to 42 only the last digit rolls, and from 99 to 100 all three columns
roll. A larger number comes up from below, and a smaller number comes down from
above. Set `Down` to roll down whatever the values are.

A numeric text asks the font for tabular figures. Where the font has them, all
digits have the same width, so the text does not change width as it counts.

## Behaviors worth knowing

**A frame of a transition does no layout.** The text moves in render-only
frames, so a counter that changes fast costs no layout pass for each frame.

**Reduced motion fades.** When the system asks for reduced motion, every kind
fades.

**Wrapped and multiline texts fade.** A roll or a scale is for one line of text.

**Some texts show the new value at once.** A password, a focused text, a text
with a selection, and a change to the empty text start no transition.

**A change during a transition starts a new one.** It starts from the text that
the old transition was moving to.

**`TextCfg.Anim` waits.** While a transition runs, the opacity of `Anim` still
applies, but its motion, shimmer and reveal start again when the transition
ends.
