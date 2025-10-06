//go:build cgo

package capi

import (
	"errors"
	"unsafe"
)

/*
#cgo pkg-config: libfabric
#include <stdlib.h>
#include <rdma/fabric.h>
#include <rdma/fi_domain.h>
#include <rdma/fi_rma.h>
#include <rdma/fi_atomic.h>
*/
import "C"

// MRAccess represents libfabric memory registration access flags.
type MRAccess uint64

const (
	// MRAccessLocal enables local CPU access to the registered memory.
	MRAccessLocal MRAccess = MRAccess(C.FI_MR_LOCAL)
	// MRAccessRemoteRead enables remote read access.
	MRAccessRemoteRead MRAccess = MRAccess(C.FI_REMOTE_READ)
	// MRAccessRemoteWrite enables remote write access.
	MRAccessRemoteWrite MRAccess = MRAccess(C.FI_REMOTE_WRITE)
)

const (
	// MRModeLocal indicates local registration semantics are required.
	MRModeLocal = uint64(C.FI_MR_LOCAL)
	// MRModeRaw indicates raw memory registration semantics.
	MRModeRaw = uint64(C.FI_MR_RAW)
	// MRModeVirtAddr indicates virtual address registration semantics.
	MRModeVirtAddr = uint64(C.FI_MR_VIRT_ADDR)
	// MRModeAllocated indicates provider-allocated registrations.
	MRModeAllocated = uint64(C.FI_MR_ALLOCATED)
	// MRModeProvKey indicates provider-generated keys are required.
	MRModeProvKey = uint64(C.FI_MR_PROV_KEY)
	// MRModeMMUNotify indicates memory mapping changes must be reported.
	MRModeMMUNotify = uint64(C.FI_MR_MMU_NOTIFY)
	// MRModeRMAEvent indicates RMA operations must produce events.
	MRModeRMAEvent = uint64(C.FI_MR_RMA_EVENT)
	// MRModeEndpoint indicates registrations are endpoint-specific.
	MRModeEndpoint = uint64(C.FI_MR_ENDPOINT)
	// MRModeHMEM indicates heterogeneous memory registration semantics.
	MRModeHMEM = uint64(C.FI_MR_HMEM)
	// MRModeCollective indicates collective registration semantics.
	MRModeCollective = uint64(C.FI_MR_COLLECTIVE)
)

// MemoryRegion wraps a libfabric fid_mr handle.
type MemoryRegion struct {
	ptr *C.struct_fid_mr
}

// MRIOVec describes a memory segment for vector registration.
type MRIOVec struct {
	Base   unsafe.Pointer
	Length uintptr
}

// RegisterMemory registers the supplied buffer with the given access flags.
func (d *Domain) RegisterMemory(buf unsafe.Pointer, length uintptr, access MRAccess, offset uint64, requestedKey uint64, flags uint64) (*MemoryRegion, error) {
	if d == nil || d.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_mr_reg")
	}
	if length == 0 {
		return nil, ErrUnavailable.WithOp("fi_mr_reg")
	}

	var mr *C.struct_fid_mr
	status := C.fi_mr_reg(d.ptr, buf, C.size_t(length), C.uint64_t(access), C.uint64_t(offset), C.uint64_t(requestedKey), C.uint64_t(flags), &mr, nil)
	if err := ErrorFromStatus(int(status), "fi_mr_reg"); err != nil {
		return nil, err
	}
	return &MemoryRegion{ptr: mr}, nil
}

// RegisterMemoryIOV registers a scatter/gather list of buffers with the domain.
func (d *Domain) RegisterMemoryIOV(iov []MRIOVec, access MRAccess, offset uint64, requestedKey uint64, flags uint64) (*MemoryRegion, error) {
	if d == nil || d.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_mr_regv")
	}
	if len(iov) == 0 {
		return nil, errors.New("fi_mr_regv: empty iov")
	}

	count := len(iov)
	ciov := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(C.struct_iovec{})))
	if ciov == nil {
		return nil, ErrNoMemory.WithOp("fi_mr_regv")
	}
	defer C.free(ciov)
	slice := (*[1 << 30]C.struct_iovec)(ciov)[:count:count]
	for idx, seg := range iov {
		if seg.Base == nil || seg.Length == 0 {
			return nil, errors.New("fi_mr_regv: segment requires base pointer and length")
		}
		slice[idx].iov_base = seg.Base
		slice[idx].iov_len = C.size_t(seg.Length)
	}

	var mr *C.struct_fid_mr
	status := C.fi_mr_regv(d.ptr, (*C.struct_iovec)(ciov), C.size_t(count), C.uint64_t(access), C.uint64_t(offset), C.uint64_t(requestedKey), C.uint64_t(flags), &mr, unsafe.Pointer(nil))
	if err := ErrorFromStatus(int(status), "fi_mr_regv"); err != nil {
		return nil, err
	}
	return &MemoryRegion{ptr: mr}, nil
}

// Close releases the memory region.
func (m *MemoryRegion) Close() error {
	if m == nil || m.ptr == nil {
		return nil
	}
	status := C.fi_close((*C.struct_fid)(unsafe.Pointer(m.ptr)))
	if err := ErrorFromStatus(int(status), "fi_close(mr)"); err != nil {
		return err
	}
	m.ptr = nil
	return nil
}

// Key returns the registration key for the memory region.
func (m *MemoryRegion) Key() uint64 {
	if m == nil || m.ptr == nil {
		return 0
	}
	return uint64(C.fi_mr_key(m.ptr))
}

// Descriptor returns the provider-specific descriptor pointer for the region.
func (m *MemoryRegion) Descriptor() unsafe.Pointer {
	if m == nil || m.ptr == nil {
		return nil
	}
	return C.fi_mr_desc(m.ptr)
}

// DescriptorSize reports the size of the provider-specific descriptor.
func (m *MemoryRegion) DescriptorSize() uintptr {
	if m == nil || m.ptr == nil {
		return 0
	}
	return 0
}
