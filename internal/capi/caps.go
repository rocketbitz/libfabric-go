//go:build cgo

package capi

/*
#cgo pkg-config: libfabric
#include <rdma/fabric.h>
*/
import "C"

const (
	// CapMsg matches the FI_MSG capability bit.
	CapMsg = uint64(C.FI_MSG)
	// CapTagged matches the FI_TAGGED capability bit.
	CapTagged = uint64(C.FI_TAGGED)
	// CapRMA matches the FI_RMA capability bit.
	CapRMA = uint64(C.FI_RMA)
	// CapAtomic matches the FI_ATOMIC capability bit.
	CapAtomic = uint64(C.FI_ATOMIC)
	// CapInject matches the FI_INJECT capability bit.
	CapInject = uint64(C.FI_INJECT)
	// CapMultiRecv matches the FI_MULTI_RECV capability bit.
	CapMultiRecv = uint64(C.FI_MULTI_RECV)
	// CapRemoteRead matches the FI_REMOTE_READ capability bit.
	CapRemoteRead = uint64(C.FI_REMOTE_READ)
	// CapRemoteWrite matches the FI_REMOTE_WRITE capability bit.
	CapRemoteWrite = uint64(C.FI_REMOTE_WRITE)
)

const (
	// ModeContext matches the FI_CONTEXT mode bit.
	ModeContext = uint64(C.FI_CONTEXT)
	// ModeMsgPrefix matches the FI_MSG_PREFIX mode bit.
	ModeMsgPrefix = uint64(C.FI_MSG_PREFIX)
)
