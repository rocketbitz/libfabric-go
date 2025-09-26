package fi

import (
	"errors"
	"strings"
	"testing"
)

func setupSocketsResourcesWithType(t *testing.T, epType EndpointType) (Descriptor, *Fabric, *Domain) {
	t.Helper()
	options := []DiscoverOption{WithProvider("sockets")}
	if epType != 0 {
		options = append(options, WithEndpointType(epType))
	}
	discovery, err := DiscoverDescriptors(options...)
	if err != nil {
		t.Fatalf("DiscoverDescriptors failed: %v", err)
	}
	t.Cleanup(discovery.Close)

	descs := discovery.Descriptors()
	if len(descs) == 0 {
		t.Skip("sockets provider not available on this system")
	}

	desc := descs[0]
	for _, candidate := range descs {
		info := candidate.Info()
		fabric := strings.ToLower(info.Fabric)
		if fabric == "lo" || fabric == "lo0" || strings.HasPrefix(fabric, "loopback") {
			desc = candidate
			break
		}
	}

	fabric, err := desc.OpenFabric()
	if err != nil {
		t.Skipf("unable to open fabric for sockets provider: %v", err)
	}
	t.Cleanup(func() {
		_ = fabric.Close()
	})

	domain, err := desc.OpenDomain(fabric)
	if err != nil {
		t.Skipf("unable to open domain for sockets provider: %v", err)
	}
	t.Cleanup(func() {
		_ = domain.Close()
	})

	return desc, fabric, domain
}

func setupSocketsResources(t *testing.T) (Descriptor, *Fabric, *Domain) {
	return setupSocketsResourcesWithType(t, EndpointTypeMsg)
}

func TestEndpointLifecycleSockets(t *testing.T) {
	desc, fabric, domain := setupSocketsResources(t)

	cq, err := domain.OpenCompletionQueue(nil)
	if err != nil {
		t.Skipf("unable to open completion queue: %v", err)
	}
	t.Cleanup(func() {
		_ = cq.Close()
	})

	eq, err := fabric.OpenEventQueue(nil)
	if err != nil {
		t.Skipf("unable to open event queue: %v", err)
	}
	t.Cleanup(func() {
		_ = eq.Close()
	})

	ep, err := desc.OpenEndpoint(domain)
	if err != nil {
		t.Skipf("unable to open endpoint: %v", err)
	}
	t.Cleanup(func() {
		_ = ep.Close()
	})

	if err := ep.BindCompletionQueue(cq, BindSend|BindRecv); err != nil {
		t.Skipf("bind completion queue failed: %v", err)
	}

	if err := ep.BindEventQueue(eq, BindSend|BindRecv); err != nil {
		t.Skipf("bind event queue failed: %v", err)
	}

	if err := ep.Enable(); err != nil {
		t.Skipf("endpoint enable failed: %v", err)
	}
}

func TestCompletionQueueReadEmpty(t *testing.T) {
	_, _, domain := setupSocketsResources(t)
	cq, err := domain.OpenCompletionQueue(nil)
	if err != nil {
		t.Skipf("unable to open completion queue: %v", err)
	}
	t.Cleanup(func() {
		_ = cq.Close()
	})

	if _, err := cq.ReadContext(); !errors.Is(err, ErrNoCompletion) {
		t.Fatalf("expected ErrNoCompletion, got %v", err)
	}

	if _, err := cq.ReadError(0); !errors.Is(err, ErrNoCompletion) {
		t.Fatalf("expected ErrNoCompletion from ReadError, got %v", err)
	}
}

func TestEventQueueReadEmpty(t *testing.T) {
	_, fabric, _ := setupSocketsResources(t)
	eq, err := fabric.OpenEventQueue(nil)
	if err != nil {
		t.Skipf("unable to open event queue: %v", err)
	}
	t.Cleanup(func() {
		_ = eq.Close()
	})

	if _, err := eq.Read(0); !errors.Is(err, ErrNoEvent) {
		t.Fatalf("expected ErrNoEvent, got %v", err)
	}

	if _, err := eq.ReadError(0); !errors.Is(err, ErrNoEvent) {
		t.Fatalf("expected ErrNoEvent from ReadError, got %v", err)
	}
}
