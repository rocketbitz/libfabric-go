//go:build integration

package integration

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rocketbitz/libfabric-go/client"
	fi "github.com/rocketbitz/libfabric-go/fi"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestClientMSGEndToEnd(t *testing.T) {
	cfg := integrationProviderConfig()
	applyProviderEnvironment(t, cfg)

	listenNode := cfg.Node
	clientNode := cfg.Node
	if clientNode == "" {
		clientNode = "127.0.0.1"
	}

	service := cfg.Service
	if service == "" {
		service = pickServicePort()
	}

	listenerCfg := client.ListenerConfig{Provider: cfg.Provider, Service: service}
	if listenNode != "" {
		listenerCfg.Node = listenNode
	}

	listener, err := client.Listen(listenerCfg)
	if err != nil {
		t.Skipf("MSG listener unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	serverDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		srv, err := listener.Accept(ctx)
		if err != nil {
			serverDone <- fmt.Errorf("accept MSG connection: %w", err)
			return
		}
		defer srv.Close()

		buf := make([]byte, 64)
		n, err := srv.Receive(ctx, buf)
		if err != nil {
			serverDone <- fmt.Errorf("receive payload: %w", err)
			return
		}
		msg := string(buf[:n])
		if msg != "hello libfabric" {
			serverDone <- fmt.Errorf("unexpected payload %q", msg)
			return
		}

		if err := srv.Send(ctx, []byte("ack:"+msg)); err != nil {
			serverDone <- fmt.Errorf("send reply: %w", err)
			return
		}
		serverDone <- nil
	}()

	clientCfg := client.Config{
		Provider:     cfg.Provider,
		Service:      service,
		EndpointType: fi.EndpointTypeMsg,
		Timeout:      5 * time.Second,
	}
	clientCfg.Node = clientNode

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dialer, err := client.Connect(clientCfg)
	if err != nil {
		t.Skipf("MSG connect unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = dialer.Close()
	})

	require.NoError(t, dialer.Send(ctx, []byte("hello libfabric")))
	resp := make([]byte, 64)
	n, err := dialer.Receive(ctx, resp)
	require.NoError(t, err, "receive response")
	require.Equal(t, "ack:hello libfabric", string(resp[:n]))

	select {
	case err := <-serverDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("listener goroutine did not complete")
	}
}

func integrationProviderConfig() providerConfig {
	defaults := providerConfig{
		Provider: firstNonEmpty(
			os.Getenv("LIBFABRIC_E2E_PROVIDER"),
			os.Getenv("LIBFABRIC_INTEGRATION_PROVIDER"),
			"sockets",
		),
		Node: firstNonEmpty(
			os.Getenv("LIBFABRIC_E2E_NODE"),
			os.Getenv("LIBFABRIC_INTEGRATION_NODE"),
		),
		Service: os.Getenv("LIBFABRIC_E2E_SERVICE"),
	}

	configs := providerConfigs(
		firstNonEmpty(os.Getenv("LIBFABRIC_E2E_PROVIDERS"), os.Getenv("LIBFABRIC_TEST_CLIENT_MSG_PROVIDERS")),
		firstNonEmpty(os.Getenv("LIBFABRIC_E2E_HINTS"), os.Getenv("LIBFABRIC_TEST_CLIENT_MSG_HINTS")),
		[]providerConfig{defaults},
	)

	cfg := defaults
	if len(configs) > 0 {
		cfg = configs[0]
	}

	if provider := firstNonEmpty(os.Getenv("LIBFABRIC_E2E_PROVIDER"), os.Getenv("LIBFABRIC_INTEGRATION_PROVIDER")); provider != "" {
		cfg.Provider = provider
	}
	if node := firstNonEmpty(os.Getenv("LIBFABRIC_E2E_NODE"), os.Getenv("LIBFABRIC_INTEGRATION_NODE")); node != "" {
		cfg.Node = node
	}
	if service := os.Getenv("LIBFABRIC_E2E_SERVICE"); service != "" {
		cfg.Service = service
	}
	if cfg.Provider == "" {
		cfg.Provider = "sockets"
	}
	return cfg
}

func applyProviderEnvironment(t *testing.T, cfg providerConfig) {
	for key, value := range cfg.Env {
		if value == "" {
			continue
		}
		t.Setenv(key, value)
	}
	if strings.EqualFold(cfg.Provider, "sockets") {
		if cfg.Env == nil || cfg.Env["FI_SOCKETS_IFACE"] == "" {
			if os.Getenv("FI_SOCKETS_IFACE") == "" {
				if iface := defaultLoopbackInterface(); iface != "" {
					t.Setenv("FI_SOCKETS_IFACE", iface)
				}
			}
		}
	}
}

type providerConfig struct {
	Provider string
	Node     string
	Service  string
	Env      map[string]string
}

func providerConfigs(providersEnv, hintsEnv string, defaults []providerConfig) []providerConfig {
	raw := strings.TrimSpace(providersEnv)
	hints := parseProviderHints(hintsEnv)

	var configs []providerConfig
	if raw == "" {
		configs = append(configs, defaults...)
	} else {
		for _, part := range strings.Split(raw, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			configs = append(configs, providerConfig{Provider: name})
		}
		if len(configs) == 0 {
			configs = append(configs, defaults...)
		}
	}
	if len(configs) == 0 {
		return nil
	}

	result := make([]providerConfig, 0, len(configs))
	for _, cfg := range configs {
		lower := strings.ToLower(cfg.Provider)
		cfg = applyProviderHints(cfg, hints[lower])
		result = append(result, cfg)
	}
	return result
}

func parseProviderHints(raw string) map[string]map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	hints := make(map[string]map[string]string)
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
			hint = make(map[string]string)
			hints[provider] = hint
		}
		if len(parts) == 1 {
			continue
		}
		for _, kv := range strings.Split(parts[1], ",") {
			kv = strings.TrimSpace(kv)
			if kv == "" {
				continue
			}
			pair := strings.SplitN(kv, "=", 2)
			key := strings.ToLower(strings.TrimSpace(pair[0]))
			value := ""
			if len(pair) == 2 {
				value = strings.TrimSpace(pair[1])
			}
			hint[key] = value
		}
	}
	if len(hints) == 0 {
		return nil
	}
	return hints
}

func applyProviderHints(cfg providerConfig, hint map[string]string) providerConfig {
	if len(hint) == 0 {
		return cfg
	}
	if v := hint["provider"]; v != "" && cfg.Provider == "" {
		cfg.Provider = v
	}
	if v := hint["node"]; v != "" {
		cfg.Node = v
	}
	if v := hint["service"]; v != "" {
		cfg.Service = v
	}
	if v := hint["iface"]; v != "" {
		if cfg.Env == nil {
			cfg.Env = make(map[string]string)
		}
		cfg.Env["FI_SOCKETS_IFACE"] = v
	}
	for key, value := range hint {
		if strings.HasPrefix(key, "env.") {
			name := strings.TrimPrefix(key, "env.")
			if name == "" {
				continue
			}
			if cfg.Env == nil {
				cfg.Env = make(map[string]string)
			}
			cfg.Env[name] = value
		}
	}
	return cfg
}

func defaultLoopbackInterface() string {
	switch runtime.GOOS {
	case "darwin":
		return "lo0"
	case "linux":
		return "lo"
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func pickServicePort() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err == nil {
		defer ln.Close()
		if tcp, ok := ln.Addr().(*net.TCPAddr); ok {
			return strconv.Itoa(tcp.Port)
		}
	}
	return strconv.Itoa(randomPort())
}

func randomPort() int {
	return 40000 + rand.Intn(20000)
}
