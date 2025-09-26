//go:build cgo

package capi

import "unsafe"

/*
#cgo pkg-config: libfabric
#include <rdma/fi_rma.h>
*/
import "C"

// Read posts an RMA read operation.
func (e *Endpoint) Read(buf unsafe.Pointer, length uintptr, desc unsafe.Pointer, srcAddr FIAddr, key uint64, addr uint64, context unsafe.Pointer) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_read")
	}
	status := C.fi_read(e.ptr, buf, C.size_t(length), desc, C.fi_addr_t(srcAddr), C.uint64_t(addr), C.uint64_t(key), context)
	return ErrorFromStatus(int(status), "fi_read")
}

// Write posts an RMA write operation.
func (e *Endpoint) Write(buf unsafe.Pointer, length uintptr, desc unsafe.Pointer, destAddr FIAddr, key uint64, addr uint64, context unsafe.Pointer) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_write")
	}
	status := C.fi_write(e.ptr, buf, C.size_t(length), desc, C.fi_addr_t(destAddr), C.uint64_t(addr), C.uint64_t(key), context)
	return ErrorFromStatus(int(status), "fi_write")
}
