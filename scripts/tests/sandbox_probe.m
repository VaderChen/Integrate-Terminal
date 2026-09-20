#import <Foundation/Foundation.h>
#import <Security/Security.h>
#import <arpa/inet.h>
#import <errno.h>
#import <sys/socket.h>
#import <unistd.h>

// This standalone diagnostic only accesses a freshly generated, namespaced fake item.
// It never opens application stores, enumerates Keychain items, or starts the actual app.
static NSDictionary *probeKeychain(NSString *service, NSString *account, BOOL dataProtection) {
    NSMutableDictionary *query = [@{
        (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
        (__bridge id)kSecAttrService: service,
        (__bridge id)kSecAttrAccount: account,
        (__bridge id)kSecAttrSynchronizable: @NO,
        (__bridge id)kSecUseDataProtectionKeychain: @(dataProtection),
        (__bridge id)kSecUseAuthenticationUI: (__bridge id)kSecUseAuthenticationUIFail,
    } mutableCopy];
    NSData *first = [@"non-secret-sandbox-probe" dataUsingEncoding:NSUTF8StringEncoding];
    NSData *second = [@"updated-non-secret-sandbox-probe" dataUsingEncoding:NSUTF8StringEncoding];
    NSMutableDictionary *add = [query mutableCopy];
    add[(__bridge id)kSecValueData] = first;
    if (dataProtection) add[(__bridge id)kSecAttrAccessible] = (__bridge id)kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly;
    OSStatus status = SecItemAdd((__bridge CFDictionaryRef)add, NULL);
    NSMutableDictionary *result = [@{@"backend": dataProtection ? @"data-protection" : @"classic", @"add_status": @(status)} mutableCopy];
    if (status != errSecSuccess) return result; // A duplicate is not ours: never read or delete it.
    NSMutableDictionary *read = [query mutableCopy];
    read[(__bridge id)kSecReturnData] = @YES;
    read[(__bridge id)kSecMatchLimit] = (__bridge id)kSecMatchLimitOne;
    CFTypeRef value = NULL;
    OSStatus getStatus = SecItemCopyMatching((__bridge CFDictionaryRef)read, &value);
    result[@"read_status"] = @(getStatus);
    result[@"read_matches"] = @(getStatus == errSecSuccess && value && [(__bridge NSData *)value isEqualToData:first]);
    if (value) CFRelease(value);
    OSStatus updateStatus = SecItemUpdate((__bridge CFDictionaryRef)query, (__bridge CFDictionaryRef)@{(__bridge id)kSecValueData: second});
    result[@"update_status"] = @(updateStatus);
    value = NULL;
    getStatus = SecItemCopyMatching((__bridge CFDictionaryRef)read, &value);
    result[@"updated_read_status"] = @(getStatus);
    result[@"updated_read_matches"] = @(getStatus == errSecSuccess && value && [(__bridge NSData *)value isEqualToData:second]);
    if (value) CFRelease(value);
    result[@"delete_status"] = @(SecItemDelete((__bridge CFDictionaryRef)query));
    value = NULL;
    result[@"after_delete_status"] = @(SecItemCopyMatching((__bridge CFDictionaryRef)read, &value));
    if (value) CFRelease(value);
    return result;
}

static NSDictionary *probeListener(void) {
    int fd = socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) return @{@"socket_errno": @(errno)};
    struct sockaddr_in address = {0};
    address.sin_family = AF_INET;
    address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    address.sin_port = 0;
    int bindResult = bind(fd, (struct sockaddr *)&address, sizeof(address));
    int bindError = bindResult == 0 ? 0 : errno;
    int listenResult = bindResult == 0 ? listen(fd, 1) : -1;
    int listenError = bindResult == 0 && listenResult != 0 ? errno : 0;
    close(fd);
    return @{@"bind_result": @(bindResult), @"bind_errno": @(bindError), @"listen_result": @(listenResult), @"listen_errno": @(listenError)};
}

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        if (argc != 2) return 64;
        NSString *identifier = [NSString stringWithUTF8String:argv[1]];
        NSUUID *uuid = [[NSUUID alloc] initWithUUIDString:identifier];
        if (!uuid) return 64;
        NSString *bundleID = NSBundle.mainBundle.bundleIdentifier;
        NSString *expectedID = [@"com.vader.integterm.sandbox-probe." stringByAppendingString:identifier.lowercaseString];
        if (![bundleID isEqualToString:expectedID]) return 65;
        NSString *probeContainerSuffix = [NSString stringWithFormat:@"/Library/Containers/%@/Data", bundleID];
        BOOL probeContainer = [NSHomeDirectory() hasSuffix:probeContainerSuffix];
        if ([NSHomeDirectory() containsString:@"/Library/Containers/com.vader.integterm/"]) return 66;
        // Process-local: diagnostics must never prompt to unlock/modify the user's Keychain.
        SecKeychainSetUserInteractionAllowed(false);
        NSString *service = [@"com.vader.integterm.sandbox-probe." stringByAppendingString:identifier];
        NSDictionary *result = @{
            @"bundle_id": bundleID,
            @"home_is_probe_container": @(probeContainer),
            @"network": probeListener(),
            @"classic": probeKeychain(service, @"classic-fake-item", NO),
            @"data_protection": probeKeychain(service, @"data-protection-fake-item", YES),
        };
        NSData *encoded = [NSJSONSerialization dataWithJSONObject:result options:NSJSONWritingSortedKeys error:NULL];
        fwrite(encoded.bytes, 1, encoded.length, stdout);
        fputc('\n', stdout);
    }
    return 0;
}
