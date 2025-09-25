//go:build cgo

package capi

import "testing"

func TestGetInfoReturnsProviders(t *testing.T) {
	info, err := GetInfo(BuildVersion(), "", "", 0, nil)
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}
	defer info.Free()

	entries := info.Entries()
	if len(entries) == 0 {
		t.Fatalf("expected at least one fi_info entry")
	}

	for _, entry := range entries {
		if entry.ProviderName() == "" {
			t.Fatalf("provider name should not be empty")
		}
		if entry.EndpointType() == EndpointTypeUnspec {
			t.Logf("provider %q reports unspecified endpoint type", entry.ProviderName())
		}
	}
}
