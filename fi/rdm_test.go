package fi

import "testing"

// TestRDMDiscovery verifies we can locate RDM-capable providers (best effort).
func TestRDMDiscovery(t *testing.T) {
	discovery, err := DiscoverDescriptors()
	if err != nil {
		t.Fatalf("DiscoverDescriptors failed: %v", err)
	}
	defer discovery.Close()

	if !discovery.SupportsEndpointType(EndpointTypeRDM) {
		t.Skip("no RDM-capable provider available")
	}

	for _, desc := range discovery.Descriptors() {
		if !desc.SupportsEndpointType(EndpointTypeRDM) {
			continue
		}
		info := desc.Info()
		t.Logf("found RDM provider=%s fabric=%s inject=%d", info.Provider, info.Fabric, info.InjectSize)
		return
	}
}
