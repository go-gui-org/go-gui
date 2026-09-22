#import <UIKit/UIKit.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#include "ios_app.h"

// ─── GoGuiView ───────────────────────────────────────────────

@interface GoGuiView : UIView
@end

@implementation GoGuiView
+ (Class)layerClass { return [CAMetalLayer class]; }
@end

// ─── GoGuiViewController ─────────────────────────────────────

@interface GoGuiViewController : UIViewController
@property (nonatomic, strong) CADisplayLink *displayLink;
@property (nonatomic, assign) BOOL started;
@end

@implementation GoGuiViewController

- (void)loadView {
    GoGuiView *v = [[GoGuiView alloc] init];
    v.backgroundColor = [UIColor blackColor];
    v.multipleTouchEnabled = YES;
    self.view = v;
}

- (void)viewDidLayoutSubviews {
    [super viewDidLayoutSubviews];
    CGRect bounds = self.view.bounds;
    UIScreen *screen = self.view.window.screen;
    CGFloat scale = screen ? screen.scale : 2.0;

    CAMetalLayer *layer = (CAMetalLayer *)self.view.layer;
    layer.contentsScale = scale;
    layer.drawableSize = CGSizeMake(
        bounds.size.width * scale,
        bounds.size.height * scale);

    if (!self.started) {
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        if (!device) return;

        layer.device = device;
        layer.pixelFormat = MTLPixelFormatBGRA8Unorm;
        layer.framebufferOnly = YES;

        // Seed the appearance before goIOSInit: it attaches the
        // native platform and runs OnInit, and both can query the
        // setting (FollowSystemAppearance). Seeded later, the
        // first query reads the zero value (light) on a dark
        // device, and nothing corrects it until a trait change.
        gIOSAppearanceDark = iosTraitDark(self.traitCollection);

        void *layerPtr = (__bridge void *)layer;
        goIOSInit(layerPtr,
                  (int)bounds.size.width,
                  (int)bounds.size.height,
                  (float)scale);

        self.displayLink = [CADisplayLink
            displayLinkWithTarget:self
            selector:@selector(render:)];
        [self.displayLink addToRunLoop:[NSRunLoop mainRunLoop]
                               forMode:NSRunLoopCommonModes];

        [[NSNotificationCenter defaultCenter]
            addObserver:self
            selector:@selector(appWillResignActive:)
            name:UIApplicationWillResignActiveNotification
            object:nil];
        [[NSNotificationCenter defaultCenter]
            addObserver:self
            selector:@selector(appDidBecomeActive:)
            name:UIApplicationDidBecomeActiveNotification
            object:nil];

        self.started = YES;
    } else {
        goIOSResize((int)bounds.size.width,
                    (int)bounds.size.height,
                    (float)scale);
    }
}

- (void)render:(CADisplayLink *)link {
    goIOSRender();
}

// ─── Touch handling ──────────────────────────────────────────
// Each UITouch is dispatched individually with its pointer as
// identifier. The gesture recognizer in gui/gesture.go tracks
// multi-touch state across these per-finger events.

- (void)touchesBegan:(NSSet<UITouch *> *)touches
           withEvent:(UIEvent *)event {
    for (UITouch *touch in touches) {
        CGPoint loc = [touch locationInView:self.view];
        goIOSTouchEvent(IOS_TOUCH_BEGAN,
            (uintptr_t)touch,
            (float)loc.x, (float)loc.y);
    }
}

- (void)touchesMoved:(NSSet<UITouch *> *)touches
           withEvent:(UIEvent *)event {
    for (UITouch *touch in touches) {
        CGPoint loc = [touch locationInView:self.view];
        goIOSTouchEvent(IOS_TOUCH_MOVED,
            (uintptr_t)touch,
            (float)loc.x, (float)loc.y);
    }
}

- (void)touchesEnded:(NSSet<UITouch *> *)touches
           withEvent:(UIEvent *)event {
    for (UITouch *touch in touches) {
        CGPoint loc = [touch locationInView:self.view];
        goIOSTouchEvent(IOS_TOUCH_ENDED,
            (uintptr_t)touch,
            (float)loc.x, (float)loc.y);
    }
}

- (void)touchesCancelled:(NSSet<UITouch *> *)touches
               withEvent:(UIEvent *)event {
    for (UITouch *touch in touches) {
        CGPoint loc = [touch locationInView:self.view];
        goIOSTouchEvent(IOS_TOUCH_CANCELLED,
            (uintptr_t)touch,
            (float)loc.x, (float)loc.y);
    }
}

- (void)appWillResignActive:(NSNotification *)n {
    self.displayLink.paused = YES;
}

- (void)appDidBecomeActive:(NSNotification *)n {
    self.displayLink.paused = NO;
}

// ─── Appearance (issue #752) ────────────────────────────────
// The current setting, seeded at first layout before goIOSInit and
// refreshed on every trait change. Go reads it through iosAppearanceDark and
// learns of changes through goIOSAppearanceChanged.

static int gIOSAppearanceDark = 0;

static int iosTraitDark(UITraitCollection *traits) {
    return traits.userInterfaceStyle == UIUserInterfaceStyleDark;
}

int iosAppearanceDark(void) {
    return gIOSAppearanceDark;
}

- (void)traitCollectionDidChange:(UITraitCollection *)previous {
    [super traitCollectionDidChange:previous];
    gIOSAppearanceDark = iosTraitDark(self.traitCollection);
    goIOSAppearanceChanged(gIOSAppearanceDark);
}

- (void)dealloc {
    [[NSNotificationCenter defaultCenter] removeObserver:self];
    [self.displayLink invalidate];
}

@end

// ─── GoGuiAppDelegate ────────────────────────────────────────

@interface GoGuiAppDelegate : UIResponder <UIApplicationDelegate>
@property (nonatomic, strong) UIWindow *window;
@end

@implementation GoGuiAppDelegate

- (BOOL)application:(UIApplication *)application
    didFinishLaunchingWithOptions:(NSDictionary *)opts {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    self.window = [[UIWindow alloc]
        initWithFrame:[UIScreen mainScreen].bounds];
#pragma clang diagnostic pop
    self.window.rootViewController =
        [[GoGuiViewController alloc] init];
    [self.window makeKeyAndVisible];
    return YES;
}

@end

// ─── Entry Point ─────────────────────────────────────────────

void iosStartApp(void) {
    @autoreleasepool {
        char *argv[] = {""};
        UIApplicationMain(1, argv, nil,
            NSStringFromClass([GoGuiAppDelegate class]));
    }
}
