//go:build cgo

package capi

/*
#cgo pkg-config: libfabric
#include <rdma/fabric.h>
*/
import "C"

const (
	CapMsg         = uint64(C.FI_MSG)
	CapTagged      = uint64(C.FI_TAGGED)
	CapRMA         = uint64(C.FI_RMA)
	CapAtomic      = uint64(C.FI_ATOMIC)
	CapInject      = uint64(C.FI_INJECT)
	CapMultiRecv   = uint64(C.FI_MULTI_RECV)
	CapRemoteRead  = uint64(C.FI_REMOTE_READ)
	CapRemoteWrite = uint64(C.FI_REMOTE_WRITE)
)

const (
	ModeContext   = uint64(C.FI_CONTEXT)
	ModeMsgPrefix = uint64(C.FI_MSG_PREFIX)
)
