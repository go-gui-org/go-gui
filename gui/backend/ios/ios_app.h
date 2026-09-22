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

#endif
