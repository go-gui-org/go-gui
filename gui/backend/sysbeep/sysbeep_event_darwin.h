//go:build darwin && !ios

#ifndef SYSBEEP_EVENT_DARWIN_H
#define SYSBEEP_EVENT_DARWIN_H

// sysbeepPlayEvent plays the named system sound. name is one of the
// sounds in /System/Library/Sounds (NSSound soundNamed:). A name the
// system does not have is silent.
void sysbeepPlayEvent(const char *name);

// sysbeepEventAvailable reports whether the system sounds these events
// map onto can be loaded at all.
int sysbeepEventAvailable(void);

#endif
