package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fi "github.com/rocketbitz/libfabric-go/fi"
)

// ErrClosed indicates the client has already been closed.
var ErrClosed = errors.New("libfabric client: closed")

// Config controls Dial behaviour for the high-level Client.
type Config struct {
	Provider         string
	EndpointType     fi.EndpointType
	Timeout          time.Duration
	MRPoolSize       int
	MRPoolCapacity   int
	MRPoolAccess     fi.MRAccessFlag
	Node             string
	Service          string
	Logger           Logger
	StructuredLogger StructuredLogger
	Tracer           Tracer
	Metrics          MetricHook
}

// Client owns the resources necessary to perform message send/receive operations.
type Client struct {
	cfg            Config
	fabric         *fi.Fabric
	domain         *fi.Domain
	endpoint       *fi.Endpoint
	cq             *fi.CompletionQueue
	eq             *fi.EventQueue
	av             *fi.AddressVector
	selfAddr       fi.Address
	peerAddr       atomic.Uint64
	peerConfigured atomic.Bool
	selfRaw        []byte
	mrPool         *fi.MRPool
	mrAccess       fi.MRAccessFlag
	requiresMR     bool
	closed         atomic.Bool
	dispatcherErr  atomic.Pointer[errorHolder]

	stopCh chan struct{}
	wg     sync.WaitGroup

	handlersMu      sync.RWMutex
	sendHandlers    map[uint64]SendHandler
	receiveHandlers map[uint64]ReceiveHandler
	handlerSeq      atomic.Uint64

	logger           Logger
	structuredLogger StructuredLogger
	tracer           Tracer
	metrics          MetricHook
	stats            clientStats
	ownFabric        bool
	ownDomain        bool
	ownEndpoint      bool
	ownCompletion    bool
	ownEventQueue    bool
}

// OperationKind identifies the type of libfabric operation tracked by a future.
type OperationKind int

type errorHolder struct {
	err error
}

const (
	OperationSend OperationKind = iota
	OperationReceive
)

func (k OperationKind) String() string {
	switch k {
	case OperationSend:
		return "send"
	case OperationReceive:
		return "receive"
	default:
		return "operation"
	}
}

// OperationError exposes detailed completion error information surfaced by libfabric.
type OperationError struct {
	Kind        OperationKind
	Errno       fi.Errno
	ProviderErr int
	Flags       uint64
	Length      uint64
	Data        uint64
	Tag         uint64
}

// SendCompletion describes the outcome of a send operation dispatched through a handler.
type SendCompletion struct {
	Size int
	Err  error
}

// ReceiveCompletion describes a completed receive operation delivered through a handler.
type ReceiveCompletion struct {
	Payload []byte
	Source  fi.Address
	Err     error
}

// SendHandler is invoked when a send operation completes.
type SendHandler func(SendCompletion)

// ReceiveHandler is invoked when a receive operation completes.
type ReceiveHandler func(ReceiveCompletion)

// Logger provides structured debug logging hooks for the client.
type Logger interface {
	Debugf(format string, args ...any)
}

// StructuredLogger emits key/value pairs for structured logging backends.
type StructuredLogger interface {
	Debugw(msg string, keyvals ...any)
}

// TraceAttribute represents a tracing attribute attached to dispatcher spans or events.
type TraceAttribute struct {
	Key   string
	Value any
}

// Tracer starts spans that wrap dispatcher activity.
type Tracer interface {
	StartSpan(name string, attrs ...TraceAttribute) Span
}

// Span records dispatcher lifecycle, events, and errors for tracing systems.
type Span interface {
	End(err error)
	AddEvent(name string, attrs ...TraceAttribute)
	RecordError(err error)
}

// Stats contains counters for client operations.
type Stats struct {
	SendPosted     uint64
	SendCompleted  uint64
	SendErrored    uint64
	ReceivePosted  uint64
	ReceiveMatched uint64
	ReceiveErrored uint64
}

type clientStats struct {
	sendPosted    atomic.Uint64
	sendCompleted atomic.Uint64
	sendErrored   atomic.Uint64
	recvPosted    atomic.Uint64
	recvMatched   atomic.Uint64
	recvErrored   atomic.Uint64
}

// MetricHook captures dispatcher telemetry events.
type MetricHook interface {
	DispatcherStarted(attrs map[string]string)
	DispatcherStopped(attrs map[string]string)
	DispatcherCQError(kind string, err error, attrs map[string]string)
	SendCompleted(attrs map[string]string)
	SendFailed(err error, attrs map[string]string)
	ReceiveCompleted(attrs map[string]string)
	ReceiveFailed(err error, attrs map[string]string)
}

type logField struct {
	key   string
	value any
}

func logKV(key string, value any) logField {
	return logField{key: key, value: value}
}

func (c *Client) metricAttrs(fields ...logField) map[string]string {
	attrs := make(map[string]string, len(fields)+4)
	attrs["endpoint_type"] = c.cfg.EndpointType.String()
	if c.cfg.Provider != "" {
		attrs["provider"] = c.cfg.Provider
	}
	if c.cfg.Node != "" {
		attrs["node"] = c.cfg.Node
	}
	if c.cfg.Service != "" {
		attrs["service"] = c.cfg.Service
	}
	for _, field := range fields {
		if field.key == "" {
			continue
		}
		attrs[field.key] = fmt.Sprint(field.value)
	}
	return attrs
}

func (c *Client) logDispatcherEvent(event string, fields ...logField) {
	if c == nil {
		return
	}
	if c.structuredLogger != nil {
		kv := make([]any, 0, len(fields)*2+2)
		kv = append(kv, "event", event)
		for _, field := range fields {
			if field.key == "" {
				continue
			}
			kv = append(kv, field.key, field.value)
		}
		c.structuredLogger.Debugw("libfabric client dispatcher", kv...)
		return
	}
	if c.logger == nil {
		return
	}
	var b strings.Builder
	b.WriteString(event)
	for _, field := range fields {
		if field.key == "" {
			continue
		}
		b.WriteString(" ")
		b.WriteString(field.key)
		b.WriteString("=")
		b.WriteString(fmt.Sprint(field.value))
	}
	c.logger.Debugf("client dispatcher %s", b.String())
}

func (c *Client) metricDispatcherStarted(fields ...logField) {
	if c == nil || c.metrics == nil {
		return
	}
	c.metrics.DispatcherStarted(c.metricAttrs(fields...))
}

func (c *Client) metricDispatcherStopped(fields ...logField) {
	if c == nil || c.metrics == nil {
		return
	}
	c.metrics.DispatcherStopped(c.metricAttrs(fields...))
}

func (c *Client) metricDispatcherCQError(kind string, err error, fields ...logField) {
	if c == nil || c.metrics == nil {
		return
	}
	c.metrics.DispatcherCQError(kind, err, c.metricAttrs(fields...))
}

func (c *Client) metricSendCompleted(fields ...logField) {
	if c == nil || c.metrics == nil {
		return
	}
	c.metrics.SendCompleted(c.metricAttrs(fields...))
}

func (c *Client) metricSendFailed(err error, fields ...logField) {
	if c == nil || c.metrics == nil {
		return
	}
	c.metrics.SendFailed(err, c.metricAttrs(fields...))
}

func (c *Client) metricReceiveCompleted(fields ...logField) {
	if c == nil || c.metrics == nil {
		return
	}
	c.metrics.ReceiveCompleted(c.metricAttrs(fields...))
}

func (c *Client) metricReceiveFailed(err error, fields ...logField) {
	if c == nil || c.metrics == nil {
		return
	}
	c.metrics.ReceiveFailed(err, c.metricAttrs(fields...))
}

func (e OperationError) Error() string {
	return fmt.Sprintf("libfabric %s completion error: %s (provider=%d flags=0x%x len=%d)", e.Kind, e.Errno, e.ProviderErr, e.Flags, e.Length)
}

// Unwrap allows errors.Is / errors.As to match against the underlying Errno.
func (e OperationError) Unwrap() error {
	return e.Errno
}

type operationResult struct {
	length int
	err    error
}

type operation struct {
	client  *Client
	kind    OperationKind
	size    int
	done    chan struct{}
	release func()
	meta    any

	mu        sync.Mutex
	once      sync.Once
	completed bool
	result    operationResult
	callbacks []func(operationResult)
}

type receiveMeta struct {
	buffer []byte
	source atomic.Uint64
}

func newOperation(client *Client, kind OperationKind, size int, meta any) *operation {
	return &operation{
		client: client,
		kind:   kind,
		size:   size,
		done:   make(chan struct{}),
		meta:   meta,
	}
}

func (op *operation) complete(res operationResult) {
	op.once.Do(func() {
		op.mu.Lock()
		op.result = res
		op.completed = true
		callbacks := append([]func(operationResult){}, op.callbacks...)
		op.callbacks = nil
		op.mu.Unlock()

		if op.client != nil {
			op.client.emit(op, res)
		}

		if op.release != nil {
			op.release()
		}

		close(op.done)

		for _, cb := range callbacks {
			cb := cb
			go cb(res)
		}
	})
}

func (op *operation) resultSnapshot() operationResult {
	op.mu.Lock()
	defer op.mu.Unlock()
	return op.result
}

func (op *operation) addCallback(cb func(operationResult)) {
	if cb == nil {
		return
	}
	op.mu.Lock()
	if op.completed {
		res := op.result
		op.mu.Unlock()
		go cb(res)
		return
	}
	op.callbacks = append(op.callbacks, cb)
	op.mu.Unlock()
}

// SendFuture tracks the completion of a posted send operation.
type SendFuture struct {
	op *operation
}

// Await blocks until the send operation completes or the context is cancelled.
func (f *SendFuture) Await(ctx context.Context) error {
	if f == nil || f.op == nil {
		return errors.New("libfabric client: nil send future")
	}
	ctx = ensureContext(ctx)
	select {
	case <-ctx.Done():
		select {
		case <-f.op.done:
			return f.op.resultSnapshot().err
		default:
		}
		return ctx.Err()
	case <-f.op.done:
		return f.op.resultSnapshot().err
	}
}

// Done exposes a channel that closes when the send operation resolves.
func (f *SendFuture) Done() <-chan struct{} {
	if f == nil || f.op == nil {
		return nil
	}
	return f.op.done
}

// OnComplete registers a callback invoked asynchronously when the send resolves.
func (f *SendFuture) OnComplete(fn func(error)) {
	if f == nil || f.op == nil || fn == nil {
		return
	}
	f.op.addCallback(func(res operationResult) {
		fn(res.err)
	})
}

// ReceiveFuture tracks the completion of a posted receive operation.
type ReceiveFuture struct {
	op   *operation
	buf  []byte
	meta *receiveMeta
}

// Await blocks until the receive resolves or the context is cancelled.
func (f *ReceiveFuture) Await(ctx context.Context) (int, error) {
	if f == nil || f.op == nil {
		return 0, errors.New("libfabric client: nil receive future")
	}
	ctx = ensureContext(ctx)
	select {
	case <-ctx.Done():
		select {
		case <-f.op.done:
			res := f.op.resultSnapshot()
			return res.length, res.err
		default:
		}
		return 0, ctx.Err()
	case <-f.op.done:
		res := f.op.resultSnapshot()
		return res.length, res.err
	}
}

// Buffer returns the caller-provided buffer passed to ReceiveAsync.
func (f *ReceiveFuture) Buffer() []byte {
	if f == nil {
		return nil
	}
	return f.buf
}

// Source returns the address of the peer that produced the data, when available.
func (f *ReceiveFuture) Source() fi.Address {
	if f == nil || f.meta == nil {
		return 0
	}
	return fi.Address(f.meta.source.Load())
}

// Done exposes a channel that closes when the receive completes.
func (f *ReceiveFuture) Done() <-chan struct{} {
	if f == nil || f.op == nil {
		return nil
	}
	return f.op.done
}

// OnComplete registers a callback invoked asynchronously once data arrives.
func (f *ReceiveFuture) OnComplete(fn func(int, error)) {
	if f == nil || f.op == nil || fn == nil {
		return
	}
	f.op.addCallback(func(res operationResult) {
		fn(res.length, res.err)
	})
}

// Dial discovers a compatible provider and prepares the client resources.
func Dial(cfg Config) (*Client, error) {
	if cfg.Provider == "" {
		cfg.Provider = "sockets"
	}
	if cfg.EndpointType == 0 {
		// TODO: add MSG default once connection management is layered into the high-level client.
		cfg.EndpointType = fi.EndpointTypeRDM
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}

	opts := []fi.DiscoverOption{fi.WithProvider(cfg.Provider), fi.WithEndpointType(cfg.EndpointType)}
	discovery, err := fi.DiscoverDescriptors(opts...)
	if err != nil {
		return nil, fmt.Errorf("discover descriptors: %w", err)
	}
	defer discovery.Close()

	descriptors := discovery.Descriptors()
	if len(descriptors) == 0 {
		return nil, fmt.Errorf("no descriptors found for provider %s", cfg.Provider)
	}

	var selected *fi.Descriptor
	for i := range descriptors {
		info := descriptors[i].Info()
		if info.Endpoint == cfg.EndpointType {
			selected = &descriptors[i]
			break
		}
	}
	if selected == nil {
		selected = &descriptors[0]
	}

	info := selected.Info()

	fabric, err := selected.OpenFabric()
	if err != nil {
		return nil, fmt.Errorf("open fabric: %w", err)
	}

	domain, err := selected.OpenDomain(fabric)
	if err != nil {
		fabric.Close()
		return nil, fmt.Errorf("open domain: %w", err)
	}

	var cqAttr *fi.CompletionQueueAttr
	if cfg.EndpointType == fi.EndpointTypeRDM {
		cqAttr = &fi.CompletionQueueAttr{Format: fi.CQFormatMsg}
	}
	cq, err := domain.OpenCompletionQueue(cqAttr)
	if err != nil {
		domain.Close()
		fabric.Close()
		return nil, fmt.Errorf("open completion queue: %w", err)
	}

	endpoint, err := selected.OpenEndpoint(domain)
	if err != nil {
		cq.Close()
		domain.Close()
		fabric.Close()
		return nil, fmt.Errorf("open endpoint: %w", err)
	}

	if err := endpoint.BindCompletionQueue(cq, fi.BindSend|fi.BindRecv); err != nil {
		endpoint.Close()
		cq.Close()
		domain.Close()
		fabric.Close()
		return nil, fmt.Errorf("bind completion queue: %w", err)
	}

	if err := endpoint.Enable(); err != nil {
		endpoint.Close()
		cq.Close()
		domain.Close()
		fabric.Close()
		return nil, fmt.Errorf("enable endpoint: %w", err)
	}

	var av *fi.AddressVector
	var selfAddr fi.Address
	var selfRaw []byte
	if cfg.EndpointType == fi.EndpointTypeRDM {
		av, err = domain.OpenAddressVector(&fi.AddressVectorAttr{Type: fi.AVTypeMap})
		if err != nil {
			endpoint.Close()
			cq.Close()
			domain.Close()
			fabric.Close()
			return nil, fmt.Errorf("open address vector: %w", err)
		}
		if err := endpoint.BindAddressVector(av, 0); err != nil {
			av.Close()
			endpoint.Close()
			cq.Close()
			domain.Close()
			fabric.Close()
			return nil, fmt.Errorf("bind address vector: %w", err)
		}
		selfAddr, err = endpoint.RegisterAddress(av, 0)
		if err != nil {
			av.Close()
			endpoint.Close()
			cq.Close()
			domain.Close()
			fabric.Close()
			return nil, fmt.Errorf("register endpoint address: %w", err)
		}
		selfRaw, err = endpoint.Name()
		if err != nil {
			av.Close()
			endpoint.Close()
			cq.Close()
			domain.Close()
			fabric.Close()
			return nil, fmt.Errorf("query endpoint address: %w", err)
		}
	}

	structured := cfg.StructuredLogger
	if structured == nil {
		if logger, ok := cfg.Logger.(StructuredLogger); ok {
			structured = logger
		}
	}
	metrics := cfg.Metrics

	client := &Client{
		cfg:              cfg,
		fabric:           fabric,
		domain:           domain,
		endpoint:         endpoint,
		cq:               cq,
		stopCh:           make(chan struct{}),
		av:               av,
		selfAddr:         selfAddr,
		selfRaw:          selfRaw,
		logger:           cfg.Logger,
		structuredLogger: structured,
		tracer:           cfg.Tracer,
		metrics:          metrics,
	}
	client.ownFabric = true
	client.ownDomain = true
	client.ownEndpoint = true
	client.ownCompletion = true

	client.peerAddr.Store(uint64(selfAddr))
	client.peerConfigured.Store(true)

	if av != nil && (cfg.Node != "" || cfg.Service != "") {
		peerAddr, err := av.InsertService(cfg.Node, cfg.Service, 0)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("register peer service: %w", err)
		}
		client.peerAddr.Store(uint64(peerAddr))
		client.peerConfigured.Store(true)
	}

	client.requiresMR = domain.RequiresMRMode(fi.MRModeLocal)
	access := cfg.MRPoolAccess
	if access == 0 {
		access = fi.MRAccessLocal
	}
	client.mrAccess = access

	poolSize := cfg.MRPoolSize
	poolCapacity := cfg.MRPoolCapacity
	if client.requiresMR {
		if poolSize <= 0 {
			if info.InjectSize > 0 {
				poolSize = int(info.InjectSize)
			}
			if poolSize <= 0 {
				poolSize = 4096
			}
		}
		if poolCapacity <= 0 {
			poolCapacity = 32
		}
	}

	if poolSize > 0 {
		pool, err := fi.NewMRPool(domain, poolSize, access, poolCapacity)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("create MR pool: %w", err)
		}
		client.mrPool = pool
		client.cfg.MRPoolSize = poolSize
		client.cfg.MRPoolCapacity = poolCapacity
	}

	client.wg.Add(1)
	go client.dispatch()

	return client, nil
}

// Close releases the underlying libfabric resources.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}

	close(c.stopCh)
	c.wg.Wait()

	c.handlersMu.Lock()
	c.sendHandlers = nil
	c.receiveHandlers = nil
	c.handlersMu.Unlock()

	if c.mrPool != nil {
		c.mrPool.Close()
	}
	if c.av != nil {
		_ = c.av.Close()
	}
	if c.endpoint != nil && c.ownEndpoint {
		_ = c.endpoint.Close()
	}
	if c.cq != nil && c.ownCompletion {
		_ = c.cq.Close()
	}
	if c.eq != nil && c.ownEventQueue {
		_ = c.eq.Close()
	}
	if c.domain != nil && c.ownDomain {
		_ = c.domain.Close()
	}
	if c.fabric != nil && c.ownFabric {
		_ = c.fabric.Close()
	}
	return nil
}

// Send posts a blocking send operation using the configured timeout when the
// supplied context lacks a deadline.
func (c *Client) Send(ctx context.Context, payload []byte) error {
	ctx, cancel := c.operationContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	dest, err := c.defaultDestination()
	if err != nil {
		return err
	}
	future, err := c.sendAsync(payload, dest)
	if err != nil {
		return err
	}
	return future.Await(ctx)
}

// SendAsync posts a send and returns a future that resolves when libfabric reports completion.
func (c *Client) SendAsync(payload []byte) (*SendFuture, error) {
	dest, err := c.defaultDestination()
	if err != nil {
		return nil, err
	}
	return c.sendAsync(payload, dest)
}

// SendTo transmits payload to the specified destination address.
func (c *Client) SendTo(ctx context.Context, dest fi.Address, payload []byte) error {
	ctx, cancel := c.operationContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	future, err := c.sendAsync(payload, dest)
	if err != nil {
		return err
	}
	return future.Await(ctx)
}

// SendToAsync posts a send targeted at the provided destination address.
func (c *Client) SendToAsync(payload []byte, dest fi.Address) (*SendFuture, error) {
	return c.sendAsync(payload, dest)
}

func (c *Client) sendAsync(payload []byte, dest fi.Address) (*SendFuture, error) {
	if err := c.ensureOpen(); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, errors.New("libfabric client: empty payload")
	}
	if err := c.dispatchFailure(); err != nil {
		return nil, err
	}

	req, release, err := c.prepareSend(payload, dest)
	if err != nil {
		return nil, err
	}
	postedSize := len(req.Buffer)
	if postedSize == 0 && req.Region != nil {
		postedSize = int(req.Region.Size())
	}

	op := newOperation(c, OperationSend, postedSize, nil)
	op.release = release

	ctx, err := c.endpoint.PostSend(&req)
	if err != nil {
		if release != nil {
			release()
		}
		return nil, fmt.Errorf("post send: %w", err)
	}
	c.stats.sendPosted.Add(1)
	c.logf("client: send posted size=%d dest=%v", postedSize, dest)

	if ctx == nil {
		op.complete(operationResult{length: len(req.Buffer)})
		return &SendFuture{op: op}, nil
	}

	ctx.SetValue(op)
	return &SendFuture{op: op}, nil
}

// Receive posts a blocking receive, filling buf once the completion resolves.
func (c *Client) Receive(ctx context.Context, buf []byte) (int, error) {
	count, _, err := c.ReceiveFrom(ctx, buf)
	return count, err
}

// ReceiveFrom behaves like Receive but also returns the peer address.
func (c *Client) ReceiveFrom(ctx context.Context, buf []byte) (int, fi.Address, error) {
	ctx, cancel := c.operationContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}
	future, err := c.ReceiveAsync(buf)
	if err != nil {
		return 0, 0, err
	}
	count, err := future.Await(ctx)
	if err != nil {
		return 0, 0, err
	}
	return count, future.Source(), nil
}

// ReceiveAsync posts a receive and returns a future that resolves when data arrives.
func (c *Client) ReceiveAsync(buf []byte) (*ReceiveFuture, error) {
	if err := c.ensureOpen(); err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, errors.New("libfabric client: buffer must be non-empty")
	}
	if err := c.dispatchFailure(); err != nil {
		return nil, err
	}

	req, release, err := c.prepareReceive(buf)
	if err != nil {
		return nil, err
	}

	recvMeta := &receiveMeta{buffer: buf}
	op := newOperation(c, OperationReceive, len(buf), recvMeta)
	op.release = release
	repostedSize := len(buf)
	if req.Region != nil && repostedSize == 0 {
		repostedSize = int(req.Region.Size())
	}

	ctx, err := c.endpoint.PostRecv(&req)
	if err != nil {
		if release != nil {
			release()
		}
		return nil, fmt.Errorf("post recv: %w", err)
	}
	c.stats.recvPosted.Add(1)
	c.logf("client: receive posted size=%d", repostedSize)
	ctx.SetValue(op)

	return &ReceiveFuture{op: op, buf: buf, meta: recvMeta}, nil
}

func (c *Client) ensureOpen() error {
	if c == nil {
		return ErrClosed
	}
	if c.closed.Load() {
		return ErrClosed
	}
	return nil
}

func (c *Client) dispatchFailure() error {
	if err := c.dispatcherError(); err != nil {
		return fmt.Errorf("libfabric client dispatcher failed: %w", err)
	}
	return nil
}

func (c *Client) defaultDestination() (fi.Address, error) {
	switch c.cfg.EndpointType {
	case fi.EndpointTypeRDM, fi.EndpointTypeDgram:
		if !c.peerConfigured.Load() {
			return 0, errors.New("libfabric client: destination address not configured")
		}
		dest := fi.Address(c.peerAddr.Load())
		return dest, nil
	default:
		return fi.AddressUnspecified, nil
	}
}

// Stats returns a snapshot of client counters.
func (c *Client) Stats() Stats {
	if c == nil {
		return Stats{}
	}
	return Stats{
		SendPosted:     c.stats.sendPosted.Load(),
		SendCompleted:  c.stats.sendCompleted.Load(),
		SendErrored:    c.stats.sendErrored.Load(),
		ReceivePosted:  c.stats.recvPosted.Load(),
		ReceiveMatched: c.stats.recvMatched.Load(),
		ReceiveErrored: c.stats.recvErrored.Load(),
	}
}

func (c *Client) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := c.cfg.Timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ctx, func() {}
		}
		if timeout <= 0 || remaining < timeout {
			return ctx, func() {}
		}
		timeout = remaining
	}
	if timeout <= 0 {
		return ctx, func() {}
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	return ctxWithTimeout, cancel
}

func (c *Client) prepareSend(payload []byte, dest fi.Address) (fi.SendRequest, func(), error) {
	if len(payload) == 0 {
		return fi.SendRequest{}, nil, errors.New("libfabric client: empty payload")
	}

	switch c.cfg.EndpointType {
	case fi.EndpointTypeRDM, fi.EndpointTypeDgram:
		if dest == fi.AddressUnspecified {
			return fi.SendRequest{}, nil, errors.New("libfabric client: destination address required")
		}
	case fi.EndpointTypeMsg:
		dest = fi.AddressUnspecified
	}

	req := fi.SendRequest{Dest: dest}

	if c.mrPool != nil && len(payload) <= c.mrPool.Size() {
		region, err := c.mrPool.Acquire()
		if err != nil {
			return fi.SendRequest{}, nil, err
		}
		buf := region.Bytes()
		if len(buf) < len(payload) {
			c.mrPool.Release(region)
			return fi.SendRequest{}, nil, fmt.Errorf("libfabric client: MR pool region too small (have %d want %d)", len(buf), len(payload))
		}
		copy(buf[:len(payload)], payload)
		req.Region = region
		req.Buffer = buf[:len(payload)]
		return req, func() { c.mrPool.Release(region) }, nil
	}

	if c.requiresMR {
		region, err := c.domain.RegisterMemory(payload, c.mrAccess)
		if err != nil {
			return fi.SendRequest{}, nil, err
		}
		req.Region = region
		req.Buffer = region.Bytes()[:len(payload)]
		return req, func() { _ = region.Close() }, nil
	}

	req.Buffer = payload
	return req, nil, nil
}

func (c *Client) prepareReceive(buf []byte) (fi.RecvRequest, func(), error) {
	req := fi.RecvRequest{Buffer: buf}
	if len(buf) == 0 {
		return req, nil, errors.New("libfabric client: buffer must be non-empty")
	}

	if c.mrPool != nil && len(buf) <= c.mrPool.Size() {
		region, err := c.mrPool.Acquire()
		if err != nil {
			return fi.RecvRequest{}, nil, err
		}
		if int(region.Size()) < len(buf) {
			c.mrPool.Release(region)
			return fi.RecvRequest{}, nil, fmt.Errorf("libfabric client: MR pool region too small (have %d want %d)", region.Size(), len(buf))
		}
		req.Region = region
		return req, func() { c.mrPool.Release(region) }, nil
	}

	if c.requiresMR {
		zero := make([]byte, len(buf))
		region, err := c.domain.RegisterMemory(zero, c.mrAccess)
		if err != nil {
			return fi.RecvRequest{}, nil, err
		}
		req.Region = region
		return req, func() { _ = region.Close() }, nil
	}

	return req, nil, nil
}

// LocalAddress returns the provider-specific address bytes for the client's endpoint.
func (c *Client) LocalAddress() ([]byte, error) {
	if err := c.ensureOpen(); err != nil {
		return nil, err
	}
	if len(c.selfRaw) > 0 {
		dup := make([]byte, len(c.selfRaw))
		copy(dup, c.selfRaw)
		return dup, nil
	}
	addr, err := c.endpoint.Name()
	if err != nil {
		return nil, err
	}
	dup := make([]byte, len(addr))
	copy(dup, addr)
	return dup, nil
}

// RegisterPeer inserts the supplied provider address into the client's address vector.
// When setDefault is true, subsequent calls to Send/SendAsync target the peer automatically.
func (c *Client) RegisterPeer(addr []byte, setDefault bool) (fi.Address, error) {
	if err := c.ensureOpen(); err != nil {
		return 0, err
	}
	if c.av == nil {
		return 0, errors.New("libfabric client: address vector unavailable for this endpoint type")
	}
	if len(addr) == 0 {
		return 0, errors.New("libfabric client: peer address must be non-empty")
	}
	fiAddr, err := c.av.InsertRaw(addr, 0)
	if err != nil {
		return 0, err
	}
	if setDefault {
		c.peerAddr.Store(uint64(fiAddr))
		c.peerConfigured.Store(true)
	}
	return fiAddr, nil
}

// SetDefaultPeer configures the destination address used by Send/SendAsync for RDM endpoints.
func (c *Client) SetDefaultPeer(addr fi.Address) {
	if c == nil {
		return
	}
	c.peerAddr.Store(uint64(addr))
	c.peerConfigured.Store(true)
}

// DefaultPeer returns the currently configured destination address for automatic sends.
func (c *Client) DefaultPeer() fi.Address {
	if c == nil {
		return 0
	}
	return fi.Address(c.peerAddr.Load())
}

// RegisterSendHandler installs a callback invoked for every completed send. The returned
// function unregisters the handler when invoked. Passing a nil handler is a no-op.
func (c *Client) RegisterSendHandler(handler SendHandler) func() {
	if c == nil || handler == nil {
		return func() {}
	}
	id := c.handlerSeq.Add(1)
	c.handlersMu.Lock()
	if c.sendHandlers == nil {
		c.sendHandlers = make(map[uint64]SendHandler)
	}
	c.sendHandlers[id] = handler
	c.handlersMu.Unlock()
	return func() {
		c.handlersMu.Lock()
		delete(c.sendHandlers, id)
		c.handlersMu.Unlock()
	}
}

// RegisterReceiveHandler installs a callback invoked for every completed receive. The returned
// function unregisters the handler when invoked. Passing a nil handler is a no-op.
func (c *Client) RegisterReceiveHandler(handler ReceiveHandler) func() {
	if c == nil || handler == nil {
		return func() {}
	}
	id := c.handlerSeq.Add(1)
	c.handlersMu.Lock()
	if c.receiveHandlers == nil {
		c.receiveHandlers = make(map[uint64]ReceiveHandler)
	}
	c.receiveHandlers[id] = handler
	c.handlersMu.Unlock()
	return func() {
		c.handlersMu.Lock()
		delete(c.receiveHandlers, id)
		c.handlersMu.Unlock()
	}
}

func (c *Client) dispatch() {
	defer c.wg.Done()

	span := c.startDispatcherSpan()
	startFields := []logField{
		logKV("endpoint_type", c.cfg.EndpointType.String()),
	}
	if c.cfg.Provider != "" {
		startFields = append(startFields, logKV("provider", c.cfg.Provider))
	}
	c.logDispatcherEvent("start", startFields...)
	spanAddEvent(span, "start", startFields...)
	c.metricDispatcherStarted(startFields...)

	defer func() {
		err := c.dispatcherError()
		status := "ok"
		fields := []logField{logKV("status", status)}
		if err != nil {
			status = "error"
			fields[0] = logKV("status", status)
			fields = append(fields, logKV("error", err))
			spanRecordError(span, err)
		}
		c.logDispatcherEvent("stop", fields...)
		spanAddEvent(span, "stop", fields...)
		c.metricDispatcherStopped(fields...)
		c.finishDispatcherSpan(span, err)
	}()

	backoff := time.Millisecond
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if event, err := c.cq.ReadContext(); err == nil && event != nil {
			c.handleCompletion(event, nil, span)
			backoff = time.Millisecond
			continue
		} else if err != nil && !errors.Is(err, fi.ErrNoCompletion) {
			dispatchErr := fmt.Errorf("cq read: %w", err)
			c.recordDispatcherFailure(span, "cq_read_error", dispatchErr)
			c.recordDispatcherError(dispatchErr)
		}

		if entry, err := c.cq.ReadError(0); err == nil && entry != nil {
			c.handleCompletion(nil, entry, span)
			backoff = time.Millisecond
			continue
		} else if err != nil && !errors.Is(err, fi.ErrNoCompletion) {
			dispatchErr := fmt.Errorf("cq readerr: %w", err)
			c.recordDispatcherFailure(span, "cq_readerr_error", dispatchErr)
			c.recordDispatcherError(dispatchErr)
		}

		select {
		case <-c.stopCh:
			return
		case <-time.After(backoff):
		}

		if backoff < 10*time.Millisecond {
			backoff *= 2
		}
	}
}

func (c *Client) handleCompletion(event *fi.CompletionEvent, entry *fi.CompletionError, span Span) {
	var (
		ctx *fi.CompletionContext
		err error
	)
	switch {
	case event != nil:
		ctx, err = event.Resolve()
	case entry != nil:
		ctx, err = entry.Resolve()
	default:
		return
	}
	if err != nil {
		// Context already released or unknown; nothing to surface.
		return
	}
	val := ctx.Value()
	op, ok := val.(*operation)
	if !ok || op == nil {
		return
	}
	if op.kind == OperationReceive {
		if meta, ok := op.meta.(*receiveMeta); ok && meta != nil {
			if event != nil {
				meta.source.Store(uint64(event.Source))
			} else if entry != nil {
				meta.source.Store(entry.SrcAddr)
			}
		}
	}
	result := operationResult{length: op.size}
	if entry != nil {
		result.err = OperationError{
			Kind:        op.kind,
			Errno:       entry.Err,
			ProviderErr: entry.ProviderErr,
			Flags:       entry.Flags,
			Length:      entry.Length,
			Data:        entry.Data,
			Tag:         entry.Tag,
		}
	}
	c.logOperationCompletion(op, result, event, entry, span)
	op.complete(result)
}

func (c *Client) emit(op *operation, res operationResult) {
	if c == nil {
		return
	}
	switch op.kind {
	case OperationSend:
		if res.err != nil {
			c.stats.sendErrored.Add(1)
			c.logf("client: send errored: %v", res.err)
		} else {
			c.stats.sendCompleted.Add(1)
			c.logf("client: send completed size=%d", res.length)
		}
		c.handlersMu.RLock()
		handlers := make([]SendHandler, 0, len(c.sendHandlers))
		for _, h := range c.sendHandlers {
			handlers = append(handlers, h)
		}
		c.handlersMu.RUnlock()
		if len(handlers) == 0 {
			return
		}
		completion := SendCompletion{Size: res.length, Err: res.err}
		for _, handler := range handlers {
			h := handler
			go h(completion)
		}
	case OperationReceive:
		if res.err != nil {
			c.stats.recvErrored.Add(1)
			c.logf("client: receive errored: %v", res.err)
		} else {
			c.stats.recvMatched.Add(1)
		}
		meta, _ := op.meta.(*receiveMeta)
		c.handlersMu.RLock()
		handlers := make([]ReceiveHandler, 0, len(c.receiveHandlers))
		for _, h := range c.receiveHandlers {
			handlers = append(handlers, h)
		}
		c.handlersMu.RUnlock()
		if len(handlers) == 0 {
			return
		}
		var basePayload []byte
		if res.length > 0 {
			if meta != nil && meta.buffer != nil && len(meta.buffer) >= res.length {
				basePayload = make([]byte, res.length)
				copy(basePayload, meta.buffer[:res.length])
			} else {
				basePayload = make([]byte, res.length)
			}
		}
		source := fi.Address(0)
		if meta != nil {
			source = fi.Address(meta.source.Load())
		}
		for _, handler := range handlers {
			h := handler
			var payloadCopy []byte
			if basePayload != nil {
				payloadCopy = append([]byte(nil), basePayload...)
			}
			go h(ReceiveCompletion{Payload: payloadCopy, Source: source, Err: res.err})
		}
		if res.err == nil {
			c.logf("client: receive completed size=%d source=%v", res.length, source)
		}
	}
}

func (c *Client) recordDispatcherError(err error) {
	if err == nil {
		return
	}
	c.dispatcherErr.CompareAndSwap(nil, &errorHolder{err: err})
}

func (c *Client) dispatcherError() error {
	if c == nil {
		return nil
	}
	if holder := c.dispatcherErr.Load(); holder != nil {
		return holder.err
	}
	return nil
}

func (c *Client) startDispatcherSpan() Span {
	if c == nil || c.tracer == nil {
		return nil
	}
	attrs := []TraceAttribute{
		{Key: "component", Value: "libfabric-client"},
		{Key: "endpoint_type", Value: c.cfg.EndpointType.String()},
	}
	if c.cfg.Provider != "" {
		attrs = append(attrs, TraceAttribute{Key: "provider", Value: c.cfg.Provider})
	}
	if c.cfg.Node != "" {
		attrs = append(attrs, TraceAttribute{Key: "node", Value: c.cfg.Node})
	}
	if c.cfg.Service != "" {
		attrs = append(attrs, TraceAttribute{Key: "service", Value: c.cfg.Service})
	}
	return c.tracer.StartSpan("libfabric-client-dispatcher", attrs...)
}

func (c *Client) finishDispatcherSpan(span Span, err error) {
	if span == nil {
		return
	}
	span.End(err)
}

func (c *Client) recordDispatcherFailure(span Span, event string, err error) {
	if err == nil {
		return
	}
	fields := []logField{logKV("error", err)}
	c.logDispatcherEvent(event, fields...)
	spanAddEvent(span, event, fields...)
	spanRecordError(span, err)
	c.metricDispatcherCQError(event, err, fields...)
}

func (c *Client) logOperationCompletion(op *operation, res operationResult, event *fi.CompletionEvent, entry *fi.CompletionError, span Span) {
	if c == nil || op == nil {
		return
	}
	status := "ok"
	if res.err != nil {
		status = "error"
	}
	eventName := "completion"
	if status != "ok" {
		eventName = "completion_error"
	}
	fields := []logField{
		logKV("operation", op.kind.String()),
		logKV("status", status),
	}
	if op.size > 0 {
		fields = append(fields, logKV("requested_size", op.size))
	}
	if res.length > 0 {
		fields = append(fields, logKV("length", res.length))
	}
	if event != nil && event.Source != 0 {
		fields = append(fields, logKV("source", event.Source))
	}
	if entry != nil {
		fields = append(fields,
			logKV("errno", entry.Err),
			logKV("provider_err", entry.ProviderErr),
			logKV("flags", fmt.Sprintf("0x%x", entry.Flags)),
			logKV("provider_length", entry.Length),
			logKV("provider_data", entry.Data),
			logKV("provider_tag", entry.Tag),
		)
		if entry.SrcAddr != 0 {
			fields = append(fields, logKV("source", fi.Address(entry.SrcAddr)))
		}
	}
	if res.err != nil {
		fields = append(fields, logKV("error", res.err))
	}
	c.logDispatcherEvent(eventName, fields...)
	spanAddEvent(span, eventName, fields...)
	if res.err != nil {
		spanRecordError(span, res.err)
	}
	switch op.kind {
	case OperationSend:
		if res.err != nil {
			c.metricSendFailed(res.err, fields...)
		} else {
			c.metricSendCompleted(fields...)
		}
	case OperationReceive:
		if res.err != nil {
			c.metricReceiveFailed(res.err, fields...)
		} else {
			c.metricReceiveCompleted(fields...)
		}
	}
}

func spanAddEvent(span Span, name string, fields ...logField) {
	if span == nil {
		return
	}
	span.AddEvent(name, attributesFromFields(fields...)...)
}

func spanRecordError(span Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
}

func attributesFromFields(fields ...logField) []TraceAttribute {
	if len(fields) == 0 {
		return nil
	}
	attrs := make([]TraceAttribute, 0, len(fields))
	for _, field := range fields {
		if field.key == "" {
			continue
		}
		attrs = append(attrs, TraceAttribute{Key: field.key, Value: field.value})
	}
	return attrs
}

func ensureContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func (c *Client) logf(format string, args ...any) {
	if c == nil || c.logger == nil {
		return
	}
	c.logger.Debugf(format, args...)
}
