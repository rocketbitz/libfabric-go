//go:build cgo

package capi

import (
	"unsafe"
)

/*
#cgo pkg-config: libfabric
#include <rdma/fi_tagged.h>
*/
import "C"

// TSend posts a tagged send operation.
func (e *Endpoint) TSend(buffer unsafe.Pointer, length uintptr, desc unsafe.Pointer, dest FIAddr, tag uint64, context unsafe.Pointer) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_tsend")
	}
	status := C.fi_tsend(e.ptr, buffer, C.size_t(length), desc, C.fi_addr_t(dest), C.uint64_t(tag), context)
	return ErrorFromStatus(int(status), "fi_tsend")
}

// TRecv posts a tagged receive operation.
func (e *Endpoint) TRecv(buffer unsafe.Pointer, length uintptr, desc unsafe.Pointer, src FIAddr, tag uint64, ignore uint64, context unsafe.Pointer) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_trecv")
	}
	status := C.fi_trecv(e.ptr, buffer, C.size_t(length), desc, C.fi_addr_t(src), C.uint64_t(tag), C.uint64_t(ignore), context)
	return ErrorFromStatus(int(status), "fi_trecv")
}

// TInject sends a tagged message using the inject fast-path.
func (e *Endpoint) TInject(buffer unsafe.Pointer, length uintptr, dest FIAddr, tag uint64) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_tinject")
	}
	status := C.fi_tinject(e.ptr, buffer, C.size_t(length), C.fi_addr_t(dest), C.uint64_t(tag))
	return ErrorFromStatus(int(status), "fi_tinject")
}
