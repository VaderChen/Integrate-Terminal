//go:build darwin && cgo

package credentials

/*
#cgo darwin CFLAGS: -mmacosx-version-min=12.0
#cgo darwin LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

// Use one file-based vault for both ad-hoc local builds and signed sandbox
// builds. Switching implementations based on entitlements would strand existing
// references. No access group or ACL override is supplied: Security.framework
// applies its default ACL, trusting the creating application, not every app.
// Never change the user's default keychain or keychain search list.
static CFMutableDictionaryRef integterm_query(const char *account, int nonInteractive) {
    CFMutableDictionaryRef query = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    if (!query) return NULL;
    CFStringRef accountValue = CFStringCreateWithCString(kCFAllocatorDefault, account, kCFStringEncodingUTF8);
    if (!accountValue) { CFRelease(query); return NULL; }
    CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
    CFDictionarySetValue(query, kSecAttrService, CFSTR("com.vader.integterm.credentials.v1"));
    CFDictionarySetValue(query, kSecAttrAccount, accountValue);
    CFDictionarySetValue(query, kSecUseDataProtectionKeychain, kCFBooleanFalse);
    CFDictionarySetValue(query, kSecAttrSynchronizable, kCFBooleanFalse);
    if (nonInteractive) {
        // Legacy/file-based Keychain also requires the scoped process flag
        // below; query-only UI controls are not sufficient for that vault.
        #pragma clang diagnostic push
        #pragma clang diagnostic ignored "-Wdeprecated-declarations"
        CFDictionarySetValue(query, kSecUseAuthenticationUI, kSecUseAuthenticationUIFail);
        #pragma clang diagnostic pop
    }
    CFRelease(accountValue);
    return query;
}

static void integterm_free_secret(void *bytes, size_t length) {
    if (bytes) {
        volatile unsigned char *cursor = bytes;
        while (length-- > 0) *cursor++ = 0;
        free(bytes);
    }
}

// These deprecated APIs are still required by the file-based Keychain. They
// control this process's UI behavior, not item ACLs or system authorization.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
static OSStatus integterm_get_interaction(Boolean *allowed) {
    return SecKeychainGetUserInteractionAllowed(allowed);
}
static OSStatus integterm_set_interaction(Boolean allowed) {
    return SecKeychainSetUserInteractionAllowed(allowed);
}
#pragma clang diagnostic pop

static OSStatus integterm_get(const char *account, int nonInteractive, void **bytes, int *length) {
    *bytes = NULL;
    *length = 0;
    CFMutableDictionaryRef query = integterm_query(account, nonInteractive);
    if (!query) return errSecAllocate;
    CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
    CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
    CFTypeRef result = NULL;
    OSStatus status = SecItemCopyMatching(query, &result);
    CFRelease(query);
    if (status != errSecSuccess) { if (result) CFRelease(result); return status; }
    if (!result || CFGetTypeID(result) != CFDataGetTypeID()) {
        if (result) CFRelease(result);
        return errSecDecode;
    }
    CFIndex count = CFDataGetLength((CFDataRef)result);
    if (count < 0 || count > 1024 * 1024) { CFRelease(result); return errSecDataTooLarge; }
    void *copy = malloc(count > 0 ? (size_t)count : 1);
    if (!copy) { CFRelease(result); return errSecAllocate; }
    if (count > 0) memcpy(copy, CFDataGetBytePtr((CFDataRef)result), (size_t)count);
    CFRelease(result);
    *bytes = copy;
    *length = (int)count;
    return errSecSuccess;
}

static OSStatus integterm_put(const char *account, int nonInteractive, const void *bytes, int length) {
    CFMutableDictionaryRef query = integterm_query(account, nonInteractive);
    if (!query) return errSecAllocate;
    CFDataRef data = CFDataCreate(kCFAllocatorDefault, bytes, length);
    if (!data) { CFRelease(query); return errSecAllocate; }
    CFMutableDictionaryRef changes = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    if (!changes) { CFRelease(data); CFRelease(query); return errSecAllocate; }
    CFDictionarySetValue(changes, kSecValueData, data);
    OSStatus status = SecItemUpdate(query, changes);
    if (status == errSecItemNotFound) {
        CFMutableDictionaryRef attributes = CFDictionaryCreateMutableCopy(kCFAllocatorDefault, 0, query);
        if (!attributes) { status = errSecAllocate; }
        else {
            CFDictionarySetValue(attributes, kSecValueData, data);
            CFDictionarySetValue(attributes, kSecAttrLabel, CFSTR("IntegTERM credentials"));
            status = SecItemAdd(attributes, NULL);
            CFRelease(attributes);
            // Another process may have created this exact account meanwhile.
            if (status == errSecDuplicateItem) status = SecItemUpdate(query, changes);
        }
    }
    CFRelease(changes);
    CFRelease(data);
    CFRelease(query);
    return status;
}

static OSStatus integterm_delete(const char *account, int nonInteractive) {
    CFMutableDictionaryRef query = integterm_query(account, nonInteractive);
    if (!query) return errSecAllocate;
    OSStatus status = SecItemDelete(query);
    CFRelease(query);
    return status;
}
*/
import "C"

import (
	"sync"
	"unsafe"
)

type nativeDriver interface {
	get(string, bool) ([]byte, int32)
	put(string, []byte, bool) int32
	delete(string, bool) int32
	interactionAllowed() (bool, int32)
	setInteractionAllowed(bool) int32
}

type keychainBackend struct {
	driver         nativeDriver
	nonInteractive bool
}
type securityDriver struct{}

// Every native call from this package participates, including GUI backends, so
// one call cannot observe another call's temporary process-local UI flag.
var nativeCallMu sync.Mutex

// New does not query or modify Keychain until a Backend method is called.
func New() Backend { return &keychainBackend{driver: securityDriver{}} }

// NewNonInteractive preserves the same vault and ACLs but fails when Keychain
// needs a user prompt. Creating the backend has no native side effects.
func NewNonInteractive() Backend {
	return &keychainBackend{driver: securityDriver{}, nonInteractive: true}
}

func (b *keychainBackend) nativeCall(work func() int32) (status int32) {
	nativeCallMu.Lock()
	defer nativeCallMu.Unlock()
	if !b.nonInteractive {
		return work()
	}
	allowed, status := b.driver.interactionAllowed()
	if status != 0 {
		return status
	}
	if status = b.driver.setInteractionAllowed(false); status != 0 {
		return status
	}
	defer func() {
		restoreStatus := b.driver.setInteractionAllowed(allowed)
		if status == 0 {
			status = restoreStatus
		}
	}()
	return work()
}

func (b *keychainBackend) Get(ref string) ([]byte, error) {
	if err := validateReference(ref); err != nil {
		return nil, err
	}
	var secret []byte
	status := b.nativeCall(func() int32 {
		var status int32
		secret, status = b.driver.get(ref, b.nonInteractive)
		return status
	})
	if err := statusError("get", status); err != nil {
		clear(secret)
		return nil, err
	}
	return secret, nil
}

func (b *keychainBackend) Put(ref string, secret []byte) error {
	if err := validateReference(ref); err != nil {
		return err
	}
	if len(secret) > maxSecretSize {
		return ErrTooLarge
	}
	return statusError("put", b.nativeCall(func() int32 { return b.driver.put(ref, secret, b.nonInteractive) }))
}

func (b *keychainBackend) Delete(ref string) error {
	if err := validateReference(ref); err != nil {
		return err
	}
	return statusError("delete", b.nativeCall(func() int32 { return b.driver.delete(ref, b.nonInteractive) }))
}

func (securityDriver) interactionAllowed() (bool, int32) {
	var allowed C.Boolean
	status := C.integterm_get_interaction(&allowed)
	return allowed != 0, int32(status)
}

func (securityDriver) setInteractionAllowed(allowed bool) int32 {
	return int32(C.integterm_set_interaction(C.Boolean(nativeBool(allowed))))
}

func nativeBool(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

func (securityDriver) get(ref string, nonInteractive bool) ([]byte, int32) {
	account := C.CString(ref)
	defer C.free(unsafe.Pointer(account))
	var data unsafe.Pointer
	var length C.int
	status := C.integterm_get(account, nativeBool(nonInteractive), &data, &length)
	if data != nil {
		defer C.integterm_free_secret(data, C.size_t(length))
	}
	if status != C.errSecSuccess {
		return nil, int32(status)
	}
	return C.GoBytes(data, length), int32(status)
}

func (securityDriver) put(ref string, secret []byte, nonInteractive bool) int32 {
	account := C.CString(ref)
	defer C.free(unsafe.Pointer(account))
	var data unsafe.Pointer
	if len(secret) > 0 {
		data = C.CBytes(secret)
		defer C.integterm_free_secret(data, C.size_t(len(secret)))
	}
	return int32(C.integterm_put(account, nativeBool(nonInteractive), data, C.int(len(secret))))
}

func (securityDriver) delete(ref string, nonInteractive bool) int32 {
	account := C.CString(ref)
	defer C.free(unsafe.Pointer(account))
	return int32(C.integterm_delete(account, nativeBool(nonInteractive)))
}

func statusError(operation string, status int32) error {
	if status == int32(C.errSecSuccess) {
		return nil
	}
	err := &Error{Operation: operation, Status: status}
	switch status {
	case int32(C.errSecItemNotFound):
		err.Cause = ErrNotFound
	case int32(C.errSecInteractionNotAllowed), int32(C.errSecInteractionRequired):
		err.Cause = ErrLocked
	case int32(C.errSecAuthFailed), int32(C.errSecUserCanceled), int32(C.errSecMissingEntitlement), int32(C.errSecReadOnly):
		err.Cause = ErrDenied
	case int32(C.errSecNotAvailable), int32(C.errSecNoSuchKeychain), int32(C.errSecInvalidKeychain):
		err.Cause = ErrUnavailable
	case int32(C.errSecDataTooLarge):
		err.Cause = ErrTooLarge
	}
	return err
}
