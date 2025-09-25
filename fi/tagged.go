package fi

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

// TaggedSendRequest describes a tagged message transmit operation.
type TaggedSendRequest struct {
	Buffer  []byte
	Dest    Address
	Tag     uint64
	Context *CompletionContext
}

// TaggedRecvRequest describes a tagged message receive operation.
type TaggedRecvRequest struct {
	Buffer  []byte
	Source  Address
	Tag     uint64
	Ignore  uint64
	Context *CompletionContext
}

// PostTaggedSend posts a tagged send operation when the provider supports tagged messaging.
func (e *Endpoint) PostTaggedSend(req *TaggedSendRequest) (*CompletionContext, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"endpoint"}
	}
	if !e.SupportsTagged() {
		return nil, ErrCapabilityUnsupported
	}
	if req == nil {
		return nil, errors.New("libfabric: nil tagged send request")
	}

	ctx, err := ensureContext(req.Context)
	if err != nil {
		return nil, err
	}

	length := len(req.Buffer)
	dest := req.Dest
	tag := req.Tag

	var cBuf unsafe.Pointer
	if length > 0 {
		var allocErr error
		cBuf, allocErr = ctx.ensureBuffer(uintptr(length))
		if allocErr != nil {
			ctx.Release()
			return nil, allocErr
		}
		capi.Memcpy(cBuf, unsafe.Pointer(&req.Buffer[0]), uintptr(length))
	}

	status := e.handle.TSend(cBuf, uintptr(length), nil, capi.FIAddr(dest), tag, ctx.Pointer())
	if status != nil {
		ctx.Release()
		return nil, status
	}
	return ctx, nil
}

// PostTaggedRecv posts a tagged receive operation.
func (e *Endpoint) PostTaggedRecv(req *TaggedRecvRequest) (*CompletionContext, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"endpoint"}
	}
	if !e.SupportsTagged() {
		return nil, ErrCapabilityUnsupported
	}
	if req == nil {
		return nil, errors.New("libfabric: nil tagged recv request")
	}

	ctx, err := ensureContext(req.Context)
	if err != nil {
		return nil, err
	}

	var cBuf unsafe.Pointer
	if len(req.Buffer) > 0 {
		var allocErr error
		cBuf, allocErr = ctx.ensureBuffer(uintptr(len(req.Buffer)))
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

	if err := e.handle.TRecv(cBuf, uintptr(len(req.Buffer)), nil, capi.FIAddr(source), req.Tag, req.Ignore, ctx.Pointer()); err != nil {
		ctx.Release()
		return nil, err
	}

	return ctx, nil
}

// TaggedSendSync posts a tagged send operation and waits for the matching
// completion event. The helper requires a completion queue configured for
// tagged formats and honours the same timeout semantics as SendSync. For
// connectionless endpoints supply the peer address produced by an address
// vector lookup.
func (e *Endpoint) TaggedSendSync(buf []byte, dest Address, tag uint64, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return fmt.Errorf("libfabric: completion queue required")
	}
	ctx, err := e.PostTaggedSend(&TaggedSendRequest{Buffer: buf, Dest: dest, Tag: tag})
	if err != nil {
		return err
	}
	return waitForContext(cq, ctx, timeout)
}

// TaggedSendSyncContext is the context-aware counterpart to TaggedSendSync.
// Cancellation of ctx aborts the wait loop and returns ctx.Err().
func (e *Endpoint) TaggedSendSyncContext(ctx context.Context, buf []byte, dest Address, tag uint64, cq *CompletionQueue, timeout time.Duration) error {
	if cq == nil {
		return fmt.Errorf("libfabric: completion queue required")
	}
	postCtx, err := e.PostTaggedSend(&TaggedSendRequest{Buffer: buf, Dest: dest, Tag: tag})
	if err != nil {
		return err
	}
	return waitForContextWithContext(ctx, cq, postCtx, timeout)
}

// TaggedRecvSync posts a tagged receive and blocks until the completion queue
// yields the corresponding event or the timeout expires. The returned tag/data
// values mirror the provider's completion payload, allowing callers to inspect
// the received tag without re-reading the queue.
func (e *Endpoint) TaggedRecvSync(buf []byte, tag uint64, ignore uint64, cq *CompletionQueue, timeout time.Duration) (uint64, uint64, error) {
	if cq == nil {
		return 0, 0, fmt.Errorf("libfabric: completion queue required")
	}
	ctx, err := e.PostTaggedRecv(&TaggedRecvRequest{Buffer: buf, Tag: tag, Ignore: ignore})
	if err != nil {
		return 0, 0, err
	}
	evt, err := awaitContextWithEvent(cq, ctx, timeout)
	if err != nil {
		return 0, 0, err
	}
	return evt.Tag, evt.Data, nil
}

// TaggedRecvSyncContext mirrors TaggedRecvSync but aborts when ctx is cancelled.
func (e *Endpoint) TaggedRecvSyncContext(ctx context.Context, buf []byte, tag uint64, ignore uint64, cq *CompletionQueue, timeout time.Duration) (uint64, uint64, error) {
	if cq == nil {
		return 0, 0, fmt.Errorf("libfabric: completion queue required")
	}
	postCtx, err := e.PostTaggedRecv(&TaggedRecvRequest{Buffer: buf, Tag: tag, Ignore: ignore})
	if err != nil {
		return 0, 0, err
	}
	evt, err := awaitContextWithEventContext(ctx, cq, postCtx, timeout)
	if err != nil {
		return 0, 0, err
	}
	return evt.Tag, evt.Data, nil
}
