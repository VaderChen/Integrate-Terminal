// Package credentials stores opaque credential bytes in the operating system's
// credential vault. References, rather than credential bytes, belong in JSON.
package credentials

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const Service = "com.vader.integterm.credentials.v1"

const maxSecretSize = 1024 * 1024

type Backend interface {
	Get(ref string) ([]byte, error)
	Put(ref string, secret []byte) error
	Delete(ref string) error
}

var (
	ErrNotFound = errors.New("credential not found")
	// macOS reports the same status for a locked keychain and an operation that
	// needs user interaction unavailable in the current execution context.
	ErrLocked           = errors.New("keychain is locked or requires user interaction")
	ErrDenied           = errors.New("keychain access denied")
	ErrUnavailable      = errors.New("keychain unavailable")
	ErrUnsupported      = errors.New("native credential storage is unsupported on this platform or without cgo")
	ErrInvalidReference = errors.New("invalid credential reference")
	ErrTooLarge         = errors.New("credential data exceeds the supported size")
)

// Error preserves the native status without including the account reference or
// secret in logs. errors.Is distinguishes missing, locked and denied results.
type Error struct {
	Operation string
	Status    int32
	Cause     error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("keychain %s: %v (OSStatus %d)", e.Operation, e.Cause, e.Status)
	}
	return fmt.Sprintf("keychain %s failed (OSStatus %d)", e.Operation, e.Status)
}

func (e *Error) Unwrap() error { return e.Cause }

func validateReference(ref string) error {
	if strings.TrimSpace(ref) == "" || len(ref) > 1024 || !utf8.ValidString(ref) || strings.ContainsRune(ref, '\x00') {
		return ErrInvalidReference
	}
	return nil
}
