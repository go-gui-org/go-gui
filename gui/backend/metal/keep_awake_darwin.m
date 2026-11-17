// keep_awake_darwin.m — keeping the display on (issue #1172).
//
// NSActivityIdleDisplaySleepDisabled keeps the display on, and with it
// the Mac out of idle sleep. The activity token is the request: ending
// it, or the process exiting, lets the display sleep again. Foundation
// only, so no new framework is linked.

#import <Foundation/Foundation.h>

#import "keep_awake_darwin.h"

// activity is the held token, nil while none is held. Called only from
// the UI thread, so no lock.
static id<NSObject> activity;

int metalKeepAwakeBegin(const char *reason) {
  if (activity != nil) {
    return 1;
  }
  NSString *text = [NSString stringWithUTF8String:reason];
  if (text == nil || text.length == 0) {
    text = @"Keeping the display awake";
  }
  activity = [[NSProcessInfo processInfo]
      beginActivityWithOptions:NSActivityIdleDisplaySleepDisabled
                        reason:text];
  return activity != nil ? 1 : 0;
}

void metalKeepAwakeEnd(void) {
  if (activity == nil) {
    return;
  }
  [[NSProcessInfo processInfo] endActivity:activity];
  activity = nil;
}
