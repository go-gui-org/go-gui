#import <AppKit/AppKit.h>
#include "a11y_darwin.h"

// ─── Role Map ────────────────────────────────────────────────

static NSAccessibilityRole _roleMap[A11Y_ROLE_COUNT];

static void initRoleMap(void) {
    _roleMap[A11Y_ROLE_NONE]          = NSAccessibilityUnknownRole;
    _roleMap[A11Y_ROLE_BUTTON]        = NSAccessibilityButtonRole;
    _roleMap[A11Y_ROLE_CHECKBOX]      = NSAccessibilityCheckBoxRole;
    _roleMap[A11Y_ROLE_COLOR_WELL]    = NSAccessibilityColorWellRole;
    _roleMap[A11Y_ROLE_COMBO_BOX]     = NSAccessibilityComboBoxRole;
    _roleMap[A11Y_ROLE_DATE_FIELD]    = NSAccessibilityTextFieldRole;
    _roleMap[A11Y_ROLE_DIALOG]        = NSAccessibilitySheetRole;
    _roleMap[A11Y_ROLE_DISCLOSURE]    =
        NSAccessibilityDisclosureTriangleRole;
    _roleMap[A11Y_ROLE_GRID]          = NSAccessibilityTableRole;
    _roleMap[A11Y_ROLE_GRID_CELL]     = NSAccessibilityCellRole;
    _roleMap[A11Y_ROLE_GROUP]         = NSAccessibilityGroupRole;
    _roleMap[A11Y_ROLE_HEADING]       = NSAccessibilityStaticTextRole;
    _roleMap[A11Y_ROLE_IMAGE]         = NSAccessibilityImageRole;
    _roleMap[A11Y_ROLE_LINK]          = NSAccessibilityLinkRole;
    _roleMap[A11Y_ROLE_LIST]          = NSAccessibilityListRole;
    _roleMap[A11Y_ROLE_LIST_ITEM]     = NSAccessibilityGroupRole;
    _roleMap[A11Y_ROLE_MENU]          = NSAccessibilityMenuRole;
    _roleMap[A11Y_ROLE_MENU_BAR]      = NSAccessibilityMenuBarRole;
    _roleMap[A11Y_ROLE_MENU_ITEM]     = NSAccessibilityMenuItemRole;
    _roleMap[A11Y_ROLE_PROGRESS_BAR]  =
        NSAccessibilityProgressIndicatorRole;
    _roleMap[A11Y_ROLE_RADIO_BUTTON]  = NSAccessibilityRadioButtonRole;
    _roleMap[A11Y_ROLE_RADIO_GROUP]   = NSAccessibilityRadioGroupRole;
    _roleMap[A11Y_ROLE_SCROLL_AREA]   = NSAccessibilityScrollAreaRole;
    _roleMap[A11Y_ROLE_SCROLL_BAR]    = NSAccessibilityScrollBarRole;
    _roleMap[A11Y_ROLE_SLIDER]        = NSAccessibilitySliderRole;
    _roleMap[A11Y_ROLE_SPLITTER]      = NSAccessibilitySplitGroupRole;
    _roleMap[A11Y_ROLE_STATIC_TEXT]   = NSAccessibilityStaticTextRole;
    _roleMap[A11Y_ROLE_SWITCH_TOGGLE] = NSAccessibilityCheckBoxRole;
    _roleMap[A11Y_ROLE_TAB]           = NSAccessibilityTabGroupRole;
    _roleMap[A11Y_ROLE_TAB_ITEM]      =
        NSAccessibilityRadioButtonRole;
    _roleMap[A11Y_ROLE_TEXT_FIELD]    = NSAccessibilityTextFieldRole;
    _roleMap[A11Y_ROLE_TEXT_AREA]     = NSAccessibilityTextAreaRole;
    _roleMap[A11Y_ROLE_TOOLBAR]       = NSAccessibilityToolbarRole;
    _roleMap[A11Y_ROLE_TREE]          = NSAccessibilityOutlineRole;
    _roleMap[A11Y_ROLE_TREE_ITEM]     = NSAccessibilityRowRole;
}

// ─── State Flags (must match gui.AccessState bitmask) ────────

enum {
    STATE_EXPANDED  = 1,
    STATE_SELECTED  = 2,
    STATE_CHECKED   = 4,
    STATE_REQUIRED  = 8,
    STATE_INVALID   = 16,
    STATE_BUSY      = 32,
    STATE_READ_ONLY = 64,
    STATE_MODAL     = 128,
    // STATE_LIVE = 256 — not mapped to NSAccessibility
    STATE_DISABLED  = 512,
};

// ─── Per-window state ────────────────────────────────────────
// One live VoiceOver tree per window: a second window's init/sync
// never disturbs the first's. Contexts are keyed by the opaque
// window handle and dropped by a11yDestroy. Declared before the
// element implementation, which reads the focused index off it.

@class GUIAccessibilityElement;

@interface A11yContext : NSObject

@property (nonatomic, weak) NSWindow *nsWindow;
@property (nonatomic, weak) NSView   *nsView;
@property (nonatomic, strong) NSMutableArray<
    GUIAccessibilityElement *> *pool;
@property (nonatomic, strong) GUIAccessibilityElement *root;
@property (nonatomic) int activeCount;
@property (nonatomic) int curFocusedIdx;
@property (nonatomic) int prevFocusedIdx;

@end

@implementation A11yContext

@end

// ─── GUIAccessibilityElement ─────────────────────────────────

@interface GUIAccessibilityElement : NSAccessibilityElement

@property (nonatomic) int         nodeIndex;
@property (nonatomic) int         nodeRole;
@property (nonatomic) int         nodeState;
@property (nonatomic, copy) NSString *nodeLabel;
@property (nonatomic, copy) NSString *nodeValue;
@property (nonatomic, copy) NSString *nodeDescription;
@property (nonatomic) NSRect      nodeFrame;

// Owning window's context (weak: the context map owns it) and the
// opaque window token handed back to Go with actions.
@property (nonatomic, weak) A11yContext *ctx;
@property (nonatomic) uintptr_t  ownerToken;

@property (nonatomic, strong) NSMutableArray<
    GUIAccessibilityElement *> *nodeChildren;
@property (nonatomic, weak) id nodeParent;

@end

@implementation GUIAccessibilityElement

- (NSAccessibilityRole)accessibilityRole {
    if (_nodeRole >= 0 && _nodeRole < A11Y_ROLE_COUNT) {
        return _roleMap[_nodeRole];
    }
    return NSAccessibilityUnknownRole;
}

- (NSString *)accessibilityLabel {
    return _nodeLabel;
}

- (NSString *)accessibilityValue {
    // Checkboxes/toggles: return @(1) or @(0) for checked state.
    if (_nodeRole == A11Y_ROLE_CHECKBOX ||
        _nodeRole == A11Y_ROLE_SWITCH_TOGGLE) {
        return (_nodeState & STATE_CHECKED)
            ? @"1" : @"0";
    }
    return _nodeValue;
}

- (NSString *)accessibilityHelp {
    return _nodeDescription;
}

- (NSRect)accessibilityFrame {
    return _nodeFrame;
}

- (id)accessibilityParent {
    return _nodeParent;
}

- (NSArray *)accessibilityChildren {
    return _nodeChildren;
}

- (BOOL)isAccessibilityElement {
    return YES;
}

- (BOOL)isAccessibilityEnabled {
    return !(_nodeState & (STATE_READ_ONLY | STATE_DISABLED));
}

- (BOOL)isAccessibilityExpanded {
    return (_nodeState & STATE_EXPANDED) != 0;
}

- (BOOL)isAccessibilitySelected {
    return (_nodeState & STATE_SELECTED) != 0;
}

- (BOOL)isAccessibilityRequired {
    return (_nodeState & STATE_REQUIRED) != 0;
}

- (BOOL)isAccessibilityModal {
    return (_nodeState & STATE_MODAL) != 0;
}

- (BOOL)isAccessibilityFocused {
    return _ctx && _nodeIndex == _ctx.curFocusedIdx;
}

// ─── Actions ─────────────────────────────────────────────────

- (BOOL)accessibilityPerformPress {
    goA11yAction(A11Y_ACTION_PRESS, _nodeIndex, _ownerToken);
    return YES;
}

- (BOOL)accessibilityPerformIncrement {
    goA11yAction(A11Y_ACTION_INCREMENT, _nodeIndex, _ownerToken);
    return YES;
}

- (BOOL)accessibilityPerformDecrement {
    goA11yAction(A11Y_ACTION_DECREMENT, _nodeIndex, _ownerToken);
    return YES;
}

- (BOOL)accessibilityPerformConfirm {
    goA11yAction(A11Y_ACTION_CONFIRM, _nodeIndex, _ownerToken);
    return YES;
}

- (BOOL)accessibilityPerformCancel {
    goA11yAction(A11Y_ACTION_CANCEL, _nodeIndex, _ownerToken);
    return YES;
}

@end

// ─── Context registry ────────────────────────────────────────

static NSMutableDictionary<NSValue *, A11yContext *> *_contexts;

static A11yContext *ctxForHandle(GoGuiNSWindow w) {
    if (!w || !_contexts) {
        return nil;
    }
    return _contexts[[NSValue valueWithPointer:w]];
}

static GUIAccessibilityElement *poolGet(A11yContext *ctx, int idx) {
    while (idx >= (int)ctx.pool.count) {
        GUIAccessibilityElement *el =
            [[GUIAccessibilityElement alloc] init];
        el.nodeChildren =
            [[NSMutableArray alloc] initWithCapacity:8];
        [ctx.pool addObject:el];
    }
    return ctx.pool[idx];
}

// ─── Coordinate Conversion ───────────────────────────────────
// Framework uses top-left origin; macOS uses bottom-left screen
// coords.

static NSRect convertFrame(A11yContext *ctx, float x, float y,
                           float w, float h, float windowH) {
    float flippedY = windowH - y - h;
    NSRect localRect = NSMakeRect(x, flippedY, w, h);
    NSRect windowRect = [ctx.nsView convertRect:localRect toView:nil];
    return [ctx.nsWindow convertRectToScreen:windowRect];
}

// ─── Public API ──────────────────────────────────────────────

void a11yInit(GoGuiNSWindow w) {
    initRoleMap();
    if (!_contexts) {
        _contexts = [[NSMutableDictionary alloc] init];
    }

    // Extract NSWindow/NSView from native window handle.
    NSWindow *nsWindow = (__bridge NSWindow *)metalWindowGetNSWindow(w);
    if (!nsWindow) return;
    NSView *nsView = [nsWindow contentView];
    if (!nsView) {
        return;
    }

    NSValue *key = [NSValue valueWithPointer:w];
    A11yContext *ctx = ctxForHandle(w);
    if (!ctx) {
        ctx = [[A11yContext alloc] init];
        ctx.curFocusedIdx = -1;
        _contexts[key] = ctx;
    }
    ctx.nsWindow = nsWindow;
    ctx.nsView = nsView;

    ctx.pool = [[NSMutableArray alloc] initWithCapacity:256];
    ctx.activeCount = 0;
    ctx.prevFocusedIdx = -1;

    // Create root container element.
    ctx.root = [[GUIAccessibilityElement alloc] init];
    ctx.root.nodeLabel = @"Application";
    ctx.root.nodeRole = A11Y_ROLE_GROUP;
    ctx.root.nodeParent = nsView;
    ctx.root.ctx = ctx;
    ctx.root.ownerToken = (uintptr_t)w;
    ctx.root.nodeChildren =
        [[NSMutableArray alloc] initWithCapacity:64];

    // Attach root element to the content view's accessibility
    // children.
    nsView.accessibilityChildren = @[ctx.root];
}

void a11ySync(GoGuiNSWindow w, const A11yCNode *nodes, int count,
              int focusedIdx, float windowH) {
    A11yContext *ctx = ctxForHandle(w);
    if (!ctx || !nodes || count <= 0) {
        // An emptied tree clears rather than lingers: drop all
        // children so VoiceOver sees nothing, not the previous
        // content. Re-sync repopulates from an empty pool state.
        if (ctx) {
            [ctx.root.nodeChildren removeAllObjects];
            ctx.activeCount = 0;
            ctx.curFocusedIdx = -1;
            ctx.prevFocusedIdx = -1;
        }
        return;
    }

    // Update or grow pool elements.
    for (int i = 0; i < count; i++) {
        GUIAccessibilityElement *el = poolGet(ctx, i);
        const A11yCNode *n = &nodes[i];

        el.nodeIndex = i;
        el.ctx = ctx;
        el.ownerToken = (uintptr_t)w;
        el.nodeRole  = n->role;
        el.nodeState = n->state;
        el.nodeLabel = n->label
            ? [NSString stringWithUTF8String:n->label] : @"";
        el.nodeValue = n->value
            ? [NSString stringWithUTF8String:n->value] : @"";
        el.nodeDescription = n->description
            ? [NSString stringWithUTF8String:n->description]
            : @"";
        el.nodeFrame = convertFrame(ctx,
            n->x, n->y, n->w, n->h, windowH);
        [el.nodeChildren removeAllObjects];

        // Parent: root if parentIdx < 0, else pool element.
        // Out-of-range input attaches to the root rather than
        // growing the pool without bound.
        if (n->parentIdx < 0 || n->parentIdx >= count) {
            el.nodeParent = ctx.root;
        } else {
            el.nodeParent = poolGet(ctx, n->parentIdx);
        }
    }
    ctx.activeCount = count;

    // Wire children arrays.
    [ctx.root.nodeChildren removeAllObjects];
    for (int i = 0; i < count; i++) {
        const A11yCNode *n = &nodes[i];
        GUIAccessibilityElement *el = ctx.pool[i];

        if (n->parentIdx < 0) {
            [ctx.root.nodeChildren addObject:el];
        } else if (n->parentIdx < count) {
            [ctx.pool[n->parentIdx].nodeChildren
                addObject:el];
        }
    }

    // Update root frame to cover the whole window.
    ctx.root.nodeFrame = convertFrame(ctx,
        0, 0, (float)ctx.nsView.bounds.size.width,
        (float)ctx.nsView.bounds.size.height, windowH);

    // Update current focused index for isAccessibilityFocused.
    ctx.curFocusedIdx = focusedIdx;

    // Focus notification.
    if (focusedIdx != ctx.prevFocusedIdx && focusedIdx >= 0 &&
        focusedIdx < count) {
        ctx.prevFocusedIdx = focusedIdx;
        NSAccessibilityPostNotification(
            ctx.pool[focusedIdx],
            NSAccessibilityFocusedUIElementChangedNotification);
    }
}

void a11yDestroy(GoGuiNSWindow w) {
    if (!_contexts) {
        return;
    }
    NSValue *key = [NSValue valueWithPointer:w];
    A11yContext *ctx = _contexts[key];
    if (ctx) {
        if (ctx.nsView) {
            ctx.nsView.accessibilityChildren = nil;
        }
        [_contexts removeObjectForKey:key];
    }
}

void a11yAnnounce(const char *text) {
    if (!text) {
        return;
    }
    NSString *str = [NSString stringWithUTF8String:text];
    NSDictionary *info = @{
        NSAccessibilityAnnouncementKey: str,
        NSAccessibilityPriorityKey:
            @(NSAccessibilityPriorityHigh),
    };
    NSAccessibilityPostNotificationWithUserInfo(
        NSApp,
        NSAccessibilityAnnouncementRequestedNotification,
        info);
}
