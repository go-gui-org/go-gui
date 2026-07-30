//go:build darwin && !ios

#ifndef NATIVEMENU_DARWIN_H
#define NATIVEMENU_DARWIN_H

#include <stdint.h>

// Flattened menu item — children are referenced by index range.
typedef struct {
    const char *id;
    const char *text;
    int         separator;
    int         disabled;
    int         checked;
    // Shortcut — single ASCII char + modifier mask.
    char        shortcutChar;
    int         shortcutMods; // 1=Cmd, 2=Shift, 4=Alt, 8=Ctrl
    // Children in the flat array.
    int         childStart;
    int         childCount;
} NativeMenuItemC;

// Build and install the application menubar.
void nativemenuSetMenubar(const char *appName,
    NativeMenuItemC *menus, int menuCount,
    NativeMenuItemC *allItems, int itemCount,
    int includeEditMenu,
    int suppressSystemEditItems,
    const char *aboutActionID,
    int omitAboutItem,
    int includeWindowMenu);

// Remove the custom menubar (revert to default).
void nativemenuClearMenubar(void);

// System tray management.
int  nativemenuCreateTray(const void *iconData, int iconLen,
    const char *tooltip,
    NativeMenuItemC *items, int itemCount);
void nativemenuUpdateTray(int trayID,
    const void *iconData, int iconLen,
    const char *tooltip,
    NativeMenuItemC *items, int itemCount);
void nativemenuRemoveTray(int trayID);

#endif
