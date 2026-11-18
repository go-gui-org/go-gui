# Soft keyboard

Issue #770. Status: implemented for Android and web (#770) and iOS (#806).
Keyboard avoidance in layout: #1109. Desktop implements the methods as no-ops.

## Decision

The soft keyboard is a property of the focused text field, and it follows the
existing IME edit-context gate (`gui/ime_context.go`, `applyIMEEditContext`). It
adds no second show/hide path.

- `InputCfg.Keyboard KeyboardKind` names the layout. The zero value is
  `KeyboardText`. `KeyboardNone` keeps the OS keyboard down while the field
  keeps focus and its input method (for an app keypad). A password field is
  "secure", from `IsPassword`. `NumericInput` asks for `KeyboardNumber` or
  `KeyboardDecimal`, `InputDate` for `KeyboardNumber`.
- The platform gets `ShowSoftKeyboard(kind, secure)` right after `IMEStart` and
  `HideSoftKeyboard()` right before `IMEStop` (`nativeSoftKeyboard` in
  `gui/native_platform.go`). A kind change on the same field re-requests without
  cycling the input method (#156 invariant).
- A press on the field that already has focus re-requests the keyboard, so a
  keyboard the user dismissed (Android Back) comes back on tap.
- `Window.ShowSoftKeyboard` / `Window.HideSoftKeyboard` repeat or dismiss the
  request for the focused field. Both do nothing without an edit target.
- `Window.SoftKeyboardInset` reports the covered height from
  `EventSoftKeyboard`, clamped to `[0, window height]`. `Window.SafeAreaPadding`
  shrinks its bottom edge by this inset, so the home bar under the keyboard is
  not counted twice (`docs/specs/safe-area.md`).
- A press that a non-focusable widget consumes keeps focus
  (`blurUnclaimedPress`). Inside a `DragScroll` container the pan claim holds
  every press, so the blur and keyboard decisions wait: a pan blurs when it
  starts, and a tap decides on its replayed press. A pan never re-opens the
  keyboard. On web, `IMEStart` no longer focuses the hidden input;
  `ShowSoftKeyboard` sets `inputmode` first and then focuses, so the keyboard
  does not swap.
- An app keypad sends the events a physical key sends, through `QueueCommand` +
  `Window.EventFn`. No text-injection API is added.

## Platforms

| Platform | Kind                                                         | Inset                                                     |
| -------- | ------------------------------------------------------------ | --------------------------------------------------------- |
| Android  | `EditorInfo.inputType` via `PendingIMEKeyboardKind`; restart | `WindowInsets.Type.ime()` over the view, API 30+; 0 below |
| Web      | hidden input `inputmode` and `type=password`                 | `visualViewport` resize: `innerHeight` − bottom           |
| iOS      | `UIKeyInput` traits `keyboardType`, `secureTextEntry`        | `UIKeyboardWillChangeFrame`, docked only                  |
| Desktop  | no-op                                                        | none                                                      |

The Android host sets `windowSoftInputMode="adjustNothing"`, so the GL surface
keeps its size and Go-Gui decides what to move (see below).

## Keyboard avoidance (#1109)

Layout runs in the part of the window above the keyboard, by default. Before
#1109 nothing moved: a field low on the screen and the lower half of a centred
dialog (showcase Dialog → Prompt) sat under the keyboard, and an app could not
fix a built-in dialog.

- `Window.layoutRect` (`gui/soft_keyboard_reveal.go`) is the window less the
  inset. The `FillFill` root is pinned to its height (`updateLayout`), and
  `floatAttachLayout` places floats against it: a dialog centres above the
  keyboard (a dialog taller than the area keeps its top at the top), and an
  auto-flip dropdown opens upward near it. The root clip, pointer tests and the
  frame clear keep the whole window, so the strip under a sliding keyboard shows
  the background.
- The focused field is scrolled into view once per trigger
  (`layoutRevealFocused`): when the inset grows while a field has focus, and
  when focus moves while the keyboard is up. Each vertical scroll ancestor moves
  just enough, inner first, and the walk repeats while an outer move brings a
  skipped inner container into view. Each move is clamped to that container's
  scroll range, so a field with too little content around it stays partly
  covered. The pass runs in `layoutPipeline` after `layoutDisables` and shifts
  the positioned children in the frame it runs in. A focus the focus repair
  moved is revealed in a second pass of the same `FrameFn` (`refreshReveal`).
  The user can scroll away while typing; a shrinking inset reveals nothing.
- `WindowCfg.SoftKeyboardOverlays` opts out: the keyboard covers the view as
  before, and the app uses the inset itself.
- Failure mode: a view taller than the area above the keyboard with no scroll
  container is squeezed or clipped while the keyboard is up. A root that is not
  `FillFill` on height is not pinned, and its content can still sit under the
  keyboard.

### iOS (#806)

The Metal view (`GoGuiView`, `gui/backend/ios/ios_app.m`) adopts `UIKeyInput`.
It is first responder only while a text field is active. With no text field, the
view controller is first responder: it opens no keyboard, and hardware keys
still reach Go. `insertText:` becomes an `EventChar` (a line break becomes
`KeyEnter`), and `deleteBackward` becomes a `KeyBackspace` key-down. Hardware
keys that type no text, and Command or Control chords, come from
`pressesBegan:`. The conversion is in `gui/backend/internal/ioskey`, which has
host tests.

- Hide (`HideSoftKeyboard`, `KeyboardNone`) sets an empty `inputView`. The view
  stays first responder, so a hardware keyboard still types, as with the web
  backend's `inputmode="none"`.
- The four gui calls only record the wanted state. One `dispatch_async` block
  applies the last state, so a move between two fields does not close and open
  the keyboard, and UIKit is never called with the window lock held.
- Autocorrect, autocapitalization, spell check and smart punctuation are off:
  UIKit sees none of the text, so it would rewrite text it cannot see.
- No marked text: CJK conversion needs `UITextInput`, a separate change.
- A floating or undocked iPad keyboard reports inset 0.

## Rejected Approaches

- **iOS: full `UITextInput`** now. Marked text (CJK) and autocorrect need the
  field's text and caret mirrored into UIKit on each change. That is large and
  easy to desync. Kept for a later change.
- **iOS: hidden `UITextField` proxy**, as on web. Two text buffers, and diffing
  the field's text into commits and deletes drifts on paste and autocorrect. Web
  needs a hidden input because the browser gives no other hook; UIKit does.

- **Imperative keyboard object** (`w.Keyboard().Show(kind)`, called from
  `OnFocus`). It duplicates the edit-context gate, lets an app show a keyboard
  with no text target, and every app would have to wire it.
- **Kind on the focus call** (`SetFocusInput(id, kind)`). Click, Tab and
  accessibility focus never pass a kind, so the wrong keyboard shows on every
  focus that is not programmatic.
- **A built-in general soft-keyboard widget.** Non-goal of the issue. A
  multilingual keyboard is a product of its own; apps draw specialized keypads
  with ordinary widgets.
- **Pan the whole frame up by the field's overlap (#1109 D2).** No relayout, but
  the top of the UI goes off-screen while typing, and every coordinate consumer
  (pointer, a11y bounds, IME caret rect, popups) would need the offset.
- **Opt-in helpers only (#1109 D3):** a keyboard-avoiding container and an
  exported scroll-into-view. The default stays broken, and built-in dialogs
  would need it inside anyway. This replaces the #770 rejection of auto-padding
  in core layout as app policy: with `adjustNothing` Go-Gui owns the whole
  surface, so no app got avoidance for free.
- **OS-level resize** (Android `adjustResize`, web
  `interactive-widget=resizes-content`). Enforced edge-to-edge on Android 15
  stops `adjustResize` resizing, it resizes the GL surface during the keyboard
  animation, Safari lacks the web setting, and a reveal step is still needed.
- **Smooth reveal** (`ScrollVerticalToSmooth`). Android streams inset events
  while the keyboard slides, so an ease would chase a moving target and lag the
  keyboard. The jump is shifted into the same frame, and the keyboard's slide
  animates it.
- **Reveal every frame while typing.** Pins the field: the user could not scroll
  to read the rest of a form.
- **Revealing past a container's scroll range.** A container cannot scroll
  further than its content runs; an overscroll would show empty space and break
  every offset clamp. The outer containers in the walk carry the rest when they
  can (PR #1294 review).
- **Deferring the reveal's `OnScroll` out of the frame lock in this change.** It
  fires the way the scroll-anchor and index-scroll passes in the same pipeline
  already fire it. Moving one site alone would make the three disagree; a move
  belongs to all three at once (PR #1294 review).
- **`KeyboardPassword` as a kind.** It would duplicate `IsPassword`; the secure
  flag is derived from it.
- **`SoftKeyboardVisible()`.** `SoftKeyboardInset() > 0` says the same thing
  from one source.
- **An opt-in `KeepFocus` Cfg field for keypad keys.** Rejected for the behavior
  change above: it matches native toolkits and adds no API.
