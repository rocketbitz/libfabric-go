package fi

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

var (
	contextRegistry sync.Map // uintptr -> *CompletionContext
)

// CompletionContext carries metadata and cleanup actions associated with
// a posted libfabric operation. Instances are not safe for reuse after they
// have been resolved or released.
type CompletionContext struct {
	ptr        unsafe.Pointer
	value      any
	mu         sync.Mutex
	onComplete []func(*CompletionContext)
	cleanups   []func()
	buffer     unsafe.Pointer
	bufCap     uintptr
	closed     atomic.Bool
	copyBack   []byte
}

func (c *CompletionContext) setCopyBack(buf []byte) {
	c.mu.Lock()
	c.copyBack = buf
	c.mu.Unlock()
}

// NewCompletionContext allocates a new completion context and registers it with
// the runtime so completions can be resolved back into Go.
func NewCompletionContext() (*CompletionContext, error) {
	ptr := capi.CompletionContextAlloc()
	if ptr == nil {
		return nil, fmt.Errorf("libfabric: unable to allocate completion context")
	}
	ctx := &CompletionContext{ptr: ptr}
	contextRegistry.Store(uintptr(ptr), ctx)
	return ctx, nil
}

// Value returns the associated arbitrary value.
func (c *CompletionContext) Value() any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// SetValue attaches arbitrary metadata to the context for retrieval when the
// completion is resolved.
func (c *CompletionContext) SetValue(v any) {
	c.mu.Lock()
	c.value = v
	c.mu.Unlock()
}

// Pointer exposes the underlying opaque pointer passed to libfabric.
func (c *CompletionContext) Pointer() unsafe.Pointer {
	return c.ptr
}

// AddOnComplete registers a callback executed exactly once when the completion
// is resolved.
func (c *CompletionContext) AddOnComplete(fn func(*CompletionContext)) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	c.onComplete = append(c.onComplete, fn)
	c.mu.Unlock()
}

// AddCleanup registers a cleanup callback that runs after completion callbacks.
func (c *CompletionContext) AddCleanup(fn func()) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	c.cleanups = append(c.cleanups, fn)
	c.mu.Unlock()
}

// IsReleased reports whether the context has already been resolved or released.
func (c *CompletionContext) IsReleased() bool {
	return c.closed.Load()
}

// Release tears down the context without executing completion callbacks. It is
// intended for failure paths where the libfabric operation did not get posted.
func (c *CompletionContext) Release() {
	c.release(false, true)
}

func (c *CompletionContext) release(runComplete bool, remove bool) {
	if c == nil {
		return
	}
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	if remove {
		contextRegistry.Delete(uintptr(c.ptr))
	}

	c.mu.Lock()
	completions := append([]func(*CompletionContext){}, c.onComplete...)
	cleanups := append([]func(){}, c.cleanups...)
	copyBack := c.copyBack
	c.onComplete = nil
	c.cleanups = nil
	c.copyBack = nil
	c.mu.Unlock()

	if runComplete {
		for _, fn := range completions {
			fn(c)
		}
	}
	if copyBack != nil && c.buffer != nil {
		capi.Memcpy(unsafe.Pointer(&copyBack[0]), c.buffer, uintptr(len(copyBack)))
		copyBack = nil
	}
	if c.buffer != nil {
		capi.FreeBytes(c.buffer)
		c.buffer = nil
		c.bufCap = 0
	}
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
	capi.CompletionContextFree(c.ptr)
}

func resolveCompletion(ptr unsafe.Pointer) (*CompletionContext, error) {
	if ptr == nil {
		return nil, ErrContextUnknown
	}
	value, ok := contextRegistry.LoadAndDelete(uintptr(ptr))
	if !ok {
		return nil, ErrContextUnknown
	}
	ctx := value.(*CompletionContext)
	ctx.release(true, false)
	return ctx, nil
}

func (c *CompletionContext) ensureBuffer(size uintptr) (unsafe.Pointer, error) {
	if size == 0 {
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.buffer != nil && c.bufCap >= size {
		return c.buffer, nil
	}
	if c.buffer != nil {
		capi.FreeBytes(c.buffer)
		c.buffer = nil
		c.bufCap = 0
	}
	buf := capi.AllocBytes(size)
	if buf == nil {
		return nil, fmt.Errorf("libfabric: unable to allocate context buffer")
	}
	c.buffer = buf
	c.bufCap = size
	return buf, nil
}
