#ifndef A11Y_DARWIN_H
#define A11Y_DARWIN_H

#include "metal_window.h"

// Role constants matching gui.AccessRole iota order (0-34).
enum {
    A11Y_ROLE_NONE = 0,
    A11Y_ROLE_BUTTON,
    A11Y_ROLE_CHECKBOX,
    A11Y_ROLE_COLOR_WELL,
    A11Y_ROLE_COMBO_BOX,
    A11Y_ROLE_DATE_FIELD,
    A11Y_ROLE_DIALOG,
    A11Y_ROLE_DISCLOSURE,
    A11Y_ROLE_GRID,
    A11Y_ROLE_GRID_CELL,
    A11Y_ROLE_GROUP,
    A11Y_ROLE_HEADING,
    A11Y_ROLE_IMAGE,
    A11Y_ROLE_LINK,
    A11Y_ROLE_LIST,
    A11Y_ROLE_LIST_ITEM,
    A11Y_ROLE_MENU,
    A11Y_ROLE_MENU_BAR,
    A11Y_ROLE_MENU_ITEM,
    A11Y_ROLE_PROGRESS_BAR,
    A11Y_ROLE_RADIO_BUTTON,
    A11Y_ROLE_RADIO_GROUP,
    A11Y_ROLE_SCROLL_AREA,
    A11Y_ROLE_SCROLL_BAR,
    A11Y_ROLE_SLIDER,
    A11Y_ROLE_SPLITTER,
    A11Y_ROLE_STATIC_TEXT,
    A11Y_ROLE_SWITCH_TOGGLE,
    A11Y_ROLE_TAB,
    A11Y_ROLE_TAB_ITEM,
    A11Y_ROLE_TEXT_FIELD,
    A11Y_ROLE_TEXT_AREA,
    A11Y_ROLE_TOOLBAR,
    A11Y_ROLE_TREE,
    A11Y_ROLE_TREE_ITEM,
    A11Y_ROLE_COUNT,
};

// Action constants matching gui.A11yAction* (0-4).
enum {
    A11Y_ACTION_PRESS     = 0,
    A11Y_ACTION_INCREMENT = 1,
    A11Y_ACTION_DECREMENT = 2,
    A11Y_ACTION_CONFIRM   = 3,
    A11Y_ACTION_CANCEL    = 4,
};

// A11yCNode mirrors gui.A11yNode for CGo marshaling.
typedef struct {
    int         role;
    int         state;
    const char *label;
    const char *value;
    const char *description;
    float       x, y, w, h;
    int         parentIdx;
    int         childrenStart;
    int         childrenCount;
} A11yCNode;

// Initialize the accessibility element tree.
void a11yInit(GoGuiNSWindow w);

// Sync the accessibility tree with the current frame's nodes.
void a11ySync(const A11yCNode *nodes, int count, int focusedIdx,
              float windowH);

// Destroy the accessibility tree and release all elements.
void a11yDestroy(void);

// Post an announcement via VoiceOver.
void a11yAnnounce(const char *text);

// Implemented in Go — called from ObjC when VoiceOver triggers
// an action on an element.
extern void goA11yAction(int action, int index);

#endif
