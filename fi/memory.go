package fi

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

// MRAccessFlag represents allowed operations on a registered memory region.
type MRAccessFlag uint64

const (
	// MRAccessLocal allows local CPU access to the registered memory.
	MRAccessLocal MRAccessFlag = MRAccessFlag(capi.MRAccessLocal)
	// MRAccessRemoteRead allows remote peers to issue read operations.
	MRAccessRemoteRead MRAccessFlag = MRAccessFlag(capi.MRAccessRemoteRead)
	// MRAccessRemoteWrite allows remote peers to issue write operations.
	MRAccessRemoteWrite MRAccessFlag = MRAccessFlag(capi.MRAccessRemoteWrite)
)

const supportedDomainMRModes = capi.MRModeLocal | capi.MRModeVirtAddr | capi.MRModeProvKey | capi.MRModeMMUNotify | capi.MRModeRMAEvent

// MemoryRegion wraps a registered memory buffer usable with libfabric operations.
type MemoryRegion struct {
	handle   *capi.MemoryRegion
	buffer   unsafe.Pointer
	length   uintptr
	access   MRAccessFlag
	descSize uintptr
	ownsBuf  bool
}

// Bytes returns a Go slice view of the registered buffer.
func (m *MemoryRegion) Bytes() []byte {
	if m == nil || m.buffer == nil || m.length == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(m.buffer), m.length)
}

// DescriptorBytes copies the provider-specific descriptor into a Go slice.
func (m *MemoryRegion) DescriptorBytes() []byte {
	if m == nil || m.handle == nil {
		return nil
	}
	desc := m.handle.Descriptor()
	if desc == nil {
		return nil
	}
	size := m.handle.DescriptorSize()
	if size == 0 {
		size = m.descSize
		if size == 0 {
			return nil
		}
	}
	buf := make([]byte, size)
	ptr := unsafe.Slice((*byte)(desc), size)
	copy(buf, ptr)
	return buf
}

// Key returns the registration key for the memory region.
func (m *MemoryRegion) Key() uint64 {
	if m == nil || m.handle == nil {
		return 0
	}
	return m.handle.Key()
}

// Access reports the access flags the region was registered with.
func (m *MemoryRegion) Access() MRAccessFlag {
	if m == nil {
		return 0
	}
	return m.access
}

// Descriptor returns the provider-specific descriptor pointer and length.
func (m *MemoryRegion) Descriptor() unsafe.Pointer {
	if m == nil || m.handle == nil {
		return nil
	}
	return m.handle.Descriptor()
}

// DescriptorSize reports the provider descriptor size when known.
func (m *MemoryRegion) DescriptorSize() uintptr {
	if m == nil {
		return 0
	}
	if size := m.handle.DescriptorSize(); size != 0 {
		return size
	}
	return m.descSize
}

// Size returns the registered length in bytes.
func (m *MemoryRegion) Size() uintptr {
	if m == nil {
		return 0
	}
	return m.length
}

// Close deregisters the memory region and frees the underlying buffer.
func (m *MemoryRegion) Close() error {
	if m == nil {
		return nil
	}
	if m.handle != nil {
		if err := m.handle.Close(); err != nil {
			return err
		}
	}
	if m.buffer != nil && m.ownsBuf {
		capi.FreeBytes(m.buffer)
	}
	m.handle = nil
	m.buffer = nil
	m.length = 0
	m.access = 0
	m.descSize = 0
	m.ownsBuf = false
	return nil
}

// MRRegisterOptions provides advanced controls for memory registration.
type MRRegisterOptions struct {
	Access       MRAccessFlag
	RequestedKey uint64
	Offset       uint64
	Flags        uint64
}

// MRSegment represents a single scatter/gather segment for registration.
type MRSegment struct {
	Pointer unsafe.Pointer
	Length  uintptr
}

// RegisterMemory allocates provider-accessible memory, optionally seeded with the contents of buf, and registers it with the domain.
func (d *Domain) RegisterMemory(buf []byte, access MRAccessFlag) (*MemoryRegion, error) {
	return d.RegisterMemoryWithOptions(buf, &MRRegisterOptions{Access: access})
}

// RegisterMemoryWithOptions registers the provided buffer with optional advanced flags.
// The buffer is copied into C-managed memory to satisfy CGO pointer safety rules.
func (d *Domain) RegisterMemoryWithOptions(buf []byte, opts *MRRegisterOptions) (*MemoryRegion, error) {
	if d == nil || d.handle == nil {
		return nil, ErrInvalidHandle{"domain"}
	}
	if len(buf) == 0 {
		return nil, errors.New("libfabric: memory registration requires non-empty buffer")
	}

	if unsupported := d.mrMode &^ supportedDomainMRModes; unsupported != 0 {
		return nil, fmt.Errorf("libfabric: %w (domain requires unsupported mr_mode bits 0x%x)", ErrCapabilityUnsupported, unsupported)
	}

	var access MRAccessFlag
	if opts != nil {
		access = opts.Access
	}
	if access == 0 {
		access = MRAccessLocal
	}
	if d.RequiresMRMode(MRModeLocal) {
		access |= MRAccessLocal
	}

	cbuf := capi.AllocBytes(uintptr(len(buf)))
	if cbuf == nil {
		return nil, errors.New("libfabric: unable to allocate registration buffer")
	}
	capi.Memcpy(cbuf, unsafe.Pointer(&buf[0]), uintptr(len(buf)))

	var requestedKey, offset, flags uint64
	if opts != nil {
		requestedKey = opts.RequestedKey
		offset = opts.Offset
		flags = opts.Flags
	}

	mr, err := d.handle.RegisterMemory(cbuf, uintptr(len(buf)), capi.MRAccess(access), offset, requestedKey, flags)
	if err != nil {
		capi.FreeBytes(cbuf)
		return nil, err
	}

	return &MemoryRegion{handle: mr, buffer: cbuf, length: uintptr(len(buf)), access: access, descSize: d.mrKeySize, ownsBuf: true}, nil
}

// RegisterMemoryPointer registers memory referenced by ptr without copying. The caller
// must ensure ptr refers to C-allocated memory that remains valid while the
// registration is active.
func (d *Domain) RegisterMemoryPointer(ptr unsafe.Pointer, length uintptr, opts *MRRegisterOptions) (*MemoryRegion, error) {
	if d == nil || d.handle == nil {
		return nil, ErrInvalidHandle{"domain"}
	}
	if ptr == nil || length == 0 {
		return nil, errors.New("libfabric: pointer registration requires non-nil pointer and positive length")
	}

	if unsupported := d.mrMode &^ supportedDomainMRModes; unsupported != 0 {
		return nil, fmt.Errorf("libfabric: %w (domain requires unsupported mr_mode bits 0x%x)", ErrCapabilityUnsupported, unsupported)
	}

	var access MRAccessFlag
	if opts != nil {
		access = opts.Access
	}
	if access == 0 {
		access = MRAccessLocal
	}
	if d.RequiresMRMode(MRModeLocal) {
		access |= MRAccessLocal
	}

	var requestedKey, offset, flags uint64
	if opts != nil {
		requestedKey = opts.RequestedKey
		offset = opts.Offset
		flags = opts.Flags
	}

	mr, err := d.handle.RegisterMemory(ptr, length, capi.MRAccess(access), offset, requestedKey, flags)
	if err != nil {
		return nil, err
	}

	return &MemoryRegion{handle: mr, buffer: ptr, length: length, access: access, descSize: d.mrKeySize, ownsBuf: false}, nil
}

// RegisterMemorySegments registers a scatter/gather list of memory segments.
// The caller retains ownership of the underlying memory and must ensure it
// remains valid for the lifetime of the registration.
func (d *Domain) RegisterMemorySegments(segments []MRSegment, opts *MRRegisterOptions) (*MemoryRegion, error) {
	if d == nil || d.handle == nil {
		return nil, ErrInvalidHandle{"domain"}
	}
	if len(segments) == 0 {
		return nil, errors.New("libfabric: memory registration requires at least one segment")
	}
	if unsupported := d.mrMode &^ supportedDomainMRModes; unsupported != 0 {
		return nil, fmt.Errorf("libfabric: %w (domain requires unsupported mr_mode bits 0x%x)", ErrCapabilityUnsupported, unsupported)
	}
	if limit := d.MRIovLimit(); limit > 0 && uintptr(len(segments)) > limit {
		return nil, fmt.Errorf("libfabric: provider allows at most %d segments", limit)
	}

	var access MRAccessFlag
	if opts != nil {
		access = opts.Access
	}
	if access == 0 {
		access = MRAccessLocal
	}
	if d.RequiresMRMode(MRModeLocal) {
		access |= MRAccessLocal
	}

	total := uintptr(0)
	iov := make([]capi.MRIOVec, len(segments))
	for idx, seg := range segments {
		if seg.Pointer == nil || seg.Length == 0 {
			return nil, errors.New("libfabric: segment requires non-nil pointer and length")
		}
		iov[idx] = capi.MRIOVec{Base: seg.Pointer, Length: seg.Length}
		total += seg.Length
	}

	var requestedKey, offset, flags uint64
	if opts != nil {
		requestedKey = opts.RequestedKey
		offset = opts.Offset
		flags = opts.Flags
	}

	mr, err := d.handle.RegisterMemoryIOV(iov, capi.MRAccess(access), offset, requestedKey, flags)
	if err != nil {
		return nil, err
	}

	return &MemoryRegion{handle: mr, buffer: nil, length: total, access: access, descSize: d.mrKeySize, ownsBuf: false}, nil
}

func (m *MemoryRegion) hasAccess(flag MRAccessFlag) bool {
	if m == nil {
		return false
	}
	return m.access&flag == flag
}

func ensureRegionAccess(region *MemoryRegion, required MRAccessFlag) error {
	if region == nil {
		return nil
	}
	if region.handle == nil || region.buffer == nil {
		return ErrInvalidHandle{"memory region"}
	}
	if required != 0 && !region.hasAccess(required) {
		return ErrInsufficientAccess
	}
	return nil
}
