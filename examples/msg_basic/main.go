// Package main demonstrates basic message send/receive operations with libfabric-go.
package main

import (
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

	var desc fi.Descriptor
	found := false
	for _, candidate := range discovery.Descriptors() {
		info := candidate.Info()
		if info.SupportsMsg() && candidate.EndpointType() == fi.EndpointTypeMsg {
			desc = candidate
			found = true
			break
		}
	}
	if !found {
		log.Fatalf("no provider found that supports MSG endpoints")
	}

	fabric, err := desc.OpenFabric()
	if err != nil {
		log.Fatalf("open fabric: %v", err)
	}
	defer func() {
		_ = fabric.Close()
	}()

	domain, err := desc.OpenDomain(fabric)
	if err != nil {
		log.Fatalf("open domain: %v", err)
	}
	defer func() {
		_ = domain.Close()
	}()

	cq, err := domain.OpenCompletionQueue(nil)
	if err != nil {
		log.Fatalf("open completion queue: %v", err)
	}
	defer func() {
		_ = cq.Close()
	}()

	ep, err := desc.OpenEndpoint(domain)
	if err != nil {
		log.Fatalf("open endpoint: %v", err)
	}
	defer func() {
		_ = ep.Close()
	}()

	if err := ep.BindCompletionQueue(cq, fi.BindSend|fi.BindRecv); err != nil {
		log.Fatalf("bind completion queue: %v", err)
	}
	if err := ep.Enable(); err != nil {
		if errors.Is(err, fi.ErrCapabilityUnsupported) {
			log.Fatalf("endpoint enable unsupported: %v", err)
		}
		log.Fatalf("enable endpoint: %v", err)
	}

	message := []byte("hello, libfabric")
	if err := ep.SendSync(message, fi.AddressUnspecified, cq, time.Second); err != nil {
		log.Fatalf("send sync: %v", err)
	}

	recvBuf := make([]byte, len(message))
	if err := ep.RecvSync(recvBuf, cq, time.Second); err != nil {
		log.Fatalf("recv sync: %v", err)
	}

	fmt.Printf("received: %s\n", string(recvBuf))
}
