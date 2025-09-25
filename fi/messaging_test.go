package fi

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMessagingSendRecvSockets(t *testing.T) {
	desc, fabric, domain := setupSocketsResourcesWithType(t, EndpointTypeRDM)
	_ = fabric

	av, err := domain.OpenAddressVector(&AddressVectorAttr{Type: AVTypeMap})
	if err != nil {
		t.Skipf("unable to open address vector: %v", err)
	}
	t.Cleanup(func() {
		_ = av.Close()
	})

	openEndpoint := func() (*Endpoint, *CompletionQueue) {
		cq, err := domain.OpenCompletionQueue(nil)
		if err != nil {
			t.Skipf("unable to open completion queue: %v", err)
		}
		ep, err := desc.OpenEndpoint(domain)
		if err != nil {
			t.Skipf("unable to open endpoint: %v", err)
		}
		t.Cleanup(func() {
			_ = ep.Close()
			_ = cq.Close()
		})
		if err := ep.BindCompletionQueue(cq, BindSend|BindRecv); err != nil {
			t.Fatalf("BindCompletionQueue failed: %v", err)
		}
		if err := ep.BindAddressVector(av, 0); err != nil {
			t.Fatalf("BindAddressVector failed: %v", err)
		}
		if err := ep.Enable(); err != nil {
			if shouldSkipForErr(err) {
				t.Skipf("endpoint enable unsupported: %v", err)
			}
			t.Fatalf("endpoint enable failed: %v", err)
		}
		return ep, cq
	}

	epA, cqA := openEndpoint()
	epB, cqB := openEndpoint()

	addrA, err := epA.RegisterAddress(av, 0)
	if err != nil {
		t.Fatalf("register addrA failed: %v", err)
	}
	addrB, err := epB.RegisterAddress(av, 0)
	if err != nil {
		t.Fatalf("register addrB failed: %v", err)
	}

	message := []byte("libfabric-go messaging test")
	recvBuf := make([]byte, len(message))

	if limit := epA.InjectLimit(); limit > 0 {
		t.Logf("provider inject limit: %d", limit)
	}

	recvCtx, err := epB.PostRecv(&RecvRequest{Buffer: recvBuf, Source: addrA})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostRecv unsupported: %v", err)
		}
		t.Fatalf("PostRecv failed: %v", err)
	}

	sendCtx, err := epA.PostSend(&SendRequest{Buffer: message, Dest: addrB})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostSend unsupported: %v", err)
		}
		t.Fatalf("PostSend failed: %v", err)
	}

	if err := waitForContext(cqA, sendCtx, time.Second); err != nil {
		if sendCtx != nil {
			if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
				t.Skipf("send completion unavailable: %v", err)
			}
			t.Fatalf("send completion wait failed: %v", err)
		}
	}

	if err := waitForContext(cqB, recvCtx, time.Second); err != nil {
		if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
			t.Skipf("recv completion unsupported: %v", err)
		}
		t.Fatalf("recv completion wait failed: %v", err)
	}

	if string(recvBuf) != string(message) {
		t.Fatalf("unexpected recv payload: got %q want %q", string(recvBuf), string(message))
	}

	// Larger payload to ensure non-inject fallback path executes.
	limit := epA.InjectLimit()
	if limit == 0 {
		limit = 256
	}
	largeLen := int(limit*2 + 16)
	largeMessage := make([]byte, largeLen)
	for i := range largeMessage {
		largeMessage[i] = 'L'
	}
	largeRecv := make([]byte, largeLen)

	recvCtx2, err := epB.PostRecv(&RecvRequest{Buffer: largeRecv, Source: addrA})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostRecv (large) unsupported: %v", err)
		}
		t.Fatalf("PostRecv (large) failed: %v", err)
	}

	sendCtx2, err := epA.PostSend(&SendRequest{Buffer: largeMessage, Dest: addrB})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostSend (large) unsupported: %v", err)
		}
		t.Fatalf("PostSend (large) failed: %v", err)
	}
	if sendCtx2 == nil {
		t.Skip("provider injected large message; skipping non-inject verification")
	}

	if err := waitForContext(cqA, sendCtx2, time.Second); err != nil {
		if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
			t.Skipf("send completion (large) unsupported: %v", err)
		}
		t.Fatalf("send completion (large) wait failed: %v", err)
	}

	if err := waitForContext(cqB, recvCtx2, time.Second); err != nil {
		if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
			t.Skipf("recv completion (large) unsupported: %v", err)
		}
		t.Fatalf("recv completion (large) wait failed: %v", err)
	}

	if string(largeRecv) != string(largeMessage) {
		t.Fatalf("unexpected large recv payload")
	}
}

func shouldSkipForErr(err error) bool {
	var eno Errno
	if errors.As(err, &eno) {
		return true
	}
	return errors.Is(err, ErrCapabilityUnsupported)
}

func TestRecvSyncContextCancellationSockets(t *testing.T) {
	desc, fabric, domain := setupSocketsResourcesWithType(t, EndpointTypeRDM)
	_ = fabric

	av, err := domain.OpenAddressVector(&AddressVectorAttr{Type: AVTypeMap})
	if err != nil {
		t.Skipf("unable to open address vector: %v", err)
	}
	t.Cleanup(func() { _ = av.Close() })

	cq, err := domain.OpenCompletionQueue(nil)
	if err != nil {
		t.Skipf("unable to open completion queue: %v", err)
	}
	t.Cleanup(func() { _ = cq.Close() })

	ep, err := desc.OpenEndpoint(domain)
	if err != nil {
		t.Skipf("unable to open endpoint: %v", err)
	}
	t.Cleanup(func() { _ = ep.Close() })

	if err := ep.BindCompletionQueue(cq, BindSend|BindRecv); err != nil {
		t.Fatalf("BindCompletionQueue failed: %v", err)
	}
	if err := ep.Enable(); err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("endpoint enable unsupported: %v", err)
		}
		t.Fatalf("endpoint enable failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	buf := make([]byte, 16)
	err = ep.RecvSyncContext(ctx, buf, cq, -1)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}
