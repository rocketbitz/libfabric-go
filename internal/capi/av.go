//go:build cgo

package capi

import (
	"unsafe"
)

/*
#cgo pkg-config: libfabric
#include <stdlib.h>
#include <rdma/fi_domain.h>
*/
import "C"

// AVType mirrors enum fi_av_type.
type AVType int

const (
	AVTypeUnspec AVType = AVType(C.FI_AV_UNSPEC)
	AVTypeMap    AVType = AVType(C.FI_AV_MAP)
	AVTypeTable  AVType = AVType(C.FI_AV_TABLE)
)

// AVAttr describes address vector configuration.
type AVAttr struct {
	Type      AVType
	RXCtxBits int
	Count     uint64
	EPPerNode uint64
	Name      string
	MapAddr   unsafe.Pointer
	Flags     uint64
}

// AV wraps a libfabric fid_av handle.
type AV struct {
	ptr *C.struct_fid_av
}

// FIAddr represents an fi_addr_t value returned from libfabric.
type FIAddr uint64

const (
	FIAddrUnspec FIAddr = ^FIAddr(0)
)

// OpenAV opens an address vector for the given domain.
func OpenAV(domain *Domain, attr *AVAttr) (*AV, error) {
	if domain == nil || domain.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_av_open")
	}

	var ca *C.struct_fi_av_attr
	var tmp C.struct_fi_av_attr
	if attr != nil {
		tmp._type = C.enum_fi_av_type(attr.Type)
		tmp.rx_ctx_bits = C.int(attr.RXCtxBits)
		tmp.count = C.size_t(attr.Count)
		tmp.ep_per_node = C.size_t(attr.EPPerNode)
		tmp.map_addr = attr.MapAddr
		tmp.flags = C.uint64_t(attr.Flags)
		if attr.Name != "" {
			tmp.name = C.CString(attr.Name)
			defer C.free(unsafe.Pointer(tmp.name))
		}
		ca = &tmp
	}

	var av *C.struct_fid_av
	status := C.fi_av_open(domain.ptr, ca, &av, nil)
	if err := ErrorFromStatus(int(status), "fi_av_open"); err != nil {
		return nil, err
	}
	return &AV{ptr: av}, nil
}

// Close releases the address vector.
func (a *AV) Close() error {
	if a == nil || a.ptr == nil {
		return nil
	}
	status := C.fi_close((*C.struct_fid)(unsafe.Pointer(a.ptr)))
	if err := ErrorFromStatus(int(status), "fi_close(av)"); err != nil {
		return err
	}
	a.ptr = nil
	return nil
}

// InsertService resolves a node/service pair and inserts it into the AV.
func (a *AV) InsertService(node, service string, flags uint64) (FIAddr, error) {
	if a == nil || a.ptr == nil {
		return 0, ErrUnavailable.WithOp("fi_av_insertsvc")
	}

	var cNode, cService *C.char
	if node != "" {
		cNode = C.CString(node)
		defer C.free(unsafe.Pointer(cNode))
	}
	if service != "" {
		cService = C.CString(service)
		defer C.free(unsafe.Pointer(cService))
	}

	var out C.fi_addr_t
	status := C.fi_av_insertsvc(a.ptr, cNode, cService, &out, C.uint64_t(flags), nil)
	if err := ErrorFromStatus(int(status), "fi_av_insertsvc"); err != nil {
		return 0, err
	}
	if status == 0 {
		return 0, ErrUnavailable.WithOp("fi_av_insertsvc")
	}
	return FIAddr(out), nil
}

// InsertRaw inserts a provider-specific address into the AV.
func (a *AV) InsertRaw(addr unsafe.Pointer, flags uint64) (FIAddr, error) {
	if a == nil || a.ptr == nil || addr == nil {
		return 0, ErrUnavailable.WithOp("fi_av_insert")
	}
	var out C.fi_addr_t
	status := C.fi_av_insert(a.ptr, addr, 1, &out, C.uint64_t(flags), nil)
	if err := ErrorFromStatus(int(status), "fi_av_insert"); err != nil {
		return 0, err
	}
	return FIAddr(out), nil
}

// Remove removes the provided fi_addr_t entries from the AV.
func (a *AV) Remove(addrs []FIAddr, flags uint64) error {
	if a == nil || a.ptr == nil {
		return ErrUnavailable.WithOp("fi_av_remove")
	}
	if len(addrs) == 0 {
		return nil
	}
	status := C.fi_av_remove(a.ptr, (*C.fi_addr_t)(unsafe.Pointer(&addrs[0])), C.size_t(len(addrs)), C.uint64_t(flags))
	return ErrorFromStatus(int(status), "fi_av_remove")
}
