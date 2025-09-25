package fi

import "testing"

func TestAddressVectorInsertServiceSockets(t *testing.T) {
	discovery, err := DiscoverDescriptors(WithProvider("sockets"), WithEndpointType(EndpointTypeMsg))
	if err != nil {
		t.Fatalf("DiscoverDescriptors failed: %v", err)
	}
	defer discovery.Close()

	descs := discovery.Descriptors()
	if len(descs) == 0 {
		t.Skip("sockets provider not available on this system")
	}

	desc := descs[0]

	fabric, err := desc.OpenFabric()
	if err != nil {
		t.Skipf("unable to open fabric: %v", err)
	}
	defer fabric.Close()

	domain, err := desc.OpenDomain(fabric)
	if err != nil {
		t.Skipf("unable to open domain: %v", err)
	}
	defer domain.Close()

	av, err := domain.OpenAddressVector(&AddressVectorAttr{Type: AVTypeMap})
	if err != nil {
		t.Skipf("unable to open address vector: %v", err)
	}
	defer av.Close()

	addr, err := av.InsertService("127.0.0.1", "8080", 0)
	if err != nil {
		t.Skipf("insert service failed: %v", err)
	}

	if err := av.Remove([]Address{addr}, 0); err != nil {
		t.Fatalf("remove address failed: %v", err)
	}
}
