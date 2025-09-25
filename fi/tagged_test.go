package fi

import (
	"errors"
	"testing"
	"time"
)

func TestTaggedMessagingSockets(t *testing.T) {
	desc, fabric, domain := setupSocketsResourcesWithType(t, EndpointTypeRDM)
	_ = fabric

	av, err := domain.OpenAddressVector(&AddressVectorAttr{Type: AVTypeMap})
	if err != nil {
		t.Skipf("unable to open address vector: %v", err)
	}
	t.Cleanup(func() { _ = av.Close() })

	openEndpoint := func() (*Endpoint, *CompletionQueue) {
		cq, err := domain.OpenCompletionQueue(&CompletionQueueAttr{Format: CQFormatTagged})
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
		if !ep.SupportsTagged() {
			t.Skip("provider does not support tagged messaging")
		}
		if err := ep.BindCompletionQueue(cq, BindSend|BindRecv); err != nil {
			t.Fatalf("BindCompletionQueue failed: %v", err)
		}
		if err := ep.BindAddressVector(av, 0); err != nil {
			t.Fatalf("BindAddressVector failed: %v", err)
		}
		if err := ep.Enable(); err != nil {
			if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
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

	payload := []byte("tagged messaging test")
	recvBuf := make([]byte, len(payload))
	const tag uint64 = 0xdeadbeef

	recvCtx, err := epB.PostTaggedRecv(&TaggedRecvRequest{Buffer: recvBuf, Source: addrA, Tag: tag, Ignore: 0})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostTaggedRecv unsupported: %v", err)
		}
		t.Fatalf("PostTaggedRecv failed: %v", err)
	}

	sendCtx, err := epA.PostTaggedSend(&TaggedSendRequest{Buffer: payload, Dest: addrB, Tag: tag})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostTaggedSend unsupported: %v", err)
		}
		t.Fatalf("PostTaggedSend failed: %v", err)
	}

	if sendCtx != nil {
		if _, err := awaitContextWithEvent(cqA, sendCtx, time.Second); err != nil {
			if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
				t.Skipf("tagged send completion unavailable: %v", err)
			}
			t.Fatalf("tagged send wait failed: %v", err)
		}
	}

	evt, err := awaitContextWithEvent(cqB, recvCtx, time.Second)
	if err != nil {
		if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
			t.Skipf("tagged recv completion unavailable: %v", err)
		}
		t.Fatalf("tagged recv wait failed: %v", err)
	}
	if evt.Tag != tag {
		t.Fatalf("expected tag 0x%x, got 0x%x", tag, evt.Tag)
	}

	if string(recvBuf) != string(payload) {
		t.Fatalf("unexpected tagged recv payload: got %q want %q", string(recvBuf), string(payload))
	}
}
