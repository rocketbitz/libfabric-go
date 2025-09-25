//go:build cgo

package capi

import (
	"unsafe"
)

/*
#cgo pkg-config: libfabric
#include <rdma/fabric.h>
#include <rdma/fi_domain.h>
*/
import "C"

// Fabric represents an opened libfabric fid_fabric handle.
type Fabric struct {
	ptr *C.struct_fid_fabric
}

// Domain represents an opened fid_domain handle.
type Domain struct {
	ptr *C.struct_fid_domain
}

// OpenFabric opens a fabric handle using the fabric attributes from the
// provided descriptor entry.
func OpenFabric(entry InfoEntry) (*Fabric, error) {
	if entry.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_fabric")
	}
	if entry.ptr.fabric_attr == nil {
		return nil, ErrUnavailable.WithOp("fi_fabric")
	}

	var fabric *C.struct_fid_fabric
	status := C.fi_fabric(entry.ptr.fabric_attr, &fabric, nil)
	if err := ErrorFromStatus(int(status), "fi_fabric"); err != nil {
		return nil, err
	}
	return &Fabric{ptr: fabric}, nil
}

// Close releases the fabric handle.
func (f *Fabric) Close() error {
	if f == nil || f.ptr == nil {
		return nil
	}
	status := C.fi_close((*C.struct_fid)(unsafe.Pointer(f.ptr)))
	if err := ErrorFromStatus(int(status), "fi_close(fabric)"); err != nil {
		return err
	}
	f.ptr = nil
	return nil
}

// OpenDomain opens a domain associated with the fabric using the provided
// descriptor entry.
func OpenDomain(fabric *Fabric, entry InfoEntry) (*Domain, error) {
	if fabric == nil || fabric.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_domain")
	}
	if entry.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_domain")
	}

	var dom *C.struct_fid_domain
	status := C.fi_domain(fabric.ptr, entry.ptr, &dom, nil)
	if err := ErrorFromStatus(int(status), "fi_domain"); err != nil {
		return nil, err
	}
	return &Domain{ptr: dom}, nil
}

// Close releases the domain handle.
func (d *Domain) Close() error {
	if d == nil || d.ptr == nil {
		return nil
	}
	status := C.fi_close((*C.struct_fid)(unsafe.Pointer(d.ptr)))
	if err := ErrorFromStatus(int(status), "fi_close(domain)"); err != nil {
		return err
	}
	d.ptr = nil
	return nil
}
