#ifndef IOS_APP_H
#define IOS_APP_H

#include <stdint.h>

// Touch phases for goIOSTouchEvent.
enum {
    IOS_TOUCH_BEGAN     = 0,
    IOS_TOUCH_MOVED     = 1,
    IOS_TOUCH_ENDED     = 2,
    IOS_TOUCH_CANCELLED = 3,
};

// Start the UIKit application. Calls UIApplicationMain which
// blocks forever. The UIKit lifecycle calls back into Go via
// the goIOS* exported functions.
void iosStartApp(void);

// Exported Go callbacks — implemented in backend.go.
extern void goIOSInit(void* layer, int w, int h, float scale);
extern void goIOSRender(void);
extern void goIOSResize(int w, int h, float scale);

// Multi-touch callback. phase is one of IOS_TOUCH_* constants.
// identifier uniquely identifies a finger for the duration of
// a touch sequence.
extern void goIOSTouchEvent(int phase, uintptr_t identifier,
    float x, float y);

// OS appearance (issue #752): 1 for dark, 0 for light. Reads the
// last UITraitCollection the view controller saw.
int iosAppearanceDark(void);

// Appearance change callback, implemented in appearance_ios.go.
extern void goIOSAppearanceChanged(int dark);

// ─── Text input and soft keyboard (issue #806) ──────────────
// Implemented in keyboard_ios.go.

// Queue one block on the main queue that reads the wanted keyboard
// state with goIOSKeyboardTake and applies it to the view. Safe to
// call from any thread.
void iosKeyboardQueueApply(void);

// Report the wanted keyboard state. Each out value is 0 or 1, except
// type, which is a UIKeyboardType.
extern void goIOSKeyboardTake(int *active, int *hidden, int *type,
    int *secure);

// UIKeyInput callbacks. text is UTF-8.
extern void goIOSInsertText(char *text);
extern void goIOSDeleteBackward(void);

// Hardware key press (down 1) or release (down 0). usage is a
// UIKeyboardHIDUsage, flags UIKeyModifierFlags. editing is 1 while a
// text field is active. Returns 1 when Go took the key; 0 means pass
// the press on to UIKit.
extern int goIOSKey(int usage, int flags, int down, int repeat,
    int editing);

// Soft keyboard height over the bottom of the view, in points.
extern void goIOSSoftKeyboardInset(float h);

#endif
