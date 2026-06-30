//go:build darwin && !ios

#import <AppKit/AppKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include "dialog_darwin.h"
#include <stdlib.h>
#include <string.h>

// Build an array of UTTypes from lowercase extension strings.
// Returns nil (allow all) when extCount == 0.
static NSArray<UTType *> *utTypesFromExtensions(
    const char **extensions, int extCount) {
    if (extCount == 0) return nil;
    NSMutableArray<UTType *> *types =
        [NSMutableArray arrayWithCapacity:extCount];
    for (int i = 0; i < extCount; i++) {
        NSString *ext =
            [NSString stringWithUTF8String:extensions[i]];
        UTType *t =
            [UTType typeWithFilenameExtension:ext];
        if (t != nil) [types addObject:t];
    }
    return types.count > 0 ? types : nil;
}

// Generate a security-scoped bookmark for a URL.
// Returns malloc'd data and sets *outLen, or NULL on failure.
static void *bookmarkForURL(NSURL *url, int *outLen) {
    NSError *err = nil;
    NSData *data = [url
        bookmarkDataWithOptions:
            NSURLBookmarkCreationWithSecurityScope
        includingResourceValuesForKeys:nil
        relativeToURL:nil
        error:&err];
    if (data == nil) {
        *outLen = 0;
        return NULL;
    }
    *outLen = (int)data.length;
    void *buf = malloc(data.length);
    memcpy(buf, data.bytes, data.length);
    return buf;
}

// Populate a DialogResult from an array of NSURLs.
static DialogResult resultFromURLs(NSArray<NSURL *> *urls) {
    DialogResult r = {0};
    r.status = DIALOG_OK;
    int count = (int)urls.count;
    r.pathCount = count;
    r.paths = (char **)calloc(count, sizeof(char *));
    r.bookmarkData = (void **)calloc(count, sizeof(void *));
    r.bookmarkLens = (int *)calloc(count, sizeof(int));
    for (int i = 0; i < count; i++) {
        const char *p = urls[i].fileSystemRepresentation;
        r.paths[i] = strdup(p);
        r.bookmarkData[i] = bookmarkForURL(urls[i],
            &r.bookmarkLens[i]);
    }
    return r;
}

// Defined below, near the alert helpers; used by the panels too.
static void activateForModal(void);

static void setDirectoryURL(NSSavePanel *panel, const char *dir) {
    if (dir != NULL && dir[0] != '\0') {
        NSString *s = [NSString stringWithUTF8String:dir];
        panel.directoryURL = [NSURL fileURLWithPath:s];
    }
}

DialogResult filedialogOpen(const char *title, const char *startDir,
    const char **extensions, int extCount, int allowMultiple) {
    @autoreleasepool {
        NSOpenPanel *panel = [NSOpenPanel openPanel];
        if (title != NULL && title[0] != '\0')
            panel.title = [NSString stringWithUTF8String:title];
        setDirectoryURL(panel, startDir);
        panel.canChooseFiles = YES;
        panel.canChooseDirectories = NO;
        panel.allowsMultipleSelection = (allowMultiple != 0);

        NSArray<UTType *> *types =
            utTypesFromExtensions(extensions, extCount);
        if (types != nil)
            panel.allowedContentTypes = types;

        activateForModal();
        NSModalResponse resp = [panel runModal];
        if (resp != NSModalResponseOK) {
            DialogResult r = {0};
            r.status = DIALOG_CANCEL;
            return r;
        }
        return resultFromURLs(panel.URLs);
    }
}

DialogResult filedialogSave(const char *title, const char *startDir,
    const char *defaultName, const char *defaultExt,
    const char **extensions, int extCount, int confirmOverwrite) {
    (void)confirmOverwrite; // NSSavePanel always confirms.
    @autoreleasepool {
        NSSavePanel *panel = [NSSavePanel savePanel];
        if (title != NULL && title[0] != '\0')
            panel.title = [NSString stringWithUTF8String:title];
        setDirectoryURL(panel, startDir);
        panel.canCreateDirectories = YES;

        if (defaultName != NULL && defaultName[0] != '\0')
            panel.nameFieldStringValue =
                [NSString stringWithUTF8String:defaultName];

        // Build allowed types from extensions + defaultExt.
        NSArray<UTType *> *types =
            utTypesFromExtensions(extensions, extCount);
        if (types != nil)
            panel.allowedContentTypes = types;

        activateForModal();
        NSModalResponse resp = [panel runModal];
        if (resp != NSModalResponseOK) {
            DialogResult r = {0};
            r.status = DIALOG_CANCEL;
            return r;
        }
        return resultFromURLs(@[panel.URL]);
    }
}

DialogResult filedialogFolder(const char *title, const char *startDir) {
    @autoreleasepool {
        NSOpenPanel *panel = [NSOpenPanel openPanel];
        if (title != NULL && title[0] != '\0')
            panel.title = [NSString stringWithUTF8String:title];
        setDirectoryURL(panel, startDir);
        panel.canChooseFiles = NO;
        panel.canChooseDirectories = YES;
        panel.allowsMultipleSelection = NO;

        activateForModal();
        NSModalResponse resp = [panel runModal];
        if (resp != NSModalResponseOK) {
            DialogResult r = {0};
            r.status = DIALOG_CANCEL;
            return r;
        }
        return resultFromURLs(panel.URLs);
    }
}

static NSAlertStyle alertStyleFromLevel(int level) {
    switch (level) {
    case ALERT_WARNING:  return NSAlertStyleWarning;
    case ALERT_CRITICAL: return NSAlertStyleCritical;
    default:             return NSAlertStyleInformational;
    }
}

// Force the app frontmost and pump the run loop until activation lands,
// so a modal NSAlert / NSSavePanel reliably becomes key on its FIRST
// presentation. Manual-pump backends (metal) start the modal loop
// before the asynchronous activateIgnoringOtherApps: takes effect,
// leaving the dialog non-key (Return/Esc/Tab dead) until a later one
// happens to come up after activation — the "first no, second yes"
// race. Spinning here makes activation synchronous w.r.t. runModal.
// No-op when already active; bounded so it degrades to prior behavior
// if activation never lands.
static void activateForModal(void) {
    // Nil guard: with NSApp nil, [nil isActive] is NO and
    // [nil nextEventMatchingMask:...] returns immediately rather than
    // blocking to the deadline, turning the wait into a 0.5s hot spin.
    if (NSApp == nil || [NSApp isActive]) return;
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    [NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:0.5];
    while (![NSApp isActive] && [deadline timeIntervalSinceNow] > 0) {
        NSEvent *e = [NSApp nextEventMatchingMask:NSEventMaskAny
            untilDate:deadline
            inMode:NSDefaultRunLoopMode
            dequeue:YES];
        if (e != nil) [NSApp sendEvent:e];
    }
}

AlertResult filedialogMessage(const char *title, const char *body,
    int level) {
    @autoreleasepool {
        NSAlert *alert = [[NSAlert alloc] init];
        alert.alertStyle = alertStyleFromLevel(level);
        if (title != NULL && title[0] != '\0')
            alert.messageText =
                [NSString stringWithUTF8String:title];
        if (body != NULL && body[0] != '\0')
            alert.informativeText =
                [NSString stringWithUTF8String:body];
        [alert addButtonWithTitle:@"OK"];
        activateForModal();
        [alert runModal];
        AlertResult r = {0};
        r.status = DIALOG_OK;
        return r;
    }
}

AlertResult filedialogConfirm(const char *title, const char *body,
    int level) {
    @autoreleasepool {
        NSAlert *alert = [[NSAlert alloc] init];
        alert.alertStyle = alertStyleFromLevel(level);
        if (title != NULL && title[0] != '\0')
            alert.messageText =
                [NSString stringWithUTF8String:title];
        if (body != NULL && body[0] != '\0')
            alert.informativeText =
                [NSString stringWithUTF8String:body];
        [alert addButtonWithTitle:@"OK"];
        [alert addButtonWithTitle:@"Cancel"];
        activateForModal();
        NSModalResponse resp = [alert runModal];
        AlertResult r = {0};
        r.status = (resp == NSAlertFirstButtonReturn)
            ? DIALOG_OK : DIALOG_CANCEL;
        return r;
    }
}

// filedialogSaveDiscard shows a 3-button Save/Don't Save/Cancel alert.
// Button order on macOS: Save (right/default), Cancel (middle), Don't Save (left).
// NSAlertFirstButtonReturn  → Save    → DIALOG_OK
// NSAlertSecondButtonReturn → Cancel  → DIALOG_CANCEL
// NSAlertThirdButtonReturn  → Discard → DIALOG_DISCARD
AlertResult filedialogSaveDiscard(const char *title, const char *body,
    int level) {
    @autoreleasepool {
        NSAlert *alert = [[NSAlert alloc] init];
        alert.alertStyle = alertStyleFromLevel(level);
        if (title != NULL && title[0] != '\0')
            alert.messageText =
                [NSString stringWithUTF8String:title];
        if (body != NULL && body[0] != '\0')
            alert.informativeText =
                [NSString stringWithUTF8String:body];
        [alert addButtonWithTitle:@"Save"];
        [alert addButtonWithTitle:@"Cancel"];
        [alert addButtonWithTitle:@"Don't Save"];
        activateForModal();
        NSModalResponse resp = [alert runModal];
        AlertResult r = {0};
        if (resp == NSAlertFirstButtonReturn)
            r.status = DIALOG_OK;
        else if (resp == NSAlertThirdButtonReturn)
            r.status = DIALOG_DISCARD;
        else
            r.status = DIALOG_CANCEL;
        return r;
    }
}

void filedialogFreeResult(DialogResult r) {
    for (int i = 0; i < r.pathCount; i++) {
        free(r.paths[i]);
        free(r.bookmarkData[i]);
    }
    free(r.paths);
    free(r.bookmarkData);
    free(r.bookmarkLens);
    free(r.errorMessage);
}
