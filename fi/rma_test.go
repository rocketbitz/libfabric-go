package fi

import (
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestRMAReadWriteProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping RMA test in short mode")
	}

	providers, configured := parseRMAProviders()
	if !configured {
		t.Skip("RMA providers not configured; set LIBFABRIC_TEST_RMA_PROVIDERS")
	}

	hints := parseRMAHints()
	if len(providers) == 0 {
		t.Skip("no RMA providers enabled via LIBFABRIC_TEST_RMA_PROVIDERS")
	}

	keys := make([]string, 0, len(providers))
	for provider := range providers {
		keys = append(keys, provider)
	}
	sort.Strings(keys)

	var ran bool
	for _, provider := range keys {
		prov := provider
		hint := hints[prov]
		t.Run(prov, func(t *testing.T) {
			ran = true
			runRMAReadWrite(t, prov, hint)
		})
	}

	if !ran {
		t.Skip("configured RMA providers not discovered; check LIBFABRIC_TEST_RMA_PROVIDERS")
	}
}

type rmaProviderHints struct {
	EndpointType EndpointType
}

func runRMAReadWrite(t *testing.T, provider string, hint *rmaProviderHints) {
	options := []DiscoverOption{WithProvider(provider)}
	epType := EndpointTypeRDM
	if hint != nil && hint.EndpointType != 0 {
		epType = hint.EndpointType
	}
	options = append(options, WithEndpointType(epType))

	discovery, err := DiscoverDescriptors(options...)
	if err != nil {
		t.Skipf("discover descriptors failed for provider %s: %v", provider, err)
		return
	}
	defer discovery.Close()

	descs := discovery.Descriptors()
	if len(descs) == 0 {
		t.Skipf("provider %s did not expose endpoint type %s", provider, epType)
		return
	}

	chosen := selectDescriptor(descs)
	info := chosen.Info()

	if !chosen.SupportsRemoteRead() || !chosen.SupportsRemoteWrite() {
		t.Skipf("descriptor %s/%s lacks remote read/write capability", info.Provider, info.Fabric)
		return
	}

	fabric, err := chosen.OpenFabric()
	if err != nil {
		t.Skipf("open fabric failed for %s/%s: %v", info.Provider, info.Fabric, err)
		return
	}
	t.Cleanup(func() { _ = fabric.Close() })

	domain, err := chosen.OpenDomain(fabric)
	if err != nil {
		t.Skipf("open domain failed for %s/%s: %v", info.Provider, info.Domain, err)
		return
	}
	t.Cleanup(func() { _ = domain.Close() })

	av, err := domain.OpenAddressVector(&AddressVectorAttr{Type: AVTypeMap})
	if err != nil {
		t.Skipf("open address vector failed: %v", err)
		return
	}
	t.Cleanup(func() { _ = av.Close() })

	openEndpoint := func(label string) (*Endpoint, *CompletionQueue) {
		cq, err := domain.OpenCompletionQueue(nil)
		if err != nil {
			t.Skipf("%s: open completion queue failed: %v", label, err)
			return nil, nil
		}

		ep, err := chosen.OpenEndpoint(domain)
		if err != nil {
			_ = cq.Close()
			t.Skipf("%s: open endpoint failed: %v", label, err)
			return nil, nil
		}

		t.Cleanup(func() {
			_ = ep.Close()
			_ = cq.Close()
		})

		if err := ep.BindCompletionQueue(cq, BindSend|BindRecv); err != nil {
			if shouldSkipForErr(err) {
				t.Skipf("%s: BindCompletionQueue unsupported: %v", label, err)
				return nil, nil
			}
			t.Fatalf("%s: BindCompletionQueue failed: %v", label, err)
		}

		if err := ep.BindAddressVector(av, 0); err != nil {
			if shouldSkipForErr(err) {
				t.Skipf("%s: BindAddressVector unsupported: %v", label, err)
				return nil, nil
			}
			t.Fatalf("%s: BindAddressVector failed: %v", label, err)
		}

		if err := ep.Enable(); err != nil {
			if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
				t.Skipf("%s: endpoint enable unsupported: %v", label, err)
				return nil, nil
			}
			t.Fatalf("%s: endpoint enable failed: %v", label, err)
		}

		return ep, cq
	}

	epA, cqA := openEndpoint("initiator")
	if epA == nil {
		return
	}
	epB, _ := openEndpoint("target")
	if epB == nil {
		return
	}

	addrB, err := epB.RegisterAddress(av, 0)
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("RegisterAddress unsupported: %v", err)
			return
		}
		t.Fatalf("RegisterAddress failed: %v", err)
	}

	writePayload := []byte("rma write payload")
	localMR, err := domain.RegisterMemory(make([]byte, len(writePayload)), MRAccessLocal)
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("local RegisterMemory unsupported: %v", err)
			return
		}
		t.Fatalf("RegisterMemory(local) failed: %v", err)
	}
	t.Cleanup(func() { _ = localMR.Close() })
	copy(localMR.Bytes(), writePayload)

	remoteMR, err := domain.RegisterMemory(make([]byte, len(writePayload)), MRAccessLocal|MRAccessRemoteRead|MRAccessRemoteWrite)
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("remote RegisterMemory unsupported: %v", err)
			return
		}
		t.Fatalf("RegisterMemory(remote) failed: %v", err)
	}
	t.Cleanup(func() { _ = remoteMR.Close() })

	writeCtx, err := epA.PostWrite(&RMARequest{
		Region:  localMR,
		Address: addrB,
		Key:     remoteMR.Key(),
	})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostWrite unsupported: %v", err)
			return
		}
		t.Fatalf("PostWrite failed: %v", err)
	}

	if err := waitForContext(cqA, writeCtx, time.Second); err != nil {
		if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
			t.Skipf("write completion unavailable: %v", err)
			return
		}
		t.Fatalf("wait for write completion failed: %v", err)
	}

	if got := string(remoteMR.Bytes()); got != string(writePayload) {
		t.Fatalf("remote region mismatch after write: got %q want %q", got, string(writePayload))
	}

	readPayload := []byte("rma read payload")
	copy(remoteMR.Bytes(), readPayload)

	readBacking := make([]byte, len(readPayload))
	readMR, err := domain.RegisterMemory(make([]byte, len(readPayload)), MRAccessLocal)
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("read RegisterMemory unsupported: %v", err)
			return
		}
		t.Fatalf("RegisterMemory(read) failed: %v", err)
	}
	t.Cleanup(func() { _ = readMR.Close() })

	readCtx, err := epA.PostRead(&RMARequest{
		Region:  readMR,
		Buffer:  readBacking,
		Address: addrB,
		Key:     remoteMR.Key(),
	})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostRead unsupported: %v", err)
			return
		}
		t.Fatalf("PostRead failed: %v", err)
	}

	if err := waitForContext(cqA, readCtx, time.Second); err != nil {
		if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
			t.Skipf("read completion unavailable: %v", err)
			return
		}
		t.Fatalf("wait for read completion failed: %v", err)
	}

	if got := string(readBacking); got != string(readPayload) {
		t.Fatalf("read payload mismatch: got %q want %q", got, string(readPayload))
	}
	if got := string(readMR.Bytes()); got != string(readPayload) {
		t.Fatalf("read region contents mismatch: got %q want %q", got, string(readPayload))
	}
}

func parseRMAProviders() (map[string]struct{}, bool) {
	raw := os.Getenv("LIBFABRIC_TEST_RMA_PROVIDERS")
	if raw == "" {
		return nil, false
	}
	allowed := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		allowed[name] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, false
	}
	return allowed, true
}

func parseRMAHints() map[string]*rmaProviderHints {
	raw := os.Getenv("LIBFABRIC_TEST_RMA_HINTS")
	if raw == "" {
		return nil
	}
	hints := make(map[string]*rmaProviderHints)
	for _, entry := range strings.Split(raw, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		provider := strings.ToLower(strings.TrimSpace(parts[0]))
		if provider == "" {
			continue
		}
		hint := hints[provider]
		if hint == nil {
			hint = &rmaProviderHints{}
			hints[provider] = hint
		}
		if len(parts) == 1 {
			continue
		}
		for _, assignment := range strings.Split(parts[1], ",") {
			token := strings.TrimSpace(assignment)
			if token == "" {
				continue
			}
			kv := strings.SplitN(token, "=", 2)
			key := strings.ToLower(strings.TrimSpace(kv[0]))
			value := ""
			if len(kv) == 2 {
				value = strings.TrimSpace(kv[1])
			}
			switch key {
			case "endpoint_type":
				if ep, ok := parseEndpointType(value); ok {
					hint.EndpointType = ep
				}
			}
		}
	}
	if len(hints) == 0 {
		return nil
	}
	return hints
}

func parseEndpointType(value string) (EndpointType, bool) {
	switch strings.ToLower(value) {
	case "msg":
		return EndpointTypeMsg, true
	case "rdm":
		return EndpointTypeRDM, true
	case "dgram":
		return EndpointTypeDgram, true
	default:
		return 0, false
	}
}

func selectDescriptor(descs []Descriptor) Descriptor {
	chosen := descs[0]
	for _, candidate := range descs {
		info := candidate.Info()
		fabric := strings.ToLower(info.Fabric)
		if fabric == "lo" || fabric == "lo0" || strings.HasPrefix(fabric, "loopback") {
			chosen = candidate
			break
		}
	}
	return chosen
}
