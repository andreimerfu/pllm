// Package metrics publishes Prometheus counters and gauges for the
// deployment service. Scope is intentionally small — only the auto-scaler
// exposes anything here today; drift, crashloops etc. can follow the
// same pattern if we need them later.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	oomBumpsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pllm",
		Subsystem: "deployment",
		Name:      "oom_bumps_total",
		Help:      "Count of automatic memory bumps performed by the deployment OOM auto-scaler, labeled by namespace/workload/new_limit.",
	}, []string{"namespace", "workload", "new_limit"})

	oomAttempts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pllm",
		Subsystem: "deployment",
		Name:      "oom_attempts",
		Help:      "Current OOM bump counter for each workload. Reset to zero when the workload has been running long enough for the reset_after_healthy window.",
	}, []string{"namespace", "workload"})
)

// Sink implements deployment.MetricsSink against the default Prometheus
// registry. One-liner wiring from the router; nothing stateful here.
type Sink struct{}

// NewSink returns the default Sink. Calling it more than once is safe;
// the underlying vectors come from promauto (singleton-backed).
func NewSink() Sink { return Sink{} }

// IncOOMBump implements deployment.MetricsSink.
func (Sink) IncOOMBump(namespace, workload, newLimit string) {
	oomBumpsTotal.WithLabelValues(namespace, workload, newLimit).Inc()
}

// SetOOMAttempts implements deployment.MetricsSink.
func (Sink) SetOOMAttempts(namespace, workload string, attempts int) {
	oomAttempts.WithLabelValues(namespace, workload).Set(float64(attempts))
}
