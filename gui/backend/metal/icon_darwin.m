#import <Cocoa/Cocoa.h>

void metalActivateApp(void) {
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    [NSApp activateIgnoringOtherApps:YES];
}

void metalSetDockIcon(const void *data, int len) {
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

    NSData *d = [NSData dataWithBytes:data length:len];
    NSImage *img = [[NSImage alloc] initWithData:d];
    if (!img) return;

    [NSApp setApplicationIconImage:img];

    // For non-bundled (CLI-launched) apps, the Dock tile ignores
    // applicationIconImage. Explicitly set a content view and
    // force a display to update the tile.
    NSDockTile *tile = [NSApp dockTile];
    NSImageView *iv = [[NSImageView alloc] init];
    [iv setImage:img];
    [tile setContentView:iv];
    [tile display];
}
