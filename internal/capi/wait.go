//go:build cgo

package capi

import "unsafe"

/*
#cgo pkg-config: libfabric
#cgo CFLAGS: -Wno-deprecated-declarations
#include <rdma/fabric.h>
#include <rdma/fi_domain.h>
#include <rdma/fi_eq.h>
*/
import "C"

// WaitAttr configures wait set creation.
type WaitAttr struct {
	WaitObj WaitObj
	Flags   uint64
}

// WaitSetHandle wraps a libfabric fid_wait handle.
type WaitSetHandle struct {
	ptr *C.struct_fid_wait
}

// OpenWaitSet opens a wait set associated with the fabric.
func (f *Fabric) OpenWaitSet(attr *WaitAttr) (*WaitSetHandle, error) {
	if f == nil || f.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_wait_open")
	}

	var wa *C.struct_fi_wait_attr
	var tmp C.struct_fi_wait_attr
	if attr != nil {
		tmp.wait_obj = C.enum_fi_wait_obj(attr.WaitObj)
		tmp.flags = C.uint64_t(attr.Flags)
		wa = &tmp
	}

	var ws *C.struct_fid_wait
	status := C.fi_wait_open(f.ptr, wa, &ws)
	if err := ErrorFromStatus(int(status), "fi_wait_open"); err != nil {
		return nil, err
	}
	return &WaitSetHandle{ptr: ws}, nil
}

// Close releases the wait set.
func (w *WaitSetHandle) Close() error {
	if w == nil || w.ptr == nil {
		return nil
	}
	status := C.fi_close((*C.struct_fid)(unsafe.Pointer(w.ptr)))
	if err := ErrorFromStatus(int(status), "fi_close(waitset)"); err != nil {
		return err
	}
	w.ptr = nil
	return nil
}

// Wait waits for an event on the wait set with the supplied timeout in milliseconds.
// A negative timeout results in an infinite wait.
func (w *WaitSetHandle) Wait(timeout int) error {
	if w == nil || w.ptr == nil {
		return ErrUnavailable.WithOp("fi_wait")
	}
	status := C.fi_wait(w.ptr, C.int(timeout))
	if status >= 0 {
		return nil
	}
	return ErrorFromStatus(int(status), "fi_wait")
}

// FD returns the underlying file descriptor for wait objects that support it.
func (w *WaitSetHandle) FD() (int, error) {
	if w == nil || w.ptr == nil {
		return -1, ErrUnavailable.WithOp("fi_control(FI_GETWAIT)")
	}
	var fd C.int
	status := C.fi_control((*C.struct_fid)(unsafe.Pointer(w.ptr)), C.int(C.FI_GETWAIT), unsafe.Pointer(&fd))
	if err := ErrorFromStatus(int(status), "fi_control(FI_GETWAIT)"); err != nil {
		return -1, err
	}
	return int(fd), nil
}
