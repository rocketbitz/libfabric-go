//go:build cgo

package capi

import "unsafe"

/*
#cgo pkg-config: libfabric
#include <stdlib.h>
*/
import "C"

// CompletionContextAlloc allocates an opaque context pointer for use with
// libfabric operations. Call CompletionContextFree to release it after the
// completion has been processed.
func CompletionContextAlloc() unsafe.Pointer {
	ptr := C.malloc(C.size_t(1))
	if ptr == nil {
		return nil
	}
	return ptr
}

// CompletionContextFree releases a context previously allocated with
// CompletionContextAlloc.
func CompletionContextFree(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}
	C.free(ptr)
}
