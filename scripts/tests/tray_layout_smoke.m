// 以真正的 AppKit 按鈕及位圖 context 驗證排版，不建立系統選單列項目。
#import <Cocoa/Cocoa.h>
#ifndef INTEGTERM_SYSTRAY_SOURCE
#define INTEGTERM_SYSTRAY_SOURCE "../../third_party/systray/systray_darwin.m"
#endif
#include INTEGTERM_SYSTRAY_SOURCE

void systray_ready(void) {}
void systray_on_exit(void) {}
void systray_menu_item_selected(int menuId) { (void)menuId; }
void systray_reopen(void) {}

@interface SmokeStatusItem : NSObject
@property(strong) NSStatusBarButton *button;
@end
@implementation SmokeStatusItem
@end

static void require(BOOL condition, NSString *message) {
  if (!condition) {
    fprintf(stderr, "FAIL: %s\n", message.UTF8String);
    exit(1);
  }
}

static NSBitmapImageRep *render(NSImage *image, CGFloat scale, BOOL preview) {
  NSSize canvas = NSMakeSize(image.size.width + 4, image.size.height + 4);
  NSBitmapImageRep *bitmap = [[NSBitmapImageRep alloc]
    initWithBitmapDataPlanes:NULL pixelsWide:ceil(canvas.width * scale) pixelsHigh:ceil(canvas.height * scale)
    bitsPerSample:8 samplesPerPixel:4 hasAlpha:YES isPlanar:NO colorSpaceName:NSDeviceRGBColorSpace
    bytesPerRow:0 bitsPerPixel:0];
  [NSGraphicsContext saveGraphicsState];
  NSGraphicsContext.currentContext = [NSGraphicsContext graphicsContextWithBitmapImageRep:bitmap];
  CGContextScaleCTM(NSGraphicsContext.currentContext.CGContext, scale, scale);
  if (preview) {
    [NSColor.whiteColor setFill];
    NSRectFill(NSMakeRect(0, 0, canvas.width, canvas.height));
  }
  [image drawInRect:NSMakeRect(2, 2, image.size.width, image.size.height)
          fromRect:NSZeroRect operation:NSCompositingOperationSourceOver fraction:1.0];
  [NSGraphicsContext restoreGraphicsState];
  return bitmap;
}

static void checkPixels(NSImage *image, CGFloat scale, CGFloat iconWidth) {
  NSBitmapImageRep *bitmap = render(image, scale, NO);
  NSUInteger iconInk = 0, titleInk = 0;
  for (NSInteger y = 0; y < bitmap.pixelsHigh; y++) {
    for (NSInteger x = 0; x < bitmap.pixelsWide; x++) {
      if ([bitmap colorAtX:x y:y].alphaComponent < 0.05) continue;
      require(x > 0 && y > 0 && x < bitmap.pixelsWide - 1 && y < bitmap.pixelsHigh - 1,
              @"內容超出繪圖邊界");
      if (x < (2 + iconWidth) * scale) iconInk++; else titleInk++;
    }
  }
  require(iconInk > 0 && titleInk > 0, @"圖示或雙行文字未完整參與繪製");
}

int main(int argc, const char **argv) {
  @autoreleasepool {
    require(argc >= 2, @"請提供圖示檔案");
    [NSApplication sharedApplication];
    SmokeStatusItem *item = [SmokeStatusItem new];
    item.button = [[NSStatusBarButton alloc] initWithFrame:NSMakeRect(0, 0, 24, 22)];
    item.button.bordered = NO;
    IntegTERMSystrayAppDelegate *delegate = [IntegTERMSystrayAppDelegate new];
    [delegate setValue:item forKey:@"statusItem"];
    item.button.target = delegate;
    item.button.action = @selector(togglePopover:);
    [delegate setTooltip:@"背景服務"];

    NSImage *icon = [[NSImage alloc] initWithContentsOfFile:[NSString stringWithUTF8String:argv[1]]];
    require(icon != nil, @"圖示載入失敗");
    icon.size = NSMakeSize(18, 18);
    icon.template = YES;
    [delegate setIcon:icon];
    [delegate setTitle:@""];
    CGFloat iconOnlyWidth = item.button.cell.cellSize.width;
    CGFloat shortWidth = 0, longWidth = 0;
    NSArray<NSString *> *counts = @[@"0", @"1", @"9", @"10", @"999", @"123456789012"];
    for (NSString *count in counts) {
      [delegate setTitle:[NSString stringWithFormat:@"ACT\n%@", count]];
      NSImage *image = item.button.image;
      CGFloat textWidth = MAX([@"ACT" sizeWithAttributes:@{NSFontAttributeName: [NSFont systemFontOfSize:6.5 weight:NSFontWeightSemibold]}].width,
                              [count sizeWithAttributes:@{NSFontAttributeName: [NSFont systemFontOfSize:11.5 weight:NSFontWeightSemibold]}].width);
      require(image.size.width >= icon.size.width + textWidth, @"選單列圖像未包含完整標題寬度");
      require(item.button.cell.cellSize.width > iconOnlyWidth, @"原生按鈕只預留圖示寬度");
      require(image.isTemplate && item.button.imagePosition == NSImageOnly, @"原生深淺色樣式遺失");
      require([item.button.title isEqualToString:@""], @"重複顯示原生標題");
      if ([count isEqualToString:@"0"]) shortWidth = item.button.cell.cellSize.width;
      longWidth = item.button.cell.cellSize.width;

      for (NSNumber *height in @[@22, @24, @37]) {
        item.button.frame = NSMakeRect(0, 0, ceil(longWidth), height.doubleValue);
        NSRect rect = [(NSButtonCell *)item.button.cell imageRectForBounds:item.button.bounds];
        require(NSContainsRect(item.button.bounds, rect), @"選單列高度改變後圖像被裁切");
        require(rect.size.width >= image.size.width && rect.size.height >= image.size.height, @"圖像遭非預期縮小");
      }
      // 同一張 NSImage 交替畫到不同解析度，涵蓋換螢幕時的快取使用。
      for (NSNumber *scale in @[@1, @2, @1.5, @1, @2]) {
        checkPixels(image, scale.doubleValue, icon.size.width);
      }
      if (argc >= 3 && [count isEqualToString:@"0"]) {
        NSString *directory = [NSString stringWithUTF8String:argv[2]];
        for (NSNumber *scale in @[@1, @2]) {
          NSData *png = [render(image, scale.doubleValue, YES) representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
          [png writeToFile:[directory stringByAppendingPathComponent:[NSString stringWithFormat:@"tray-%@x.png", scale]] atomically:YES];
        }
      }
    }
    require(longWidth > shortWidth, @"多位數連線數未增加預留寬度");
    [delegate setTitle:@"ACT\n0"];
    require(item.button.cell.cellSize.width == shortWidth, @"連線數縮短後未收回多餘寬度");
    require(item.button.target == delegate && item.button.action == @selector(togglePopover:), @"點擊動作被修改");
    require([item.button.toolTip isEqualToString:@"背景服務"], @"提示文字被修改");

    [delegate setTitle:@"一般標題"];
    require(item.button.image == icon && item.button.imagePosition == NSImageLeft, @"單行標題未恢復原生配置");
    [delegate setTitle:@""];
    require(item.button.image == icon && item.button.cell.cellSize.width == iconOnlyWidth, @"空標題未恢復圖示尺寸");
    [delegate setIcon:nil];
    [delegate setTitle:@"ACT\n0"];
    require(item.button.image != nil && item.button.image.size.width < shortWidth, @"無圖示時標題配置錯誤");
    [delegate setIcon:icon];
    require(item.button.cell.cellSize.width == shortWidth, @"先設定標題再設定圖示時配置錯誤");
    printf("PASS: 6 種連線數、3 種選單列高度、1×／1.5×／2× 交替繪製與標題切換\n");
  }
  return 0;
}
