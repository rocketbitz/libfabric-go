package client

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestPrometheusMetricsCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics, err := NewPrometheusMetrics(PrometheusMetricsOptions{Registerer: reg})
	if err != nil {
		t.Fatalf("NewPrometheusMetrics: %v", err)
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

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	cases := map[string]float64{
		"libfabric_client_dispatcher_started_total":   1,
		"libfabric_client_dispatcher_stopped_total":   1,
		"libfabric_client_dispatcher_cq_errors_total": 1,
		"libfabric_client_send_completed_total":       1,
		"libfabric_client_send_failed_total":          1,
		"libfabric_client_receive_completed_total":    1,
		"libfabric_client_receive_failed_total":       1,
	}

	for name, want := range cases {
		if got := findCounterValue(mfs, name); got != want {
			t.Fatalf("unexpected counter %s: got %v want %v", name, got, want)
		}
	}
}

func findCounterValue(mfs []*dto.MetricFamily, name string) float64 {
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		var sum float64
		for _, m := range mf.Metric {
			sum += m.GetCounter().GetValue()
		}
		return sum
	}
	return 0
}
