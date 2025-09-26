package fi

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

func TestRegisterMemorySockets(t *testing.T) {
	_, _, domain := setupSocketsResources(t)

	data := []byte("hello registered memory")

	mr, err := domain.RegisterMemory(data, MRAccessLocal|MRAccessRemoteRead)
	if err != nil {
		t.Skipf("memory registration unsupported: %v", err)
	}
	t.Cleanup(func() { _ = mr.Close() })

	if mr.Key() == 0 {
		t.Skip("provider returned zero memory region key")
	}

	buf := mr.Bytes()
	if string(buf) != string(data) {
		t.Fatalf("unexpected contents: got %q want %q", string(buf), string(data))
	}

	buf[0] = 'H'
	if mr.Bytes()[0] != 'H' {
		t.Fatalf("mutation not reflected in view")
	}

	if ptr := mr.Descriptor(); ptr == nil {
		t.Log("memory region descriptor not provided by provider")
	} else {
		if len(mr.DescriptorBytes()) == 0 {
			t.Fatalf("descriptor pointer returned but descriptor bytes empty")
		}
	}
}

func TestEnsureRegionAccessValidation(t *testing.T) {
	if err := ensureRegionAccess(nil, MRAccessLocal); err != nil {
		t.Fatalf("expected nil error for nil region, got %v", err)
	}

	buf := capi.AllocBytes(8)
	if buf == nil {
		t.Fatalf("AllocBytes returned nil buffer")
	}
	t.Cleanup(func() { capi.FreeBytes(buf) })

	regionNoHandle := &MemoryRegion{buffer: buf, length: 8}
	if err := ensureRegionAccess(regionNoHandle, MRAccessLocal); err == nil {
		t.Fatalf("expected error for nil memory region handle")
	} else {
		var invalid ErrInvalidHandle
		if !errors.As(err, &invalid) || invalid.Resource != "memory region" {
			t.Fatalf("expected ErrInvalidHandle for memory region, got %v", err)
		}
	}

	regionMissingAccess := &MemoryRegion{handle: &capi.MemoryRegion{}, buffer: buf, length: 8}
	if err := ensureRegionAccess(regionMissingAccess, MRAccessLocal); !errors.Is(err, ErrInsufficientAccess) {
		t.Fatalf("expected ErrInsufficientAccess, got %v", err)
	}

	regionOK := &MemoryRegion{handle: &capi.MemoryRegion{}, buffer: buf, length: 8, access: MRAccessLocal | MRAccessRemoteRead}
	if err := ensureRegionAccess(regionOK, MRAccessLocal); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ensureRegionAccess(regionOK, MRAccessRemoteRead); err != nil {
		t.Fatalf("expected remote read access to be permitted, got %v", err)
	}
}

func TestRegisterMemoryWithOptions(t *testing.T) {
	_, _, domain := setupSocketsResources(t)

	buf := []byte("options")
	mr, err := domain.RegisterMemoryWithOptions(buf, &MRRegisterOptions{RequestedKey: 42})
	if err != nil {
		t.Skipf("register with options unsupported: %v", err)
	}
	defer mr.Close()

	if mr.Key() == 0 {
		t.Skip("provider returned zero key; cannot verify requested key")
	}
}

func TestRegisterMemoryPointer(t *testing.T) {
	_, _, domain := setupSocketsResources(t)

	ptr := capi.AllocBytes(32)
	if ptr == nil {
		t.Fatalf("AllocBytes returned nil")
	}
	defer capi.FreeBytes(ptr)

	mr, err := domain.RegisterMemoryPointer(ptr, 32, &MRRegisterOptions{Access: MRAccessRemoteRead})
	if err != nil {
		t.Skipf("pointer registration unsupported: %v", err)
	}

	if mr.Descriptor() == nil {
		t.Log("provider did not supply descriptor pointer")
	}

	if err := mr.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Ensure the underlying pointer remains valid by writing to it.
	mem := unsafe.Slice((*byte)(ptr), 32)
	mem[0] = 'x'
}

func TestRegisterMemorySegments(t *testing.T) {
	_, _, domain := setupSocketsResources(t)

	seg1 := capi.AllocBytes(16)
	seg2 := capi.AllocBytes(24)
	if seg1 == nil || seg2 == nil {
		t.Fatalf("AllocBytes returned nil segment")
	}
	defer capi.FreeBytes(seg1)
	defer capi.FreeBytes(seg2)

	mr, err := domain.RegisterMemorySegments([]MRSegment{{Pointer: seg1, Length: 16}, {Pointer: seg2, Length: 24}}, &MRRegisterOptions{Access: MRAccessLocal})
	if err != nil {
		t.Skipf("segment registration unsupported: %v", err)
	}
	defer func() {
		_ = mr.Close()
	}()

	if mr.Size() != 40 {
		t.Fatalf("unexpected summed length: got %d", mr.Size())
	}
}
