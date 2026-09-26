#import <UIKit/UIKit.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#include "ios_app.h"

// ─── GoGuiView ───────────────────────────────────────────────

// GoGuiView is also the text-input responder (issue #806). UIKit sends
// typed text to insertText:, Backspace to deleteBackward and hardware
// keys to pressesBegan:. It is first responder only while a text field
// holds focus in Go (editActive), so a tap elsewhere never opens a
// keyboard.
//
// It adopts UIKeyInput, not UITextInput: Go keeps the text, and UIKit
// sees none of it. So there is no marked text (CJK conversion), and
// autocorrect and the other text services are turned off: they would
// rewrite text they cannot see.

@interface GoGuiView : UIView <UIKeyInput>
@property (nonatomic, assign) BOOL editActive;
// UITextInputTraits. UIKit reads these when the view becomes first
// responder.
@property (nonatomic) UIKeyboardType keyboardType;
@property (nonatomic, getter=isSecureTextEntry) BOOL secureTextEntry;
@property (nonatomic) UITextAutocorrectionType autocorrectionType;
@property (nonatomic) UITextAutocapitalizationType autocapitalizationType;
@property (nonatomic) UITextSpellCheckingType spellCheckingType;
@property (nonatomic) UITextSmartQuotesType smartQuotesType;
@property (nonatomic) UITextSmartDashesType smartDashesType;
@property (nonatomic) UITextSmartInsertDeleteType smartInsertDeleteType;
// Hardware key repeat. UIKit sends pressesBegan: once per press, so
// the view repeats a key Go took while it is held.
@property (nonatomic, strong) NSTimer *repeatTimer;
@property (nonatomic, assign) int repeatUsage;
@property (nonatomic, assign) int repeatFlags;
@property (nonatomic, assign) BOOL repeatEditing;
- (NSSet<UIPress *> *)takePresses:(NSSet<UIPress *> *)presses
                          editing:(BOOL)editing;
- (NSSet<UIPress *> *)releasePresses:(NSSet<UIPress *> *)presses;
@end

// Hardware key repeat timing, near the iOS text-field defaults.
static const NSTimeInterval kKeyRepeatDelay = 0.5;
static const NSTimeInterval kKeyRepeatInterval = 0.05;

@implementation GoGuiView {
    // Replaces the soft keyboard while it is hidden (HideSoftKeyboard,
    // KeyboardNone). An empty input view shows nothing, and the view
    // stays first responder, so a hardware keyboard still types.
    UIView *_inputView;
    // HID usages of the keys that are down and went to Go.
    NSMutableSet<NSNumber *> *_goKeys;
}

+ (Class)layerClass { return [CAMetalLayer class]; }

- (instancetype)initWithFrame:(CGRect)frame {
    self = [super initWithFrame:frame];
    if (self) {
        _autocorrectionType = UITextAutocorrectionTypeNo;
        _autocapitalizationType = UITextAutocapitalizationTypeNone;
        _spellCheckingType = UITextSpellCheckingTypeNo;
        _smartQuotesType = UITextSmartQuotesTypeNo;
        _smartDashesType = UITextSmartDashesTypeNo;
        _smartInsertDeleteType = UITextSmartInsertDeleteTypeNo;
        _goKeys = [NSMutableSet set];
    }
    return self;
}

- (UIView *)inputView { return _inputView; }

- (BOOL)canBecomeFirstResponder { return self.editActive; }

// ─── UIKeyInput ──────────────────────────────────────────────

// Always YES: UIKit cannot see Go's text, and NO would stop Backspace
// on a field Go knows is not empty.
- (BOOL)hasText { return YES; }

- (void)insertText:(NSString *)text {
    goIOSInsertText((char *)text.UTF8String);
}

- (void)deleteBackward {
    goIOSDeleteBackward();
}

// ─── Hardware keys ───────────────────────────────────────────
// Presses reach the first responder: this view while a text field is
// active (editing YES), else the view controller, which hands them here
// with editing NO. The controller is no UIKeyInput, so it opens no
// keyboard, and Tab, arrows and shortcuts still reach Go when a button
// or list holds focus.
//
// While editing, Go takes keys that type no text (arrows, Tab, Escape)
// and Command or Control chords. Every other press goes on to UIKit
// (super), which types it through insertText: or deleteBackward, so
// each character arrives once. With no text field, Go takes every key
// it maps.

- (void)pressesBegan:(NSSet<UIPress *> *)presses
           withEvent:(UIPressesEvent *)event {
    NSSet<UIPress *> *rest = [self takePresses:presses editing:YES];
    if (rest.count > 0) {
        [super pressesBegan:rest withEvent:event];
    }
}

- (void)pressesEnded:(NSSet<UIPress *> *)presses
           withEvent:(UIPressesEvent *)event {
    NSSet<UIPress *> *rest = [self releasePresses:presses];
    if (rest.count > 0) {
        [super pressesEnded:rest withEvent:event];
    }
}

- (void)pressesCancelled:(NSSet<UIPress *> *)presses
               withEvent:(UIPressesEvent *)event {
    NSSet<UIPress *> *rest = [self releasePresses:presses];
    if (rest.count > 0) {
        [super pressesCancelled:rest withEvent:event];
    }
}

// takePresses sends the presses Go takes to Go and returns the rest,
// for the caller to pass to super.
- (NSSet<UIPress *> *)takePresses:(NSSet<UIPress *> *)presses
                          editing:(BOOL)editing {
    NSMutableSet<UIPress *> *rest = [NSMutableSet set];
    for (UIPress *press in presses) {
        UIKey *key = press.key;
        if (key && goIOSKey((int)key.keyCode, (int)key.modifierFlags,
                            1, 0, editing)) {
            [_goKeys addObject:@(key.keyCode)];
            [self startRepeat:(int)key.keyCode
                        flags:(int)key.modifierFlags
                      editing:editing];
        } else {
            [rest addObject:press];
        }
    }
    return rest;
}

// releasePresses sends the key-up of each key whose press Go took and
// returns the rest. A key goes up where it went down: the modifiers can
// differ by now (Cmd lifted before C), and the first responder can have
// changed, so neither decides. Go never sees a key stuck down.
- (NSSet<UIPress *> *)releasePresses:(NSSet<UIPress *> *)presses {
    NSMutableSet<UIPress *> *rest = [NSMutableSet set];
    for (UIPress *press in presses) {
        UIKey *key = press.key;
        if (key && (int)key.keyCode == self.repeatUsage) {
            [self stopRepeat];
        }
        if (key && [_goKeys containsObject:@(key.keyCode)]) {
            [_goKeys removeObject:@(key.keyCode)];
            goIOSKey((int)key.keyCode, (int)key.modifierFlags, 0, 0, 0);
        } else {
            [rest addObject:press];
        }
    }
    return rest;
}

- (void)startRepeat:(int)usage flags:(int)flags editing:(BOOL)editing {
    [self stopRepeat];
    self.repeatUsage = usage;
    self.repeatFlags = flags;
    self.repeatEditing = editing;
    // A weak self: the run loop keeps the timer, and the timer must not
    // keep the view.
    __weak GoGuiView *weakSelf = self;
    self.repeatTimer = [NSTimer
        scheduledTimerWithTimeInterval:kKeyRepeatDelay
                               repeats:NO
                                 block:^(NSTimer *t) {
        GoGuiView *s = weakSelf;
        if (!s) return;
        s.repeatTimer = [NSTimer
            scheduledTimerWithTimeInterval:kKeyRepeatInterval
                                   repeats:YES
                                     block:^(NSTimer *t2) {
            GoGuiView *s2 = weakSelf;
            if (!s2) {
                [t2 invalidate];
                return;
            }
            goIOSKey(s2.repeatUsage, s2.repeatFlags, 1, 1,
                     s2.repeatEditing);
        }];
    }];
}

- (void)stopRepeat {
    [self.repeatTimer invalidate];
    self.repeatTimer = nil;
    self.repeatUsage = 0;
}

// ─── Keyboard state from Go ──────────────────────────────────

// applyKeyboardActive puts the view in the state Go asked for.
//
// UIKit reads keyboardType and secureTextEntry only when the view
// becomes first responder, so a change of either while it is first
// responder resigns and becomes again. A change of only the input view
// (hide or show) needs just reloadInputViews.
- (void)applyKeyboardActive:(BOOL)active
                     hidden:(BOOL)hidden
                       type:(UIKeyboardType)type
                     secure:(BOOL)secure {
    if (!active) {
        self.editActive = NO;
        // The view controller takes over as first responder: the soft
        // keyboard goes down and hardware keys still reach Go. A held
        // key keeps repeating (Tab held walks from a field to a button).
        if (self.isFirstResponder) {
            [self.nextResponder becomeFirstResponder];
        }
        return;
    }
    self.editActive = YES;

    BOOL traitsChanged = self.keyboardType != type ||
                         self.secureTextEntry != secure;
    self.keyboardType = type;
    self.secureTextEntry = secure;

    BOOL wasHidden = _inputView != nil;
    if (hidden && !wasHidden) {
        _inputView = [[UIView alloc] initWithFrame:CGRectZero];
    } else if (!hidden && wasHidden) {
        _inputView = nil;
    }

    if (!self.isFirstResponder) {
        [self becomeFirstResponder];
    } else if (traitsChanged) {
        [self resignFirstResponder];
        [self becomeFirstResponder];
    } else if (hidden != wasHidden) {
        [self reloadInputViews];
    }
}

@end

// gGoGuiView is the Pattern A view. Weak: the view controller owns it.
static __weak GoGuiView *gGoGuiView;

void iosKeyboardQueueApply(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        int active = 0, hidden = 0, type = 0, secure = 0;
        // Take the state even with no view, so the next request can
        // queue a new block.
        goIOSKeyboardTake(&active, &hidden, &type, &secure);
        GoGuiView *v = gGoGuiView;
        if (!v) return;
        [v applyKeyboardActive:active != 0
                        hidden:hidden != 0
                          type:(UIKeyboardType)type
                        secure:secure != 0];
    });
}

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
    gGoGuiView = v;
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
        [[NSNotificationCenter defaultCenter]
            addObserver:self
            selector:@selector(keyboardWillChangeFrame:)
            name:UIKeyboardWillChangeFrameNotification
            object:nil];
        [[NSNotificationCenter defaultCenter]
            addObserver:self
            selector:@selector(keyboardWillHide:)
            name:UIKeyboardWillHideNotification
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

// ─── First responder with no text field (issue #806) ────────
// The controller is first responder while no text field is active, so
// hardware keys reach Go (Tab, arrows, shortcuts on a focused button).
// It adopts no UIKeyInput, so no keyboard opens for it.

- (BOOL)canBecomeFirstResponder { return YES; }

- (void)viewDidAppear:(BOOL)animated {
    [super viewDidAppear:animated];
    GoGuiView *v = (GoGuiView *)self.view;
    if (!v.isFirstResponder) {
        [self becomeFirstResponder];
    }
}

- (void)pressesBegan:(NSSet<UIPress *> *)presses
           withEvent:(UIPressesEvent *)event {
    NSSet<UIPress *> *rest =
        [(GoGuiView *)self.view takePresses:presses editing:NO];
    if (rest.count > 0) {
        [super pressesBegan:rest withEvent:event];
    }
}

- (void)pressesEnded:(NSSet<UIPress *> *)presses
           withEvent:(UIPressesEvent *)event {
    NSSet<UIPress *> *rest = [(GoGuiView *)self.view releasePresses:presses];
    if (rest.count > 0) {
        [super pressesEnded:rest withEvent:event];
    }
}

- (void)pressesCancelled:(NSSet<UIPress *> *)presses
               withEvent:(UIPressesEvent *)event {
    NSSet<UIPress *> *rest = [(GoGuiView *)self.view releasePresses:presses];
    if (rest.count > 0) {
        [super pressesCancelled:rest withEvent:event];
    }
}

// ─── Soft keyboard inset (issue #806) ────────────────────────
// Reports how much of the view's bottom the docked keyboard covers. A
// floating or undocked iPad keyboard does not reach the bottom edge and
// reports 0: it covers no fixed band an app could pad for.

- (void)keyboardWillChangeFrame:(NSNotification *)n {
    NSValue *v = n.userInfo[UIKeyboardFrameEndUserInfoKey];
    UIView *view = self.view;
    UIScreen *screen = view.window.screen;
    if (!v || !screen) {
        return;
    }
    // The end frame is in screen coordinates.
    CGRect kb = [view convertRect:v.CGRectValue
              fromCoordinateSpace:screen.coordinateSpace];
    CGRect bounds = view.bounds;
    CGFloat inset = 0;
    if (CGRectGetMaxY(kb) >= CGRectGetMaxY(bounds) - 1 &&
        CGRectIntersectsRect(kb, bounds)) {
        inset = CGRectGetMaxY(bounds) - CGRectGetMinY(kb);
    }
    goIOSSoftKeyboardInset((float)(inset > 0 ? inset : 0));
}

- (void)keyboardWillHide:(NSNotification *)n {
    goIOSSoftKeyboardInset(0);
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
