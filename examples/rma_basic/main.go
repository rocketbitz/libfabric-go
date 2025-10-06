// Package main demonstrates basic libfabric RMA usage with libfabric-go.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	fi "github.com/rocketbitz/libfabric-go/fi"
)

func main() {
	log.SetFlags(0)

	provider := os.Getenv("LIBFABRIC_EXAMPLE_PROVIDER")
	if provider == "" {
		provider = "sockets"
		fmt.Println("defaulting to provider sockets; override with LIBFABRIC_EXAMPLE_PROVIDER")
	} else {
		fmt.Printf("requesting provider %s\n", provider)
	}

	baseOpts := []fi.DiscoverOption{
		fi.WithProvider(provider),
		fi.WithCaps(fi.CapRMA),
	}

	discovery, descriptors, err := discoverRMADescriptors(baseOpts)
	if err != nil {
		log.Fatalf("discover descriptors: %v", err)
	}
	if discovery == nil || len(descriptors) == 0 {
		log.Fatalf("no RMA-capable descriptors found for provider %s", provider)
	}
	defer discovery.Close()

	ordered := orderDescriptors(descriptors)
	var lastErr error
	for _, desc := range ordered {
		info := desc.Info()
		fmt.Printf("attempting provider %s fabric %s domain %s\n", info.Provider, info.Fabric, info.Domain)
		if !desc.SupportsRemoteRead() || !desc.SupportsRemoteWrite() {
			log.Printf("descriptor skipped: provider lacks remote read/write capability")
			lastErr = fmt.Errorf("descriptor missing remote capability")
			continue
		}
		if err := runRMAExample(desc); err != nil {
			log.Printf("descriptor skipped: %v", err)
			lastErr = err
			continue
		}
		return
	}

	if lastErr != nil {
		log.Fatalf("no descriptor succeeded: %v (ensure the selected provider exposes RDM/RMA; sockets may require FI_SOCKETS_IFACE=lo to force loopback)", lastErr)
	}
	log.Fatalf("no descriptor succeeded (provider did not expose usable descriptors)")
}

func discoverRMADescriptors(base []fi.DiscoverOption) (*fi.Discovery, []fi.Descriptor, error) {
	withRDM := append([]fi.DiscoverOption{}, base...)
	withRDM = append(withRDM, fi.WithEndpointType(fi.EndpointTypeRDM))

	discovery, err := fi.DiscoverDescriptors(withRDM...)
	if err != nil {
		return nil, nil, err
	}
	descs := discovery.Descriptors()
	if len(descs) > 0 {
		return discovery, descs, nil
	}
	discovery.Close()

	discovery, err = fi.DiscoverDescriptors(base...)
	if err != nil {
		return nil, nil, err
	}
	descs = discovery.Descriptors()
	if len(descs) > 0 {
		return discovery, descs, nil
	}
	discovery.Close()
	return nil, nil, nil
}

func orderDescriptors(descs []fi.Descriptor) []fi.Descriptor {
	var loopbacks []fi.Descriptor
	var others []fi.Descriptor
	for _, d := range descs {
		fabric := strings.ToLower(d.Info().Fabric)
		if fabric == "" || fabric == "lo" || fabric == "lo0" || strings.HasPrefix(fabric, "loopback") {
			loopbacks = append(loopbacks, d)
		} else {
			others = append(others, d)
		}
	}
	return append(loopbacks, others...)
}

func runRMAExample(desc fi.Descriptor) error {
	if desc.EndpointType() != fi.EndpointTypeRDM {
		return fmt.Errorf("descriptor endpoint type %d lacks RDM support", desc.EndpointType())
	}

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

	av, err := domain.OpenAddressVector(nil)
	if err != nil {
		return fmt.Errorf("open address vector: %w", err)
	}
	defer func() { _ = av.Close() }()

	initiator, initiatorCQ, err := openEndpoint(domain, desc, av, "initiator")
	if err != nil {
		return err
	}
	defer func() { _ = initiator.Close() }()
	defer func() { _ = initiatorCQ.Close() }()

	target, targetCQ, err := openEndpoint(domain, desc, av, "target")
	if err != nil {
		return err
	}
	defer func() { _ = target.Close() }()
	defer func() { _ = targetCQ.Close() }()

	addrTarget, err := target.RegisterAddress(av, 0)
	if err != nil {
		return fmt.Errorf("register target address: %w", err)
	}

	writePayload := []byte("rma write payload")
	localMR, err := domain.RegisterMemory(append([]byte(nil), writePayload...), fi.MRAccessLocal)
	if err != nil {
		return fmt.Errorf("register local memory: %w", err)
	}
	defer func() { _ = localMR.Close() }()

	remoteMR, err := domain.RegisterMemory(make([]byte, len(writePayload)), fi.MRAccessLocal|fi.MRAccessRemoteRead|fi.MRAccessRemoteWrite)
	if err != nil {
		return fmt.Errorf("register remote memory: %w", err)
	}
	defer func() { _ = remoteMR.Close() }()

	fmt.Println("posting RMA write...")
	if err := initiator.WriteSync(&fi.RMARequest{
		Region:  localMR,
		Address: addrTarget,
		Key:     remoteMR.Key(),
	}, initiatorCQ, time.Second); err != nil {
		return fmt.Errorf("write sync: %w", err)
	}
	fmt.Printf("remote buffer now contains %q\n", string(remoteMR.Bytes()))

	readPayload := []byte("updated remote payload")
	copy(remoteMR.Bytes(), readPayload)

	readMR, err := domain.RegisterMemory(make([]byte, len(readPayload)), fi.MRAccessLocal)
	if err != nil {
		return fmt.Errorf("register read staging region: %w", err)
	}
	defer func() { _ = readMR.Close() }()
	readBacking := make([]byte, len(readPayload))

	fmt.Println("posting RMA read...")
	if err := initiator.ReadSync(&fi.RMARequest{
		Region:  readMR,
		Buffer:  readBacking,
		Address: addrTarget,
		Key:     remoteMR.Key(),
	}, initiatorCQ, time.Second); err != nil {
		return fmt.Errorf("read sync: %w", err)
	}

	fmt.Printf("read returned %q\n", string(readBacking))
	return nil
}

func openEndpoint(domain *fi.Domain, desc fi.Descriptor, av *fi.AddressVector, label string) (*fi.Endpoint, *fi.CompletionQueue, error) {
	cq, err := domain.OpenCompletionQueue(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: open completion queue: %w", label, err)
	}

	ep, err := desc.OpenEndpoint(domain)
	if err != nil {
		_ = cq.Close()
		return nil, nil, fmt.Errorf("%s: open endpoint: %w", label, err)
	}

	if err := ep.BindCompletionQueue(cq, fi.BindSend|fi.BindRecv); err != nil {
		_ = ep.Close()
		_ = cq.Close()
		return nil, nil, fmt.Errorf("%s: bind completion queue: %w", label, err)
	}

	if err := ep.BindAddressVector(av, 0); err != nil {
		_ = ep.Close()
		_ = cq.Close()
		return nil, nil, fmt.Errorf("%s: bind address vector: %w", label, err)
	}

	if err := ep.Enable(); err != nil {
		_ = ep.Close()
		_ = cq.Close()
		return nil, nil, fmt.Errorf("%s: enable endpoint: %w", label, err)
	}

	return ep, cq, nil
}
