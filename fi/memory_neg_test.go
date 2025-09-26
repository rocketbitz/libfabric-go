package fi

import "testing"

func TestRegisterMemoryInvalidHandle(t *testing.T) {
	domain := &Domain{}
	if _, err := domain.RegisterMemory([]byte("data"), MRAccessLocal); err == nil {
		t.Fatalf("expected error for invalid domain handle")
	}
}

func TestRegisterMemoryEmptyBuffer(t *testing.T) {
	_, _, domain := setupSocketsResources(t)
	if _, err := domain.RegisterMemory(nil, MRAccessLocal); err == nil {
		t.Fatalf("expected error for empty buffer")
	}
}
