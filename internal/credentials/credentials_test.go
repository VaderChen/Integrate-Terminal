package credentials

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateReference(t *testing.T) {
	for _, ref := range []string{"", " \n", "account\x00extra", string([]byte{0xff}), strings.Repeat("x", 1025)} {
		if !errors.Is(validateReference(ref), ErrInvalidReference) {
			t.Fatalf("invalid reference accepted: %q", ref)
		}
	}
	for _, ref := range []string{"v1:store-namespace/random-reference", "utf8/測試-reference"} {
		if err := validateReference(ref); err != nil {
			t.Fatalf("valid reference rejected: %v", err)
		}
	}
}

func TestNativeErrorRetainsStatusAndClassification(t *testing.T) {
	err := &Error{Operation: "get", Status: -25308, Cause: ErrLocked}
	if !errors.Is(err, ErrLocked) || errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong error classification: %v", err)
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Status != -25308 {
		t.Fatalf("native status was lost: %v", err)
	}
}
