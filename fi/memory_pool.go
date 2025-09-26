package fi

import (
	"errors"
	"sync/atomic"
)

// MRPool manages reusable memory regions of a fixed size.
type MRPool struct {
	domain *Domain
	size   int
	access MRAccessFlag
	pool   chan *MemoryRegion
	closed atomic.Bool
}

// NewMRPool constructs a pool that dispenses memory regions registered with the supplied domain.
// The pool provisions regions lazily up to the specified capacity.
func NewMRPool(domain *Domain, size int, access MRAccessFlag, capacity int) (*MRPool, error) {
	if domain == nil || domain.handle == nil {
		return nil, ErrInvalidHandle{"domain"}
	}
	if size <= 0 {
		return nil, errors.New("libfabric: MRPool requires positive region size")
	}
	if capacity < 0 {
		capacity = 0
	}
	return &MRPool{
		domain: domain,
		size:   size,
		access: access,
		pool:   make(chan *MemoryRegion, capacity),
	}, nil
}

// Acquire returns a registered memory region from the pool, provisioning a new
// region when the pool is empty. Callers must Release the region when finished.
func (p *MRPool) Acquire() (*MemoryRegion, error) {
	if p == nil {
		return nil, errors.New("libfabric: nil MRPool")
	}
	if p.closed.Load() {
		return nil, errors.New("libfabric: MRPool closed")
	}
	select {
	case mr := <-p.pool:
		return mr, nil
	default:
		buf := make([]byte, p.size)
		mr, err := p.domain.RegisterMemory(buf, p.access)
		if err != nil {
			return nil, err
		}
		return mr, nil
	}
}

// Release returns the memory region to the pool for reuse. Regions with a
// mismatched size or when the pool is closed are closed immediately.
func (p *MRPool) Release(mr *MemoryRegion) {
	if p == nil || mr == nil {
		return
	}
	if p.closed.Load() || int(mr.Size()) != p.size {
		_ = mr.Close()
		return
	}
	select {
	case p.pool <- mr:
	default:
		_ = mr.Close()
	}
}

// Close releases all pooled regions and prevents further acquisitions.
func (p *MRPool) Close() {
	if p == nil || !p.closed.CompareAndSwap(false, true) {
		return
	}
	for {
		select {
		case mr := <-p.pool:
			_ = mr.Close()
		default:
			return
		}
	}
}
