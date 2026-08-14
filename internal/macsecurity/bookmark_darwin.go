//go:build darwin

// Package macsecurity 提供 App Sandbox 下存取容器外檔案所需的原生能力。
//
// 沙箱中透過檔案對話框選取的檔案，授權只在該次執行期間有效；App 重新啟動後
// 直接用先前存下的路徑開檔會得到 operation not permitted。
// security-scoped bookmark 是 Apple 提供的持久化機制：選檔當下建立 bookmark，
// 之後每次啟動解析它並呼叫 CFURLStartAccessingSecurityScopedResource 就能重新取得存取權。
//
// 這裡刻意使用 CoreFoundation 的 C API（而非 NSURL 的 Objective-C API），
// 因此不需要額外的 Swift/ObjC 動態庫，也不必調整簽章與打包流程。
package macsecurity

/*
#cgo darwin LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	unsigned char *bytes;
	int            length;
	char          *error;
} IntegTermBookmarkResult;

typedef struct {
	char *path;
	int   stale;
	int   accessing;
	void *url;
	char *error;
} IntegTermResolveResult;

static char *integtermCopyErrorDescription(CFErrorRef error) {
	if (error == NULL) {
		return strdup("unknown error");
	}
	CFStringRef description = CFErrorCopyDescription(error);
	if (description == NULL) {
		return strdup("unknown error");
	}
	CFIndex maxSize = CFStringGetMaximumSizeForEncoding(CFStringGetLength(description), kCFStringEncodingUTF8) + 1;
	char *buffer = (char *)malloc((size_t)maxSize);
	if (buffer == NULL) {
		CFRelease(description);
		return NULL;
	}
	if (!CFStringGetCString(description, buffer, maxSize, kCFStringEncodingUTF8)) {
		free(buffer);
		CFRelease(description);
		return strdup("unknown error");
	}
	CFRelease(description);
	return buffer;
}

static IntegTermBookmarkResult integtermCreateBookmark(const char *path, int isDirectory) {
	IntegTermBookmarkResult result;
	result.bytes = NULL;
	result.length = 0;
	result.error = NULL;

	CFStringRef pathRef = CFStringCreateWithCString(kCFAllocatorDefault, path, kCFStringEncodingUTF8);
	if (pathRef == NULL) {
		result.error = strdup("cannot convert path to CFString");
		return result;
	}

	CFURLRef url = CFURLCreateWithFileSystemPath(kCFAllocatorDefault, pathRef, kCFURLPOSIXPathStyle, isDirectory ? true : false);
	CFRelease(pathRef);
	if (url == NULL) {
		result.error = strdup("cannot create CFURL for path");
		return result;
	}

	CFErrorRef cfError = NULL;
	CFDataRef bookmark = CFURLCreateBookmarkData(
		kCFAllocatorDefault,
		url,
		kCFURLBookmarkCreationWithSecurityScope,
		NULL,
		NULL,
		&cfError);
	CFRelease(url);

	if (bookmark == NULL) {
		result.error = integtermCopyErrorDescription(cfError);
		if (cfError != NULL) {
			CFRelease(cfError);
		}
		return result;
	}

	CFIndex length = CFDataGetLength(bookmark);
	if (length <= 0) {
		CFRelease(bookmark);
		result.error = strdup("bookmark data is empty");
		return result;
	}

	result.bytes = (unsigned char *)malloc((size_t)length);
	if (result.bytes == NULL) {
		CFRelease(bookmark);
		result.error = strdup("out of memory");
		return result;
	}
	CFDataGetBytes(bookmark, CFRangeMake(0, length), result.bytes);
	result.length = (int)length;
	CFRelease(bookmark);
	return result;
}

static IntegTermResolveResult integtermResolveBookmark(const unsigned char *bytes, int length) {
	IntegTermResolveResult result;
	result.path = NULL;
	result.stale = 0;
	result.accessing = 0;
	result.url = NULL;
	result.error = NULL;

	CFDataRef data = CFDataCreate(kCFAllocatorDefault, bytes, (CFIndex)length);
	if (data == NULL) {
		result.error = strdup("cannot create CFData for bookmark");
		return result;
	}

	Boolean isStale = false;
	CFErrorRef cfError = NULL;
	CFURLRef url = CFURLCreateByResolvingBookmarkData(
		kCFAllocatorDefault,
		data,
		kCFURLBookmarkResolutionWithSecurityScope,
		NULL,
		NULL,
		&isStale,
		&cfError);
	CFRelease(data);

	if (url == NULL) {
		result.error = integtermCopyErrorDescription(cfError);
		if (cfError != NULL) {
			CFRelease(cfError);
		}
		return result;
	}

	if (CFURLStartAccessingSecurityScopedResource(url)) {
		result.accessing = 1;
	}

	char buffer[4096];
	if (CFURLGetFileSystemRepresentation(url, true, (UInt8 *)buffer, (CFIndex)sizeof(buffer))) {
		result.path = strdup(buffer);
	} else {
		result.error = strdup("cannot read resolved path");
	}

	result.stale = isStale ? 1 : 0;
	result.url = (void *)url;
	return result;
}

// 釋放 resolve 取得的資源。停止存取後必須 CFRelease，否則會洩漏 CFURL。
static void integtermReleaseResolved(void *url, int accessing) {
	if (url == NULL) {
		return;
	}
	if (accessing) {
		CFURLStopAccessingSecurityScopedResource((CFURLRef)url);
	}
	CFRelease((CFURLRef)url);
}
*/
import "C"

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"unsafe"
)

// ErrUnsupported 在非 macOS 平台回傳。
var ErrUnsupported = errors.New("security-scoped bookmark 僅支援 macOS")

// Available 回報目前平台是否支援 security-scoped bookmark。
func Available() bool { return true }

// CreateBookmark 為 path 建立 security-scoped bookmark，回傳 base64 字串以便存進 JSON。
//
// 必須在使用者剛透過檔案對話框選取該檔案、App 仍持有存取權時呼叫，否則會失敗。
func CreateBookmark(path string) (string, error) {
	if path == "" {
		return "", errors.New("路徑為空")
	}

	isDirectory := 0
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		isDirectory = 1
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	result := C.integtermCreateBookmark(cPath, C.int(isDirectory))
	if result.error != nil {
		message := C.GoString(result.error)
		C.free(unsafe.Pointer(result.error))
		if result.bytes != nil {
			C.free(unsafe.Pointer(result.bytes))
		}
		return "", fmt.Errorf("建立 bookmark 失敗: %s", message)
	}
	if result.bytes == nil || result.length <= 0 {
		return "", errors.New("建立 bookmark 失敗: 未取得資料")
	}

	raw := C.GoBytes(unsafe.Pointer(result.bytes), result.length)
	C.free(unsafe.Pointer(result.bytes))
	return base64.StdEncoding.EncodeToString(raw), nil
}

// Access 代表一次已啟動的 security-scoped 存取。用畢必須呼叫 Release。
type Access struct {
	// Path 是 bookmark 解析出的實際路徑，可能與當初建立時不同（檔案被搬移過）。
	Path string
	// Stale 為 true 表示 bookmark 已過期，應在存取成功後重新建立並存回。
	Stale bool

	url       unsafe.Pointer
	accessing bool
	released  bool
}

// Release 停止存取並釋放原生資源。重複呼叫是安全的。
func (a *Access) Release() {
	if a == nil || a.released || a.url == nil {
		return
	}
	accessing := 0
	if a.accessing {
		accessing = 1
	}
	C.integtermReleaseResolved(a.url, C.int(accessing))
	a.released = true
	a.url = nil
}

// ResolveBookmark 解析 base64 bookmark 並啟動 security-scoped 存取。
//
// 呼叫端務必在讀完檔案後呼叫 Access.Release，否則會持續佔用存取權與記憶體。
func ResolveBookmark(encoded string) (*Access, error) {
	if encoded == "" {
		return nil, errors.New("bookmark 為空")
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("bookmark 解碼失敗: %w", err)
	}
	if len(raw) == 0 {
		return nil, errors.New("bookmark 內容為空")
	}

	result := C.integtermResolveBookmark(
		(*C.uchar)(unsafe.Pointer(&raw[0])),
		C.int(len(raw)),
	)

	if result.error != nil {
		message := C.GoString(result.error)
		C.free(unsafe.Pointer(result.error))
		if result.path != nil {
			C.free(unsafe.Pointer(result.path))
		}
		C.integtermReleaseResolved(result.url, result.accessing)
		return nil, fmt.Errorf("解析 bookmark 失敗: %s", message)
	}

	access := &Access{
		Stale:     result.stale == 1,
		accessing: result.accessing == 1,
		url:       unsafe.Pointer(result.url),
	}
	if result.path != nil {
		access.Path = C.GoString(result.path)
		C.free(unsafe.Pointer(result.path))
	}

	if !access.accessing {
		// 非沙箱環境（例如開發模式）解析會成功但不需要、也無法啟動 scoped 存取，
		// 此時只要路徑可讀就視為可用；真的讀不到會在後續開檔時報錯。
		if access.Path == "" {
			access.Release()
			return nil, errors.New("解析 bookmark 失敗: 無法取得路徑")
		}
	}

	return access, nil
}
