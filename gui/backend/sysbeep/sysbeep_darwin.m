//go:build darwin && !ios

#import <AppKit/AppKit.h>
#include "sysbeep_darwin.h"

// NSBeep is the AppKit alert sound. It respects the user's selected
// alert sound, the alert volume slider, and the "Play user interface
// sound effects" setting, so a muted Mac stays silent. It is also
// self-coalescing: overlapping calls do not stack into a chorus.
void sysbeepPlay(void) {
    @autoreleasepool {
        NSBeep();
    }
}
