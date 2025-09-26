package fi

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

// SendRequest describes a message transmit operation to post on an endpoint.
type SendRequest struct {
	Buffer  []byte
	Dest    Address
	Flags   uint64
	Context *CompletionContext
	Region  *MemoryRegion
}

// RecvRequest describes a message receive operation.
type RecvRequest struct {
	Buffer  []byte
	Source  Address
	Flags   uint64
	Context *CompletionContext
	Region  *MemoryRegion
}

func ensureContext(ctx *CompletionContext) (*CompletionContext, error) {
	if ctx != nil {
		if ctx.IsReleased() {
			return nil, fmt.Errorf("libfabric: completion context already released")
		}
		return ctx, nil
	}
	return NewCompletionContext()
}

// PostSend posts a send operation to the endpoint. The returned CompletionContext
// resolves when the provider reports completion.
func (e *Endpoint) PostSend(req *SendRequest) (*CompletionContext, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"endpoint"}
	}
	if req == nil {
		return nil, errors.New("libfabric: nil send request")
	}

	ctx, err := ensureContext(req.Context)
	if err != nil {
		return nil, err
	}

	dest := req.Dest
	length := len(req.Buffer)

	if length > 0 && req.Context == nil && e.injectLimit > 0 && uintptr(length) <= e.injectLimit {
		if err := e.handle.Inject(unsafe.Pointer(&req.Buffer[0]), uintptr(length), capi.FIAddr(dest)); err == nil {
			ctx.Release()
			return nil, nil
		}
	}

	var cBuf unsafe.Pointer
	var desc unsafe.Pointer
	if req.Region != nil {
		if err := ensureRegionAccess(req.Region, MRAccessLocal); err != nil {
			ctx.Release()
			return nil, err
		}
		cBuf = req.Region.buffer
		desc = req.Region.Descriptor()
		if length == 0 {
			length = int(req.Region.length)
		} else if uintptr(length) > req.Region.length {
			ctx.Release()
			return nil, fmt.Errorf("libfabric: send length exceeds registered region")
		}
	} else if length > 0 {
		var allocErr error
		cBuf, allocErr = ctx.ensureBuffer(uintptr(length))
		if allocErr != nil {
			ctx.Release()
			return nil, allocErr
		}
		capi.Memcpy(cBuf, unsafe.Pointer(&req.Buffer[0]), uintptr(length))
	}
	status := e.handle.Send(cBuf, uintptr(length), desc, capi.FIAddr(dest), ctx.Pointer())
	if status != nil {
		ctx.Release()
		return nil, status
	}

	return ctx, nil
}

// PostRecv posts a receive operation to the endpoint. The provided buffer is
// populated once the completion context resolves.
func (e *Endpoint) PostRecv(req *RecvRequest) (*CompletionContext, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"endpoint"}
	}
	if req == nil {
		return nil, errors.New("libfabric: nil recv request")
	}

	ctx, err := ensureContext(req.Context)
	if err != nil {
		return nil, err
	}

	var cBuf unsafe.Pointer
	var desc unsafe.Pointer
	length := len(req.Buffer)
	if req.Region != nil {
		if err := ensureRegionAccess(req.Region, MRAccessLocal); err != nil {
			ctx.Release()
			return nil, err
		}
		cBuf = req.Region.buffer
		desc = req.Region.Descriptor()
		if length == 0 {
			length = int(req.Region.length)
		} else if uintptr(length) > req.Region.length {
			ctx.Release()
			return nil, fmt.Errorf("libfabric: recv length exceeds registered region")
		}
		if req.Buffer != nil && len(req.Buffer) > 0 {
			ctx.setCopyBack(req.Buffer)
		}
	} else if length > 0 {
		var allocErr error
		cBuf, allocErr = ctx.ensureBuffer(uintptr(length))
		if allocErr != nil {
			ctx.Release()
			return nil, allocErr
		}
		ctx.setCopyBack(req.Buffer)
	}

	source := req.Source
	if source == 0 {
		source = AddressUnspecified
	}

	status := e.handle.Recv(cBuf, uintptr(length), desc, capi.FIAddr(source), ctx.Pointer())
	if status != nil {
		ctx.Release()
		return nil, status
	}

	return ctx, nil
}

// SendSync posts a send and waits for the provider to report completion on the
// supplied completion queue. Callers should pass an explicit destination address
// when working with connectionless endpoints (RDM/datagram). For MSG endpoints
// it is common to use AddressUnspecified. The call polls the queue until the
// associated completion context is resolved or the timeout expires. A timeout
// of zero returns ErrTimeout if the completion is not immediately available;
// a negative timeout waits indefinitely.
func (e *Endpoint) SendSync(buf []byte, dest Address, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return errors.New("libfabric: completion queue required")
	}
	ctx, err := e.PostSend(&SendRequest{Buffer: buf, Dest: dest})
	if err != nil {
		return err
	}
	return waitForContext(cq, ctx, timeout)
}

// SendSyncContext behaves like SendSync but honours cancellation from the
// provided context. When ctx is nil, the call behaves identically to SendSync.
// The timeout parameter retains the same semantics as SendSync; callers can set
// it to -1 to rely solely on ctx for cancellation.
func (e *Endpoint) SendSyncContext(ctx context.Context, buf []byte, dest Address, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return errors.New("libfabric: completion queue required")
	}
	postCtx, err := e.PostSend(&SendRequest{Buffer: buf, Dest: dest})
	if err != nil {
		return err
	}
	return waitForContextWithContext(ctx, cq, postCtx, timeout)
}

// RecvSync posts a receive and blocks until the completion queue reports the
// matching completion or the timeout elapses. The method uses AddressUnspecified
// by default, allowing MSG endpoints to accept any peer. Consumers that need to
// filter by peer address (e.g., RDM/datagram) should prepare a RecvRequest and
// invoke PostRecv instead. Timeout semantics mirror SendSync.
func (e *Endpoint) RecvSync(buf []byte, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return errors.New("libfabric: completion queue required")
	}
	ctx, err := e.PostRecv(&RecvRequest{Buffer: buf})
	if err != nil {
		return err
	}
	return waitForContext(cq, ctx, timeout)
}

// RecvSyncContext mirrors RecvSync but allows the caller to cancel the wait via
// a context. Passing a nil context results in the same behaviour as RecvSync.
func (e *Endpoint) RecvSyncContext(ctx context.Context, buf []byte, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return errors.New("libfabric: completion queue required")
	}
	postCtx, err := e.PostRecv(&RecvRequest{Buffer: buf})
	if err != nil {
		return err
	}
	return waitForContextWithContext(ctx, cq, postCtx, timeout)
}
