#ifndef GOGUI_APPEARANCE_H
#define GOGUI_APPEARANCE_H

// appearance_darwin.h — OS light/dark setting for the Metal backend
// (issue #752). Query plus a KVO watcher on NSApp.effectiveAppearance.

#ifdef __OBJC__
#import <Cocoa/Cocoa.h>
#endif

// metalSystemAppearanceDark reports the current OS setting: 1 for
// dark, 0 for light. Reads NSApp.effectiveAppearance, so a per-app
// NSAppearance override is honored the way the titlebar is.
int metalSystemAppearanceDark(void);

// metalAppearanceWatchStart installs the effectiveAppearance KVO
// observer; a no-op when already installed. Changes arrive in Go as
// goMetalAppearanceChanged (declared by the Go side). Main-thread
// only, like every other NSApp call — the Go side calls both from
// the frame thread.
void metalAppearanceWatchStart(void);

// metalAppearanceWatchStop removes the observer; a no-op when not
// installed. Main-thread only.
void metalAppearanceWatchStop(void);

#endif
