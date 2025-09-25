package fi

import (
	"context"
	"errors"
	"time"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

// WaitAttr mirrors capi.WaitAttr for wait set configuration.
type WaitAttr struct {
	WaitObj WaitObj
	Flags   uint64
}

// WaitSet wraps a libfabric wait set handle.
type WaitSet struct {
	handle *capi.WaitSetHandle
}

// OpenWaitSet opens a wait set associated with the fabric.
func (f *Fabric) OpenWaitSet(attr *WaitAttr) (*WaitSet, error) {
	if f == nil || f.handle == nil {
		return nil, ErrInvalidHandle{"fabric"}
	}

	var ca *capi.WaitAttr
	var tmp capi.WaitAttr
	if attr != nil {
		tmp = capi.WaitAttr{
			WaitObj: capi.WaitObj(attr.WaitObj),
			Flags:   attr.Flags,
		}
		ca = &tmp
	}

	handle, err := f.handle.OpenWaitSet(ca)
	if err != nil {
		return nil, err
	}
	return &WaitSet{handle: handle}, nil
}

// Close releases the underlying wait set handle.
func (w *WaitSet) Close() error {
	if w == nil || w.handle == nil {
		return nil
	}
	err := w.handle.Close()
	w.handle = nil
	return err
}

// Wait blocks on the wait set until the timeout expires. A negative timeout waits indefinitely.
func (w *WaitSet) Wait(timeout time.Duration) error {
	if w == nil || w.handle == nil {
		return ErrInvalidHandle{"wait set"}
	}
	ms := -1
	if timeout >= 0 {
		ms = int(timeout / time.Millisecond)
	}
	return w.handle.Wait(ms)
}

// FD exposes the wait object's file descriptor when supported by the provider.
func (w *WaitSet) FD() (int, error) {
	if w == nil || w.handle == nil {
		return -1, ErrInvalidHandle{"wait set"}
	}
	return w.handle.FD()
}

func waitForCompletion(ctx context.Context, cq *CompletionQueue, target *CompletionContext, timeout time.Duration, wantEvent bool) (*CompletionEvent, error) {
	if target == nil {
		return nil, nil
	}
	if cq == nil || cq.handle == nil {
		return nil, ErrInvalidHandle{"completion queue"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		evt, err := cq.ReadContext()
		if err != nil {
			if errors.Is(err, ErrNoCompletion) {
				if timeout > 0 && time.Now().After(deadline) {
					return nil, ErrTimeout
				}
				if timeout == 0 {
					return nil, ErrTimeout
				}
				time.Sleep(time.Millisecond)
				continue
			}
			return nil, err
		}

		ctxVal, err := evt.Resolve()
		if err != nil {
			if errors.Is(err, ErrContextUnknown) {
				continue
			}
			return nil, err
		}
		if ctxVal == target {
			if wantEvent {
				return evt, nil
			}
			return nil, nil
		}
	}
}

// awaitContextWithEvent waits for the specified completion context and returns the completion event that resolved it.
func awaitContextWithEvent(cq *CompletionQueue, target *CompletionContext, timeout time.Duration) (*CompletionEvent, error) {
	return awaitContextWithEventContext(nil, cq, target, timeout)
}

func awaitContextWithEventContext(ctx context.Context, cq *CompletionQueue, target *CompletionContext, timeout time.Duration) (*CompletionEvent, error) {
	return waitForCompletion(ctx, cq, target, timeout, true)
}

func waitForContext(cq *CompletionQueue, target *CompletionContext, timeout time.Duration) error {
	_, err := waitForCompletion(nil, cq, target, timeout, false)
	return err
}

func waitForContextWithContext(ctx context.Context, cq *CompletionQueue, target *CompletionContext, timeout time.Duration) error {
	_, err := waitForCompletion(ctx, cq, target, timeout, false)
	return err
}
