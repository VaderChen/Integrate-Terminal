//go:build darwin && cgo

package credentials

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type fakeNativeDriver struct {
	status int32
	secret []byte
	calls  int
}

func (d *fakeNativeDriver) get(string, bool) ([]byte, int32) {
	d.calls++
	return append([]byte(nil), d.secret...), d.status
}
func (d *fakeNativeDriver) put(string, []byte, bool) int32    { d.calls++; return d.status }
func (d *fakeNativeDriver) delete(string, bool) int32         { d.calls++; return d.status }
func (d *fakeNativeDriver) interactionAllowed() (bool, int32) { return true, 0 }
func (d *fakeNativeDriver) setInteractionAllowed(bool) int32  { return 0 }

func TestBackendPreservesNativeFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int32
		cause  error
	}{
		{"missing", -25300, ErrNotFound},
		{"locked", -25308, ErrLocked},
		{"interaction required", -25315, ErrLocked},
		{"denied", -25293, ErrDenied},
		{"cancelled permission", -128, ErrDenied},
		{"entitlement denied", -34018, ErrDenied},
		{"unavailable", -25291, ErrUnavailable},
		{"too large", -25302, ErrTooLarge},
		{"unknown", -987654, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver := &fakeNativeDriver{status: test.status, secret: []byte("must not escape on failure")}
			backend := &keychainBackend{driver: driver}
			secret, getErr := backend.Get("v1:test/unique-reference")
			if secret != nil {
				t.Fatal("failed read returned credential data")
			}
			for _, err := range []error{getErr, backend.Put("v1:test/unique-reference", []byte("private value")), backend.Delete("v1:test/unique-reference")} {
				if err == nil {
					t.Fatal("native failure became success")
				}
				if test.cause != nil && !errors.Is(err, test.cause) {
					t.Fatalf("wrong classification: %v", err)
				}
				var nativeErr *Error
				if !errors.As(err, &nativeErr) || nativeErr.Status != test.status {
					t.Fatalf("status lost: %v", err)
				}
				if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "unique-reference") {
					t.Fatal("error discloses credential contents/reference")
				}
			}
		})
	}
}

func TestBackendRejectsInvalidInputBeforeNativeCall(t *testing.T) {
	driver := &fakeNativeDriver{}
	backend := &keychainBackend{driver: driver}
	if _, err := backend.Get(""); !errors.Is(err, ErrInvalidReference) {
		t.Fatal(err)
	}
	if err := backend.Put("bad\x00account", nil); !errors.Is(err, ErrInvalidReference) {
		t.Fatal(err)
	}
	if err := backend.Delete(""); !errors.Is(err, ErrInvalidReference) {
		t.Fatal(err)
	}
	if err := backend.Put("v1:test/ref", make([]byte, maxSecretSize+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	if driver.calls != 0 {
		t.Fatal("invalid input reached Keychain")
	}
}

// Explicitly opt in: this test creates only one unique fake credential under
// the app's service, then deletes that exact account. It never enumerates items,
// changes a keychain setting, or reads an existing user's credential.
func TestKeychainIntegration(t *testing.T) {
	if os.Getenv("INTEGTERM_KEYCHAIN_INTEGRATION") != "1" {
		t.Skip("set INTEGTERM_KEYCHAIN_INTEGRATION=1 to test one UUID-scoped fake Keychain item")
	}
	backend := New()
	ref := "test/" + uuid.NewString()
	if _, err := backend.Get(ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("fresh UUID lookup: %v", err)
	}
	secret := []byte{'f', 'a', 'k', 'e', 0, 255, 1}
	if err := backend.Put(ref, secret); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Delete(ref); err != nil && !errors.Is(err, ErrNotFound) {
			t.Errorf("remove fake Keychain item: %v", err)
		}
	})
	if actual, err := New().Get(ref); err != nil || !bytes.Equal(actual, secret) {
		t.Fatalf("new instance readback mismatch (error: %v)", err)
	}
	replacement := []byte("different fake credential")
	if err := backend.Put(ref, replacement); err != nil {
		t.Fatal(err)
	}
	if actual, err := backend.Get(ref); err != nil || !bytes.Equal(actual, replacement) {
		t.Fatalf("updated readback mismatch (error: %v)", err)
	}
	if err := backend.Delete(ref); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Get(ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted item should be missing: %v", err)
	}
}
