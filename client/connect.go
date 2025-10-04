package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	fi "github.com/rocketbitz/libfabric-go/fi"
)

// Connect dials a MSG endpoint exposed by a listener using the supplied configuration.
func Connect(cfg Config) (*Client, error) {
	if cfg.Service == "" {
		return nil, errors.New("libfabric client connect: service required")
	}
	if cfg.EndpointType == 0 {
		cfg.EndpointType = fi.EndpointTypeMsg
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}

	opts := []fi.DiscoverOption{fi.WithEndpointType(cfg.EndpointType)}
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
		return nil, fmt.Errorf("libfabric client connect: no descriptors found for provider %s", cfg.Provider)
	}
	desc := descriptors[0]

	fabric, err := desc.OpenFabric()
	if err != nil {
		return nil, fmt.Errorf("open fabric: %w", err)
	}

	domain, err := desc.OpenDomain(fabric)
	if err != nil {
		_ = fabric.Close()
		return nil, fmt.Errorf("open domain: %w", err)
	}

	cq, err := domain.OpenCompletionQueue(nil)
	if err != nil {
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("open completion queue: %w", err)
	}

	eq, err := fabric.OpenEventQueue(nil)
	if err != nil {
		_ = cq.Close()
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("open event queue: %w", err)
	}

	endpoint, err := desc.OpenEndpoint(domain)
	if err != nil {
		_ = eq.Close()
		_ = cq.Close()
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("open endpoint: %w", err)
	}

	if err := endpoint.BindCompletionQueue(cq, fi.BindSend|fi.BindRecv); err != nil {
		_ = endpoint.Close()
		_ = eq.Close()
		_ = cq.Close()
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("bind completion queue: %w", err)
	}

	if err := endpoint.BindEventQueue(eq, 0); err != nil {
		_ = endpoint.Close()
		_ = eq.Close()
		_ = cq.Close()
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("bind event queue: %w", err)
	}

	if err := endpoint.Enable(); err != nil {
		_ = endpoint.Close()
		_ = eq.Close()
		_ = cq.Close()
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("enable endpoint: %w", err)
	}

	if err := endpoint.Connect(nil); err != nil {
		_ = endpoint.Close()
		_ = eq.Close()
		_ = cq.Close()
		_ = domain.Close()
		_ = fabric.Close()
		return nil, fmt.Errorf("connect: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if err := waitForConnected(ctx, eq); err != nil {
		endpoint.Close()
		eq.Close()
		cq.Close()
		domain.Close()
		fabric.Close()
		return nil, err
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
		eq:               eq,
		stopCh:           make(chan struct{}),
		logger:           cfg.Logger,
		structuredLogger: structured,
		tracer:           cfg.Tracer,
		metrics:          metrics,
		ownFabric:        true,
		ownDomain:        true,
		ownEndpoint:      true,
		ownCompletion:    true,
		ownEventQueue:    true,
	}

	client.requiresMR = domain.RequiresMRMode(fi.MRModeLocal)

	client.wg.Add(1)
	go client.dispatch()

	return client, nil
}
