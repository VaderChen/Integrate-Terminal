#import <Cocoa/Cocoa.h>
#include <stdio.h>
#include "systray.h"

#if __MAC_OS_X_VERSION_MIN_REQUIRED < 101400

    #ifndef NSControlStateValueOff
      #define NSControlStateValueOff NSOffState
    #endif

    #ifndef NSControlStateValueOn
      #define NSControlStateValueOn NSOnState
    #endif

#endif

@interface MenuItem : NSObject
{
  @public
    NSNumber* menuId;
    NSNumber* parentMenuId;
    NSString* title;
    NSString* tooltip;
    short disabled;
    short checked;
}
-(id) initWithId: (int)theMenuId
withParentMenuId: (int)theParentMenuId
       withTitle: (const char*)theTitle
     withTooltip: (const char*)theTooltip
    withDisabled: (short)theDisabled
     withChecked: (short)theChecked;
     @end
@implementation MenuItem
     -(id) initWithId: (int)theMenuId
     withParentMenuId: (int)theParentMenuId
            withTitle: (const char*)theTitle
          withTooltip: (const char*)theTooltip
         withDisabled: (short)theDisabled
          withChecked: (short)theChecked
{
  menuId = [NSNumber numberWithInt:theMenuId];
  parentMenuId = [NSNumber numberWithInt:theParentMenuId];
  title = [[NSString alloc] initWithCString:theTitle
                                   encoding:NSUTF8StringEncoding];
  tooltip = [[NSString alloc] initWithCString:theTooltip
                                     encoding:NSUTF8StringEncoding];
  disabled = theDisabled;
  checked = theChecked;
  return self;
}
@end

@interface PanelActionButton : NSButton
@property (nonatomic, strong) NSNumber *menuId;
@end

@implementation PanelActionButton
- (BOOL)acceptsFirstMouse:(NSEvent *)event
{
  (void)event;
  return YES;
}
@end

@interface IntegTERMSystrayAppDelegate: NSObject <NSApplicationDelegate>
  - (void) add_or_update_menu_item:(MenuItem*) item;
  - (IBAction)menuHandler:(id)sender;
  - (IBAction)panelButtonPressed:(id)sender;
  - (IBAction)togglePopover:(id)sender;
  @property (assign) IBOutlet NSWindow *window;
  @end

  @implementation IntegTERMSystrayAppDelegate
{
  NSStatusItem *statusItem;
  NSPopover *popover;
  NSViewController *popoverController;
  NSStackView *popoverStack;
  NSStackView *contentColumns;
  NSMutableDictionary<NSNumber*, MenuItem*> *panelItems;
  NSMutableArray<NSNumber*> *panelOrder;
  NSMutableSet<NSNumber*> *separatorIds;
}

@synthesize window = _window;

- (void)applicationDidFinishLaunching:(NSNotification *)aNotification
{
  [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
  self->statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
  self->panelItems = [[NSMutableDictionary alloc] init];
  self->panelOrder = [[NSMutableArray alloc] init];
  self->separatorIds = [[NSMutableSet alloc] init];
  self->popover = [[NSPopover alloc] init];
  self->popover.behavior = NSPopoverBehaviorApplicationDefined;
  self->popoverController = [[NSViewController alloc] init];
  self->popover.contentViewController = self->popoverController;

  NSView *contentView = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 340, 170)];
  contentView.wantsLayer = YES;
  contentView.layer.backgroundColor = [[NSColor colorWithRed:0.10 green:0.17 blue:0.38 alpha:0.98] CGColor];
  contentView.layer.cornerRadius = 18.0;
  self->popoverController.view = contentView;

  self->popoverStack = [[NSStackView alloc] init];
  self->popoverStack.orientation = NSUserInterfaceLayoutOrientationVertical;
  self->popoverStack.alignment = NSLayoutAttributeLeading;
  self->popoverStack.spacing = 8.0;
  self->popoverStack.translatesAutoresizingMaskIntoConstraints = NO;
  [contentView addSubview:self->popoverStack];

  [NSLayoutConstraint activateConstraints:@[
    [self->popoverStack.topAnchor constraintEqualToAnchor:contentView.topAnchor constant:12.0],
    [self->popoverStack.leadingAnchor constraintEqualToAnchor:contentView.leadingAnchor constant:12.0],
    [self->popoverStack.trailingAnchor constraintEqualToAnchor:contentView.trailingAnchor constant:-12.0],
    [self->popoverStack.bottomAnchor constraintLessThanOrEqualToAnchor:contentView.bottomAnchor constant:-12.0]
  ]];

  statusItem.button.target = self;
  statusItem.button.action = @selector(togglePopover:);
  systray_ready();
}

- (void)applicationWillTerminate:(NSNotification *)aNotification
{
  systray_on_exit();
}

- (BOOL)applicationShouldHandleReopen:(NSApplication *)sender hasVisibleWindows:(BOOL)flag
{
  (void)sender;
  (void)flag;
  systray_reopen();
  return NO;
}

- (void)application:(NSApplication *)application openURLs:(NSArray<NSURL *> *)urls
{
  (void)application;
  (void)urls;
  systray_reopen();
}

- (void)setIcon:(NSImage *)image {
  statusItem.button.image = image;
  [self updateTitleButtonStyle];
}

- (void)setTitle:(NSString *)title {
  NSArray<NSString *> *parts = [title componentsSeparatedByString:@"\n"];
  if ([parts count] >= 2) {
    NSString *line1 = parts[0];
    NSString *line2 = parts[1];
    [self setCustomTitleTop:line1 bottom:line2];
    statusItem.button.attributedTitle = [[NSAttributedString alloc] initWithString:@""];
    statusItem.button.title = @"";
  } else {
    [self clearCustomTitle];
    statusItem.button.attributedTitle = [[NSAttributedString alloc] initWithString:@""];
    statusItem.button.title = title;
  }
  [self updateTitleButtonStyle];
}

-(void)updateTitleButtonStyle {
  if (statusItem.button.image != nil) {
    if ([statusItem.button.title length] == 0) {
      statusItem.button.imagePosition = NSImageOnly;
    } else {
      statusItem.button.imagePosition = NSImageLeft;
    }
  } else {
    statusItem.button.imagePosition = NSNoImage;
  }
}


- (void)setTooltip:(NSString *)tooltip {
  statusItem.button.toolTip = tooltip;
}

- (void)setCustomTitleTop:(NSString *)top bottom:(NSString *)bottom {
  NSStatusBarButton *button = statusItem.button;
  NSStackView *stack = nil;
  for (NSView *subview in button.subviews) {
    if ([subview isKindOfClass:[NSStackView class]] && [subview.identifier isEqualToString:@"IntegTERMCustomTitleStack"]) {
      stack = (NSStackView *)subview;
      break;
    }
  }
  NSTextField *topLabel;
  NSTextField *bottomLabel;

  if (stack == nil) {
    stack = [[NSStackView alloc] init];
    stack.identifier = @"IntegTERMCustomTitleStack";
    stack.orientation = NSUserInterfaceLayoutOrientationVertical;
    stack.alignment = NSLayoutAttributeCenterX;
    stack.spacing = -3.0;
    stack.translatesAutoresizingMaskIntoConstraints = NO;

    topLabel = [NSTextField labelWithString:@""];
    topLabel.identifier = @"IntegTERMCustomTitleTop";
    topLabel.alignment = NSTextAlignmentCenter;
    topLabel.font = [NSFont systemFontOfSize:6.5 weight:NSFontWeightSemibold];

    bottomLabel = [NSTextField labelWithString:@""];
    bottomLabel.identifier = @"IntegTERMCustomTitleBottom";
    bottomLabel.alignment = NSTextAlignmentCenter;
    bottomLabel.font = [NSFont systemFontOfSize:11.5 weight:NSFontWeightSemibold];

    [stack addArrangedSubview:topLabel];
    [stack addArrangedSubview:bottomLabel];
    [button addSubview:stack];

    [NSLayoutConstraint activateConstraints:@[
      [stack.centerYAnchor constraintEqualToAnchor:button.centerYAnchor constant:-0.5],
      [stack.trailingAnchor constraintEqualToAnchor:button.trailingAnchor constant:-1.0],
      [stack.leadingAnchor constraintGreaterThanOrEqualToAnchor:button.leadingAnchor constant:38.0]
    ]];
  } else {
    topLabel = nil;
    bottomLabel = nil;
    for (NSView *subview in stack.arrangedSubviews) {
      if ([subview isKindOfClass:[NSTextField class]] && [subview.identifier isEqualToString:@"IntegTERMCustomTitleTop"]) {
        topLabel = (NSTextField *)subview;
      }
      if ([subview isKindOfClass:[NSTextField class]] && [subview.identifier isEqualToString:@"IntegTERMCustomTitleBottom"]) {
        bottomLabel = (NSTextField *)subview;
      }
    }
  }

  topLabel.stringValue = top;
  bottomLabel.stringValue = bottom;
  stack.hidden = NO;
}

- (void)clearCustomTitle {
  for (NSView *subview in statusItem.button.subviews) {
    if ([subview isKindOfClass:[NSStackView class]] && [subview.identifier isEqualToString:@"IntegTERMCustomTitleStack"]) {
      subview.hidden = YES;
    }
  }
}

- (IBAction)menuHandler:(id)sender
{
  NSNumber* menuId = nil;
  if ([sender isKindOfClass:[PanelActionButton class]]) {
    menuId = ((PanelActionButton *)sender).menuId;
  } else if ([sender respondsToSelector:@selector(representedObject)]) {
    menuId = [sender representedObject];
  } else if ([sender respondsToSelector:@selector(tag)]) {
    menuId = [NSNumber numberWithInteger:[sender tag]];
  }
  if (menuId == nil || menuId.intValue == 0) {
    fprintf(stderr, "IntegTERM tray: menuHandler ignored\n");
    fflush(stderr);
    return;
  }
  fprintf(stderr, "IntegTERM tray: menuHandler menuId=%d\n", menuId.intValue);
  fflush(stderr);
  systray_menu_item_selected(menuId.intValue);
  [popover close];
}

- (IBAction)panelButtonPressed:(id)sender
{
  fprintf(stderr, "IntegTERM tray: panelButtonPressed\n");
  fflush(stderr);
  [self menuHandler:sender];
}

- (void)add_or_update_menu_item:(MenuItem *)item {
  self->panelItems[item->menuId] = item;
  if (![self->panelOrder containsObject:item->menuId]) {
    [self->panelOrder addObject:item->menuId];
  }
  [self rebuildPopoverContent];
}

NSMenuItem *find_menu_item(NSMenu *ourMenu, NSNumber *menuId) {
  NSMenuItem *foundItem = [ourMenu itemWithTag:[menuId integerValue]];
  if (foundItem != NULL) {
    return foundItem;
  }
  NSArray *menu_items = ourMenu.itemArray;
  int i;
  for (i = 0; i < [menu_items count]; i++) {
    NSMenuItem *i_item = [menu_items objectAtIndex:i];
    if (i_item.hasSubmenu) {
      foundItem = find_menu_item(i_item.submenu, menuId);
      if (foundItem != NULL) {
        return foundItem;
      }
    }
  }

  return NULL;
};

- (void) add_separator:(NSNumber*) menuId
{
  if (![self->panelOrder containsObject:menuId]) {
    [self->panelOrder addObject:menuId];
  }
  [self->separatorIds addObject:menuId];
  [self rebuildPopoverContent];
}

- (void) hide_menu_item:(NSNumber*) menuId
{
  (void)menuId;
}

- (void) setMenuItemIcon:(NSArray*)imageAndMenuId {
  NSImage* image = [imageAndMenuId objectAtIndex:0];
  NSNumber* menuId = [imageAndMenuId objectAtIndex:1];

  (void)image;
  (void)menuId;
}

- (void) show_menu_item:(NSNumber*) menuId
{
  (void)menuId;
}

- (IBAction)togglePopover:(id)sender
{
  if ([popover isShown]) {
    [popover close];
    return;
  }
  [self rebuildPopoverContent];
  [popover showRelativeToRect:[statusItem.button bounds] ofView:statusItem.button preferredEdge:NSRectEdgeMinY];
  (void)sender;
}

- (void)rebuildPopoverContent
{
  while (self->popoverStack.arrangedSubviews.count > 0) {
    NSView *view = self->popoverStack.arrangedSubviews[0];
    [self->popoverStack removeArrangedSubview:view];
    [view removeFromSuperview];
  }

  NSMutableArray<MenuItem *> *statusItems = [[NSMutableArray alloc] init];
  NSMutableArray<MenuItem *> *backendItems = [[NSMutableArray alloc] init];
  NSMutableArray<MenuItem *> *actionItems = [[NSMutableArray alloc] init];
  NSString *statusTitle = @"";
  NSString *actionTitle = @"";
  BOOL inActionSection = NO;

  for (NSNumber *menuId in self->panelOrder) {
    if ([self->separatorIds containsObject:menuId]) {
      inActionSection = YES;
      continue;
    }

    MenuItem *item = self->panelItems[menuId];
    if (item == nil) {
      continue;
    }

    if (!inActionSection && statusTitle.length == 0) {
      statusTitle = item->title;
      continue;
    }

    if (inActionSection && actionTitle.length == 0) {
      actionTitle = item->title;
      continue;
    }

    if (!inActionSection) {
      if ([self isBackendServiceItem:item->title]) {
        [backendItems addObject:item];
      } else {
        [statusItems addObject:item];
      }
      continue;
    }

    [actionItems addObject:item];
  }

  NSStackView *row = [[NSStackView alloc] init];
  row.orientation = NSUserInterfaceLayoutOrientationHorizontal;
  row.alignment = NSLayoutAttributeTop;
  row.distribution = NSStackViewDistributionFill;
  row.spacing = 8.0;
  row.translatesAutoresizingMaskIntoConstraints = NO;

  NSView *statusCard = nil;
  NSView *backendCard = nil;
  NSView *actionCard = nil;
  if (statusItems.count > 0) {
    NSString *resolvedStatusTitle = statusTitle.length > 0 ? statusTitle : @"Status";
    statusCard = [self sectionCard:resolvedStatusTitle items:statusItems actionMode:NO];
  }
  if (backendItems.count > 0) {
    MenuItem *backendItem = backendItems[0];
    backendCard = [self detailCard:backendItem];
  }
  if (statusCard != nil) {
    [row addArrangedSubview:statusCard];
  }
  if (actionItems.count > 0) {
    NSString *resolvedActionTitle = actionTitle.length > 0 ? actionTitle : @"Actions";
    actionCard = [self sectionCard:resolvedActionTitle items:actionItems actionMode:YES];
    [row addArrangedSubview:actionCard];
  }

  if (statusCard != nil && actionCard != nil) {
    [statusCard.widthAnchor constraintEqualToConstant:200.0].active = YES;
    [actionCard.widthAnchor constraintEqualToConstant:108.0].active = YES;
  }
  [self->popoverStack addArrangedSubview:row];

  if (backendCard != nil) {
    [self->popoverStack addArrangedSubview:backendCard];
  }
}

- (NSView *)sectionCard:(NSString *)title items:(NSArray<MenuItem *> *)items actionMode:(BOOL)actionMode
{
  CGFloat cardWidth = actionMode ? 108.0 : 200.0;
  NSView *card = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, cardWidth, actionMode ? 122 : 122)];
  card.translatesAutoresizingMaskIntoConstraints = NO;
  card.wantsLayer = YES;
  card.layer.backgroundColor = [[NSColor colorWithRed:0.16 green:0.27 blue:0.58 alpha:0.92] CGColor];
  card.layer.cornerRadius = 14.0;

  NSStackView *headerRow = [[NSStackView alloc] init];
  headerRow.orientation = NSUserInterfaceLayoutOrientationHorizontal;
  headerRow.alignment = NSLayoutAttributeCenterY;
  headerRow.spacing = 6.0;
  headerRow.translatesAutoresizingMaskIntoConstraints = NO;

  NSImageView *iconView = [[NSImageView alloc] init];
  iconView.translatesAutoresizingMaskIntoConstraints = NO;
  NSImage *symbol = nil;
  if (@available(macOS 11.0, *)) {
    NSString *symbolName = actionMode ? @"bolt.circle" : @"list.bullet.rectangle";
    symbol = [NSImage imageWithSystemSymbolName:symbolName accessibilityDescription:title];
    if (symbol != nil) {
      NSImageSymbolConfiguration *config = [NSImageSymbolConfiguration configurationWithPointSize:12.0 weight:NSFontWeightSemibold];
      symbol = [symbol imageWithSymbolConfiguration:config];
      symbol.template = YES;
    }
  }
  iconView.image = symbol;
  iconView.contentTintColor = [NSColor colorWithWhite:1.0 alpha:0.82];

  NSTextField *label = [NSTextField labelWithString:title];
  label.font = [NSFont systemFontOfSize:11.0 weight:NSFontWeightSemibold];
  label.textColor = [NSColor colorWithWhite:1.0 alpha:0.74];
  label.translatesAutoresizingMaskIntoConstraints = NO;

  [headerRow addArrangedSubview:iconView];
  [headerRow addArrangedSubview:label];
  [card addSubview:headerRow];

  NSStackView *stack = [[NSStackView alloc] init];
  stack.orientation = NSUserInterfaceLayoutOrientationVertical;
  stack.alignment = NSLayoutAttributeLeading;
  stack.spacing = actionMode ? 8.0 : 6.0;
  stack.translatesAutoresizingMaskIntoConstraints = NO;
  [card addSubview:stack];

  for (MenuItem *item in items) {
    if (actionMode) {
      [stack addArrangedSubview:[self actionCard:item]];
    } else {
      [stack addArrangedSubview:[self statusRow:item->title]];
    }
  }

  CGFloat estimatedHeight = 18.0 + 10.0 + (items.count * (actionMode ? 36.0 : 22.0)) + ((items.count > 0 ? items.count - 1 : 0) * (actionMode ? 8.0 : 4.0)) + 12.0;
  [NSLayoutConstraint activateConstraints:@[
    [card.widthAnchor constraintEqualToConstant:cardWidth],
    [card.heightAnchor constraintGreaterThanOrEqualToConstant:estimatedHeight],
    [headerRow.topAnchor constraintEqualToAnchor:card.topAnchor constant:10.0],
    [headerRow.leadingAnchor constraintEqualToAnchor:card.leadingAnchor constant:12.0],
    [headerRow.trailingAnchor constraintLessThanOrEqualToAnchor:card.trailingAnchor constant:-12.0],
    [iconView.widthAnchor constraintEqualToConstant:14.0],
    [iconView.heightAnchor constraintEqualToConstant:14.0],
    [stack.topAnchor constraintEqualToAnchor:headerRow.bottomAnchor constant:8.0],
    [stack.leadingAnchor constraintEqualToAnchor:card.leadingAnchor constant:12.0],
    [stack.trailingAnchor constraintEqualToAnchor:card.trailingAnchor constant:-12.0],
    [stack.bottomAnchor constraintEqualToAnchor:card.bottomAnchor constant:-12.0]
  ]];

  return card;
}

- (NSArray<NSString *> *)splitTitleAndValue:(NSString *)title
{
  NSRange fullWidth = [title rangeOfString:@"："];
  if (fullWidth.location != NSNotFound) {
    return @[[title substringToIndex:fullWidth.location], [title substringFromIndex:fullWidth.location + 1]];
  }
  NSRange normal = [title rangeOfString:@":"];
  if (normal.location != NSNotFound) {
    return @[[title substringToIndex:normal.location], [title substringFromIndex:normal.location + 1]];
  }
  return @[title, @""];
}

- (NSView *)statusRow:(NSString *)title
{
  NSArray<NSString *> *parts = [self splitTitleAndValue:title];
  NSView *row = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 176, 22)];
  row.translatesAutoresizingMaskIntoConstraints = NO;

  NSString *valueString = [parts[1] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
  NSString *line = valueString.length > 0 ? [NSString stringWithFormat:@"%@：%@", parts[0], valueString] : parts[0];
  NSTextField *label = [NSTextField labelWithString:line];
  label.font = [NSFont systemFontOfSize:10.5 weight:NSFontWeightSemibold];
  label.textColor = [self colorForStatusLine:line];
  label.lineBreakMode = NSLineBreakByTruncatingTail;
  label.translatesAutoresizingMaskIntoConstraints = NO;

  [row addSubview:label];

  [NSLayoutConstraint activateConstraints:@[
    [row.widthAnchor constraintEqualToConstant:176.0],
    [row.heightAnchor constraintEqualToConstant:22.0],
    [label.centerYAnchor constraintEqualToAnchor:row.centerYAnchor],
    [label.leadingAnchor constraintEqualToAnchor:row.leadingAnchor],
    [label.trailingAnchor constraintEqualToAnchor:row.trailingAnchor]
  ]];

  return row;
}

- (NSColor *)colorForStatusLine:(NSString *)line
{
  NSArray<NSString *> *alertKeywords = @[
    @"已停止", @"停止中", @"启动失败", @"啟動失敗", @"无法取得", @"無法取得", @"取得不可",
    @"Stopped", @"Failed", @"Unavailable", @"停止", @"起動失敗", @"取得不可",
    @"중지됨", @"시작 실패", @"사용 불가"
  ];
  for (NSString *keyword in alertKeywords) {
    if ([line rangeOfString:keyword].location != NSNotFound) {
      return [NSColor colorWithRed:1.0 green:0.42 blue:0.40 alpha:1.0];
    }
  }
  return [NSColor whiteColor];
}

- (BOOL)isBackendServiceItem:(NSString *)title
{
  NSString *key = [[self splitTitleAndValue:title][0] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
  return [@[@"後端服務", @"后端服务", @"Backend Service", @"バックエンドサービス", @"백엔드 서비스"] containsObject:key];
}

- (NSView *)detailCard:(MenuItem *)item
{
  NSArray<NSString *> *parts = [self splitTitleAndValue:item->title];
  NSString *title = [parts[0] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
  NSString *value = [parts[1] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];

  NSView *card = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 316, 42)];
  card.translatesAutoresizingMaskIntoConstraints = NO;
  card.wantsLayer = YES;
  card.layer.backgroundColor = [[NSColor colorWithRed:0.16 green:0.27 blue:0.58 alpha:0.92] CGColor];
  card.layer.cornerRadius = 14.0;

  NSString *line = value.length > 0 ? [NSString stringWithFormat:@"%@：%@", title, value] : title;
  NSTextField *label = [NSTextField labelWithString:line];
  label.font = [NSFont monospacedSystemFontOfSize:10.0 weight:NSFontWeightSemibold];
  label.textColor = [self colorForBackendLineValue:value];
  label.lineBreakMode = NSLineBreakByTruncatingMiddle;
  label.translatesAutoresizingMaskIntoConstraints = NO;

  [card addSubview:label];

  [NSLayoutConstraint activateConstraints:@[
    [card.widthAnchor constraintEqualToConstant:316.0],
    [card.heightAnchor constraintEqualToConstant:42.0],
    [label.centerYAnchor constraintEqualToAnchor:card.centerYAnchor],
    [label.leadingAnchor constraintEqualToAnchor:card.leadingAnchor constant:12.0],
    [label.trailingAnchor constraintEqualToAnchor:card.trailingAnchor constant:-12.0]
  ]];

  return card;
}

- (NSColor *)colorForBackendLineValue:(NSString *)value
{
  NSString *trimmed = [value stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
  if ([trimmed hasPrefix:@"http://"] || [trimmed hasPrefix:@"https://"]) {
    return [NSColor whiteColor];
  }
  return [NSColor colorWithWhite:1.0 alpha:0.55];
}

- (NSView *)actionCard:(MenuItem *)item
{
  NSView *container = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 84, 34)];
  container.translatesAutoresizingMaskIntoConstraints = NO;

  PanelActionButton *button = [PanelActionButton buttonWithTitle:item->title target:nil action:nil];
  button.translatesAutoresizingMaskIntoConstraints = NO;
  button.bezelStyle = NSBezelStyleRegularSquare;
  button.wantsLayer = YES;
  button.layer.backgroundColor = [[NSColor colorWithRed:0.20 green:0.47 blue:0.95 alpha:1.0] CGColor];
  button.layer.cornerRadius = 10.0;
  button.font = [NSFont systemFontOfSize:11.0 weight:NSFontWeightSemibold];
  button.contentTintColor = [NSColor whiteColor];
  button.bordered = NO;
  button.toolTip = item->tooltip;
  button.menuId = item->menuId;
  button.target = self;
  button.action = @selector(panelButtonPressed:);
  button.buttonType = NSButtonTypeMomentaryPushIn;
  button.refusesFirstResponder = YES;
  button.continuous = NO;
  button.transparent = NO;
  button.enabled = item->disabled == 0;
  button.alphaValue = item->disabled == 0 ? 1.0 : 0.45;
  [container addSubview:button];

  [NSLayoutConstraint activateConstraints:@[
    [container.widthAnchor constraintEqualToConstant:84.0],
    [container.heightAnchor constraintEqualToConstant:34.0],
    [button.topAnchor constraintEqualToAnchor:container.topAnchor],
    [button.leadingAnchor constraintEqualToAnchor:container.leadingAnchor],
    [button.trailingAnchor constraintEqualToAnchor:container.trailingAnchor],
    [button.bottomAnchor constraintEqualToAnchor:container.bottomAnchor]
  ]];

  return container;
}

- (void) quit
{
  [NSApp terminate:self];
}

@end

void registerSystray(void) {
  IntegTERMSystrayAppDelegate *delegate = [[IntegTERMSystrayAppDelegate alloc] init];
  [[NSApplication sharedApplication] setDelegate:delegate];
  // A workaround to avoid crashing on macOS versions before Catalina. Somehow
  // SIGSEGV would happen inside AppKit if [NSApp run] is called from a
  // different function, even if that function is called right after this.
  if (floor(NSAppKitVersionNumber) <= /*NSAppKitVersionNumber10_14*/ 1671){
    [NSApp run];
  }
}

int nativeLoop(void) {
  if (floor(NSAppKitVersionNumber) > /*NSAppKitVersionNumber10_14*/ 1671){
    [NSApp run];
  }
  return EXIT_SUCCESS;
}

void runInMainThread(SEL method, id object) {
  [(IntegTERMSystrayAppDelegate*)[NSApp delegate]
    performSelectorOnMainThread:method
                     withObject:object
                  waitUntilDone: YES];
}

void setIcon(const char* iconBytes, int length, bool template) {
  NSData* buffer = [NSData dataWithBytes: iconBytes length:length];
  NSImage *image = [[NSImage alloc] initWithData:buffer];
  [image setSize:NSMakeSize(18, 18)];
  image.template = template;
  runInMainThread(@selector(setIcon:), (id)image);
}

void setMenuItemIcon(const char* iconBytes, int length, int menuId, bool template) {
  NSData* buffer = [NSData dataWithBytes: iconBytes length:length];
  NSImage *image = [[NSImage alloc] initWithData:buffer];
  [image setSize:NSMakeSize(16, 16)];
  image.template = template;
  NSNumber *mId = [NSNumber numberWithInt:menuId];
  runInMainThread(@selector(setMenuItemIcon:), @[image, (id)mId]);
}

void setTitle(char* ctitle) {
  NSString* title = [[NSString alloc] initWithCString:ctitle
                                             encoding:NSUTF8StringEncoding];
  free(ctitle);
  runInMainThread(@selector(setTitle:), (id)title);
}

void setTooltip(char* ctooltip) {
  NSString* tooltip = [[NSString alloc] initWithCString:ctooltip
                                               encoding:NSUTF8StringEncoding];
  free(ctooltip);
  runInMainThread(@selector(setTooltip:), (id)tooltip);
}

void add_or_update_menu_item(int menuId, int parentMenuId, char* title, char* tooltip, short disabled, short checked, short isCheckable) {
  MenuItem* item = [[MenuItem alloc] initWithId: menuId withParentMenuId: parentMenuId withTitle: title withTooltip: tooltip withDisabled: disabled withChecked: checked];
  free(title);
  free(tooltip);
  runInMainThread(@selector(add_or_update_menu_item:), (id)item);
}

void add_separator(int menuId) {
  NSNumber *mId = [NSNumber numberWithInt:menuId];
  runInMainThread(@selector(add_separator:), (id)mId);
}

void hide_menu_item(int menuId) {
  NSNumber *mId = [NSNumber numberWithInt:menuId];
  runInMainThread(@selector(hide_menu_item:), (id)mId);
}

void show_menu_item(int menuId) {
  NSNumber *mId = [NSNumber numberWithInt:menuId];
  runInMainThread(@selector(show_menu_item:), (id)mId);
}

void quit() {
  runInMainThread(@selector(quit), nil);
}
