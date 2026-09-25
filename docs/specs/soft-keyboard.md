# Soft keyboard

Issue #770. Status: implemented for Android and web. iOS and desktop implement
the methods as no-ops.

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
  `EventSoftKeyboard`, clamped to `[0, window height]`. The framework moves no
  layout for it.
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

| Platform | Kind                                                         | Inset                                           |
| -------- | ------------------------------------------------------------ | ----------------------------------------------- |
| Android  | `EditorInfo.inputType` via `PendingIMEKeyboardKind`; restart | `WindowInsets.Type.ime()`, API 30+; 0 below     |
| Web      | hidden input `inputmode` and `type=password`                 | `visualViewport` resize: `innerHeight` − bottom |
| iOS      | no-op (backend has no text input; separate issue)            | none                                            |
| Desktop  | no-op                                                        | none                                            |

The Android host sets `windowSoftInputMode="adjustNothing"`, so the GL surface
keeps its size and the app decides what to move.

## Rejected Approaches

- **Imperative keyboard object** (`w.Keyboard().Show(kind)`, called from
  `OnFocus`). It duplicates the edit-context gate, lets an app show a keyboard
  with no text target, and every app would have to wire it.
- **Kind on the focus call** (`SetFocusInput(id, kind)`). Click, Tab and
  accessibility focus never pass a kind, so the wrong keyboard shows on every
  focus that is not programmatic.
- **A built-in general soft-keyboard widget.** Non-goal of the issue. A
  multilingual keyboard is a product of its own; apps draw specialized keypads
  with ordinary widgets.
- **Auto-scroll or auto-padding for the keyboard in core layout.** That is
  mobile layout policy. The framework reports the inset only.
- **`KeyboardPassword` as a kind.** It would duplicate `IsPassword`; the secure
  flag is derived from it.
- **`SoftKeyboardVisible()`.** `SoftKeyboardInset() > 0` says the same thing
  from one source.
- **An opt-in `KeepFocus` Cfg field for keypad keys.** Rejected for the behavior
  change above: it matches native toolkits and adds no API.
