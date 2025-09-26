package fi

import "testing"

func TestMRPoolAcquireRelease(t *testing.T) {
	_, _, domain := setupSocketsResources(t)

	pool, err := NewMRPool(domain, 64, MRAccessLocal, 2)
	if err != nil {
		t.Fatalf("NewMRPool failed: %v", err)
	}
	defer pool.Close()

	mr1, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if mr1 == nil || mr1.Size() != 64 {
		t.Fatalf("unexpected region from pool")
	}

	pool.Release(mr1)

	mr2, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Acquire after release failed: %v", err)
	}
	if mr2 == nil || mr2.Size() != 64 {
		t.Fatalf("unexpected region size from pool")
	}

	pool.Release(mr2)
}

func TestMRPoolClose(t *testing.T) {
	_, _, domain := setupSocketsResources(t)

	pool, err := NewMRPool(domain, 32, MRAccessLocal, 1)
	if err != nil {
		t.Fatalf("NewMRPool failed: %v", err)
	}

	mr, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	pool.Release(mr)

	pool.Close()

	if _, err := pool.Acquire(); err == nil {
		t.Fatalf("expected error acquiring from closed pool")
	}

	other, err := domain.RegisterMemory(make([]byte, 16), MRAccessLocal)
	if err != nil {
		t.Fatalf("RegisterMemory failed: %v", err)
	}
	pool.Release(other) // should close without panic
}
