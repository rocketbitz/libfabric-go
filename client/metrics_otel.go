package client

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelMetricsOptions configures NewOTelMetrics.
type OTelMetricsOptions struct {
	MeterProvider          metric.MeterProvider
	Meter                  metric.Meter
	InstrumentationName    string
	InstrumentationVersion string
}

var _ MetricHook = (*OTelMetrics)(nil)

// OTelMetrics implements MetricHook using OpenTelemetry counters.
type OTelMetrics struct {
	meter             metric.Meter
	dispatcherStarted metric.Int64Counter
	dispatcherStopped metric.Int64Counter
	dispatcherCQError metric.Int64Counter
	sendCompleted     metric.Int64Counter
	sendFailed        metric.Int64Counter
	receiveCompleted  metric.Int64Counter
	receiveFailed     metric.Int64Counter
}

// NewOTelMetrics constructs a MetricHook that emits OpenTelemetry counter measurements.
func NewOTelMetrics(opts OTelMetricsOptions) (*OTelMetrics, error) {
	meter := opts.Meter
	if meter == nil {
		provider := opts.MeterProvider
		if provider == nil {
			provider = otel.GetMeterProvider()
		}
		name := opts.InstrumentationName
		if name == "" {
			name = "github.com/rocketbitz/libfabric-go/client"
		}
		meter = provider.Meter(name, metric.WithInstrumentationVersion(opts.InstrumentationVersion))
	}

	dispatcherStarted, err := meter.Int64Counter("libfabric.client.dispatcher.started")
	if err != nil {
		return nil, err
	}
	dispatcherStopped, err := meter.Int64Counter("libfabric.client.dispatcher.stopped")
	if err != nil {
		return nil, err
	}
	dispatcherCQError, err := meter.Int64Counter("libfabric.client.dispatcher.cq_errors")
	if err != nil {
		return nil, err
	}
	sendCompleted, err := meter.Int64Counter("libfabric.client.send.completed")
	if err != nil {
		return nil, err
	}
	sendFailed, err := meter.Int64Counter("libfabric.client.send.failed")
	if err != nil {
		return nil, err
	}
	receiveCompleted, err := meter.Int64Counter("libfabric.client.receive.completed")
	if err != nil {
		return nil, err
	}
	receiveFailed, err := meter.Int64Counter("libfabric.client.receive.failed")
	if err != nil {
		return nil, err
	}

	return &OTelMetrics{
		meter:             meter,
		dispatcherStarted: dispatcherStarted,
		dispatcherStopped: dispatcherStopped,
		dispatcherCQError: dispatcherCQError,
		sendCompleted:     sendCompleted,
		sendFailed:        sendFailed,
		receiveCompleted:  receiveCompleted,
		receiveFailed:     receiveFailed,
	}, nil
}

// DispatcherStarted records that the dispatcher loop has started executing.
func (o *OTelMetrics) DispatcherStarted(attrs map[string]string) {
	o.dispatcherStarted.Add(context.Background(), 1, metric.WithAttributes(otelAttrs(attrs)...))
}

// DispatcherStopped records that the dispatcher loop has exited.
func (o *OTelMetrics) DispatcherStopped(attrs map[string]string) {
	o.dispatcherStopped.Add(context.Background(), 1, metric.WithAttributes(otelAttrs(attrs)...))
}

// DispatcherCQError counts completion queue errors observed by the dispatcher.
func (o *OTelMetrics) DispatcherCQError(kind string, _ error, attrs map[string]string) {
	attributes := append(otelAttrs(attrs), attribute.String(labelKind, kind))
	o.dispatcherCQError.Add(context.Background(), 1, metric.WithAttributes(attributes...))
}

// SendCompleted records a successful send completion.
func (o *OTelMetrics) SendCompleted(attrs map[string]string) {
	o.sendCompleted.Add(context.Background(), 1, metric.WithAttributes(otelAttrsWithOperation(attrs)...))
}

// SendFailed records a failed send completion.
func (o *OTelMetrics) SendFailed(_ error, attrs map[string]string) {
	o.sendFailed.Add(context.Background(), 1, metric.WithAttributes(otelAttrsWithOperation(attrs)...))
}

// ReceiveCompleted records a successful receive completion.
func (o *OTelMetrics) ReceiveCompleted(attrs map[string]string) {
	o.receiveCompleted.Add(context.Background(), 1, metric.WithAttributes(otelAttrsWithOperation(attrs)...))
}

// ReceiveFailed records a failed receive completion.
func (o *OTelMetrics) ReceiveFailed(_ error, attrs map[string]string) {
	o.receiveFailed.Add(context.Background(), 1, metric.WithAttributes(otelAttrsWithOperation(attrs)...))
}

func otelAttrs(attrs map[string]string) []attribute.KeyValue {
	kvs := []attribute.KeyValue{
		attribute.String(labelEndpointType, attrs[labelEndpointType]),
		attribute.String(labelProvider, attrs[labelProvider]),
	}
	if v := attrs[labelNode]; v != "" {
		kvs = append(kvs, attribute.String(labelNode, v))
	}
	if v := attrs[labelService]; v != "" {
		kvs = append(kvs, attribute.String(labelService, v))
	}
	return kvs
}

func otelAttrsWithOperation(attrs map[string]string) []attribute.KeyValue {
	kvs := otelAttrs(attrs)
	if v := attrs[labelOperation]; v != "" {
		kvs = append(kvs, attribute.String(labelOperation, v))
	}
	if v := attrs[labelStatus]; v != "" {
		kvs = append(kvs, attribute.String(labelStatus, v))
	}
	return kvs
}
