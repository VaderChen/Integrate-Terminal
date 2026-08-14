//go:build !darwin

package macsecurity

import "errors"

// ErrUnsupported 在非 macOS 平台回傳。
var ErrUnsupported = errors.New("security-scoped bookmark 僅支援 macOS")

// Available 回報目前平台是否支援 security-scoped bookmark。
func Available() bool { return false }

// Access 代表一次已啟動的 security-scoped 存取。非 macOS 平台不會產生。
type Access struct {
	Path  string
	Stale bool
}

// Release 在非 macOS 平台為 no-op。
func (a *Access) Release() {}

// CreateBookmark 在非 macOS 平台一律失敗。
func CreateBookmark(string) (string, error) { return "", ErrUnsupported }

// ResolveBookmark 在非 macOS 平台一律失敗。
func ResolveBookmark(string) (*Access, error) { return nil, ErrUnsupported }
