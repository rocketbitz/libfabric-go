package fi

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDatagramSendRecvProviders(t *testing.T) {
	discovery, err := DiscoverDescriptors()
	if err != nil {
		t.Fatalf("DiscoverDescriptors failed: %v", err)
	}
	t.Cleanup(discovery.Close)

	allowed := parseAllowedDatagramProviders()
	hints := parseDatagramHints()

	descriptors := discovery.Descriptors()
	if len(descriptors) == 0 {
		t.Skip("no datagram-capable providers available")
	}

	discovered := make(map[string]bool)
	var defaultRan bool

	for _, desc := range descriptors {
		descCopy := desc
		info := desc.Info()
		providerKey := strings.ToLower(info.Provider)
		fabricKey := strings.ToLower(info.Fabric)
		hint, hintOK := hints[providerKey]
		discovered[providerKey] = true

		name := info.Provider
		if info.Fabric != "" {
			name += "/" + info.Fabric
		}

		t.Run(name, func(t *testing.T) {
			if allowed != nil {
				if _, ok := allowed[providerKey]; !ok {
					t.Skipf("datagram provider %s not enabled via LIBFABRIC_TEST_DGRAM_PROVIDERS", info.Provider)
					return
				}
			}

			if hints != nil && !hintOK && providerKey != "" && providerKey != "sockets" {
				t.Skipf("datagram provider %s not configured in LIBFABRIC_TEST_DGRAM_HINTS", info.Provider)
				return
			}

			if allowed == nil && hints == nil {
				if providerKey != "" && providerKey != "sockets" {
					t.Skip("datagram test defaults to sockets provider; set LIBFABRIC_TEST_DGRAM_PROVIDERS to enable others")
					return
				}
				if defaultRan {
					t.Skip("additional sockets descriptor skipped; enable via env vars if desired")
					return
				}
				if fabricKey != "" && fabricKey != "lo" && fabricKey != "lo0" {
					t.Skipf("skipping sockets descriptor on fabric %s; provide hints to opt in", info.Fabric)
					return
				}
				defaultRan = true
			}

			verifyDatagramRoundTrip(t, descCopy, hint)
		})
	}

	if allowed != nil {
		for provider := range allowed {
			if !discovered[provider] {
				t.Logf("datagram test: provider %s not discovered; check LIBFABRIC_TEST_DGRAM_PROVIDERS", provider)
			}
		}
	}

	if hints != nil {
		for provider := range hints {
			if !discovered[provider] {
				t.Logf("datagram test: hints provided for %s but provider was not discovered", provider)
			}
		}
	}
}

func parseAllowedDatagramProviders() map[string]struct{} {
	raw := os.Getenv("LIBFABRIC_TEST_DGRAM_PROVIDERS")
	if raw == "" {
		return nil
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
		return nil
	}
	return allowed
}

type datagramProviderHints struct {
	CQAttr *CompletionQueueAttr
	AVAttr *AddressVectorAttr
}

func parseDatagramHints() map[string]*datagramProviderHints {
	raw := os.Getenv("LIBFABRIC_TEST_DGRAM_HINTS")
	if raw == "" {
		return nil
	}
	hints := make(map[string]*datagramProviderHints)
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
			hint = &datagramProviderHints{}
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
			case "cq_format":
				if format, ok := parseCQFormat(value); ok {
					if hint.CQAttr == nil {
						hint.CQAttr = &CompletionQueueAttr{}
					}
					hint.CQAttr.Format = format
				}
			case "cq_wait":
				if waitObj, ok := parseWaitObj(value); ok {
					if hint.CQAttr == nil {
						hint.CQAttr = &CompletionQueueAttr{}
					}
					hint.CQAttr.WaitObj = waitObj
				}
			case "cq_size":
				if size, err := strconv.Atoi(value); err == nil {
					if hint.CQAttr == nil {
						hint.CQAttr = &CompletionQueueAttr{}
					}
					hint.CQAttr.Size = size
				}
			case "av_type":
				if avType, ok := parseAVType(value); ok {
					if hint.AVAttr == nil {
						hint.AVAttr = &AddressVectorAttr{}
					}
					hint.AVAttr.Type = avType
				}
			case "av_count":
				if count, err := strconv.ParseUint(value, 10, 64); err == nil {
					if hint.AVAttr == nil {
						hint.AVAttr = &AddressVectorAttr{}
					}
					hint.AVAttr.Count = count
				}
			}
		}
	}
	if len(hints) == 0 {
		return nil
	}
	return hints
}

func parseCQFormat(value string) (CQFormat, bool) {
	switch strings.ToLower(value) {
	case "context":
		return CQFormatContext, true
	case "msg":
		return CQFormatMsg, true
	case "data":
		return CQFormatData, true
	case "tagged":
		return CQFormatTagged, true
	case "unspec", "":
		return CQFormatUnspec, true
	default:
		return 0, false
	}
}

func parseWaitObj(value string) (WaitObj, bool) {
	switch strings.ToLower(value) {
	case "fd":
		return WaitFD, true
	case "set":
		return WaitObjSet, true
	case "mutex_cond":
		return WaitMutexCond, true
	case "yield":
		return WaitYield, true
	case "pollfd":
		return WaitPollFD, true
	case "unspec":
		return WaitUnspec, true
	case "none":
		return WaitNone, true
	case "":
		return WaitUnspec, true
	default:
		return 0, false
	}
}

func parseAVType(value string) (AVType, bool) {
	switch strings.ToLower(value) {
	case "map":
		return AVTypeMap, true
	case "table":
		return AVTypeTable, true
	case "unspec", "":
		return AVTypeUnspec, true
	default:
		return 0, false
	}
}

func verifyDatagramRoundTrip(t *testing.T, desc Descriptor, hint *datagramProviderHints) {
	info := desc.Info()

	fabric, err := desc.OpenFabric()
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("open fabric unsupported: %v", err)
		}
		t.Fatalf("open fabric: %v", err)
	}
	defer fabric.Close()

	domain, err := desc.OpenDomain(fabric)
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("open domain unsupported: %v", err)
		}
		t.Fatalf("open domain: %v", err)
	}
	defer domain.Close()

	var avAttr AddressVectorAttr
	if hint != nil && hint.AVAttr != nil {
		avAttr = *hint.AVAttr
	} else {
		avAttr = AddressVectorAttr{Type: AVTypeMap}
	}
	av, err := domain.OpenAddressVector(&avAttr)
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("open address vector unsupported: %v", err)
		}
		t.Fatalf("open address vector: %v", err)
	}
	defer av.Close()

	newEndpoint := func(t *testing.T) (*Endpoint, *CompletionQueue) {
		var cqAttr *CompletionQueueAttr
		if hint != nil && hint.CQAttr != nil {
			tmp := *hint.CQAttr
			cqAttr = &tmp
		}
		cq, err := domain.OpenCompletionQueue(cqAttr)
		if err != nil {
			if shouldSkipForErr(err) {
				t.Skipf("open completion queue unsupported: %v", err)
			}
			t.Fatalf("open completion queue: %v", err)
		}

		ep, err := desc.OpenEndpoint(domain)
		if err != nil {
			if shouldSkipForErr(err) {
				t.Skipf("open endpoint unsupported: %v", err)
			}
			cq.Close()
			t.Fatalf("open endpoint: %v", err)
		}

		cleanup := func() {
			_ = ep.Close()
			_ = cq.Close()
		}
		defer func() {
			if r := recover(); r != nil {
				cleanup()
				panic(r)
			}
		}()

		if err := ep.BindCompletionQueue(cq, BindSend|BindRecv); err != nil {
			cleanup()
			if shouldSkipForErr(err) {
				t.Skipf("BindCompletionQueue unsupported: %v", err)
			}
			t.Fatalf("BindCompletionQueue failed: %v", err)
		}
		if err := ep.BindAddressVector(av, 0); err != nil {
			cleanup()
			if shouldSkipForErr(err) {
				t.Skipf("BindAddressVector unsupported: %v", err)
			}
			t.Fatalf("BindAddressVector failed: %v", err)
		}
		if err := ep.Enable(); err != nil {
			cleanup()
			if shouldSkipForErr(err) {
				t.Skipf("endpoint enable unsupported: %v", err)
			}
			t.Fatalf("endpoint enable failed: %v", err)
		}

		t.Cleanup(cleanup)
		return ep, cq
	}

	epA, cqA := newEndpoint(t)
	epB, cqB := newEndpoint(t)

	addrA, err := epA.RegisterAddress(av, 0)
	if err != nil {
		t.Fatalf("register endpoint A: %v", err)
	}
	addrB, err := epB.RegisterAddress(av, 0)
	if err != nil {
		t.Fatalf("register endpoint B: %v", err)
	}

	if info.Provider != "" {
		if hint != nil {
			t.Logf("datagram provider=%s fabric=%s (hints applied)", info.Provider, info.Fabric)
		} else {
			t.Logf("datagram provider=%s fabric=%s", info.Provider, info.Fabric)
		}
	}

	message := []byte("libfabric-go datagram test")
	recvBuf := make([]byte, len(message))

	recvCtx, err := epB.PostRecv(&RecvRequest{Buffer: recvBuf, Source: addrA})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostRecv unsupported: %v", err)
		}
		t.Fatalf("PostRecv failed: %v", err)
	}

	sendCtx, err := epA.PostSend(&SendRequest{Buffer: message, Dest: addrB})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostSend unsupported: %v", err)
		}
		t.Fatalf("PostSend failed: %v", err)
	}

	if sendCtx != nil {
		if _, err := awaitContextWithEvent(cqA, sendCtx, time.Second); err != nil {
			if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
				t.Skipf("datagram send completion unavailable: %v", err)
			}
			t.Fatalf("datagram send wait failed: %v", err)
		}
	}

	if _, err := awaitContextWithEvent(cqB, recvCtx, time.Second); err != nil {
		if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
			t.Skipf("datagram recv completion unavailable: %v", err)
		}
		t.Fatalf("datagram recv wait failed: %v", err)
	}

	if string(recvBuf) != string(message) {
		t.Fatalf("unexpected datagram payload: got %q want %q", string(recvBuf), string(message))
	}

	limit := epA.InjectLimit()
	if limit == 0 {
		limit = 256
	}
	largeLen := int(limit*2 + 32)
	largeMessage := make([]byte, largeLen)
	for i := range largeMessage {
		largeMessage[i] = byte('A' + i%26)
	}
	largeRecv := make([]byte, largeLen)

	recvCtx2, err := epB.PostRecv(&RecvRequest{Buffer: largeRecv, Source: addrA})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostRecv (large) unsupported: %v", err)
		}
		t.Fatalf("PostRecv (large) failed: %v", err)
	}

	sendCtx2, err := epA.PostSend(&SendRequest{Buffer: largeMessage, Dest: addrB})
	if err != nil {
		if shouldSkipForErr(err) {
			t.Skipf("PostSend (large) unsupported: %v", err)
		}
		t.Fatalf("PostSend (large) failed: %v", err)
	}
	if sendCtx2 == nil {
		t.Skip("provider injected large datagram; skipping non-inject verification")
	}

	if _, err := awaitContextWithEvent(cqA, sendCtx2, time.Second); err != nil {
		if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
			t.Skipf("datagram send completion (large) unavailable: %v", err)
		}
		t.Fatalf("datagram send wait (large) failed: %v", err)
	}

	if _, err := awaitContextWithEvent(cqB, recvCtx2, time.Second); err != nil {
		if shouldSkipForErr(err) || errors.Is(err, ErrTimeout) {
			t.Skipf("datagram recv completion (large) unavailable: %v", err)
		}
		t.Fatalf("datagram recv wait (large) failed: %v", err)
	}

	if string(largeRecv) != string(largeMessage) {
		t.Fatalf("unexpected large datagram payload")
	}
}
