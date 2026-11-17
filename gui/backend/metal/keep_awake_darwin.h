#ifndef GOGUI_KEEP_AWAKE_H
#define GOGUI_KEEP_AWAKE_H

// keep_awake_darwin.h — keeping the display on (issue #1172), through
// an NSProcessInfo activity that disables idle display sleep.

// metalKeepAwakeBegin begins the activity with reason, a UTF-8 string
// that must not be empty: macOS requires one, and pmset -g assertions
// shows it. A no-op when one is already held. Returns 1 when the
// activity is held, 0 when macOS gave none.
int metalKeepAwakeBegin(const char *reason);

// metalKeepAwakeEnd ends the activity; a no-op when none is held.
void metalKeepAwakeEnd(void);

#endif
