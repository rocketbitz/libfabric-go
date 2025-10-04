package client

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	fi "github.com/rocketbitz/libfabric-go/fi"
)

func TestClientSendReceiveAsync(t *testing.T) {
	cli, err := Dial(Config{Timeout: 2 * time.Second})
	if err != nil {
		t.Skipf("Dial skipped: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	payload := []byte("phase6-async")
	recvBuf := make([]byte, len(payload))

	recvFuture, err := cli.ReceiveAsync(recvBuf)
	if err != nil {
		t.Fatalf("ReceiveAsync failed: %v", err)
	}

	callback := make(chan error, 1)
	recvFuture.OnComplete(func(n int, err error) {
		if err != nil {
			callback <- err
			return
		}
		if n != len(payload) {
			callback <- fmt.Errorf("callback length mismatch: got %d want %d", n, len(payload))
			return
		}
		if string(recvBuf[:n]) != string(payload) {
			callback <- fmt.Errorf("callback payload mismatch: got %q want %q", string(recvBuf[:n]), string(payload))
			return
		}
		callback <- nil
	})

	sendFuture, err := cli.SendAsync(payload)
	if err != nil {
		t.Fatalf("SendAsync failed: %v", err)
	}

	if err := sendFuture.Await(context.Background()); err != nil {
		t.Fatalf("Send await failed: %v", err)
	}

	n, err := recvFuture.Await(context.Background())
	if err != nil {
		t.Fatalf("Receive await failed: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("unexpected length: got %d want %d", n, len(payload))
	}
	if string(recvBuf[:n]) != string(payload) {
		t.Fatalf("payload mismatch: got %q want %q", string(recvBuf[:n]), string(payload))
	}

	select {
	case cbErr := <-callback:
		if cbErr != nil {
			t.Fatalf("receive callback error: %v", cbErr)
		}
	case <-time.After(time.Second):
		t.Fatal("receive callback not invoked")
	}
}

func TestClientSendReceiveSync(t *testing.T) {
	cli, err := Dial(Config{Timeout: 2 * time.Second})
	if err != nil {
		t.Skipf("Dial skipped: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	payload := []byte("phase6-sync")
	recvBuf := make([]byte, len(payload))

	recvErr := make(chan error, 1)
	go func() {
		n, err := cli.Receive(context.Background(), recvBuf)
		if err != nil {
			recvErr <- err
			return
		}
		if n != len(payload) {
			recvErr <- fmt.Errorf("unexpected length: got %d want %d", n, len(payload))
			return
		}
		if string(recvBuf[:n]) != string(payload) {
			recvErr <- fmt.Errorf("payload mismatch: got %q want %q", string(recvBuf[:n]), string(payload))
			return
		}
		recvErr <- nil
	}()

	time.Sleep(20 * time.Millisecond)

	if err := cli.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	select {
	case err := <-recvErr:
		if err != nil {
			t.Fatalf("receive failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("receive timed out")
	}
}

func TestClientRDMSendToPeer(t *testing.T) {
	t.Setenv("FI_SOCKETS_IFACE", "lo0")

	sender, receiver, receiverPeerAddr, _ := setupPeerClients(t)

	payload := []byte("rdm-peer-test")
	buf := make([]byte, len(payload))

	recvErr := make(chan error, 1)
	go func() {
		n, err := receiver.Receive(context.Background(), buf)
		if err != nil {
			recvErr <- err
			return
		}
		if n != len(payload) {
			recvErr <- fmt.Errorf("unexpected length: got %d want %d", n, len(payload))
			return
		}
		if string(buf[:n]) != string(payload) {
			recvErr <- fmt.Errorf("payload mismatch: got %q want %q", string(buf[:n]), string(payload))
			return
		}
		recvErr <- nil
	}()

	time.Sleep(50 * time.Millisecond)

	if err := sender.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	select {
	case err := <-recvErr:
		if err != nil {
			t.Fatalf("receive failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("receive timed out")
	}

	buf2 := make([]byte, len(payload))
	recvErr2 := make(chan error, 1)
	go func() {
		n, err := receiver.Receive(context.Background(), buf2)
		if err != nil {
			recvErr2 <- err
			return
		}
		if n != len(payload) || string(buf2[:n]) != string(payload) {
			recvErr2 <- fmt.Errorf("unexpected payload: got %q", string(buf2[:n]))
			return
		}
		recvErr2 <- nil
	}()

	time.Sleep(50 * time.Millisecond)

	if err := sender.SendTo(context.Background(), receiverPeerAddr, payload); err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}

	select {
	case err := <-recvErr2:
		if err != nil {
			t.Fatalf("receive (SendTo) failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("receive (SendTo) timed out")
	}
}

func TestClientSendHandler(t *testing.T) {
	t.Setenv("FI_SOCKETS_IFACE", "lo0")

	sender, receiver, _, _ := setupPeerClients(t)

	handlerCh := make(chan SendCompletion, 1)
	unregister := sender.RegisterSendHandler(func(comp SendCompletion) {
		handlerCh <- comp
	})
	defer unregister()

	payload := []byte("handler-send")
	recvBuf := make([]byte, len(payload))

	recvFuture, err := receiver.ReceiveAsync(recvBuf)
	if err != nil {
		t.Fatalf("ReceiveAsync failed: %v", err)
	}

	sendFuture, err := sender.SendAsync(payload)
	if err != nil {
		t.Fatalf("SendAsync failed: %v", err)
	}

	if err := sendFuture.Await(context.Background()); err != nil {
		t.Fatalf("send await failed: %v", err)
	}

	if _, err := recvFuture.Await(context.Background()); err != nil {
		t.Fatalf("receive await failed: %v", err)
	}

	select {
	case comp := <-handlerCh:
		if comp.Err != nil {
			t.Fatalf("handler error: %v", comp.Err)
		}
		if comp.Size != len(payload) {
			t.Fatalf("unexpected size: got %d want %d", comp.Size, len(payload))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("send handler not invoked")
	}
}

func TestClientReceiveHandler(t *testing.T) {
	t.Setenv("FI_SOCKETS_IFACE", "lo0")

	sender, receiver, _, senderPeerAddr := setupPeerClients(t)

	payload := []byte("handler-recv")
	recvBuf := make([]byte, len(payload))

	handlerCh := make(chan ReceiveCompletion, 1)
	unregister := receiver.RegisterReceiveHandler(func(comp ReceiveCompletion) {
		handlerCh <- comp
	})
	defer unregister()

	recvFuture, err := receiver.ReceiveAsync(recvBuf)
	if err != nil {
		t.Fatalf("ReceiveAsync failed: %v", err)
	}

	if err := sender.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	select {
	case comp := <-handlerCh:
		if comp.Err != nil {
			t.Fatalf("handler error: %v", comp.Err)
		}
		if string(comp.Payload) != string(payload) {
			t.Fatalf("handler payload mismatch: got %q want %q", string(comp.Payload), string(payload))
		}
		// mutate original buffer to ensure handler payload is an isolated copy
		copy(recvBuf, []byte("mutated"))
		if comp.Source != senderPeerAddr {
			t.Fatalf("handler source mismatch: got %v want %v", comp.Source, senderPeerAddr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("receive handler not invoked")
	}

	if n, err := recvFuture.Await(context.Background()); err != nil {
		t.Fatalf("receive await failed: %v", err)
	} else if n != len(payload) {
		t.Fatalf("unexpected length: got %d want %d", n, len(payload))
	}
	if src := recvFuture.Source(); src != senderPeerAddr {
		t.Fatalf("future source mismatch: got %v want %v", src, senderPeerAddr)
	}
}

func TestClientReceiveFrom(t *testing.T) {
	t.Setenv("FI_SOCKETS_IFACE", "lo0")

	sender, receiver, receiverPeerAddr, senderPeerAddr := setupPeerClients(t)

	payload := []byte("receive-from")

	type result struct {
		n    int
		addr fi.Address
		err  error
	}

	resCh := make(chan result, 1)

	go func() {
		buf := make([]byte, len(payload))
		n, addr, err := receiver.ReceiveFrom(context.Background(), buf)
		if err == nil && string(buf[:n]) != string(payload) {
			err = fmt.Errorf("payload mismatch: got %q", string(buf[:n]))
		}
		resCh <- result{n: n, addr: addr, err: err}
	}()

	time.Sleep(50 * time.Millisecond)

	if err := sender.SendTo(context.Background(), receiverPeerAddr, payload); err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}

	select {
	case res := <-resCh:
		if res.err != nil {
			t.Fatalf("ReceiveFrom failed: %v", res.err)
		}
		if res.n != len(payload) {
			t.Fatalf("unexpected length: got %d want %d", res.n, len(payload))
		}
		if res.addr != senderPeerAddr {
			t.Fatalf("ReceiveFrom addr mismatch: got %v want %v", res.addr, senderPeerAddr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReceiveFrom timed out")
	}
}

func TestClientMSGListenerConnect(t *testing.T) {
	t.Setenv("FI_SOCKETS_IFACE", "lo0")

	service := freePort(t)

	listener, err := Listen(ListenerConfig{Provider: "sockets", Node: "127.0.0.1", Service: service})
	if err != nil {
		t.Skipf("Listen unavailable: %v", err)
	}
	defer listener.Close()

	acceptCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverCh := make(chan *Client, 1)
	errCh := make(chan error, 1)

	go func() {
		conn, err := listener.Accept(acceptCtx)
		if err != nil {
			errCh <- err
			return
		}
		serverCh <- conn
	}()

	clientConn, err := Connect(Config{Provider: "sockets", EndpointType: fi.EndpointTypeMsg, Node: "127.0.0.1", Service: service, Timeout: 5 * time.Second})
	if err != nil {
		t.Skipf("Connect skipped: %v", err)
	}
	defer clientConn.Close()

	var serverConn *Client
	select {
	case serverConn = <-serverCh:
	case err := <-errCh:
		t.Fatalf("Accept failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Accept timed out")
	}
	defer serverConn.Close()

	message := []byte("hello-msg")
	recvBuf := make([]byte, len(message))

	recvDone := make(chan error, 1)
	go func() {
		n, err := serverConn.Receive(context.Background(), recvBuf)
		if err != nil {
			recvDone <- err
			return
		}
		if n != len(message) || string(recvBuf[:n]) != string(message) {
			recvDone <- fmt.Errorf("unexpected message: %q", string(recvBuf[:n]))
			return
		}
		recvDone <- nil
	}()

	if err := clientConn.Send(context.Background(), message); err != nil {
		t.Fatalf("client send failed: %v", err)
	}

	if err := <-recvDone; err != nil {
		t.Fatalf("server receive failed: %v", err)
	}

	ack := []byte("ack")
	ackBuf := make([]byte, len(ack))
	tAck := make(chan error, 1)
	go func() {
		n, err := clientConn.Receive(context.Background(), ackBuf)
		if err != nil {
			tAck <- err
			return
		}
		if n != len(ack) || string(ackBuf[:n]) != string(ack) {
			tAck <- fmt.Errorf("unexpected ack: %q", string(ackBuf[:n]))
			return
		}
		tAck <- nil
	}()

	if err := serverConn.Send(context.Background(), ack); err != nil {
		t.Fatalf("server send failed: %v", err)
	}

	if err := <-tAck; err != nil {
		t.Fatalf("client receive ack failed: %v", err)
	}
}

func TestClientStats(t *testing.T) {
	t.Setenv("FI_SOCKETS_IFACE", "lo0")

	sender, receiver, receiverPeerAddr, _ := setupPeerClients(t)

	payload := []byte("stats")
	recvBuf := make([]byte, len(payload))

	recvFuture, err := receiver.ReceiveAsync(recvBuf)
	if err != nil {
		t.Fatalf("ReceiveAsync failed: %v", err)
	}

	if err := sender.SendTo(context.Background(), receiverPeerAddr, payload); err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}

	if _, err := recvFuture.Await(context.Background()); err != nil {
		t.Fatalf("Receive await failed: %v", err)
	}

	sStats := sender.Stats()
	if sStats.SendPosted != 1 || sStats.SendCompleted != 1 || sStats.SendErrored != 0 {
		t.Fatalf("unexpected sender stats: %+v", sStats)
	}

	rStats := receiver.Stats()
	if rStats.ReceivePosted != 1 || rStats.ReceiveMatched != 1 || rStats.ReceiveErrored != 0 {
		t.Fatalf("unexpected receiver stats: %+v", rStats)
	}
}

func TestClientStructuredLoggingAndTracing(t *testing.T) {
	t.Setenv("FI_SOCKETS_IFACE", "lo0")

	logger, observedLogs := newObservedLogger()
	tp, recorder := newTestTracerProvider()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}()
	tracer := &otelTracerAdapter{tracer: tp.Tracer("client-structured-test")}

	metrics := newMetricRecorder()
	cfg := Config{
		Timeout:          2 * time.Second,
		EndpointType:     fi.EndpointTypeRDM,
		Logger:           logger,
		StructuredLogger: logger,
		Tracer:           tracer,
		Metrics:          metrics,
	}

	sender, err := Dial(cfg)
	if err != nil {
		t.Skipf("sender Dial skipped: %v", err)
	}
	defer func() { _ = sender.Close() }()

	receiver, err := Dial(cfg)
	if err != nil {
		t.Skipf("receiver Dial skipped: %v", err)
	}
	defer func() { _ = receiver.Close() }()

	receiverAddr, err := receiver.LocalAddress()
	if err != nil {
		t.Fatalf("receiver LocalAddress: %v", err)
	}
	receiverDest, err := sender.RegisterPeer(receiverAddr, true)
	if err != nil {
		t.Fatalf("sender RegisterPeer: %v", err)
	}

	senderAddr, err := sender.LocalAddress()
	if err != nil {
		t.Fatalf("sender LocalAddress: %v", err)
	}
	_, err = receiver.RegisterPeer(senderAddr, true)
	if err != nil {
		t.Fatalf("receiver RegisterPeer: %v", err)
	}

	payload := []byte("structured-logging")
	recvBuf := make([]byte, len(payload))

	recvFuture, err := receiver.ReceiveAsync(recvBuf)
	if err != nil {
		t.Fatalf("ReceiveAsync failed: %v", err)
	}

	if err := sender.SendTo(context.Background(), receiverDest, payload); err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}

	n, err := recvFuture.Await(context.Background())
	if err != nil {
		t.Fatalf("Receive await failed: %v", err)
	}
	if n != len(payload) || string(recvBuf[:n]) != string(payload) {
		t.Fatalf("unexpected payload: %q", string(recvBuf[:n]))
	}

	if err := sender.Close(); err != nil {
		t.Fatalf("sender close failed: %v", err)
	}
	if err := receiver.Close(); err != nil {
		t.Fatalf("receiver close failed: %v", err)
	}

	if !waitForLogEvent(observedLogs, "start", time.Second) {
		t.Fatal("missing dispatcher start log")
	}
	if !waitForLogEvent(observedLogs, "completion", time.Second) {
		t.Fatal("missing dispatcher completion log")
	}
	if !waitForLogEvent(observedLogs, "stop", time.Second) {
		t.Fatal("missing dispatcher stop log")
	}

	if !spanHasEvent(recorder, "start") {
		t.Fatal("missing dispatcher start span event")
	}
	if !spanHasEvent(recorder, "completion") {
		t.Fatal("missing dispatcher completion span event")
	}
	if !spanHasEvent(recorder, "stop") {
		t.Fatal("missing dispatcher stop span event")
	}

	_ = logger.Sync()

	snapshot := metrics.Snapshot()
	if snapshot.DispatcherStarted < 2 {
		t.Fatalf("expected >=2 dispatcher starts, got %d", snapshot.DispatcherStarted)
	}
	if snapshot.DispatcherStopped < 2 {
		t.Fatalf("expected >=2 dispatcher stops, got %d", snapshot.DispatcherStopped)
	}
	if snapshot.SendCompleted < 1 {
		t.Fatalf("expected at least one send completion, got %d", snapshot.SendCompleted)
	}
	if snapshot.ReceiveCompleted < 1 {
		t.Fatalf("expected at least one receive completion, got %d", snapshot.ReceiveCompleted)
	}
	if snapshot.SendFailed != 0 || snapshot.ReceiveFailed != 0 {
		t.Fatalf("unexpected failure metrics: send=%d recv=%d", snapshot.SendFailed, snapshot.ReceiveFailed)
	}
	if len(snapshot.CQErrors) != 0 {
		t.Fatalf("unexpected CQ errors recorded: %+v", snapshot.CQErrors)
	}
}

func TestClientDispatcherLogsCQError(t *testing.T) {
	t.Setenv("FI_SOCKETS_IFACE", "lo0")

	logger, observedLogs := newObservedLogger()
	tp, recorder := newTestTracerProvider()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}()
	tracer := &otelTracerAdapter{tracer: tp.Tracer("client-cq-error-test")}

	metrics := newMetricRecorder()
	cfg := Config{
		Timeout:          2 * time.Second,
		EndpointType:     fi.EndpointTypeRDM,
		Logger:           logger,
		StructuredLogger: logger,
		Tracer:           tracer,
		Metrics:          metrics,
	}

	cli, err := Dial(cfg)
	if err != nil {
		t.Skipf("Dial skipped: %v", err)
	}
	// Ensure cleanup in case of early return.
	defer func() { _ = cli.Close() }()

	if err := cli.cq.Close(); err != nil {
		t.Fatalf("close completion queue: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var dispatchErr error
	for time.Now().Before(deadline) {
		dispatchErr = cli.dispatchFailure()
		if dispatchErr != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if dispatchErr == nil {
		t.Fatal("expected dispatcher failure after CQ close")
	}

	if err := cli.Close(); err != nil {
		t.Fatalf("client close failed: %v", err)
	}

	if !waitForLogEvent(observedLogs, "cq_read_error", time.Second) && !waitForLogEvent(observedLogs, "cq_readerr_error", time.Second) {
		t.Fatal("missing dispatcher CQ error log entry")
	}
	if !spanHasEvent(recorder, "cq_read_error") && !spanHasEvent(recorder, "cq_readerr_error") {
		t.Fatal("missing dispatcher CQ error span event")
	}

	_ = logger.Sync()

	snapshot := metrics.Snapshot()
	if snapshot.DispatcherStarted < 1 {
		t.Fatalf("expected dispatcher to start, got %d", snapshot.DispatcherStarted)
	}
	if snapshot.DispatcherStopped < 1 {
		t.Fatalf("expected dispatcher to stop, got %d", snapshot.DispatcherStopped)
	}
	if len(snapshot.CQErrors) == 0 {
		t.Fatal("expected dispatcher CQ error metric")
	}
	if snapshot.SendCompleted != 0 || snapshot.ReceiveCompleted != 0 {
		t.Fatalf("unexpected data-path completion metrics: send=%d recv=%d", snapshot.SendCompleted, snapshot.ReceiveCompleted)
	}
}

func setupPeerClients(t *testing.T) (*Client, *Client, fi.Address, fi.Address) {
	t.Helper()
	cfg := Config{Timeout: 2 * time.Second, EndpointType: fi.EndpointTypeRDM}

	sender, err := Dial(cfg)
	if err != nil {
		t.Skipf("sender Dial skipped: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

	receiver, err := Dial(cfg)
	if err != nil {
		t.Skipf("receiver Dial skipped: %v", err)
	}
	t.Cleanup(func() { _ = receiver.Close() })

	receiverAddr, err := receiver.LocalAddress()
	if err != nil {
		t.Fatalf("receiver LocalAddress: %v", err)
	}

	receiverPeerAddr, err := sender.RegisterPeer(receiverAddr, true)
	if err != nil {
		t.Fatalf("RegisterPeer failed: %v", err)
	}

	senderAddrBytes, err := sender.LocalAddress()
	if err != nil {
		t.Fatalf("sender LocalAddress: %v", err)
	}
	senderPeerAddr, err := receiver.RegisterPeer(senderAddrBytes, false)
	if err != nil {
		t.Fatalf("receiver RegisterPeer failed: %v", err)
	}

	return sender, receiver, receiverPeerAddr, senderPeerAddr
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("free port unavailable: %v", err)
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return strconv.Itoa(addr.Port)
}

func newObservedLogger() (*zap.SugaredLogger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	return logger.Sugar(), logs
}

func newTestTracerProvider() (*tracesdk.TracerProvider, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	tp := tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder))
	return tp, recorder
}

func waitForLogEvent(logs *observer.ObservedLogs, event string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		entries := logs.All()
		for _, entry := range entries {
			if evt, ok := entry.ContextMap()["event"].(string); ok && evt == event {
				return true
			}
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func spanHasEvent(recorder *tracetest.SpanRecorder, event string) bool {
	for _, span := range recorder.Ended() {
		if span.Name() != "libfabric-client-dispatcher" {
			continue
		}
		for _, evt := range span.Events() {
			if evt.Name == event {
				return true
			}
		}
	}
	return false
}

type otelTracerAdapter struct {
	tracer trace.Tracer
}

func (o *otelTracerAdapter) StartSpan(name string, attrs ...TraceAttribute) Span {
	if o == nil || o.tracer == nil {
		return nil
	}
	attributes := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		attributes = append(attributes, toAttribute(attr))
	}
	_, span := o.tracer.Start(context.Background(), name, trace.WithAttributes(attributes...))
	return &otelSpanAdapter{span: span}
}

type otelSpanAdapter struct {
	span trace.Span
}

func (s *otelSpanAdapter) End(err error) {
	if s == nil || s.span == nil {
		return
	}
	if err != nil {
		s.span.RecordError(err)
	}
	s.span.End()
}

func (s *otelSpanAdapter) AddEvent(name string, attrs ...TraceAttribute) {
	if s == nil || s.span == nil {
		return
	}
	attributes := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		attributes = append(attributes, toAttribute(attr))
	}
	s.span.AddEvent(name, trace.WithAttributes(attributes...))
}

func (s *otelSpanAdapter) RecordError(err error) {
	if s == nil || s.span == nil || err == nil {
		return
	}
	s.span.RecordError(err)
}

func toAttribute(attr TraceAttribute) attribute.KeyValue {
	if attr.Key == "" {
		return attribute.String("undefined", fmt.Sprint(attr.Value))
	}
	switch v := attr.Value.(type) {
	case nil:
		return attribute.String(attr.Key, "")
	case string:
		return attribute.String(attr.Key, v)
	case fmt.Stringer:
		return attribute.String(attr.Key, v.String())
	case bool:
		return attribute.Bool(attr.Key, v)
	case int:
		return attribute.Int(attr.Key, v)
	case int8:
		return attribute.Int(attr.Key, int(v))
	case int16:
		return attribute.Int(attr.Key, int(v))
	case int32:
		return attribute.Int(attr.Key, int(v))
	case int64:
		return attribute.Int64(attr.Key, v)
	case uint:
		return attribute.Int64(attr.Key, int64(v))
	case uint8:
		return attribute.Int(attr.Key, int(v))
	case uint16:
		return attribute.Int(attr.Key, int(v))
	case uint32:
		return attribute.Int64(attr.Key, int64(v))
	case uint64:
		return attribute.Int64(attr.Key, int64(v))
	case fi.Address:
		return attribute.Int64(attr.Key, int64(v))
	case float32:
		return attribute.Float64(attr.Key, float64(v))
	case float64:
		return attribute.Float64(attr.Key, v)
	case error:
		return attribute.String(attr.Key, v.Error())
	default:
		return attribute.String(attr.Key, fmt.Sprint(attr.Value))
	}
}

type metricRecorder struct {
	mu                sync.Mutex
	dispatcherStarted int
	dispatcherStopped int
	cqErrors          []string
	sendCompleted     int
	sendFailed        int
	receiveCompleted  int
	receiveFailed     int
}

func newMetricRecorder() *metricRecorder {
	return &metricRecorder{}
}

func (m *metricRecorder) DispatcherStarted(_ map[string]string) {
	m.mu.Lock()
	m.dispatcherStarted++
	m.mu.Unlock()
}

func (m *metricRecorder) DispatcherStopped(_ map[string]string) {
	m.mu.Lock()
	m.dispatcherStopped++
	m.mu.Unlock()
}

func (m *metricRecorder) DispatcherCQError(kind string, _ error, _ map[string]string) {
	m.mu.Lock()
	m.cqErrors = append(m.cqErrors, kind)
	m.mu.Unlock()
}

func (m *metricRecorder) SendCompleted(_ map[string]string) {
	m.mu.Lock()
	m.sendCompleted++
	m.mu.Unlock()
}

func (m *metricRecorder) SendFailed(_ error, _ map[string]string) {
	m.mu.Lock()
	m.sendFailed++
	m.mu.Unlock()
}

func (m *metricRecorder) ReceiveCompleted(_ map[string]string) {
	m.mu.Lock()
	m.receiveCompleted++
	m.mu.Unlock()
}

func (m *metricRecorder) ReceiveFailed(_ error, _ map[string]string) {
	m.mu.Lock()
	m.receiveFailed++
	m.mu.Unlock()
}

func (m *metricRecorder) Snapshot() metricSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyErrors := append([]string(nil), m.cqErrors...)
	return metricSnapshot{
		DispatcherStarted: m.dispatcherStarted,
		DispatcherStopped: m.dispatcherStopped,
		CQErrors:          copyErrors,
		SendCompleted:     m.sendCompleted,
		SendFailed:        m.sendFailed,
		ReceiveCompleted:  m.receiveCompleted,
		ReceiveFailed:     m.receiveFailed,
	}
}

type metricSnapshot struct {
	DispatcherStarted int
	DispatcherStopped int
	CQErrors          []string
	SendCompleted     int
	SendFailed        int
	ReceiveCompleted  int
	ReceiveFailed     int
}
