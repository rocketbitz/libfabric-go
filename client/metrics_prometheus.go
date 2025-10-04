package client

import "github.com/prometheus/client_golang/prometheus"

// PrometheusMetricsOptions configures NewPrometheusMetrics.
type PrometheusMetricsOptions struct {
	Registerer  prometheus.Registerer
	Namespace   string
	Subsystem   string
	ConstLabels prometheus.Labels
}

// PrometheusMetrics implements MetricHook using Prometheus counters.
var _ MetricHook = (*PrometheusMetrics)(nil)

// PrometheusMetrics implements MetricHook using Prometheus counters.
type PrometheusMetrics struct {
	dispatcherStarted *prometheus.CounterVec
	dispatcherStopped *prometheus.CounterVec
	dispatcherCQError *prometheus.CounterVec
	sendCompleted     *prometheus.CounterVec
	sendFailed        *prometheus.CounterVec
	receiveCompleted  *prometheus.CounterVec
	receiveFailed     *prometheus.CounterVec
}

// NewPrometheusMetrics constructs a MetricHook backed by Prometheus counters.
func NewPrometheusMetrics(opts PrometheusMetricsOptions) (*PrometheusMetrics, error) {
	reg := opts.Registerer
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	p := &PrometheusMetrics{
		dispatcherStarted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "libfabric_client_dispatcher_started_total",
			Help:        "Number of times the dispatcher loop started",
			ConstLabels: opts.ConstLabels,
		}, dispatcherLabelKeys),
		dispatcherStopped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "libfabric_client_dispatcher_stopped_total",
			Help:        "Number of times the dispatcher loop stopped",
			ConstLabels: opts.ConstLabels,
		}, dispatcherLabelKeys),
		dispatcherCQError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "libfabric_client_dispatcher_cq_errors_total",
			Help:        "Number of completion queue read errors surfaced by the dispatcher",
			ConstLabels: opts.ConstLabels,
		}, cqErrorLabelKeys),
		sendCompleted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "libfabric_client_send_completed_total",
			Help:        "Number of successful send completions",
			ConstLabels: opts.ConstLabels,
		}, completionLabelKeys),
		sendFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "libfabric_client_send_failed_total",
			Help:        "Number of errored send completions",
			ConstLabels: opts.ConstLabels,
		}, failureLabelKeys),
		receiveCompleted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "libfabric_client_receive_completed_total",
			Help:        "Number of successful receive completions",
			ConstLabels: opts.ConstLabels,
		}, completionLabelKeys),
		receiveFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "libfabric_client_receive_failed_total",
			Help:        "Number of errored receive completions",
			ConstLabels: opts.ConstLabels,
		}, failureLabelKeys),
	}

	var err error
	if p.dispatcherStarted, err = registerCounterVec(reg, p.dispatcherStarted); err != nil {
		return nil, err
	}
	if p.dispatcherStopped, err = registerCounterVec(reg, p.dispatcherStopped); err != nil {
		return nil, err
	}
	if p.dispatcherCQError, err = registerCounterVec(reg, p.dispatcherCQError); err != nil {
		return nil, err
	}
	if p.sendCompleted, err = registerCounterVec(reg, p.sendCompleted); err != nil {
		return nil, err
	}
	if p.sendFailed, err = registerCounterVec(reg, p.sendFailed); err != nil {
		return nil, err
	}
	if p.receiveCompleted, err = registerCounterVec(reg, p.receiveCompleted); err != nil {
		return nil, err
	}
	if p.receiveFailed, err = registerCounterVec(reg, p.receiveFailed); err != nil {
		return nil, err
	}

	return p, nil
}

var (
	dispatcherLabelKeys = []string{labelEndpointType, labelProvider, labelNode, labelService}
	cqErrorLabelKeys    = []string{labelEndpointType, labelProvider, labelNode, labelService, labelKind}
	completionLabelKeys = []string{labelEndpointType, labelProvider, labelNode, labelService, labelOperation, labelStatus}
	failureLabelKeys    = []string{labelEndpointType, labelProvider, labelNode, labelService, labelOperation}
)

func (p *PrometheusMetrics) DispatcherStarted(attrs map[string]string) {
	p.dispatcherStarted.With(labels(attrs, dispatcherLabelKeys...)).Inc()
}

func (p *PrometheusMetrics) DispatcherStopped(attrs map[string]string) {
	p.dispatcherStopped.With(labels(attrs, dispatcherLabelKeys...)).Inc()
}

func (p *PrometheusMetrics) DispatcherCQError(kind string, _ error, attrs map[string]string) {
	labs := labels(attrs, cqErrorLabelKeys...)
	labs[labelKind] = kind
	p.dispatcherCQError.With(labs).Inc()
}

func (p *PrometheusMetrics) SendCompleted(attrs map[string]string) {
	p.sendCompleted.With(labels(attrs, completionLabelKeys...)).Inc()
}

func (p *PrometheusMetrics) SendFailed(_ error, attrs map[string]string) {
	p.sendFailed.With(labels(attrs, failureLabelKeys...)).Inc()
}

func (p *PrometheusMetrics) ReceiveCompleted(attrs map[string]string) {
	p.receiveCompleted.With(labels(attrs, completionLabelKeys...)).Inc()
}

func (p *PrometheusMetrics) ReceiveFailed(_ error, attrs map[string]string) {
	p.receiveFailed.With(labels(attrs, failureLabelKeys...)).Inc()
}

func registerCounterVec(reg prometheus.Registerer, vec *prometheus.CounterVec) (*prometheus.CounterVec, error) {
	if err := reg.Register(vec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing, nil
			}
		}
		return nil, err
	}
	return vec, nil
}

func labels(attrs map[string]string, keys ...string) prometheus.Labels {
	labs := make(prometheus.Labels, len(keys))
	for _, key := range keys {
		labs[key] = attrs[key]
	}
	return labs
}
