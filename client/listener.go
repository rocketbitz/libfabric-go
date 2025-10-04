package client

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	fi "github.com/rocketbitz/libfabric-go/fi"
)

// ListenerConfig controls MSG listener setup.
type ListenerConfig struct {
	Provider         string
	Node             string
	Service          string
	Logger           Logger
	StructuredLogger StructuredLogger
	Tracer           Tracer
	Metrics          MetricHook
}

// Listener accepts MSG connections and converts them into high-level Clients.
type Listener struct {
	cfg        ListenerConfig
	fabric     *fi.Fabric
	domain     *fi.Domain
	pep        *fi.PassiveEndpoint
	eq         *fi.EventQueue
	closed     atomic.Bool
	logger     Logger
	structured StructuredLogger
	tracer     Tracer
	metrics    MetricHook
}

// Listen prepares a MSG listener using the provided configuration.
func Listen(cfg ListenerConfig) (*Listener, error) {
	if cfg.Service == "" {
		return nil, errors.New("libfabric client listener: service required")
	}

	opts := []fi.DiscoverOption{
		fi.WithEndpointType(fi.EndpointTypeMsg),
		fi.WithFlags(fi.FlagSource),
	}
	if cfg.Provider != "" {
		opts = append(opts, fi.WithProvider(cfg.Provider))
	}
	if cfg.Node != "" {
		opts = append(opts, fi.WithNode(cfg.Node))
	}
	opts = append(opts, fi.WithService(cfg.Service))

	discovery, err := fi.DiscoverDescriptors(opts...)
	if err != nil {
		return nil, fmt.Errorf("discover descriptors: %w", err)
	}
	defer discovery.Close()

	descriptors := discovery.Descriptors()
	if len(descriptors) == 0 {
		return nil, fmt.Errorf("libfabric client listener: no descriptors found for provider %s", cfg.Provider)
	}
	desc := descriptors[0]

	fabric, err := desc.OpenFabric()
	if err != nil {
		return nil, fmt.Errorf("open fabric: %w", err)
	}

	domain, err := desc.OpenDomain(fabric)
	if err != nil {
		fabric.Close()
		return nil, fmt.Errorf("open domain: %w", err)
	}

	eq, err := fabric.OpenEventQueue(nil)
	if err != nil {
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("open event queue: %w", err)
	}

	pep, err := desc.OpenPassiveEndpoint(fabric)
	if err != nil {
		_ = eq.Close()
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("open passive endpoint: %w", err)
	}

	if err := pep.BindEventQueue(eq, 0); err != nil {
		_ = pep.Close()
		_ = eq.Close()
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("bind event queue: %w", err)
	}

	if err := pep.Listen(); err != nil {
		_ = pep.Close()
		_ = eq.Close()
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("listen passive endpoint: %w", err)
	}

	structured := cfg.StructuredLogger
	if structured == nil {
		if logger, ok := cfg.Logger.(StructuredLogger); ok {
			structured = logger
		}
	}

	return &Listener{
		cfg:        cfg,
		fabric:     fabric,
		domain:     domain,
		pep:        pep,
		eq:         eq,
		logger:     cfg.Logger,
		structured: structured,
		tracer:     cfg.Tracer,
		metrics:    cfg.Metrics,
	}, nil
}

// Close releases listener resources.
func (l *Listener) Close() error {
	if l == nil {
		return nil
	}
	if !l.closed.CompareAndSwap(false, true) {
		return nil
	}
	if l.pep != nil {
		_ = l.pep.Close()
	}
	if l.eq != nil {
		_ = l.eq.Close()
	}
	if l.domain != nil {
		_ = l.domain.Close()
	}
	if l.fabric != nil {
		_ = l.fabric.Close()
	}
	return nil
}

// Addr returns the bound provider address for the listener.
func (l *Listener) Addr() ([]byte, error) {
	if l == nil || l.pep == nil {
		return nil, errors.New("libfabric client listener: closed")
	}
	return l.pep.Name()
}

// Accept waits for the next MSG connection request and returns a fully initialised Client.
func (l *Listener) Accept(ctx context.Context) (*Client, error) {
	if l == nil || l.pep == nil {
		return nil, errors.New("libfabric client listener: closed")
	}
	for {
		if l.closed.Load() {
			return nil, errors.New("libfabric client listener: closed")
		}
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		evt, err := l.eq.ReadCM(100 * time.Millisecond)
		if err != nil {
			if errors.Is(err, fi.ErrNoEvent) {
				continue
			}
			return nil, err
		}
		if evt == nil {
			continue
		}
		l.logf("listener: received CM event %v", evt.Type())
		switch evt.Type() {
		case fi.ConnectionEventConnReq:
			client, acceptErr := l.handleConnReq(ctx, evt)
			if acceptErr != nil {
				return nil, acceptErr
			}
			return client, nil
		default:
			evt.Free()
		}
	}
}

func (l *Listener) handleConnReq(ctx context.Context, evt *fi.ConnectionEvent) (*Client, error) {
	defer evt.Free()
	endpoint, err := evt.OpenEndpoint(l.domain)
	if err != nil {
		return nil, fmt.Errorf("open endpoint: %w", err)
	}

	cq, err := l.domain.OpenCompletionQueue(nil)
	if err != nil {
		_ = endpoint.Close()
		return nil, fmt.Errorf("open completion queue: %w", err)
	}

	clientEQ, err := l.fabric.OpenEventQueue(nil)
	if err != nil {
		_ = cq.Close()
		_ = endpoint.Close()
		return nil, fmt.Errorf("open event queue: %w", err)
	}

	if err := endpoint.BindCompletionQueue(cq, fi.BindSend|fi.BindRecv); err != nil {
		_ = clientEQ.Close()
		_ = cq.Close()
		_ = endpoint.Close()
		return nil, fmt.Errorf("bind completion queue: %w", err)
	}
	if err := endpoint.BindEventQueue(clientEQ, 0); err != nil {
		_ = clientEQ.Close()
		_ = cq.Close()
		_ = endpoint.Close()
		return nil, fmt.Errorf("bind event queue: %w", err)
	}
	if err := endpoint.Enable(); err != nil {
		_ = clientEQ.Close()
		_ = cq.Close()
		_ = endpoint.Close()
		return nil, fmt.Errorf("enable endpoint: %w", err)
	}

	if err := endpoint.Accept(evt.Data()); err != nil {
		clientEQ.Close()
		cq.Close()
		endpoint.Close()
		return nil, fmt.Errorf("accept: %w", err)
	}

	if err := waitForConnected(ctx, clientEQ); err != nil {
		clientEQ.Close()
		cq.Close()
		endpoint.Close()
		return nil, err
	}

	structured := l.structured
	if structured == nil {
		if logger, ok := l.logger.(StructuredLogger); ok {
			structured = logger
		}
	}
	metrics := l.metrics
	cfg := Config{Provider: l.cfg.Provider, EndpointType: fi.EndpointTypeMsg, Logger: l.logger, StructuredLogger: structured, Tracer: l.tracer, Metrics: metrics}
	client := &Client{
		cfg:              cfg,
		fabric:           l.fabric,
		domain:           l.domain,
		endpoint:         endpoint,
		cq:               cq,
		eq:               clientEQ,
		stopCh:           make(chan struct{}),
		logger:           l.logger,
		structuredLogger: structured,
		tracer:           l.tracer,
		metrics:          metrics,
		ownFabric:        false,
		ownDomain:        false,
		ownEndpoint:      true,
		ownCompletion:    true,
		ownEventQueue:    true,
	}

	client.requiresMR = l.domain.RequiresMRMode(fi.MRModeLocal)
	if client.logger != nil {
		client.logger.Debugf("client: MSG connection accepted")
	}

	client.wg.Add(1)
	go client.dispatch()

	return client, nil
}

func waitForConnected(ctx context.Context, eq *fi.EventQueue) error {
	deadline := time.Now().Add(5 * time.Second)
	if ctx != nil {
		if d, ok := ctx.Deadline(); ok {
			deadline = d
		}
	}
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		timeout := time.Until(deadline)
		if timeout <= 0 {
			return errors.New("libfabric client: connect timeout exceeded")
		}
		evt, err := eq.ReadCM(timeout)
		if err != nil {
			if errors.Is(err, fi.ErrNoEvent) {
				continue
			}
			return err
		}
		if evt == nil {
			continue
		}
		typ := evt.Type()
		evt.Free()
		if typ == fi.ConnectionEventConnected {
			return nil
		}
		if typ == fi.ConnectionEventShutdown {
			return errors.New("libfabric client: connection closed during handshake")
		}
	}
}

func (l *Listener) logf(format string, args ...any) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Debugf(format, args...)
}
