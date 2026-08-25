# IME activation is a text-edit context, not focus

Issue #393. Status: implemented.

## Problem

`Window.setFocusLocked` called `NativePlatform.IMEStart()` on every focus change
to any focusable widget — buttons, sliders, listboxes, splitters. The platform
input method was therefore live whenever anything at all held focus.

On macOS that has a visible cost. `MetalContentView.keyDown:` routes every key
through `interpretKeyEvents:`, and the old code treated any resulting preedit as
the input method's claim, so `metalPollEvent` dropped the key-down. On the US
layout Option+I is the circumflex dead key, so `ModAlt|KeyI` never reached Go
and the app's Option shortcut did nothing. Worse, the accent stayed pending and
composed itself into the next keystroke. Option+arrow shortcuts worked only
because arrows produce no preedit.

The other backends already assumed the narrower contract. `ime_win32.go` says
outright that "composition only becomes possible once a text widget takes
focus". On X11 `IMEStart` is an ibus `FocusIn`. On web it creates a hidden
`<input>` and moves DOM focus into it.

## Rule

The platform input method is activated only while an **editable text context**
holds focus: a widget that can commit a composition.

`shapeIsIMEEditTarget` (`gui/ime_context.go`) decides it from the arranged tree,
using the same discriminators `render_text.go` already uses to decide where a
preedit is drawn:

- a text shape whose `focusOwner` is set — only input widgets set it
  (`view_input.go`), so a selection-only focusable `Text` is excluded — and
  whose `TC.textReadOnly` is false, since a read-only input stays focusable for
  its caret but can never commit.
- a focusable term grid, which consumes typed text directly.

`(*Window).syncIMEEditContext` runs from `buildRenderers` and pushes only
transitions, preserving the issue #156 invariant that re-asserting focus on the
widget that already holds it does not cycle the input context.

Moving between two text fields does cycle it. `IMEStop` is what cancels a
composition still live inside the engine — an ibus `FocusOut`, the IMM context
detach, the removal of the web hidden `<input>` — so skipping it lets a
half-typed word commit into the field that just took focus. The activation is
therefore keyed on the focused ID as well as on the boolean.

## Why the render pass, not `setFocusLocked`

An ID alone cannot say whether a widget is editable, and `SetFocus` is
legitimately called from inside a `View` function, where `w.layout` still holds
the previous frame — a newly created input is not in it yet. The arranged tree
in `buildRenderers` is the first place the answer exists. Reporting platform
state from the render pass has precedent: `renderText` reports the caret rect
through `IMESetRect` the same way.

## macOS dead-key rule

`imeSettleAfterKeyWasComposing:` decides who owns a key, in order:

1. a composition already in flight owns everything, arrows and Return included.
2. no preedit — the key is the widget's.
3. a preedit started **with** an edit context focused is real input (a CJK first
   letter, or the first half of Option+e → é) and the key is claimed.
4. a preedit started **without** one is an unwanted dead key: it is discarded
   (`unmarkText`, an empty `METAL_EVENT_IME_COMP` to clear Go's preedit, and
   `[self.inputContext discardMarkedText]` to drop it inside the input source),
   and the key-down goes to the app.

Step 4's discard is what keeps a stray accent from composing into the next
keystroke.

## A key the input source declines

`doCommandBySelector:` is the input source handing a key back: it inspected the
key, declined it as text input, and returned it as a command for the widget.
`_imeDeclinedKey` records that, and it outranks a live composition — step 1
above applies only to keys the input source actually consumed.

Without it, Tab cannot leave a field that has a dead key pending. The observed
callback order for Option+e then Tab is:

```
insertText: ´              the accent commits
doCommandBySelector: insertTab:   the key comes back
```

so `wasComposing` was true and the Tab was dropped, leaving focus stuck in the
field with the accent already typed.

Because the commit is queued before the key comes back, delivering the key
immediately moves focus before the text lands, and the accent is typed into the
widget that just took focus. `metalPollEvent` therefore holds a declined key for
one poll whenever the same keystroke queued text (`_deferredKey`), releasing it
once the queue has drained.

The consequence, deliberate: inside a focused text field macOS keeps native
behavior and an Option+letter shortcut does not fire, where X11 delivers the
key-down and suppresses only the character.

## Tests

- `gui/ime_context_test.go` — the predicate table, that focusing a button starts
  nothing while focusing an input starts exactly once, and that moving between
  two fields cycles the context.
- `gui/ime_test.go: TestIMEStartNotRepeatedOnRefocusSameID` — the #156
  invariant, now driven through the render pass.
- `metalTestContentViewIsFirstResponder` — key delivery no longer depends on the
  IME activation path, which used to install the first responder.
- `metalTestIMEKeySuppressedWhileComposing` — a key the input source consumed
  mid-composition is claimed, one it declined reaches the widget, and an idle
  key is untouched.
- `metalTestIMEDeadKeyDeliveredWithoutTextContext` — the four-way decision
  above, run from the main-thread block in `gui/backend/metal/icon_test.go`
  (`GO_GUI_MAIN_THREAD_TESTS=1`).
