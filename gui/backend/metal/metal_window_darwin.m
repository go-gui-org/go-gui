// metal_window.m — Native NSWindow + CAMetalLayer window manager.
// Owns window creation, the event loop, clipboard, cursors, and IME.

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
// deltas, converting them to gui.Event's line unit. AppKit reports ~1
// line per notch (more once acceleration kicks in); the platform
// convention every other backend also reports is three lines per notch,
// so that is the factor. Combined with gui's ScrollMultiplier (20) this
// lands one notch at ~60px, which the gui side then eases into place
// (exponential smoothing). Was 2.5 (~50px/notch) back when the unit was
// undefined and each backend picked its own — Win32 and X11 reported a
// bare 1.0, so macOS scrolled 2.5x faster than either for the same
// gesture. Was 10.0 before that (~200px/notch), which felt far too fast.
static const float kWheelScrollScale = 3.0f;

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
static int             _quitRequested;     // set by quit:/appShouldTerminate: (main thread only)
// Set by windowWillStartLiveResize:, consumed in metalPollEvent.
// AppKit's resize tracking loop runs *inside* [NSApp sendEvent:], so
// this flag tells us — after sendEvent returns — that the mouse-down
// we already stored was swallowed to drive a window resize.
static int             _liveResizeStarted;
// Set by keyDown: when an input method owned the keystroke, consumed in
// metalPollEvent. While a composition is live the IME owns the keyboard
// — arrows move between conversion clauses, Enter commits, Escape
// reverts — so the raw key must not also reach the widget, or the
// field's own caret moves under the preedit.
static int             _imeConsumedKey;

// ─── Text-input event queue (IME) ──────────────────────────────
//
// A single [NSApp sendEvent:] can fire several NSTextInputClient
// callbacks synchronously — for CJK, insertText: (the commit)
// immediately followed by setMarkedText: (a residual composition).
// The _ev* globals above are one slot, so the earlier callback used
// to be overwritten and silently lost.  Queue the text events instead
// and let metalPollEvent hand them to Go one per call.
//
// Main thread only: every writer is an AppKit callback and the only
// reader is metalPollEvent, so no locking is needed.

typedef struct {
    int          type;       // METAL_EVENT_CHAR | METAL_EVENT_IME_COMP
    unsigned int windowID;
    char        *text;       // malloc'd, owned by the entry
    int          imeStart;
    int          imeLength;
    unsigned int modifiers;  // snapshot: _evModifiers moves on before the pop
} TextEvent;

#define TEXT_EVENT_CAP 32

static TextEvent _textQ[TEXT_EVENT_CAP];
static int       _textQHead;   // index of the oldest entry
static int       _textQCount;

// Append a text event.  `utf8` may be NULL or empty (an empty
// composition is how the IME reports "preedit ended").
static void pushTextEvent(int type, const char *utf8, unsigned int wid,
                          int start, int len) {
    // Full queue: drop the oldest entry rather than the new one, so the
    // most recent input state always survives.  Unreachable in practice —
    // the queue drains on every poll and one sendEvent yields a handful.
    if (_textQCount == TEXT_EVENT_CAP) {
        TextEvent *oldest = &_textQ[_textQHead];
        if (oldest->text) free(oldest->text);
        _textQHead = (_textQHead + 1) % TEXT_EVENT_CAP;
        _textQCount--;
    }

    TextEvent *e = &_textQ[(_textQHead + _textQCount) % TEXT_EVENT_CAP];
    e->type      = type;
    e->windowID  = wid;
    e->text      = strdup(utf8 ? utf8 : "");
    e->imeStart  = start;
    e->imeLength = len;
    e->modifiers = _evModifiers;
    _textQCount++;
}

// Move the oldest queued text event into the _ev* globals so the Go
// accessors see it.  Returns 1 if an event was delivered, 0 if empty.
static int popTextEvent(void) {
    if (_textQCount == 0) return 0;

    TextEvent *e = &_textQ[_textQHead];
    _textQHead = (_textQHead + 1) % TEXT_EVENT_CAP;
    _textQCount--;

    // Same ownership rule as storeEvent: the globals own _evText and
    // free it before taking the next one.  Go copies via C.GoString
    // immediately after the poll returns.
    if (_evText) free(_evText);
    _evText      = e->text;
    e->text      = NULL;
    _evType      = e->type;
    _evWindowID  = e->windowID;
    _evIMEStart  = e->imeStart;
    _evIMELength = e->imeLength;
    _evModifiers = e->modifiers;
    return 1;
}

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
@property (nonatomic, assign) NSRange     selRange;
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
        _selRange = NSMakeRange(0, 0);

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
    // A composition already in progress means the input method owns
    // this key. So does a key that starts one: it produced a preedit,
    // not a command.
    BOOL wasComposing = (_markedText != nil);
    [self interpretKeyEvents:@[event]];
    if (wasComposing || _markedText != nil) {
        _imeConsumedKey = 1;
    }
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
    NSString *committed = nil;
    if ([string isKindOfClass:[NSAttributedString class]]) {
        committed = [(NSAttributedString *)string string];
    } else if ([string isKindOfClass:[NSString class]]) {
        committed = (NSString *)string;
    }

    // Queued, not written straight to the globals: a CJK commit is
    // routinely followed by setMarkedText: inside the same sendEvent:.
    pushTextEvent(METAL_EVENT_CHAR, [committed UTF8String], _windowID, 0, 0);

    [self unmarkText];
}

- (void)setMarkedText:(id)string
        selectedRange:(NSRange)selectedRange
     replacementRange:(NSRange)replacementRange {
    if ([string isKindOfClass:[NSAttributedString class]]) {
        _markedText = [(NSAttributedString *)string copy];
    } else if ([string isKindOfClass:[NSString class]]) {
        _markedText = [[NSAttributedString alloc] initWithString:(NSString *)string];
    }

    if (_markedText && [_markedText length] > 0) {
        _markedRange = NSMakeRange(0, [_markedText length]);
        // Selection inside the composition, read back via
        // selectedRange while converting. Kept verbatim (length and
        // all — conversion selects a whole clause), guarding only
        // against a location the composition cannot hold.
        _selRange = selectedRange;
        if (_selRange.location == NSNotFound ||
            _selRange.location > [_markedText length]) {
            _selRange = NSMakeRange([_markedText length], 0);
        }
        // Subtraction, not a sum: unsigned range members wrap.
        if (_selRange.length > [_markedText length] - _selRange.location) {
            _selRange.length = [_markedText length] - _selRange.location;
        }
        // Report the clamped range, not the raw one: NSNotFound
        // truncates to -1 in an int and would reach the renderer as a
        // negative caret offset.
        pushTextEvent(METAL_EVENT_IME_COMP, [[_markedText string] UTF8String],
                      _windowID, (int)_selRange.location,
                      (int)_selRange.length);
    } else {
        _markedText = nil;
        _markedRange = NSMakeRange(NSNotFound, 0);
        _selRange = NSMakeRange(0, 0);
        // Empty marked text means the IME ended the composition (cancel,
        // or commit handled by insertText:).  Deliver it as an empty
        // composition so Go can clear the preedit; unmarkText stays
        // silent so a commit does not emit a second one.
        pushTextEvent(METAL_EVENT_IME_COMP, "", _windowID, 0, 0);
    }
}

- (void)unmarkText {
    _markedText = nil;
    _markedRange = NSMakeRange(NSNotFound, 0);
    _selRange = NSMakeRange(0, 0);
}

- (BOOL)hasMarkedText {
    return _markedText != nil;
}

- (NSRange)markedRange {
    return _markedRange;
}

// The input method does range arithmetic against this value and
// abandons a commit when it is NSNotFound — a converted commit has to
// replace text at the insertion point. go-gui keeps the document text
// on the Go side, so the range is expressed against the composition
// alone: marked text always starts at 0, and _selRange tracks the
// caret the IME last reported inside it.
- (NSRange)selectedRange {
    return _selRange;
}

// Returning nil here makes the input method abandon a converted
// commit — it asks for the text it is about to replace. go-gui keeps
// the document on the Go side, so the composition itself stands in as
// the document: marked text always starts at 0, which is the same
// coordinate space markedRange and selectedRange report.
- (NSAttributedString *)attributedSubstringForProposedRange:(NSRange)range
                                                actualRange:(NSRangePointer)actualRange {
    NSString *doc = [_markedText string];
    if (!doc) doc = @"";

    NSRange r = range;
    if (r.location == NSNotFound || r.location > [doc length]) {
        if (actualRange) *actualRange = NSMakeRange(NSNotFound, 0);
        return nil;
    }
    // Subtract rather than compare location + length: NSRange members
    // are unsigned, and IMK does send lengths near NSUIntegerMax, which
    // would wrap the sum and slip past a comparison into a crash inside
    // substringWithRange:.
    if (r.length > [doc length] - r.location) {
        r.length = [doc length] - r.location;
    }
    if (actualRange) *actualRange = r;
    return [[NSAttributedString alloc]
        initWithString:[doc substringWithRange:r]];
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

// The attributes a text client advertises for marked text. An empty
// list tells the input method the client cannot render clause
// segments, which degrades conversion behavior.
- (NSArray<NSAttributedStringKey> *)validAttributesForMarkedText {
    return @[NSUnderlineStyleAttributeName,
             NSUnderlineColorAttributeName,
             NSMarkedClauseSegmentAttributeName];
}

@end

// ─── GUIApplication (NSApplication subclass) ────────────────────
// Exists so [GUIApplication sharedApplication] returns an instance
// of this class rather than plain NSApplication — any code that
// checks isKindOfClass: or swizzles on the application class will
// see GUIApplication. The sendEvent: override is intentionally a
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

// A resize drag begun on the window border. The press that started it
// belongs to AppKit, not to the app — see the _liveResizeStarted
// consumer in metalPollEvent.
- (void)windowWillStartLiveResize:(NSNotification *)notification {
    _liveResizeStarted = 1;
}

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
__attribute__((weak)) void goMetalPumpFrames(void) {}

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

    // Drain text-input events queued by NSTextInputClient callbacks
    // during a previous sendEvent before touching the NSEvent queue.
    // This is what keeps a multi-callback keystroke (CJK commit +
    // residual composition) in order and intact.
    if (popTextEvent()) return 1;

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
        // IME callbacks can fire during the blocking wait — the
        // candidate window runs the runloop — so check the queue again.
        if (popTextEvent()) return 1;
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

    _liveResizeStarted = 0;
    _imeConsumedKey = 0;

    // Forward to AppKit for window management and text input.
    // Key events: keyDown: → interpretKeyEvents: → insertText: fires
    // for printable characters, doCommandBySelector: (no-op) for
    // non-printable keys. Go receives EventChar or EventKeyDown.
    // Mouse/scroll: standard AppKit routing (resize, close, focus).
    [NSApp sendEvent:event];

    // Drop a mouse-down that AppKit consumed to resize the window.
    // The resize grab area overlaps the content view by a few points,
    // so storeEvent (which runs before sendEvent, with no way to know
    // where AppKit will route the press) hands Go a plain content-
    // relative click. A widget filling the window then treats it as a
    // real press — e.g. a text input starts a drag-selection and calls
    // MouseLock. The matching mouse-up never arrives because AppKit's
    // resize tracking loop, which runs to completion inside the
    // sendEvent above, swallows it, so the drag stays locked forever.
    if (_liveResizeStarted && _evType == METAL_EVENT_MOUSE_DOWN) {
        _evType = METAL_EVENT_NONE;
    }
    _liveResizeStarted = 0;

    // Drop a key the input method claimed. Its visible effect arrives
    // as the queued composition or commit instead.
    if (_imeConsumedKey && _evType == METAL_EVENT_KEY_DOWN) {
        _evType = METAL_EVENT_NONE;
    }
    _imeConsumedKey = 0;

    // sendEvent may have queued text events (insertText:/setMarkedText:).
    // Return the NSEvent-derived event now and let the drain at the top
    // of the next poll deliver them, so a printable key yields KEY_DOWN
    // followed by CHAR — the order X11, win32 and web already use. If
    // the NSEvent produced nothing of its own, deliver a queued event
    // straight away rather than handing Go an empty poll.
    if (_evType == METAL_EVENT_NONE) {
        popTextEvent();
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

    // Restore key repeat.  macOS press-and-hold (on by default) hands a
    // held key to the accent-palette machinery after the first
    // insertText:, and stops calling insertText: for the auto-repeat
    // keyDown: events that follow.  Every repeat then arrives as a bare
    // EventKeyDown with no EventChar behind it, so holding a letter types
    // one character, holding Backspace deletes one character, and a
    // terminal pane echoes nothing while the key is down.
    //
    // Registered rather than written: the registration domain sits at the
    // bottom of the defaults search order, so a user who genuinely wants
    // the accent palette keeps the escape hatch
    //
    //     defaults write -g ApplePressAndHoldEnabled -bool true
    //
    // which lives in NSGlobalDomain and outranks this.  Must run before
    // sharedApplication so AppKit reads it during its own setup.
    [[NSUserDefaults standardUserDefaults] registerDefaults:@{
        @"ApplePressAndHoldEnabled": @NO,
    }];

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

    // Windows exist now, so frames are meaningful: arm the nested-runloop
    // frame pump. Idempotent, so the repeat calls for later windows are
    // harmless.
    metalStartFramePump();
}

// ─── Frame pump (nested runloops) ──────────────────────────────
//
// Rationale lives in metal_window.h. The short version: the Go event loop
// only pumps NSDefaultRunLoopMode, so a nested AppKit runloop (modal
// dialog, menu tracking, live resize) starves every window of frames.
// This timer is scheduled in NSRunLoopCommonModes, which includes
// NSEventTrackingRunLoopMode and NSModalPanelRunLoopMode, so it keeps
// firing exactly when the Go loop cannot.

// Timer interval — 60 Hz. Frames are cheap when nothing is dirty:
// Window.PumpFrame flushes commands and returns false without rendering
// unless a refresh is actually pending.
static const NSTimeInterval kFramePumpInterval = 1.0 / 60.0;

static NSTimer *_framePumpTimer;

void metalStartFramePump(void) {
    if (_framePumpTimer) return;
    _framePumpTimer = [NSTimer timerWithTimeInterval:kFramePumpInterval
                                             repeats:YES
                                               block:^(NSTimer *timer) {
        // In the default mode the Go event loop is running and owns frame
        // timing; pumping here too would double every frame. Only the
        // nested modes — where that loop is blocked — need this timer.
        NSString *mode = [[NSRunLoop currentRunLoop] currentMode];
        if (!mode || [mode isEqualToString:NSDefaultRunLoopMode]) return;
        goMetalPumpFrames();
    }];
    // Common modes, not the default mode: see above.
    [[NSRunLoop mainRunLoop] addTimer:_framePumpTimer
                              forMode:NSRunLoopCommonModes];
}

void metalStopFramePump(void) {
    if (!_framePumpTimer) return;
    [_framePumpTimer invalidate];
    _framePumpTimer = nil;
}

// ─── Test helpers (called from Go tests via cgo) ─────────────────
//
// These live in the production file, not a separate _test.m, on
// purpose: they reach file-static state — the event globals (_evType,
// _quitRequested…), the text-event queue, metalCursorInContentBounds, and
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

// Push onto the text-event queue exactly as insertText: /
// setMarkedText: do, so Go tests can drive the IME path (and the
// multi-event-per-sendEvent case behind issue #149) without a running
// event loop or a real input method.
void metalTestPushIMECommit(const char *utf8) {
    pushTextEvent(METAL_EVENT_CHAR, utf8, 0, 0, 0);
}

void metalTestPushIMEComposition(const char *utf8, int start, int len) {
    pushTextEvent(METAL_EVENT_IME_COMP, utf8, 0, start, len);
}

int metalTestIMEQueueDepth(void) { return _textQCount; }

// Forward declaration: the conformance helpers below reset the queue
// they dirty, and the definition sits after them.
void metalTestResetIMEQueue(void);

// Drive keyDown: with a composition in progress and report whether the
// raw key was claimed by the input method. A key that reaches the
// widget while composing moves the field's caret out from under the
// preedit and can fire Enter/Escape actions the user never meant.
int metalTestIMEKeySuppressedWhileComposing(void *windowHandle) {
    if (!windowHandle) return 0;
    GoGuiWindow *gw = (GoGuiWindow *)windowHandle;
    if (!gw->nsWindow) return 0;
    NSView *cv = gw->nsWindow.contentView;
    if (![cv isKindOfClass:[MetalContentView class]]) return 0;
    MetalContentView *view = (MetalContentView *)cv;

    NSEvent *key = [NSEvent keyEventWithType:NSEventTypeKeyDown
                                    location:NSZeroPoint
                               modifierFlags:0
                                   timestamp:0
                                windowNumber:[gw->nsWindow windowNumber]
                                     context:nil
                                  characters:@""
                 charactersIgnoringModifiers:@""
                                   isARepeat:NO
                                     keyCode:123]; // left arrow
    if (!key) return 0;

    // Composing: the key belongs to the input method.
    [view setMarkedText:@"あい"
          selectedRange:NSMakeRange(0, 2)
       replacementRange:NSMakeRange(NSNotFound, 0)];
    _imeConsumedKey = 0;
    [view keyDown:key];
    int suppressed = _imeConsumedKey;

    // Not composing: the key is the widget's to handle.
    [view unmarkText];
    _imeConsumedKey = 0;
    [view keyDown:key];
    int deliveredWhenIdle = (_imeConsumedKey == 0);

    _imeConsumedKey = 0;
    metalTestResetIMEQueue();
    return (suppressed && deliveredWhenIdle) ? 1 : 0;
}

// Verify the NSTextInputClient answers an input method needs to commit
// converted text. Returning NSNotFound from selectedRange, or nil from
// attributedSubstringForProposedRange:, makes the Japanese IME abandon
// the commit after conversion — the composition stays marked forever
// and nothing reaches the app. Reproduced in a plain AppKit program
// with no go-gui code in it, so the contract is AppKit's, not go-gui's.
// Empty validAttributesForMarkedText tells the IME the client cannot
// render clause segments.
int metalTestIMEClientConformance(void *windowHandle) {
    if (!windowHandle) return 0;
    GoGuiWindow *gw = (GoGuiWindow *)windowHandle;
    if (!gw->nsWindow) return 0;
    NSView *cv = gw->nsWindow.contentView;
    if (![cv isKindOfClass:[MetalContentView class]]) return 0;
    MetalContentView *view = (MetalContentView *)cv;

    int ok = 1;

    // Compose "あい" with the whole clause selected, as conversion does.
    [view setMarkedText:@"あい"
          selectedRange:NSMakeRange(0, 2)
       replacementRange:NSMakeRange(NSNotFound, 0)];

    NSRange sel = [view selectedRange];
    if (sel.location == NSNotFound || sel.location + sel.length > 2) ok = 0;

    NSRange actual = NSMakeRange(NSNotFound, 0);
    NSAttributedString *sub =
        [view attributedSubstringForProposedRange:NSMakeRange(0, 2)
                                      actualRange:&actual];
    if (!sub || ![[sub string] isEqualToString:@"あい"]) ok = 0;

    // An out-of-range request must degrade to nil, not crash or lie.
    NSAttributedString *bad =
        [view attributedSubstringForProposedRange:NSMakeRange(99, 2)
                                      actualRange:NULL];
    if (bad != nil) ok = 0;

    // A length near NSUIntegerMax must clamp, not wrap past the guard
    // into substringWithRange: — IMK really does ask for these.
    NSAttributedString *huge =
        [view attributedSubstringForProposedRange:
                  NSMakeRange(1, NSUIntegerMax)
                                      actualRange:NULL];
    if (!huge || ![[huge string] isEqualToString:@"い"]) ok = 0;

    if ([[view validAttributesForMarkedText] count] == 0) ok = 0;

    // The queued composition must carry the clamped selection, not the
    // raw one: NSNotFound truncates to -1 in the int the event holds,
    // and Go would draw the clause underline from a negative offset.
    metalTestResetIMEQueue();
    [view setMarkedText:@"あい"
          selectedRange:NSMakeRange(NSNotFound, 0)
       replacementRange:NSMakeRange(NSNotFound, 0)];
    if (!popTextEvent()) {
        ok = 0;
    } else if (_evIMEStart != 2 || _evIMELength != 0) {
        ok = 0;
    }

    [view unmarkText];
    if ([view selectedRange].location == NSNotFound) ok = 0;

    metalTestResetIMEQueue();
    return ok;
}

// Drain one entry without going through metalPollEvent.  Used for the
// empty-queue assertions: metalPollEvent would fall through to
// [NSApp nextEventMatchingMask:…], which is main-thread-only and must
// not be called from the ordinary (non-main-thread) test binary.
int metalTestPopTextEvent(void) { return popTextEvent(); }

// Drop every queued entry so one test cannot leak state into the next.
void metalTestResetIMEQueue(void) {
    while (_textQCount > 0) {
        TextEvent *e = &_textQ[_textQHead];
        if (e->text) { free(e->text); e->text = NULL; }
        _textQHead = (_textQHead + 1) % TEXT_EVENT_CAP;
        _textQCount--;
    }
    _evType = METAL_EVENT_NONE;
}

// Run the main runloop for ms milliseconds in a nested mode, standing in
// for what AppKit does inside [NSAlert runModal] or menu tracking. The
// frame-pump timer must fire here (mode != NSDefaultRunLoopMode) and must
// not fire in the default-mode variant. See TestFramePumpNestedMode.
// runMode:beforeDate: returns as soon as it services one input source
// (and immediately when the mode has none), so loop until the deadline.
static void metalTestRunMode(NSString *mode, int ms) {
    NSDate *until = [NSDate dateWithTimeIntervalSinceNow:ms / 1000.0];
    while ([until timeIntervalSinceNow] > 0) {
        [[NSRunLoop mainRunLoop] runMode:mode beforeDate:until];
    }
}

void metalTestRunModalMode(int ms) {
    metalTestRunMode(NSModalPanelRunLoopMode, ms);
}

void metalTestRunDefaultMode(int ms) {
    metalTestRunMode(NSDefaultRunLoopMode, ms);
}

// Report ApplePressAndHoldEnabled as it stands in the registration
// domain: 0 registered off, 1 registered on, -1 not registered at all.
// Deliberately not the *effective* value — a developer who set
// NSGlobalDomain themselves would otherwise fail this test for holding a
// preference the fix intentionally leaves them.
int metalTestPressAndHoldRegistered(void) {
    NSDictionary *reg = [[NSUserDefaults standardUserDefaults]
                             volatileDomainForName:NSRegistrationDomain];
    id v = reg[@"ApplePressAndHoldEnabled"];
    if (v == nil) return -1;
    return [v boolValue] ? 1 : 0;
}

// Report whether the frame-pump timer is currently installed.
int metalTestFramePumpActive(void) {
    return (_framePumpTimer && [_framePumpTimer isValid]) ? 1 : 0;
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
