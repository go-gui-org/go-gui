// appearance_darwin.m — OS light/dark setting (issue #752).

#import "appearance_darwin.h"

// Declared by the Go side (appearance_darwin.go).
extern void goMetalAppearanceChanged(int dark);

int metalSystemAppearanceDark(void) {
  NSAppearance *appearance = NSApp.effectiveAppearance;
  NSAppearanceName match = [appearance
      bestMatchFromAppearancesWithNames:@[
        NSAppearanceNameAqua, NSAppearanceNameDarkAqua
      ]];
  return [match isEqualToString:NSAppearanceNameDarkAqua] ? 1 : 0;
}

// AppearanceObserver forwards effectiveAppearance KVO changes to Go.
@interface GoGuiAppearanceObserver : NSObject
@end

@implementation GoGuiAppearanceObserver

- (void)observeValueForKeyPath:(NSString *)keyPath
                      ofObject:(id)object
                        change:(NSDictionary *)change
                       context:(void *)context {
  (void)keyPath;
  (void)object;
  (void)change;
  (void)context;
  goMetalAppearanceChanged(metalSystemAppearanceDark());
}

@end

static GoGuiAppearanceObserver *gAppearanceObserver = nil;

void metalAppearanceWatchStart(void) {
  if (gAppearanceObserver != nil) {
    return;
  }
  gAppearanceObserver = [[GoGuiAppearanceObserver alloc] init];
  [NSApp addObserver:gAppearanceObserver
          forKeyPath:@"effectiveAppearance"
             options:0
             context:nil];
}

void metalAppearanceWatchStop(void) {
  if (gAppearanceObserver == nil) {
    return;
  }
  [NSApp removeObserver:gAppearanceObserver
             forKeyPath:@"effectiveAppearance"];
  gAppearanceObserver = nil;
}
