// Package main demonstrates switching libfabric providers at runtime.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	fi "github.com/rocketbitz/libfabric-go/fi"
)

func main() {
	discovery, err := fi.DiscoverDescriptors()
	if err != nil {
		log.Fatalf("discover descriptors: %v", err)
	}
	defer discovery.Close()

	var rdmDesc *fi.Descriptor
	var msgDesc *fi.Descriptor
	for _, candidate := range discovery.Descriptors() {
		info := candidate.Info()
		if rdmDesc == nil && info.SupportsRDM() {
			desc := candidate
			rdmDesc = &desc
			continue
		}
		if msgDesc == nil && info.SupportsMsg() && candidate.EndpointType() == fi.EndpointTypeMsg {
			desc := candidate
			msgDesc = &desc
		}
	}

	switch {
	case rdmDesc != nil:
		info := rdmDesc.Info()
		fmt.Printf("using RDM provider: %s\n", fi.FormatInfo(info))
		if err := runRDMExample(*rdmDesc); err != nil {
			log.Fatalf("rdm example failed: %v", err)
		}
	case msgDesc != nil:
		info := msgDesc.Info()
		fmt.Printf("using MSG provider: %s\n", fi.FormatInfo(info))
		if err := runMSGExample(*msgDesc); err != nil {
			log.Fatalf("msg example failed: %v", err)
		}
	default:
		log.Fatalf("no MSG or RDM capable providers available")
	}
}

func runMSGExample(desc fi.Descriptor) error {
	fabric, err := desc.OpenFabric()
	if err != nil {
		return fmt.Errorf("open fabric: %w", err)
	}
	defer func() { _ = fabric.Close() }()

	domain, err := desc.OpenDomain(fabric)
	if err != nil {
		return fmt.Errorf("open domain: %w", err)
	}
	defer func() { _ = domain.Close() }()

	cq, err := domain.OpenCompletionQueue(nil)
	if err != nil {
		return fmt.Errorf("open completion queue: %w", err)
	}
	defer func() { _ = cq.Close() }()

	ep, err := desc.OpenEndpoint(domain)
	if err != nil {
		return fmt.Errorf("open endpoint: %w", err)
	}
	defer func() { _ = ep.Close() }()

	if err := ep.BindCompletionQueue(cq, fi.BindSend|fi.BindRecv); err != nil {
		return fmt.Errorf("bind completion queue: %w", err)
	}
	if err := ep.Enable(); err != nil {
		if errors.Is(err, fi.ErrCapabilityUnsupported) {
			return fmt.Errorf("endpoint enable unsupported: %w", err)
		}
		return fmt.Errorf("enable endpoint: %w", err)
	}

	message := []byte("hello from MSG endpoint")
	if err := ep.SendSync(message, fi.AddressUnspecified, cq, time.Second); err != nil {
		return fmt.Errorf("send sync: %w", err)
	}

	recvBuf := make([]byte, len(message))
	if err := ep.RecvSync(recvBuf, cq, time.Second); err != nil {
		return fmt.Errorf("recv sync: %w", err)
	}

	fmt.Printf("MSG round-trip payload: %q\n", string(recvBuf))
	return nil
}

func runRDMExample(desc fi.Descriptor) error {
	fabric, err := desc.OpenFabric()
	if err != nil {
		return fmt.Errorf("open fabric: %w", err)
	}
	defer func() { _ = fabric.Close() }()

	domain, err := desc.OpenDomain(fabric)
	if err != nil {
		return fmt.Errorf("open domain: %w", err)
	}
	defer func() { _ = domain.Close() }()

	av, err := domain.OpenAddressVector(&fi.AddressVectorAttr{Type: fi.AVTypeMap})
	if err != nil {
		return fmt.Errorf("open address vector: %w", err)
	}
	defer func() { _ = av.Close() }()

	type endpointResources struct {
		ep *fi.Endpoint
		cq *fi.CompletionQueue
	}

	newEndpoint := func() (*endpointResources, error) {
		cq, err := domain.OpenCompletionQueue(nil)
		if err != nil {
			return nil, fmt.Errorf("open completion queue: %w", err)
		}

		ep, err := desc.OpenEndpoint(domain)
		if err != nil {
			_ = cq.Close()
			return nil, fmt.Errorf("open endpoint: %w", err)
		}

		if err := ep.BindCompletionQueue(cq, fi.BindSend|fi.BindRecv); err != nil {
			_ = ep.Close()
			_ = cq.Close()
			return nil, fmt.Errorf("bind completion queue: %w", err)
		}
		if err := ep.BindAddressVector(av, 0); err != nil {
			_ = ep.Close()
			_ = cq.Close()
			return nil, fmt.Errorf("bind address vector: %w", err)
		}
		if err := ep.Enable(); err != nil {
			_ = ep.Close()
			_ = cq.Close()
			if errors.Is(err, fi.ErrCapabilityUnsupported) {
				return nil, fmt.Errorf("endpoint enable unsupported: %w", err)
			}
			return nil, fmt.Errorf("enable endpoint: %w", err)
		}
		return &endpointResources{ep: ep, cq: cq}, nil
	}

	a, err := newEndpoint()
	if err != nil {
		return err
	}
	defer func() {
		_ = a.ep.Close()
		_ = a.cq.Close()
	}()

	b, err := newEndpoint()
	if err != nil {
		return err
	}
	defer func() {
		_ = b.ep.Close()
		_ = b.cq.Close()
	}()

	addrA, err := a.ep.RegisterAddress(av, 0)
	if err != nil {
		return fmt.Errorf("register endpoint A address: %w", err)
	}
	addrB, err := b.ep.RegisterAddress(av, 0)
	if err != nil {
		return fmt.Errorf("register endpoint B address: %w", err)
	}

	fmt.Printf("RDM endpoint addresses: local=%#x peer=%#x\n", addrA, addrB)

	message := []byte("hello from RDM endpoint")
	recvBuf := make([]byte, len(message))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	recvErr := make(chan error, 1)
	go func() {
		recvErr <- b.ep.RecvSyncContext(ctx, recvBuf, b.cq, time.Second)
	}()

	if err := a.ep.SendSyncContext(ctx, message, addrB, a.cq, time.Second); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("rdm send sync deadline: %w", err)
		}
		return fmt.Errorf("rdm send sync: %w", err)
	}

	if err := <-recvErr; err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("rdm recv sync deadline: %w", err)
		}
		return fmt.Errorf("rdm recv sync: %w", err)
	}

	fmt.Printf("RDM round-trip payload: %q\n", string(recvBuf))
	return nil
}
