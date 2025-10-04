package client

import (
	"context"
	"errors"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestOTelMetricsCounters(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	metrics, err := NewOTelMetrics(OTelMetricsOptions{MeterProvider: provider})
	if err != nil {
		t.Fatalf("NewOTelMetrics: %v", err)
	}

	base := map[string]string{
		labelEndpointType: "rdm",
		labelProvider:     "sockets",
		labelNode:         "node0",
		labelService:      "demo",
	}
	metrics.DispatcherStarted(base)
	metrics.DispatcherStopped(base)
	metrics.DispatcherCQError("cq_read_error", errors.New("boom"), base)

	sendAttrs := map[string]string{
		labelEndpointType: "rdm",
		labelProvider:     "sockets",
		labelNode:         "node0",
		labelService:      "demo",
		labelOperation:    "send",
		labelStatus:       "ok",
	}
	metrics.SendCompleted(sendAttrs)
	metrics.SendFailed(errors.New("fail"), sendAttrs)
	metrics.ReceiveCompleted(sendAttrs)
	metrics.ReceiveFailed(errors.New("rfail"), sendAttrs)

	ctx := context.Background()
	if err := provider.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	cases := map[string]float64{
		"libfabric.client.dispatcher.started":   1,
		"libfabric.client.dispatcher.stopped":   1,
		"libfabric.client.dispatcher.cq_errors": 1,
		"libfabric.client.send.completed":       1,
		"libfabric.client.send.failed":          1,
		"libfabric.client.receive.completed":    1,
		"libfabric.client.receive.failed":       1,
	}

	for name, want := range cases {
		if got := otelCounterValue(rm, name); got != want {
			t.Fatalf("unexpected counter %s: got %v want %v", name, got, want)
		}
	}

	if err := provider.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func otelCounterValue(rm metricdata.ResourceMetrics, name string) float64 {
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != name {
				continue
			}
			switch data := metric.Data.(type) {
			case metricdata.Sum[int64]:
				var sum float64
				for _, dp := range data.DataPoints {
					sum += float64(dp.Value)
				}
				return sum
			}
		}
	}
	return 0
}
