//go:build !darwin || !cgo

package credentials

import (
	"errors"
	"testing"
)

func TestUnsupportedBackendNeverFallsBackToPlaintext(t *testing.T) {
	for _, backend := range []Backend{New(), NewNonInteractive()} {
		secret, err := backend.Get("v1:test/ref")
		if secret != nil || !errors.Is(err, ErrUnsupported) {
			t.Fatalf("unsupported read: %v", err)
		}
		if !errors.Is(backend.Put("v1:test/ref", []byte("fake secret")), ErrUnsupported) {
			t.Fatal("unsupported write succeeded")
		}
		if !errors.Is(backend.Delete("v1:test/ref"), ErrUnsupported) {
			t.Fatal("unsupported delete succeeded")
		}
	}
}
