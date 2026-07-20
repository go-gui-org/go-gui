//go:build darwin && !ios

#ifndef SYSBEEP_DARWIN_H
#define SYSBEEP_DARWIN_H

// sysbeepPlay plays the user's configured system alert sound.
void sysbeepPlay(void);

#endif
