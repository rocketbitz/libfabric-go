package fi

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

// RMARequest describes a remote memory access operation.
type RMARequest struct {
	Buffer  []byte
	Region  *MemoryRegion
	Key     uint64
	Offset  uint64
	Address Address
	Context *CompletionContext
	Flags   uint64
}

// PostRead posts an RMA read from the remote address into the local buffer or registered region.
func (e *Endpoint) PostRead(req *RMARequest) (*CompletionContext, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"endpoint"}
	}
	if req == nil {
		return nil, errors.New("libfabric: nil RMA request")
	}

	ctx, err := ensureContext(req.Context)
	if err != nil {
		return nil, err
	}

	var buf unsafe.Pointer
	var desc unsafe.Pointer
	length := len(req.Buffer)

	if req.Region != nil {
		if err := ensureRegionAccess(req.Region, MRAccessLocal); err != nil {
			ctx.Release()
			return nil, err
		}
		buf = req.Region.buffer
		desc = req.Region.Descriptor()
		if length == 0 {
			length = int(req.Region.length)
		} else if uintptr(length) > req.Region.length {
			ctx.Release()
			return nil, fmt.Errorf("libfabric: read length exceeds registered region")
		}
		if len(req.Buffer) > 0 {
			ctx.setCopyBack(req.Buffer)
		}
	} else if length > 0 {
		var allocErr error
		buf, allocErr = ctx.ensureBuffer(uintptr(length))
		if allocErr != nil {
			ctx.Release()
			return nil, allocErr
		}
		ctx.setCopyBack(req.Buffer)
	} else {
		ctx.Release()
		return nil, errors.New("libfabric: RMA read requires buffer or region")
	}

	if err := e.handle.Read(buf, uintptr(length), desc, capi.FIAddr(req.Address), req.Key, req.Offset, ctx.Pointer()); err != nil {
		ctx.Release()
		return nil, err
	}
	return ctx, nil
}

// PostWrite posts an RMA write from the local buffer or region to the remote address.
func (e *Endpoint) PostWrite(req *RMARequest) (*CompletionContext, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"endpoint"}
	}
	if req == nil {
		return nil, errors.New("libfabric: nil RMA request")
	}

	ctx, err := ensureContext(req.Context)
	if err != nil {
		return nil, err
	}

	var buf unsafe.Pointer
	var desc unsafe.Pointer
	length := len(req.Buffer)

	if req.Region != nil {
		if err := ensureRegionAccess(req.Region, MRAccessLocal); err != nil {
			ctx.Release()
			return nil, err
		}
		buf = req.Region.buffer
		desc = req.Region.Descriptor()
		if length == 0 {
			length = int(req.Region.length)
		} else if uintptr(length) > req.Region.length {
			ctx.Release()
			return nil, fmt.Errorf("libfabric: write length exceeds registered region")
		}
	} else if length > 0 {
		var allocErr error
		buf, allocErr = ctx.ensureBuffer(uintptr(length))
		if allocErr != nil {
			ctx.Release()
			return nil, allocErr
		}
		capi.Memcpy(buf, unsafe.Pointer(&req.Buffer[0]), uintptr(length))
	} else {
		ctx.Release()
		return nil, errors.New("libfabric: RMA write requires buffer or region")
	}

	if err := e.handle.Write(buf, uintptr(length), desc, capi.FIAddr(req.Address), req.Key, req.Offset, ctx.Pointer()); err != nil {
		ctx.Release()
		return nil, err
	}
	return ctx, nil
}

// ReadSync posts a read and waits for completion.
func (e *Endpoint) ReadSync(req *RMARequest, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return errors.New("libfabric: completion queue required")
	}
	ctx, err := e.PostRead(req)
	if err != nil {
		return err
	}
	return waitForContext(cq, ctx, timeout)
}

// WriteSync posts a write and waits for completion.
func (e *Endpoint) WriteSync(req *RMARequest, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return errors.New("libfabric: completion queue required")
	}
	ctx, err := e.PostWrite(req)
	if err != nil {
		return err
	}
	return waitForContext(cq, ctx, timeout)
}

// ReadSyncContext posts a read and waits for completion with context cancellation support.
func (e *Endpoint) ReadSyncContext(ctx context.Context, req *RMARequest, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return errors.New("libfabric: completion queue required")
	}
	completion, err := e.PostRead(req)
	if err != nil {
		return err
	}
	return waitForContextWithContext(ctx, cq, completion, timeout)
}

// WriteSyncContext posts a write and waits for completion with context cancellation support.
func (e *Endpoint) WriteSyncContext(ctx context.Context, req *RMARequest, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return errors.New("libfabric: completion queue required")
	}
	completion, err := e.PostWrite(req)
	if err != nil {
		return err
	}
	return waitForContextWithContext(ctx, cq, completion, timeout)
}
