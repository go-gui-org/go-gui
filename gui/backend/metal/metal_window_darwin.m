// metal_window.m — Native NSWindow + CAMetalLayer window manager.
// Replaces SDL2 window creation, event loop, clipboard, cursors, and IME.

#import "metal_window.h"
#import <Cocoa/Cocoa.h>
#import <Metal/Metal.h>

#import <QuartzCore/CAMetalLayer.h>
#include <string.h>
#include <stdlib.h>
#include <math.h>

// Scale applied to AppKit precise (trackpad / high-res) scrolling
// deltas before they reach the shared gui ScrollMultiplier (20).
// AppKit reports precise deltas in points; combined with the gui
// multiplier this factor lands trackpad motion at ~1.5x raw points,
// close to a 1:1 feel. NOTE: this value is coupled to gui's
// ScrollMultiplier — it exists to divide most of that gain back out
// for the precise path, which (unlike the mouse wheel) is not
// smoothed on the gui side. See go-gui-org/go-gui#22.
static const float kPreciseScrollScale = 0.075f;

// Scale applied to AppKit line-based (discrete mouse wheel) scrolling
// deltas. A notch reports ~1 line; combined with gui's ScrollMultiplier
// (20) this lands one notch at ~50px, which the gui side then eases
// into place (exponential smoothing). Was 10.0 (net ~200px/notch),
// which felt far too fast and abrupt.
static const float kWheelScrollScale = 2.5f;

// Event mask covering all event types the app needs to receive.
// MouseEntered/Exited are required by AppKit's internal tracking-
// area system — filtering them out prevents traffic-light button
// hover effects and cursor-rect transitions from firing.
// Periodic is needed for tracking-loop timers (window resize).
static const NSEventMask ALL_EVENTS =
    NSEventMaskLeftMouseDown | NSEventMaskLeftMouseUp |
    NSEventMaskRightMouseDown | NSEventMaskRightMouseUp |
    NSEventMaskOtherMouseDown | NSEventMaskOtherMouseUp |
    NSEventMaskMouseMoved |
    NSEventMaskLeftMouseDragged | NSEventMaskRightMouseDragged |
    NSEventMaskOtherMouseDragged |
    NSEventMaskMouseEntered | NSEventMaskMouseExited |
    NSEventMaskScrollWheel |
    NSEventMaskKeyDown | NSEventMaskKeyUp |
    NSEventMaskFlagsChanged |
    NSEventMaskAppKitDefined | NSEventMaskSystemDefined |
    NSEventMaskApplicationDefined |
    NSEventMaskPeriodic |
    NSEventMaskGesture |
    NSEventMaskMagnify | NSEventMaskRotate | NSEventMaskSwipe |
    NSEventMaskBeginGesture | NSEventMaskEndGesture |
    NSEventMaskCursorUpdate |
    NSEventMaskTabletPoint | NSEventMaskTabletProximity;

// ─── Static event state (read by Go via accessors) ─────────────

static int             _evType;
static unsigned int    _evWindowID;
static float           _evMouseX, _evMouseY;
static float           _evMouseDX, _evMouseDY;
static int             _evMouseButton;
static int             _evClickCount;
static float           _evScrollX, _evScrollY;
static int             _evScrollPhase;
static int             _evScrollPrecise;  // 1 if hasPreciseScrollingDeltas
static unsigned short  _evKeyCode;
static unsigned int    _evModifiers;
static int             _evKeyRepeat;
static char           *_evText;           // malloc'd, freed on next event
static int             _evIMEStart;
static int             _evIMELength;
static int             _evIMEGeneration;   // incremented on each IME event
static int             _evIMEConsumedGen;  // last generation consumed by poll
static int             _quitRequested;     // set by quit:/appShouldTerminate: (main thread only)

// ─── Window ID counter ─────────────────────────────────────────

static uint32_t _nextWindowID = 1;

// ─── MetalContentView ──────────────────────────────────────────

@interface MetalContentView : NSView <NSTextInputClient>
@property (nonatomic, weak) id<MTLDevice> metalDevice;
@property (nonatomic, assign) uint32_t    windowID;
@property (nonatomic, assign) BOOL        imeActive;
@property (nonatomic, assign) NSRect      imeCursorRect;
@property (nonatomic, copy)   NSAttributedString *markedText;
@property (nonatomic, assign) NSRange     markedRange;
@end

@implementation MetalContentView

- (instancetype)initWithFrame:(NSRect)frame
                       device:(id<MTLDevice>)device
                     windowID:(uint32_t)windowID {
    // Create the CAMetalLayer explicitly and assign it as the
    // backing layer. Using layerClass is unreliable because
    // AppKit may create a default CALayer before our override
    // takes effect if wantsLayer is accessed in a particular order.
    CAMetalLayer *metalLayer = [CAMetalLayer layer];
    metalLayer.device = device;
    metalLayer.pixelFormat = MTLPixelFormatBGRA8Unorm;
    metalLayer.framebufferOnly = NO; // allow readback for filters
    // Eliminate content shift during live window resize.
    metalLayer.presentsWithTransaction = YES;

    self = [super initWithFrame:frame];
    if (self) {
        _metalDevice = device;
        _windowID = windowID;
        _imeActive = NO;
        _imeCursorRect = NSZeroRect;
        _markedRange = NSMakeRange(NSNotFound, 0);

        self.wantsLayer = YES;
        self.layer = metalLayer;
    }
    return self;
}

// Accept first responder so we receive key events.
- (BOOL)acceptsFirstResponder {
    return YES;
}

// Participate in AppKit's cursor-rect system so the window server
// can properly manage cursors for the window frame (resize edges,
// title-bar buttons).  Without at least one cursor rect, AppKit
// cursor-tracking is effectively dead — even for the frame region.
// Go overrides the content cursor via metalWindowSetCursor when
// widgets request a different shape.
- (void)resetCursorRects {
    [self addCursorRect:self.bounds cursor:[NSCursor arrowCursor]];
}

// Explicitly invalidate cursor rects when added to a window.
// AppKit may not call resetCursorRects automatically during
// setContentView:, which would leave cursor tracking dormant.
- (void)viewDidMoveToWindow {
    [super viewDidMoveToWindow];
    [self.window invalidateCursorRectsForView:self];
}

// Route all key events through the input method system so
// insertText: fires for printable characters. Non-printable
// keys (arrows, etc.) return to Go as EventKeyDown.
- (void)keyDown:(NSEvent *)event {
    [self interpretKeyEvents:@[event]];
}

- (void)keyUp:(NSEvent *)event {
    // Handled by Go via metalPollEvent.
}

- (void)flagsChanged:(NSEvent *)event {
    // Handled by Go.
}

// Suppress system beep for unhandled key commands (arrows, etc.).
// Go handles all key events through the event loop.
- (void)doCommandBySelector:(SEL)selector {
    // Intentionally empty — no beep.
}

// ─── NSTextInputClient (IME) ───────────────────────────────────

- (void)insertText:(id)string replacementRange:(NSRange)replacementRange {
    // Free previous event text.
    if (_evText) { free(_evText); _evText = NULL; }

    if ([string isKindOfClass:[NSAttributedString class]]) {
        _evText = strdup([[(NSAttributedString *)string string] UTF8String]);
    } else if ([string isKindOfClass:[NSString class]]) {
        _evText = strdup([(NSString *)string UTF8String]);
    }
    _evType = METAL_EVENT_CHAR;
    _evWindowID = _windowID;
    _evIMEGeneration++; // new IME text to deliver

    [self unmarkText];
}

- (void)setMarkedText:(id)string
        selectedRange:(NSRange)selectedRange
     replacementRange:(NSRange)replacementRange {
    if (_evText) { free(_evText); _evText = NULL; }

    if ([string isKindOfClass:[NSAttributedString class]]) {
        _markedText = [(NSAttributedString *)string copy];
    } else if ([string isKindOfClass:[NSString class]]) {
        _markedText = [[NSAttributedString alloc] initWithString:(NSString *)string];
    }

    if (_markedText && [_markedText length] > 0) {
        _evText = strdup([[_markedText string] UTF8String]);
        _markedRange = NSMakeRange(0, [_markedText length]);
        _evIMEStart = (int)selectedRange.location;
        _evIMELength = (int)selectedRange.length;
        _evType = METAL_EVENT_IME_COMP;
        _evWindowID = _windowID;
        _evIMEGeneration++; // new IME composition to deliver
    } else {
        _markedText = nil;
        _markedRange = NSMakeRange(NSNotFound, 0);
    }
}

- (void)unmarkText {
    _markedText = nil;
    _markedRange = NSMakeRange(NSNotFound, 0);
}

- (BOOL)hasMarkedText {
    return _markedText != nil;
}

- (NSRange)markedRange {
    return _markedRange;
}

- (NSRange)selectedRange {
    return NSMakeRange(NSNotFound, 0);
}

- (NSAttributedString *)attributedSubstringForProposedRange:(NSRange)range
                                                actualRange:(NSRangePointer)actualRange {
    return nil;
}

- (NSRect)firstRectForCharacterRange:(NSRange)range
                         actualRange:(NSRangePointer)actualRange {
    // Convert IME cursor rect to screen coordinates for CJK candidate window.
    NSWindow *window = self.window;
    if (!window) return NSZeroRect;

    NSRect screenRect = [window convertRectToScreen:_imeCursorRect];
    return screenRect;
}

- (NSUInteger)characterIndexForPoint:(NSPoint)point {
    return 0;
}

- (NSArray<NSAttributedStringKey> *)validAttributesForMarkedText {
    return @[];
}

@end

// ─── GUIApplication (NSApplication subclass) ────────────────────
// Exists so [GUIApplication sharedApplication] returns an instance
// of this class rather than plain NSApplication. This matches
// SDL2's SDLApplication pattern — any code that checks
// isKindOfClass: or swizzles on the application class will see
// GUIApplication. The sendEvent: override is intentionally a
// passthrough; custom event routing happens in metalPollEvent.

@interface GUIApplication : NSApplication
@end
@implementation GUIApplication
- (void)sendEvent:(NSEvent *)event {
    [super sendEvent:event];
}
@end

// ─── GUIWindow (NSWindow subclass) ────────────────────────────

@interface GUIWindow : NSWindow <NSWindowDelegate, NSDraggingDestination>
@property (nonatomic, assign) uint32_t  windowID;
@property (nonatomic, assign) BOOL      closed;
@end

@implementation GUIWindow

- (instancetype)initWithContentRect:(NSRect)contentRect
                          styleMask:(NSWindowStyleMask)style
                            backing:(NSBackingStoreType)backing
                              defer:(BOOL)defer
                           windowID:(uint32_t)windowID {
    self = [super initWithContentRect:contentRect
                            styleMask:style
                              backing:backing
                                defer:defer];
    if (self) {
        _windowID = windowID;
        _closed = NO;
        self.delegate = self;
        // Use the property setter, not a getter override.
        // acceptsMouseMovedEvents is a stored ivar in NSWindow;
        // the setter tells AppKit to post NSEventTypeMouseMoved
        // events even when no button is pressed.
        self.acceptsMouseMovedEvents = YES;
        [self registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
    }
    return self;
}

- (BOOL)canBecomeKeyWindow { return YES; }
- (BOOL)canBecomeMainWindow { return YES; }

// ─── NSWindowDelegate ──────────────────────────────────────────

- (void)windowDidResize:(NSNotification *)notification {
    // Report content-bounds size, not frame size. The frame
    // includes title bar and window borders (~28pt extra height),
    // which would cause the Go layout to allocate space for
    // widgets that cannot be rendered.
    NSRect bounds = self.contentView.bounds;
    goMetalWindowResized(_windowID,
                         (int)bounds.size.width,
                         (int)bounds.size.height);
}

- (BOOL)windowShouldClose:(NSWindow *)sender {
    goMetalWindowShouldClose(_windowID);
    return NO; // Go decides when to destroy
}

- (void)windowDidBecomeKey:(NSNotification *)notification {
    goMetalWindowFocusChanged(_windowID, 1);
}

- (void)windowDidResignKey:(NSNotification *)notification {
    goMetalWindowFocusChanged(_windowID, 0);
}

// ─── NSDraggingDestination (file drop) ─────────────────────────

- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {
    if ([sender.draggingPasteboard canReadObjectForClasses:@[[NSURL class]]
                                                   options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}]) {
        return NSDragOperationCopy;
    }
    return NSDragOperationNone;
}

- (BOOL)prepareForDragOperation:(id<NSDraggingInfo>)sender {
    return YES;
}

- (BOOL)performDragOperation:(id<NSDraggingInfo>)sender {
    NSArray<NSURL *> *urls = [sender.draggingPasteboard
        readObjectsForClasses:@[[NSURL class]]
                      options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
    if (!urls || urls.count == 0) return NO;
    for (NSURL *url in urls) {
        if (![url isFileURL]) continue;
        const char *utf8 = [[url path] UTF8String];
        if (utf8) goMetalFileDrop(_windowID, (char *)utf8);
    }
    return YES;
}

@end

// ─── GoGuiWindow struct (C-compatible) ─────────────────────────

typedef struct {
    GUIWindow          *nsWindow;
    MetalContentView   *contentView;
    NSVisualEffectView *effectView;  // vibrancy backdrop; nil when opaque
    uint32_t            windowID;
} GoGuiWindow;

// ─── Lifecycle ─────────────────────────────────────────────────

GoGuiNSWindow metalWindowCreate(const char *title, int width, int height,
                                int fixedSize) {
    id<MTLDevice> device = MTLCreateSystemDefaultDevice();
    if (!device) return NULL;

    // Clamp to sensible minimums. AppKit may reject zero/negative
    // content rects or produce invisible windows.
    if (width < 1)  width = 1;
    if (height < 1) height = 1;

    NSWindowStyleMask style = NSWindowStyleMaskTitled
                            | NSWindowStyleMaskClosable
                            | NSWindowStyleMaskMiniaturizable;
    if (!fixedSize) {
        style |= NSWindowStyleMaskResizable;
    }

    uint32_t wid = _nextWindowID++;

    NSRect contentRect = NSMakeRect(0, 0, (CGFloat)width, (CGFloat)height);
    GUIWindow *win = [[GUIWindow alloc]
        initWithContentRect:contentRect
                  styleMask:style
                    backing:NSBackingStoreBuffered
                      defer:NO
                   windowID:wid];
    if (!win) return NULL;

    NSString *nsTitle = nil;
    if (title) {
        nsTitle = [NSString stringWithUTF8String:title];
    }
    if (nsTitle) {
        [win setTitle:nsTitle];
    }
    [win center];

    // Create Metal content view.
    MetalContentView *contentView =
        [[MetalContentView alloc] initWithFrame:contentRect
                                         device:device
                                       windowID:wid];
    if (!contentView) {
        [win close];
        return NULL;
    }
    contentView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [win setContentView:contentView];

    [win setCollectionBehavior:NSWindowCollectionBehaviorMoveToActiveSpace];
    [win makeKeyAndOrderFront:nil];

    // Tell AppKit to update window state — required for cursor
    // tracking and frame management to initialize on windows
    // created before the run loop is running.
    [NSApp setWindowsNeedUpdate:YES];

    // Bring the app forward for dynamically-opened windows.
    // For initial windows, metalAppFinishLaunch is also called from Go
    // after all windows are on screen.
    metalAppFinishLaunch();

    // Allocate GoGuiWindow on heap.
    GoGuiWindow *gw = (GoGuiWindow *)calloc(1, sizeof(GoGuiWindow));
    if (!gw) {
        [win close];
        return NULL;
    }
    gw->nsWindow = win;
    gw->contentView = contentView;
    gw->windowID = wid;
    return (GoGuiNSWindow)gw;
}

void metalWindowDestroy(GoGuiNSWindow w) {
    if (!w) return;
    GoGuiWindow *gw = (GoGuiWindow *)w;
    gw->nsWindow.delegate = nil;
    [gw->nsWindow unregisterDraggedTypes];
    [gw->nsWindow close];
    gw->nsWindow = nil;
    gw->contentView = nil;
    free(gw);
}

// ─── Properties ────────────────────────────────────────────────

void metalWindowSetTitle(GoGuiNSWindow w, const char *title) {
    if (!w || !title) return;
    GoGuiWindow *gw = (GoGuiWindow *)w;
    NSString *nsTitle = [NSString stringWithUTF8String:title];
    if (nsTitle) [gw->nsWindow setTitle:nsTitle];
}

void metalWindowGetSize(GoGuiNSWindow w, int *width, int *height) {
    *width = 0; *height = 0;
    if (!w) return;
    GoGuiWindow *gw = (GoGuiWindow *)w;
    // Content bounds, not frame — matches the coordinate space the Go
    // framework renders into and what goMetalWindowResized reports.
    NSRect bounds = gw->contentView.bounds;
    *width  = (int)bounds.size.width;
    *height = (int)bounds.size.height;
}

void metalWindowGetFramebufferSize(GoGuiNSWindow w, int *width, int *height) {
    *width = 0; *height = 0;
    if (!w) return;
    GoGuiWindow *gw = (GoGuiWindow *)w;
    CAMetalLayer *layer = (CAMetalLayer *)gw->contentView.layer;

    // Compute drawable size from view bounds × screen backing scale.
    // This must be called after the window is displayed so the view
    // has a valid frame and the window is on a screen.
    NSRect bounds = gw->contentView.bounds;
    CGFloat scale = gw->nsWindow.backingScaleFactor;
    if (scale <= 0) scale = 1.0;

    CGSize drawableSize = CGSizeMake(bounds.size.width * scale,
                                     bounds.size.height * scale);
    layer.drawableSize = drawableSize;

    *width  = (int)drawableSize.width;
    *height = (int)drawableSize.height;
}

void *metalWindowGetLayer(GoGuiNSWindow w) {
    if (!w) return NULL;
    GoGuiWindow *gw = (GoGuiWindow *)w;
    return (__bridge void *)gw->contentView.layer;
}

unsigned int metalWindowGetID(GoGuiNSWindow w) {
    if (!w) return 0;
    GoGuiWindow *gw = (GoGuiWindow *)w;
    return gw->windowID;
}

// ─── Vibrancy ──────────────────────────────────────────────────

// Map the cross-platform material enum (see gui.VibrancyMaterial) to an
// NSVisualEffectMaterial. Keep in sync with the Go enum order.
static NSVisualEffectMaterial vibrancyMaterial(int material) {
    switch (material) {
        case 1:  return NSVisualEffectMaterialSidebar;
        case 2:  return NSVisualEffectMaterialMenu;
        case 3:  return NSVisualEffectMaterialHUDWindow;
        case 4:  return NSVisualEffectMaterialUnderWindowBackground;
        default: return NSVisualEffectMaterialUnderWindowBackground;
    }
}

void metalWindowSetVibrancy(GoGuiNSWindow w, int material) {
    if (!w) return;
    GoGuiWindow *gw = (GoGuiWindow *)w;
    CAMetalLayer *layer = (CAMetalLayer *)gw->contentView.layer;

    if (material == 0) {
        // Disable: remove the backdrop and restore an opaque window.
        [gw->effectView removeFromSuperview];
        gw->effectView = nil;
        gw->nsWindow.opaque = YES;
        gw->nsWindow.backgroundColor = [NSColor windowBackgroundColor];
        layer.opaque = YES;
        return;
    }

    // Lazily create the effect view and insert it behind the Metal content
    // view (as a sibling in the window frame). The content view stays the
    // first responder, so key events, resize, and cursor tracking are
    // unaffected.
    if (!gw->effectView) {
        NSVisualEffectView *fx =
            [[NSVisualEffectView alloc] initWithFrame:gw->contentView.frame];
        fx.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
        fx.blendingMode = NSVisualEffectBlendingModeBehindWindow;
        fx.state = NSVisualEffectStateActive;
        [gw->contentView.superview addSubview:fx
                                   positioned:NSWindowBelow
                                   relativeTo:gw->contentView];
        gw->effectView = fx;
    }
    gw->effectView.material = vibrancyMaterial(material);

    // Make the window and drawable non-opaque so the backdrop shows through
    // content cleared with a translucent color (see renderFrame in Go).
    gw->nsWindow.opaque = NO;
    gw->nsWindow.backgroundColor = [NSColor clearColor];
    layer.opaque = NO;
}

// ─── Event callbacks (weak, defined in Go) ─────────────────────

__attribute__((weak)) void goMetalWindowResized(unsigned int wid,
                                                int width, int height) {}
__attribute__((weak)) void goMetalWindowShouldClose(unsigned int wid) {}
__attribute__((weak)) void goMetalWindowFocusChanged(unsigned int wid,
                                                     int focused) {}
__attribute__((weak)) void goMetalFileDrop(unsigned int wid, char *path) {}

// ─── Event polling ─────────────────────────────────────────────

static void storeEvent(NSEvent *event, uint32_t wid) {
    _evType = METAL_EVENT_NONE;
    _evWindowID = wid;

    // Free previous text buffer.
    if (_evText) {
        free(_evText);
        _evText = NULL;
    }

    if (!event) return;

    // Pre-compute flipped Y. NSEvent locationInWindow has origin at
    // bottom-left; Go's framework uses top-left. Flip once here.
    CGFloat winH = 0;
    {
        NSWindow *w = [event window];
        if (w && w.contentView) {
            winH = w.contentView.bounds.size.height;
        }
    }
    CGFloat nsY = (CGFloat)[event locationInWindow].y;
    CGFloat flippedY = winH - nsY;

    NSEventType type = [event type];
    switch (type) {
        case NSEventTypeLeftMouseDown:
        case NSEventTypeRightMouseDown:
        case NSEventTypeOtherMouseDown:
            _evType = METAL_EVENT_MOUSE_DOWN;
            _evMouseX = (float)[event locationInWindow].x;
            _evMouseY = (float)flippedY;
            _evClickCount = (int)[event clickCount];
            _evModifiers = (unsigned int)[event modifierFlags];
            switch (type) {
                case NSEventTypeLeftMouseDown:  _evMouseButton = 0; break;
                case NSEventTypeRightMouseDown: _evMouseButton = 1; break;
                default:                        _evMouseButton = 2; break;
            }
            break;

        case NSEventTypeLeftMouseUp:
        case NSEventTypeRightMouseUp:
        case NSEventTypeOtherMouseUp:
            _evType = METAL_EVENT_MOUSE_UP;
            _evMouseX = (float)[event locationInWindow].x;
            _evMouseY = (float)flippedY;
            _evModifiers = (unsigned int)[event modifierFlags];
            switch (type) {
                case NSEventTypeLeftMouseUp:  _evMouseButton = 0; break;
                case NSEventTypeRightMouseUp: _evMouseButton = 1; break;
                default:                      _evMouseButton = 2; break;
            }
            break;

        case NSEventTypeMouseMoved:
        case NSEventTypeLeftMouseDragged:
        case NSEventTypeRightMouseDragged:
        case NSEventTypeOtherMouseDragged:
            _evType = METAL_EVENT_MOUSE_MOVE;
            _evMouseX = (float)[event locationInWindow].x;
            _evMouseY = (float)flippedY;
            _evMouseDX = (float)[event deltaX];
            _evMouseDY = (float)[event deltaY];
            _evModifiers = (unsigned int)[event modifierFlags];
            break;

        case NSEventTypeScrollWheel:
            _evType = METAL_EVENT_SCROLL_WHEEL;
            _evMouseX = (float)[event locationInWindow].x;
            _evMouseY = (float)flippedY;
            _evModifiers = (unsigned int)[event modifierFlags];
            {
                NSEventPhase phase = [event phase];
                if (phase == NSEventPhaseMayBegin)      _evScrollPhase = 1;
                else if (phase == NSEventPhaseBegan)    _evScrollPhase = 2;
                else if (phase == NSEventPhaseEnded
                      || phase == NSEventPhaseCancelled) _evScrollPhase = 3;
                else                                     _evScrollPhase = 0;
            }
            if ([event hasPreciseScrollingDeltas]) {
                _evScrollPrecise = 1;
                _evScrollX = (float)[event scrollingDeltaX] * kPreciseScrollScale;
                _evScrollY = (float)[event scrollingDeltaY] * kPreciseScrollScale;
            } else {
                // Discrete mouse wheel: line deltas scaled toward pixels.
                _evScrollPrecise = 0;
                _evScrollX = (float)[event scrollingDeltaX] * kWheelScrollScale;
                _evScrollY = (float)[event scrollingDeltaY] * kWheelScrollScale;
            }
            break;

        case NSEventTypeKeyDown:
            _evType = METAL_EVENT_KEY_DOWN;
            _evKeyCode = [event keyCode];
            _evModifiers = (unsigned int)[event modifierFlags];
            _evKeyRepeat = [event isARepeat] ? 1 : 0;
            break;

        case NSEventTypeKeyUp:
            _evType = METAL_EVENT_KEY_UP;
            _evKeyCode = [event keyCode];
            _evModifiers = (unsigned int)[event modifierFlags];
            break;

        case NSEventTypeFlagsChanged:
            _evType = METAL_EVENT_FLAGS_CHANGED;
            _evKeyCode = [event keyCode];
            _evModifiers = (unsigned int)[event modifierFlags];
            break;

        default:
            break;
    }
}

int metalPollEvent(int timeoutMs) {
    // Drain queued events first (non-blocking), then wait if empty.
    NSEvent *event = nil;

    // Quit request from app delegate (quit:, applicationShouldTerminate:).
    // These may fire without a corresponding event in NSDefaultRunLoopMode
    // (menu-bar quit, system logout), so they won't be caught by the
    // normal dequeue below.  Consume the flag here so a vetoed quit
    // does not re-fire on the next poll.
    if (_quitRequested) {
        _quitRequested = 0;
        _evType = METAL_EVENT_QUIT;
        _evWindowID = 0;
        return 1;
    }

    // Check for unconsumed IME events (char / composition) stored
    // by NSTextInputClient callbacks during the previous sendEvent.
    // Uses a generation counter so stale _evType values are not
    // re-delivered across successive poll calls.
    if (_evIMEConsumedGen < _evIMEGeneration) {
        _evIMEConsumedGen = _evIMEGeneration;
        return 1;
    }

    // Non-blocking dequeue.
    event = [NSApp nextEventMatchingMask:ALL_EVENTS
                               untilDate:nil
                                  inMode:NSDefaultRunLoopMode
                                 dequeue:YES];

    // If no pending event and timeout is non-zero, wait.
    if (!event && timeoutMs != 0) {
        NSDate *until = nil;
        if (timeoutMs > 0) {
            until = [NSDate dateWithTimeIntervalSinceNow:timeoutMs / 1000.0];
        } else {
            until = [NSDate distantFuture];
        }
        event = [NSApp nextEventMatchingMask:ALL_EVENTS
                                   untilDate:until
                                      inMode:NSDefaultRunLoopMode
                                     dequeue:YES];
    }

    if (!event) {
        // Check if IME left us an event during the blocking wait
        // (e.g., from a concurrent dispatch). Same generation
        // check as above.
        if (_evIMEConsumedGen < _evIMEGeneration) {
            _evIMEConsumedGen = _evIMEGeneration;
            return 1;
        }
        return 0;
    }

    // Get window ID from the event's window.
    NSWindow *eventWindow = [event window];
    uint32_t wid = 0;
    if ([eventWindow isKindOfClass:[GUIWindow class]]) {
        wid = ((GUIWindow *)eventWindow).windowID;
    }

    // Store event data for Go accessors.
    storeEvent(event, wid);

    // Forward to AppKit for window management and text input.
    // Key events: keyDown: → interpretKeyEvents: → insertText: fires
    // for printable characters, doCommandBySelector: (no-op) for
    // non-printable keys. Go receives EventChar or EventKeyDown.
    // Mouse/scroll: standard AppKit routing (resize, close, focus).
    [NSApp sendEvent:event];

    // If sendEvent triggered IME callbacks that overwrote _evType
    // to CHAR or IME_COMP, mark this generation as consumed so
    // the next poll call doesn't re-deliver it.
    if (_evType == METAL_EVENT_CHAR || _evType == METAL_EVENT_IME_COMP) {
        _evIMEConsumedGen = _evIMEGeneration;
    }

    return 1;
}

// ─── Event accessors ───────────────────────────────────────────

int metalEventType(void) { return _evType; }
unsigned int metalEventWindowID(void) { return _evWindowID; }
float metalEventMouseX(void) { return _evMouseX; }
float metalEventMouseY(void) { return _evMouseY; }
float metalEventMouseDX(void) { return _evMouseDX; }
float metalEventMouseDY(void) { return _evMouseDY; }
int   metalEventMouseButton(void) { return _evMouseButton; }
int   metalEventClickCount(void) { return _evClickCount; }
float metalEventScrollX(void) { return _evScrollX; }
float metalEventScrollY(void) { return _evScrollY; }
int   metalEventScrollPhase(void) { return _evScrollPhase; }
int   metalEventScrollPrecise(void) { return _evScrollPrecise; }
unsigned short metalEventKeyCode(void) { return _evKeyCode; }
unsigned int   metalEventModifiers(void) { return _evModifiers; }
int            metalEventKeyRepeat(void) { return _evKeyRepeat; }

const char *metalEventText(void) {
    return _evText;
}

int metalEventIMEStart(void)  { return _evIMEStart; }
int metalEventIMELength(void) { return _evIMELength; }

// ─── Cursors ───────────────────────────────────────────────────

// Shared bounds guard used by metalWindowSetCursor (production) and
// metalTestCursorBoundsCheck (test helper) so the test always runs
// the exact same logic as production.
static inline bool metalCursorInContentBounds(float mouseX, float mouseY,
                                               float width, float height) {
    if (!isfinite(mouseX) || !isfinite(mouseY)) return false;
    return mouseX >= 0 && mouseX < width && mouseY >= 0 && mouseY < height;
}

void metalWindowSetCursor(GoGuiNSWindow w, const char *cursorName,
                          float mouseX, float mouseY) {
    if (!w || !cursorName) return;
    GoGuiWindow *gw = (GoGuiWindow *)w;

    // Do not set the cursor when the mouse is outside the content
    // view — the window server manages cursors for the frame area
    // (resize edges, title-bar buttons).  Overriding there would
    // prevent edge-resize cursors and traffic-light hover effects.
    NSRect bounds = gw->contentView.bounds;
    if (!metalCursorInContentBounds(mouseX, mouseY,
                                     bounds.size.width, bounds.size.height)) {
        return;
    }

    SEL sel = sel_registerName(cursorName);
    if (sel && [NSCursor respondsToSelector:sel]) {
        NSCursor *cursor = [NSCursor performSelector:sel];
        [cursor set];
    }
}

// ─── Clipboard ─────────────────────────────────────────────────

// Maximum clipboard string size (16 MB). Prevents OOM from
// malicious or corrupted pasteboard content.
static const NSUInteger MAX_CLIPBOARD_BYTES = 16 * 1024 * 1024;

char *metalClipboardGet(void) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSString *s = [pb stringForType:NSPasteboardTypeString];
    if (!s) return NULL;
    // Cap to prevent excessive allocation.
    if ([s lengthOfBytesUsingEncoding:NSUTF8StringEncoding] > MAX_CLIPBOARD_BYTES) {
        return NULL;
    }
    return strdup([s UTF8String]);
}

void metalClipboardSet(const char *text) {
    if (!text) return;
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    [pb setString:[NSString stringWithUTF8String:text]
          forType:NSPasteboardTypeString];
}

// ─── Bridge helper ─────────────────────────────────────────────

void *metalWindowGetNSWindow(GoGuiNSWindow w) {
    if (!w) return NULL;
    GoGuiWindow *gw = (GoGuiWindow *)w;
    return (__bridge void *)gw->nsWindow;
}

// ─── IME ───────────────────────────────────────────────────────

void metalWindowIMESetCursorRect(GoGuiNSWindow win,
                                 float x, float y, float width, float height) {
    if (!win) return;
    GoGuiWindow *gw = (GoGuiWindow *)win;
    gw->contentView.imeCursorRect = NSMakeRect((CGFloat)x, (CGFloat)y,
                                                (CGFloat)width, (CGFloat)height);
}

void metalWindowIMESetActive(GoGuiNSWindow w, int active) {
    if (!w) return;
    GoGuiWindow *gw = (GoGuiWindow *)w;
    gw->contentView.imeActive = (BOOL)active;
    if (active) {
        [gw->nsWindow makeFirstResponder:gw->contentView];
    }
}

// ─── Wake ─────────────────────────────────────────────────────

// Build the dummy application-defined event used to wake or unblock
// the run loop (poll wake and the launch-handshake stop:).
static NSEvent *metalWakeEvent(void) {
    return [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                             location:NSZeroPoint
                        modifierFlags:0
                            timestamp:0
                         windowNumber:0
                              context:nil
                              subtype:0
                                data1:0
                                data2:0];
}

void metalPostEmptyEvent(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp postEvent:metalWakeEvent() atStart:NO];
    });
}

// ─── NSApplication Delegate ────────────────────────────────────

@interface GoGuiAppDelegate : NSObject <NSApplicationDelegate>
@end

@implementation GoGuiAppDelegate

// Sets both _evType (immediate delivery when quit fires during a
// poll's sendEvent:) and _quitRequested (delivery on the next poll
// when quit fires out-of-band, e.g. menu-bar click during tracking
// mode).  Both are necessary: _evType gives same-poll dispatch;
// _quitRequested is the only signal that survives when the quit
// lands outside NSDefaultRunLoopMode.
- (void)quit:(id)sender {
    _evType = METAL_EVENT_QUIT;
    _quitRequested = 1;
}

// Intercept system-initiated termination (logout, shutdown,
// [NSApp terminate:]) so the Go event loop can run its own
// teardown path instead of being SIGKILL'd.  Sets both _evType
// and _quitRequested for same reason as quit: above.
- (NSApplicationTerminateReply)applicationShouldTerminate:
    (NSApplication *)sender {
    _evType = METAL_EVENT_QUIT;
    _quitRequested = 1;
    return NSTerminateCancel;
}

// Return the GUIWindow that should receive focus events, preferring
// the key window, then main, then any visible GUIWindow.
static GUIWindow *metalFocusedGUIWindow(void) {
    NSWindow *keyWindow = [NSApp keyWindow];
    if (keyWindow && [keyWindow isKindOfClass:[GUIWindow class]]) {
        return (GUIWindow *)keyWindow;
    }
    NSWindow *mainWindow = [NSApp mainWindow];
    if (mainWindow && [mainWindow isKindOfClass:[GUIWindow class]]) {
        return (GUIWindow *)mainWindow;
    }
    for (NSWindow *win in [NSApp windows]) {
        if ([win isKindOfClass:[GUIWindow class]] && [win isVisible]) {
            return (GUIWindow *)win;
        }
    }
    return nil;
}

// After switching back from another app (or a system dialog such as
// TCC permissions), applicationDidBecomeActive: fires but
// windowDidBecomeKey: may not — leaving w.focused=false in Go and
// blocking keyboard/left-click input via eventAllowed(). Restore
// focus for the frontmost GUIWindow.
- (void)applicationDidBecomeActive:(NSNotification *)notification {
    GUIWindow *gw = metalFocusedGUIWindow();
    if (!gw) return;
    if ([NSApp keyWindow] != gw) {
        // Window is not key: makeKeyAndOrderFront triggers
        // windowDidBecomeKey:, which delivers EventFocused. Do not
        // also fire it here, or Go's OnEvent sees a duplicate.
        [gw makeKeyAndOrderFront:nil];
        return;
    }
    // Window is already key but Go's focus state may have drifted to
    // false (windowDidBecomeKey: not re-fired on reactivation). This
    // is the only path that re-asserts focus directly.
    goMetalWindowFocusChanged(gw.windowID, 1);
}

// Stops the bootstrap [NSApp run] started by metalAppFinishLaunch so
// the custom event loop can take over.  -stop: only takes effect after
// the next event is dequeued, so post a dummy event to force a return.
// See the App-level section header for the full handshake narrative.
- (void)applicationDidFinishLaunching:(NSNotification *)note {
    [NSApp stop:nil];
    [NSApp postEvent:metalWakeEvent() atStart:YES];
}

@end

// ─── App-level ─────────────────────────────────────────────────
//
// Launch / activation sequence. Three steps, called in order; the
// header (metal_window.h) lists the Go call sites.
//
//   1. metalAppInit  (before any window exists)
//        NSApplication singleton + activation policy + menu bar +
//        delegate.  Force-activates early so Launch Services registers
//        the Dock tile and Cmd+Tab entry before it times out.
//
//   2. metalWindowCreate  (per window)
//        Orders the window front and force-activates again.
//
//   3. metalAppFinishLaunch  (after the first window exists)
//        finishLaunching is deferred to here: a .app bundle only shows
//        windows on the active Space if the notification fires with a
//        window already on screen.  The first call also drives the
//        Launch Services handshake (below); every call force-activates
//        so dynamically-opened windows come forward.
//
// Launch Services handshake: a .app launched via LS receives
// applicationWillFinishLaunching: but NEVER applicationDidFinish-
// Launching: until the run loop processes the kAEOpenApplication
// event.  Until then the app is in launch-limbo (no Cmd+Tab, gray
// titlebar, two-click close) even though -isActive is YES.
// metalAppFinishLaunch runs [NSApp run] once to process that event;
// applicationDidFinishLaunching: then calls [NSApp stop:] (plus a wake
// event, since -stop takes effect only after the next dequeue) so the
// custom event loop can take over.  A bare CLI exec has no handshake;
// -run still calls finishLaunching and returns via the same stop.

// Force the app to the foreground.  [NSApp activate] (macOS 14+) is
// cooperative: it refuses to steal focus from the currently-active
// app (the launching terminal/Finder), so a freshly-launched window
// comes up inactive — gray traffic lights, two-click close, and no
// Cmd+Tab entry until the user clicks the window.  activateIgnoring-
// OtherApps: forces foreground regardless of who is active; it is
// deprecated on 14+ but remains the only reliable launch-time path.
static void metalForceActivate(void) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    [NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
}

void metalAppInit(void) {
    // Idempotent — safe to call multiple times.
    static BOOL activated = NO;
    if (activated) return;
    activated = YES;

    [GUIApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

    NSString *appName = [[NSProcessInfo processInfo] processName];
    if (!appName || [appName length] == 0) {
        appName = @"go-gui";
    }

    // Static — NSMenuItem.target is weak, and NSApp.delegate is
    // weak.  The static keeps the delegate alive.
    static GoGuiAppDelegate *delegate = nil;
    if (!delegate) {
        delegate = [[GoGuiAppDelegate alloc] init];
    }

    NSMenu *bar = [[NSMenu alloc] init];
    NSMenu *appMenu = [[NSMenu alloc] init];

    [appMenu addItemWithTitle:
        [@"About " stringByAppendingString:appName]
                       action:@selector(orderFrontStandardAboutPanel:)
                keyEquivalent:@""];
    [appMenu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *quitItem =
        [appMenu addItemWithTitle:[@"Quit " stringByAppendingString:appName]
                           action:@selector(quit:)
                    keyEquivalent:@"q"];
    [quitItem setKeyEquivalentModifierMask:NSEventModifierFlagCommand];
    [quitItem setTarget:delegate];

    NSMenuItem *appItem = [[NSMenuItem alloc] initWithTitle:appName
                                                     action:nil
                                              keyEquivalent:@""];
    [appItem setSubmenu:appMenu];
    [bar addItem:appItem];
    // ── Window menu ──
    NSMenu *windowMenu = [[NSMenu alloc] init];
    [windowMenu addItemWithTitle:@"Close"
                          action:@selector(performClose:)
                   keyEquivalent:@"w"];
    [windowMenu addItemWithTitle:@"Minimize"
                          action:@selector(performMiniaturize:)
                   keyEquivalent:@"m"];
    [windowMenu addItemWithTitle:@"Zoom"
                          action:@selector(performZoom:)
                   keyEquivalent:@""];
    [windowMenu addItem:[NSMenuItem separatorItem]];
    [windowMenu addItemWithTitle:@"Bring All to Front"
                          action:@selector(arrangeInFront:)
                   keyEquivalent:@""];

    NSMenuItem *windowItem = [[NSMenuItem alloc] initWithTitle:@"Window"
                                                        action:nil
                                                 keyEquivalent:@""];
    [windowItem setSubmenu:windowMenu];
    [bar addItem:windowItem];
    [NSApp setWindowsMenu:windowMenu];
    [NSApp setMainMenu:bar];

    [NSApp setDelegate:delegate];
    // finishLaunching is deferred to metalAppFinishLaunch; activate
    // early so Launch Services registers the app before timing out.
    // See the App-level section header for why.
    metalForceActivate();
}

void metalAppFinishLaunch(void) {
    // Run the Launch Services handshake exactly once (see the App-level
    // section header).  Skipped for a bare CLI exec, which is already
    // finished launching.
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        if (![[NSRunningApplication currentApplication] isFinishedLaunching]) {
            [NSApp run];
        }
    });
    // Not guarded by the dispatch_once: every call must re-activate so
    // newly-created windows come to the foreground.
    metalForceActivate();
}

// ─── Test helpers (called from Go tests via cgo) ─────────────────
//
// These live in the production file, not a separate _test.m, on
// purpose: they reach file-static state — the event globals (_evType,
// _quitRequested, _evIMEGeneration…), metalCursorInContentBounds, and
// metalFocusedGUIWindow. C `static` is translation-unit-local, so a
// split would force exposing those symbols via the header, widening
// the production surface just to relocate test code. Keeping the
// helpers here preserves that encapsulation.

int metalTestActivationPolicyIsRegular(void) {
    return [NSApp activationPolicy] == NSApplicationActivationPolicyRegular ? 1 : 0;
}

int metalTestDelegateIsSet(void) {
    return [NSApp delegate] != nil ? 1 : 0;
}

int metalTestMainMenuExists(void) {
    NSMenu *mainMenu = [NSApp mainMenu];
    return (mainMenu && [mainMenu numberOfItems] > 0) ? 1 : 0;
}

// Shared menu-navigation helper used by metalTestMenuQuitWired,
// metalTestMenuAboutExists, and any future menu-item tests.
static NSMenu *metalTestAppMenu(void) {
    NSMenu *mainMenu = [NSApp mainMenu];
    if (!mainMenu || [mainMenu numberOfItems] == 0) return NULL;
    return [[mainMenu itemAtIndex:0] submenu];
}

int metalTestMenuQuitWired(void) {
    id delegate = [NSApp delegate];
    if (!delegate) return 0;
    NSMenu *am = metalTestAppMenu();
    if (!am) return 0;
    for (NSMenuItem *item in [am itemArray]) {
        if ([item action] == @selector(quit:) &&
            [item target] == delegate) {
            return 1;
        }
    }
    return 0;
}

int metalTestWindowDelegateExists(void *windowHandle) {
    if (!windowHandle) return 0;
    // Dereference the GoGuiWindow struct — first field is nsWindow.
    GoGuiWindow *gw = (GoGuiWindow *)windowHandle;
    if (!gw->nsWindow) return 0;
    return gw->nsWindow.delegate != nil ? 1 : 0;
}

// Inject a synthetic key-down event into the static event globals
// so Go tests can verify mapMetalEvent without a running event loop.
void metalTestInjectKeyDown(unsigned short keyCode, unsigned int modifiers) {
    _evType = METAL_EVENT_KEY_DOWN;
    _evWindowID = 0;
    _evKeyCode = keyCode;
    _evModifiers = modifiers;
    _evKeyRepeat = 0;
}

// Inject a synthetic quit event so Go tests can verify mapMetalEvent
// returns cont=false for METAL_EVENT_QUIT.
void metalTestInjectQuitEvent(void) {
    _evType = METAL_EVENT_QUIT;
    _evWindowID = 0;
}

// Directly invoke -[GoGuiAppDelegate quit:] on the current delegate
// and verify it sets both _evType and _quitRequested.  The
// _quitRequested flag is how metalPollEvent surfaces the quit when
// the triggering event is not in NSDefaultRunLoopMode (menu-bar
// click, system termination); if it is not set the quit is silently
// lost.
int metalTestQuitActionSetsQuitEvent(void) {
    id delegate = [NSApp delegate];
    if (!delegate) return 0;
    _evType = METAL_EVENT_NONE;
    _quitRequested = 0;
    [delegate quit:nil];
    return (_evType == METAL_EVENT_QUIT && _quitRequested == 1) ? 1 : 0;
}

// Directly invoke -applicationShouldTerminate: on the current delegate
// and verify (a) it returns NSTerminateCancel, (b) it sets the quit
// event flag, and (c) it sets _quitRequested so metalPollEvent
// surfaces the quit in NSDefaultRunLoopMode.
int metalTestAppShouldTerminateCorrect(void) {
    id delegate = [NSApp delegate];
    if (!delegate) return 0;
    _evType = METAL_EVENT_NONE;
    _quitRequested = 0;
    NSApplicationTerminateReply reply =
        [delegate applicationShouldTerminate:NSApp];
    return (reply == NSTerminateCancel &&
            _evType == METAL_EVENT_QUIT &&
            _quitRequested == 1) ? 1 : 0;
}

// Verify that metalPollEvent returns 1 when _quitRequested is set
// (before any event dequeue), sets _evType to METAL_EVENT_QUIT, and
// consumes the flag.  Regression test for the out-of-band quit path
// (menu-bar click, system termination) not landing in the event loop.
int metalTestPollReturnsOnQuitRequested(void) {
    _evType = METAL_EVENT_NONE;
    _quitRequested = 1;
    int ret = metalPollEvent(0);
    return (ret == 1 &&
            _evType == METAL_EVENT_QUIT &&
            _quitRequested == 0) ? 1 : 0;
}

// Delegates to the shared metalCursorInContentBounds helper so the
// 10 Go tests exercise the exact same logic as production code.
int metalTestCursorBoundsCheck(float mouseX, float mouseY,
                               float width, float height) {
    return metalCursorInContentBounds(mouseX, mouseY, width, height) ? 0 : 1;
}

int metalTestMenuAboutExists(void) {
    NSMenu *am = metalTestAppMenu();
    if (!am) return 0;
    for (NSMenuItem *item in [am itemArray]) {
        if ([item action] == @selector(orderFrontStandardAboutPanel:)) {
            return 1;
        }
    }
    return 0;
}

// Verify the Windows menu is registered with AppKit.
int metalTestWindowsMenuExists(void) {
    NSMenu *wm = [NSApp windowsMenu];
    return (wm && [wm numberOfItems] > 0) ? 1 : 0;
}

// Verify metalFocusedGUIWindow finds the given window handle.
int metalTestFocusedGUIWindowMatches(void *windowHandle) {
    if (!windowHandle) return 0;
    GoGuiWindow *gw = (GoGuiWindow *)windowHandle;
    GUIWindow *focused = metalFocusedGUIWindow();
    return (focused == gw->nsWindow) ? 1 : 0;
}

// Invoke applicationDidBecomeActive: on the app delegate. Regression
// test for app-switch focus restoration when keyWindow is nil.
int metalTestApplicationDidBecomeActive(void) {
    id delegate = [NSApp delegate];
    if (!delegate) return 0;
    NSNotification *note = [NSNotification notificationWithName:
        NSApplicationDidBecomeActiveNotification object:NSApp];
    [delegate applicationDidBecomeActive:note];
    return 1;
}

void metalSetDockIcon(const void *data, int len) {
    if (!data || len <= 0) return;
    NSData *imgData = [NSData dataWithBytes:data length:(NSUInteger)len];
    NSImage *img = [[NSImage alloc] initWithData:imgData];
    if (img) {
        [NSApp setApplicationIconImage:img];
    }
}
