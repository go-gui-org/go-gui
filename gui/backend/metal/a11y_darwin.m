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

// Forward declaration — used by isAccessibilityFocused.
static int _curFocusedIdx = -1;

// ─── GUIAccessibilityElement ─────────────────────────────────

@interface GUIAccessibilityElement : NSAccessibilityElement

@property (nonatomic) int         nodeIndex;
@property (nonatomic) int         nodeRole;
@property (nonatomic) int         nodeState;
@property (nonatomic, copy) NSString *nodeLabel;
@property (nonatomic, copy) NSString *nodeValue;
@property (nonatomic, copy) NSString *nodeDescription;
@property (nonatomic) NSRect      nodeFrame;

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
    return _nodeIndex == _curFocusedIdx;
}

// ─── Actions ─────────────────────────────────────────────────

- (BOOL)accessibilityPerformPress {
    goA11yAction(A11Y_ACTION_PRESS, _nodeIndex);
    return YES;
}

- (BOOL)accessibilityPerformIncrement {
    goA11yAction(A11Y_ACTION_INCREMENT, _nodeIndex);
    return YES;
}

- (BOOL)accessibilityPerformDecrement {
    goA11yAction(A11Y_ACTION_DECREMENT, _nodeIndex);
    return YES;
}

- (BOOL)accessibilityPerformConfirm {
    goA11yAction(A11Y_ACTION_CONFIRM, _nodeIndex);
    return YES;
}

- (BOOL)accessibilityPerformCancel {
    goA11yAction(A11Y_ACTION_CANCEL, _nodeIndex);
    return YES;
}

@end

// ─── Pool + State ────────────────────────────────────────────

static NSMutableArray<GUIAccessibilityElement *> *_elementPool;
static int _activeCount;
static NSWindow *_nsWindow;
static NSView   *_nsView;
static int       _prevFocusedIdx = -1;

// Root element acts as the accessibility container attached to
// the content view.
static GUIAccessibilityElement *_rootElement;

static GUIAccessibilityElement *poolGet(int idx) {
    while (idx >= (int)_elementPool.count) {
        GUIAccessibilityElement *el =
            [[GUIAccessibilityElement alloc] init];
        el.nodeChildren =
            [[NSMutableArray alloc] initWithCapacity:8];
        [_elementPool addObject:el];
    }
    return _elementPool[idx];
}

// ─── Coordinate Conversion ───────────────────────────────────
// Framework uses top-left origin; macOS uses bottom-left screen
// coords.

static NSRect convertFrame(float x, float y, float w, float h,
                           float windowH) {
    float flippedY = windowH - y - h;
    NSRect localRect = NSMakeRect(x, flippedY, w, h);
    NSRect windowRect = [_nsView convertRect:localRect toView:nil];
    return [_nsWindow convertRectToScreen:windowRect];
}

// ─── Public API ──────────────────────────────────────────────

void a11yInit(GoGuiNSWindow w) {
    initRoleMap();

    // Extract NSWindow/NSView from native window handle.
    _nsWindow = (__bridge NSWindow *)metalWindowGetNSWindow(w);
    if (!_nsWindow) return;
    _nsView = [_nsWindow contentView];
    if (!_nsView) {
        return;
    }

    _elementPool =
        [[NSMutableArray alloc] initWithCapacity:256];
    _activeCount = 0;
    _prevFocusedIdx = -1;

    // Create root container element.
    _rootElement = [[GUIAccessibilityElement alloc] init];
    _rootElement.nodeLabel = @"Application";
    _rootElement.nodeRole = A11Y_ROLE_GROUP;
    _rootElement.nodeParent = _nsView;
    _rootElement.nodeChildren =
        [[NSMutableArray alloc] initWithCapacity:64];

    // Attach root element to the content view's accessibility
    // children.
    _nsView.accessibilityChildren = @[_rootElement];
}

void a11ySync(const A11yCNode *nodes, int count,
              int focusedIdx, float windowH) {
    if (!_rootElement || !nodes || count <= 0) {
        return;
    }

    // Update or grow pool elements.
    for (int i = 0; i < count; i++) {
        GUIAccessibilityElement *el = poolGet(i);
        const A11yCNode *n = &nodes[i];

        el.nodeIndex = i;
        el.nodeRole  = n->role;
        el.nodeState = n->state;
        el.nodeLabel = n->label
            ? [NSString stringWithUTF8String:n->label] : @"";
        el.nodeValue = n->value
            ? [NSString stringWithUTF8String:n->value] : @"";
        el.nodeDescription = n->description
            ? [NSString stringWithUTF8String:n->description]
            : @"";
        el.nodeFrame = convertFrame(
            n->x, n->y, n->w, n->h, windowH);
        [el.nodeChildren removeAllObjects];

        // Parent: root if parentIdx < 0, else pool element.
        if (n->parentIdx < 0) {
            el.nodeParent = _rootElement;
        } else {
            el.nodeParent = poolGet(n->parentIdx);
        }
    }
    _activeCount = count;

    // Wire children arrays.
    [_rootElement.nodeChildren removeAllObjects];
    for (int i = 0; i < count; i++) {
        const A11yCNode *n = &nodes[i];
        GUIAccessibilityElement *el = _elementPool[i];

        if (n->parentIdx < 0) {
            [_rootElement.nodeChildren addObject:el];
        } else if (n->parentIdx < count) {
            [_elementPool[n->parentIdx].nodeChildren
                addObject:el];
        }
    }

    // Update root frame to cover the whole window.
    _rootElement.nodeFrame = convertFrame(
        0, 0, (float)_nsView.bounds.size.width,
        (float)_nsView.bounds.size.height, windowH);

    // Update current focused index for isAccessibilityFocused.
    _curFocusedIdx = focusedIdx;

    // Focus notification.
    if (focusedIdx != _prevFocusedIdx && focusedIdx >= 0 &&
        focusedIdx < count) {
        _prevFocusedIdx = focusedIdx;
        NSAccessibilityPostNotification(
            _elementPool[focusedIdx],
            NSAccessibilityFocusedUIElementChangedNotification);
    }
}

void a11yDestroy(void) {
    if (_nsView) {
        _nsView.accessibilityChildren = nil;
    }
    _rootElement = nil;
    [_elementPool removeAllObjects];
    _elementPool = nil;
    _activeCount = 0;
    _prevFocusedIdx = -1;
    _nsWindow = nil;
    _nsView = nil;
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
