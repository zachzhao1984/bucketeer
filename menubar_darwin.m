//go:build darwin && cgo

#import <Cocoa/Cocoa.h>

extern void bktMenuBarEvent(int action, const char *payload);

@interface BKTDropView : NSView <NSDraggingDestination>
@property(nonatomic, strong) NSMutableArray<NSString *> *temporaryDirectories;
@end

@implementation BKTDropView
- (instancetype)initWithFrame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (self) {
        _temporaryDirectories = [NSMutableArray new];
        [self registerForDraggedTypes:@[NSPasteboardTypeFileURL, NSPasteboardTypePNG, NSPasteboardTypeTIFF]];
        [self setAccessibilityElement:YES];
        [self setAccessibilityRole:NSAccessibilityButtonRole];
        [self setAccessibilityLabel:@"Bucketeer uploads"];
        self.toolTip = @"Drop files to upload to your default bucket";
    }
    return self;
}
- (void)drawRect:(NSRect)dirtyRect {
    NSImage *image = [NSImage imageWithSystemSymbolName:@"shippingbox.fill" accessibilityDescription:@"Bucketeer"];
    [image drawInRect:NSInsetRect(self.bounds, 5, 3)];
}
- (void)mouseDown:(NSEvent *)event {
    bktMenuBarEvent(4, NULL);
    [NSMenu popUpContextMenu:self.menu withEvent:event forView:self];
}
- (BOOL)accessibilityPerformPress {
    bktMenuBarEvent(4, NULL);
    [self.menu popUpMenuPositioningItem:nil atLocation:NSMakePoint(0, 0) inView:self];
    return YES;
}
- (void)rightMouseDown:(NSEvent *)event { [self mouseDown:event]; }
- (void)menuAction:(NSMenuItem *)item { bktMenuBarEvent((int)item.tag, NULL); }
- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {
    NSPasteboard *pb = sender.draggingPasteboard;
    BOOL files = [pb canReadObjectForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
    return (files || [pb availableTypeFromArray:@[NSPasteboardTypePNG, NSPasteboardTypeTIFF]]) ? NSDragOperationCopy : NSDragOperationNone;
}
- (NSDragOperation)draggingUpdated:(id<NSDraggingInfo>)sender { return [self draggingEntered:sender]; }
- (BOOL)prepareForDragOperation:(id<NSDraggingInfo>)sender { return [self draggingEntered:sender] != NSDragOperationNone; }
- (BOOL)performDragOperation:(id<NSDraggingInfo>)sender {
    NSPasteboard *pb = sender.draggingPasteboard;
    NSMutableArray *paths = [NSMutableArray new];
    NSArray *urls = [pb readObjectsForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
    for (NSURL *url in urls) { if (url.isFileURL) [paths addObject:url.path]; }
    if (paths.count == 0) {
        NSData *png = [pb dataForType:NSPasteboardTypePNG];
        if (!png) {
            NSData *tiff = [pb dataForType:NSPasteboardTypeTIFF];
            NSBitmapImageRep *bitmap = tiff ? [NSBitmapImageRep imageRepWithData:tiff] : nil;
            png = [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        }
        if (png) {
            NSString *dir = [NSTemporaryDirectory() stringByAppendingPathComponent:[[NSUUID UUID] UUIDString]];
            if (![[NSFileManager defaultManager] createDirectoryAtPath:dir withIntermediateDirectories:YES attributes:@{NSFilePosixPermissions: @0700} error:nil]) return NO;
            NSString *path = [dir stringByAppendingPathComponent:[NSString stringWithFormat:@"image-%@.png", [[NSUUID UUID] UUIDString]]];
            if (![png writeToFile:path options:NSDataWritingAtomic error:nil]) {
                [[NSFileManager defaultManager] removeItemAtPath:dir error:nil];
                return NO;
            }
            [self.temporaryDirectories addObject:dir];
            [paths addObject:path];
        }
    }
    if (paths.count == 0) return NO;
    NSData *json = [NSJSONSerialization dataWithJSONObject:paths options:0 error:nil];
    NSString *text = [[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding];
    bktMenuBarEvent(3, text.UTF8String);
    return YES;
}
@end

static NSStatusItem *bktItem;
static BKTDropView *bktDropView;
static NSMenuItem *bktTarget;

void bktMenuBarStart(void) {
    bktItem = [[NSStatusBar systemStatusBar] statusItemWithLength:28];
    bktItem.autosaveName = @"bucketeer.upload";
    bktDropView = [[BKTDropView alloc] initWithFrame:bktItem.button.bounds];
    bktDropView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    NSMenu *menu = [[NSMenu alloc] initWithTitle:@"Bucketeer"];
    bktTarget = [[NSMenuItem alloc] initWithTitle:@"Default bucket: not set" action:nil keyEquivalent:@""];
    [menu addItem:bktTarget];
    NSArray *titles = @[@"Open Bucketeer", @"Use Current Bucket as Default", @"Quit Bucketeer"];
    for (NSInteger i = 0; i < titles.count; i++) {
        NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:titles[i] action:@selector(menuAction:) keyEquivalent:@""];
        item.target = bktDropView;
        item.tag = i;
        [menu addItem:item];
    }
    bktDropView.menu = menu;
    [bktItem.button addSubview:bktDropView];
}
void bktMenuBarTarget(const char *title) { bktTarget.title = [NSString stringWithUTF8String:title]; }
void bktMenuBarStop(void) {
    [[NSStatusBar systemStatusBar] removeStatusItem:bktItem];
    for (NSString *dir in bktDropView.temporaryDirectories) {
        [[NSFileManager defaultManager] removeItemAtPath:dir error:nil];
    }
    bktDropView = nil;
    bktTarget = nil;
    bktItem = nil;
}
