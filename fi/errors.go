package fi

import (
	"errors"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

var (
	// ErrNoCompletion indicates that no completion entries were available.
	ErrNoCompletion = errors.New("libfabric: no completion available")
	// ErrNoEvent indicates that no event entries were available.
	ErrNoEvent = errors.New("libfabric: no event available")
	// ErrTimeout indicates that a wait operation timed out.
	ErrTimeout = errors.New("libfabric: wait timed out")
	// ErrContextUnknown indicates that a completion context was not found.
	ErrContextUnknown = errors.New("libfabric: completion context not found")
	// ErrCapabilityUnsupported indicates that the provider does not support the requested capability.
	ErrCapabilityUnsupported = errors.New("libfabric: capability not supported")
	// ErrInsufficientAccess indicates that a memory region lacks the required access flags for the requested operation.
	ErrInsufficientAccess = errors.New("libfabric: memory region missing required access")
)

// Errno re-exports the libfabric errno type for consumers of the fi package.
type Errno = capi.Errno
