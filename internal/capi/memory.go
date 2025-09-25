//go:build cgo

package capi

import "unsafe"

/*
#cgo pkg-config: libfabric
#include <stdlib.h>
#include <string.h>
*/
import "C"

// AllocBytes allocates C-managed memory of the specified size.
func AllocBytes(size uintptr) unsafe.Pointer {
	if size == 0 {
		return nil
	}
	ptr := C.malloc(C.size_t(size))
	if ptr == nil {
		return nil
	}
	return ptr
}

// FreeBytes frees memory allocated via AllocBytes.
func FreeBytes(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}
	C.free(ptr)
}

// Memcpy copies length bytes from src to dst using C's memcpy.
func Memcpy(dst, src unsafe.Pointer, length uintptr) {
	if length == 0 || dst == nil || src == nil {
		return
	}
	C.memcpy(dst, src, C.size_t(length))
}
