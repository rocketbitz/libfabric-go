//go:build cgo

package capi

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorFromStatus(t *testing.T) {
	if err := ErrorFromStatus(0, "noop"); err != nil {
		t.Fatalf("expected nil error for success status, got %v", err)
	}

	err := ErrorFromStatus(-int(ErrAgain), "fi_send")
	if err == nil {
		t.Fatalf("expected error for ErrAgain status")
	}
	if !errors.Is(err, ErrAgain) {
		t.Fatalf("expected errors.Is match ErrAgain, got %v", err)
	}
	if !strings.Contains(err.Error(), "fi_send") {
		t.Fatalf("expected operation context in error string, got %q", err)
	}

	err = ErrorFromStatus(-int(ErrUnavailable), "fi_cq_read")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestErrnoString(t *testing.T) {
	msg := ErrAgain.String()
	if msg == "" || strings.EqualFold(msg, "unknown") {
		t.Fatalf("unexpected strerror message: %q", msg)
	}
}
