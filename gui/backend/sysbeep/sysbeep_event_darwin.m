//go:build darwin && !ios

#import <AppKit/AppKit.h>
#include "sysbeep_event_darwin.h"

// NSSound objects are cached by name: soundNamed: reads the file the
// first time, and a UI cue must not touch the disk on every click. The
// cache is only ever reached from the main thread (the event-dispatch
// goroutine is locked to it), so it needs no lock of its own.
static NSMutableDictionary<NSString *, NSSound *> *sysbeepSoundCache(void) {
    static NSMutableDictionary<NSString *, NSSound *> *cache = nil;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        cache = [[NSMutableDictionary alloc] init];
    });
    return cache;
}

// sysbeepNamedSound returns the cached NSSound for name, or nil if the
// system has no such sound — a user can remove one from
// /System/Library/Sounds, and a missing sound is silence, not an error.
static NSSound *sysbeepNamedSound(NSString *name) {
    NSMutableDictionary<NSString *, NSSound *> *cache = sysbeepSoundCache();
    NSSound *sound = cache[name];
    if (sound != nil) {
        return sound;
    }
    sound = [NSSound soundNamed:name];
    if (sound == nil) {
        return nil;
    }
    cache[name] = sound;
    return sound;
}

void sysbeepPlayEvent(const char *name) {
    if (name == NULL) {
        return;
    }
    @autoreleasepool {
        NSString *key = [NSString stringWithUTF8String:name];
        if (key == nil) {
            return;
        }
        NSSound *sound = sysbeepNamedSound(key);
        if (sound == nil) {
            return;
        }
        // Stop before playing: a cached NSSound already playing ignores
        // a second play, which would swallow the second of two quick
        // cues. Restarting is the behaviour a UI sound wants.
        [sound stop];
        [sound play];
    }
}

int sysbeepEventAvailable(void) {
    @autoreleasepool {
        // Tink is the click sound and has shipped in every macOS
        // release; if it cannot be loaded, neither can the rest.
        return sysbeepNamedSound(@"Tink") != nil ? 1 : 0;
    }
}
