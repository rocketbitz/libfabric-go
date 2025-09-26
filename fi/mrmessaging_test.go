package fi

import (
	"testing"
	"time"
)

func TestMessagingWithRegisteredMemorySockets(t *testing.T) {
	desc, fabric, domain := setupSocketsResources(t)
	_ = fabric

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
		t.Skipf("BindCompletionQueue failed: %v", err)
	}
	if err := ep.Enable(); err != nil {
		t.Skipf("endpoint enable failed: %v", err)
	}

	data := []byte("registered message buffer")
	sendMR, err := domain.RegisterMemory(data, MRAccessLocal)
	if err != nil {
		t.Skipf("RegisterMemory unsupported: %v", err)
	}
	t.Cleanup(func() { _ = sendMR.Close() })

	recvBacking := make([]byte, len(data))
	recvMR, err := domain.RegisterMemory(make([]byte, len(data)), MRAccessLocal)
	if err != nil {
		t.Skipf("RegisterMemory (recv) unsupported: %v", err)
	}
	t.Cleanup(func() { _ = recvMR.Close() })

	ctxRecv, err := ep.PostRecv(&RecvRequest{Region: recvMR, Buffer: recvBacking})
	if err != nil {
		t.Fatalf("PostRecv failed: %v", err)
	}

	ctxSend, err := ep.PostSend(&SendRequest{Region: sendMR})
	if err != nil {
		t.Fatalf("PostSend failed: %v", err)
	}

	if err := waitForContext(cq, ctxSend, time.Second); err != nil {
		t.Fatalf("send wait failed: %v", err)
	}
	if err := waitForContext(cq, ctxRecv, time.Second); err != nil {
		t.Fatalf("recv wait failed: %v", err)
	}

	if string(recvBacking) != string(data) {
		t.Fatalf("unexpected recv payload: got %q want %q", string(recvBacking), string(data))
	}

	if string(sendMR.Bytes()) != string(data) {
		t.Fatalf("send region altered unexpectedly")
	}
}
