package fi

import "testing"

func TestDiscover(t *testing.T) {
	infos, err := Discover()
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(infos) == 0 {
		t.Fatalf("expected at least one provider from Discover")
	}
	for _, info := range infos {
		if info.Provider == "" {
			t.Fatalf("provider name should not be empty")
		}
		_ = info.MRModeFlags()
		_ = info.RequiresMRMode(MRModeLocal)
	}
}

func TestDiscoverWithEndpointType(t *testing.T) {
	result, err := DiscoverDescriptors(WithProvider("sockets"), WithEndpointType(EndpointTypeMsg))
	if err != nil {
		t.Fatalf("DiscoverDescriptors with hints failed: %v", err)
	}
	defer result.Close()
	descs := result.Descriptors()
	if len(descs) == 0 {
		t.Skip("sockets provider unavailable in this environment")
	}
	for _, desc := range descs {
		info := desc.Info()
		if info.Provider != "sockets" {
			t.Fatalf("expected sockets provider, got %s", info.Provider)
		}
		if info.Endpoint != EndpointTypeMsg {
			t.Fatalf("expected msg endpoint, got %v", info.Endpoint)
		}
		_ = desc.MRModeFlags()
		_ = desc.MRKeySize()
		_ = desc.MRIovLimit()
	}
}

func TestDiscoveryOpenFabricDomain(t *testing.T) {
	result, err := DiscoverDescriptors()
	if err != nil {
		t.Fatalf("DiscoverDescriptors failed: %v", err)
	}
	defer result.Close()
	descs := result.Descriptors()
	if len(descs) == 0 {
		t.Fatalf("expected at least one descriptor")
	}

	var fabric *Fabric
	var domain *Domain
	for _, desc := range descs {
		fab, ferr := desc.OpenFabric()
		if ferr != nil {
			t.Logf("provider %s fabric open failed: %v", desc.Provider(), ferr)
			continue
		}
		dom, derr := desc.OpenDomain(fab)
		if derr != nil {
			t.Logf("provider %s domain open failed: %v", desc.Provider(), derr)
			fab.Close()
			continue
		}
		fabric = fab
		domain = dom
		break
	}

	if fabric == nil {
		t.Skip("no provider allowing fabric/domain open in this environment")
	}
	defer fabric.Close()
	defer domain.Close()
}
