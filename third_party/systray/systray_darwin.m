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
  NSMutableSet<NSNumber*> *hiddenIds;
}

@synthesize window = _window;

- (void)applicationDidFinishLaunching:(NSNotification *)aNotification
{
  [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
  self->statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
  self->panelItems = [[NSMutableDictionary alloc] init];
  self->panelOrder = [[NSMutableArray alloc] init];
  self->separatorIds = [[NSMutableSet alloc] init];
  self->hiddenIds = [[NSMutableSet alloc] init];
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
  if (![self->hiddenIds containsObject:menuId]) {
    [self->hiddenIds addObject:menuId];
    [self rebuildPopoverContent];
  }
}

- (void) setMenuItemIcon:(NSArray*)imageAndMenuId {
  NSImage* image = [imageAndMenuId objectAtIndex:0];
  NSNumber* menuId = [imageAndMenuId objectAtIndex:1];

  (void)image;
  (void)menuId;
}

- (void) show_menu_item:(NSNumber*) menuId
{
  if ([self->hiddenIds containsObject:menuId]) {
    [self->hiddenIds removeObject:menuId];
    [self rebuildPopoverContent];
  }
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
    if ([self->hiddenIds containsObject:menuId]) {
      continue;
    }
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

    // The separator is the primary section boundary.  Keep the heading as a
    // fallback as well: on macOS menu callbacks can arrive out of order while
    // the status item is being rebuilt, and losing the separator would merge
    // the action buttons into the status card.
    if (!inActionSection && [self isActionSectionTitle:item->title]) {
      inActionSection = YES;
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
    backendCard = [self detailCard:backendItems];
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
  (void)title;
  CGFloat cardWidth = actionMode ? 108.0 : 200.0;
  CGFloat cardHeight = 148.0;
  NSView *card = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, cardWidth, cardHeight)];
  card.translatesAutoresizingMaskIntoConstraints = NO;
  card.wantsLayer = YES;
  card.layer.backgroundColor = [[NSColor colorWithRed:0.16 green:0.27 blue:0.58 alpha:0.92] CGColor];
  card.layer.cornerRadius = 14.0;

  NSStackView *stack = [[NSStackView alloc] init];
  stack.orientation = NSUserInterfaceLayoutOrientationVertical;
  stack.alignment = NSLayoutAttributeLeading;
  // Both upper cards use a fixed height. Distribute the four status rows and
  // three action buttons over that same height so neither card leaves a
  // variable blank area when the endpoint card is rebuilt.
  stack.spacing = actionMode ? 8.0 : 12.0;
  stack.translatesAutoresizingMaskIntoConstraints = NO;
  [card addSubview:stack];

  for (MenuItem *item in items) {
    if (actionMode) {
      [stack addArrangedSubview:[self actionCard:item]];
    } else {
      [stack addArrangedSubview:[self statusRow:item->title]];
    }
  }

  [NSLayoutConstraint activateConstraints:@[
    [card.widthAnchor constraintEqualToConstant:cardWidth],
    [card.heightAnchor constraintEqualToConstant:cardHeight],
    [stack.topAnchor constraintEqualToAnchor:card.topAnchor constant:12.0],
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
  label.usesSingleLineMode = YES;
  label.maximumNumberOfLines = 1;
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
    @"已停止", @"關閉", @"关闭", @"停止中", @"启动失败", @"啟動失敗", @"无法取得", @"無法取得", @"取得不可",
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
  NSArray<NSString *> *parts = [self splitTitleAndValue:title];
  NSString *key = [parts[0] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
  NSString *value = [parts[1] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
  if ([key isEqualToString:@"本機服務"] || [key isEqualToString:@"本机服务"] ||
      [key isEqualToString:@"遠端服務"] || [key isEqualToString:@"远端服务"]) {
    // Status rows use the same localized labels as endpoint rows. Only a URI
    // identifies an endpoint and should be moved to the lower detail card.
    return [value hasPrefix:@"integterm-vfs://"] ||
           [value hasPrefix:@"http://"] ||
           [value hasPrefix:@"https://"];
  }
  return [@[@"後端服務", @"后端服务", @"Backend Service", @"バックエンドサービス", @"백엔드 서비스"] containsObject:key] || [key hasPrefix:@"後端服務"];
}

- (BOOL)isActionSectionTitle:(NSString *)title
{
  NSString *trimmed = [title stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
  return [@[@"功能", @"Actions", @"機能", @"기능"] containsObject:trimmed];
}

- (NSView *)detailCard:(NSArray<MenuItem *> *)items
{
  CGFloat entryHeight = 30.0;
  CGFloat entrySpacing = 4.0;
  CGFloat cardHeight = items.count * entryHeight + (items.count - 1) * entrySpacing;
  NSView *card = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 316, cardHeight)];
  card.translatesAutoresizingMaskIntoConstraints = NO;
  card.wantsLayer = YES;
  card.layer.backgroundColor = [[NSColor colorWithRed:0.16 green:0.27 blue:0.58 alpha:0.92] CGColor];
  card.layer.cornerRadius = 14.0;

  NSStackView *stack = [[NSStackView alloc] init];
  stack.orientation = NSUserInterfaceLayoutOrientationVertical;
  stack.spacing = entrySpacing;
  stack.translatesAutoresizingMaskIntoConstraints = NO;
  [card addSubview:stack];
  for (MenuItem *entry in items) {
    NSArray<NSString *> *entryParts = [self splitTitleAndValue:entry->title];
    NSString *entryTitle = [entryParts[0] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *entryValue = [entryParts[1] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *entryLine = entryValue.length > 0 ? [NSString stringWithFormat:@"%@：%@", entryTitle, entryValue] : entryTitle;
    NSView *entryRow = [[NSView alloc] init];
    entryRow.translatesAutoresizingMaskIntoConstraints = NO;
    NSTextField *entryLabel = [NSTextField labelWithString:entryLine];
    entryLabel.font = [NSFont monospacedSystemFontOfSize:10.0 weight:NSFontWeightSemibold];
    entryLabel.textColor = [self colorForBackendLineValue:entryValue];
    entryLabel.lineBreakMode = NSLineBreakByTruncatingMiddle;
    entryLabel.translatesAutoresizingMaskIntoConstraints = NO;
    PanelActionButton *copyButton = [PanelActionButton buttonWithTitle:@"" target:self action:@selector(panelButtonPressed:)];
    copyButton.translatesAutoresizingMaskIntoConstraints = NO;
    copyButton.menuId = entry->menuId;
    copyButton.bordered = NO;
    copyButton.toolTip = entry->tooltip;
    copyButton.contentTintColor = [NSColor colorWithWhite:1.0 alpha:0.78];
    if (@available(macOS 11.0, *)) {
      copyButton.image = [NSImage imageWithSystemSymbolName:@"doc.on.doc" accessibilityDescription:entry->tooltip];
    }
    [entryRow addSubview:entryLabel];
    [entryRow addSubview:copyButton];
    [stack addArrangedSubview:entryRow];
    [NSLayoutConstraint activateConstraints:@[
      [entryRow.widthAnchor constraintEqualToConstant:292.0],
      [entryRow.heightAnchor constraintEqualToConstant:entryHeight],
      [entryLabel.leadingAnchor constraintEqualToAnchor:entryRow.leadingAnchor],
      [entryLabel.centerYAnchor constraintEqualToAnchor:entryRow.centerYAnchor],
      [entryLabel.trailingAnchor constraintLessThanOrEqualToAnchor:copyButton.leadingAnchor constant:-8.0],
      [copyButton.trailingAnchor constraintEqualToAnchor:entryRow.trailingAnchor],
      [copyButton.centerYAnchor constraintEqualToAnchor:entryRow.centerYAnchor],
      [copyButton.widthAnchor constraintEqualToConstant:26.0],
      [copyButton.heightAnchor constraintEqualToConstant:26.0]
    ]];
  }
  [NSLayoutConstraint activateConstraints:@[
    [card.widthAnchor constraintEqualToConstant:316.0],
    [card.heightAnchor constraintEqualToConstant:cardHeight],
    [stack.leadingAnchor constraintEqualToAnchor:card.leadingAnchor constant:12.0],
    [stack.trailingAnchor constraintEqualToAnchor:card.trailingAnchor constant:-12.0],
    [stack.topAnchor constraintEqualToAnchor:card.topAnchor],
    [stack.bottomAnchor constraintEqualToAnchor:card.bottomAnchor]
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
