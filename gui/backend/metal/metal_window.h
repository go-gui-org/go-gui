#ifndef METAL_WINDOW_H
#define METAL_WINDOW_H

// metal_window.h — Native NSWindow + CAMetalLayer window manager.
// Replaces SDL2 window creation and event loop for the Metal backend.

#ifdef __OBJC__
#import <Cocoa/Cocoa.h>
#else
#include <stdint.h>
#endif

// Opaque window handle. Internally wraps NSWindow + MetalContentView.
typedef void *GoGuiNSWindow;

// ─── Lifecycle ─────────────────────────────────────────────────

// Create a window with a CAMetalLayer-backed content view.
// title: UTF-8 window title.
// width, height: logical pixels.
// fixedSize: if non-zero, window is not resizable.
// Returns NULL on failure.
GoGuiNSWindow metalWindowCreate(const char *title, int width, int height,
                                int fixedSize);

// Destroy the window and release all resources.
void metalWindowDestroy(GoGuiNSWindow w);

// ─── Properties ────────────────────────────────────────────────

// Set the window title.
void metalWindowSetTitle(GoGuiNSWindow w, const char *title);

// Get the window size in logical pixels.
void metalWindowGetSize(GoGuiNSWindow w, int *width, int *height);

// Get the framebuffer (drawable) size in physical pixels.
void metalWindowGetFramebufferSize(GoGuiNSWindow w, int *width, int *height);

// Get the CAMetalLayer pointer for the Metal rendering context.
void *metalWindowGetLayer(GoGuiNSWindow w);

// Get the window ID (unique per-process, matches gui.App registration).
unsigned int metalWindowGetID(GoGuiNSWindow w);

// ─── Event callbacks (set from Go via cgo) ─────────────────────

// Called when the window is resized. width/height in logical pixels.
// Set to NULL to disable.
extern void goMetalWindowResized(unsigned int windowID,
                                 int width, int height);

// Called when the user clicks the close button. Go calls
// gui.DispatchCloseRequest and may destroy the window.
extern void goMetalWindowShouldClose(unsigned int windowID);

// Called when the window gains or loses key (focus) status.
extern void goMetalWindowFocusChanged(unsigned int windowID,
                                      int focused);

// Called when files are dropped on the window.
extern void goMetalFileDrop(unsigned int windowID, char *path);

// ─── Event polling ─────────────────────────────────────────────

// Poll for the next event with an optional timeout.
// timeoutMs: timeout in milliseconds, or -1 to wait indefinitely.
// Returns 1 if an event is available, 0 on timeout.
// Call metalEvent* accessors to read the event fields.
int metalPollEvent(int timeoutMs);

// Event type constants. Must match the switch in events.go.
enum {
    METAL_EVENT_NONE = 0,
    METAL_EVENT_QUIT,
    METAL_EVENT_MOUSE_DOWN,
    METAL_EVENT_MOUSE_UP,
    METAL_EVENT_MOUSE_MOVE,
    METAL_EVENT_SCROLL_WHEEL,
    METAL_EVENT_KEY_DOWN,
    METAL_EVENT_KEY_UP,
    METAL_EVENT_FLAGS_CHANGED,
    METAL_EVENT_CHAR,       // IME committed text
    METAL_EVENT_IME_COMP,   // IME composition in progress
};

// Current event type (set after metalPollEvent returns 1).
int metalEventType(void);

// Current event window ID.
unsigned int metalEventWindowID(void);

// Mouse event fields.
float metalEventMouseX(void);
float metalEventMouseY(void);
float metalEventMouseDX(void);
float metalEventMouseDY(void);
int   metalEventMouseButton(void);   // 0=left, 1=right, 2=middle
int   metalEventClickCount(void);
float metalEventScrollX(void);
float metalEventScrollY(void);
int   metalEventScrollPhase(void);   // 0=normal, 1=maybegin, 2=began, 3=ended

// Keyboard event fields.
unsigned short metalEventKeyCode(void);   // macOS virtual key code
unsigned int   metalEventModifiers(void); // NSEventModifierFlags
int            metalEventKeyRepeat(void);

// IME / text event fields.
const char *metalEventText(void);     // UTF-8, caller must not free
int         metalEventIMEStart(void); // composition start offset
int         metalEventIMELength(void); // composition length

// ─── Cursors ───────────────────────────────────────────────────

// Set the system cursor for the given window. mouseX/mouseY are the
// last-known mouse position in content-view coordinates (origin top-left).
// When the mouse is outside the content bounds (e.g. in the title bar or
// window border), the call is a no-op — the window server owns cursor
// management for the frame area.
// cursorName: NSCursor class method selector name (e.g., "arrowCursor").
void metalWindowSetCursor(GoGuiNSWindow w, const char *cursorName,
                          float mouseX, float mouseY);

// ─── Vibrancy ──────────────────────────────────────────────────

// Set the window's translucent backdrop material (macOS NSVisualEffectView).
// material: 0 = None (opaque window, effect removed); 1..N map to
// NSVisualEffectMaterial (see metalWindowSetVibrancy in the .m file). Makes the
// window and its CAMetalLayer non-opaque so the backdrop shows through content
// drawn with a translucent clear color.
void metalWindowSetVibrancy(GoGuiNSWindow w, int material);

// ─── Clipboard ─────────────────────────────────────────────────

// Get the clipboard text. Returns NULL if empty or not a string.
// Caller must free with free().
char *metalClipboardGet(void);

// Set the clipboard text.
void metalClipboardSet(const char *text);

// ─── Bridge helper ─────────────────────────────────────────────

// Get the NSWindow pointer from the opaque handle.
// Used by bridge files (a11y, etc.) to access the Cocoa window.
void *metalWindowGetNSWindow(GoGuiNSWindow w);

// ─── IME (Input Method Editor) ─────────────────────────────────

// Set the IME cursor rectangle (in window-relative logical coords).
void metalWindowIMESetCursorRect(GoGuiNSWindow win,
                                 float x, float y, float width, float height);

// Enable or disable IME for the window.
void metalWindowIMESetActive(GoGuiNSWindow w, int active);

// ─── Wake ─────────────────────────────────────────────────────

// Post an empty event to break the event loop out of a wait.
// Safe to call from any goroutine.
void metalPostEmptyEvent(void);

// ─── App-level ─────────────────────────────────────────────────
//
// Launch sequence (see metal_window_darwin.m for the full narrative):
//   1. metalAppInit         — before any window: NSApplication singleton,
//                             activation policy, menu bar, app delegate.
//   2. metalWindowCreate    — order each window front.
//   3. metalAppFinishLaunch — after windows exist: complete the Launch
//                             Services handshake, bring app to foreground.

// Initialize the NSApplication singleton, menu bar, and delegate.
// Must be called once before creating any windows. Idempotent.
void metalAppInit(void);

// Complete launch and bring the app to the foreground. Must be called
// after the first window exists (required for .app bundles to display
// on the active Space). Runs the Launch Services handshake once;
// re-activates the app on every call so dynamically-opened windows
// come forward.
void metalAppFinishLaunch(void);

// Set the dock icon from PNG data.
void metalSetDockIcon(const void *data, int len);

#endif // METAL_WINDOW_H
