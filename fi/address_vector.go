package fi

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

// AVType mirrors capi.AVType for public use.
type AVType = capi.AVType

const (
	// AVTypeUnspec requests the provider's default address vector implementation.
	AVTypeUnspec = capi.AVTypeUnspec
	// AVTypeMap selects a map-based address vector implementation.
	AVTypeMap = capi.AVTypeMap
	// AVTypeTable selects a table-based address vector implementation.
	AVTypeTable = capi.AVTypeTable
)

// Address represents an fi_addr_t assigned by the provider.
type Address = capi.FIAddr

const (
	// AddressUnspecified represents an invalid or unspecified remote address.
	AddressUnspecified = Address(capi.FIAddrUnspec)
)

// AddressVectorAttr mirrors libfabric fi_av_attr for configuration.
type AddressVectorAttr struct {
	Type      AVType
	RXCtxBits int
	Count     uint64
	EPPerNode uint64
	Name      string
	Flags     uint64
}

// AddressVector provides access to an underlying libfabric AV handle.
type AddressVector struct {
	handle *capi.AV
}

// Close releases the AV handle.
func (a *AddressVector) Close() error {
	if a == nil || a.handle == nil {
		return nil
	}
	err := a.handle.Close()
	a.handle = nil
	return err
}

// OpenAddressVector opens an address vector on the domain.
func (d *Domain) OpenAddressVector(attr *AddressVectorAttr) (*AddressVector, error) {
	if d == nil || d.handle == nil {
		return nil, ErrInvalidHandle{"domain"}
	}

	var ca *capi.AVAttr
	var tmp capi.AVAttr
	if attr != nil {
		tmp = capi.AVAttr{
			Type:      capi.AVType(attr.Type),
			RXCtxBits: attr.RXCtxBits,
			Count:     attr.Count,
			EPPerNode: attr.EPPerNode,
			Name:      attr.Name,
			Flags:     attr.Flags,
		}
		ca = &tmp
	}

	handle, err := capi.OpenAV(d.handle, ca)
	if err != nil {
		return nil, err
	}
	return &AddressVector{handle: handle}, nil
}

// InsertService resolves and inserts a node/service pair into the AV.
func (a *AddressVector) InsertService(node, service string, flags uint64) (Address, error) {
	if a == nil || a.handle == nil {
		return 0, ErrInvalidHandle{"address vector"}
	}
	return a.handle.InsertService(node, service, flags)
}

// Remove removes the provided addresses from the AV.
func (a *AddressVector) Remove(addrs []Address, flags uint64) error {
	if a == nil || a.handle == nil {
		return ErrInvalidHandle{"address vector"}
	}
	return a.handle.Remove(addrs, flags)
}

// InsertRaw inserts a provider-specific address byte sequence.
func (a *AddressVector) InsertRaw(addr []byte, flags uint64) (Address, error) {
	if a == nil || a.handle == nil {
		return 0, ErrInvalidHandle{"address vector"}
	}
	if len(addr) == 0 {
		return 0, errors.New("libfabric: empty address payload")
	}
	buf := capi.AllocBytes(uintptr(len(addr)))
	if buf == nil {
		return 0, fmt.Errorf("libfabric: unable to allocate address buffer")
	}
	capi.Memcpy(buf, unsafe.Pointer(&addr[0]), uintptr(len(addr)))
	fiAddr, err := a.handle.InsertRaw(buf, flags)
	capi.FreeBytes(buf)
	if err != nil {
		return 0, err
	}
	return Address(fiAddr), nil
}
